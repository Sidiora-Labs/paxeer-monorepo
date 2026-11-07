//! Repair record 22.31.198, finding artifact-revocation-restore:
//! startup_requires_both_markers_outside_data_including_symlinks covers placement;
//! grant_revocation_survives_restart_and_refuses_old_backup and
//! publisher_revocation_survives_restart_and_refuses_old_backup cover recovery;
//! retention_barrier_independently_refuses_old_backup covers the other barrier.
//! No out-of-scope wiring is required. Baseline verification was not run because
//! the ordered gate contract requires complete implementation before any gate.
//! The verified candidate revision and private evidence are recorded by hf-done.
#[path = "../src/crypto.rs"]
#[allow(dead_code)]
mod crypto;

use chacha20poly1305::aead::{Aead, KeyInit, Payload};
use chacha20poly1305::{XChaCha20Poly1305, XNonce};
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::codec::derive_market;
use layerx_programs_ai_market::evidence::{
    encode_envelope, encode_manifest, manifest_root, object_content_root, publisher_digest,
    ArtifactContext, ArtifactKind, ArtifactManifest, Items, Privacy, PublisherEnvelope,
    MAX_ENVELOPE_BYTES, MAX_MANIFEST_BYTES,
};
use layerx_programs_ai_market::types::{
    ChainDomain, PolicyDigest, PrincipalId, ProgramId, PublicKey32, Signature64, Version,
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpStream;
use std::os::unix::fs::symlink;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, ExitStatus, Stdio};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

const OWNER_TOKEN: &str = "repair-publisher-token";
const READER_TOKEN: &str = "repair-reader-token";
const PRINCIPAL: [u8; 32] = [0x55; 32];
const TASK: [u8; 32] = [0x31; 32];
const PLAIN: &[u8] = b"artifact whose authority must not return from a backup";

fn context() -> ArtifactContext {
    let chain = ChainDomain::new([0x11; 32]).unwrap();
    let program = ProgramId::new([0x22; 32]).unwrap();
    ArtifactContext {
        chain,
        program,
        market: derive_market(chain, program).unwrap(),
        policy: PolicyDigest::new([0x44; 32]).unwrap(),
    }
}

fn request_id(n: u8) -> String {
    hex::encode([n; 32])
}

struct Fixture {
    root: PathBuf,
    data: PathBuf,
    revocation: PathBuf,
    retention: PathBuf,
}

impl Fixture {
    fn new() -> Self {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let root = PathBuf::from(env!("CARGO_TARGET_TMPDIR"))
            .join(format!("repair-22-31-198-{}-{nanos}", std::process::id()));
        fs::create_dir_all(&root).unwrap();
        let root = root.canonicalize().unwrap();
        fs::write(root.join("kek.bin"), crypto::random::<32>().unwrap()).unwrap();
        Self {
            data: root.join("data"),
            revocation: root.join("revocation.mark"),
            retention: root.join("retention.mark"),
            root,
        }
    }

    fn start(&self) -> Result<Service, (ExitStatus, String)> {
        let hash = |token: &str| hex::encode(Sha256::digest(token.as_bytes()));
        let config = json!({
            "capacity_bytes": 1u64 << 20,
            "listener": "plaintext",
            "revocation_mark": self.revocation,
            "retention_mark": self.retention,
            "tenants": [
                {"id": "owner", "token_sha256": hash(OWNER_TOKEN), "quota_bytes": 1u64 << 20},
                {"id": "reader", "token_sha256": hash(READER_TOKEN), "quota_bytes": 0}
            ],
            "publishers": [{"tenant": "owner", "principal": hex::encode(PRINCIPAL),
                "generation": 1, "key": hex::encode(SigningKey::from_bytes(&[1; 32]).verifying_key().to_bytes())}]
        });
        fs::write(self.root.join("config.json"), config.to_string()).unwrap();
        let log = fs::File::create(self.root.join("service.log")).unwrap();
        let child = Command::new(env!("CARGO_BIN_EXE_layerx-ai-artifacts"))
            .arg("--data-dir")
            .arg(&self.data)
            .arg("--config")
            .arg(self.root.join("config.json"))
            .arg("--key-file")
            .arg(self.root.join("kek.bin"))
            .arg("--listen")
            .arg("127.0.0.1:0")
            .stdout(Stdio::piped())
            .stderr(Stdio::from(log))
            .spawn()
            .unwrap();
        let mut service = Service {
            child,
            addr: String::new(),
        };
        let stdout = service.child.stdout.take().unwrap();
        let (send, recv) = std::sync::mpsc::channel();
        std::thread::spawn(move || {
            let mut line = String::new();
            BufReader::new(stdout).read_line(&mut line).unwrap();
            let _ = send.send(line);
        });
        let line = recv
            .recv_timeout(Duration::from_secs(10))
            .expect("startup timed out");
        if let Some(addr) = line.trim().strip_prefix("listening ") {
            service.addr = addr.to_string();
            Ok(service)
        } else {
            let status = service.child.wait().unwrap();
            Err((
                status,
                fs::read_to_string(self.root.join("service.log")).unwrap(),
            ))
        }
    }

    fn running(&self) -> Service {
        self.start()
            .unwrap_or_else(|(status, log)| panic!("{status}: {log}"))
    }

    fn refused(&self, code: i32, reason: &str) {
        match self.start() {
            Ok(_) => panic!("startup accepted unsafe configuration or stale backup"),
            Err((status, log)) => {
                assert_eq!(status.code(), Some(code), "{log}");
                assert!(log.contains(reason), "{log}");
            }
        }
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root).unwrap();
    }
}

struct Service {
    child: Child,
    addr: String,
}

impl Drop for Service {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

fn http(service: &Service, method: &str, path: &str, token: &str, body: &[u8]) -> (u16, Vec<u8>) {
    let mut stream = TcpStream::connect(&service.addr).unwrap();
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap();
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .unwrap();
    write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {token}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap();
    stream.write_all(body).unwrap();
    let mut response = Vec::new();
    stream.read_to_end(&mut response).unwrap();
    let split = response.windows(4).position(|w| w == b"\r\n\r\n").unwrap();
    let status = std::str::from_utf8(&response[..split])
        .unwrap()
        .split_whitespace()
        .nth(1)
        .unwrap()
        .parse()
        .unwrap();
    (status, response[split + 4..].to_vec())
}

fn post(service: &Service, path: &str, token: &str, body: &Value) -> (u16, Value) {
    let (status, bytes) = http(service, "POST", path, token, body.to_string().as_bytes());
    (status, serde_json::from_slice(&bytes).unwrap())
}

fn publish(service: &Service) -> String {
    let framed_length = PLAIN.len() + 8 + 34 + 16;
    let (status, prepared) = post(
        service,
        "/v1/prepare",
        OWNER_TOKEN,
        &json!({
            "request_id": request_id(1), "kind": ArtifactKind::Input as u8,
            "privacy": Privacy::Encrypted as u8, "context": hex::encode(context().bytes()),
            "byte_length": framed_length
        }),
    );
    assert_eq!(status, 200, "{prepared}");
    let key: [u8; 32] = hex::decode(prepared["object_key"].as_str().unwrap())
        .unwrap()
        .try_into()
        .unwrap();
    let nonce = crypto::random::<24>().unwrap();
    let mut aad = crypto::FRAME_AAD_PREFIX.to_vec();
    aad.extend_from_slice(&context().bytes());
    aad.extend_from_slice(&TASK);
    aad.push(ArtifactKind::Input as u8);
    aad.extend_from_slice(&0u32.to_be_bytes());
    let plain = [&(PLAIN.len() as u64).to_be_bytes()[..], PLAIN].concat();
    let sealed = XChaCha20Poly1305::new((&key).into())
        .encrypt(
            XNonce::from_slice(&nonce),
            Payload {
                msg: &plain,
                aad: &aad,
            },
        )
        .unwrap();
    let mut bytes = crypto::FRAME_VERSION.to_be_bytes().to_vec();
    bytes.extend_from_slice(&0u32.to_be_bytes());
    bytes.extend_from_slice(&nonce);
    bytes.extend_from_slice(&(sealed.len() as u32).to_be_bytes());
    bytes.extend_from_slice(&sealed);
    assert_eq!(bytes.len(), framed_length);
    let session = prepared["session"].as_str().unwrap();
    let (status, body) = http(
        service,
        "PUT",
        &format!("/v1/upload/{session}/0"),
        OWNER_TOKEN,
        &bytes,
    );
    assert_eq!(status, 200, "{}", String::from_utf8_lossy(&body));
    let manifest = ArtifactManifest {
        kind: ArtifactKind::Input,
        privacy: Privacy::Encrypted,
        context: context(),
        epoch: 9,
        publisher: PrincipalId::new(PRINCIPAL).unwrap(),
        subject: TASK,
        byte_length: bytes.len() as u64,
        chunk_count: 1,
        content_root: object_content_root(&bytes, &mut [[0; 32]; 1]).unwrap(),
        parents: Items::Typed(&[]),
        declaration_root: [0; 32],
        reproduction_root: [0; 32],
        access_policy_root: [0x66; 32],
        not_after_height: 0,
    };
    let mut encoded = vec![0; MAX_MANIFEST_BYTES];
    let n = encode_manifest(&manifest, &mut encoded).unwrap();
    encoded.truncate(n);
    let signing_key = SigningKey::from_bytes(&[1; 32]);
    let generation = Version::new(1).unwrap();
    let digest = publisher_digest(&context(), manifest_root(&encoded).unwrap(), generation);
    let envelope = PublisherEnvelope {
        manifest: &encoded,
        generation,
        key: PublicKey32(signing_key.verifying_key().to_bytes()),
        signature: Signature64(signing_key.sign(&digest).to_bytes()),
    };
    let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
    let n = encode_envelope(&envelope, &mut encoded).unwrap();
    let (status, receipt) = post(
        service,
        "/v1/finalize",
        OWNER_TOKEN,
        &json!({
            "request_id": request_id(2), "session": session, "envelope": hex::encode(&encoded[..n])
        }),
    );
    assert_eq!(status, 200, "{receipt}");
    receipt["root"].as_str().unwrap().to_string()
}

fn grant(service: &Service, root: &str) -> Value {
    let (status, grant) = post(
        service,
        "/v1/grants",
        OWNER_TOKEN,
        &json!({
            "request_id": request_id(3), "root": root, "task": hex::encode(TASK),
            "grantee": "reader", "purposes": 2, "ttl_secs": 3600
        }),
    );
    assert_eq!(status, 200, "{grant}");
    grant
}

fn access(root: &str, grant: &Value, id: u8) -> Value {
    json!({"request_id": request_id(id), "root": root, "task": hex::encode(TASK),
        "purpose": "evaluation", "grant_id": grant["grant_id"], "index": 0})
}

fn assert_readable(service: &Service, access: &Value) {
    let (status, resolved) = post(service, "/v1/resolve", READER_TOKEN, access);
    assert_eq!(status, 200, "{resolved}");
    assert_eq!(resolved["status"], "AVAILABLE_AUTHORIZED");
    let key: [u8; 32] = hex::decode(resolved["object_key"].as_str().unwrap())
        .unwrap()
        .try_into()
        .unwrap();
    let (status, content) = http(
        service,
        "POST",
        "/v1/content",
        READER_TOKEN,
        access.to_string().as_bytes(),
    );
    assert_eq!(status, 200);
    let mut plain = Vec::new();
    crypto::open_stream(
        &key,
        &crypto::FrameContext {
            context: context().bytes(),
            subject: TASK,
            kind: ArtifactKind::Input as u8,
        },
        content.as_slice(),
        |part| plain.extend_from_slice(part),
    )
    .unwrap();
    assert_eq!(plain, PLAIN);
}

fn assert_revoked(service: &Service, access: &Value) {
    for route in ["/v1/resolve", "/v1/content"] {
        let (status, refusal) = post(service, route, READER_TOKEN, access);
        assert_eq!(status, 403, "{refusal}");
        assert_eq!(refusal["error"], "AuthorityRevoked");
        assert!(refusal.get("object_key").is_none());
    }
}

fn copy_dir(from: &Path, to: &Path) {
    fs::create_dir_all(to).unwrap();
    for entry in fs::read_dir(from).unwrap() {
        let entry = entry.unwrap();
        let target = to.join(entry.file_name());
        if entry.file_type().unwrap().is_dir() {
            copy_dir(&entry.path(), &target);
        } else {
            fs::copy(entry.path(), target).unwrap();
        }
    }
}

#[test]
fn startup_requires_both_markers_outside_data_including_symlinks() {
    for revocation in [true, false] {
        for case in [
            "absent",
            "existing",
            "parent_alias",
            "file_alias",
            "reverse_file_alias",
            "data_alias",
            "dangling_alias",
            "unresolvable",
        ] {
            let mut fixture = Fixture::new();
            fs::create_dir_all(&fixture.data).unwrap();
            let inside = fixture.data.join("live.mark");
            let candidate = match case {
                "absent" => inside,
                "existing" => {
                    fs::write(&inside, "0\n").unwrap();
                    inside
                }
                "parent_alias" => {
                    let alias = fixture.root.join("alias");
                    symlink(&fixture.data, &alias).unwrap();
                    alias.join("live.mark")
                }
                "file_alias" | "dangling_alias" => {
                    if case == "file_alias" {
                        fs::write(&inside, "0\n").unwrap();
                    }
                    let alias = fixture.root.join("alias.mark");
                    symlink(&inside, &alias).unwrap();
                    alias
                }
                "reverse_file_alias" => {
                    let external = fixture.root.join("external.mark");
                    fs::write(&external, "0\n").unwrap();
                    symlink(&external, &inside).unwrap();
                    inside
                }
                "data_alias" => {
                    let alias = fixture.root.join("data-alias");
                    symlink(&fixture.data, &alias).unwrap();
                    fixture.data = alias;
                    inside
                }
                "unresolvable" => fixture.root.join("missing-parent/live.mark"),
                _ => unreachable!(),
            };
            let role = if revocation {
                fixture.revocation = candidate;
                "revocation"
            } else {
                fixture.retention = candidate;
                "retention"
            };
            fixture.refused(1, &format!("{role} mark inside the data dir"));
            assert!(
                !fixture.data.join("state.json").exists(),
                "validation must precede recovery"
            );
        }
    }
}

#[test]
fn external_markers_allow_new_store_and_resolved_external_aliases() {
    let mut fixture = Fixture::new();
    assert!(!fixture.data.exists());
    drop(fixture.running());
    fs::create_dir(fixture.root.join("external")).unwrap();
    symlink(
        fixture.root.join("external"),
        fixture.root.join("external-alias"),
    )
    .unwrap();
    fixture.revocation = fixture.root.join("external-alias/revocation.mark");
    fixture.retention = fixture.root.join("external-alias/retention.mark");
    drop(fixture.running());
    fs::write(&fixture.revocation, "0\n").unwrap();
    fs::write(&fixture.retention, "0\n").unwrap();
    symlink(
        &fixture.revocation,
        fixture.root.join("revocation-alias.mark"),
    )
    .unwrap();
    symlink(
        &fixture.retention,
        fixture.root.join("retention-alias.mark"),
    )
    .unwrap();
    fixture.revocation = fixture.root.join("revocation-alias.mark");
    fixture.retention = fixture.root.join("retention-alias.mark");
    let data_alias = fixture.root.join("data-alias");
    symlink(&fixture.data, &data_alias).unwrap();
    fixture.data = data_alias;
    drop(fixture.running());
}

fn revocation_recovery(publisher: bool) {
    let fixture = Fixture::new();
    let service = fixture.running();
    let root = publish(&service);
    let grant = grant(&service, &root);
    assert_readable(&service, &access(&root, &grant, 4));
    place_hold(&service, &root);
    drop(service);
    copy_dir(&fixture.data, &fixture.root.join("backup"));
    let service = fixture.running();
    let retention_before = fs::read(&fixture.retention).unwrap();
    let (route, request) = if publisher {
        (
            "/v1/publishers/revoke",
            json!({"request_id": request_id(5),
            "principal": hex::encode(PRINCIPAL), "generation": 1}),
        )
    } else {
        (
            "/v1/revoke",
            json!({"request_id": request_id(5), "grant_id": grant["grant_id"],
            "generation": grant["generation"], "sequence": 1}),
        )
    };
    let (status, revoked) = post(&service, route, OWNER_TOKEN, &request);
    assert_eq!(status, 200, "{revoked}");
    assert_eq!(revoked["revoked"], true);
    assert_revoked(&service, &access(&root, &grant, 6));
    drop(service);
    assert_eq!(fs::read(&fixture.revocation).unwrap(), b"1\n");
    assert_eq!(fs::read(&fixture.retention).unwrap(), retention_before);
    let service = fixture.running();
    assert_revoked(&service, &access(&root, &grant, 7));
    drop(service);
    fs::rename(&fixture.data, fixture.root.join("current")).unwrap();
    copy_dir(&fixture.root.join("backup"), &fixture.data);
    let stale: Value =
        serde_json::from_slice(&fs::read(fixture.data.join("state.json")).unwrap()).unwrap();
    assert_eq!(stale["revocation_mark"], 0);
    assert_eq!(
        stale["grants"][grant["grant_id"].as_str().unwrap()]["revoked"],
        false
    );
    assert!(stale["publisher_revocations"]
        .as_object()
        .unwrap()
        .is_empty());
    fixture.refused(3, "revocation mark 0 below live mark 1");
    assert_eq!(fs::read(&fixture.revocation).unwrap(), b"1\n");
    assert_eq!(fs::read(&fixture.retention).unwrap(), retention_before);
    fs::remove_dir_all(&fixture.data).unwrap();
    fs::rename(fixture.root.join("current"), &fixture.data).unwrap();
    let service = fixture.running();
    assert_revoked(&service, &access(&root, &grant, 8));
}

#[test]
fn grant_revocation_survives_restart_and_refuses_old_backup() {
    revocation_recovery(false);
}

#[test]
fn publisher_revocation_survives_restart_and_refuses_old_backup() {
    revocation_recovery(true);
}

#[test]
fn retention_barrier_independently_refuses_old_backup() {
    let fixture = Fixture::new();
    let service = fixture.running();
    let root = publish(&service);
    drop(service);
    copy_dir(&fixture.data, &fixture.root.join("backup"));
    let service = fixture.running();
    place_hold(&service, &root);
    drop(service);
    let retention_live = fs::read(&fixture.retention).unwrap();
    assert!(!fixture.revocation.exists());
    fs::rename(&fixture.data, fixture.root.join("current")).unwrap();
    copy_dir(&fixture.root.join("backup"), &fixture.data);
    fixture.refused(3, "retention ledger below its live mark");
    assert_eq!(fs::read(&fixture.retention).unwrap(), retention_live);
    assert!(!fixture.revocation.exists());
    fs::remove_dir_all(&fixture.data).unwrap();
    fs::rename(fixture.root.join("current"), &fixture.data).unwrap();
    drop(fixture.running());
}

fn place_hold(service: &Service, root: &str) {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs();
    let (status, held) = post(
        service,
        "/v1/retention/hold",
        OWNER_TOKEN,
        &json!({
            "request_id": request_id(9), "root": root, "hold": request_id(10), "until": now + 3600
        }),
    );
    assert_eq!(status, 200, "{held}");
}

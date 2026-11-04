//! Task 22.33.1470: mandatory, self-contained registry integration proofs.
//! Peers use the committed node transaction/result corpus from 22.31.256;
//! deployment records are also produced by the real Programs Lifecycle.
//! The registry parser, clients, verifier, journal and Registrar are production code.

#[allow(dead_code)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/chain.rs"]
mod chain;

use chain::ChainFixture;
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::{
    chain_evidence::{
        deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
        program_state::ChainProgramStateProof, programs_state_key, wind_down::ChainWindDownProof,
    },
    chain_proof::{
        decode::decode_commitment_proof,
        merkle::{hash_from_byte_slices, SimpleProof},
        results::TxResult,
        validators::TrustedValidator,
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
    },
    interface_state_key, interface_state_value, DeploymentExecutionProof, DeploymentProof,
    DeploymentRecord, ProgramId, ProgramInterface, ProgramLifecycleProof, ProgramStateProof,
    ProtocolDeploymentVerifier, Registry, UpgradePolicy,
};
use layerx_programs_runtime::{Deploy, Lifecycle, ProgramVersion, Upgrade};
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    fs,
    path::PathBuf,
    sync::atomic::{AtomicU64, Ordering},
};

const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const CHAIN: &str = "repair-22-31-257";
fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
pub(crate) fn program() -> ProgramId {
    ProgramId::new([0x31; 32]).unwrap()
}
fn policy() -> UpgradePolicy {
    UpgradePolicy::Authority([0x51; 32])
}
fn transcript() -> BTreeMap<String, Vec<u8>> {
    let text = include_str!(
        "../../../../../programs/crates/layerx-programs-registry/src/repair_22_31_256.hex"
    );
    assert_eq!(
        layerx_programs::hex::encode(&hash(text.as_bytes())),
        "5798c9304ce5dd774f153da4f4f3208f1205f4cbfd1854732a49636dc9d76c6e"
    );
    text.lines()
        .map(|line| {
            let (key, value) = line.split_once('=').unwrap();
            (key.into(), layerx_programs::hex::decode(value).unwrap())
        })
        .collect()
}
fn descriptor(version: usize) -> ProgramInterface {
    ProgramInterface::decode(&transcript()[&format!("interface.{version}")]).unwrap()
}
pub(crate) fn code(version: usize) -> Vec<u8> {
    transcript()[&format!("wasm.{version}")].clone()
}
fn program_key() -> Vec<u8> {
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program().bytes());
    programs_state_key(&key)
}
fn interface_key() -> Vec<u8> {
    programs_state_key(&interface_state_key(program()))
}
fn records(abi: u16) -> [DeploymentRecord; 2] {
    let mut runtime = Lifecycle::declared().unwrap();
    let first = runtime
        .deploy(Deploy {
            program: program(),
            code_hash: hash(&code(1)),
            wasm: code(1),
            abi_version: abi,
            upgrade_policy: policy(),
        })
        .unwrap();
    let second = runtime
        .upgrade(Upgrade {
            program: program(),
            authority: [0x51; 32],
            code_hash: hash(&code(2)),
            wasm: code(2),
            abi_version: abi,
            migration: None,
        })
        .unwrap();
    [first, second]
        .iter()
        .enumerate()
        .map(|(i, receipt)| {
            let wasm = code(i + 1);
            DeploymentRecord::from_deployment(
                receipt,
                &ProgramVersion {
                    code_hash: hash(&wasm),
                    wasm,
                    abi_version: abi,
                },
                policy(),
                39 + i as u64,
                1_700_000_001_123 + i as u64 * 1000,
            )
            .unwrap()
        })
        .collect::<Vec<_>>()
        .try_into()
        .unwrap()
}
fn program_value(version: usize, abi: u16) -> Vec<u8> {
    record_value(&records(abi)[version - 1])
}
fn record_value(record: &DeploymentRecord) -> Vec<u8> {
    let mut value = vec![1];
    value.extend_from_slice(&[0x51; 32]);
    value.extend_from_slice(&record.new_code_hash);
    value.extend_from_slice(&record.abi_version.to_be_bytes());
    value.extend_from_slice(&record.version.to_be_bytes());
    value
}
fn state_fixture(
    version: usize,
    abi: u16,
    interface: Option<Vec<u8>>,
) -> (ChainFixture, ChainDeploymentProof) {
    let mut rows = vec![(program_key(), program_value(version, abi))];
    if let Some(value) = interface {
        rows.push((interface_key(), value));
    }
    let chain = ChainFixture::new(CHAIN, &SEEDS, rows).unwrap();
    let witness = decode_commitment_proof(&chain.encode_key(&interface_key()).unwrap()).unwrap();
    let deployment = ChainDeploymentProof::from_state_proofs(
        ChainProgramStateProof {
            program: program(),
            ops: chain.prove(&program_key()).unwrap(),
        },
        code(version),
        &witness,
    )
    .unwrap();
    (chain, deployment)
}
fn value(version: usize) -> Vec<u8> {
    interface_state_value(program(), version as u32, &descriptor(version)).unwrap()
}
fn sign(fixture: &mut ChainFixture) {
    fixture.commit.height = fixture.header.height;
    fixture.commit.block_id.hash = fixture.header.hash().unwrap().to_vec();
    let timestamp = Timestamp {
        seconds: fixture.header.time.seconds,
        nanos: fixture.header.time.nanos,
    };
    let bytes = vote_sign_bytes(
        &fixture.header.chain_id,
        fixture.commit.height,
        fixture.commit.round,
        Some(&fixture.commit.block_id),
        timestamp,
    )
    .unwrap();
    let mut keys = SEEDS.iter().map(SigningKey::from_bytes).collect::<Vec<_>>();
    keys.sort_by_key(|key| TrustedValidator::new(key.verifying_key().to_bytes(), 1).address);
    fixture.commit.signatures = keys
        .into_iter()
        .map(|key| CommitSig {
            flag: BlockIdFlag::Commit,
            validator_address: TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                .address
                .to_vec(),
            timestamp,
            signature: key.sign(&bytes).to_bytes().to_vec(),
        })
        .collect();
}

pub(crate) struct Admission {
    pub(crate) chain: ChainFixture,
    pub(crate) proof: DeploymentProof,
    pub(crate) verifier: ProtocolDeploymentVerifier,
    pub(crate) now: u64,
}
pub(crate) fn admission(version: usize) -> Admission {
    admission_abi(version, 1)
}
fn admission_abi(version: usize, abi: u16) -> Admission {
    let producer = transcript();
    let abi_five: Value =
        serde_json::from_str(include_str!("../src/repair_22_31_206_abi5.json")).unwrap();
    let five_tx = hex::decode(abi_five["transaction_hex"].as_str().unwrap()).unwrap();
    let transaction = if abi == 5 {
        &five_tx
    } else {
        &producer[if version == 1 { "deploy" } else { "upgrade" }]
    };
    let mut prior = if version == 1 {
        let mut prior =
            ChainFixture::new(CHAIN, &SEEDS, vec![(b"unrelated".to_vec(), vec![1])]).unwrap();
        prior.header.height = 38;
        prior
    } else {
        admission(1).chain
    };
    let tx_root = hash_from_byte_slices(&[&hash(transaction)]);
    prior.header.data_hash = tx_root.to_vec();
    sign(&mut prior);
    let (mut chain, _) = state_fixture(version, abi, (abi == 1).then(|| value(version)));
    chain.header.height = prior.header.height + 1;
    chain.header.time.seconds = prior.header.time.seconds + 1;
    chain.header.last_block_id.hash = prior.commit.block_id.hash.clone();
    chain.header.last_block_id.part_set_total = prior.commit.block_id.part_set_header.total;
    chain.header.last_block_id.part_set_hash = prior.commit.block_id.part_set_header.hash.clone();
    let result_root = hash_from_byte_slices(&[&producer["success.leaf"]]);
    assert_eq!(result_root.as_slice(), producer["success.root"]);
    chain.header.last_results_hash = result_root.to_vec();
    chain.header.data_hash = hash_from_byte_slices(&[&hash(
        &producer[if version == 1 && abi == 1 {
            "upgrade"
        } else {
            "unrelated"
        }],
    )])
    .to_vec();
    sign(&mut chain);
    let commitment = decode_commitment_proof(&chain.encode_key(&program_key()).unwrap()).unwrap();
    let interface_state =
        decode_commitment_proof(&chain.encode_key(&interface_key()).unwrap()).unwrap();
    let state = ChainProgramStateProof {
        program: program(),
        ops: chain.prove(&program_key()).unwrap(),
    };
    let mut status_key = b"wind-down\0s".to_vec();
    status_key.extend_from_slice(&program().bytes());
    let status_ops = chain
        .prove_absent(&programs_state_key(&status_key))
        .unwrap();
    let deployment =
        ChainDeploymentProof::from_state_proofs(state.clone(), code(version), &interface_state)
            .unwrap();
    let mut proof = DeploymentProof {
        state: ProgramStateProof {
            commitment,
            state: state.clone(),
            lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                program: program(),
                record: Some(program_value(version, abi)),
                record_ops: state.ops,
                status: None,
                status_ops: status_ops.clone(),
            }),
            wind_down: ChainWindDownProof {
                program: program(),
                value: None,
                ops: status_ops,
            },
            transaction: vec![],
            transaction_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: tx_root,
                aunts: vec![],
            },
        },
        deployment,
    };
    proof
        .bind_execution(&DeploymentExecutionProof {
            transaction: transaction.clone(),
            previous: decode_commitment_proof(&prior.encode_key(&program_key()).unwrap()).unwrap(),
            result: TxResult {
                code: 0,
                data: producer["success"].clone(),
                gas_wanted: 0,
                gas_used: 0,
            },
            result_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: result_root,
                aunts: vec![],
            },
            interface_ops: if abi == 1 {
                chain.prove(&interface_key()).unwrap()
            } else {
                chain.prove_absent(&interface_key()).unwrap()
            },
            migration: None,
            migration_ops: vec![],
            migration_previous_ops: vec![],
        })
        .unwrap();
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        CHAIN,
        chain.verifier.validators.validators().to_vec(),
        5000,
    )
    .unwrap();
    let now = chain.header.time.seconds as u64 * 1000 + chain.header.time.nanos as u64 / 1_000_000;
    Admission {
        chain,
        proof,
        verifier,
        now,
    }
}

use layerx_platform_registry::{
    committed_head_route, parse_request, write_response, Config, FileDeploymentJournal,
    HermeticBuilder, NodeProgramStateSource, Registrar, RegistryAuthority, Request, Response,
};
use layerx_programs::{hex, DeploymentJournal as _};
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    io::{BufRead, BufReader, Read},
    net::TcpListener,
    os::unix::fs::PermissionsExt,
    sync::{atomic::AtomicBool, Arc},
    thread,
    time::{Duration, Instant},
};
const TOKEN: &str = "registry-repair-request-credential";
const REPLICA: [u8; 32] = [0x91; 32];

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1470-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap();
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn tls() -> (Vec<u8>, Arc<rustls::ServerConfig>) {
    use openssl::{
        asn1::Asn1Time,
        bn::BigNum,
        hash::MessageDigest,
        pkey::PKey,
        rsa::Rsa,
        x509::{
            extension::{BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName},
            X509NameBuilder, X509,
        },
    };
    let ca_key = PKey::from_rsa(Rsa::generate(2048).unwrap()).unwrap();
    let mut ca_name = X509NameBuilder::new().unwrap();
    ca_name
        .append_entry_by_text("CN", "Registry repair test CA")
        .unwrap();
    let ca_name = ca_name.build();
    let mut ca = X509::builder().unwrap();
    ca.set_version(2).unwrap();
    ca.set_serial_number(&BigNum::from_u32(1).unwrap().to_asn1_integer().unwrap())
        .unwrap();
    ca.set_subject_name(&ca_name).unwrap();
    ca.set_issuer_name(&ca_name).unwrap();
    ca.set_pubkey(&ca_key).unwrap();
    ca.set_not_before(&Asn1Time::days_from_now(0).unwrap())
        .unwrap();
    ca.set_not_after(&Asn1Time::days_from_now(1).unwrap())
        .unwrap();
    ca.append_extension(BasicConstraints::new().critical().ca().build().unwrap())
        .unwrap();
    ca.append_extension(KeyUsage::new().critical().key_cert_sign().build().unwrap())
        .unwrap();
    ca.sign(&ca_key, MessageDigest::sha256()).unwrap();
    let ca = ca.build();
    let key = PKey::from_rsa(Rsa::generate(2048).unwrap()).unwrap();
    let mut name = X509NameBuilder::new().unwrap();
    name.append_entry_by_text("CN", "localhost").unwrap();
    let name = name.build();
    let mut cert = X509::builder().unwrap();
    cert.set_version(2).unwrap();
    cert.set_serial_number(&BigNum::from_u32(2).unwrap().to_asn1_integer().unwrap())
        .unwrap();
    cert.set_subject_name(&name).unwrap();
    cert.set_issuer_name(&ca_name).unwrap();
    cert.set_pubkey(&key).unwrap();
    cert.set_not_before(&Asn1Time::days_from_now(0).unwrap())
        .unwrap();
    cert.set_not_after(&Asn1Time::days_from_now(1).unwrap())
        .unwrap();
    cert.append_extension(BasicConstraints::new().critical().build().unwrap())
        .unwrap();
    cert.append_extension(
        KeyUsage::new()
            .digital_signature()
            .key_encipherment()
            .build()
            .unwrap(),
    )
    .unwrap();
    cert.append_extension(ExtendedKeyUsage::new().server_auth().build().unwrap())
        .unwrap();
    cert.append_extension(
        SubjectAlternativeName::new()
            .dns("localhost")
            .ip("127.0.0.1")
            .build(&cert.x509v3_context(Some(&ca), None))
            .unwrap(),
    )
    .unwrap();
    cert.sign(&ca_key, MessageDigest::sha256()).unwrap();
    let der = cert.build().to_der().unwrap();
    let config = rustls::ServerConfig::builder()
        .with_no_client_auth()
        .with_single_cert(
            vec![rustls::pki_types::CertificateDer::from(der.clone())],
            rustls::pki_types::PrivateKeyDer::Pkcs8(key.private_key_to_pkcs8().unwrap().into()),
        )
        .unwrap();
    (ca.to_der().unwrap(), Arc::new(config))
}

pub(crate) struct Server {
    pub(crate) endpoint: String,
    stop: Arc<AtomicBool>,
    thread: Option<thread::JoinHandle<()>>,
}

// The registry parser normalizes away queries. Peer authority and node routes
// need the original target to serve pinned historical heights.
fn peer_request(stream: &mut impl Read) -> Result<Request, String> {
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    reader
        .read_line(&mut line)
        .map_err(|error| error.to_string())?;
    let target = line
        .split_whitespace()
        .nth(1)
        .ok_or("missing request target")?
        .to_owned();
    let mut request = parse_request(&mut line.as_bytes().chain(reader))?;
    request.path = target;
    Ok(request)
}

impl Server {
    fn start(
        tls: Option<Arc<rustls::ServerConfig>>,
        handler: impl Fn(Request) -> Response + Send + Sync + 'static,
    ) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let endpoint = format!(
            "{}://{}",
            if tls.is_some() { "https" } else { "http" },
            listener.local_addr().unwrap()
        );
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let thread = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (socket, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                socket
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                socket
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                if let Some(tls) = &tls {
                    let connection = rustls::ServerConnection::new(tls.clone()).unwrap();
                    let mut stream = rustls::StreamOwned::new(connection, socket);
                    if let Ok(request) = peer_request(&mut stream) {
                        write_response(&mut stream, &handler(request)).unwrap();
                    }
                } else {
                    let mut stream = socket;
                    let request = peer_request(&mut stream).unwrap();
                    write_response(&mut stream, &handler(request)).unwrap();
                }
            }
        });
        Self {
            endpoint,
            stop,
            thread: Some(thread),
        }
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.thread.take().unwrap().join().unwrap();
    }
}
fn response(status: u16, body: Value) -> Response {
    Response {
        status,
        body: body.to_string(),
    }
}

fn head_document(admission: &Admission) -> Value {
    let raw = transcript()[if admission.proof.claimed_record().unwrap().version == 1
        && admission.proof.claimed_record().unwrap().abi_version == 1
    {
        "upgrade"
    } else {
        "unrelated"
    }]
    .clone();
    let root = hash_from_byte_slices(&[&hash(&raw)]);
    let header = &admission.chain.header;
    json!({"current": true, "authority_replica_id": hex::encode(&REPLICA),
        "receipt_digest": hex::encode(&hash(&raw)), "state_root": hex::encode(&header.app_hash),
        "observed_sequence": header.height, "observed_at": admission.now,
        "chain_evidence": {"commitment_hex": hex::encode(&admission.proof.state.commitment.encode().unwrap()),
            "transaction_hex": hex::encode(&raw), "transaction_proof": {"total":1,"index":0,"leaf_hash":hex::encode(&root),"aunts":[]}}})
}

struct Peers {
    node: Server,
    authority: Server,
    _replica: Server,
    rpc: Server,
    ca: Vec<u8>,
    tls: Arc<rustls::ServerConfig>,
}
impl Peers {
    fn new(admission: &Admission) -> Self {
        Self::with_history(admission, None)
    }
    fn with_history(admission: &Admission, historical: Option<&Admission>) -> Self {
        let (ca, tls) = tls();
        let document = head_document(admission);
        let mut heads = BTreeMap::from([(admission.chain.header.height as u64, document.clone())]);
        if let Some(historical) = historical {
            heads.insert(
                historical.chain.header.height as u64,
                head_document(historical),
            );
        }
        let replica_heads = heads.clone();
        let replica_document = document.clone();
        let replica = Server::start(None, move |request| {
            assert_eq!(
                request.headers.get("authorization").map(String::as_str),
                Some("Bearer replica-repair-credential")
            );
            assert!(request.path.starts_with(committed_head_route::HEAD_PATH));
            let height = request
                .path
                .split_once("?height=")
                .and_then(|(_, height)| height.parse::<u64>().ok());
            response(
                200,
                height
                    .and_then(|height| replica_heads.get(&height))
                    .unwrap_or(&replica_document)
                    .clone(),
            )
        });
        let replica_endpoint = replica.endpoint.clone();
        let authority = Server::start(Some(tls.clone()), move |request| {
            let (path, query) = request
                .path
                .split_once('?')
                .map_or((request.path.as_str(), None), |(path, query)| {
                    (path, Some(query))
                });
            let routed = committed_head_route::route(
                &request.method,
                path,
                query,
                || {
                    request
                        .headers
                        .get("authorization")
                        .is_some_and(|value| value == "Bearer authority-repair-credential")
                },
                REPLICA,
                |path| {
                    let mut answer = ureq::get(format!("{replica_endpoint}{path}"))
                        .header("authorization", "Bearer replica-repair-credential")
                        .call()
                        .map_err(|error| error.to_string())?;
                    answer
                        .body_mut()
                        .read_json::<Value>()
                        .map_err(|error| error.to_string())
                },
            );
            let (status, body) = routed.unwrap_or((404, json!({"error":"not_found"})));
            response(status, body)
        });
        let bundle = layerx_programs::ProgramStateBundle {
            commitment: admission.proof.state.commitment.clone(),
            program: program(),
            program_record_ops: admission.proof.state.state.ops.clone(),
            interface_ops: decode_commitment_proof(
                admission.proof.deployment.interface.as_deref().unwrap(),
            )
            .unwrap()
            .proof_ops,
            wind_down: Some(admission.proof.state.wind_down.clone()),
        };
        let encoded = bundle.canonical_encoding().unwrap();
        let rpc = Server::start(None, move |request| {
            assert_eq!(request.path, "/");
            assert_eq!(request.method, "POST");
            let rpc: Value = serde_json::from_slice(&request.body).unwrap();
            assert_eq!(rpc["method"], "eth_call");
            assert_eq!(rpc["params"][0]["to"], PROGRAMS);
            response(200, rpc_result(storage_head_abi()))
        });
        let head = document.clone();
        let node = Server::start(Some(tls.clone()), move |request| {
            assert_eq!(
                request.headers.get("authorization").map(String::as_str),
                Some("Bearer node-repair-credential")
            );
            if request.path == committed_head_route::HEAD_PATH {
                return response(200, head.clone());
            }
            if let Some(digest) = request
                .path
                .strip_prefix("/v1/receipts/")
                .and_then(|rest| rest.strip_suffix("/account-state"))
            {
                let historical = heads
                    .values()
                    .find(|head| head["receipt_digest"].as_str() == Some(digest))
                    .unwrap();
                return response(200, historical.clone());
            }
            if request
                .path
                .starts_with("/v1/programs/account-state/changes?")
            {
                return response(
                    200,
                    json!({"complete_through":{"sequence":head["observed_sequence"],"ordinal":0},"scanned_through_sequence":head["observed_sequence"],"caught_up":true,"records":[]}),
                );
            }
            if request.path.contains("/state-proof?at=") {
                return response(
                    200,
                    json!({"program_id":hex::encode(&program().bytes()),"record_hex":hex::encode(&encoded),"record_digest":hex::encode(&hash(&encoded)),"receipt_digest":head["receipt_digest"]}),
                );
            }
            panic!("unexpected node request {}", request.path)
        });
        Self {
            node,
            authority,
            _replica: replica,
            rpc,
            ca,
            tls,
        }
    }
    fn source(&self, admission: &Admission) -> NodeProgramStateSource {
        NodeProgramStateSource::connect(
            &self.node.endpoint,
            "node-repair-credential".into(),
            &self.ca,
            &self.authority.endpoint,
            "authority-repair-credential".into(),
            REPLICA,
            admission.verifier.clone(),
        )
        .unwrap()
    }
}

fn configuration(root: &Directory, peers: &Peers, admission: &Admission) -> Config {
    let header = admission
        .chain
        .verifier
        .verify_head(&admission.chain.header, &admission.chain.commit)
        .unwrap();
    let trust = layerx_programs::chain_proof::trust_store::TrustRoot::from_head(
        &header,
        admission.chain.verifier.validators.clone(),
    )
    .unwrap();
    let trust_path = root.0.join("trust");
    fs::write(&trust_path, trust.canonical_bytes().unwrap()).unwrap();
    fs::set_permissions(&trust_path, fs::Permissions::from_mode(0o600)).unwrap();
    Config {
        listen: "127.0.0.1:0".into(),
        journal: root.0.join("journal"),
        deployment_lni_socket: None,
        mirror: root.0.join("mirror"),
        verified: root.0.join("verified"),
        workspace: root.0.join("workspace"),
        builder_image_digest: [1; 32],
        builder_environment_root: root.0.join("environment"),
        builder_entrypoint: "/build".into(),
        builder_isolation_runtime: root.0.join("isolation"),
        builder_isolation_runtime_digest: [2; 32],
        builder_job_supervisor: root.0.join("supervisor"),
        builder_job_supervisor_digest: [3; 32],
        builder_cgroup_root: root.0.join("cgroup"),
        build_timeout_seconds: 30,
        build_memory_bytes: 67_108_864,
        build_process_limit: 8,
        build_file_size_bytes: 33_554_432,
        attempts: 2,
        staleness_ms: 5000,
        node_endpoint: peers.node.endpoint.clone(),
        node_authorization: "node-repair-credential".into(),
        outbound_ca_der: peers.ca.clone(),
        outbound_client_identity: None,
        receipt_authority_endpoint: peers.authority.endpoint.clone(),
        receipt_authority_authorization: "authority-repair-credential".into(),
        receipt_authority_replica_id: REPLICA,
        sequencer_trust_history: trust_path,
        request_authority: RegistryAuthority::new(TOKEN.to_owned().into()).unwrap(),
        publication_authority: RegistryAuthority::new(
            "operator-repair-credential".to_owned().into(),
        )
        .unwrap(),
        request_timeout_seconds: 10,
        max_connections: 8,
        max_builds: 1,
        tls: peers.tls.clone(),
    }
}

// Restore the production worker's serialized builder boundary. These tests
// exercise deployment and recorded-result readers; no compiler is invoked.
fn worker_builder(config: &Config) -> HermeticBuilder {
    serde_json::from_value(json!({
        "workspace":config.workspace,"builder_image_digest":config.builder_image_digest,
        "environment_root":config.builder_environment_root,"entrypoint":config.builder_entrypoint,
        "isolation_runtime":config.builder_isolation_runtime,"isolation_runtime_digest":config.builder_isolation_runtime_digest,
        "job_supervisor":config.builder_job_supervisor,"job_supervisor_digest":config.builder_job_supervisor_digest,
        "cgroup_root":config.builder_cgroup_root,"timeout_seconds":config.build_timeout_seconds,
        "memory_bytes":config.build_memory_bytes,"process_limit":config.build_process_limit,
        "file_size_bytes":config.build_file_size_bytes,"environment_metadata":vec![0;32],
    })).unwrap()
}
fn open(config: &Config, now: u64, peers: &Peers) -> Registrar {
    Registrar::open_with_builder_and_chain(
        config,
        now,
        worker_builder(config),
        true,
        &peers.rpc.endpoint,
    )
    .unwrap()
}
fn request(method: &str, path: &str, body: Vec<u8>) -> Request {
    Request {
        method: method.into(),
        path: path.into(),
        headers: HashMap::from([("authorization".into(), format!("Bearer {TOKEN}"))]),
        body,
    }
}
fn serve(registrar: Registrar, now: u64) -> Server {
    let registrar = std::sync::Mutex::new(registrar);
    Server::start(None, move |request| {
        registrar
            .lock()
            .unwrap()
            .route(&request, now, Instant::now() + Duration::from_secs(10))
    })
}
fn exchange(service: &Server, method: &str, path: &str, body: &[u8]) -> (u16, Value) {
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .http_status_as_error(false)
        .timeout_global(Some(Duration::from_secs(10)))
        .build()
        .into();
    let url = format!("{}{path}", service.endpoint);
    let mut answer = if method == "POST" {
        agent
            .post(&url)
            .header("authorization", format!("Bearer {TOKEN}"))
            .send(body)
            .unwrap()
    } else {
        agent
            .get(&url)
            .header("authorization", format!("Bearer {TOKEN}"))
            .call()
            .unwrap()
    };
    (
        answer.status().as_u16(),
        answer.body_mut().read_json().unwrap(),
    )
}

// The task record stays in scope. A baseline verify was not run: the ordered
// qualification rules permit it only after implementation and hf-build.
const TASK_RECORD: &[(&str, &str)] = &[
    (
        "upgrade-test-accepts-quarantine",
        "upgrade_deploy_reopen_preserves_authenticated_history",
    ),
    (
        "program-state-rpc-fixture",
        "program_state_root_transport_persists_committed_head",
    ),
    (
        "programs-rpc-fixture",
        "programs_root_transport_admits_and_refuses_transactions",
    ),
    (
        "startup-test-only-journal",
        "registry_http_without_socket_recovers_the_same_view",
    ),
];
// Wiring request: precompiles/layerxprograms and rpc (domain 15) own a recorded
// full-node deployment/receipt/getProgram/getStateHead corpus; this gate uses the
// committed transaction/result corpus and the producer's empty-storage golden.
// Wiring request: services/platform/hosted/registry/src/main.rs (domain 8) owns
// privileged cgroup delegation and binary startup; this gate opens its production
// Registrar worker with the serialized builder boundary, without a subprocess.

const PROGRAMS: &str = "0x000000000000000000000000000000000000101d";

pub(crate) fn storage_head_abi() -> Vec<u8> {
    // Empty-storage golden asserted against Host.ReadProgramStateHead and
    // EVM.StaticCall in precompiles/layerxprograms/proof_layerxprograms_0x101d_test.go
    // (getStateHead); replay verbatim rather than fabricate the manifest digest.
    hex::decode(concat!(
        "0000000000000000000000000000000000000000000000000000000000000020",
        "0000000000000000000000000000000000000000000000000000000000000026",
        "00010000000001cf85cfeb36d628ca2e6f583fa2331be029b6b28e877e1008fb3f862306c086",
        "0000000000000000000000000000000000000000000000000000",
    ))
    .unwrap()
}

fn rpc_result(encoded: Vec<u8>) -> Value {
    json!({"jsonrpc":"2.0","id":1,"result":format!("0x{}",hex::encode(&encoded))})
}

fn selector(signature: &str) -> String {
    let hash = layerx_crypto::evm_transaction::transaction_hash(signature.as_bytes());
    format!(
        "0x{}{}",
        hex::encode(&hash[..4]),
        hex::encode(&program().bytes())
    )
}

fn word(value: u64) -> Vec<u8> {
    let mut word = vec![0; 24];
    word.extend_from_slice(&value.to_be_bytes());
    word
}

pub(crate) fn program_abi(record: &DeploymentRecord) -> Vec<u8> {
    let (policy, authority) = match record.upgrade_policy {
        UpgradePolicy::Immutable => (0, [0; 32]),
        UpgradePolicy::Authority(authority) => (1, authority),
    };
    let mut encoded = word(policy);
    encoded.extend_from_slice(&authority);
    encoded.extend_from_slice(&record.new_code_hash);
    encoded.extend_from_slice(&word(u64::from(record.abi_version)));
    encoded.extend_from_slice(&word(u64::from(record.version)));
    encoded.extend_from_slice(&word(1));
    encoded
}

fn rpc_request(request: &Request) -> Value {
    assert_eq!(request.method, "POST");
    assert_eq!(request.path, "/");
    let rpc: Value = serde_json::from_slice(&request.body).unwrap();
    assert_eq!(rpc["jsonrpc"], "2.0");
    assert_eq!(rpc["id"], 1);
    rpc
}

pub(crate) fn state_rpc(height: u64) -> Server {
    Server::start(None, move |request| {
        let rpc = rpc_request(&request);
        assert_eq!(rpc["method"], "eth_call");
        assert_eq!(
            rpc["params"],
            json!([
                {"to":PROGRAMS,"data":selector("getStateHead(bytes32)")},
                format!("0x{height:x}")
            ])
        );
        response(200, rpc_result(storage_head_abi()))
    })
}

#[test]
pub(crate) fn upgrade_deploy_reopen_preserves_authenticated_history() {
    use layerx_platform_registry::{DeploymentEnvelope, UnitDefect, UnitPart};
    let first = admission(1);
    let second = admission(2);
    let deployed = first
        .verifier
        .verify_deployment(&first.proof, first.now)
        .unwrap();
    let upgraded = second
        .verifier
        .verify_deployment(&second.proof, second.now)
        .unwrap();
    assert_eq!(deployed.record(), &records(1)[0]);
    assert_eq!(upgraded.record(), &records(1)[1]);
    assert_eq!(upgraded.record().old_code_hash, Some(deployed.code_hash()));
    upgraded.record().validate().unwrap();
    let mut registry = Registry::new();
    assert_eq!(
        registry.record_verified_upgrade(&deployed, &upgraded),
        Err(layerx_programs::RegistryError::VersionHistoryMismatch)
    );
    registry.record_verified_deployment(&deployed).unwrap();
    registry
        .record_verified_upgrade(&deployed, &upgraded)
        .unwrap();
    assert_eq!(registry.latest_version(program()).unwrap(), 2);

    let root = Directory::new();
    let journal_path = root.0.join("journal");
    let journal = FileDeploymentJournal::open(journal_path.clone()).unwrap();
    journal.append(&deployed).unwrap();
    journal.append(&upgraded).unwrap();
    let before = journal.load().unwrap();
    assert_eq!(before.units.len(), 2);
    assert!(before.quarantined.is_empty());
    assert!(before.leftovers.is_empty());
    let proofs = journal.proofs().unwrap();
    drop(journal);
    let reopened = FileDeploymentJournal::open(journal_path.clone()).unwrap();
    let after = reopened.load().unwrap();
    assert_eq!(before.units, after.units);
    assert!(after.quarantined.is_empty());
    assert!(after.leftovers.is_empty());
    assert_eq!(reopened.proofs().unwrap(), proofs);
    let current = second
        .chain
        .verifier
        .verify_head(&second.chain.header, &second.chain.commit)
        .unwrap();
    let history = [
        (
            first.chain.header.clone(),
            first.chain.commit.clone(),
            first.chain.verifier.validators.clone(),
        ),
        (
            second.chain.header.clone(),
            second.chain.commit.clone(),
            second.chain.verifier.validators.clone(),
        ),
    ];
    let mut recovered = Registry::new();
    let mut verified = proofs
        .iter()
        .map(|proof| {
            let height = proof.claimed_record().unwrap().sequence;
            let start = history
                .iter()
                .position(|(header, _, _)| u64::try_from(header.height).unwrap() == height)
                .unwrap();
            second
                .verifier
                .verify_historical_deployment(proof, &current, &history[start..], second.now)
                .unwrap()
        })
        .collect::<Vec<_>>();
    verified.sort_by_key(|evidence| evidence.version());
    assert_eq!(verified, vec![deployed.clone(), upgraded.clone()]);
    for evidence in &verified {
        reopened.audit_projection(evidence).unwrap();
        assert_eq!(
            reopened
                .canonical_record(evidence.receipt_digest())
                .unwrap(),
            evidence.record().canonical_encoding()
        );
    }
    recovered.record_verified_deployment(&verified[0]).unwrap();
    recovered
        .record_verified_upgrade(&verified[0], &verified[1])
        .unwrap();
    assert_eq!(recovered.latest_version(program()).unwrap(), 2);
    assert_eq!(
        recovered
            .resolve_deployment(upgraded.clone())
            .unwrap()
            .record(),
        upgraded.record()
    );

    let mut absent = second.proof.clone();
    let mut execution = DeploymentExecutionProof::decode(&absent.state.transaction).unwrap();
    execution.previous.proof_ops.clear();
    absent.bind_execution(&execution).unwrap();
    assert!(second
        .verifier
        .verify_deployment(&absent, second.now)
        .is_err());
    let mut corrupt = second.proof.clone();
    let mut execution = DeploymentExecutionProof::decode(&corrupt.state.transaction).unwrap();
    execution.previous.proof_ops[0].data[0] ^= 1;
    corrupt.bind_execution(&execution).unwrap();
    assert!(second
        .verifier
        .verify_deployment(&corrupt, second.now)
        .is_err());

    let envelope = DeploymentEnvelope::from_evidence(&upgraded);
    let encoded = envelope.canonical_encoding().unwrap();
    let path = journal_path.join(format!(
        "{}.envelope",
        hex::encode(&upgraded.receipt_digest())
    ));
    let mut corrupt = encoded.clone();
    *corrupt.last_mut().unwrap() ^= 1;
    fs::write(&path, &corrupt).unwrap();
    let damaged = FileDeploymentJournal::open(journal_path.clone()).unwrap();
    let loaded = damaged.load().unwrap();
    assert_eq!(loaded.units.len(), 1);
    assert_eq!(loaded.quarantined.len(), 1);
    assert!(matches!(
        loaded.quarantined[0].defect,
        UnitDefect::Corrupt {
            part: UnitPart::Seal,
            ..
        }
    ));
    assert!(damaged.proofs().is_err());
    let (record_frame, _, _) = envelope.frames().unwrap();
    fs::write(&path, &record_frame).unwrap();
    let missing = FileDeploymentJournal::open(journal_path).unwrap();
    let loaded = missing.load().unwrap();
    assert_eq!(loaded.units.len(), 1);
    assert_eq!(loaded.quarantined.len(), 1);
    assert_eq!(
        loaded.quarantined[0].defect,
        UnitDefect::Missing(UnitPart::Proof)
    );
    assert!(missing.proofs().is_err());
}

#[test]
pub(crate) fn program_state_root_transport_persists_committed_head() {
    use layerx_platform_registry::{FileProgramStateJournal, ProgramStateCursor};
    let fixture = admission(1);
    let root = Directory::new();
    let peer = state_rpc(fixture.chain.header.height as u64);
    let journal = FileProgramStateJournal::open(root.0.join("state")).unwrap();
    let encoded = storage_head_abi();
    assert_eq!(encoded.len(), 128);
    let expected = encoded[64..102].to_vec();
    assert_eq!(&expected[6..], hash(&expected[..6]));
    let deadline = || Instant::now() + Duration::from_secs(5);
    let stored = journal
        .store(
            &format!("{}/", peer.endpoint),
            program(),
            fixture.chain.header.height as u64,
            deadline(),
        )
        .unwrap();
    assert_eq!(stored, expected);
    let digest = hash(&stored);
    let path = root.0.join("state").join(format!(
        "{}.{}.program-state",
        hex::encode(&program().bytes()),
        hex::encode(&digest)
    ));
    assert_eq!(fs::read(&path).unwrap(), expected);
    assert_eq!(journal.cursor().unwrap(), ProgramStateCursor::default());
    let cursor = ProgramStateCursor {
        sequence: fixture.chain.header.height as u64,
        ordinal: 0,
    };
    journal.advance(cursor).unwrap();
    journal.audit().unwrap();
    drop(journal);
    let reopened = FileProgramStateJournal::open(root.0.join("state")).unwrap();
    assert_eq!(reopened.cursor().unwrap(), cursor);
    reopened.audit().unwrap();
    assert_eq!(fs::read(&path).unwrap(), stored);
    let deployment_journal = FileDeploymentJournal::open(root.0.join("deployments")).unwrap();
    let head_peer = head_rpc(&fixture);
    let authority = RegistryAuthority::new(TOKEN.to_owned().into()).unwrap();
    let response = Registrar::route_chain_head(
        &request(
            "POST",
            "/__registry/head",
            serde_json::to_vec(&json!({"program_id":hex::encode(&program().bytes())})).unwrap(),
        ),
        &authority,
        &deployment_journal,
        &format!("{}/", head_peer.endpoint),
        deadline(),
    );
    assert_eq!(response.status, 200, "{}", response.body);
    let body: Value = serde_json::from_str(&response.body).unwrap();
    let keccak = layerx_crypto::evm_transaction::transaction_hash(&stored);
    assert_eq!(body["state_root"], hex::encode(&keccak));
    assert_eq!(body["sequence"], cursor.sequence);
    drop(deployment_journal);
    let recovered = FileDeploymentJournal::open(root.0.join("deployments")).unwrap();
    assert_eq!(recovered.observed_head().unwrap().sequence, cursor.sequence);
    drop(peer);
    assert!(reopened
        .store(
            &format!("{}/", head_peer.endpoint),
            program(),
            0,
            deadline()
        )
        .is_err());
    assert!(reopened
        .store(
            &format!("{}/", head_peer.endpoint),
            program(),
            cursor.sequence,
            Instant::now()
        )
        .is_err());
    assert!(reopened
        .store(
            "http://chain.example/",
            program(),
            cursor.sequence,
            deadline()
        )
        .is_err());
    for (index, value) in [
        (0, 1),
        (31, 64),
        (32, 1),
        (63, 37),
        (64, 1),
        (102, 1),
        (101, 0),
    ] {
        let mut invalid = encoded.clone();
        invalid[index] = value;
        assert!(
            reopened
                .store_response(
                    program(),
                    &serde_json::to_vec(&rpc_result(invalid)).unwrap()
                )
                .is_err(),
            "mutation at {index}"
        );
    }
    for invalid in [
        encoded[..127].to_vec(),
        [encoded.clone(), vec![0; 32]].concat(),
    ] {
        assert!(reopened
            .store_response(
                program(),
                &serde_json::to_vec(&rpc_result(invalid)).unwrap()
            )
            .is_err());
    }
    for invalid in [
        json!({"jsonrpc":"2.0","id":1,"error":{"code":3,"message":"execution reverted"}}),
        json!({"jsonrpc":"2.0","id":2,"result":rpc_result(encoded.clone())["result"]}),
        json!({"jsonrpc":"1.0","id":1,"result":"0x"}),
        json!({"jsonrpc":"2.0","id":1,"result":"0xzz"}),
        json!({"jsonrpc":"2.0","id":1,"result":null}),
    ] {
        assert!(reopened
            .store_response(program(), &serde_json::to_vec(&invalid).unwrap())
            .is_err());
    }
    assert!(reopened.store_response(program(), b"{").is_err());
    let unavailable = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
    let endpoint = format!("http://{}/", unavailable.local_addr().unwrap());
    let observer = thread::spawn(move || {
        let (mut socket, _) = unavailable.accept().unwrap();
        socket
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        let message = peer_request(&mut socket).unwrap();
        let rpc = rpc_request(&message);
        assert_eq!(rpc["params"][1], format!("0x{:x}", cursor.sequence));
    });
    assert!(reopened
        .store(&endpoint, program(), cursor.sequence, deadline())
        .is_err());
    observer.join().unwrap();
    assert_eq!(reopened.cursor().unwrap(), cursor);
    assert_eq!(fs::read(&path).unwrap(), stored);
    assert_eq!(fs::read_dir(root.0.join("state")).unwrap().count(), 2);
    fs::write(&path, b"corrupt").unwrap();
    assert!(reopened.audit().is_err());
}

fn head_rpc(fixture: &Admission) -> Server {
    let record = program_abi(&fixture.proof.claimed_record().unwrap());
    let height = fixture.chain.header.height as u64;
    let timestamp = fixture.chain.header.time.seconds;
    Server::start(None, move |request| {
        let rpc = rpc_request(&request);
        let result = match rpc["method"].as_str().unwrap() {
            "eth_blockNumber" => {
                assert_eq!(rpc["params"], json!([]));
                json!(format!("0x{height:x}"))
            }
            "eth_getBlockByNumber" => {
                assert_eq!(rpc["params"], json!([format!("0x{height:x}"), false]));
                json!({"number":format!("0x{height:x}"),"timestamp":format!("0x{timestamp:x}")})
            }
            "eth_call" => {
                assert_eq!(rpc["params"][0]["to"], PROGRAMS);
                assert_eq!(rpc["params"][1], format!("0x{height:x}"));
                let data = rpc["params"][0]["data"].as_str().unwrap();
                if data == selector("getProgram(bytes32)") {
                    rpc_result(record.clone())["result"].clone()
                } else {
                    assert_eq!(data, selector("getStateHead(bytes32)"));
                    rpc_result(storage_head_abi())["result"].clone()
                }
            }
            method => panic!("unexpected node method {method}"),
        };
        response(200, json!({"jsonrpc":"2.0","id":1,"result":result}))
    })
}

#[test]
pub(crate) fn programs_root_transport_admits_and_refuses_transactions() {
    let fixture = admission(1);
    let evidence = fixture
        .verifier
        .verify_deployment(&fixture.proof, fixture.now)
        .unwrap();
    let root = Directory::new();
    let journal = FileDeploymentJournal::open(root.0.join("journal")).unwrap();
    journal.append(&evidence).unwrap();
    let transaction = evidence.activity_id();
    let failed_transaction = hash(&transcript()["unrelated"]);
    let block_hash = fixture.chain.header.hash().unwrap();
    let block_number = fixture.chain.header.height as u64;
    let topic =
        layerx_crypto::evm_transaction::transaction_hash(b"ProgramDeployed(bytes32,bytes32)");
    let receipt = json!({
        "transactionHash":format!("0x{}",hex::encode(&transaction)),
        "blockHash":format!("0x{}",hex::encode(&block_hash)),"blockNumber":format!("0x{block_number:x}"),"status":"0x1",
        "logs":[{"address":PROGRAMS,"topics":[format!("0x{}",hex::encode(&topic)),format!("0x{}",hex::encode(&program().bytes()))],
        "data":format!("0x{}",hex::encode(&evidence.code_hash())),"transactionHash":format!("0x{}",hex::encode(&transaction)),
        "blockHash":format!("0x{}",hex::encode(&block_hash)),"blockNumber":format!("0x{block_number:x}"),"removed":false}]
    });
    let valid = receipt.clone();
    let encoded = program_abi(evidence.record());
    let counter = Arc::new(std::sync::Mutex::new(0_usize));
    let seen = counter.clone();
    let peer = Server::start(None, move |request| {
        let rpc = rpc_request(&request);
        *seen.lock().unwrap() += 1;
        let result = match rpc["method"].as_str().unwrap() {
            "eth_getTransactionReceipt" => {
                if rpc["params"] == json!([format!("0x{}", hex::encode(&failed_transaction))]) {
                    let mut refused = valid.clone();
                    refused["status"] = json!("0x0");
                    refused["logs"] = json!([]);
                    refused["transactionHash"] = rpc["params"][0].clone();
                    refused
                } else {
                    assert_eq!(
                        rpc["params"],
                        json!([format!("0x{}", hex::encode(&transaction))])
                    );
                    valid.clone()
                }
            }
            "eth_call" => {
                assert_eq!(
                    rpc["params"],
                    json!([{"to":PROGRAMS,"data":selector("getProgram(bytes32)")},{"blockHash":format!("0x{}",hex::encode(&block_hash)),"requireCanonical":true}])
                );
                rpc_result(encoded.clone())["result"].clone()
            }
            method => panic!("unexpected method {method}"),
        };
        response(200, json!({"jsonrpc":"2.0","id":1,"result":result}))
    });
    let authority = RegistryAuthority::new(TOKEN.to_owned().into()).unwrap();
    let deadline = || Instant::now() + Duration::from_secs(5);
    let mut req = request("POST", "/__registry/deployments", transaction.to_vec());
    let endpoint = format!("{}/", peer.endpoint);
    let route = |req: &Request| {
        Registrar::route_chain_deployment(
            req,
            &authority,
            &root.0.join("journal"),
            &endpoint,
            deadline(),
        )
    };
    req.headers.clear();
    assert_eq!(route(&req).status, 401);
    req.headers
        .insert("authorization".into(), format!("Bearer {TOKEN}"));
    for malformed in [
        Vec::new(),
        b"{\"transaction_hash\":\"0x01\"}".to_vec(),
        b"{\"transaction_hash\":null}".to_vec(),
        b"{\"transaction_hash\":\"bad\",\"extra\":1}".to_vec(),
    ] {
        req.body = malformed;
        assert_eq!(route(&req).status, 400);
    }
    assert_eq!(*counter.lock().unwrap(), 0);
    req.body = transaction.to_vec();
    let answer = route(&req);
    assert_eq!(answer.status, 200, "{}", answer.body);
    let body: Value = serde_json::from_str(&answer.body).unwrap();
    assert_eq!(body["program_id"], hex::encode(&program().bytes()));
    assert_eq!(body["code_hash"], hex::encode(&evidence.code_hash()));
    assert_eq!(body["abi_version"], evidence.abi_version());
    assert_eq!(body["version"], evidence.version());
    assert_eq!(body["policy"], 1);
    assert_eq!(body["authority"], hex::encode(&[0x51; 32]));
    assert_eq!(body["block_number"], block_number);
    let path = root
        .0
        .join("journal/chain-deployments")
        .join(format!("{}.json", hex::encode(&transaction)));
    let stored = fs::read(&path).unwrap();
    let metadata: Value = serde_json::from_slice(&stored).unwrap();
    for field in [
        "transaction_hash",
        "program_id",
        "code_hash",
        "abi_version",
        "version",
        "policy",
        "authority",
        "block_hash",
        "block_number",
    ] {
        assert_eq!(metadata[field], body[field], "{field}");
    }
    assert_eq!(metadata["active"], true);
    req.body =
        serde_json::to_vec(&json!({"transaction_hash":format!("0x{}",hex::encode(&transaction))}))
            .unwrap();
    assert_eq!(route(&req).status, 200);
    assert_eq!(fs::read(&path).unwrap(), stored);
    req.body = failed_transaction.to_vec();
    let refused = route(&req);
    assert_eq!(refused.status, 422, "{}", refused.body);
    assert_eq!(
        serde_json::from_str::<Value>(&refused.body).unwrap()["error"]["code"],
        "deployment_proof_refused"
    );
    assert!(!path
        .with_file_name(format!("{}.json", hex::encode(&failed_transaction)))
        .exists());
    assert_eq!(fs::read(&path).unwrap(), stored);
    drop(peer);
    for mutation in 0..5 {
        let mut bad = receipt.clone();
        match mutation {
            0 => bad = Value::Null,
            1 => bad["transactionHash"] = json!(format!("0x{}", hex::encode(&failed_transaction))),
            2 => bad["logs"][0]["address"] = json!("0x000000000000000000000000000000000000101c"),
            3 => bad["logs"][0]["removed"] = json!(true),
            4 => {}
            _ => unreachable!(),
        }
        let mut record = program_abi(evidence.record());
        if mutation == 4 {
            record[64] ^= 1;
        }
        let peer = Server::start(None, move |request| {
            let rpc = rpc_request(&request);
            let result = if rpc["method"] == "eth_getTransactionReceipt" {
                bad.clone()
            } else {
                assert_eq!(rpc["method"], "eth_call");
                rpc_result(record.clone())["result"].clone()
            };
            response(200, json!({"jsonrpc":"2.0","id":1,"result":result}))
        });
        req.body = transaction.to_vec();
        let refused = Registrar::route_chain_deployment(
            &req,
            &authority,
            &root.0.join("journal"),
            &format!("{}/", peer.endpoint),
            deadline(),
        );
        assert_eq!(refused.status, 422, "mutation {mutation}: {}", refused.body);
        assert_eq!(fs::read(&path).unwrap(), stored);
    }
    let head_peer = head_rpc(&fixture);
    let head_request = request(
        "POST",
        "/__registry/head",
        serde_json::to_vec(&json!({"program_id":hex::encode(&program().bytes())})).unwrap(),
    );
    let head = Registrar::route_chain_head(
        &head_request,
        &authority,
        &journal,
        &format!("{}/", head_peer.endpoint),
        deadline(),
    );
    assert_eq!(head.status, 200, "{}", head.body);
    let head: Value = serde_json::from_str(&head.body).unwrap();
    assert_eq!(head["sequence"], block_number);
    assert_eq!(
        head["state_root"],
        hex::encode(&layerx_crypto::evm_transaction::transaction_hash(
            &storage_head_abi()[64..102]
        ))
    );
    assert!(head.get("receipt_digest").is_none());
    drop(journal);
    let reopened = FileDeploymentJournal::open(root.0.join("journal")).unwrap();
    assert_eq!(reopened.observed_head().unwrap().sequence, block_number);
    assert_eq!(reopened.load().unwrap().units.len(), 1);
    assert_eq!(fs::read(&path).unwrap(), stored);
}

#[test]
pub(crate) fn registry_http_without_socket_recovers_the_same_view() {
    assert_eq!(TASK_RECORD.len(), 4);
    let admission = admission(1);
    let root = Directory::new();
    let peers = Peers::new(&admission);
    let config = configuration(&root, &peers, &admission);
    assert!(config.deployment_lni_socket.is_none());
    let path = format!("/v1/programs/registry/{}", hex::encode(&program().bytes()));
    let service = serve(open(&config, admission.now, &peers), admission.now);
    assert_eq!(exchange(&service, "GET", &path, &[]).0, 404);
    let proof = admission.proof.canonical_encoding().unwrap();
    let (status, deployed) = exchange(&service, "POST", "/__registry/deployments", &proof);
    assert_eq!(status, 200, "{deployed}");
    assert_eq!(deployed["version"], 1);
    assert_eq!(deployed["code_hash"], hex::encode(&hash(&code(1))));
    let (status, before) = exchange(&service, "GET", &path, &[]);
    assert_eq!(status, 200, "{before}");
    assert_eq!(before["versions"][0]["code_hash"], deployed["code_hash"]);
    assert_eq!(
        exchange(&service, "GET", "/v1/programs/registry", &[]),
        (
            200,
            json!({
                "program_ids":[hex::encode(&program().bytes())],
                "state_root":hex::encode(&admission.chain.header.app_hash),
                "observed_sequence":admission.chain.header.height,
                "observed_at":admission.now,"valid_through":admission.now+5000,
                "verification":"registry-receipt-and-current-head-verified",
            })
        )
    );
    let (status, refused) = exchange(
        &service,
        "POST",
        "/__registry/deployments",
        &transcript()["deploy"],
    );
    assert_eq!(status, 422, "{refused}");
    assert_eq!(
        exchange(&service, "POST", "/__registry/deployments", &proof).0,
        200
    );
    drop(service);
    let registrar = open(&config, admission.now, &peers);
    let service = serve(registrar, admission.now);
    let (status, after) = exchange(&service, "GET", &path, &[]);
    assert_eq!(status, 200, "{after}");
    assert_eq!(after, before);
    let loaded = FileDeploymentJournal::open(config.journal.clone())
        .unwrap()
        .load()
        .unwrap();
    assert_eq!(loaded.units.len(), 1);
    assert!(loaded.quarantined.is_empty());
    assert_eq!(loaded.units[0].proof(), &admission.proof);
    drop(service);
}

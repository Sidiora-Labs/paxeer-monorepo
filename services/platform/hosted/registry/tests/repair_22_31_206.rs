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
fn program() -> ProgramId {
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
fn code(version: usize) -> Vec<u8> {
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

struct Admission {
    chain: ChainFixture,
    proof: DeploymentProof,
    verifier: ProtocolDeploymentVerifier,
    now: u64,
}
fn admission(version: usize) -> Admission {
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
    SourceMirror, VerifiedSource, VerifiedSourceDeployment, VerifiedSourceStore,
};
use layerx_programs::{hex, BuildEnvironment, BuildPlan, ObservedHead, SourceArchive, SourceFile};
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
            "repair-22-31-206-{}-{}",
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

struct Server {
    endpoint: String,
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
            assert_eq!(request.method, "POST");
            let rpc: Value = serde_json::from_slice(&request.body).unwrap();
            assert_eq!(rpc["method"], "eth_call");
            let mut native_head = vec![0, 1, 0, 0, 0, 0];
            native_head.extend_from_slice(&hash(&native_head.clone()));
            let mut abi = vec![0; 64];
            abi[31] = 32;
            abi[63] = native_head.len() as u8;
            abi.extend_from_slice(&native_head);
            abi.resize(64 + native_head.len().div_ceil(32) * 32, 0);
            response(
                200,
                json!({"jsonrpc":"2.0","id":1,"result":format!("0x{}",hex::encode(&abi))}),
            )
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

#[test]
fn deployment_projection_post_get_source_and_restart() {
    let admission = admission(1);
    let root = Directory::new();
    let peers = Peers::new(&admission);
    let config = configuration(&root, &peers, &admission);
    let path = format!("/v1/programs/registry/{}", hex::encode(&program().bytes()));
    let service = serve(open(&config, admission.now, &peers), admission.now);
    assert_eq!(exchange(&service, "GET", &path, &[]).0, 404);
    let proof = admission.proof.canonical_encoding().unwrap();
    let (status, body) = exchange(&service, "POST", "/__registry/deployments", &proof);
    assert_eq!(status, 200, "{body}");
    assert_eq!(body["version"], 1);
    let (status, body) = exchange(&service, "GET", &path, &[]);
    assert_eq!(status, 200, "{body}");
    assert_eq!(
        body["versions"][0]["code_hash"],
        hex::encode(&hash(&code(1)))
    );
    let (status, listed) = exchange(&service, "GET", "/v1/programs/registry", &[]);
    assert_eq!(status, 200, "{listed}");
    assert_eq!(
        listed["program_ids"],
        json!([hex::encode(&program().bytes())])
    );
    assert_eq!(
        exchange(&service, "POST", "/__registry/deployments", &proof).0,
        200
    );
    let (status, refused) = exchange(
        &service,
        "POST",
        "/__registry/deployments",
        &transcript()["deploy"],
    );
    assert_eq!(status, 422, "{refused}");
    drop(service);
    let mut recovered = open(&config, admission.now, &peers);
    let deployment = recovered
        .source_deployment(program(), admission.now)
        .unwrap();
    assert_eq!(deployment.code_hash(), hash(&code(1)));
    assert_eq!(deployment.proof(), &admission.proof);
    let service = serve(recovered, admission.now);
    let (status, body) = exchange(&service, "GET", &path, &[]);
    assert_eq!(status, 200, "{body}");
    assert_eq!(body["versions"].as_array().unwrap().len(), 1);
    let loaded = FileDeploymentJournal::open(config.journal)
        .unwrap()
        .load()
        .unwrap();
    assert_eq!(loaded.units.len(), 1);
    assert!(loaded.quarantined.is_empty());
}

#[test]
fn head_authority_tls_current_and_historical_replay() {
    let first = admission(1);
    let second = admission(2);
    let peers = Peers::with_history(&second, Some(&first));
    let source = peers.source(&second);
    let current = source.current_head(second.now).unwrap();
    assert_eq!(
        current.freshness.observed_sequence,
        second.chain.header.height as u64
    );
    assert_eq!(current.state_root.as_slice(), second.chain.header.app_hash);
    let historical_document = head_document(&first);
    let digest =
        hex::decode_digest(historical_document["receipt_digest"].as_str().unwrap()).unwrap();
    let historical = source.receipt_head(digest).unwrap();
    assert_eq!(
        historical.freshness.observed_sequence,
        first.chain.header.height as u64
    );
    assert!(historical.freshness.observed_sequence < current.freshness.observed_sequence);
    let recovered = source
        .verify_stored_deployment(&first.proof, second.now)
        .unwrap();
    assert_eq!(recovered.record().old_code_hash, None);
    assert_eq!(recovered.code_hash(), hash(&code(1)));
    let tls = ureq::tls::TlsConfig::builder()
        .provider(ureq::tls::TlsProvider::Rustls)
        .root_certs(ureq::tls::RootCerts::new_with_certs(&[
            ureq::tls::Certificate::from_der(&peers.ca).to_owned(),
        ]))
        .build();
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .http_status_as_error(false)
        .tls_config(tls)
        .build()
        .into();
    let endpoint = format!(
        "{}{}?height={}",
        peers.authority.endpoint,
        committed_head_route::HEAD_PATH,
        first.chain.header.height
    );
    assert_eq!(agent.get(&endpoint).call().unwrap().status(), 401);
    assert_eq!(
        agent
            .get(format!(
                "{}{}?height=01",
                peers.authority.endpoint,
                committed_head_route::HEAD_PATH
            ))
            .header("authorization", "Bearer authority-repair-credential")
            .call()
            .unwrap()
            .status(),
        400
    );
    assert_eq!(
        agent
            .get(format!(
                "{}{}?height=1",
                peers.authority.endpoint,
                committed_head_route::HEAD_PATH
            ))
            .header("authorization", "Bearer authority-repair-credential")
            .call()
            .unwrap()
            .status(),
        503
    );
    let mut forged = head_document(&second);
    forged["authority_replica_id"] = json!(hex::encode(&[0x99; 32]));
    let rejected = committed_head_route::route(
        "GET",
        committed_head_route::HEAD_PATH,
        None,
        || true,
        REPLICA,
        |_| Ok(forged),
    )
    .unwrap();
    assert_eq!(rejected.0, 503);
}

fn source_record(
    evidence: &layerx_programs::VerifiedDeploymentEvidence,
    config: &Config,
) -> VerifiedSource {
    let archive = SourceArchive::new(vec![
        SourceFile {
            path: "module.wasm".into(),
            content: evidence.module().to_vec(),
            executable: false,
        },
        SourceFile {
            path: "toolchain".into(),
            content: b"recorded-production-module".to_vec(),
            executable: false,
        },
        SourceFile {
            path: "dependencies.lock".into(),
            content: b"no-dependencies".to_vec(),
            executable: false,
        },
    ])
    .unwrap();
    let plan = BuildPlan {
        environment: BuildEnvironment {
            builder_image_digest: [1; 32],
            toolchain_digest: hash(b"recorded-production-module"),
            dependency_lock_digest: hash(b"no-dependencies"),
            source_date_epoch: 1,
            command: vec!["cp".into(), "module.wasm".into(), "artifact.wasm".into()],
        },
        artifact_path: "artifact.wasm".into(),
        toolchain_manifest: "toolchain".into(),
        dependency_lock: "dependencies.lock".into(),
    };
    let mirror = SourceMirror::open(config.mirror.clone()).unwrap();
    let uri = "ipfs://recorded-program-module";
    let digest = mirror.publish(uri, &plan, &archive).unwrap();
    let published = mirror.fetch(uri, digest).unwrap();
    let build = layerx_programs::ReproducibleBuild::from_output(
        &published.source,
        plan.environment.clone(),
        evidence.module(),
    )
    .unwrap();
    assert_eq!(build.artifact_digest, evidence.code_hash());
    let header = layerx_programs::chain_proof::header::ChainHeader::decode(
        &evidence.proof().state.commitment.signed_header,
    )
    .unwrap();
    VerifiedSource {
        program: evidence.program(),
        version: evidence.version(),
        source_uri: build.source_uri,
        source_digest: build.source_digest,
        artifact_digest: build.artifact_digest,
        plan,
        deployment_provenance: VerifiedSourceDeployment {
            abi_version: evidence.abi_version(),
            code_hash: evidence.code_hash(),
            activity_id: evidence.activity_id(),
            receipt_digest: evidence.receipt_digest(),
            batch_header_digest: header.hash().unwrap(),
            state_root: evidence.state_root(),
            programs_root: evidence.programs_root(),
            observed_sequence: evidence.freshness().observed_sequence,
            observed_at: evidence.freshness().observed_at,
        },
    }
}

#[test]
fn source_route_and_store_abi_five_publication_and_reopen() {
    let admission = admission_abi(1, 5);
    let root = Directory::new();
    let peers = Peers::new(&admission);
    let config = configuration(&root, &peers, &admission);
    let producer: Value =
        serde_json::from_str(include_str!("../src/repair_22_31_206_abi5.json")).unwrap();
    let transaction = DeploymentExecutionProof::decode(&admission.proof.state.transaction)
        .unwrap()
        .transaction;
    assert_eq!(
        hex::encode(&hash(&transaction)),
        producer["transaction_digest"]
    );
    assert_eq!(hex::encode(&code(1)), producer["wasm_hex"]);
    let mut registrar = open(&config, admission.now, &peers);
    let post = request(
        "POST",
        "/__registry/deployments",
        serde_json::to_vec(&json!({"admission":admission.proof.canonical_encoding().unwrap()}))
            .unwrap(),
    );
    let accepted = registrar.route(
        &post,
        admission.now,
        Instant::now() + Duration::from_secs(10),
    );
    assert_eq!(accepted.status, 200, "{}", accepted.body);
    let deployment = registrar
        .source_deployment(program(), admission.now)
        .unwrap();
    assert_eq!(deployment.abi_version(), 5);
    assert_eq!(deployment.code_hash(), hash(&code(1)));
    assert!(deployment.interface().is_none());
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
    let transaction = transcript()["unrelated"].clone();
    let chain = admission
        .verifier
        .verify_current_chain_head(
            layerx_programs::ProgramHeadKind::Ordinary,
            &bundle.commitment,
            &transaction,
            &SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: hash_from_byte_slices(&[&hash(&transaction)]),
                aunts: vec![],
            },
            admission.now,
        )
        .unwrap();
    let verified_head = admission
        .verifier
        .verify_current_deployment_head(&bundle, &chain, &deployment, admission.now)
        .unwrap();
    assert_eq!(verified_head.code_hash(), deployment.code_hash());
    for invalid in 0..5 {
        let mut altered = bundle.clone();
        match invalid {
            0 => altered.interface_ops[0].data.push(0),
            1 => altered.wind_down = None,
            2 => altered.program_record_ops[0].data.push(0),
            3 => altered.program = ProgramId::new([0x32; 32]).unwrap(),
            4 => altered.commitment = admission_abi(1, 1).proof.state.commitment,
            _ => unreachable!(),
        }
        assert!(admission
            .verifier
            .verify_current_deployment_head(&altered, &chain, &deployment, admission.now)
            .is_err());
    }
    assert!(admission
        .verifier
        .verify_current_deployment_head(&bundle, &chain, &deployment, admission.now + 5001)
        .is_err());
    let source = source_record(&deployment, &config);
    let store = VerifiedSourceStore::open(config.verified.clone()).unwrap();
    store.record(&source).unwrap();
    drop(store);
    drop(registrar);
    let records = VerifiedSourceStore::open(config.verified.clone())
        .unwrap()
        .records()
        .unwrap();
    assert_eq!(records, vec![source.clone()]);
    let mut reopened = open(&config, admission.now, &peers);
    assert_eq!(
        reopened
            .source_deployment(program(), admission.now)
            .unwrap(),
        deployment
    );
    let journal = FileDeploymentJournal::open(config.journal.clone()).unwrap();
    journal
        .refresh_head(ObservedHead {
            sequence: deployment.freshness().observed_sequence,
            observed_at: admission.now,
        })
        .unwrap();
    let mut registry = Registry::new();
    registry.record_verified_deployment(&deployment).unwrap();
    let build = layerx_programs::ReproducibleBuild::from_record(
        source.source_uri.clone(),
        source.source_digest,
        source.plan.environment.clone(),
        source.artifact_digest,
    )
    .unwrap();
    assert!(matches!(
        registry.verify_source(program(), 1, &build).unwrap(),
        layerx_programs::SourceStatus::Verified { .. }
    ));
    let authority =
        layerx_programs::JournalReadAuthority::new(&journal, admission.now, 5000).unwrap();
    let read = registry.read(program(), &authority).unwrap();
    assert_eq!(read.entry.versions[0].abi_version, 5);
    assert!(matches!(
        read.entry.versions[0].source,
        layerx_programs::SourceStatus::Verified { .. }
    ));
    for abi in [0, 6, u16::MAX] {
        assert!(layerx_programs::VerifiedDeploymentEvidence::admit_abi_version(abi).is_err());
        let mut unsupported = source.clone();
        unsupported.deployment_provenance.abi_version = abi;
        assert!(VerifiedSourceStore::open(config.verified.clone())
            .unwrap()
            .record(&unsupported)
            .is_err());
    }
    let stored_path = config
        .verified
        .join(format!("{}-1.verified", hex::encode(&program().bytes())));
    let mut bytes: Value = serde_json::from_slice(&fs::read(&stored_path).unwrap()).unwrap();
    bytes["deployment_provenance"]["abi_version"] = json!(6);
    fs::write(stored_path, serde_json::to_vec(&bytes).unwrap()).unwrap();
    assert!(VerifiedSourceStore::open(config.verified.clone())
        .unwrap()
        .records()
        .is_err());
}

#[test]
fn upgrade_journal_schema_deploy_upgrade_restart() {
    let first = admission(1);
    let second = admission(2);
    let root = Directory::new();
    let peers = Peers::with_history(&second, Some(&first));
    let config = configuration(&root, &peers, &second);
    let first_evidence = first
        .verifier
        .verify_deployment(&first.proof, first.now)
        .unwrap();
    let second_evidence = second
        .verifier
        .verify_deployment(&second.proof, second.now)
        .unwrap();
    assert_eq!(
        second_evidence.record().old_code_hash,
        Some(first_evidence.code_hash())
    );
    let canonical =
        DeploymentRecord::decode(&second_evidence.record().canonical_encoding()).unwrap();
    canonical.validate().unwrap();
    assert_eq!(canonical, *second_evidence.record());
    let mut registrar = open(&config, second.now, &peers);
    for proof in [&first.proof, &second.proof] {
        let answer = registrar.route(
            &request(
                "POST",
                "/__registry/deployments",
                proof.canonical_encoding().unwrap(),
            ),
            second.now,
            Instant::now() + Duration::from_secs(10),
        );
        assert_eq!(answer.status, 200, "{}", answer.body);
    }
    assert_eq!(
        registrar
            .source_deployment(program(), second.now)
            .unwrap()
            .version(),
        2
    );
    drop(registrar);
    let mut restarted = open(&config, second.now, &peers);
    let recovered = restarted.source_deployment(program(), second.now).unwrap();
    assert_eq!(recovered, second_evidence);
    assert!(restarted.quarantined_units().is_empty());
    let journal = FileDeploymentJournal::open(config.journal.clone()).unwrap();
    let loaded = journal.load().unwrap();
    assert_eq!(loaded.units.len(), 2);
    assert!(loaded.quarantined.is_empty());
    let (replayed, quarantined) = journal
        .replay_deployments(&peers.source(&second), second.now)
        .unwrap();
    assert!(quarantined.is_empty());
    assert_eq!(
        replayed
            .iter()
            .map(|evidence| evidence.version())
            .collect::<Vec<_>>(),
        vec![1, 2]
    );
    assert_eq!(
        replayed[1].record().old_code_hash,
        Some(replayed[0].code_hash())
    );
    let mut broken = second.proof.clone();
    let mut execution = DeploymentExecutionProof::decode(&broken.state.transaction).unwrap();
    execution.previous.key.push(0);
    broken.bind_execution(&execution).unwrap();
    assert!(broken.claimed_record().is_err());
    assert!(second
        .verifier
        .verify_deployment(&broken, second.now)
        .is_err());
}

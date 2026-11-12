//! Task 22.33.1470: mandatory, self-contained registry integration proofs.
//! Peers use the committed node transaction/result corpus from 22.31.256;
//! deployment records are also produced by the real Programs Lifecycle.
//! The registry parser, clients, verifier, journal and Registrar are production code.

#[allow(dead_code)]
#[path = "support/repair_22_33_1470_chain.rs"]
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
    ProgramId::new([0x31; 32]).unwrap_or_else(|error| panic!("{error:?}"))
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
            let (key, value) = line
                .split_once('=')
                .unwrap_or_else(|| panic!("expected a value"));
            (
                key.into(),
                layerx_programs::hex::decode(value).unwrap_or_else(|error| panic!("{error:?}")),
            )
        })
        .collect()
}
fn descriptor(version: usize) -> ProgramInterface {
    ProgramInterface::decode(&transcript()[&format!("interface.{version}")])
        .unwrap_or_else(|error| panic!("{error:?}"))
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
    let mut runtime = Lifecycle::declared().unwrap_or_else(|error| panic!("{error:?}"));
    let first = runtime
        .deploy(Deploy {
            program: program(),
            code_hash: hash(&code(1)),
            wasm: code(1),
            abi_version: abi,
            upgrade_policy: policy(),
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
    let second = runtime
        .upgrade(Upgrade {
            program: program(),
            authority: [0x51; 32],
            code_hash: hash(&code(2)),
            wasm: code(2),
            abi_version: abi,
            migration: None,
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
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
            .unwrap_or_else(|error| panic!("{error:?}"))
        })
        .collect::<Vec<_>>()
        .try_into()
        .unwrap_or_else(|error| panic!("{error:?}"))
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
    let chain = ChainFixture::new(CHAIN, &SEEDS, rows).unwrap_or_else(|error| panic!("{error:?}"));
    let witness = decode_commitment_proof(
        &chain
            .encode_key(&interface_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let deployment = ChainDeploymentProof::from_state_proofs(
        ChainProgramStateProof {
            program: program(),
            ops: chain
                .prove(&program_key())
                .unwrap_or_else(|error| panic!("{error:?}")),
        },
        code(version),
        &witness,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    (chain, deployment)
}
fn value(version: usize) -> Vec<u8> {
    interface_state_value(
        program(),
        u32::try_from(version).unwrap_or_else(|error| panic!("{error:?}")),
        &descriptor(version),
    )
    .unwrap_or_else(|error| panic!("{error:?}"))
}
fn sign(fixture: &mut ChainFixture) {
    fixture.commit.height = fixture.header.height;
    fixture.commit.block_id.hash = fixture
        .header
        .hash()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .to_vec();
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
    .unwrap_or_else(|error| panic!("{error:?}"));
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
fn prior_chain(version: usize, transaction: &[u8]) -> (ChainFixture, [u8; 32]) {
    let mut prior = if version == 1 {
        let mut prior = ChainFixture::new(CHAIN, &SEEDS, vec![(b"unrelated".to_vec(), vec![1])])
            .unwrap_or_else(|error| panic!("{error:?}"));
        prior.header.height = 38;
        prior
    } else {
        admission(1).chain
    };
    let tx_root = hash_from_byte_slices(&[&hash(transaction)]);
    prior.header.data_hash = tx_root.to_vec();
    sign(&mut prior);
    (prior, tx_root)
}

fn assemble_admission(chain: ChainFixture, proof: DeploymentProof) -> Admission {
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        CHAIN,
        chain.verifier.validators.validators().to_vec(),
        5000,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let now = u64::try_from(chain.header.time.seconds).unwrap_or_else(|error| panic!("{error:?}"))
        * 1000
        + u64::try_from(chain.header.time.nanos).unwrap_or_else(|error| panic!("{error:?}"))
            / 1_000_000;
    Admission {
        chain,
        proof,
        verifier,
        now,
    }
}

fn current_chain(
    version: usize,
    abi: u16,
    producer: &BTreeMap<String, Vec<u8>>,
    prior: &ChainFixture,
) -> (ChainFixture, [u8; 32]) {
    let (mut chain, _) = state_fixture(version, abi, (abi == 1).then(|| value(version)));
    chain.header.height = prior.header.height + 1;
    chain.header.time.seconds = prior.header.time.seconds + 1;
    chain
        .header
        .last_block_id
        .hash
        .clone_from(&prior.commit.block_id.hash);
    chain.header.last_block_id.part_set_total = prior.commit.block_id.part_set_header.total;
    chain
        .header
        .last_block_id
        .part_set_hash
        .clone_from(&prior.commit.block_id.part_set_header.hash);
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
    (chain, result_root)
}

fn wind_down_ops(chain: &ChainFixture) -> Vec<layerx_programs::chain_proof::decode::ProofOp> {
    let mut status_key = b"wind-down\0s".to_vec();
    status_key.extend_from_slice(&program().bytes());
    let status_ops = chain
        .prove_absent(&programs_state_key(&status_key))
        .unwrap_or_else(|error| panic!("{error:?}"));
    status_ops
}

fn admission_abi(version: usize, abi: u16) -> Admission {
    let producer = transcript();
    let abi_five: Value = serde_json::from_str(include_str!("../src/repair_22_31_206_abi5.json"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let five_tx = hex::decode(
        abi_five["transaction_hex"]
            .as_str()
            .unwrap_or_else(|| panic!("expected a value")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let transaction = if abi == 5 {
        &five_tx
    } else {
        &producer[if version == 1 { "deploy" } else { "upgrade" }]
    };
    let (prior, tx_root) = prior_chain(version, transaction);
    let (chain, result_root) = current_chain(version, abi, &producer, &prior);
    let commitment = decode_commitment_proof(
        &chain
            .encode_key(&program_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let interface_state = decode_commitment_proof(
        &chain
            .encode_key(&interface_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let state = ChainProgramStateProof {
        program: program(),
        ops: chain
            .prove(&program_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    };
    let status_ops = wind_down_ops(&chain);
    let deployment =
        ChainDeploymentProof::from_state_proofs(state.clone(), code(version), &interface_state)
            .unwrap_or_else(|error| panic!("{error:?}"));
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
            previous: decode_commitment_proof(
                &prior
                    .encode_key(&program_key())
                    .unwrap_or_else(|error| panic!("{error:?}")),
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
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
                chain
                    .prove(&interface_key())
                    .unwrap_or_else(|error| panic!("{error:?}"))
            } else {
                chain
                    .prove_absent(&interface_key())
                    .unwrap_or_else(|error| panic!("{error:?}"))
            },
            migration: None,
            migration_ops: vec![],
            migration_previous_ops: vec![],
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
    assemble_admission(chain, proof)
}

use layerx_platform_registry::{
    committed_head_route, parse_request, write_response, Config, FileDeploymentJournal,
    HermeticBuilder, Registrar, RegistryAuthority, Request, Response,
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
        fs::create_dir(&path).unwrap_or_else(|error| panic!("{error:?}"));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn fixture_ca() -> (
    openssl::pkey::PKey<openssl::pkey::Private>,
    openssl::x509::X509,
) {
    use openssl::{
        asn1::Asn1Time,
        bn::BigNum,
        hash::MessageDigest,
        pkey::PKey,
        rsa::Rsa,
        x509::{
            extension::{BasicConstraints, KeyUsage},
            X509NameBuilder, X509,
        },
    };
    let ca_key = PKey::from_rsa(Rsa::generate(2048).unwrap_or_else(|error| panic!("{error:?}")))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut ca_name = X509NameBuilder::new().unwrap_or_else(|error| panic!("{error:?}"));
    ca_name
        .append_entry_by_text("CN", "Registry repair test CA")
        .unwrap_or_else(|error| panic!("{error:?}"));
    let ca_name = ca_name.build();
    let mut ca = X509::builder().unwrap_or_else(|error| panic!("{error:?}"));
    ca.set_version(2)
        .unwrap_or_else(|error| panic!("{error:?}"));
    ca.set_serial_number(
        &BigNum::from_u32(1)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .to_asn1_integer()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    ca.set_subject_name(&ca_name)
        .unwrap_or_else(|error| panic!("{error:?}"));
    ca.set_issuer_name(&ca_name)
        .unwrap_or_else(|error| panic!("{error:?}"));
    ca.set_pubkey(&ca_key)
        .unwrap_or_else(|error| panic!("{error:?}"));
    ca.set_not_before(&Asn1Time::days_from_now(0).unwrap_or_else(|error| panic!("{error:?}")))
        .unwrap_or_else(|error| panic!("{error:?}"));
    ca.set_not_after(&Asn1Time::days_from_now(1).unwrap_or_else(|error| panic!("{error:?}")))
        .unwrap_or_else(|error| panic!("{error:?}"));
    ca.append_extension(
        BasicConstraints::new()
            .critical()
            .ca()
            .build()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    ca.append_extension(
        KeyUsage::new()
            .critical()
            .key_cert_sign()
            .build()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    ca.sign(&ca_key, MessageDigest::sha256())
        .unwrap_or_else(|error| panic!("{error:?}"));
    let ca = ca.build();
    (ca_key, ca)
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
    let (ca_key, ca) = fixture_ca();
    let key = PKey::from_rsa(Rsa::generate(2048).unwrap_or_else(|error| panic!("{error:?}")))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut name = X509NameBuilder::new().unwrap_or_else(|error| panic!("{error:?}"));
    name.append_entry_by_text("CN", "localhost")
        .unwrap_or_else(|error| panic!("{error:?}"));
    let name = name.build();
    let mut cert = X509::builder().unwrap_or_else(|error| panic!("{error:?}"));
    cert.set_version(2)
        .unwrap_or_else(|error| panic!("{error:?}"));
    cert.set_serial_number(
        &BigNum::from_u32(2)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .to_asn1_integer()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    cert.set_subject_name(&name)
        .unwrap_or_else(|error| panic!("{error:?}"));
    cert.set_issuer_name(ca.subject_name())
        .unwrap_or_else(|error| panic!("{error:?}"));
    cert.set_pubkey(&key)
        .unwrap_or_else(|error| panic!("{error:?}"));
    cert.set_not_before(&Asn1Time::days_from_now(0).unwrap_or_else(|error| panic!("{error:?}")))
        .unwrap_or_else(|error| panic!("{error:?}"));
    cert.set_not_after(&Asn1Time::days_from_now(1).unwrap_or_else(|error| panic!("{error:?}")))
        .unwrap_or_else(|error| panic!("{error:?}"));
    cert.append_extension(
        BasicConstraints::new()
            .critical()
            .build()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    cert.append_extension(
        KeyUsage::new()
            .digital_signature()
            .key_encipherment()
            .build()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    cert.append_extension(
        ExtendedKeyUsage::new()
            .server_auth()
            .build()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    cert.append_extension(
        SubjectAlternativeName::new()
            .dns("localhost")
            .ip("127.0.0.1")
            .build(&cert.x509v3_context(Some(&ca), None))
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    cert.sign(&ca_key, MessageDigest::sha256())
        .unwrap_or_else(|error| panic!("{error:?}"));
    let der = cert
        .build()
        .to_der()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let config = rustls::ServerConfig::builder()
        .with_no_client_auth()
        .with_single_cert(
            vec![rustls::pki_types::CertificateDer::from(der.clone())],
            rustls::pki_types::PrivateKeyDer::Pkcs8(
                key.private_key_to_pkcs8()
                    .unwrap_or_else(|error| panic!("{error:?}"))
                    .into(),
            ),
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
    (
        ca.to_der().unwrap_or_else(|error| panic!("{error:?}")),
        Arc::new(config),
    )
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
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{error:?}"));
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{error:?}"));
        let endpoint = format!(
            "{}://{}",
            if tls.is_some() { "https" } else { "http" },
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("{error:?}"))
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
                    .unwrap_or_else(|error| panic!("{error:?}"));
                socket
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap_or_else(|error| panic!("{error:?}"));
                if let Some(tls) = &tls {
                    let connection = rustls::ServerConnection::new(tls.clone())
                        .unwrap_or_else(|error| panic!("{error:?}"));
                    let mut stream = rustls::StreamOwned::new(connection, socket);
                    if let Ok(request) = peer_request(&mut stream) {
                        write_response(&mut stream, &handler(request))
                            .unwrap_or_else(|error| panic!("{error:?}"));
                    }
                } else {
                    let mut stream = socket;
                    let request =
                        peer_request(&mut stream).unwrap_or_else(|error| panic!("{error:?}"));
                    write_response(&mut stream, &handler(request))
                        .unwrap_or_else(|error| panic!("{error:?}"));
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
        self.thread
            .take()
            .unwrap_or_else(|| panic!("expected a value"))
            .join()
            .unwrap_or_else(|error| panic!("{error:?}"));
    }
}
fn response(status: u16, body: &Value) -> Response {
    Response {
        status,
        body: body.to_string(),
    }
}

fn head_document(admission: &Admission) -> Value {
    let raw = transcript()[if admission
        .proof
        .claimed_record()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .version
        == 1
        && admission
            .proof
            .claimed_record()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .abi_version
            == 1
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
        "chain_evidence": {"commitment_hex": hex::encode(&admission.proof.state.commitment.encode().unwrap_or_else(|error| panic!("{error:?}"))),
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
        let mut heads = BTreeMap::from([(
            u64::try_from(admission.chain.header.height)
                .unwrap_or_else(|error| panic!("{error:?}")),
            document.clone(),
        )]);
        if let Some(historical) = historical {
            heads.insert(
                u64::try_from(historical.chain.header.height)
                    .unwrap_or_else(|error| panic!("{error:?}")),
                head_document(historical),
            );
        }
        let (replica, authority) = start_head_servers(&heads, &document, &tls);
        let (encoded, rpc) = program_rpc(admission);
        let head = document.clone();
        let node = Server::start(Some(tls.clone()), move |request| {
            assert_eq!(
                request.headers.get("authorization").map(String::as_str),
                Some("Bearer node-repair-credential")
            );
            if request.path == committed_head_route::HEAD_PATH {
                return response(200, &head);
            }
            if let Some(digest) = request
                .path
                .strip_prefix("/v1/receipts/")
                .and_then(|rest| rest.strip_suffix("/account-state"))
            {
                let historical = heads
                    .values()
                    .find(|head| head["receipt_digest"].as_str() == Some(digest))
                    .unwrap_or_else(|| panic!("expected a value"));
                return response(200, historical);
            }
            if request
                .path
                .starts_with("/v1/programs/account-state/changes?")
            {
                return response(
                    200,
                    &json!({"complete_through":{"sequence":head["observed_sequence"],"ordinal":0},"scanned_through_sequence":head["observed_sequence"],"caught_up":true,"records":[]}),
                );
            }
            if request.path.contains("/state-proof?at=") {
                return response(
                    200,
                    &json!({"program_id":hex::encode(&program().bytes()),"record_hex":hex::encode(&encoded),"record_digest":hex::encode(&hash(&encoded)),"receipt_digest":head["receipt_digest"]}),
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
}

fn start_head_servers(
    heads: &BTreeMap<u64, Value>,
    document: &Value,
    tls: &Arc<rustls::ServerConfig>,
) -> (Server, Server) {
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
            &height
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
        response(status, &body)
    });
    (replica, authority)
}

fn program_rpc(admission: &Admission) -> (Vec<u8>, Server) {
    let bundle = layerx_programs::ProgramStateBundle {
        commitment: admission.proof.state.commitment.clone(),
        program: program(),
        program_record_ops: admission.proof.state.state.ops.clone(),
        interface_ops: decode_commitment_proof(
            admission
                .proof
                .deployment
                .interface
                .as_deref()
                .unwrap_or_else(|| panic!("expected a value")),
        )
        .unwrap_or_else(|error| panic!("{error:?}"))
        .proof_ops,
        wind_down: Some(admission.proof.state.wind_down.clone()),
    };
    let encoded = bundle
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let rpc = Server::start(None, move |request| {
        assert_eq!(request.path, "/");
        assert_eq!(request.method, "POST");
        let rpc: Value =
            serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(rpc["method"], "eth_call");
        assert_eq!(rpc["params"][0]["to"], PROGRAMS);
        response(200, &rpc_result(&storage_head_abi()))
    });
    (encoded, rpc)
}

fn configuration(root: &Directory, peers: &Peers, admission: &Admission) -> Config {
    let header = admission
        .chain
        .verifier
        .verify_head(&admission.chain.header, &admission.chain.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let trust = layerx_programs::chain_proof::trust_store::TrustRoot::from_head(
        &header,
        admission.chain.verifier.validators.clone(),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let trust_path = root.0.join("trust");
    fs::write(
        &trust_path,
        trust
            .canonical_bytes()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    fs::set_permissions(&trust_path, fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("{error:?}"));
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
        request_authority: RegistryAuthority::new(TOKEN.to_owned().into())
            .unwrap_or_else(|error| panic!("{error:?}")),
        publication_authority: RegistryAuthority::new(
            "operator-repair-credential".to_owned().into(),
        )
        .unwrap_or_else(|error| panic!("{error:?}")),
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
    })).unwrap_or_else(|error| panic!("{error:?}"))
}
fn open(config: &Config, now: u64, peers: &Peers) -> Registrar {
    Registrar::open_with_builder_and_chain(
        config,
        now,
        worker_builder(config),
        true,
        &peers.rpc.endpoint,
    )
    .unwrap_or_else(|error| panic!("{error:?}"))
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
            .unwrap_or_else(|error| panic!("{error:?}"))
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
            .unwrap_or_else(|error| panic!("{error:?}"))
    } else {
        agent
            .get(&url)
            .header("authorization", format!("Bearer {TOKEN}"))
            .call()
            .unwrap_or_else(|error| panic!("{error:?}"))
    };
    (
        answer.status().as_u16(),
        answer
            .body_mut()
            .read_json()
            .unwrap_or_else(|error| panic!("{error:?}")),
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
    .unwrap_or_else(|error| panic!("{error:?}"))
}

fn rpc_result(encoded: &[u8]) -> Value {
    json!({"jsonrpc":"2.0","id":1,"result":format!("0x{}",hex::encode(encoded))})
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
    let rpc: Value =
        serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{error:?}"));
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
        response(200, &rpc_result(&storage_head_abi()))
    })
}

fn assert_recovered_history(
    first: &Admission,
    second: &Admission,
    proofs: &[DeploymentProof],
    reopened: &FileDeploymentJournal,
    deployed: &layerx_programs::VerifiedDeploymentEvidence,
    upgraded: &layerx_programs::VerifiedDeploymentEvidence,
) {
    let current = second
        .chain
        .verifier
        .verify_head(&second.chain.header, &second.chain.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
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
            let height = proof
                .claimed_record()
                .unwrap_or_else(|error| panic!("{error:?}"))
                .sequence;
            let start = history
                .iter()
                .position(|(header, _, _)| {
                    u64::try_from(header.height).unwrap_or_else(|error| panic!("{error:?}"))
                        == height
                })
                .unwrap_or_else(|| panic!("expected a value"));
            second
                .verifier
                .verify_historical_deployment(proof, &current, &history[start..], second.now)
                .unwrap_or_else(|error| panic!("{error:?}"))
        })
        .collect::<Vec<_>>();
    verified.sort_by_key(layerx_programs::VerifiedDeploymentEvidence::version);
    assert_eq!(verified, vec![deployed.clone(), upgraded.clone()]);
    for evidence in &verified {
        reopened
            .audit_projection(evidence)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(
            reopened
                .canonical_record(evidence.receipt_digest())
                .unwrap_or_else(|error| panic!("{error:?}")),
            evidence.record().canonical_encoding()
        );
    }
    recovered
        .record_verified_deployment(&verified[0])
        .unwrap_or_else(|error| panic!("{error:?}"));
    recovered
        .record_verified_upgrade(&verified[0], &verified[1])
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        recovered
            .latest_version(program())
            .unwrap_or_else(|error| panic!("{error:?}")),
        2
    );
    assert_eq!(
        recovered
            .resolve_deployment(upgraded.clone())
            .unwrap_or_else(|error| panic!("{error:?}"))
            .record(),
        upgraded.record()
    );
}

#[test]
pub(crate) fn upgrade_deploy_reopen_preserves_authenticated_history() {
    let first = admission(1);
    let second = admission(2);
    let deployed = first
        .verifier
        .verify_deployment(&first.proof, first.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let upgraded = second
        .verifier
        .verify_deployment(&second.proof, second.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(deployed.record(), &records(1)[0]);
    assert_eq!(upgraded.record(), &records(1)[1]);
    assert_eq!(upgraded.record().old_code_hash, Some(deployed.code_hash()));
    upgraded
        .record()
        .validate()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut registry = Registry::new();
    assert_eq!(
        registry.record_verified_upgrade(&deployed, &upgraded),
        Err(layerx_programs::RegistryError::VersionHistoryMismatch)
    );
    registry
        .record_verified_deployment(&deployed)
        .unwrap_or_else(|error| panic!("{error:?}"));
    registry
        .record_verified_upgrade(&deployed, &upgraded)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        registry
            .latest_version(program())
            .unwrap_or_else(|error| panic!("{error:?}")),
        2
    );

    let root = Directory::new();
    let journal_path = root.0.join("journal");
    let journal = FileDeploymentJournal::open(journal_path.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    journal
        .append(&deployed)
        .unwrap_or_else(|error| panic!("{error:?}"));
    journal
        .append(&upgraded)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let before = journal.load().unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(before.units.len(), 2);
    assert!(before.quarantined.is_empty());
    assert!(before.leftovers.is_empty());
    let proofs = journal.proofs().unwrap_or_else(|error| panic!("{error:?}"));
    drop(journal);
    let reopened = FileDeploymentJournal::open(journal_path.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    let after = reopened.load().unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(before.units, after.units);
    assert!(after.quarantined.is_empty());
    assert!(after.leftovers.is_empty());
    assert_eq!(
        reopened
            .proofs()
            .unwrap_or_else(|error| panic!("{error:?}")),
        proofs
    );
    assert_recovered_history(&first, &second, &proofs, &reopened, &deployed, &upgraded);
    assert_invalid_previous_proofs(&second);
    assert_corrupt_upgrade_envelopes(&journal_path, &upgraded);
}

fn assert_invalid_previous_proofs(second: &Admission) {
    let mut absent = second.proof.clone();
    let mut execution = DeploymentExecutionProof::decode(&absent.state.transaction)
        .unwrap_or_else(|error| panic!("{error:?}"));
    execution.previous.proof_ops.clear();
    absent
        .bind_execution(&execution)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(second
        .verifier
        .verify_deployment(&absent, second.now)
        .is_err());
    let mut corrupt = second.proof.clone();
    let mut execution = DeploymentExecutionProof::decode(&corrupt.state.transaction)
        .unwrap_or_else(|error| panic!("{error:?}"));
    execution.previous.proof_ops[0].data[0] ^= 1;
    corrupt
        .bind_execution(&execution)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(second
        .verifier
        .verify_deployment(&corrupt, second.now)
        .is_err());
}

fn assert_corrupt_upgrade_envelopes(
    journal_path: &std::path::Path,
    upgraded: &layerx_programs::VerifiedDeploymentEvidence,
) {
    use layerx_platform_registry::{DeploymentEnvelope, UnitDefect, UnitPart};
    let envelope = DeploymentEnvelope::from_evidence(upgraded);
    let encoded = envelope
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let path = journal_path.join(format!(
        "{}.envelope",
        hex::encode(&upgraded.receipt_digest())
    ));
    let mut corrupt = encoded.clone();
    *corrupt
        .last_mut()
        .unwrap_or_else(|| panic!("expected a value")) ^= 1;
    fs::write(&path, &corrupt).unwrap_or_else(|error| panic!("{error:?}"));
    let damaged = FileDeploymentJournal::open(journal_path.to_path_buf())
        .unwrap_or_else(|error| panic!("{error:?}"));
    let loaded = damaged.load().unwrap_or_else(|error| panic!("{error:?}"));
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
    let (record_frame, _, _) = envelope
        .frames()
        .unwrap_or_else(|error| panic!("{error:?}"));
    fs::write(&path, &record_frame).unwrap_or_else(|error| panic!("{error:?}"));
    let missing = FileDeploymentJournal::open(journal_path.to_path_buf())
        .unwrap_or_else(|error| panic!("{error:?}"));
    let loaded = missing.load().unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(loaded.units.len(), 1);
    assert_eq!(loaded.quarantined.len(), 1);
    assert_eq!(
        loaded.quarantined[0].defect,
        UnitDefect::Missing(UnitPart::Proof)
    );
    assert!(missing.proofs().is_err());
}

fn assert_state_head(
    fixture: &Admission,
    deployment_journal: &FileDeploymentJournal,
    stored: &[u8],
    cursor: layerx_platform_registry::ProgramStateCursor,
) -> Server {
    let head_peer = head_rpc(fixture);
    let authority =
        RegistryAuthority::new(TOKEN.to_owned().into()).unwrap_or_else(|error| panic!("{error:?}"));
    let response = Registrar::route_chain_head(
        &request(
            "POST",
            "/__registry/head",
            serde_json::to_vec(&json!({"program_id":hex::encode(&program().bytes())}))
                .unwrap_or_else(|error| panic!("{error:?}")),
        ),
        &authority,
        deployment_journal,
        &format!("{}/", head_peer.endpoint),
        Instant::now() + Duration::from_secs(5),
    );
    assert_eq!(response.status, 200, "{}", response.body);
    let body: Value =
        serde_json::from_str(&response.body).unwrap_or_else(|error| panic!("{error:?}"));
    let keccak = layerx_crypto::evm_transaction::transaction_hash(stored);
    assert_eq!(body["state_root"], hex::encode(&keccak));
    assert_eq!(body["sequence"], cursor.sequence);
    head_peer
}

#[test]
pub(crate) fn program_state_root_transport_persists_committed_head() {
    use layerx_platform_registry::{FileProgramStateJournal, ProgramStateCursor};
    let fixture = admission(1);
    let root = Directory::new();
    let peer = state_rpc(
        u64::try_from(fixture.chain.header.height).unwrap_or_else(|error| panic!("{error:?}")),
    );
    let journal = FileProgramStateJournal::open(root.0.join("state"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let encoded = storage_head_abi();
    assert_eq!(encoded.len(), 128);
    let expected = encoded[64..102].to_vec();
    assert_eq!(&expected[6..], hash(&expected[..6]));
    let deadline = || Instant::now() + Duration::from_secs(5);
    let stored = journal
        .store(
            &format!("{}/", peer.endpoint),
            program(),
            u64::try_from(fixture.chain.header.height).unwrap_or_else(|error| panic!("{error:?}")),
            deadline(),
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(stored, expected);
    let digest = hash(&stored);
    let path = root.0.join("state").join(format!(
        "{}.{}.program-state",
        hex::encode(&program().bytes()),
        hex::encode(&digest)
    ));
    assert_eq!(
        fs::read(&path).unwrap_or_else(|error| panic!("{error:?}")),
        expected
    );
    assert_eq!(
        journal.cursor().unwrap_or_else(|error| panic!("{error:?}")),
        ProgramStateCursor::default()
    );
    let cursor = ProgramStateCursor {
        sequence: u64::try_from(fixture.chain.header.height)
            .unwrap_or_else(|error| panic!("{error:?}")),
        ordinal: 0,
    };
    journal
        .advance(cursor)
        .unwrap_or_else(|error| panic!("{error:?}"));
    journal.audit().unwrap_or_else(|error| panic!("{error:?}"));
    drop(journal);
    let reopened = FileProgramStateJournal::open(root.0.join("state"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        reopened
            .cursor()
            .unwrap_or_else(|error| panic!("{error:?}")),
        cursor
    );
    reopened.audit().unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        fs::read(&path).unwrap_or_else(|error| panic!("{error:?}")),
        stored
    );
    let deployment_journal = FileDeploymentJournal::open(root.0.join("deployments"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let head_peer = assert_state_head(&fixture, &deployment_journal, &stored, cursor);
    drop(deployment_journal);
    let recovered = FileDeploymentJournal::open(root.0.join("deployments"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        recovered
            .observed_head()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .sequence,
        cursor.sequence
    );
    drop(peer);
    assert_program_state_refusals(
        &reopened,
        &encoded,
        cursor,
        &head_peer.endpoint,
        &path,
        &stored,
        &root,
    );
}

fn assert_invalid_state_responses(
    reopened: &layerx_platform_registry::FileProgramStateJournal,
    encoded: &[u8],
) {
    for (index, value) in [
        (0, 1),
        (31, 64),
        (32, 1),
        (63, 37),
        (64, 1),
        (102, 1),
        (101, 0),
    ] {
        let mut invalid = encoded.to_vec();
        invalid[index] = value;
        assert!(
            reopened
                .store_response(
                    program(),
                    &serde_json::to_vec(&rpc_result(&invalid))
                        .unwrap_or_else(|error| panic!("{error:?}"))
                )
                .is_err(),
            "mutation at {index}"
        );
    }
    for invalid in [
        encoded[..127].to_vec(),
        [encoded.to_vec(), vec![0; 32]].concat(),
    ] {
        assert!(reopened
            .store_response(
                program(),
                &serde_json::to_vec(&rpc_result(&invalid))
                    .unwrap_or_else(|error| panic!("{error:?}"))
            )
            .is_err());
    }
    for invalid in [
        json!({"jsonrpc":"2.0","id":1,"error":{"code":3,"message":"execution reverted"}}),
        json!({"jsonrpc":"2.0","id":2,"result":rpc_result(encoded)["result"]}),
        json!({"jsonrpc":"1.0","id":1,"result":"0x"}),
        json!({"jsonrpc":"2.0","id":1,"result":"0xzz"}),
        json!({"jsonrpc":"2.0","id":1,"result":null}),
    ] {
        assert!(reopened
            .store_response(
                program(),
                &serde_json::to_vec(&invalid).unwrap_or_else(|error| panic!("{error:?}"))
            )
            .is_err());
    }
    assert!(reopened.store_response(program(), b"{").is_err());
}

fn assert_program_state_refusals(
    reopened: &layerx_platform_registry::FileProgramStateJournal,
    encoded: &[u8],
    cursor: layerx_platform_registry::ProgramStateCursor,
    head_endpoint: &str,
    path: &std::path::Path,
    stored: &[u8],
    root: &Directory,
) {
    let deadline = || Instant::now() + Duration::from_secs(5);
    assert!(reopened
        .store(&format!("{head_endpoint}/"), program(), 0, deadline())
        .is_err());
    assert!(reopened
        .store(
            &format!("{head_endpoint}/"),
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
    assert_invalid_state_responses(reopened, encoded);
    let unavailable =
        std::net::TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{error:?}"));
    let endpoint = format!(
        "http://{}/",
        unavailable
            .local_addr()
            .unwrap_or_else(|error| panic!("{error:?}"))
    );
    let observer = thread::spawn(move || {
        let (mut socket, _) = unavailable
            .accept()
            .unwrap_or_else(|error| panic!("{error:?}"));
        socket
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap_or_else(|error| panic!("{error:?}"));
        let message = peer_request(&mut socket).unwrap_or_else(|error| panic!("{error:?}"));
        let rpc = rpc_request(&message);
        assert_eq!(rpc["params"][1], format!("0x{:x}", cursor.sequence));
    });
    assert!(reopened
        .store(&endpoint, program(), cursor.sequence, deadline())
        .is_err());
    observer.join().unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        reopened
            .cursor()
            .unwrap_or_else(|error| panic!("{error:?}")),
        cursor
    );
    assert_eq!(
        fs::read(path).unwrap_or_else(|error| panic!("{error:?}")),
        stored
    );
    assert_eq!(
        fs::read_dir(root.0.join("state"))
            .unwrap_or_else(|error| panic!("{error:?}"))
            .count(),
        2
    );
    fs::write(path, b"corrupt").unwrap_or_else(|error| panic!("{error:?}"));
    assert!(reopened.audit().is_err());
}

fn head_rpc(fixture: &Admission) -> Server {
    let record = program_abi(
        &fixture
            .proof
            .claimed_record()
            .unwrap_or_else(|error| panic!("{error:?}")),
    );
    let height =
        u64::try_from(fixture.chain.header.height).unwrap_or_else(|error| panic!("{error:?}"));
    let timestamp = fixture.chain.header.time.seconds;
    Server::start(None, move |request| {
        let rpc = rpc_request(&request);
        let result = match rpc["method"]
            .as_str()
            .unwrap_or_else(|| panic!("expected a value"))
        {
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
                let data = rpc["params"][0]["data"]
                    .as_str()
                    .unwrap_or_else(|| panic!("expected a value"));
                if data == selector("getProgram(bytes32)") {
                    rpc_result(&record)["result"].clone()
                } else {
                    assert_eq!(data, selector("getStateHead(bytes32)"));
                    rpc_result(&storage_head_abi())["result"].clone()
                }
            }
            method => panic!("unexpected node method {method}"),
        };
        response(200, &json!({"jsonrpc":"2.0","id":1,"result":result}))
    })
}

fn deployment_rpc_peer(
    receipt: &Value,
    evidence: &layerx_programs::VerifiedDeploymentEvidence,
    transaction: [u8; 32],
    failed_transaction: [u8; 32],
    block_hash: [u8; 32],
) -> (Arc<std::sync::Mutex<usize>>, Server) {
    let valid = receipt.clone();
    let encoded = program_abi(evidence.record());
    let counter = Arc::new(std::sync::Mutex::new(0_usize));
    let seen = counter.clone();
    let peer = Server::start(None, move |request| {
        let rpc = rpc_request(&request);
        *seen.lock().unwrap_or_else(|error| panic!("{error:?}")) += 1;
        let result = match rpc["method"]
            .as_str()
            .unwrap_or_else(|| panic!("expected a value"))
        {
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
                rpc_result(&encoded)["result"].clone()
            }
            method => panic!("unexpected method {method}"),
        };
        response(200, &json!({"jsonrpc":"2.0","id":1,"result":result}))
    });
    (counter, peer)
}

fn assert_mutated_deployments(
    receipt: &Value,
    evidence: &layerx_programs::VerifiedDeploymentEvidence,
    root: &Directory,
    path: &std::path::Path,
    stored: &[u8],
    transaction: [u8; 32],
    failed_transaction: [u8; 32],
) {
    let authority =
        RegistryAuthority::new(TOKEN.to_owned().into()).unwrap_or_else(|error| panic!("{error:?}"));
    let mut req = request("POST", "/__registry/deployments", transaction.to_vec());
    req.headers
        .insert("authorization".into(), format!("Bearer {TOKEN}"));
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
                rpc_result(&record)["result"].clone()
            };
            response(200, &json!({"jsonrpc":"2.0","id":1,"result":result}))
        });
        req.body = transaction.to_vec();
        let refused = Registrar::route_chain_deployment(
            &req,
            &authority,
            &root.0.join("journal"),
            &format!("{}/", peer.endpoint),
            Instant::now() + Duration::from_secs(5),
        );
        assert_eq!(refused.status, 422, "mutation {mutation}: {}", refused.body);
        assert_eq!(
            fs::read(path).unwrap_or_else(|error| panic!("{error:?}")),
            stored
        );
    }
}

fn deployment_receipt(
    evidence: &layerx_programs::VerifiedDeploymentEvidence,
    transaction: [u8; 32],
    block_hash: [u8; 32],
    block_number: u64,
) -> Value {
    let topic =
        layerx_crypto::evm_transaction::transaction_hash(b"ProgramDeployed(bytes32,bytes32)");
    let receipt = json!({
        "transactionHash":format!("0x{}",hex::encode(&transaction)),
        "blockHash":format!("0x{}",hex::encode(&block_hash)),"blockNumber":format!("0x{block_number:x}"),"status":"0x1",
        "logs":[{"address":PROGRAMS,"topics":[format!("0x{}",hex::encode(&topic)),format!("0x{}",hex::encode(&program().bytes()))],
        "data":format!("0x{}",hex::encode(&evidence.code_hash())),"transactionHash":format!("0x{}",hex::encode(&transaction)),
        "blockHash":format!("0x{}",hex::encode(&block_hash)),"blockNumber":format!("0x{block_number:x}"),"removed":false}]
    });
    receipt
}

fn assert_stored_deployment(
    root: &Directory,
    transaction: [u8; 32],
    body: &Value,
) -> (std::path::PathBuf, Vec<u8>) {
    let path = root
        .0
        .join("journal/chain-deployments")
        .join(format!("{}.json", hex::encode(&transaction)));
    let stored = fs::read(&path).unwrap_or_else(|error| panic!("{error:?}"));
    let metadata: Value =
        serde_json::from_slice(&stored).unwrap_or_else(|error| panic!("{error:?}"));
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
    (path, stored)
}

fn assert_deployment_answer(
    answer: &Response,
    evidence: &layerx_programs::VerifiedDeploymentEvidence,
    block_number: u64,
) -> Value {
    assert_eq!(answer.status, 200, "{}", answer.body);
    let body: Value =
        serde_json::from_str(&answer.body).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(body["program_id"], hex::encode(&program().bytes()));
    assert_eq!(body["code_hash"], hex::encode(&evidence.code_hash()));
    assert_eq!(body["abi_version"], evidence.abi_version());
    assert_eq!(body["version"], evidence.version());
    assert_eq!(body["policy"], 1);
    assert_eq!(body["authority"], hex::encode(&[0x51; 32]));
    assert_eq!(body["block_number"], block_number);
    body
}

fn assert_malformed_deployment_requests(
    route: &impl Fn(&Request) -> Response,
    req: &mut Request,
    counter: &Arc<std::sync::Mutex<usize>>,
) {
    req.headers.clear();
    assert_eq!(route(req).status, 401);
    req.headers
        .insert("authorization".into(), format!("Bearer {TOKEN}"));
    for malformed in [
        Vec::new(),
        b"{\"transaction_hash\":\"0x01\"}".to_vec(),
        b"{\"transaction_hash\":null}".to_vec(),
        b"{\"transaction_hash\":\"bad\",\"extra\":1}".to_vec(),
    ] {
        req.body = malformed;
        assert_eq!(route(req).status, 400);
    }
    assert_eq!(
        *counter.lock().unwrap_or_else(|error| panic!("{error:?}")),
        0
    );
}

#[test]
pub(crate) fn programs_root_transport_admits_and_refuses_transactions() {
    let fixture = admission(1);
    let evidence = fixture
        .verifier
        .verify_deployment(&fixture.proof, fixture.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let root = Directory::new();
    let journal = FileDeploymentJournal::open(root.0.join("journal"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    journal
        .append(&evidence)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let transaction = evidence.activity_id();
    let failed_transaction = hash(&transcript()["unrelated"]);
    let block_hash = fixture
        .chain
        .header
        .hash()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let block_number =
        u64::try_from(fixture.chain.header.height).unwrap_or_else(|error| panic!("{error:?}"));
    let receipt = deployment_receipt(&evidence, transaction, block_hash, block_number);
    let (counter, peer) = deployment_rpc_peer(
        &receipt,
        &evidence,
        transaction,
        failed_transaction,
        block_hash,
    );
    let authority =
        RegistryAuthority::new(TOKEN.to_owned().into()).unwrap_or_else(|error| panic!("{error:?}"));
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
    assert_malformed_deployment_requests(&route, &mut req, &counter);
    req.body = transaction.to_vec();
    let body = assert_deployment_answer(&route(&req), &evidence, block_number);
    let (path, stored) = assert_stored_deployment(&root, transaction, &body);
    req.body =
        serde_json::to_vec(&json!({"transaction_hash":format!("0x{}",hex::encode(&transaction))}))
            .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(route(&req).status, 200);
    assert_eq!(
        fs::read(&path).unwrap_or_else(|error| panic!("{error:?}")),
        stored
    );
    req.body = failed_transaction.to_vec();
    let refused = route(&req);
    assert_eq!(refused.status, 422, "{}", refused.body);
    assert_eq!(
        serde_json::from_str::<Value>(&refused.body).unwrap_or_else(|error| panic!("{error:?}"))
            ["error"]["code"],
        "deployment_proof_refused"
    );
    assert!(!path
        .with_file_name(format!("{}.json", hex::encode(&failed_transaction)))
        .exists());
    assert_eq!(
        fs::read(&path).unwrap_or_else(|error| panic!("{error:?}")),
        stored
    );
    drop(peer);
    assert_mutated_deployments(
        &receipt,
        &evidence,
        &root,
        &path,
        &stored,
        transaction,
        failed_transaction,
    );
    assert_committed_program_head(
        &fixture,
        journal,
        &authority,
        &root,
        block_number,
        &path,
        &stored,
    );
}

fn assert_committed_program_head(
    fixture: &Admission,
    journal: FileDeploymentJournal,
    authority: &RegistryAuthority,
    root: &Directory,
    block_number: u64,
    path: &std::path::Path,
    stored: &[u8],
) {
    let head_peer = head_rpc(fixture);
    let head_request = request(
        "POST",
        "/__registry/head",
        serde_json::to_vec(&json!({"program_id":hex::encode(&program().bytes())}))
            .unwrap_or_else(|error| panic!("{error:?}")),
    );
    let head = Registrar::route_chain_head(
        &head_request,
        authority,
        &journal,
        &format!("{}/", head_peer.endpoint),
        Instant::now() + Duration::from_secs(5),
    );
    assert_eq!(head.status, 200, "{}", head.body);
    let head: Value = serde_json::from_str(&head.body).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(head["sequence"], block_number);
    assert_eq!(
        head["state_root"],
        hex::encode(&layerx_crypto::evm_transaction::transaction_hash(
            &storage_head_abi()[64..102]
        ))
    );
    assert!(head.get("receipt_digest").is_none());
    drop(journal);
    let reopened = FileDeploymentJournal::open(root.0.join("journal"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        reopened
            .observed_head()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .sequence,
        block_number
    );
    assert_eq!(
        reopened
            .load()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .units
            .len(),
        1
    );
    assert_eq!(
        fs::read(path).unwrap_or_else(|error| panic!("{error:?}")),
        stored
    );
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
    let proof = admission
        .proof
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
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
        .unwrap_or_else(|error| panic!("{error:?}"))
        .load()
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(loaded.units.len(), 1);
    assert!(loaded.quarantined.is_empty());
    assert_eq!(loaded.units[0].proof(), &admission.proof);
    drop(service);
}

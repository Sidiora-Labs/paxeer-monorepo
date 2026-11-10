use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use ed25519_dalek::{Signer, SigningKey};
use layerx_agentd::read::{
    ChainHeadError, LayerxdProgramBalanceReader, ProgramAuthority, ProgramReadError,
};
use layerx_programs::{
    chain_evidence::{self, wind_down::ChainWindDownProof},
    chain_proof::{
        decode::decode_commitment_proof,
        merkle::{hash_from_byte_slices, SimpleProof},
        validators::TrustedValidator,
        vote::{vote_sign_bytes, Timestamp},
    },
    interface_state_key, interface_state_value, InterfaceEntryPoint, ProgramBundleError, ProgramId,
    ProgramInterface, ProgramStateBundle, ProtocolDeploymentVerifier, ProtocolEvidenceError,
    Registry, ValueSchema, ValueType,
};
use openssl::{
    asn1::Asn1Time,
    hash::MessageDigest,
    pkey::PKey,
    rsa::Rsa,
    x509::{X509NameBuilder, X509},
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

use deployment_support::chain;

#[allow(
    dead_code,
    unused_imports,
    clippy::too_many_lines,
    clippy::expect_used,
    clippy::manual_let_else,
    reason = "The registry workspace owns these shared fixtures; this target reports dead_code, unused_imports, clippy::too_many_lines and clippy::expect_used in mod.rs, and clippy::too_many_lines and clippy::manual_let_else in chain.rs."
)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/mod.rs"]
mod deployment_support;

const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const REPLICA: [u8; 32] = [0xa1; 32];
const STALENESS_LIMIT: u64 = 5_000;
const WASM: &[u8] = &[
    0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3, 3, 2, 0,
    1, 5, 3, 1, 0, 1, 7, 34, 3, 4, b'c', b'a', b'l', b'l', 0, 0, 14, b'l', b'a', b'y', b'e', b'r',
    b'x', b'_', b'r', b'e', b's', b'e', b'r', b'v', b'e', 0, 1, 6, b'm', b'e', b'm', b'o', b'r',
    b'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
];

fn key(prefix: &[u8], program: ProgramId) -> Vec<u8> {
    let mut key = prefix.to_vec();
    key.extend_from_slice(&program.bytes());
    chain_evidence::programs_state_key(&key)
}

fn fixture() -> (
    chain::ChainFixture,
    ProgramStateBundle,
    Vec<u8>,
    SimpleProof,
    [u8; 32],
) {
    let program = lint_success(ProgramId::new([0x31; 32]), "required test operation");
    let interface = lint_success(
        ProgramInterface::bind(
            WASM,
            1,
            vec![InterfaceEntryPoint {
                name: "call".to_owned(),
                discriminator: [1, 2, 3, 4],
                calldata: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
                response: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
                capabilities: Vec::new(),
                event_topics: Vec::new(),
                failures: Vec::new(),
            }],
        ),
        "required test operation",
    );
    let code_hash = interface.code_hash();
    let mut record = vec![0; 33];
    record.extend_from_slice(&code_hash);
    record.extend_from_slice(&1_u16.to_be_bytes());
    record.extend_from_slice(&7_u32.to_be_bytes());
    let program_key = key(b"program\0", program);
    let interface_key = chain_evidence::programs_state_key(&interface_state_key(program));
    let mut fixture = lint_success(
        chain::ChainFixture::new(
            "agent-chain-head",
            &SEEDS,
            vec![
                (program_key.clone(), record),
                (
                    interface_key.clone(),
                    lint_success(
                        interface_state_value(program, 7, &interface),
                        "required test operation",
                    ),
                ),
            ],
        ),
        "required test operation",
    );
    let transaction = b"validator committed program transaction".to_vec();
    let transaction_hash: [u8; 32] = Sha256::digest(&transaction).into();
    let transaction_root = hash_from_byte_slices(&[&transaction_hash]);
    fixture.header.data_hash = transaction_root.to_vec();
    fixture.commit.block_id.hash =
        lint_success(fixture.header.hash(), "required test operation").to_vec();
    let keys: Vec<_> = SEEDS.iter().map(SigningKey::from_bytes).collect();
    for signature in &mut fixture.commit.signatures {
        let signer = lint_present(
            keys.iter().find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            }),
            "required test operation",
        );
        let bytes = lint_success(
            vote_sign_bytes(
                &fixture.header.chain_id,
                fixture.commit.height,
                fixture.commit.round,
                Some(&fixture.commit.block_id),
                signature.timestamp,
            ),
            "required test operation",
        );
        signature.signature = signer.sign(&bytes).to_bytes().to_vec();
    }
    let commitment = lint_success(
        decode_commitment_proof(&lint_success(
            fixture.encode_key(&program_key),
            "required test operation",
        )),
        "required test operation",
    );
    let bundle = ProgramStateBundle {
        commitment,
        program,
        program_record_ops: lint_success(fixture.prove(&program_key), "required test operation"),
        interface_ops: lint_success(fixture.prove(&interface_key), "required test operation"),
        wind_down: Some(ChainWindDownProof {
            program,
            value: None,
            ops: lint_success(
                fixture.prove_absent(&key(b"wind-down\0s", program)),
                "required test operation",
            ),
        }),
    };
    let proof = SimpleProof {
        total: 1,
        index: 0,
        leaf_hash: transaction_root,
        aunts: Vec::new(),
    };
    (fixture, bundle, transaction, proof, code_hash)
}

struct HeadServer {
    endpoint: String,
    document: Arc<Mutex<Value>>,
    checkpoint_reads: Arc<Mutex<Vec<(Value, Value)>>>,
    stopping: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}

impl HeadServer {
    fn serve(document: Value, path: String, authorization: &'static str) -> Self {
        let listener = lint_success(TcpListener::bind("127.0.0.1:0"), "required test operation");
        let endpoint = format!(
            "http://{}",
            lint_success(listener.local_addr(), "required test operation")
        );
        lint_success(listener.set_nonblocking(true), "required test operation");
        let document = Arc::new(Mutex::new(document));
        let checkpoint_reads = Arc::new(Mutex::new(Vec::<(Value, Value)>::new()));
        let stopping = Arc::new(AtomicBool::new(false));
        let served = Arc::clone(&document);
        let checkpoint = Arc::clone(&checkpoint_reads);
        let stop = Arc::clone(&stopping);
        let thread = thread::spawn(move || {
            while !stop.load(Ordering::SeqCst) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(5));
                        continue;
                    }
                    Err(error) => panic!("head listener: {error}"),
                };
                lint_success(
                    stream.set_read_timeout(Some(Duration::from_secs(5))),
                    "required test operation",
                );
                lint_success(
                    stream.set_write_timeout(Some(Duration::from_secs(5))),
                    "required test operation",
                );
                let mut reader = BufReader::new(&stream);
                let mut line = String::new();
                lint_success(reader.read_line(&mut line), "required test operation");
                let request_line = line.trim_end().to_owned();
                let mut authenticated = false;
                let mut content_length = 0;
                loop {
                    line.clear();
                    assert_ne!(
                        lint_success(reader.read_line(&mut line), "required test operation"),
                        0
                    );
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("authorization") {
                            assert_eq!(value.trim(), format!("Bearer {authorization}"));
                            authenticated = true;
                        }
                        if name.eq_ignore_ascii_case("content-length") {
                            content_length = lint_success(
                                value.trim().parse::<usize>(),
                                "required test operation",
                            );
                        }
                    }
                }
                let body = if request_line == "POST / HTTP/1.1" {
                    let mut body = vec![0; content_length];
                    lint_success(reader.read_exact(&mut body), "required test operation");
                    let request: Value =
                        lint_success(serde_json::from_slice(&body), "required test operation");
                    assert_eq!(request["jsonrpc"], "2.0");
                    let reads = lint_success(checkpoint.lock(), "required test operation");
                    let key = json!([request["method"], request["params"]]);
                    let (_, result) = reads
                        .iter()
                        .find(|(expected, _)| expected == &key)
                        .unwrap_or_else(|| panic!("unexpected checkpoint read: {request}"));
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string()
                } else {
                    assert_eq!(request_line, format!("GET {path} HTTP/1.1"));
                    assert!(authenticated);
                    lint_success(served.lock(), "required test operation").to_string()
                };
                lint_success(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()), "required test operation");
            }
        });
        Self {
            endpoint,
            document,
            checkpoint_reads,
            stopping,
            thread: Some(thread),
        }
    }

    fn set(&self, document: Value) {
        *lint_success(self.document.lock(), "required test operation") = document;
    }

    fn withhold_checkpoint(&self, height: i64) {
        let mut reads = lint_success(self.checkpoint_reads.lock(), "required test operation");
        let (_, result) = lint_present(
            reads
                .iter_mut()
                .find(|(request, _)| request == &json!(["px_getCommit", [height.to_string()]])),
            "required test operation",
        );
        *result = Value::Null;
    }

    fn set_checkpoint(&self, fixture: &deployment_support::ProtocolFixture, timestamp: &str) {
        let chain = &fixture.chain;
        let header = &chain.header;
        let commit = &chain.commit;
        let height = header.height;
        let block = json!({
            "number": format!("{height:#x}"),
            "hash": format!("0x{}", hex::encode(lint_success(header.hash(), "required test operation"))),
            "parentHash": format!("0x{}", hex::encode(&header.last_block_id.hash)),
            "timestamp": format!("{:#x}", header.time.seconds),
            "transactionsRoot": format!("0x{}", hex::encode(&header.data_hash)),
            "receiptsRoot": format!("0x{}", hex::encode(&header.last_results_hash)),
            "stateRoot": format!("0x{}", hex::encode(&header.app_hash)),
            "transactions": []
        });
        let material = json!({
            "height": height.to_string(),
            "signedHeader": {
                "header": {
                    "version": {"block": header.version.block.to_string(), "app": header.version.app.to_string()},
                    "chain_id": header.chain_id,
                    "height": height.to_string(),
                    "time": timestamp,
                    "last_block_id": {
                        "hash": hex::encode(&header.last_block_id.hash),
                        "parts": {"total": header.last_block_id.part_set_total, "hash": hex::encode(&header.last_block_id.part_set_hash)}
                    },
                    "last_commit_hash": hex::encode(&header.last_commit_hash),
                    "data_hash": hex::encode(&header.data_hash),
                    "validators_hash": hex::encode(&header.validators_hash),
                    "next_validators_hash": hex::encode(&header.next_validators_hash),
                    "consensus_hash": hex::encode(&header.consensus_hash),
                    "app_hash": hex::encode(&header.app_hash),
                    "last_results_hash": hex::encode(&header.last_results_hash),
                    "evidence_hash": hex::encode(&header.evidence_hash),
                    "proposer_address": hex::encode(&header.proposer_address)
                },
                "commit": {
                    "height": commit.height.to_string(),
                    "round": commit.round,
                    "block_id": {
                        "hash": hex::encode(&commit.block_id.hash),
                        "parts": {"total": commit.block_id.part_set_header.total, "hash": hex::encode(&commit.block_id.part_set_header.hash)}
                    },
                    "signatures": commit.signatures.iter().map(|signature| json!({
                        "block_id_flag": 2,
                        "validator_address": hex::encode(&signature.validator_address),
                        "timestamp": timestamp,
                        "signature": openssl::base64::encode_block(&signature.signature)
                    })).collect::<Vec<_>>()
                }
            }
        });
        let validators = chain.verifier.validators.validators();
        let validator_set = json!({
            "height": height.to_string(),
            "total": validators.len().to_string(),
            "validators": validators.iter().map(|validator| json!({
                "address": hex::encode(validator.address),
                "pubKey": {"type": "tendermint/PubKeyEd25519", "value": openssl::base64::encode_block(&validator.public_key)},
                "votingPower": validator.voting_power.to_string()
            })).collect::<Vec<_>>()
        });
        *lint_success(self.checkpoint_reads.lock(), "required test operation") = vec![
            (
                json!(["eth_getBlockByNumber", [format!("{height:#x}"), true]]),
                block.clone(),
            ),
            (
                json!(["eth_getBlockByNumber", [format!("{height:#x}"), false]]),
                block,
            ),
            (json!(["px_getCommit", [height.to_string()]]), material),
            (
                json!(["px_getValidators", [height.to_string()]]),
                validator_set,
            ),
        ];
    }
}

impl Drop for HeadServer {
    fn drop(&mut self) {
        self.stopping.store(true, Ordering::SeqCst);
        lint_success(
            lint_present(self.thread.take(), "required test operation").join(),
            "required test operation",
        );
    }
}

fn ca_der() -> Vec<u8> {
    let key = lint_success(
        PKey::from_rsa(lint_success(Rsa::generate(2048), "required test operation")),
        "required test operation",
    );
    let mut name = lint_success(X509NameBuilder::new(), "required test operation");
    lint_success(
        name.append_entry_by_text("CN", "Chain head test CA"),
        "required test operation",
    );
    let name = name.build();
    let mut cert = lint_success(X509::builder(), "required test operation");
    lint_success(cert.set_version(2), "required test operation");
    lint_success(cert.set_subject_name(&name), "required test operation");
    lint_success(cert.set_issuer_name(&name), "required test operation");
    lint_success(cert.set_pubkey(&key), "required test operation");
    let not_before = lint_success(Asn1Time::days_from_now(0), "required test operation");
    let not_after = lint_success(Asn1Time::days_from_now(1), "required test operation");
    lint_success(cert.set_not_before(&not_before), "required test operation");
    lint_success(cert.set_not_after(&not_after), "required test operation");
    lint_success(
        cert.sign(&key, MessageDigest::sha256()),
        "required test operation",
    );
    lint_success(cert.build().to_der(), "required test operation")
}

fn connect(
    node: &HeadServer,
    authority: &HeadServer,
    verifier: ProtocolDeploymentVerifier,
    ca_der: &[u8],
) -> LayerxdProgramBalanceReader {
    lint_success(
        LayerxdProgramBalanceReader::connect(
            &node.endpoint,
            "node-read".to_owned(),
            ProgramAuthority {
                endpoint: &authority.endpoint,
                authorization: "authority-read".to_owned(),
                replica_id: REPLICA,
                ca_der,
            },
            verifier,
            Registry::new(),
        ),
        "required test operation",
    )
}

#[test]
fn chain_head_read_verifies_the_validator_signed_commitment() {
    let (fixture, bundle, transaction, proof, code_hash) = fixture();
    let now = lint_success(
        u64::try_from(fixture.header.time.seconds),
        "required test operation",
    ) * 1_000
        + lint_success(
            u64::try_from(fixture.header.time.nanos),
            "required test operation",
        ) / 1_000_000;
    let height = lint_success(
        u64::try_from(fixture.header.height),
        "required test operation",
    );
    let transaction_hash: [u8; 32] = Sha256::digest(&transaction).into();
    let document = json!({
        "current": true,
        "receipt_digest": hex::encode(transaction_hash),
        "state_root": hex::encode(&fixture.header.app_hash),
        "observed_sequence": height,
        "observed_at": now,
        "chain_evidence": {
            "commitment_hex": hex::encode(lint_success(bundle.commitment.encode(), "required test operation")),
            "transaction_hex": hex::encode(&transaction),
            "transaction_proof": {
                "total": proof.total, "index": proof.index,
                "leaf_hash": hex::encode(proof.leaf_hash), "aunts": []
            }
        }
    });
    let mut authority_document = document.clone();
    authority_document["authority_replica_id"] = json!(hex::encode(REPLICA));
    let node = HeadServer::serve(
        document.clone(),
        "/v1/protocol/account-state/head".to_owned(),
        "node-read",
    );
    let authority = HeadServer::serve(
        authority_document.clone(),
        format!("/v1/protocol/account-state/head?height={height}"),
        "authority-read",
    );
    let verifier = lint_success(
        ProtocolDeploymentVerifier::from_chain_trust(
            &fixture.header.chain_id,
            fixture.verifier.validators.validators().to_vec(),
            STALENESS_LIMIT,
        ),
        "required test operation",
    );
    let trust_der = ca_der();
    let reader = connect(&node, &authority, verifier, &trust_der);
    let head = lint_success(reader.read_chain_head(now), "required test operation");
    assert_eq!(head.receipt_digest(), transaction_hash);
    assert_eq!(head.state_root().as_slice(), fixture.header.app_hash);
    assert_eq!(head.freshness().observed_sequence, height);
    assert_eq!(head.freshness().observed_at, now);
    let encoded_bundle = lint_success(bundle.canonical_encoding(), "required test operation");
    let program_head = lint_success(
        reader.verify_program_bundle(&encoded_bundle, &head, bundle.program, now),
        "required test operation",
    );
    assert_eq!(program_head.program_head().version(), 7);
    assert_eq!(program_head.program_head().code_hash(), code_hash);

    let other_program = lint_success(ProgramId::new([0x32; 32]), "required test operation");
    assert!(matches!(
        reader.verify_program_bundle(&encoded_bundle, &head, other_program, now),
        Err(ProgramBundleError::ProgramMismatch)
    ));

    assert_chain_head_refusals(
        &fixture,
        &transaction,
        &node,
        &authority,
        &reader,
        (document, authority_document),
        (now, &trust_der),
    );
}

fn journal_head_document(fixture: &deployment_support::ProtocolFixture) -> Value {
    let chain = &fixture.chain;
    let transaction = &fixture.proof.state.transaction;
    let proof = &fixture.proof.state.transaction_proof;
    let now = lint_success(
        u64::try_from(chain.header.time.seconds),
        "required test operation",
    ) * 1_000
        + lint_success(
            u64::try_from(chain.header.time.nanos),
            "required test operation",
        ) / 1_000_000;
    json!({
        "current": true,
        "receipt_digest": hex::encode(Sha256::digest(transaction)),
        "state_root": hex::encode(&chain.header.app_hash),
        "observed_sequence": chain.header.height,
        "observed_at": now,
        "chain_evidence": {
            "commitment_hex": hex::encode(lint_success(chain.encode(), "required test operation")),
            "transaction_hex": hex::encode(transaction),
            "transaction_proof": {
                "total": proof.total, "index": proof.index,
                "leaf_hash": hex::encode(proof.leaf_hash),
                "aunts": proof.aunts.iter().map(hex::encode).collect::<Vec<_>>()
            }
        }
    })
}

fn sign_journal_head(fixture: &mut deployment_support::ProtocolFixture, now: u64) {
    let chain = &mut fixture.chain;
    chain.header.time.seconds = lint_success(i64::try_from(now / 1_000), "required test operation");
    chain.header.time.nanos = lint_success(
        i32::try_from(now % 1_000 * 1_000_000),
        "required test operation",
    );
    chain.commit.height = chain.header.height;
    chain.commit.block_id.hash =
        lint_success(chain.header.hash(), "required test operation").to_vec();
    let keys: Vec<_> = SEEDS.iter().map(SigningKey::from_bytes).collect();
    for signature in &mut chain.commit.signatures {
        let signer = lint_present(
            keys.iter().find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            }),
            "required test operation",
        );
        signature.timestamp = Timestamp {
            seconds: chain.header.time.seconds,
            nanos: chain.header.time.nanos,
        };
        let bytes = lint_success(
            vote_sign_bytes(
                &chain.header.chain_id,
                chain.commit.height,
                chain.commit.round,
                Some(&chain.commit.block_id),
                signature.timestamp,
            ),
            "required test operation",
        );
        signature.signature = signer.sign(&bytes).to_bytes().to_vec();
    }
}

#[test]
fn journal_admission_verifies_through_the_current_chain_head() {
    chain_head_read_verifies_the_validator_signed_commitment();

    let admission_time = deployment_support::NOW;
    let mut fixture = deployment_support::deploy_fixture(
        deployment_support::WASM_V1,
        layerx_programs_runtime::UpgradePolicy::Authority(deployment_support::AUTHORITY),
        41,
        admission_time,
    );
    let verifier = deployment_support::verifier_for_fixture(&fixture, STALENESS_LIMIT);
    let admission = lint_success(
        verifier.verify_deployment(&fixture.proof, admission_time),
        "required test operation",
    );
    let ca_der = ca_der();

    let document = journal_head_document(&fixture);
    let mut authority_document = document.clone();
    authority_document["authority_replica_id"] = json!(hex::encode(REPLICA));
    let node = HeadServer::serve(
        document,
        "/v1/protocol/account-state/head".to_owned(),
        "node-read",
    );
    let authority = HeadServer::serve(
        authority_document,
        format!(
            "/v1/protocol/account-state/head?height={}",
            fixture.chain.header.height
        ),
        "authority-read",
    );
    let reader = connect(&node, &authority, verifier.clone(), &ca_der);
    assert_eq!(
        lint_success(
            reader.verify_journal_admission(&fixture.proof, admission_time),
            "required test operation"
        )
        .record(),
        admission.record()
    );
    assert_eq!(
        lint_error(
            reader.verify_journal_admission(&fixture.proof, 0),
            "test operation must fail"
        ),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );
    drop(reader);
    drop(authority);

    fixture.chain.header.height += 1;
    sign_journal_head(&mut fixture, admission_time + 1_000);
    assert_eq!(fixture.chain.header.height, 43);
    assert_eq!(fixture.chain.header.time.seconds, 1_700_000_151);
    assert_eq!(fixture.chain.header.time.nanos, 0);
    node.set_checkpoint(&fixture, "2023-11-14T22:15:51Z");
    fixture.chain.header.height += 1;
    let now = admission_time + STALENESS_LIMIT + 1;
    sign_journal_head(&mut fixture, now);
    let document = journal_head_document(&fixture);
    let mut authority_document = document.clone();
    authority_document["authority_replica_id"] = json!(hex::encode(REPLICA));
    node.set(document.clone());
    let authority = HeadServer::serve(
        authority_document.clone(),
        format!(
            "/v1/protocol/account-state/head?height={}",
            fixture.chain.header.height
        ),
        "authority-read",
    );
    let reader = connect(&node, &authority, verifier.clone(), &ca_der);
    assert_eq!(
        verifier.verify_deployment(&fixture.proof, now),
        Err(ProtocolEvidenceError::Stale)
    );
    let replayed = lint_success(
        reader.verify_journal_admission(&fixture.proof, now),
        "required test operation",
    );
    assert_eq!(replayed.record(), admission.record());
    let mut registry = Registry::new();
    lint_success(
        registry.record_verified_deployment(&replayed),
        "required test operation",
    );
    assert_eq!(registry.latest_version(replayed.program()), Ok(1));

    assert_journal_head_refusals(
        &mut fixture,
        &node,
        &authority,
        &reader,
        (authority_document, admission_time, now),
        &admission,
    );
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}

#[track_caller]
fn lint_present<T>(value: Option<T>, context: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{context}: required value is absent"),
    }
}

#[track_caller]
fn lint_error<T: std::fmt::Debug, E>(result: Result<T, E>, context: &str) -> E {
    match result {
        Err(error) => error,
        Ok(value) => panic!("{context}: unexpected success {value:?}"),
    }
}

fn assert_chain_head_refusals(
    fixture: &chain::ChainFixture,
    transaction: &[u8],
    node: &HeadServer,
    authority: &HeadServer,
    reader: &LayerxdProgramBalanceReader,
    (document, authority_document): (Value, Value),
    (now, trust_der): (u64, &[u8]),
) {
    let mut altered_transaction = transaction.to_vec();
    altered_transaction[0] ^= 1;
    let mut altered_authority = authority_document.clone();
    altered_authority["chain_evidence"]["transaction_hex"] =
        json!(hex::encode(&altered_transaction));
    authority.set(altered_authority.clone());
    assert_eq!(
        lint_error(reader.read_chain_head(now), "test operation must fail"),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );
    let mut altered_node = document.clone();
    altered_node["chain_evidence"]["transaction_hex"] = json!(hex::encode(&altered_transaction));
    node.set(altered_node);
    assert!(matches!(
        reader.read_chain_head(now),
        Err(ChainHeadError::Evidence(_))
    ));
    node.set(document.clone());
    authority.set(authority_document.clone());

    let foreign_validators = [11_u8, 12, 13, 14]
        .into_iter()
        .map(|seed| {
            let key = SigningKey::from_bytes(&[seed; 32]);
            TrustedValidator::new(key.verifying_key().to_bytes(), 1)
        })
        .collect();
    let foreign_verifier = lint_success(
        ProtocolDeploymentVerifier::from_chain_trust(
            &fixture.header.chain_id,
            foreign_validators,
            STALENESS_LIMIT,
        ),
        "required test operation",
    );
    let foreign_reader = connect(node, authority, foreign_verifier, trust_der);
    assert!(matches!(
        foreign_reader.read_chain_head(now),
        Err(ChainHeadError::Evidence(_))
    ));
    assert!(lint_error(
        reader.read_chain_head(now + STALENESS_LIMIT + 1),
        "test operation must fail"
    )
    .is_stale());

    let mut wrong_replica = authority_document.clone();
    wrong_replica["authority_replica_id"] = json!(hex::encode([0xa2; 32]));
    authority.set(wrong_replica);
    assert_eq!(
        lint_error(reader.read_chain_head(now), "test operation must fail"),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );
    authority.set(authority_document);
    for (field, replacement) in [
        ("receipt_digest", json!(hex::encode([0x55; 32]))),
        ("state_root", json!(hex::encode([0x55; 32]))),
        ("observed_at", json!(now + 1)),
        ("current", json!(false)),
    ] {
        let mut changed = document.clone();
        changed[field] = replacement;
        node.set(changed);
        assert_eq!(
            lint_error(reader.read_chain_head(now), "test operation must fail"),
            ChainHeadError::View(ProgramReadError::NonCanonicalView),
            "{field}"
        );
    }
    let mut kernel_document = document;
    lint_present(kernel_document.as_object_mut(), "required test operation")
        .remove("chain_evidence");
    kernel_document["batch_evidence"] = json!({});
    node.set(kernel_document);
    assert_eq!(
        lint_error(reader.read_chain_head(now), "test operation must fail"),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );
    assert_eq!(
        lint_error(reader.read_chain_head(0), "test operation must fail"),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );
}

fn assert_journal_head_refusals(
    fixture: &mut deployment_support::ProtocolFixture,
    node: &HeadServer,
    authority: &HeadServer,
    reader: &LayerxdProgramBalanceReader,
    (authority_document, admission_time, now): (Value, u64, u64),
    admission: &layerx_programs::VerifiedDeploymentEvidence,
) {
    let mut disagreement = authority_document.clone();
    disagreement["chain_evidence"]["transaction_hex"] =
        json!(hex::encode(b"different transaction"));
    authority.set(disagreement);
    assert_eq!(
        lint_error(
            reader.verify_journal_admission(&fixture.proof, now),
            "test operation must fail"
        ),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );

    fixture.chain.commit.signatures[0].signature[0] ^= 1;
    let forged = journal_head_document(fixture);
    let mut forged_authority = forged.clone();
    forged_authority["authority_replica_id"] = json!(hex::encode(REPLICA));
    node.set(forged);
    authority.set(forged_authority);
    assert_eq!(
        lint_error(
            reader.verify_journal_admission(&fixture.proof, now),
            "test operation must fail"
        ),
        ChainHeadError::Evidence(ProtocolEvidenceError::HistoricalTrustAnchor)
    );

    let historical_now = admission_time + 14 * 24 * 60 * 60 * 1_000 + 1;
    assert_eq!(fixture.chain.header.height, 44);
    sign_journal_head(fixture, historical_now);
    let historical = journal_head_document(fixture);
    let mut historical_authority = historical.clone();
    historical_authority["authority_replica_id"] = json!(hex::encode(REPLICA));
    node.set(historical);
    authority.set(historical_authority);
    assert_eq!(
        lint_success(
            reader.verify_journal_admission(&fixture.proof, historical_now),
            "required test operation"
        )
        .record(),
        admission.record()
    );

    node.withhold_checkpoint(43);
    assert_eq!(
        lint_error(
            reader.verify_journal_admission(&fixture.proof, historical_now),
            "test operation must fail"
        ),
        ChainHeadError::Evidence(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
}

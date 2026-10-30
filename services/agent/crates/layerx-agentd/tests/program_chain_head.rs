use std::io::{BufRead, BufReader, Write};
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

#[allow(dead_code)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/chain.rs"]
mod chain;

#[allow(dead_code)]
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
    let program = ProgramId::new([0x31; 32]).unwrap();
    let interface = ProgramInterface::bind(
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
    )
    .unwrap();
    let code_hash = interface.code_hash();
    let mut record = vec![0; 33];
    record.extend_from_slice(&code_hash);
    record.extend_from_slice(&1_u16.to_be_bytes());
    record.extend_from_slice(&7_u32.to_be_bytes());
    let program_key = key(b"program\0", program);
    let interface_key = chain_evidence::programs_state_key(&interface_state_key(program));
    let mut fixture = chain::ChainFixture::new(
        "agent-chain-head",
        &SEEDS,
        vec![
            (program_key.clone(), record),
            (
                interface_key.clone(),
                interface_state_value(program, 7, &interface).unwrap(),
            ),
        ],
    )
    .unwrap();
    let transaction = b"validator committed program transaction".to_vec();
    let transaction_hash: [u8; 32] = Sha256::digest(&transaction).into();
    let transaction_root = hash_from_byte_slices(&[&transaction_hash]);
    fixture.header.data_hash = transaction_root.to_vec();
    fixture.commit.block_id.hash = fixture.header.hash().unwrap().to_vec();
    let keys: Vec<_> = SEEDS.iter().map(SigningKey::from_bytes).collect();
    for signature in &mut fixture.commit.signatures {
        let signer = keys
            .iter()
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .unwrap();
        let bytes = vote_sign_bytes(
            &fixture.header.chain_id,
            fixture.commit.height,
            fixture.commit.round,
            Some(&fixture.commit.block_id),
            signature.timestamp,
        )
        .unwrap();
        signature.signature = signer.sign(&bytes).to_bytes().to_vec();
    }
    let commitment = decode_commitment_proof(&fixture.encode_key(&program_key).unwrap()).unwrap();
    let bundle = ProgramStateBundle {
        commitment,
        program,
        program_record_ops: fixture.prove(&program_key).unwrap(),
        interface_ops: fixture.prove(&interface_key).unwrap(),
        wind_down: Some(ChainWindDownProof {
            program,
            value: None,
            ops: fixture
                .prove_absent(&key(b"wind-down\0s", program))
                .unwrap(),
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
    stopping: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}

impl HeadServer {
    fn serve(document: Value, path: String, authorization: &'static str) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let endpoint = format!("http://{}", listener.local_addr().unwrap());
        listener.set_nonblocking(true).unwrap();
        let document = Arc::new(Mutex::new(document));
        let stopping = Arc::new(AtomicBool::new(false));
        let served = Arc::clone(&document);
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
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut reader = BufReader::new(&stream);
                let mut line = String::new();
                reader.read_line(&mut line).unwrap();
                assert_eq!(line.trim_end(), format!("GET {path} HTTP/1.1"));
                let mut authenticated = false;
                loop {
                    line.clear();
                    assert_ne!(reader.read_line(&mut line).unwrap(), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("authorization") {
                            assert_eq!(value.trim(), format!("Bearer {authorization}"));
                            authenticated = true;
                        }
                    }
                }
                assert!(authenticated);
                let body = served.lock().unwrap().to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap();
            }
        });
        Self {
            endpoint,
            document,
            stopping,
            thread: Some(thread),
        }
    }

    fn set(&self, document: Value) {
        *self.document.lock().unwrap() = document;
    }
}

impl Drop for HeadServer {
    fn drop(&mut self) {
        self.stopping.store(true, Ordering::SeqCst);
        self.thread.take().unwrap().join().unwrap();
    }
}

fn ca_der() -> Vec<u8> {
    let key = PKey::from_rsa(Rsa::generate(2048).unwrap()).unwrap();
    let mut name = X509NameBuilder::new().unwrap();
    name.append_entry_by_text("CN", "Chain head test CA")
        .unwrap();
    let name = name.build();
    let mut cert = X509::builder().unwrap();
    cert.set_version(2).unwrap();
    cert.set_subject_name(&name).unwrap();
    cert.set_issuer_name(&name).unwrap();
    cert.set_pubkey(&key).unwrap();
    cert.set_not_before(&Asn1Time::days_from_now(0).unwrap())
        .unwrap();
    cert.set_not_after(&Asn1Time::days_from_now(1).unwrap())
        .unwrap();
    cert.sign(&key, MessageDigest::sha256()).unwrap();
    cert.build().to_der().unwrap()
}

fn connect(
    node: &HeadServer,
    authority: &HeadServer,
    verifier: ProtocolDeploymentVerifier,
    ca_der: &[u8],
) -> LayerxdProgramBalanceReader {
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
    )
    .unwrap()
}

#[test]
fn chain_head_read_verifies_the_validator_signed_commitment() {
    let (fixture, bundle, transaction, proof, code_hash) = fixture();
    let now = u64::try_from(fixture.header.time.seconds).unwrap() * 1_000
        + u64::try_from(fixture.header.time.nanos).unwrap() / 1_000_000;
    let height = u64::try_from(fixture.header.height).unwrap();
    let transaction_hash: [u8; 32] = Sha256::digest(&transaction).into();
    let document = json!({
        "current": true,
        "receipt_digest": hex::encode(transaction_hash),
        "state_root": hex::encode(&fixture.header.app_hash),
        "observed_sequence": height,
        "observed_at": now,
        "chain_evidence": {
            "commitment_hex": hex::encode(bundle.commitment.encode().unwrap()),
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
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.header.chain_id,
        fixture.verifier.validators.validators().to_vec(),
        STALENESS_LIMIT,
    )
    .unwrap();
    let ca_der = ca_der();
    let reader = connect(&node, &authority, verifier, &ca_der);
    let head = reader.read_chain_head(now).unwrap();
    assert_eq!(head.receipt_digest(), transaction_hash);
    assert_eq!(head.state_root().as_slice(), fixture.header.app_hash);
    assert_eq!(head.freshness().observed_sequence, height);
    assert_eq!(head.freshness().observed_at, now);
    let encoded_bundle = bundle.canonical_encoding().unwrap();
    let verified = reader
        .verify_program_bundle(&encoded_bundle, &head, bundle.program, now)
        .unwrap();
    assert_eq!(verified.program_head().version(), 7);
    assert_eq!(verified.program_head().code_hash(), code_hash);

    let other_program = ProgramId::new([0x32; 32]).unwrap();
    assert!(matches!(
        reader.verify_program_bundle(&encoded_bundle, &head, other_program, now),
        Err(ProgramBundleError::ProgramMismatch)
    ));

    let mut altered_transaction = transaction.clone();
    altered_transaction[0] ^= 1;
    let mut altered_authority = authority_document.clone();
    altered_authority["chain_evidence"]["transaction_hex"] =
        json!(hex::encode(&altered_transaction));
    authority.set(altered_authority.clone());
    assert_eq!(
        reader.read_chain_head(now).unwrap_err(),
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
    let foreign_verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.header.chain_id,
        foreign_validators,
        STALENESS_LIMIT,
    )
    .unwrap();
    let foreign_reader = connect(&node, &authority, foreign_verifier, &ca_der);
    assert!(matches!(
        foreign_reader.read_chain_head(now),
        Err(ChainHeadError::Evidence(_))
    ));
    assert!(reader
        .read_chain_head(now + STALENESS_LIMIT + 1)
        .unwrap_err()
        .is_stale());

    let mut wrong_replica = authority_document.clone();
    wrong_replica["authority_replica_id"] = json!(hex::encode([0xa2; 32]));
    authority.set(wrong_replica);
    assert_eq!(
        reader.read_chain_head(now).unwrap_err(),
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
            reader.read_chain_head(now).unwrap_err(),
            ChainHeadError::View(ProgramReadError::NonCanonicalView),
            "{field}"
        );
    }
    let mut kernel_document = document;
    kernel_document
        .as_object_mut()
        .unwrap()
        .remove("chain_evidence");
    kernel_document["batch_evidence"] = json!({});
    node.set(kernel_document);
    assert_eq!(
        reader.read_chain_head(now).unwrap_err(),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );
    assert_eq!(
        reader.read_chain_head(0).unwrap_err(),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );
}

fn journal_head_document(fixture: &deployment_support::ProtocolFixture) -> Value {
    let chain = &fixture.chain;
    let transaction = &fixture.proof.state.transaction;
    let proof = &fixture.proof.state.transaction_proof;
    let now = u64::try_from(chain.header.time.seconds).unwrap() * 1_000
        + u64::try_from(chain.header.time.nanos).unwrap() / 1_000_000;
    json!({
        "current": true,
        "receipt_digest": hex::encode(Sha256::digest(transaction)),
        "state_root": hex::encode(&chain.header.app_hash),
        "observed_sequence": chain.header.height,
        "observed_at": now,
        "chain_evidence": {
            "commitment_hex": hex::encode(chain.encode().unwrap()),
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
    chain.header.time.seconds = i64::try_from(now / 1_000).unwrap();
    chain.header.time.nanos = i32::try_from(now % 1_000 * 1_000_000).unwrap();
    chain.commit.height = chain.header.height;
    chain.commit.block_id.hash = chain.header.hash().unwrap().to_vec();
    let keys: Vec<_> = SEEDS.iter().map(SigningKey::from_bytes).collect();
    for signature in &mut chain.commit.signatures {
        let signer = keys
            .iter()
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .unwrap();
        signature.timestamp = Timestamp {
            seconds: chain.header.time.seconds,
            nanos: chain.header.time.nanos,
        };
        let bytes = vote_sign_bytes(
            &chain.header.chain_id,
            chain.commit.height,
            chain.commit.round,
            Some(&chain.commit.block_id),
            signature.timestamp,
        )
        .unwrap();
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
    let admission = verifier
        .verify_deployment(&fixture.proof, admission_time)
        .unwrap();
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
        reader
            .verify_journal_admission(&fixture.proof, admission_time)
            .unwrap()
            .record(),
        admission.record()
    );
    assert_eq!(
        reader
            .verify_journal_admission(&fixture.proof, 0)
            .unwrap_err(),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );
    drop(reader);
    drop(authority);

    fixture.chain.header.height += 2;
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
    let replayed = reader
        .verify_journal_admission(&fixture.proof, now)
        .unwrap();
    assert_eq!(replayed.record(), admission.record());
    let mut registry = Registry::new();
    registry.record_verified_deployment(&replayed).unwrap();
    assert_eq!(registry.latest_version(replayed.program()), Ok(1));

    let mut disagreement = authority_document.clone();
    disagreement["chain_evidence"]["transaction_hex"] =
        json!(hex::encode(b"different transaction"));
    authority.set(disagreement);
    assert_eq!(
        reader
            .verify_journal_admission(&fixture.proof, now)
            .unwrap_err(),
        ChainHeadError::View(ProgramReadError::NonCanonicalView)
    );

    fixture.chain.commit.signatures[0].signature[0] ^= 1;
    let forged = journal_head_document(&fixture);
    let mut forged_authority = forged.clone();
    forged_authority["authority_replica_id"] = json!(hex::encode(REPLICA));
    node.set(forged);
    authority.set(forged_authority);
    assert_eq!(
        reader
            .verify_journal_admission(&fixture.proof, now)
            .unwrap_err(),
        ChainHeadError::Evidence(ProtocolEvidenceError::ChainProof)
    );

    let expired_now = admission_time + 14 * 24 * 60 * 60 * 1_000 + 1;
    sign_journal_head(&mut fixture, expired_now);
    let expired = journal_head_document(&fixture);
    let mut expired_authority = expired.clone();
    expired_authority["authority_replica_id"] = json!(hex::encode(REPLICA));
    node.set(expired);
    authority.set(expired_authority);
    assert_eq!(
        reader
            .verify_journal_admission(&fixture.proof, expired_now)
            .unwrap_err(),
        ChainHeadError::Evidence(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
}

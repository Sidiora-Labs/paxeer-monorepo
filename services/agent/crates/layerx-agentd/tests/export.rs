use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::export::{FactRef, OfflineExport};
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::ProofBundle;
use layerx_agentd::export::{
    build, decode_header, decode_validators, encode_commit, encode_header,
    encode_storage_witnesses, encode_validators, verify, ExportTrustSource,
};
use layerx_agentd::read::{
    validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator,
    CommitVote,
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_types::rung::Rung;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, MAX_SEQUENCER_EFFECTS, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

pub fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("export fixture: {error:?}"))
}

pub fn canonical(bytes: Vec<u8>) -> CanonicalBytes {
    must(CanonicalBytes::new(bytes))
}

pub fn rlp(payload: &[u8], list: bool) -> Vec<u8> {
    if !list && payload.len() == 1 && payload[0] < 128 {
        return payload.to_vec();
    }
    let prefix = if list { 0xc0 } else { 0x80 };
    let mut out = if payload.len() < 56 {
        vec![prefix + payload.len() as u8]
    } else {
        let length = payload.len().to_be_bytes();
        let length = &length[length.iter().position(|byte| *byte != 0).unwrap_or(0)..];
        [&[prefix + 55 + length.len() as u8][..], length].concat()
    };
    out.extend_from_slice(payload);
    out
}

pub fn storage_vector() -> Value {
    let vector: Value = must(serde_json::from_str(include_str!(
        "../../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
    )));
    vector["entries"][0].clone()
}

pub fn storage_node(
    answers: Vec<(&'static str, Value, Value)>,
) -> (SequencerEndpoint, std::thread::JoinHandle<()>) {
    use std::io::{BufRead, BufReader, Read, Write};
    use std::net::TcpListener;
    use std::time::Duration;
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let endpoint = must(SequencerEndpoint::new(
        &format!("http://{}", must(listener.local_addr())),
        &[],
    ));
    let server = std::thread::spawn(move || {
        for (method, params, result) in answers {
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("content-length") {
                        length = Some(must(value.trim().parse::<usize>()));
                    }
                }
            }
            let mut bytes = vec![0; length.expect("HTTP request length")];
            must(reader.read_exact(&mut bytes));
            let request: Value = must(serde_json::from_slice(&bytes));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            let body = json!({"jsonrpc":"2.0", "id": request["id"], "result": result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
            must(stream.flush());
        }
    });
    (endpoint, server)
}

pub struct Fixture {
    pub artifact: OfflineExport,
    pub trust: ExportTrustSource,
    pub header: CheckpointHeader,
    pub commit: BlockCommit,
    pub validators: Vec<CommitValidator>,
    pub receipt_key: ReceiptKeyEntry,
    pub evm_receipt: Vec<u8>,
    pub receipt_path: Vec<Vec<u8>>,
    pub account_id: [u8; 32],
    pub account_proof: Vec<u8>,
}

fn signed_receipt(activity: [u8; 32], height: u64, status: bool, key: &SigningKey) -> Vec<u8> {
    let encode = |signature: Option<[u8; 64]>| {
        let mut summary = Vec::new();
        summary.extend_from_slice(&21_000_u64.to_be_bytes());
        summary.extend_from_slice(&21_000_u64.to_be_bytes());
        summary.extend_from_slice(&1_u64.to_be_bytes());
        summary.extend_from_slice(&0_u32.to_be_bytes());
        summary.extend_from_slice(&[0; 40]);
        summary.extend_from_slice(&Sha256::digest(b""));
        summary.extend_from_slice(&0_u32.to_be_bytes());
        summary.extend_from_slice(&0_u32.to_be_bytes());
        summary.push(4);
        summary.extend_from_slice(b"upax");
        let sequence_hash: [u8; 32] = Sha256::digest(
            [
                b"PAXEER/v1/sequence\0".as_slice(),
                &[4; 32],
                &7_u64.to_be_bytes(),
                &activity,
            ]
            .concat(),
        )
        .into();
        let mut block = [0; 32];
        block[24..].copy_from_slice(&height.to_be_bytes());
        let mut from = [0; 32];
        from[12..].copy_from_slice(&[0xaa; 20]);
        let mut to = [0; 32];
        to[12..].copy_from_slice(&[0xbb; 20]);
        let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
        must(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
        must(encoder.u16(PROTOCOL_VERSION));
        must(encoder.bytes(&activity, 32));
        must(encoder.u64(7));
        must(encoder.bytes(&[4; 32], 32));
        must(encoder.bytes(&sequence_hash, 32));
        must(encoder.bytes(&Sha256::digest(b""), 32));
        must(encoder.i32(if status { 0 } else { -1 }));
        must(encoder.sequence_length(1, MAX_SEQUENCER_EFFECTS));
        must(encoder.u16(12));
        must(encoder.u16(0));
        must(encoder.u16(0));
        must(encoder.tag(1, 3));
        must(encoder.u8(0));
        must(encoder.bytes(&[0; 32], 32));
        must(encoder.bytes(&summary, 256));
        must(encoder.u128(21_000));
        must(encoder.bytes(&block, 32));
        must(encoder.u16(12));
        must(encoder.u32(1));
        must(encoder.u32(0));
        must(encoder.u8(2));
        must(encoder.bytes(&Sha256::digest(b"upax"), 32));
        must(encoder.u128(1));
        must(encoder.bytes(&from, 32));
        must(encoder.u128(100));
        must(encoder.u128(99));
        must(encoder.u64(1));
        must(encoder.bytes(&to, 32));
        must(encoder.u128(10));
        must(encoder.u128(11));
        must(encoder.bytes(&[0; 32], 32));
        must(encoder.bytes(&[0; 32], 32));
        must(encoder.bytes(&[0x33; 32], 32));
        must(encoder.u64(1_750_000_000_250));
        must(encoder.u8(u8::from(signature.is_some())));
        if let Some(signature) = signature {
            must(encoder.bytes(&signature, 64));
        }
        encoder.finish()
    };
    let digest = must(receipt_digest(&encode(None)));
    encode(Some(key.sign(&digest).to_bytes()))
}

impl Fixture {
    pub fn new(height: u64, status: bool) -> Self {
        let raw = must(
            Eip1559Call {
                chain_id: 10,
                nonce: height,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 2,
                gas_limit: 21_000,
                to: [0x22; 20],
                data: Vec::new(),
            }
            .sign(&[1; 32]),
        );
        let activity = transaction_hash(&raw);
        let receipt_signing_key = SigningKey::from_bytes(&[7; 32]);
        let receipt_key = ReceiptKeyEntry {
            key_id: [0x33; 32],
            public_key: receipt_signing_key.verifying_key().to_bytes(),
            first_height: 1,
            last_height: 100,
        };
        let signed_receipt = signed_receipt(activity, height, status, &receipt_signing_key);
        let decoded = must(layerx_wire::receipt::decode(&signed_receipt));
        let digest = must(receipt_digest(&must(
            layerx_wire::receipt::encode_unsigned(&decoded),
        )));
        let evm_receipt = [
            &[2][..],
            &rlp(
                &[
                    rlp(if status { &[1] } else { &[] }, false),
                    rlp(&[0x52, 0x08], false),
                    rlp(&[0; 256], false),
                    rlp(&[], true),
                ]
                .concat(),
                true,
            ),
        ]
        .concat();
        let leaf = rlp(
            &[rlp(&[0x30], false), rlp(&evm_receipt, false)].concat(),
            true,
        );
        let leaf_hash = transaction_hash(&leaf);
        let mut children = Vec::new();
        for index in 0..17 {
            children.extend(rlp(if index == 8 { &leaf_hash } else { &[] }, false));
        }
        let branch = rlp(&children, true);
        let receipts_root = transaction_hash(&branch);
        let receipt_path = vec![branch, leaf];
        let inclusion = rlp(
            &[
                rlp(&evm_receipt, false),
                rlp(
                    &receipt_path
                        .iter()
                        .flat_map(|node| rlp(node, false))
                        .collect::<Vec<_>>(),
                    true,
                ),
            ]
            .concat(),
            true,
        );
        let signing_keys: Vec<_> = (1..=3)
            .map(|value| SigningKey::from_bytes(&[value; 32]))
            .collect();
        let validators: Vec<_> = signing_keys
            .iter()
            .map(|key| {
                let public_key = key.verifying_key().to_bytes();
                CommitValidator {
                    address: CommitValidator::address_for_key(&public_key),
                    public_key,
                    voting_power: 3,
                }
            })
            .collect();
        let vector = storage_vector();
        let account: [u8; 20] = must(hex::decode(vector["address"].as_str().unwrap()))
            .try_into()
            .unwrap();
        let slot: [u8; 32] = must(hex::decode(vector["slot"].as_str().unwrap()))
            .try_into()
            .unwrap();
        let mut account_id = [0; 32];
        account_id[12..].copy_from_slice(&account);
        let (endpoint, server) = storage_node(vec![(
            "eth_getProof",
            json!([
                format!("0x{}", hex::encode(account)),
                [format!("0x{}", hex::encode(slot))],
                format!("{height:#x}")
            ]),
            vector["response"].clone(),
        )]);
        let witnesses = must(endpoint.storage_witness(&account, &[slot], height));
        server.join().unwrap();
        let account_proof = must(encode_storage_witnesses(&witnesses))
            .as_bytes()
            .to_vec();
        let state_root = must(hex::decode(vector["app_hash"].as_str().unwrap()))
            .try_into()
            .unwrap();
        let header = CheckpointHeader {
            version_block: 11,
            version_app: 1,
            chain_id: "paxeer-test".to_owned(),
            height,
            time: CommitTimestamp {
                seconds: 1_700_000_000,
                nanos: 0,
            },
            last_block_id: None,
            last_commit_hash: None,
            data_hash: Some([1; 32]),
            validators_hash: validator_set_hash(&validators),
            next_validators_hash: validator_set_hash(&validators),
            consensus_hash: [2; 32],
            app_hash: state_root,
            last_results_hash: Some(receipts_root),
            evidence_hash: None,
            proposer_address: validators[0].address,
        };
        let mut commit = BlockCommit {
            height,
            round: 0,
            block_id: BlockId {
                hash: header.block_hash(),
                part_set_total: 1,
                part_set_hash: [4; 32],
            },
            votes: Vec::new(),
        };
        let timestamp = CommitTimestamp {
            seconds: 1_700_000_001,
            nanos: 0,
        };
        for (key, validator) in signing_keys.iter().zip(&validators) {
            commit.votes.push(CommitVote {
                validator_address: validator.address,
                voting_power: validator.voting_power,
                timestamp,
                signature: key
                    .sign(&commit.vote_sign_bytes(&header.chain_id, timestamp))
                    .to_bytes(),
            });
        }
        let trust = must(ExportTrustSource::new(
            vec![receipt_key],
            validators.clone(),
        ));
        let artifact = OfflineExport {
            facts: vec![must(FactRef::new(format!(
                "receipt:{}",
                hex::encode(activity)
            )))],
            height,
            bundles: vec![ProofBundle {
                format: "layerx-receipt-proof-v2".to_owned(),
                verification_level: Rung::Committed,
                raw_transaction: canonical(raw),
                signed_receipt: canonical(signed_receipt),
                receipt_digest: digest,
                receipt_key,
                block_header: Some(canonical(encode_header(&header))),
                receipt_inclusion: Some(canonical(inclusion)),
                commit: Some(canonical(encode_commit(&commit, &validators))),
                validator_set: Some(canonical(encode_validators(&validators))),
                account_proof: None,
            }],
        };
        Self {
            artifact,
            trust,
            header,
            commit,
            validators,
            receipt_key,
            evm_receipt,
            receipt_path,
            account_id,
            account_proof,
        }
    }

    pub fn state_artifact(&self) -> OfflineExport {
        let mut artifact = self.artifact.clone();
        artifact.facts[0] = must(FactRef::new(format!(
            "state:{}:{}",
            hex::encode(transaction_hash(
                artifact.bundles[0].raw_transaction.as_bytes()
            )),
            hex::encode(self.account_id)
        )));
        artifact.bundles[0].account_proof = Some(canonical(self.account_proof.clone()));
        artifact
    }
}

#[test]
fn layerx_proof_verifies_complete_export_with_no_daemon_node_or_network() {
    let fixture = Fixture::new(8, true);
    let built = must(build(fixture.artifact.clone(), &fixture.trust));
    assert_eq!(built.local_verification.verified_receipts, 1);
    assert_eq!(built.local_verification.verified_inclusions, 1);
    assert_eq!(built.local_verification.achieved_rung, Rung::Committed);
    assert_eq!(
        must(verify(&built.artifact, &fixture.trust)),
        built.local_verification
    );
    assert_eq!(
        must(decode_header(
            built.artifact.bundles[0]
                .block_header
                .as_ref()
                .unwrap()
                .as_bytes()
        )),
        fixture.header
    );
}

#[test]
fn hostile_export_changes_and_unpinned_keys_fail() {
    let fixture = Fixture::new(8, true);
    let mut changed = fixture.artifact.clone();
    changed.bundles[0].receipt_digest[0] ^= 1;
    assert!(verify(&changed, &fixture.trust).is_err());
    let mut changed = fixture.artifact.clone();
    changed.bundles[0].receipt_key.public_key[0] ^= 1;
    assert!(verify(&changed, &fixture.trust).is_err());
    let mut changed = fixture.artifact.clone();
    changed.height += 1;
    assert!(verify(&changed, &fixture.trust).is_err());
    let mut changed = fixture.artifact.clone();
    changed.bundles.push(changed.bundles[0].clone());
    assert!(verify(&changed, &fixture.trust).is_err());
    let mut changed = fixture.artifact.clone();
    let mut signature = fixture.commit.clone();
    signature.votes[0].signature[0] ^= 1;
    changed.bundles[0].commit = Some(canonical(encode_commit(&signature, &fixture.validators)));
    assert!(verify(&changed, &fixture.trust).is_err());
    let mut quorum = fixture.commit.clone();
    quorum.votes.pop();
    changed.bundles[0].commit = Some(canonical(encode_commit(&quorum, &fixture.validators)));
    assert!(
        verify(&changed, &fixture.trust).is_err(),
        "exactly two thirds cannot commit"
    );
}

#[test]
fn offline_export_requires_complete_authenticated_inclusion() {
    let fixture = Fixture::new(8, true);
    let mut changed = fixture.artifact.clone();
    changed.bundles[0].receipt_inclusion = None;
    assert!(verify(&changed, &fixture.trust).is_err());
    let mut proof = fixture.receipt_path.clone();
    proof[1][10] ^= 1;
    let inclusion = rlp(
        &[
            rlp(&fixture.evm_receipt, false),
            rlp(
                &proof
                    .iter()
                    .flat_map(|node| rlp(node, false))
                    .collect::<Vec<_>>(),
                true,
            ),
        ]
        .concat(),
        true,
    );
    changed.bundles[0].receipt_inclusion = Some(canonical(inclusion));
    assert!(verify(&changed, &fixture.trust).is_err());
    let mut unexpected_account = fixture.artifact.clone();
    unexpected_account.bundles[0].account_proof = Some(canonical(fixture.account_proof.clone()));
    assert!(verify(&unexpected_account, &fixture.trust).is_err());
    let state = fixture.state_artifact();
    assert_eq!(
        must(verify(&state, &fixture.trust)).achieved_rung,
        Rung::Committed
    );
    let mut altered = state.clone();
    altered.bundles[0].account_proof = Some(canonical(vec![0xc0]));
    assert!(verify(&altered, &fixture.trust).is_err());
    assert_eq!(
        must(decode_validators(
            state.bundles[0].validator_set.as_ref().unwrap().as_bytes()
        )),
        fixture.validators
    );
}

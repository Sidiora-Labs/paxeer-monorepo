use ed25519_dalek::{Signer as _, SigningKey};
use k256::ecdsa::SigningKey as ReceiptSigningKey;
use layerx_agent_api::proof::ProofBundleTarget;
use layerx_agentd::read::{
    proof_bundle, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CheckpointReadError,
    CommitTimestamp, CommitValidator, CommitVote, ProofBundleRequest,
};
use layerx_agentd::receipt::{TransactionReceiptEvidence, TransactionReceiptMetadata};
use layerx_client::sequencer::{SequencerEndpoint, StorageWitness};
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::time::Duration;

fn bytes(value: &[u8]) -> Vec<u8> {
    if value.len() == 1 && value[0] < 128 {
        return value.to_vec();
    }
    rlp(value, false)
}

fn rlp(payload: &[u8], list: bool) -> Vec<u8> {
    let prefix = if list { 0xc0 } else { 0x80 };
    let mut encoded = if payload.len() < 56 {
        vec![prefix + lint_success(u8::try_from(payload.len()), "integer fits u8")]
    } else {
        let length = payload.len().to_be_bytes();
        let length = &length[length.iter().position(|byte| *byte != 0).unwrap_or(0)..];
        let mut encoded =
            vec![prefix + 55 + lint_success(u8::try_from(length.len()), "integer fits u8")];
        encoded.extend_from_slice(length);
        encoded
    };
    encoded.extend_from_slice(payload);
    encoded
}

fn leaf(compact: &[u8], value: &[u8]) -> Vec<u8> {
    rlp(&[bytes(compact), bytes(value)].concat(), true)
}

fn vector_bytes<const N: usize>(value: &Value) -> [u8; N] {
    lint_success(
        hex::decode(lint_present(value.as_str(), "vector hex")),
        "valid vector hex",
    )
    .try_into()
    .unwrap_or_else(|_| panic!("vector field must contain {N} bytes"))
}

fn storage_node(
    address: [u8; 20],
    slot: [u8; 32],
    height: u64,
    response: Value,
) -> (SequencerEndpoint, std::thread::JoinHandle<()>) {
    let listener = lint_success(TcpListener::bind("127.0.0.1:0"), "storage listener");
    let endpoint = lint_success(
        SequencerEndpoint::new(
            &format!(
                "http://{}/",
                lint_success(listener.local_addr(), "storage address")
            ),
            &[],
        ),
        "storage endpoint",
    );
    let worker = std::thread::spawn(move || {
        let (stream, _) = lint_success(listener.accept(), "storage request");
        lint_success(
            stream.set_read_timeout(Some(Duration::from_secs(5))),
            "storage timeout",
        );
        let mut reader = BufReader::new(stream);
        let mut length = None;
        loop {
            let mut line = String::new();
            assert_ne!(
                lint_success(reader.read_line(&mut line), "request headers"),
                0
            );
            if line == "\r\n" {
                break;
            }
            if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                length = Some(lint_success(
                    value.trim().parse::<usize>(),
                    "request length",
                ));
            }
        }
        let mut body = vec![0; lint_present(length, "content length")];
        lint_success(reader.read_exact(&mut body), "request body");
        let request: Value = lint_success(serde_json::from_slice(&body), "JSON-RPC request");
        assert_eq!(request["jsonrpc"], "2.0");
        assert_eq!(request["method"], "eth_getProof");
        assert_eq!(
            request["params"],
            json!([
                format!("0x{}", hex::encode(address)),
                [format!("0x{}", hex::encode(slot))],
                format!("{height:#x}")
            ])
        );
        let body = json!({"jsonrpc":"2.0", "id":request["id"], "result":response}).to_string();
        let mut stream = reader.into_inner();
        lint_success(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()), "storage response");
        lint_success(stream.flush(), "storage response flush");
    });
    (endpoint, worker)
}

struct Fixture {
    raw: Vec<u8>,
    receipt: TransactionReceiptEvidence,
    public_key: Vec<u8>,
    receipt_path: Vec<Vec<u8>>,
    receipt_root: [u8; 32],
    storage: Vec<StorageWitness>,
    storage_response: Value,
    header: CheckpointHeader,
    commit: BlockCommit,
    validators: Vec<CommitValidator>,
}

impl Fixture {
    fn new(status: bool, typed: bool) -> Self {
        let raw = Eip1559Call {
            chain_id: 10,
            nonce: 0,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 21_000,
            to: [0x22; 20],
            data: Vec::new(),
        }
        .sign(&[1; 32])
        .unwrap_or_else(|error| panic!("transaction: {error:?}"));
        let vector: Value = serde_json::from_str(include_str!(
            "../../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
        ))
        .unwrap_or_else(|error| panic!("storage vector: {error}"));
        let entry = &vector["entries"][0];
        let height = lint_present(entry["height"].as_u64(), "vector height");
        let address = vector_bytes::<20>(&entry["address"]);
        let slot = vector_bytes::<32>(&entry["slot"]);
        let state_root = vector_bytes::<32>(&entry["app_hash"]);
        let storage_response = entry["response"].clone();
        let (endpoint, worker) = storage_node(address, slot, height, storage_response.clone());
        let storage = endpoint
            .storage_witness(&address, &[slot], height)
            .unwrap_or_else(|error| panic!("storage witness: {error:?}"));
        lint_success(worker.join(), "storage node");
        assert_eq!(storage.len(), 1);
        assert_eq!(storage[0].value, vector_bytes::<32>(&entry["value"]));
        let bloom = [0; 256];
        let payload = [
            bytes(if status { &[1] } else { &[] }),
            bytes(&[0x52, 0x08]),
            bytes(&bloom),
            rlp(&[], true),
        ]
        .concat();
        let mut canonical_receipt = if typed { vec![2] } else { Vec::new() };
        canonical_receipt.extend(rlp(&payload, true));
        let receipt_key = ReceiptSigningKey::from_slice(&[7; 32])
            .unwrap_or_else(|error| panic!("receipt key: {error}"));
        let digest: [u8; 32] = Sha256::digest(&canonical_receipt).into();
        let (signature, recovery) = receipt_key
            .sign_prehash_recoverable(&digest)
            .unwrap_or_else(|error| panic!("receipt signature: {error}"));
        let mut receipt_signature = [0; 65];
        receipt_signature[..64].copy_from_slice(&signature.to_bytes());
        receipt_signature[64] = 27 + recovery.to_byte();
        let receipt = TransactionReceiptEvidence {
            metadata: TransactionReceiptMetadata {
                transaction_hash: transaction_hash(&raw),
                block_height: height,
                tx_index: 0,
                status,
                gas_used: 21_000,
                logs_bloom: bloom,
                receipt_signature,
                rung: Rung::Committed,
            },
            canonical_receipt,
        };
        // RLP(0) = 0x80: a branch at nibble 8, then a leaf at nibble 0.
        let receipt_leaf = leaf(&[0x30], &receipt.canonical_receipt);
        let mut children = Vec::new();
        let receipt_leaf_hash = transaction_hash(&receipt_leaf);
        for index in 0..17 {
            children.extend(bytes(if index == 8 {
                &receipt_leaf_hash[..]
            } else {
                &[]
            }));
        }
        let receipt_branch = rlp(&children, true);
        let receipt_root = transaction_hash(&receipt_branch);
        let receipt_path = vec![receipt_branch, receipt_leaf];
        let (header, commit, validators) =
            committed_receipt_header(height, state_root, receipt_root);
        Self {
            raw,
            receipt,
            public_key: receipt_key
                .verifying_key()
                .to_encoded_point(true)
                .as_bytes()
                .to_vec(),
            receipt_path,
            receipt_root,
            storage,
            storage_response,
            header,
            commit,
            validators,
        }
    }

    fn request(&self, target: ProofBundleTarget) -> ProofBundleRequest<'_> {
        ProofBundleRequest {
            target,
            requested_rung: Rung::Submitted,
            raw_transaction: &self.raw,
            receipt: &self.receipt,
            receipt_public_key: &self.public_key,
            receipt_proof: &self.receipt_path,
            receipts_root: self.receipt_root,
            chain_id: "paxeer-test",
            header: &self.header,
            commit: Some(&self.commit),
            validators: &self.validators,
            account_proof: None,
        }
    }

    fn node(&self, response: Value) -> (SequencerEndpoint, std::thread::JoinHandle<()>) {
        storage_node(
            self.storage[0].address,
            self.storage[0].slot,
            self.header.height,
            response,
        )
    }

    fn account_target(&self) -> ProofBundleTarget {
        let mut account_id = [0; 32];
        account_id[12..].copy_from_slice(&self.storage[0].address);
        ProofBundleTarget::AccountState {
            activity_id: self.receipt.metadata.transaction_hash,
            account_id,
        }
    }
}

#[test]
fn receipt_proof_bundle_verifies_against_the_receipts_root() {
    for status in [false, true] {
        for typed in [false, true] {
            let fixture = Fixture::new(status, typed);
            for target in [
                ProofBundleTarget::Receipt(fixture.receipt.metadata.transaction_hash),
                ProofBundleTarget::Activity(fixture.receipt.metadata.transaction_hash),
            ] {
                let request = fixture.request(target);
                let served =
                    proof_bundle(&request).unwrap_or_else(|error| panic!("bundle: {error:?}"));
                assert_eq!(served.format, "layerx-receipt-proof-v2");
                assert_eq!(served.verification_level, Rung::Receipted);
                assert_eq!(served.receipt, fixture.receipt);
                assert_eq!(served.raw_transaction, fixture.raw);
                assert_eq!(served.receipt_proof, fixture.receipt_path);
                // Independently recompute both trie nodes using the served leaf value.
                let leaf = leaf(&[0x30], &served.receipt.canonical_receipt);
                let mut children = Vec::new();
                let leaf_hash = transaction_hash(&leaf);
                for index in 0..17 {
                    children.extend(bytes(if index == 8 { &leaf_hash[..] } else { &[] }));
                }
                assert_eq!(
                    transaction_hash(&rlp(&children, true)),
                    served.receipts_root
                );
                assert_eq!(
                    served.checkpoint.as_ref().map(|value| &value.commit),
                    Some(&fixture.commit)
                );
                assert_eq!(served.state_root, None);
                assert_eq!(served.account_proof, None);

                let mut uncommitted_receipt = fixture.receipt.clone();
                uncommitted_receipt.metadata.rung = Rung::Receipted;
                let uncommitted = proof_bundle(&ProofBundleRequest {
                    receipt: &uncommitted_receipt,
                    commit: None,
                    ..request
                })
                .unwrap_or_else(|error| panic!("receipted bundle: {error:?}"));
                assert_eq!(uncommitted.verification_level, Rung::Receipted);
                assert_eq!(uncommitted.receipts_root, served.receipts_root);
                assert_eq!(uncommitted.checkpoint, None);

                let mut tampered_path = fixture.receipt_path.clone();
                tampered_path[1][10] ^= 1;
                assert!(proof_bundle(&ProofBundleRequest {
                    receipt_proof: &tampered_path,
                    ..request
                })
                .is_err());
                let mut wrong_root = fixture.receipt_root;
                wrong_root[0] ^= 1;
                assert!(proof_bundle(&ProofBundleRequest {
                    receipts_root: wrong_root,
                    ..request
                })
                .is_err());
                let mut changed_receipt = fixture.receipt.clone();
                changed_receipt.canonical_receipt[10] ^= 1;
                assert!(proof_bundle(&ProofBundleRequest {
                    receipt: &changed_receipt,
                    ..request
                })
                .is_err());
                let mut wrong_index = fixture.receipt.clone();
                wrong_index.metadata.tx_index = 1;
                assert!(proof_bundle(&ProofBundleRequest {
                    receipt: &wrong_index,
                    ..request
                })
                .is_err());
                assert_eq!(
                    proof_bundle(&ProofBundleRequest {
                        requested_rung: Rung::Committed,
                        ..request
                    }),
                    Err(CheckpointReadError::BelowRequestedRung)
                );
                let mut extra = fixture.receipt_path.clone();
                extra.push(extra[1].clone());
                assert!(proof_bundle(&ProofBundleRequest {
                    receipt_proof: &extra,
                    ..request
                })
                .is_err());
            }
        }
    }
}

#[test]
fn account_proof_bundle_requires_a_committed_state_root() {
    let fixture = Fixture::new(true, true);
    let request = ProofBundleRequest {
        account_proof: Some(&fixture.storage),
        requested_rung: Rung::Committed,
        ..fixture.request(fixture.account_target())
    };
    let served = proof_bundle(&request).unwrap_or_else(|error| panic!("account bundle: {error:?}"));
    assert_eq!(served.verification_level, Rung::Committed);
    assert_eq!(served.state_root, Some(fixture.header.app_hash));
    assert_eq!(
        served.account_proof.as_ref().map(|state| &state.storage),
        Some(&vec![(fixture.storage[0].slot, fixture.storage[0].value)])
    );
    let (endpoint, worker) = fixture.node(fixture.storage_response.clone());
    assert_eq!(request.acquire(&endpoint), Ok(served.clone()));
    lint_success(worker.join(), "storage node");
    assert_eq!(
        served.checkpoint.as_ref().map(|checkpoint| checkpoint.rung),
        Some(Rung::Committed)
    );

    let pending = ProofBundleRequest {
        commit: None,
        ..request
    };
    let refused = lint_error(proof_bundle(&pending), "test operation must fail");
    assert_eq!(refused, CheckpointReadError::NotCommitted);
    assert_eq!(refused.reason(), "read.not_committed");
    // Refusal must precede any eth_getProof call: there is no worker at this endpoint.
    let endpoint = SequencerEndpoint::new("http://127.0.0.1:1", &[])
        .unwrap_or_else(|error| panic!("endpoint: {error:?}"));
    assert_eq!(
        pending.acquire(&endpoint),
        Err(CheckpointReadError::NotCommitted)
    );

    let mut bad_commit = fixture.commit.clone();
    bad_commit.votes[0].signature[0] ^= 1;
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            commit: Some(&bad_commit),
            ..request
        }),
        Err(CheckpointReadError::Commit)
    );
    assert_eq!(
        ProofBundleRequest {
            commit: Some(&bad_commit),
            ..request
        }
        .acquire(&endpoint),
        Err(CheckpointReadError::Commit)
    );
    assert_account_root_refusals(&fixture, &request);
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

fn committed_receipt_header(
    height: u64,
    state_root: [u8; 32],
    receipt_root: [u8; 32],
) -> (CheckpointHeader, BlockCommit, Vec<CommitValidator>) {
    let key = SigningKey::from_bytes(&[3; 32]);
    let public_key = key.verifying_key().to_bytes();
    let validators = vec![CommitValidator {
        address: CommitValidator::address_for_key(&public_key),
        public_key,
        voting_power: 10,
    }];
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
        last_results_hash: Some(receipt_root),
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
    let signature = key
        .sign(&commit.vote_sign_bytes(&header.chain_id, timestamp))
        .to_bytes();
    commit.votes.push(CommitVote {
        validator_address: validators[0].address,
        voting_power: 10,
        timestamp,
        signature,
    });
    (header, commit, validators)
}

fn assert_account_root_refusals(fixture: &Fixture, request: &ProofBundleRequest<'_>) {
    let mut wrong_account = fixture.storage.clone();
    wrong_account[0].value[31] ^= 1;
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            account_proof: Some(&wrong_account),
            ..*request
        }),
        Err(CheckpointReadError::InvalidProof)
    );
    let mut wrong_response = fixture.storage_response.clone();
    wrong_response["hexValues"][0] = json!(hex::encode(wrong_account[0].value));
    let (endpoint, worker) = fixture.node(wrong_response);
    assert_eq!(
        request.acquire(&endpoint),
        Err(CheckpointReadError::InvalidProof)
    );
    lint_success(worker.join(), "storage node");
    let mut wrong_address = fixture.storage.clone();
    wrong_address[0].address[0] ^= 1;
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            account_proof: Some(&wrong_address),
            ..*request
        }),
        Err(CheckpointReadError::TargetMismatch)
    );
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            account_proof: None,
            ..*request
        }),
        Err(CheckpointReadError::InvalidProof)
    );
    let mut pending_receipt = fixture.receipt.clone();
    pending_receipt.metadata.rung = Rung::Receipted;
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            receipt: &pending_receipt,
            ..*request
        }),
        Err(CheckpointReadError::NotCommitted)
    );
    let mut foreign_header = fixture.header.clone();
    foreign_header.app_hash[0] ^= 1;
    let mut foreign_commit = fixture.commit.clone();
    foreign_commit.block_id.hash = foreign_header.block_hash();
    let key = SigningKey::from_bytes(&[3; 32]);
    foreign_commit.votes[0].signature = key
        .sign(
            &foreign_commit
                .vote_sign_bytes(&foreign_header.chain_id, foreign_commit.votes[0].timestamp),
        )
        .to_bytes();
    let foreign_request = ProofBundleRequest {
        header: &foreign_header,
        commit: Some(&foreign_commit),
        ..*request
    };
    assert_eq!(
        proof_bundle(&foreign_request),
        Err(CheckpointReadError::RootMismatch)
    );
    let (endpoint, worker) = fixture.node(fixture.storage_response.clone());
    assert_eq!(
        foreign_request.acquire(&endpoint),
        Err(CheckpointReadError::RootMismatch)
    );
    lint_success(worker.join(), "storage node");
}

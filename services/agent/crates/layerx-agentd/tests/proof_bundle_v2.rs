use ed25519_dalek::{Signer as _, SigningKey};
use k256::ecdsa::SigningKey as ReceiptSigningKey;
use layerx_agent_api::proof::ProofBundleTarget;
use layerx_agentd::read::{
    proof_bundle, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CheckpointReadError,
    CommitTimestamp, CommitValidator, CommitVote, ProofBundleRequest,
};
use layerx_agentd::receipt::{TransactionReceiptEvidence, TransactionReceiptMetadata};
use layerx_client::sequencer::{AccountProof, SequencerEndpoint};
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

fn bytes(value: &[u8]) -> Vec<u8> {
    if value.len() == 1 && value[0] < 128 {
        return value.to_vec();
    }
    rlp(value, false)
}

fn rlp(payload: &[u8], list: bool) -> Vec<u8> {
    let prefix = if list { 0xc0 } else { 0x80 };
    let mut encoded = if payload.len() < 56 {
        vec![prefix + payload.len() as u8]
    } else {
        let length = payload.len().to_be_bytes();
        let length = &length[length.iter().position(|byte| *byte != 0).unwrap_or(0)..];
        let mut encoded = vec![prefix + 55 + length.len() as u8];
        encoded.extend_from_slice(length);
        encoded
    };
    encoded.extend_from_slice(payload);
    encoded
}

fn leaf(compact: &[u8], value: &[u8]) -> Vec<u8> {
    rlp(&[bytes(compact), bytes(value)].concat(), true)
}

struct Fixture {
    raw: Vec<u8>,
    receipt: TransactionReceiptEvidence,
    public_key: Vec<u8>,
    receipt_path: Vec<Vec<u8>>,
    receipt_root: [u8; 32],
    account: AccountProof,
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
                block_height: 8,
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
        let address = [0x12; 20];
        let storage_hash = transaction_hash(&[0x80]);
        let code_hash = transaction_hash(&[]);
        let account_value = rlp(
            &[
                bytes(&[1]),
                bytes(&[9]),
                bytes(&storage_hash),
                bytes(&code_hash),
            ]
            .concat(),
            true,
        );
        let compact = [&[0x20][..], &transaction_hash(&address)].concat();
        let account_leaf = leaf(&compact, &account_value);
        let state_root = transaction_hash(&account_leaf);
        let mut balance = [0; 32];
        balance[31] = 9;
        let account = AccountProof {
            address,
            balance,
            nonce: 1,
            code_hash,
            storage_hash,
            account_proof: vec![account_leaf],
            storage: Vec::new(),
        };
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
            height: 8,
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
            height: 8,
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
            account,
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

    fn account_target(&self) -> ProofBundleTarget {
        let mut account_id = [0; 32];
        account_id[12..].copy_from_slice(&self.account.address);
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
        account_proof: Some(&fixture.account),
        requested_rung: Rung::Committed,
        ..fixture.request(fixture.account_target())
    };
    let served = proof_bundle(&request).unwrap_or_else(|error| panic!("account bundle: {error:?}"));
    assert_eq!(served.verification_level, Rung::Committed);
    assert_eq!(served.state_root, Some(fixture.header.app_hash));
    assert_eq!(served.account_proof, Some(fixture.account.clone()));
    assert_eq!(
        served.checkpoint.as_ref().map(|checkpoint| checkpoint.rung),
        Some(Rung::Committed)
    );

    let pending = ProofBundleRequest {
        commit: None,
        ..request
    };
    let refused = proof_bundle(&pending).unwrap_err();
    assert_eq!(refused, CheckpointReadError::NotCommitted);
    assert_eq!(refused.reason(), "read.not_committed");
    // Refusal must precede any eth_getProof call: there is no server at this endpoint.
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
    let mut wrong_account = fixture.account.clone();
    wrong_account.balance[31] ^= 1;
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            account_proof: Some(&wrong_account),
            ..request
        }),
        Err(CheckpointReadError::InvalidProof)
    );
    let mut wrong_address = fixture.account.clone();
    wrong_address.address[0] ^= 1;
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            account_proof: Some(&wrong_address),
            ..request
        }),
        Err(CheckpointReadError::TargetMismatch)
    );
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            account_proof: None,
            ..request
        }),
        Err(CheckpointReadError::InvalidProof)
    );
    let mut pending_receipt = fixture.receipt.clone();
    pending_receipt.metadata.rung = Rung::Receipted;
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            receipt: &pending_receipt,
            ..request
        }),
        Err(CheckpointReadError::NotCommitted)
    );
    let mut wrong_proof = fixture.account.clone();
    wrong_proof.account_proof[0][10] ^= 1;
    assert_eq!(
        proof_bundle(&ProofBundleRequest {
            account_proof: Some(&wrong_proof),
            ..request
        }),
        Err(CheckpointReadError::RootMismatch)
    );
}

//! Decision 2656 / gotcha 3230: acquire and authenticate coverage for each wait.
#[path = "repair_22_31_494.rs"]
mod fixtures;

use ed25519_dalek::{Signer as _, SigningKey};
use fixtures::{options, policy, run, Node};
use layerx_client::rpc::receipt_commit::SequenceBatch;
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::payments::Payment;
use layerx_crypto::signer::LocalSigner;
use layerx_programs_registry::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    merkle::SimpleProof,
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, PartSetHeader, Timestamp},
    ChainProofVerifier,
};
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::node_commit::{
    NodeCommitAuthority, NodeCommitCoverage, NodeTransactionProof,
};
use layerx_sdk::rpc_verification::{ReceiptPolicy, VerifiedRpcReceipt};
use layerx_sdk::wallet::Wallet;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::time::Duration;

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn base64(bytes: &[u8]) -> String {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut encoded = String::new();
    for chunk in bytes.chunks(3) {
        let word = u32::from(chunk[0]) << 16
            | u32::from(*chunk.get(1).unwrap_or(&0)) << 8
            | u32::from(*chunk.get(2).unwrap_or(&0));
        for index in 0..4 {
            encoded.push(if index > chunk.len() {
                '='
            } else {
                char::from(ALPHABET[((word >> (18 - 6 * index)) & 63) as usize])
            });
        }
    }
    encoded
}

fn authority() -> NodeCommitAuthority {
    NodeCommitAuthority::new(
        "served-batch-wait".into(),
        vec![(
            SigningKey::from_bytes(&[0x72; 32])
                .verifying_key()
                .to_bytes(),
            10,
        )],
        1000,
        1001,
    )
    .expect("independently pinned Comet authority")
}

fn wallet<'a>(
    rpc: &'a RpcClient,
    signer: &'a LocalSigner,
    policy: &'a ReceiptPolicy,
) -> Wallet<'a> {
    Wallet {
        rpc,
        signer,
        policy,
        native_asset: [3; 32],
    }
}

fn payment(node: &Node) -> VerifiedRpcReceipt {
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).expect("wallet owner");
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    run(wallet(&rpc, &signer, &policy).payment(Payment::OpenAccount { asset: [3; 32] }, &options()))
        .expect("signed payment through production SDK transport and receipt verifier")
}

fn activity(receipt: &VerifiedRpcReceipt) -> [u8; 32] {
    receipt
        .receipt()
        .protocol()
        .expect("protocol receipt")
        .activity_id()
}

fn batch_requests(node: &Node) -> Vec<Value> {
    node.state.lock().expect("node store")["requests"]
        .as_array()
        .expect("request journal")
        .iter()
        .filter(|request| request["method"] == "px_getSequenceBatch")
        .map(|request| request["params"].clone())
        .collect()
}

fn wait(rpc: &RpcClient, receipt: &VerifiedRpcReceipt) -> Result<VerifiedRpcReceipt, RpcError> {
    rpc.wait_for(
        activity(receipt),
        Commitment::Committed,
        &policy(),
        Duration::from_secs(5),
    )
}

struct CommitFixture {
    coverage: NodeCommitCoverage,
    block_hash: [u8; 32],
}
impl CommitFixture {
    fn publish(node: &Node, receipt: &VerifiedRpcReceipt, height: u64) -> Self {
        let protocol = receipt.receipt().protocol().expect("protocol receipt");
        let activity = protocol.activity_id();
        let receipt_key = SigningKey::from_bytes(&[0x5e; 32]);
        let validator_key = SigningKey::from_bytes(&[0x72; 32]);
        let validator = TrustedValidator::new(validator_key.verifying_key().to_bytes(), 10);
        let verifier = ChainProofVerifier {
            chain_id: "served-batch-wait".into(),
            validators: ValidatorSet::new(vec![validator.clone()]).expect("pinned validator set"),
        };
        let mut batch = SequenceBatch {
            height,
            from_sequence: protocol.global_sequence(),
            to_sequence: protocol.global_sequence(),
            previous_root: protocol.previous_state_root(),
            resulting_root: protocol.resulting_state_root(),
            activity_root: Sha256::digest(activity).into(),
            key_id: Sha256::digest(receipt_key.verifying_key().as_bytes()).into(),
            signature: [0; 64],
        };
        batch.signature = receipt_key
            .sign(
                &batch
                    .signing_digest(&verifier.chain_id)
                    .expect("batch digest"),
            )
            .to_bytes();
        let batch_transaction = batch.encode().expect("production sequence-batch encoder");
        let leaf_hash: [u8; 32] = Sha256::new()
            .chain_update([0])
            .chain_update(Sha256::digest(&batch_transaction))
            .finalize()
            .into();
        let proof = SimpleProof {
            total: 1,
            index: 0,
            leaf_hash,
            aunts: vec![],
        };
        let header = ChainHeader {
            version: HeaderVersion { block: 11, app: 1 },
            chain_id: verifier.chain_id.clone(),
            height: height as i64,
            time: HeaderTime {
                seconds: 1000,
                nanos: 0,
            },
            last_block_id: HeaderBlockId {
                hash: vec![0x33; 32],
                part_set_total: 1,
                part_set_hash: vec![0x33; 32],
            },
            last_commit_hash: vec![0x33; 32],
            data_hash: leaf_hash.to_vec(),
            validators_hash: verifier.validators.hash().to_vec(),
            next_validators_hash: verifier.validators.hash().to_vec(),
            consensus_hash: vec![0x33; 32],
            app_hash: protocol.resulting_state_root().to_vec(),
            last_results_hash: vec![0x33; 32],
            evidence_hash: vec![0x33; 32],
            proposer_address: validator.address.to_vec(),
        };
        let block_hash = header.hash().expect("production Comet header hash");
        let block_id = BlockId {
            hash: block_hash.to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![0x33; 32],
            },
        };
        let signature = validator_key
            .sign(
                &vote_sign_bytes(
                    &verifier.chain_id,
                    height as i64,
                    0,
                    Some(&block_id),
                    Timestamp {
                        seconds: 1000,
                        nanos: 0,
                    },
                )
                .expect("production Comet vote encoder"),
            )
            .to_bytes();
        let id_json = json!({"hash":hex(&block_hash),
            "parts":{"total":1,"hash":hex(&[0x33; 32])}});
        let signed = json!({
            "header":{
                "version":{"block":"11","app":"1"}, "chain_id":verifier.chain_id,
                "height":height.to_string(), "time":"1970-01-01T00:16:40Z",
                "last_block_id":{"hash":hex(&header.last_block_id.hash),
                    "parts":{"total":1,"hash":hex(&header.last_block_id.part_set_hash)}},
                "last_commit_hash":hex(&header.last_commit_hash), "data_hash":hex(&header.data_hash),
                "validators_hash":hex(&header.validators_hash),
                "next_validators_hash":hex(&header.next_validators_hash),
                "consensus_hash":hex(&header.consensus_hash), "app_hash":hex(&header.app_hash),
                "last_results_hash":hex(&header.last_results_hash), "evidence_hash":hex(&header.evidence_hash),
                "proposer_address":hex(&header.proposer_address)
            },
            "commit":{"height":height.to_string(),"round":0,"block_id":id_json,
                "signatures":[{"block_id_flag":2,"validator_address":hex(&validator.address),
                    "timestamp":"1970-01-01T00:16:40Z","signature":base64(&signature)}]}
        });
        let mut state = node.state.lock().expect("node store");
        let summary = &mut state["receipts"][format!("0x{}", hex(&activity))];
        summary["rung"] = json!("committed");
        summary["blockNumber"] = json!(format!("0x{height:x}"));
        summary["blockHash"] = json!(format!("0x{}", hex(&block_hash)));
        let record = json!({"receiptJson":summary.clone(), "activityIds":[format!("0x{}", hex(&activity))],
            "commit":{"signed_header":signed},
            "batch":{"height":height.to_string(), "tx":format!("0x{}",hex(&batch_transaction)),
                "proof":{"total":proof.total, "index":proof.index,
                    "leafHash":format!("0x{}",hex(&proof.leaf_hash)), "aunts":[]}}});
        if !state["committed_by_height"].is_object() {
            state["committed_by_height"] = json!({});
        }
        state["committed_by_height"][height.to_string()] = record;
        Self {
            coverage: NodeCommitCoverage {
                covering_height: height,
                batch_transaction,
                transaction_proof: NodeTransactionProof {
                    total: 1,
                    index: 0,
                    aunts: vec![],
                },
                activity_ids: vec![activity],
            },
            block_hash,
        }
    }
}

#[test]
#[allow(non_snake_case)]
fn px_getSequenceBatch() {
    let node = Node::start(false);
    let first = payment(&node);
    let first_commit = CommitFixture::publish(&node, &first, 1000);
    let second = payment(&node);
    let second_commit = CommitFixture::publish(&node, &second, 1001);
    assert_ne!(activity(&first), activity(&second));
    assert_ne!(
        first_commit.coverage.batch_transaction,
        second_commit.coverage.batch_transaction
    );
    let rpc = node
        .rpc()
        .with_node_commit_authority(authority())
        .expect("authority");
    let owner = derive_from_seed(&[0x41; 32], 0).expect("owner");
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = wallet(&rpc, &signer, &policy);
    for (submitted, expected) in [
        (&first, (1000, first_commit.block_hash)),
        (&second, (1001, second_commit.block_hash)),
        (&first, (1000, first_commit.block_hash)),
    ] {
        let committed = wallet
            .wait_for_submitted(submitted, Commitment::Committed, Duration::from_secs(5))
            .expect("coverage of this wait's height");
        assert_eq!(committed.committed_block(), Some(expected));
        assert_eq!(committed.rung(), Rung::Committed);
        assert_eq!(
            committed.canonical_activity(),
            submitted.canonical_activity()
        );
        assert_eq!(activity(&committed), activity(submitted));
    }
    assert_eq!(
        batch_requests(&node),
        vec![json!(["1000"]), json!(["1001"]), json!(["1000"])]
    );
}

#[test]
fn batch_not_covering_the_receipt_is_refused() {
    let node = Node::start(false);
    let first = payment(&node);
    CommitFixture::publish(&node, &first, 1000);
    let second = payment(&node);
    let different_batch = CommitFixture::publish(&node, &second, 1000);
    {
        let mut state = node.state.lock().expect("node store");
        // A genuine signed block and valid batch proof at the requested height,
        // but this batch covers the other submitted activity and sequence.
        state["receipts"][format!("0x{}", hex(&activity(&first)))]["blockHash"] =
            json!(format!("0x{}", hex(&different_batch.block_hash)));
    }
    let rpc = node
        .rpc()
        .with_node_commit_authority(authority())
        .expect("authority");
    assert!(matches!(wait(&rpc, &first), Err(RpcError::Verification)));
    assert_eq!(batch_requests(&node), vec![json!(["1000"])]);
}

#[test]
fn malformed_or_unbound_served_proofs_are_refused() {
    let node = Node::start(false);
    let receipt = payment(&node);
    CommitFixture::publish(&node, &receipt, 1000);
    let original = node.state.lock().expect("node store")["committed_by_height"]["1000"].clone();
    let rpc = node
        .rpc()
        .with_node_commit_authority(authority())
        .expect("authority");
    for (path, replacement) in [
        ("/batch/height", json!("1001")),
        (
            "/batch/proof/leafHash",
            json!(format!("0x{}", hex(&[0; 32]))),
        ),
        ("/batch/proof/index", json!(1)),
        ("/batch/proof/total", json!(2)),
        (
            "/batch/proof/aunts",
            json!([format!("0x{}", hex(&[0; 32]))]),
        ),
        ("/batch/tx", json!("0x00")),
        ("/activityIds/0", json!(format!("0x{}", hex(&[0x88; 32])))),
        ("/activityIds", json!([])),
    ] {
        let mut changed = original.clone();
        *changed.pointer_mut(path).expect("served field") = replacement;
        node.state.lock().expect("node store")["committed_by_height"]["1000"] = changed;
        assert!(
            matches!(wait(&rpc, &receipt), Err(RpcError::Verification)),
            "accepted {path}"
        );
    }
    assert_eq!(batch_requests(&node).len(), 8);
}

#[test]
fn no_sequence_batch_is_a_verification_refusal() {
    let node = Node::start(false);
    let receipt = payment(&node);
    CommitFixture::publish(&node, &receipt, 1000);
    node.state.lock().expect("node store")["batch_error"] = json!({
        "code":-32602, "message":"Invalid params", "data":{"code":"no_sequence_batch"}
    });
    let rpc = node
        .rpc()
        .with_node_commit_authority(authority())
        .expect("authority");
    assert!(matches!(wait(&rpc, &receipt), Err(RpcError::Verification)));
    assert_eq!(batch_requests(&node), vec![json!(["1000"])]);
}

#[test]
fn coverage_override_is_explicit_and_never_replaces_verification() {
    let node = Node::start(false);
    let receipt = payment(&node);
    let committed = CommitFixture::publish(&node, &receipt, 1000);
    node.state.lock().expect("node store")["batch_error"] = json!({
        "code":-32602, "message":"Invalid params", "data":{"code":"no_sequence_batch"}
    });
    assert!(matches!(
        node.rpc()
            .with_node_commit_coverage_override(committed.coverage.clone()),
        Err(RpcError::MissingFinalityTrust)
    ));
    let live = node
        .rpc()
        .with_node_commit_authority(authority())
        .expect("authority");
    assert!(matches!(wait(&live, &receipt), Err(RpcError::Verification)));
    let rpc = node
        .rpc()
        .with_node_commit_authority(authority())
        .expect("authority")
        .with_node_commit_coverage_override(committed.coverage.clone())
        .expect("explicit fixture override");
    let result = wait(&rpc, &receipt).expect("explicit coverage still verifies Comet and batch");
    assert_eq!(result.committed_block(), Some((1000, committed.block_hash)));
    assert_eq!(batch_requests(&node).len(), 1);
    let mut invalid = committed.coverage.clone();
    invalid.activity_ids[0] = [0x88; 32];
    let invalid_rpc = node
        .rpc()
        .with_node_commit_authority(authority())
        .expect("authority")
        .with_node_commit_coverage_override(invalid)
        .expect("well-shaped override");
    assert!(matches!(
        wait(&invalid_rpc, &receipt),
        Err(RpcError::Verification)
    ));
    assert_eq!(batch_requests(&node).len(), 1);
    let reset = rpc
        .with_node_commit_authority(authority())
        .expect("replace trust and clear override");
    assert!(matches!(
        wait(&reset, &receipt),
        Err(RpcError::Verification)
    ));
    assert_eq!(batch_requests(&node).len(), 2);
}

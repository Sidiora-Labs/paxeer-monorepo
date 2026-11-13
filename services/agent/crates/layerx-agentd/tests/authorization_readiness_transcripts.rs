fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

mod support;

use layerx_agentd::boot::{Gate, GateError, Status, WriteGateError};
use layerx_agentd::finality::{augment_verified, restore_finality, FinalityError, VerifiedReceipt};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcError};
use layerx_agentd::outbox::Outbox;
use layerx_agentd::prepare::verify_call_binding;
use layerx_agentd::protocol_evidence::VerifierPolicyError;
use layerx_agentd::sign::{
    attach_external_signature, external, verify_before_submit, SigningError, SigningPreparation,
};
use layerx_agentd::store::Store;
use layerx_client::sequencer::SequencerReceipt;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use serde_json::json;
use support::{node_transcripts as node, rpc_hex, send_authorization, tenant};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

struct Directory(std::path::PathBuf);
impl Directory {
    fn new(label: &str) -> Self {
        use std::os::unix::fs::DirBuilderExt;

        let path = support::directory(label);

        must(std::fs::DirBuilder::new().mode(0o700).create(&path));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        must(std::fs::remove_dir_all(&self.0));
    }
}

#[test]
fn support_kernel_authorizations_cross_transport_receipt_consumer_and_reopen() {
    let root = Directory::new("typed-authorizations");
    let registry = must(ModuleRegistry::new(&[]));
    let prepared = send_authorization::preparation(9);
    let package = must(external(&prepared));
    assert_eq!(
        package.canonical_bytes,
        prepared.transaction.call.encode_unsigned()
    );
    let mut store = must(Store::open(root.0.join("store")));
    must(prepared.retain(&mut store, tenant(), [9; 32]));
    drop(store);
    let mut store = must(Store::open(root.0.join("store")));
    let restored = must(SigningPreparation::restore(&store, tenant(), [9; 32]));
    assert_eq!(restored, prepared);
    must(verify_call_binding(&restored.transaction));
    let raw = must(restored.transaction.call.sign(&[0xa5; 32]));
    let (_, signature) = must(decode_signed(&raw));
    assert_eq!(
        must(attach_external_signature(&restored, *signature.as_bytes())),
        raw
    );
    let send = must(verify_before_submit(
        &raw,
        &restored,
        &restored.actor_address,
        &registry,
    ));
    assert_eq!(send, support::verified_submission(9));
    let budget = support::budget_create_submission(10, [0x0b; 32], [0x33; 32], 500, 90_000);
    let mut outbox = Outbox::default();
    for submission in [send, budget] {
        let raw = submission.exact_bytes();
        let (call, signature) = must(decode_signed(raw));
        assert_eq!(
            must(recover_sender(&call, &signature)),
            prepared.actor_address
        );
        assert_eq!(raw[0], 2);
        assert_eq!(call.chain_id, 17);
        let hash = submission.activity_id();
        assert_eq!(hash, transaction_hash(raw));
        must(outbox.enqueue(&mut store, tenant(), raw.to_vec()));
        let (receipt, key) = support::sequencer_receipt(raw, true);
        let (mut config, server) = node::replay(vec![
            (
                "eth_chainId",
                json!([]),
                Some(json!(format!("{:#x}", call.chain_id))),
            ),
            (
                "eth_sendRawTransaction",
                json!([rpc_hex(raw)]),
                Some(json!(rpc_hex(&hash))),
            ),
            (
                "pax_sequencerReceipt",
                json!([rpc_hex(&hash)]),
                Some(receipt),
            ),
        ]);
        config.chain_id = call.chain_id;
        let rpc = must(NodeRpc::connect(&config));
        assert_eq!(rpc.chain_id(), call.chain_id);
        assert_eq!(must(rpc.send_raw_transaction(raw)), hash);
        let Some(SequencerReceipt::Receipted(receipt)) = must(rpc.sequencer_receipt(&hash)) else {
            panic!("executed receipt required");
        };
        assert!(receipt.success);
        assert_eq!(receipt.to, call.to);
        assert_eq!(receipt.from, prepared.actor_address);
        let evidence = must(VerifiedReceipt::verify(hash, &receipt.receipt, &[key]));
        let record = must(augment_verified(
            &mut store,
            tenant(),
            submission.idempotency_key(),
            &evidence,
            None,
        ));
        let traffic = must(server.join());
        assert_eq!(traffic.transmitted, vec![(hash, raw.to_vec())]);
        assert_eq!(traffic.lookups, 1);
        drop(rpc);
        drop(store);
        store = must(Store::open(root.0.join("store")));
        let recovered = must(restore_finality(
            &store,
            tenant(),
            submission.idempotency_key(),
            &key,
            "125",
            &[],
        ));
        assert_eq!(recovered, record);
        assert_eq!(recovered.rung, Rung::Receipted);
        let mut reopened = Outbox::default();
        must(reopened.restore(&store, tenant(), hash));
        assert_eq!(must(reopened.exact_signed_bytes(hash)), raw);
    }
}

#[test]
fn send_authorization_restored_preparation_refuses_every_changed_binding() {
    let root = Directory::new("restored-send");
    let original = send_authorization::preparation(4);
    let mut store = must(Store::open(&root.0));
    must(original.retain(&mut store, tenant(), [4; 32]));
    drop(store);
    let store = must(Store::open(&root.0));
    let restored = must(SigningPreparation::restore(&store, tenant(), [4; 32]));
    assert_eq!(restored, original);
    let raw = must(restored.transaction.call.sign(&[0xa5; 32]));
    let registry = must(ModuleRegistry::new(&[]));
    assert!(SigningPreparation::restore(&store, tenant(), [5; 32]).is_err());
    let foreign = must(restored.transaction.call.sign(&[0xa6; 32]));
    assert_eq!(
        verify_before_submit(&foreign, &restored, &restored.actor_address, &registry),
        Err(SigningError::SignerMismatch)
    );
    let mut changes = Vec::new();
    for field in 0..10 {
        let mut tampered = restored.clone();
        match field {
            0 => tampered.transaction.call.nonce += 1,
            1 => tampered.transaction.call.chain_id += 1,
            2 => tampered.transaction.call.max_fee_per_gas += 1,
            3 => tampered.transaction.call.max_priority_fee_per_gas += 1,
            4 => tampered.transaction.call.gas_limit += 1,
            5 => tampered.transaction.call.to[0] ^= 1,
            6 => tampered.transaction.call.data[80] ^= 1,
            7 => tampered.transaction.disclosure_digest[0] ^= 1,
            8 => tampered.transaction.disclosure.idempotency_key[0] ^= 1,
            9 => tampered.actor_address[0] ^= 1,
            _ => unreachable!(),
        }
        changes.push(tampered);
    }
    for tampered in changes {
        assert!(verify_before_submit(&raw, &tampered, &restored.actor_address, &registry).is_err());
    }
    assert_eq!(
        must(verify_before_submit(
            &raw,
            &restored,
            &restored.actor_address,
            &registry
        ))
        .exact_bytes(),
        raw
    );
}

#[test]
fn readiness_node_transcript_receipts_and_authority_survive_consumer_restart() {
    for success in [true, false] {
        let root = Directory::new("node-readiness");
        let case = node::case(success);
        let key = node::key(&case);
        let raw = node::unhex(&case["rawTransaction"]);
        let hash = transaction_hash(&raw);
        let mut exchanges = node::readiness_exchanges(&case);
        exchanges.push(node::receipt_exchange(&case));
        let (config, server) = node::replay(exchanges);
        let startup = node::startup(&root.0, &config, key);
        let mut gate = must(Gate::new(&startup));
        assert_eq!(gate.guard_write(|| ()), Err(WriteGateError::NotReady));
        let rpc = must(NodeRpc::connect(&config));
        let ready = must(gate.readiness_gate(&rpc));
        assert_eq!(ready.receipt_keys, vec![key]);
        assert_eq!(ready.generation, 1);
        let keys = ready.receipt_keys.clone();
        let Some(SequencerReceipt::Receipted(receipt)) = must(rpc.sequencer_receipt(&hash)) else {
            panic!("node execution receipt required");
        };
        assert_eq!(receipt.success, success);
        assert_eq!(receipt.receipt, node::unhex(&case["signedReceipt"]));
        let evidence = must(VerifiedReceipt::verify(hash, &receipt.receipt, &keys));
        let mut store = must(Store::open(root.0.join("store")));
        let record = must(augment_verified(
            &mut store,
            tenant(),
            hash,
            &evidence,
            None,
        ));
        must(server.join()); // Stop the transcript peer and the consumer before reopening.
        gate.disconnected();
        assert_eq!(gate.status(), &Status::NotReady);
        assert!(gate.evidence_authority().is_err());
        drop(gate);
        drop(rpc);
        drop(store);
        let store = must(Store::open(root.0.join("store")));
        assert_eq!(
            must(restore_finality(&store, tenant(), hash, &key, "125", &[])),
            record
        );
        let (config, server) = node::replay(node::readiness_exchanges(&case));
        let mut reopened = must(Gate::new(&startup));
        assert!(reopened.guard_write(|| ()).is_err());
        let rpc = must(NodeRpc::connect(&config));
        assert_eq!(must(reopened.readiness_gate(&rpc)).receipt_keys, vec![key]);
        assert!(reopened.evidence_authority().is_ok());
        assert_eq!(reopened.guard_write(|| ()), Ok(()));
        must(server.join());
    }
}

#[test]
fn readiness_wrong_chain_uncovered_and_rotated_trust_refuse_node_transcript() {
    let root = Directory::new("node-refusals");
    let case = node::case(true);
    let key = node::key(&case);
    let (mut config, server) = node::replay(vec![node::readiness_exchanges(&case).remove(0)]);
    config.chain_id += 1;
    assert_eq!(
        NodeRpc::connect(&config).err(),
        Some(NodeRpcError::ChainIdMismatch {
            expected: 126,
            observed: 125
        })
    );
    must(server.join());
    // Alter only the independently protected trust policy, never the node reply.
    let mut uncovered = key;
    uncovered.first_height += 1;
    let mut rotated = key;
    rotated.public_key = ed25519_dalek::SigningKey::from_bytes(&[42; 32])
        .verifying_key()
        .to_bytes();
    assert_ne!(rotated.public_key, key.public_key);
    for policy in [uncovered, rotated] {
        let (config, server) = node::replay(node::readiness_exchanges(&case));
        let startup = node::startup(&root.0, &config, policy);
        let mut gate = must(Gate::new(&startup));
        let rpc = must(NodeRpc::connect(&config));
        assert_eq!(
            gate.readiness_gate(&rpc),
            Err(GateError::Evidence(VerifierPolicyError::HandshakeKey))
        );
        assert_eq!(gate.status(), &Status::NotReady);
        assert_eq!(
            gate.guard_write(|| panic!("refused write executed")),
            Err(WriteGateError::NotReady)
        );
        assert!(gate.evidence_authority().is_err());
        must(server.join());
    }
}

#[test]
fn node_produced_uncovered_and_unpinned_receipts_fail_after_transport() {
    let corpus = node::corpus();
    for id in ["V3-key-not-pinned", "V3-height-outside-key-range"] {
        let case = require_some(
            corpus["cases"]
                .as_array()
                .unwrap_or_else(|| panic!("{}", "required test value"))
                .iter()
                .find(|case| case["id"] == id),
            "required test value",
        );
        let raw = node::unhex(&case["rawTransaction"]);
        let hash = transaction_hash(&raw);
        let (config, server) = node::replay(vec![
            ("eth_chainId", json!([]), Some(json!("0x7d"))),
            ("px_getReceiptKeys", json!([]), Some(node::key_reply(case))),
            node::receipt_exchange(case),
        ]);
        let rpc = must(NodeRpc::connect(&config));
        let keys = must(rpc.receipt_keys());
        let Some(SequencerReceipt::Receipted(receipt)) = must(rpc.sequencer_receipt(&hash)) else {
            panic!("node receipt required")
        };
        assert_eq!(receipt.receipt, node::unhex(&case["signedReceipt"]));
        assert!(matches!(
            VerifiedReceipt::verify(hash, &receipt.receipt, &keys),
            Err(FinalityError::InvalidReceipt)
        ));
        must(server.join());
    }
}

#[test]
fn focused_authority_decoder_retains_restart_coverage() {
    let seed = [0x4c; 32];
    let records = [support::TestAuthorityRecord {
        signing_seed: seed,
        epoch: 2,
        first_batch: 1,
        last_batch: 3,
        revoked_at_batch: None,
    }];
    must(support::try_evidence_authority_after_restart(
        support::TestAuthorityPolicy {
            protocol_version: 2,
            network_id: 42,
            records: &records,
            handshake_signing_seed: seed,
            handshake_batch: 3,
        },
    ));
}

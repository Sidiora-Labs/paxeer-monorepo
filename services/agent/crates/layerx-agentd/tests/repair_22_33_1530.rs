//! Task 22.33.1530, audit 22.30.94 repair record (requirement 22998.1).
//!
//! `readiness_empty_key_fixture` -> `readiness_node_rpc::daemon_is_not_ready_without_receipt_keys`,
//! `empty_receipt_keys_invalidate_an_already_ready_gate` and
//! `receipt_key_validation_still_refuses_invalid_nonempty_lists`. Committed gateway-shaped
//! JSON-RPC fixtures cross real HTTP, `NodeRpc`, the receipt-key decoder and Gate;
//! both writes and evidence remain closed on the actual typed Malformed refusal.
//! `legacy_owner_signature_test` -> `w7_owner_binding::a_foreign_key_signature_over_the_owner_preimage_is_refused`,
//! `the_bound_owner_key_still_verifies`, `a_claimed_foreign_sender_cannot_replace_the_bound_owner`,
//! and `owner_submission_transports_exact_bytes_hash_and_recovered_sender` below.
//! The native debit producer, ledger payment ABI, daemon preparation, secp256k1
//! codec, verified submission and `NodeRpc` transport are production implementations.
//!
//! Wiring request: domain 9 owns services/agent/crates/layerx-agentd/tests/handshake_gate.rs;
//! migrate its bare arrays/hex heights to {keys: [...]} with numeric heights and
//! assert `GateError::NodeRpc(NodeRpcError::Sequencer(SequencerError::Malformed))`
//! for empty lists while retaining all closed-write/evidence assertions.
//!
//! Baseline aa5e0820b3652890f3812a220f4b91f4f8a45387: no earlier gate ledger or
//! repair test existed. No pre-change test was run: the explicit gate order
//! requires implementation before building and hf-done as the sole verification.
//! Static audit observations above are not claimed as executed failures.
//! The verified revision, command, exit and private log are emitted by hf-done;
//! the task runner records them without editing the spec from this task.

#[path = "readiness_node_rpc.rs"]
mod readiness_node_rpc;
#[path = "w7_owner_binding.rs"]
mod w7_owner_binding;

use layerx_agentd::node_rpc::NodeRpc;
use layerx_agentd::sign::{attach_external_signature, verify_before_submit};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use serde_json::json;

#[test]
fn owner_submission_transports_exact_bytes_hash_and_recovered_sender() {
    let prepared = w7_owner_binding::owner_bound();
    let raw = w7_owner_binding::signed_by(&prepared, &w7_owner_binding::OWNER);
    let (call, signature) = decode_signed(&raw).unwrap_or_else(|error| panic!("decode: {error:?}"));
    let attached = attach_external_signature(&prepared, *signature.as_bytes())
        .unwrap_or_else(|error| panic!("attach: {error:?}"));
    assert_eq!(attached, raw);
    let verified = verify_before_submit(
        &attached,
        &prepared,
        &prepared.actor_address,
        &w7_owner_binding::registry(),
    )
    .unwrap_or_else(|error| panic!("verify: {error:?}"));
    let expected_hash = transaction_hash(&raw);
    assert_eq!(verified.activity_id(), expected_hash);
    assert_eq!(verified.exact_bytes(), raw);
    assert_eq!(verified.recovered_sender(), prepared.actor_address);
    assert_eq!(
        recover_sender(&call, &signature),
        Ok(verified.recovered_sender())
    );
    let (config, server) = readiness_node_rpc::serve_rpc(vec![
        ("eth_chainId", json!([]), json!("0x7d")),
        (
            "eth_sendRawTransaction",
            json!([format!("0x{}", hex::encode(&raw))]),
            json!(format!("0x{}", hex::encode(expected_hash))),
        ),
    ]);
    let node = NodeRpc::connect(&config).unwrap_or_else(|error| panic!("connect: {error:?}"));
    assert_eq!(
        node.send_raw_transaction(verified.exact_bytes()),
        Ok(expected_hash)
    );
    assert_eq!(verified.into_exact_bytes(), raw);
    server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
}

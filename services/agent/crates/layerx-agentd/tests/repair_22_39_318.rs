use layerx_agentd::budget::{hold_unknown, ProtocolBudgetState, UnknownReservation};
use layerx_agentd::human_runtime::{
    recover_tenant_budget_with_evidence, BudgetRecoveryRequest, RecoveryRefusal,
};
use layerx_agentd::protocol_evidence::RawReceiptEvidence;
use layerx_agentd::receipt::{self, ReceiptEvidenceInventory};
use layerx_agentd::store::Store;

#[path = "budget_schema.rs"]
mod budget_schema;
#[path = "budget_unknown.rs"]
mod budget_unknown;
#[path = "recovery.rs"]
mod recovery;
mod support;

const BUDGET_ID: [u8; 32] = [0x51; 32];
const PERIOD_START_MS: u64 = 1_760_000_000_000;
const DAY_MS: u64 = 86_400_000;

fn protocol(start_ms: u64, length_ms: u64, consumed: u128) -> ProtocolBudgetState {
    let mut bytes = vec![0; 278];
    bytes[1] = 1;
    bytes[2..34].copy_from_slice(&BUDGET_ID);
    bytes[34..66].fill(0x52);
    bytes[66..98].fill(0x53);
    bytes[98..130].fill(0x24);
    bytes[130..162].fill(0x54);
    bytes[162..178].copy_from_slice(&500_u128.to_be_bytes());
    bytes[178..194].copy_from_slice(&500_u128.to_be_bytes());
    bytes[210..226].copy_from_slice(&consumed.to_be_bytes());
    bytes[242..250].copy_from_slice(&length_ms.to_be_bytes());
    bytes[250..258].copy_from_slice(&start_ms.to_be_bytes());
    ProtocolBudgetState {
        evidence: support::raw_state_leaf(bytes, 99),
    }
}

fn persist(store: &mut Store, raw: &RawReceiptEvidence, evidence: bool) {
    let verifier = support::pinned_evidence_verifier();
    let retained = verifier
        .verify_receipt_for_export(raw)
        .expect("verify producer receipt");
    let id = retained.verified().activity_id();
    receipt::store(
        store,
        support::tenant(),
        id,
        raw.canonical_receipt(),
        retained.authorized_batch(),
    )
    .expect("store receipt");
    if evidence {
        receipt::persist_evidence(store, support::tenant(), id, raw).expect("store evidence");
    }
}

fn request<'a>(
    inventory: &'a ReceiptEvidenceInventory,
    start_ms: u64,
    length_ms: u64,
    consumed: u128,
) -> BudgetRecoveryRequest<'a> {
    BudgetRecoveryRequest {
        budget_id: BUDGET_ID,
        protocol_budget: protocol(start_ms, length_ms, consumed),
        verifier: support::pinned_evidence_verifier(),
        receipts_with_evidence: &inventory.with_evidence,
        receipts_without_evidence: &inventory.without_evidence,
        ceiling_maximum: 1_000,
        current_sequence: 99,
    }
}

#[test]
fn restart_selects_small_sequences_by_block_time_with_exclusive_period_end() {
    let root = support::directory("repair-318-period");
    let mut store = Store::open(&root).expect("store");
    for (id, time) in [
        (1, PERIOD_START_MS),
        (2, PERIOD_START_MS - 1),
        (3, PERIOD_START_MS + DAY_MS),
    ] {
        persist(
            &mut store,
            &support::raw_receipt_at_time([id; 32], 0, 25, u64::from(id), time, time),
            true,
        );
    }
    drop(store);
    let mut store = Store::open(&root).expect("reopen");
    let inventory =
        receipt::evidence_inventory(&store, &support::tenant()).expect("reload evidence");
    assert_eq!(inventory.with_evidence.len(), 3);
    let request = request(&inventory, PERIOD_START_MS, DAY_MS, 25);
    let first = recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request)
        .expect("recover");
    assert_eq!(first.recovered.budget_accounting.receipt_consumed, 25);
    assert_eq!(
        first.recovered.budget_accounting.protocol_consumed,
        Some(25)
    );
    assert!(first.recovered.budget_accounting.reconciled);
    assert!(first.admission.is_ok());
    first.recovered.require_write_ready().expect("write ready");
    let second = recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request)
        .expect("recover again");
    assert_eq!(
        second.recovered.budget_accounting,
        first.recovered.budget_accounting
    );
    assert!(second.admission.is_ok());
    drop(store);
    std::fs::remove_dir_all(root).expect("cleanup");
}

#[test]
fn restart_holds_receipts_without_evidence_or_establishable_time() {
    for (label, evidence) in [("missing-evidence", false), ("missing-time", true)] {
        let root = support::directory(label);
        let mut store = Store::open(&root).expect("store");
        persist(
            &mut store,
            &support::raw_receipt_at_time([1; 32], 0, 25, 1, PERIOD_START_MS, PERIOD_START_MS),
            true,
        );
        persist(
            &mut store,
            &support::raw_receipt_at_time([2; 32], 0, 10, 2, 0, 0),
            evidence,
        );
        hold_unknown(
            &mut store,
            &UnknownReservation {
                tenant: support::tenant(),
                id: [3; 32],
                amount: 15,
                expiry_sequence: 500,
                resolved: None,
            },
        )
        .expect("hold");
        drop(store);
        let mut store = Store::open(&root).expect("reopen");
        let inventory = receipt::evidence_inventory(&store, &support::tenant()).expect("reload");
        let recovery = recover_tenant_budget_with_evidence(
            &mut store,
            &support::tenant(),
            &request(&inventory, PERIOD_START_MS, DAY_MS, 25),
        )
        .expect("recover");
        assert_eq!(recovery.recovered.budget_accounting.receipt_consumed, 25);
        assert_eq!(recovery.recovered.budget_accounting.held_unresolved, 15);
        assert!(matches!(
            recovery.admission,
            Err(RecoveryRefusal::EvidenceMissing {
                budget_id: BUDGET_ID,
                count: 1
            })
        ));
        drop(store);
        std::fs::remove_dir_all(root).expect("cleanup");
    }
}

#[test]
fn restart_uses_verified_batch_header_when_receipt_has_no_time() {
    let root = support::directory("repair-318-header");
    let mut store = Store::open(&root).expect("store");
    persist(
        &mut store,
        &support::raw_receipt_at_time([1; 32], 0, 25, 1, 0, PERIOD_START_MS + 123),
        true,
    );
    drop(store);
    let mut store = Store::open(&root).expect("reopen");
    let inventory = receipt::evidence_inventory(&store, &support::tenant()).expect("reload");
    let recovered = recover_tenant_budget_with_evidence(
        &mut store,
        &support::tenant(),
        &request(&inventory, PERIOD_START_MS, DAY_MS, 25),
    )
    .expect("recover");
    assert_eq!(recovered.recovered.budget_accounting.receipt_consumed, 25);
    assert!(recovered.recovered.budget_accounting.reconciled);
    assert!(recovered.admission.is_ok());
    drop(store);
    std::fs::remove_dir_all(root).expect("cleanup");
}

#[test]
fn recorded_node_producer_receipt_recovers_by_timestamp_after_transport_and_restart() {
    use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
    use serde_json::{json, Value};
    let case = support::node_transcripts::case(true);
    let hash: [u8; 32] = support::node_transcripts::unhex(&case["receiptJson"]["txHash"])
        .try_into()
        .expect("hash");
    let (endpoint, server) = support::serve(vec![
        (
            "px_getReceiptKeys",
            json!([]),
            Some(support::node_transcripts::key_reply(&case)),
        ),
        (
            "pax_sequencerReceipt",
            json!([support::rpc_hex(&hash)]),
            Some(case["receiptJson"].clone()),
        ),
        (
            "eth_getTransactionReceipt",
            json!([support::rpc_hex(&hash)]),
            Some(Value::Null),
        ),
    ]);
    let keys = endpoint.receipt_keys().expect("keys");
    let Lookup::Verified(receipt) = lookup_authenticated(
        &endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )
    .expect("lookup") else {
        panic!("authenticated receipt")
    };
    server.join().expect("node transport");
    let timestamp = receipt.receipt().protocol().expect("canonical").timestamp();
    let root = support::directory("repair-318-node-producer");
    let mut store = Store::open(&root).expect("store");
    receipt::store_verified_if_absent(
        &mut store,
        support::tenant(),
        hash,
        receipt.canonical_bytes(),
        &receipt,
    )
    .expect("persist producer receipt");
    hold_unknown(
        &mut store,
        &UnknownReservation {
            tenant: support::tenant(),
            id: hash,
            amount: 25,
            expiry_sequence: 500,
            resolved: Some(layerx_agentd::budget::UnknownOutcome::Executed),
        },
    )
    .expect("retain requested spend");
    drop(receipt);
    drop(endpoint);
    drop(store);
    let mut store = Store::open(&root).expect("reopen");
    let inventory = receipt::evidence_inventory(&store, &support::tenant()).expect("inventory");
    let verifier = support::pinned_evidence_verifier()
        .with_receipt_keys(keys)
        .expect("authority");
    let request = BudgetRecoveryRequest {
        budget_id: BUDGET_ID,
        protocol_budget: protocol(timestamp, DAY_MS, 25),
        verifier,
        receipts_with_evidence: &inventory.with_current_evidence,
        receipts_without_evidence: &inventory.without_evidence,
        ceiling_maximum: 1_000,
        current_sequence: 99,
    };
    let recovered = recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request)
        .expect("recover recorded receipt");
    assert_eq!(recovered.recovered.budget_accounting.receipt_consumed, 25);
    assert!(recovered.recovered.budget_accounting.reconciled);
    assert!(recovered.admission.is_ok());
    drop(store);
    std::fs::remove_dir_all(root).expect("cleanup");
}

#[test]
fn current_receipt_without_time_uses_only_a_verified_header_at_its_height() {
    use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
    use serde_json::{json, Value};
    let raw = support::signed_transaction(17);
    let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
    let (view, key) = support::sequencer_receipt_at_time(&raw, true, 8, 25, 2, 0);
    let (endpoint, server) = support::serve(vec![
        (
            "pax_sequencerReceipt",
            json!([support::rpc_hex(&hash)]),
            Some(view),
        ),
        (
            "eth_getTransactionReceipt",
            json!([support::rpc_hex(&hash)]),
            Some(Value::Null),
        ),
    ]);
    let Lookup::Verified(receipt) = lookup_authenticated(
        &endpoint,
        hash,
        LookupContext {
            receipt_keys: &[key],
        },
    )
    .expect("lookup") else {
        panic!("verified receipt")
    };
    server.join().expect("transport");
    let root = support::directory("repair-318-current-header");
    let mut store = Store::open(&root).expect("store");
    receipt::store_verified_if_absent(
        &mut store,
        support::tenant(),
        hash,
        receipt.canonical_bytes(),
        &receipt,
    )
    .expect("persist");
    hold_unknown(
        &mut store,
        &UnknownReservation {
            tenant: support::tenant(),
            id: hash,
            amount: 25,
            expiry_sequence: 500,
            resolved: Some(layerx_agentd::budget::UnknownOutcome::Executed),
        },
    )
    .expect("requested amount");
    drop(receipt);
    drop(endpoint);
    drop(store);
    let mut store = Store::open(&root).expect("reopen");
    let inventory = receipt::evidence_inventory(&store, &support::tenant()).expect("inventory");
    let checkpoint = support::current_node::signed_checkpoint(
        150,
        (PERIOD_START_MS / 1_000) as i64,
        123_999_999,
    );
    let (protocol_budget, _) = support::current_node::committed_budget_with_period(
        BUDGET_ID,
        25,
        PERIOD_START_MS + 123,
        1,
        checkpoint,
    );
    let mut request = BudgetRecoveryRequest {
        budget_id: BUDGET_ID,
        protocol_budget,
        verifier: support::current_node::authority(8, 100, 199),
        receipts_with_evidence: &inventory.with_current_evidence,
        receipts_without_evidence: &inventory.without_evidence,
        ceiling_maximum: 1_000,
        current_sequence: 150,
    };
    let recovered = recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request)
        .expect("header recovery");
    assert_eq!(recovered.recovered.budget_accounting.receipt_consumed, 25);
    assert!(recovered.recovered.budget_accounting.reconciled);
    assert!(recovered.admission.is_ok());
    // A newer budget checkpoint must never stand in for the receipt's own block.
    let checkpoint = support::current_node::signed_checkpoint(
        151,
        (PERIOD_START_MS / 1_000) as i64,
        123_999_999,
    );
    request.protocol_budget = support::current_node::committed_budget_with_period(
        BUDGET_ID,
        25,
        PERIOD_START_MS + 123,
        1,
        checkpoint,
    )
    .0;
    let held = recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request)
        .expect("missing header recovery");
    assert_eq!(held.recovered.budget_accounting.receipt_consumed, 0);
    assert!(matches!(
        held.admission,
        Err(RecoveryRefusal::EvidenceMissing {
            budget_id: BUDGET_ID,
            count: 1
        })
    ));
    drop(store);
    std::fs::remove_dir_all(root).expect("cleanup");
}

#[test]
fn receipt_timestamp_is_selected_before_header_time_and_is_authenticated() {
    let root = support::directory("repair-318-time-authority");
    let mut store = Store::open(&root).expect("store");
    persist(
        &mut store,
        &support::raw_receipt_at_time(
            [1; 32],
            0,
            25,
            1,
            PERIOD_START_MS + DAY_MS - 1,
            PERIOD_START_MS - 1,
        ),
        true,
    );
    drop(store);
    let mut store = Store::open(&root).expect("reopen");
    let mut inventory = receipt::evidence_inventory(&store, &support::tenant()).expect("inventory");
    let recovered = recover_tenant_budget_with_evidence(
        &mut store,
        &support::tenant(),
        &request(&inventory, PERIOD_START_MS, DAY_MS, 25),
    )
    .expect("recover canonical timestamp");
    assert_eq!(recovered.recovered.budget_accounting.receipt_consumed, 25);
    assert!(recovered.admission.is_ok());
    let original = &inventory.with_evidence[0].evidence;
    let mut changed = original.canonical_receipt().to_vec();
    // Timestamp immediately precedes the optional signature flag and bytes.
    let timestamp_offset = changed.len() - (1 + 4 + 64) - 8;
    changed[timestamp_offset..timestamp_offset + 8]
        .copy_from_slice(&(PERIOD_START_MS - 1).to_be_bytes());
    let decoded = layerx_wire::receipt::decode(&changed).expect("canonical timestamp mutation");
    assert_eq!(
        decoded.protocol().expect("protocol receipt").timestamp(),
        PERIOD_START_MS - 1
    );
    inventory.with_evidence[0].evidence = support::corrupt_raw_receipt(original, changed);
    assert!(matches!(
        recover_tenant_budget_with_evidence(
            &mut store,
            &support::tenant(),
            &request(&inventory, PERIOD_START_MS, DAY_MS, 25)
        ),
        Err(RecoveryRefusal::Recovery { .. })
    ));
    drop(store);
    std::fs::remove_dir_all(root).expect("cleanup");
}

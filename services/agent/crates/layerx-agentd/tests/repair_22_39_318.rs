fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use layerx_agentd::budget::{hold_unknown, ProtocolBudgetState, UnknownReservation};
use layerx_agentd::human_runtime::{
    recover_tenant_budget_with_evidence, BudgetRecoveryRequest, RecoveryRefusal,
};
use layerx_agentd::protocol_evidence::RawReceiptEvidence;
use layerx_agentd::receipt::{self, ReceiptEvidenceInventory};
use layerx_agentd::store::Store;

#[path = "suites/budget_schema.rs"]
mod budget_schema;
#[path = "suites/budget_unknown.rs"]
mod budget_unknown;
#[path = "suites/recovery.rs"]
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
    let retained = require_ok(
        verifier.verify_receipt_for_export(raw),
        "verify producer receipt",
    );
    let id = retained.verified().activity_id();
    require_ok(
        receipt::store(
            store,
            support::tenant(),
            id,
            raw.canonical_receipt(),
            retained.authorized_batch(),
        ),
        "store receipt",
    );
    if evidence {
        require_ok(
            receipt::persist_evidence(store, support::tenant(), id, raw),
            "store evidence",
        );
    }
}

fn request(
    inventory: &ReceiptEvidenceInventory,
    start_ms: u64,
    length_ms: u64,
    consumed: u128,
) -> BudgetRecoveryRequest<'_> {
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
    let mut store = require_ok(Store::open(&root), "store");
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
    let mut store = require_ok(Store::open(&root), "reopen");
    let inventory = require_ok(
        receipt::evidence_inventory(&store, &support::tenant()),
        "reload evidence",
    );
    assert_eq!(inventory.with_evidence.len(), 3);
    let request = request(&inventory, PERIOD_START_MS, DAY_MS, 25);
    let first = require_ok(
        recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request),
        "recover",
    );
    assert_eq!(first.recovered.budget_accounting.receipt_consumed, 25);
    assert_eq!(
        first.recovered.budget_accounting.protocol_consumed,
        Some(25)
    );
    assert!(first.recovered.budget_accounting.reconciled);
    assert!(first.admission.is_ok());
    require_ok(first.recovered.require_write_ready(), "write ready");
    let second = require_ok(
        recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request),
        "recover again",
    );
    assert_eq!(
        second.recovered.budget_accounting,
        first.recovered.budget_accounting
    );
    assert!(second.admission.is_ok());
    drop(store);
    require_ok(std::fs::remove_dir_all(root), "cleanup");
}

#[test]
fn restart_holds_receipts_without_evidence_or_establishable_time() {
    for (label, evidence) in [("missing-evidence", false), ("missing-time", true)] {
        let root = support::directory(label);
        let mut store = require_ok(Store::open(&root), "store");
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
        require_ok(
            hold_unknown(
                &mut store,
                &UnknownReservation {
                    tenant: support::tenant(),
                    id: [3; 32],
                    amount: 15,
                    expiry_sequence: 500,
                    resolved: None,
                },
            ),
            "hold",
        );
        drop(store);
        let mut store = require_ok(Store::open(&root), "reopen");
        let inventory = require_ok(
            receipt::evidence_inventory(&store, &support::tenant()),
            "reload",
        );
        let recovery = require_ok(
            recover_tenant_budget_with_evidence(
                &mut store,
                &support::tenant(),
                &request(&inventory, PERIOD_START_MS, DAY_MS, 25),
            ),
            "recover",
        );
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
        require_ok(std::fs::remove_dir_all(root), "cleanup");
    }
}

#[test]
fn restart_uses_verified_batch_header_when_receipt_has_no_time() {
    let root = support::directory("repair-318-header");
    let mut store = require_ok(Store::open(&root), "store");
    persist(
        &mut store,
        &support::raw_receipt_at_time([1; 32], 0, 25, 1, 0, PERIOD_START_MS + 123),
        true,
    );
    drop(store);
    let mut store = require_ok(Store::open(&root), "reopen");
    let inventory = require_ok(
        receipt::evidence_inventory(&store, &support::tenant()),
        "reload",
    );
    let recovered = require_ok(
        recover_tenant_budget_with_evidence(
            &mut store,
            &support::tenant(),
            &request(&inventory, PERIOD_START_MS, DAY_MS, 25),
        ),
        "recover",
    );
    assert_eq!(recovered.recovered.budget_accounting.receipt_consumed, 25);
    assert!(recovered.recovered.budget_accounting.reconciled);
    assert!(recovered.admission.is_ok());
    drop(store);
    require_ok(std::fs::remove_dir_all(root), "cleanup");
}

#[test]
fn recorded_node_producer_receipt_recovers_by_timestamp_after_transport_and_restart() {
    use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
    use serde_json::{json, Value};
    let case = support::node_transcripts::case(true);
    let hash: [u8; 32] = require_ok(
        support::node_transcripts::unhex(&case["receiptJson"]["txHash"]).try_into(),
        "hash",
    );
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
    let keys = require_ok(endpoint.receipt_keys(), "keys");
    let Lookup::Verified(receipt) = require_ok(
        lookup_authenticated(
            &endpoint,
            hash,
            LookupContext {
                receipt_keys: &keys,
            },
        ),
        "lookup",
    ) else {
        panic!("authenticated receipt")
    };
    require_ok(server.join(), "node transport");
    let timestamp = require_some(receipt.receipt().protocol(), "canonical").timestamp();
    let root = support::directory("repair-318-node-producer");
    let mut store = require_ok(Store::open(&root), "store");
    require_ok(
        receipt::store_verified_if_absent(
            &mut store,
            support::tenant(),
            hash,
            receipt.canonical_bytes(),
            &receipt,
        ),
        "persist producer receipt",
    );
    require_ok(
        hold_unknown(
            &mut store,
            &UnknownReservation {
                tenant: support::tenant(),
                id: hash,
                amount: 25,
                expiry_sequence: 500,
                resolved: Some(layerx_agentd::budget::UnknownOutcome::Executed),
            },
        ),
        "retain requested spend",
    );
    drop(receipt);
    drop(endpoint);
    drop(store);
    let mut store = require_ok(Store::open(&root), "reopen");
    let inventory = require_ok(
        receipt::evidence_inventory(&store, &support::tenant()),
        "inventory",
    );
    let verifier = require_ok(
        support::pinned_evidence_verifier().with_receipt_keys(keys),
        "authority",
    );
    let request = BudgetRecoveryRequest {
        budget_id: BUDGET_ID,
        protocol_budget: protocol(timestamp, DAY_MS, 25),
        verifier,
        receipts_with_evidence: &inventory.with_current_evidence,
        receipts_without_evidence: &inventory.without_evidence,
        ceiling_maximum: 1_000,
        current_sequence: 99,
    };
    let recovered = require_ok(
        recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request),
        "recover recorded receipt",
    );
    assert_eq!(recovered.recovered.budget_accounting.receipt_consumed, 25);
    assert!(recovered.recovered.budget_accounting.reconciled);
    assert!(recovered.admission.is_ok());
    drop(store);
    require_ok(std::fs::remove_dir_all(root), "cleanup");
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
    let Lookup::Verified(receipt) = require_ok(
        lookup_authenticated(
            &endpoint,
            hash,
            LookupContext {
                receipt_keys: &[key],
            },
        ),
        "lookup",
    ) else {
        panic!("verified receipt")
    };
    require_ok(server.join(), "transport");
    let root = support::directory("repair-318-current-header");
    let mut store = require_ok(Store::open(&root), "store");
    require_ok(
        receipt::store_verified_if_absent(
            &mut store,
            support::tenant(),
            hash,
            receipt.canonical_bytes(),
            &receipt,
        ),
        "persist",
    );
    require_ok(
        hold_unknown(
            &mut store,
            &UnknownReservation {
                tenant: support::tenant(),
                id: hash,
                amount: 25,
                expiry_sequence: 500,
                resolved: Some(layerx_agentd::budget::UnknownOutcome::Executed),
            },
        ),
        "requested amount",
    );
    finish_current_receipt_without_time_uses_only_a_verified_header_at_its_height_1_repair_22_39_318(endpoint, receipt, root, store);
}

fn finish_current_receipt_without_time_uses_only_a_verified_header_at_its_height_1_repair_22_39_318(
    endpoint: layerx_client::sequencer::SequencerEndpoint,
    receipt: layerx_client::rpc::receipt::AuthenticatedReceipt,
    root: std::path::PathBuf,
    store: Store,
) {
    drop(receipt);
    drop(endpoint);
    drop(store);
    let mut store = require_ok(Store::open(&root), "reopen");
    let inventory = require_ok(
        receipt::evidence_inventory(&store, &support::tenant()),
        "inventory",
    );
    let checkpoint = support::current_node::signed_checkpoint(
        150,
        require_ok(
            i64::try_from(PERIOD_START_MS / 1_000),
            "test value fits i64",
        ),
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
    let recovered = require_ok(
        recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request),
        "header recovery",
    );
    assert_eq!(recovered.recovered.budget_accounting.receipt_consumed, 25);
    assert!(recovered.recovered.budget_accounting.reconciled);
    assert!(recovered.admission.is_ok());
    // A newer budget checkpoint must never stand in for the receipt's own block.
    let checkpoint = support::current_node::signed_checkpoint(
        151,
        require_ok(
            i64::try_from(PERIOD_START_MS / 1_000),
            "test value fits i64",
        ),
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
    let held = require_ok(
        recover_tenant_budget_with_evidence(&mut store, &support::tenant(), &request),
        "missing header recovery",
    );
    assert_eq!(held.recovered.budget_accounting.receipt_consumed, 0);
    assert!(matches!(
        held.admission,
        Err(RecoveryRefusal::EvidenceMissing {
            budget_id: BUDGET_ID,
            count: 1
        })
    ));
    drop(store);
    require_ok(std::fs::remove_dir_all(root), "cleanup");
}

#[test]
fn receipt_timestamp_is_selected_before_header_time_and_is_authenticated() {
    let root = support::directory("repair-318-time-authority");
    let mut store = require_ok(Store::open(&root), "store");
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
    let mut store = require_ok(Store::open(&root), "reopen");
    let mut inventory = require_ok(
        receipt::evidence_inventory(&store, &support::tenant()),
        "inventory",
    );
    let recovered = require_ok(
        recover_tenant_budget_with_evidence(
            &mut store,
            &support::tenant(),
            &request(&inventory, PERIOD_START_MS, DAY_MS, 25),
        ),
        "recover canonical timestamp",
    );
    assert_eq!(recovered.recovered.budget_accounting.receipt_consumed, 25);
    assert!(recovered.admission.is_ok());
    let original = &inventory.with_evidence[0].evidence;
    let mut changed = original.canonical_receipt().to_vec();
    // Timestamp immediately precedes the optional signature flag and bytes.
    let timestamp_offset = changed.len() - (1 + 4 + 64) - 8;
    changed[timestamp_offset..timestamp_offset + 8]
        .copy_from_slice(&(PERIOD_START_MS - 1).to_be_bytes());
    let decoded = require_ok(
        layerx_wire::receipt::decode(&changed),
        "canonical timestamp mutation",
    );
    assert_eq!(
        require_some(decoded.protocol(), "protocol receipt").timestamp(),
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
    require_ok(std::fs::remove_dir_all(root), "cleanup");
}

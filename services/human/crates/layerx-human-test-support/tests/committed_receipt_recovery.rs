#[path = "support/committed_evidence.rs"]
mod committed_evidence;

#[test]
fn real_paxd_receipt_crosses_node_rpc_and_human_store_restart() {
    committed_evidence::committed_receipt_crosses_rpc_and_recovers();
}

#[test]
fn malformed_and_foreign_recorded_evidence_is_refused() {
    committed_evidence::recorded_malformed_and_foreign_evidence_is_refused();
}

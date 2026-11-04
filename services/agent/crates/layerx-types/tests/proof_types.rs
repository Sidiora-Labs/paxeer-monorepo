//! Producer result codes and recorded committed evidence cross the shared
//! codecs into whole current API consumer modules. No extracted declarations,
//! subprocess compiler, or historical level confers current authority.
include!("support/current_consumers.rs");

#[test]
fn proof_types_real_consumers_observe_producers_and_refuse_malformed_results() {
    proof_cases::exact_retry_codes_cross_the_real_api_consumer();
    proof_cases::current_api_status_refuses_impossible_shortfalls();
    proof_cases::committed_block_and_transaction_witness_back_current_tracking();
    proof_cases::committed_evidence_refuses_tampering();
}

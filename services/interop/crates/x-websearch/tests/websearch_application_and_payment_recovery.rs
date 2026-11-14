#[path = "support/application_proof.rs"]
mod application_proof;
#[path = "proof_x_websearch_payment.rs"]
mod payment_producer;
#[path = "support/payment_repair.rs"]
mod payment_repair;
#[path = "proof_x_websearch_program_requests.rs"]
mod program_reopen;

#[test]
fn application_proof_harness_requires_producer_owned_input(
) -> Result<(), Box<dyn std::error::Error>> {
    application_proof::explicit_input_and_transport_refusal()
}

#[test]
fn metered_test_producer_supported_offers_refuse_retired_draws_and_recover_once(
) -> Result<(), Box<dyn std::error::Error>> {
    payment_repair::supported_offers_refusals_and_recovery()
}

#[path = "support/producer_path.rs"]
mod producer_path;

#[test]
fn identity_binding_producer_transport_and_reopen() -> producer_path::Result {
    producer_path::wallet_gateway_binding_producer_transport_and_reopen()
}

#[test]
fn identity_head_merged_transport_validator_and_reopen() -> producer_path::Result {
    producer_path::merged_account_head_transport_validator_and_reopen()
}

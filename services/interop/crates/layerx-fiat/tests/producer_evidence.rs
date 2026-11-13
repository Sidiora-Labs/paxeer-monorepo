#[path = "support/producer_path.rs"]
mod producer_path;

#[test]
fn provider_recordings_authenticate() -> Result<(), Box<dyn std::error::Error>> {
    producer_path::provider_recordings_authenticate()
}

#[test]
fn fiat_payment_call_carries_provider_reference() -> Result<(), Box<dyn std::error::Error>> {
    producer_path::fiat_payment_call_carries_provider_reference()
}

#[test]
fn fiat_unit_path() -> Result<(), Box<dyn std::error::Error>> {
    producer_path::fiat_unit_path()
}

#[test]
fn fiat_resigned_vectors() -> Result<(), Box<dyn std::error::Error>> {
    producer_path::fiat_resigned_vectors()
}

#[test]
fn fiat_settlement_path() -> Result<(), Box<dyn std::error::Error>> {
    producer_path::fiat_settlement_path()
}

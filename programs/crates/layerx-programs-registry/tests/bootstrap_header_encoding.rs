#[path = "support/bootstrap_headers.rs"]
mod bootstrap;

#[test]
fn bootstrap_header_encoding_go_producer_transport_consumer(
) -> Result<(), Box<dyn std::error::Error>> {
    bootstrap::bootstrap_header_encoding_go_producer_transport_consumer()
}

#[test]
fn bootstrap_header_encoding_keeps_protocol_refusals() -> Result<(), Box<dyn std::error::Error>> {
    bootstrap::bootstrap_header_encoding_keeps_protocol_refusals()
}

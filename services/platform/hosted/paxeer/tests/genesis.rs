#[allow(
    dead_code,
    reason = "The binary entrypoint retains its published command signature."
)]
#[path = "support/genesis_transport.rs"]
mod node_transport;

#[test]
fn genesis_through_real_tls_boundary() {
    node_transport::byte_identity();
    node_transport::ordered_chunks();
    node_transport::route_refusals();
    node_transport::malformed_responses();
    node_transport::invalid_chunk_metadata();
    node_transport::size_bound();
    node_transport::truncation_and_chunk_bounds();
    node_transport::deadline();
    node_transport::unreachable_and_endpoint_refusals();
    node_transport::native_profile_consumer();
}

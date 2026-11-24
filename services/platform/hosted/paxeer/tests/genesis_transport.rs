#[allow(
    dead_code,
    reason = "The binary entrypoint retains its published command signature."
)]
#[path = "support/genesis_transport.rs"]
mod node_transport;

#[test]
#[allow(non_snake_case)]
fn genesis_byte_identity() {
    node_transport::byte_identity();
}

#[test]
#[allow(non_snake_case)]
fn genesis_ordered_chunks() {
    node_transport::ordered_chunks();
}

#[test]
#[allow(non_snake_case)]
fn genesis_route_refusals() {
    node_transport::route_refusals();
}

#[test]
#[allow(non_snake_case)]
fn genesis_malformed_responses() {
    node_transport::malformed_responses();
}

#[test]
#[allow(non_snake_case)]
fn genesis_chunk_metadata() {
    node_transport::invalid_chunk_metadata();
}

#[test]
#[allow(non_snake_case)]
fn genesis_size_bound() {
    node_transport::size_bound();
}

#[test]
#[allow(non_snake_case)]
fn genesis_truncation_and_chunk_bounds() {
    node_transport::truncation_and_chunk_bounds();
}

#[test]
#[allow(non_snake_case)]
fn genesis_deadline() {
    node_transport::deadline();
}

#[test]
#[allow(non_snake_case)]
fn genesis_endpoint_refusals() {
    node_transport::unreachable_and_endpoint_refusals();
}

#[test]
#[allow(non_snake_case)]
fn native_profile_consumer() {
    node_transport::native_profile_consumer();
}

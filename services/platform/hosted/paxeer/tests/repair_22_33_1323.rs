#[path = "support/genesis_transport.rs"]
mod node_transport;

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_GenesisByteIdentity() {
    node_transport::byte_identity();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_GenesisOrderedChunks() {
    node_transport::ordered_chunks();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_GenesisRouteRefusals() {
    node_transport::route_refusals();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_GenesisMalformedResponses() {
    node_transport::malformed_responses();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_GenesisChunkMetadata() {
    node_transport::invalid_chunk_metadata();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_GenesisSizeBound() {
    node_transport::size_bound();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_GenesisTruncationAndChunkBounds() {
    node_transport::truncation_and_chunk_bounds();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_GenesisDeadline() {
    node_transport::deadline();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_GenesisEndpointRefusals() {
    node_transport::unreachable_and_endpoint_refusals();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1323_NativeProfileConsumer() {
    node_transport::native_profile_consumer();
}

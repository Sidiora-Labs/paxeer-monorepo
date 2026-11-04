//! Task 22.33.105; mapping, scope exceptions and qualification policy are in
//! repair_22_33_105.kvx. Construction refusals name the native key defect;
//! restart coverage reopens the production buyer settlement store.

#[path = "support/repair_22_33_105.rs"]
mod path;

#[test]
fn finding_34_buyer_wire_identity_reaches_native_settlement() {
    path::buyer_wire_identity();
}
#[test]
fn finding_34_buyer_refuses_missing_plane_key() {
    path::buyer_refuses_missing_plane_key();
}
#[test]
fn finding_34_buyer_refuses_conflicting_plane_key() {
    path::buyer_refuses_conflicting_plane_key();
}
#[test]
fn finding_35_independent_consensus_anchor_and_validator_progression() {
    path::consensus_anchor_progression();
}
#[test]
fn finding_37_build_submit_settle_transport_capture() {
    path::end_to_end_native_settlement();
}
#[test]
fn finding_33_identifier_case_prefix_and_refusals() {
    path::identifier_matrix();
}
#[test]
fn finding_31_facilitator_transport_buyer_uses_unchanged_extension() {
    path::facilitator_capture();
}
#[test]
fn finding_32_renderer_vectors_reach_buyer_capture() {
    path::vector_capture();
}
#[test]
fn finding_36_reopened_transcript_peer_preserves_native_identity() {
    path::reopened_transport_consumer();
}

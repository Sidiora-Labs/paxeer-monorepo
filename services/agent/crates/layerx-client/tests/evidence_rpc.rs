#[path = "support/witness_22_33_1546.rs"]
mod repair;
use layerx_client::rpc::evidence::{self, Rung};

#[test]
fn proof_bundle_reads_eth_get_proof_and_the_landing_block() {
    repair::f03_proof_bundle_retains_every_requested_slot_identity_and_value();
    repair::f03_proof_bundle_refuses_absent_mismatched_and_uncommitted_material();
}

#[test]
fn verification_label_names_only_the_three_rungs() {
    let rungs = [Rung::Submitted, Rung::Receipted, Rung::Committed];
    let labels = rungs.map(evidence::verification_label);
    assert_eq!(labels, ["submitted", "receipted", "committed"]);
    for (rung, label) in rungs.into_iter().zip(labels) {
        assert_eq!(label, rung.name());
        assert_eq!(Rung::from_name(label), Some(rung));
    }
    for retired in [
        "state_proven",
        "checkpoint_finalised",
        "settlement_anchored",
    ] {
        assert!(!labels.contains(&retired));
        assert_eq!(Rung::from_name(retired), None);
    }
}

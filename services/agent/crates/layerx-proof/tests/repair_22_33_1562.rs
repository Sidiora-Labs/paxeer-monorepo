//! Repair record 22.33.1562, audit 22.30.98, acceptance 22998.1.
//! Base revision: e3d7e5828b39e4182e9559fb6bc28d74c2f37e0f.
//! Qualification revision, command, exit code and private log are recorded by hf-done.
//!
//! checkpoint-test-mislabels-finality: the retired checkpoint suite and its case names are deleted.
//! export-negative-controls-already-fail: export_complete::rogue_sequencer_key_and_false_key_range_are_refused_against_independent_trust,
//! export_complete::swapped_and_orphan_records_are_refused, and
//! export_complete::state_path_tamper_and_missing_maintenance_link_are_refused.
//! terminal-suite-decodes-activities: terminal_v4::program_call_transactions::program_call_transaction_binds_pinned_identity_and_payload_hash,
//! terminal_v4::program_call_transactions::program_call_transaction_is_distinct_from_executed_terminal_evidence,
//! and terminal_v4::retained_positive_mutated_transfer_leg_is_refused.
//!
//! Baseline verification was not run: the explicit gate order permits only hf-build
//! after full implementation and hf-done after that build. No prior gate ledger
//! existed at session start, so no test is claimed to have failed before the change.
//! No out-of-path implementation is needed for these refusals and receipted proofs.
//! Any future Committed success requires a real authenticated BFT commit fixture.

#[test]
fn retired_checkpoint_suite_and_mislabelled_case_names_are_deleted() {
    let tests = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("tests");
    assert!(!tests.join("checkpoint.rs").exists());
    let retired_names = [
        concat!(
            "transported_registration_",
            "never_invents_paxeer_chain_finality"
        ),
        concat!("checkpoint_vectors_", "apply_the_declared_freshness_window"),
        concat!(
            "retained_receipt_inclusion_",
            "achieves_receipted_without_committed_authority"
        ),
    ];
    let mut directories = vec![tests];
    while let Some(directory) = directories.pop() {
        for entry in std::fs::read_dir(directory).expect("proof tests directory") {
            let path = entry.expect("proof test entry").path();
            if path.is_dir() {
                directories.push(path);
            } else if path.extension().is_some_and(|extension| extension == "rs") {
                let source = std::fs::read_to_string(&path).expect("proof test source");
                for retired in retired_names {
                    assert!(
                        !source.contains(retired),
                        "{} retains {retired}",
                        path.display()
                    );
                }
            }
        }
    }
}
#[path = "export_complete.rs"]
mod export_complete;
#[path = "terminal_v4.rs"]
mod terminal_v4;

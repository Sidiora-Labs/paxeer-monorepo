//! Offline exports and terminal receipts preserve proof binding and retired-certificate refusals.

#[test]
fn retired_checkpoint_suite_and_mislabelled_case_names_are_deleted() -> std::io::Result<()> {
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
        for entry in std::fs::read_dir(directory)? {
            let path = entry?.path();
            if path.is_dir() {
                directories.push(path);
            } else if path.extension().is_some_and(|extension| extension == "rs") {
                let source = std::fs::read_to_string(&path)?;
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
    Ok(())
}
#[path = "export_complete.rs"]
mod export_complete;
#[path = "terminal_v4.rs"]
mod terminal_v4;

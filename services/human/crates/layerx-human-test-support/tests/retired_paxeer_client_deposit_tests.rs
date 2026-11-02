use std::path::Path;

use layerx_human_test_support::{human_sources_containing, human_workspace_metadata};

#[test]
fn retired_paxeer_client_deposit_tests_files_and_references_are_gone() {
    let metadata = human_workspace_metadata();
    let workspace_root = Path::new(
        metadata["workspace_root"]
            .as_str()
            .unwrap_or_else(|| panic!("cargo metadata workspace_root must be a string")),
    );
    let retired_tests = workspace_root.join("crates/layerx-paxeer-client/tests");
    for filename in ["deposit.rs", "account_vectors.rs"] {
        let path = retired_tests.join(filename);
        assert!(
            !path.exists(),
            "retired Paxeer client test remains: {}",
            path.display()
        );
    }

    let retired_path = ["layerx_paxeer_client", "deposit", "account_address"].join("::");
    let sources = human_sources_containing(&retired_path);
    assert!(
        sources.is_empty(),
        "human Rust sources still name {retired_path}: {sources:?}"
    );
}

use std::path::Path;

use layerx_human_test_support::{human_sources_containing, human_workspace_metadata};

#[test]
fn retired_explorer_index_unified_tests_files_and_references_are_gone() {
    let metadata = human_workspace_metadata();
    let workspace_root = Path::new(
        metadata["workspace_root"]
            .as_str()
            .unwrap_or_else(|| panic!("cargo metadata workspace_root must be a string")),
    );
    let retired_tests = workspace_root.join("crates/layerx-explorer-index/tests");
    for filename in ["unified_availability.rs", "unified_http.rs"] {
        let path = retired_tests.join(filename);
        assert!(
            !path.exists(),
            "retired explorer unified test remains: {}",
            path.display()
        );
    }

    let retired_path = ["layerx_explorer_index", "unified", "JoinedBalance"].join("::");
    let sources = human_sources_containing(&retired_path);
    assert!(
        sources.is_empty(),
        "human Rust sources still name {retired_path}: {sources:?}"
    );
}

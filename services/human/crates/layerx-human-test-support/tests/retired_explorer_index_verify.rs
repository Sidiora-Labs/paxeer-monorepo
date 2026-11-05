use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_explorer_index_verify_files_and_references_are_gone() {
    let metadata = human_workspace_metadata();
    let workspace_root = Path::new(
        metadata["workspace_root"]
            .as_str()
            .unwrap_or_else(|| panic!("cargo metadata workspace_root must be a string")),
    );
    let expected_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    assert_eq!(workspace_root, expected_root);

    let retired_crate = workspace_root.join("crates/layerx-explorer-index");
    for relative in ["src/verify.rs", "tests/verify.rs"] {
        let path = retired_crate.join(relative);
        assert!(
            !path.exists(),
            "retired explorer verifier file remains: {}",
            path.display()
        );
    }

    let retired_reference = ["layerx_explorer_index", "verify"].join("::");
    let references = human_sources_containing(&retired_reference);
    assert!(
        references.is_empty(),
        "Rust sources still name the retired explorer verifier: {references:?}"
    );
}

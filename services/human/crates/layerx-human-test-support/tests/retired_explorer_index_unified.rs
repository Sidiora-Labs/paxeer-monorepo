use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_explorer_index_unified_files_and_references_are_gone() {
    let metadata = human_workspace_metadata();
    let workspace_root = Path::new(
        metadata["workspace_root"]
            .as_str()
            .unwrap_or_else(|| panic!("cargo metadata must name the human workspace root")),
    );
    let expected_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error:?}"));
    assert_eq!(workspace_root, expected_root);

    let retired_file = workspace_root.join("crates/layerx-explorer-index/src/unified.rs");
    assert!(
        !retired_file.exists(),
        "retired explorer unified account join remains: {}",
        retired_file.display()
    );

    let retired_reference = ["layerx_explorer_index", "unified"].join("::");
    let references = human_sources_containing(&retired_reference);
    assert!(
        references.is_empty(),
        "Rust sources still reference the retired explorer unified account join: {references:?}"
    );
}

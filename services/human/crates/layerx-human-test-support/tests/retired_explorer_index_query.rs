use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_explorer_index_query_files_and_references_are_gone() {
    let metadata = human_workspace_metadata();
    let workspace_root = Path::new(
        metadata["workspace_root"]
            .as_str()
            .expect("cargo metadata workspace_root must be a string"),
    );
    let expected_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .expect("resolve human workspace root");
    assert_eq!(workspace_root, expected_root);

    let retired_query = workspace_root.join("crates/layerx-explorer-index/src/query.rs");
    assert!(
        !retired_query.exists(),
        "retired explorer query file remains: {}",
        retired_query.display()
    );

    let references = human_sources_containing(concat!("layerx_explorer_index", "::query"));
    assert!(
        references.is_empty(),
        "Rust sources still name the retired explorer query layer: {references:?}"
    );
}

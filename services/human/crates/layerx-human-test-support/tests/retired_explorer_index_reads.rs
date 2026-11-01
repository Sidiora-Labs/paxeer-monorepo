use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_explorer_index_reads_files_and_references_are_gone() {
    let workspace_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    let metadata = human_workspace_metadata();
    assert_eq!(metadata["workspace_root"].as_str(), workspace_root.to_str());

    for relative in [
        "crates/layerx-explorer-index/src/reads.rs",
        "crates/layerx-explorer-index/tests/reads.rs",
    ] {
        let path = workspace_root.join(relative);
        assert!(
            !path.exists(),
            "retired explorer reads file remains: {}",
            path.display()
        );
    }

    let retired_path = ["layerx_explorer_index", "reads"].join("::");
    let references = human_sources_containing(&retired_path);
    assert!(
        references.is_empty(),
        "human Rust sources still name {retired_path}: {references:?}"
    );
}

use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_explorer_index_library_files_and_references_are_gone() {
    let workspace_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    let retired_library = workspace_root.join("crates/layerx-explorer-index/src/lib.rs");
    assert!(
        !retired_library.exists(),
        "retired explorer library remains: {}",
        retired_library.display()
    );

    let metadata = human_workspace_metadata();
    assert_eq!(metadata["workspace_root"].as_str(), workspace_root.to_str());
    let packages = metadata["packages"]
        .as_array()
        .unwrap_or_else(|| panic!("cargo metadata packages must be an array"));
    assert!(
        !packages.is_empty(),
        "human workspace metadata has no packages"
    );
    for package in packages {
        let name = package["name"]
            .as_str()
            .unwrap_or_else(|| panic!("cargo metadata package name must be a string"));
        assert_ne!(
            name, "layerx-explorer-index",
            "retired explorer library remains in cargo metadata"
        );
    }

    let retired_indexer = ["layerx_explorer_index", "Indexer"].join("::");
    let references = human_sources_containing(&retired_indexer);
    assert!(
        references.is_empty(),
        "Rust sources still name {retired_indexer}: {references:?}"
    );
}

use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> PathBuf {
    let metadata = layerx_human_test_support::human_workspace_metadata();
    assert_eq!(metadata["version"], 1);
    let root = metadata["workspace_root"]
        .as_str()
        .unwrap_or_else(|| panic!("cargo metadata workspace_root must be a string"));
    let expected = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    assert_eq!(Path::new(root), expected);
    PathBuf::from(root)
}

fn human_sources_containing(needle: &str) {
    let sources = layerx_human_test_support::human_sources_containing(needle);
    assert!(
        sources.is_empty(),
        "Rust sources still reference the retired explorer mirror: {sources:?}"
    );
}

#[test]
fn retired_explorer_index_mirror_files_and_references_are_gone() {
    let workspace_root = human_workspace_metadata();
    let retired_crate = workspace_root.join("crates/layerx-explorer-index");
    for relative in ["src/freshness.rs", "src/mirror.rs", "tests/rebuild.rs"] {
        let path = retired_crate.join(relative);
        assert!(
            !path.exists(),
            "retired explorer mirror file remains: {}",
            path.display()
        );
    }

    let retired_module = ["layerx_explorer_index", "mirror"].join("::");
    human_sources_containing(&retired_module);
}

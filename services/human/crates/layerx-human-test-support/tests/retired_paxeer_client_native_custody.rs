use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_paxeer_client_native_custody_files_and_references_are_gone() {
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

    let path = workspace_root.join("crates/layerx-paxeer-client/src/native_custody.rs");
    assert!(
        !path.exists(),
        "retired Paxeer client native custody file remains: {}",
        path.display()
    );

    let retired_path = ["layerx_paxeer_client", "native_custody"].join("::");
    let references = human_sources_containing(&retired_path);
    assert!(
        references.is_empty(),
        "human Rust sources still name {retired_path}: {references:?}"
    );
}

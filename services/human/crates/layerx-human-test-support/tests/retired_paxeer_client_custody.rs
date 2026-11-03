use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_paxeer_client_custody_files_and_references_are_gone() {
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

    let retired_custody = workspace_root.join("crates/layerx-paxeer-client/src/custody.rs");
    assert!(
        !retired_custody.exists(),
        "retired Paxeer client custody file remains: {}",
        retired_custody.display()
    );

    let references = human_sources_containing(concat!("layerx_paxeer_client", "::custody"));
    assert!(
        references.is_empty(),
        "Rust sources still name the retired Paxeer client custody module: {references:?}"
    );
}

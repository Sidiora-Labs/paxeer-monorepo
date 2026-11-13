use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_paxeer_client_harness_files_and_references_are_gone() {
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

    let retired_name = ["disposable", "custody"].join("_");
    let retired_tests = workspace_root.join("crates/layerx-paxeer-client/tests");
    for extension in ["rs", "py"] {
        let path = retired_tests.join(format!("{retired_name}.{extension}"));
        assert!(
            !path.exists(),
            "retired Paxeer client harness file remains: {}",
            path.display()
        );
    }

    let references = human_sources_containing(&retired_name);
    assert!(
        references.is_empty(),
        "Rust sources still name the retired Paxeer client harness: {references:?}"
    );
}

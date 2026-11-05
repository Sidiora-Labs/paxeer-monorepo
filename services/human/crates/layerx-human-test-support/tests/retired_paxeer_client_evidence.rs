use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_paxeer_client_evidence_files_and_references_are_gone() {
    let metadata = human_workspace_metadata();
    let workspace_root = Path::new(
        metadata["workspace_root"]
            .as_str()
            .unwrap_or_else(|| panic!("cargo metadata workspace_root must be a string")),
    );
    let retired_crate = workspace_root.join("crates/layerx-paxeer-client");
    for relative in ["src/evidence.rs", "tests/custody_evidence.rs"] {
        let path = retired_crate.join(relative);
        assert!(
            !path.exists(),
            "retired Paxeer client evidence file remains: {}",
            path.display()
        );
    }

    let retired_module = ["layerx_paxeer_client", "evidence"].join("::");
    let sources = human_sources_containing(&retired_module);
    assert!(
        sources.is_empty(),
        "human Rust sources still name {retired_module}: {sources:?}"
    );
}

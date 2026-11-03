use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_paxeer_client_withdraw_files_and_references_are_gone() {
    let workspace_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    let metadata = human_workspace_metadata();
    assert_eq!(metadata["version"], 1);
    assert_eq!(metadata["workspace_root"].as_str(), workspace_root.to_str());

    let retired_crate = workspace_root.join("crates/layerx-paxeer-client");
    for relative in [
        "src/withdraw.rs",
        "tests/withdraw.rs",
        "tests/vectors/native-withdrawal-proof.json",
    ] {
        let path = retired_crate.join(relative);
        assert!(
            !path.exists(),
            "retired Paxeer client withdrawal file remains: {}",
            path.display()
        );
    }

    let retired_module = ["layerx_paxeer_client", "withdraw"].join("::");
    let references = human_sources_containing(&retired_module);
    assert!(
        references.is_empty(),
        "Rust sources still reference the retired Paxeer client withdrawal module: {references:?}"
    );
}

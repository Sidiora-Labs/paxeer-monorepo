use std::path::Path;

use layerx_human_test_support::{human_sources_containing, human_workspace_metadata};

#[test]
fn retired_paxeer_client_custody_fixtures_files_and_references_are_gone() {
    let workspace_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    let metadata = human_workspace_metadata();
    assert_eq!(metadata["workspace_root"].as_str(), workspace_root.to_str());
    let metadata_root = Path::new(
        metadata["workspace_root"]
            .as_str()
            .unwrap_or_else(|| panic!("cargo metadata workspace_root must be a string")),
    );
    let retired_crate = metadata_root.join("crates/layerx-paxeer-client");
    for relative in [
        "tests/custody_abi.rs",
        "tests/vectors/custody_abi.json",
        "tests/vectors/generate_custody_abi.sh",
        "tests/contracts/AssetRegistry.hex",
        "tests/contracts/IntegrationToken.hex",
        "tests/contracts/LayerXVault.hex",
    ] {
        let path = retired_crate.join(relative);
        assert!(
            !path.exists(),
            "retired Paxeer client custody fixture remains: {}",
            path.display()
        );
    }

    let retired_reference = ["layerx_paxeer_client", "custody", "CUSTODY_PRECOMPILE"].join("::");
    let sources = human_sources_containing(&retired_reference);
    assert!(
        sources.is_empty(),
        "human Rust sources still name {retired_reference}: {sources:?}"
    );
}

use std::path::Path;

use layerx_human_test_support::{human_sources_containing, human_workspace_metadata};

#[test]
fn workspace_metadata_lists_no_layerx_paxeer_client_package() {
    let workspace_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    let retired_crate = workspace_root.join("crates/layerx-paxeer-client");
    for relative in [
        "Cargo.toml",
        "README.md",
        "src/lib.rs",
        "examples/native_publication_fetch.rs",
    ] {
        let path = retired_crate.join(relative);
        assert!(
            !path.exists(),
            "retired Paxeer client file remains: {}",
            path.display()
        );
    }

    let identities = human_sources_containing("pub const CRATE_IDENTITY:");
    assert!(
        identities
            .iter()
            .all(|path| !path.starts_with(&retired_crate)),
        "retired Paxeer client still declares its crate identity: {identities:?}"
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
            name, "layerx-paxeer-client",
            "retired Paxeer client remains in cargo metadata"
        );
    }

    let members = metadata["workspace_members"]
        .as_array()
        .unwrap_or_else(|| panic!("cargo metadata workspace_members must be an array"));
    assert!(
        !members.is_empty(),
        "human workspace metadata has no members"
    );
    for member in members {
        let id = member
            .as_str()
            .unwrap_or_else(|| panic!("cargo metadata workspace member ID must be a string"));
        let package = packages
            .iter()
            .find(|package| package["id"].as_str() == Some(id))
            .unwrap_or_else(|| panic!("workspace member {id} has no package metadata"));
        let manifest = package["manifest_path"]
            .as_str()
            .unwrap_or_else(|| panic!("cargo metadata manifest_path must be a string"));
        assert!(
            !Path::new(manifest).starts_with(&retired_crate),
            "workspace member {id} remains under the retired Paxeer client crate: {manifest}"
        );
    }
}

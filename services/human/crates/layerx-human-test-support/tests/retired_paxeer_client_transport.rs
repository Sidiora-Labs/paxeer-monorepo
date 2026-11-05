use std::path::Path;

use layerx_human_test_support::{human_sources_containing, human_workspace_metadata};

#[test]
fn retired_paxeer_client_transport_files_and_references_are_gone() {
    let metadata = human_workspace_metadata();
    let workspace_root = metadata["workspace_root"]
        .as_str()
        .unwrap_or_else(|| panic!("cargo metadata workspace_root must be a string"));
    let expected_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    assert_eq!(Path::new(workspace_root), expected_root);

    let retired_transport =
        Path::new(workspace_root).join("crates/layerx-paxeer-client/src/client.rs");
    assert!(
        !retired_transport.exists(),
        "retired Paxeer client transport remains: {}",
        retired_transport.display()
    );

    let retired_module = ["layerx_paxeer_client", "client"].join("::");
    let references = human_sources_containing(&retired_module);
    assert!(
        references.is_empty(),
        "Rust sources still name the retired Paxeer client transport: {references:?}"
    );
}

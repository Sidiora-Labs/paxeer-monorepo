use std::path::Path;

use layerx_human_test_support::{human_sources_containing, human_workspace_metadata};

#[test]
fn retired_explorer_index_receipt_authority_files_and_references_are_gone() {
    let workspace_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    let metadata = human_workspace_metadata();
    assert_eq!(metadata["workspace_root"].as_str(), workspace_root.to_str());

    let retired_crate = workspace_root.join("crates/layerx-explorer-index");
    for relative in [
        "src/receipt_authority.rs",
        "tests/receipt_authority_reader.rs",
    ] {
        let path = retired_crate.join(relative);
        assert!(
            !path.exists(),
            "retired explorer receipt authority file remains: {}",
            path.display()
        );
    }

    let retired_path = ["layerx_explorer_index", "receipt_authority"].join("::");
    let sources = human_sources_containing(&retired_path);
    assert!(
        sources.is_empty(),
        "human Rust sources still name {retired_path}: {sources:?}"
    );
}

use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_paxeer_client_wire_files_and_references_are_gone() {
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

    let retired_crate = workspace_root.join("crates/layerx-paxeer-client");
    let retired_wire = retired_crate.join("src/wire.rs");
    assert!(
        !retired_wire.exists(),
        "retired Paxeer client wire file remains: {}",
        retired_wire.display()
    );

    let sources = human_sources_containing("");
    let wire_files: Vec<_> = sources
        .iter()
        .filter(|path| {
            path.starts_with(&retired_crate)
                && path.file_name().is_some_and(|name| name == "wire.rs")
        })
        .collect();
    assert!(
        wire_files.is_empty(),
        "retired Paxeer client wire files remain: {wire_files:?}"
    );

    let references = human_sources_containing(concat!("layerx_paxeer_client", "::wire"));
    assert!(
        references.is_empty(),
        "Rust sources still name the retired Paxeer client wire module: {references:?}"
    );
}

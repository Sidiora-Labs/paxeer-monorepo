use std::path::{Path, PathBuf};

fn human_workspace_metadata() -> serde_json::Value {
    layerx_human_test_support::human_workspace_metadata()
}

fn human_sources_containing(needle: &str) -> Vec<PathBuf> {
    layerx_human_test_support::human_sources_containing(needle)
}

#[test]
fn retired_explorer_index_config_tests_files_and_references_are_gone() {
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

    let retired_package = concat!("layerx-", "explorer-index");
    let retired_tests = workspace_root
        .join("crates")
        .join(retired_package)
        .join("tests");
    for test_name in [
        concat!("authority_", "config"),
        concat!("read_principal_", "config"),
    ] {
        let path = retired_tests.join(format!("{test_name}.rs"));
        assert!(
            !path.exists(),
            "retired explorer configuration test remains: {}",
            path.display()
        );
        let references = human_sources_containing(test_name);
        assert!(
            references.is_empty(),
            "Rust sources still name the retired explorer configuration test {test_name}: {references:?}"
        );
    }

    for launch in [
        format!("CARGO_BIN_EXE_{retired_package}"),
        format!("Command::new(\"{retired_package}\")"),
    ] {
        let references = human_sources_containing(&launch);
        assert!(
            references.is_empty(),
            "Rust sources still launch the retired explorer binary via {launch}: {references:?}"
        );
    }
}

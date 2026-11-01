use std::path::Path;

use layerx_human_test_support::{
    human_sources_containing, human_workspace_metadata, workspace_package_names,
};

#[test]
fn workspace_metadata_lists_test_support_package() {
    let workspace_root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve human workspace root: {error}"));
    let metadata = human_workspace_metadata();
    assert_eq!(metadata["version"], 1);
    assert_eq!(metadata["workspace_root"].as_str(), workspace_root.to_str());

    let names = workspace_package_names(&metadata);
    for expected in ["layerx-human-test-support", "layerx-intents"] {
        assert!(
            names.iter().any(|name| name == expected),
            "human workspace is missing {expected}: {names:?}"
        );
    }
    assert_eq!(
        names.len(),
        metadata["workspace_members"]
            .as_array()
            .unwrap_or_else(|| panic!("workspace_members must be an array"))
            .len()
    );

    let needle = "pub fn version_file_bytes(";
    let sources = human_sources_containing(needle);
    assert!(sources.contains(&workspace_root.join("crates/layerx-human-test-support/src/lib.rs")));
    for source in &sources {
        assert!(source.starts_with(workspace_root.join("crates")));
        assert_eq!(
            source.extension().and_then(|value| value.to_str()),
            Some("rs")
        );
        let text = std::fs::read_to_string(source)
            .unwrap_or_else(|error| panic!("read source {}: {error}", source.display()));
        assert!(text.contains(needle));
    }
    assert!(sources.windows(2).all(|pair| pair[0] < pair[1]));
    assert!(human_sources_containing(&String::from('\0')).is_empty());
}

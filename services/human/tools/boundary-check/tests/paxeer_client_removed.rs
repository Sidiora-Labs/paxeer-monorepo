use std::{fs, path::Path};

fn human_workspace_file(relative_path: &str) -> String {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .join(relative_path);
    fs::read_to_string(&path).unwrap_or_else(|error| panic!("read {}: {error}", path.display()))
}

#[test]
fn human_workspace_has_no_paxeer_client() {
    for relative_path in [
        "Cargo.toml",
        "crates/layerx-human-service/Cargo.toml",
        "Cargo.lock",
    ] {
        let content = human_workspace_file(relative_path);
        for retired_package in [
            "layerx-paxeer-client",
            "layerx-explorer-index",
            "layerx-human-movement-provider",
        ] {
            assert!(
                !content.contains(retired_package),
                "{relative_path} still names retired package {retired_package}"
            );
        }
    }
}

#[test]
fn human_workspace_has_no_protocol_adapter() {
    for relative_path in ["Cargo.toml", "Cargo.lock"] {
        let content = human_workspace_file(relative_path);
        assert!(
            !content.contains("layerx-programs-protocol-adapter"),
            "{relative_path} still names layerx-programs-protocol-adapter"
        );
    }
}

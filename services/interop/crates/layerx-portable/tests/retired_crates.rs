use std::path::Path;
use std::process::Command;

use serde::Deserialize;

const RETIRED_PACKAGE: &str = "layerx-intent-ingester";

#[derive(Deserialize)]
struct Metadata {
    packages: Vec<Package>,
    workspace_members: Vec<String>,
}

#[derive(Deserialize)]
struct Package {
    id: String,
    name: String,
    dependencies: Vec<Dependency>,
}

#[derive(Deserialize)]
struct Dependency {
    name: String,
}

fn interop_metadata() -> Metadata {
    let manifest = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../Cargo.toml");
    let output = Command::new(env!("CARGO"))
        .args(["metadata", "--locked", "--no-deps", "--format-version", "1"])
        .arg("--manifest-path")
        .arg(&manifest)
        .output()
        .unwrap_or_else(|error| panic!("run cargo metadata for {}: {error}", manifest.display()));
    assert!(
        output.status.success(),
        "cargo metadata failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let metadata: Metadata = serde_json::from_slice(&output.stdout)
        .unwrap_or_else(|error| panic!("decode cargo metadata: {error}"));
    assert!(!metadata.workspace_members.is_empty(), "workspace is empty");
    for member in &metadata.workspace_members {
        assert!(
            metadata
                .packages
                .iter()
                .any(|package| &package.id == member),
            "workspace member {member} is missing from metadata"
        );
    }
    metadata
}

#[test]
fn the_interop_workspace_has_no_intent_ingester_package() {
    let metadata = interop_metadata();
    for package in &metadata.packages {
        assert_ne!(
            package.name, RETIRED_PACKAGE,
            "retired package remains in the interop workspace"
        );
    }
}

#[test]
fn no_interop_crate_depends_on_the_intent_ingester() {
    let metadata = interop_metadata();
    for package in &metadata.packages {
        if metadata.workspace_members.contains(&package.id) {
            for dependency in &package.dependencies {
                assert_ne!(
                    dependency.name, RETIRED_PACKAGE,
                    "{} still depends on the retired ingester",
                    package.name
                );
            }
        }
    }
}

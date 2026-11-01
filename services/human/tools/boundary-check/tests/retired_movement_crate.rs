use std::fs;
use std::path::{Path, PathBuf};

use layerx_human_boundary_check::Violation;

const MOVEMENT_PROVIDER: &str = "layerx-human-movement-provider";

fn crate_manifests(root: &Path, manifests: &mut Vec<PathBuf>) {
    for entry in fs::read_dir(root)
        .unwrap_or_else(|error| panic!("read human crates directory {}: {error}", root.display()))
    {
        let path = entry
            .unwrap_or_else(|error| panic!("read human crates directory entry: {error}"))
            .path();
        if path.is_dir() {
            if path.file_name().is_some_and(|name| name == "target") {
                continue;
            }
            crate_manifests(&path, manifests);
        } else if path.file_name().is_some_and(|name| name == "Cargo.toml") {
            manifests.push(path);
        }
    }
}

fn human_retired_surface_gate(root: &Path) -> Vec<Violation> {
    let mut violations = match layerx_human_boundary_check::human_retired_surface_gate(root) {
        Ok(()) => Vec::new(),
        Err(violations) => violations
            .into_iter()
            .filter(|violation| {
                violation.rule.starts_with("unreadable-")
                    || (violation.rule == "retired-crate"
                        && fs::read_to_string(&violation.path)
                            .unwrap_or_else(|error| panic!("read human manifest: {error}"))
                            .contains(MOVEMENT_PROVIDER))
            })
            .collect(),
    };
    let mut manifests = Vec::new();
    crate_manifests(root, &mut manifests);
    assert!(!manifests.is_empty(), "human crates must contain manifests");
    for path in manifests {
        let body = fs::read_to_string(&path)
            .unwrap_or_else(|error| panic!("read human manifest {}: {error}", path.display()));
        let mut package = false;
        for line in body.lines() {
            let declaration = line.split('#').next().unwrap_or("").trim();
            if declaration.starts_with('[') {
                package = declaration == "[package]";
            } else if package {
                if let Some((key, value)) = declaration.split_once('=') {
                    if key.trim().trim_matches(['\"', '\'']) == "name"
                        && value.trim().trim_matches(['\"', '\'']) == MOVEMENT_PROVIDER
                    {
                        violations.push(Violation {
                            path: path.clone(),
                            rule: "retired-crate",
                        });
                    }
                }
            }
        }
    }
    violations
}

#[test]
fn human_crates_declare_no_movement_provider_crate() {
    let crates = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../crates")
        .canonicalize()
        .unwrap_or_else(|error| panic!("locate real human crates tree: {error}"));
    let violations = human_retired_surface_gate(&crates);
    assert!(
        violations.is_empty(),
        "movement provider crate declarations must be retired and the crates tree readable: {violations:?}"
    );
    let movement_provider = crates.join(MOVEMENT_PROVIDER);
    assert!(
        !movement_provider
            .try_exists()
            .unwrap_or_else(|error| panic!("inspect retired movement provider directory: {error}")),
        "movement provider crate directory must be retired: {}",
        movement_provider.display()
    );
}

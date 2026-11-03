use std::fs;
use std::path::{Path, PathBuf};

use layerx_human_boundary_check::Violation;

const RETIRED_CRATES: &[&str] = &[
    "layerx-human-movement-provider",
    "layerx-paxeer-client",
    "layerx-explorer-index",
];

fn crate_manifests(root: &Path, manifests: &mut Vec<PathBuf>) {
    for entry in fs::read_dir(root)
        .unwrap_or_else(|error| panic!("read human directory {}: {error}", root.display()))
    {
        let path = entry
            .unwrap_or_else(|error| panic!("read human directory entry: {error}"))
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
                violation.rule == "retired-crate" || violation.rule.starts_with("unreadable-")
            })
            .collect(),
    };
    let mut manifests = Vec::new();
    crate_manifests(root, &mut manifests);
    assert!(
        manifests.contains(&root.join("Cargo.toml")),
        "human workspace manifest must exist"
    );
    manifests.push(root.join("Cargo.lock"));
    for path in manifests {
        let body = fs::read_to_string(&path).unwrap_or_else(|error| {
            panic!(
                "read human manifest or lockfile {}: {error}",
                path.display()
            )
        });
        if RETIRED_CRATES.iter().any(|name| body.contains(name)) {
            violations.push(Violation {
                path,
                rule: "retired-crate",
            });
        }
    }
    violations
}

#[test]
fn human_workspace_has_no_retired_member_or_dependency() {
    let human = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("locate real human workspace: {error}"));
    let violations = human_retired_surface_gate(&human);
    assert!(
        violations.is_empty(),
        "human workspace must have no retired crate and must be readable: {violations:?}"
    );
}

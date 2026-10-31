use std::path::Path;

use layerx_human_boundary_check::Violation;

fn human_retired_surface_gate(root: &Path) -> Vec<Violation> {
    match layerx_human_boundary_check::human_retired_surface_gate(root) {
        Ok(()) => Vec::new(),
        Err(violations) => violations
            .into_iter()
            .filter(|violation| {
                violation.rule.starts_with("unreadable-")
                    || (violation.rule == "retired-module"
                        && violation.path.ends_with("binding/mod.rs"))
            })
            .collect(),
    }
}

#[test]
fn human_crates_have_no_wallet_binding_module() {
    let crates = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../crates")
        .canonicalize()
        .unwrap_or_else(|error| panic!("locate real human crates tree: {error}"));
    let violations = human_retired_surface_gate(&crates);
    assert!(
        violations.is_empty(),
        "wallet binding modules must be retired and the crates tree readable: {violations:?}"
    );
}

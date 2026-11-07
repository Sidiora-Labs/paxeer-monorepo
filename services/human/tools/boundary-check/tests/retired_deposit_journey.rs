use std::path::Path;

use layerx_human_boundary_check::Violation;

fn human_retired_surface_gate(root: &Path) -> Vec<Violation> {
    match layerx_human_boundary_check::human_retired_surface_gate(root) {
        Ok(()) => Vec::new(),
        Err(violations) => {
            assert!(
                violations
                    .iter()
                    .all(|violation| !violation.rule.starts_with("unreadable-")),
                "retired-surface gate could not read the human crates: {violations:?}"
            );
            violations
                .into_iter()
                .filter(|violation| {
                    violation.rule == "retired-module"
                        && violation.path.ends_with("journeys/deposit.rs")
                })
                .collect()
        }
    }
}

#[test]
fn human_crates_have_no_deposit_journey() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../crates");
    let violations = human_retired_surface_gate(&root);
    assert!(
        violations.is_empty(),
        "retired deposit journey sources remain under {}: {violations:?}",
        root.display()
    );
}

use std::path::Path;

use layerx_human_boundary_check::Violation;

fn human_retired_surface_gate() -> Result<(), Vec<Violation>> {
    let crates = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../crates");
    layerx_human_boundary_check::human_retired_surface_gate(&crates)
}

#[test]
fn human_crates_have_no_exit_journey() {
    if let Err(violations) = human_retired_surface_gate() {
        let exit_violations: Vec<_> = violations
            .into_iter()
            .filter(|violation| {
                (violation.rule == "retired-module" && violation.path.ends_with("journeys/exit.rs"))
                    || matches!(
                        violation.rule,
                        "unreadable-human-tree" | "unreadable-human-file"
                    )
            })
            .collect();
        assert!(
            exit_violations.is_empty(),
            "retired exit journey scan failed: {exit_violations:?}"
        );
    }
}

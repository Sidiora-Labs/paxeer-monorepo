use std::path::Path;

fn human_retired_surface_gate(root: &Path) {
    if let Err(violations) = layerx_human_boundary_check::human_retired_surface_gate(root) {
        let violations: Vec<_> = violations
            .into_iter()
            .filter(|violation| {
                (violation.rule == "retired-module"
                    && violation.path.ends_with("journeys/withdraw.rs"))
                    || violation.rule.starts_with("unreadable-")
            })
            .collect();
        assert!(
            violations.is_empty(),
            "withdraw journey remains or the human crates tree could not be read: {violations:?}"
        );
    }
}

#[test]
fn human_crates_have_no_withdraw_journey() {
    let crates = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../crates");
    human_retired_surface_gate(&crates);
}

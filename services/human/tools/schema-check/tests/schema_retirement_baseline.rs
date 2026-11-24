#[path = "retired_binding_goldens.rs"]
mod retired_binding_goldens;
#[path = "retired_markers.rs"]
mod retired_markers;

#[test]
fn refreshed_real_schema_baseline_matches_current_declarations() {
    use layerx_human_schema_check::{baseline_manifest, human_api_schema, BASELINE_FILE};
    use std::fs;
    use std::path::Path;

    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/human-api");
    let manifest = baseline_manifest(&root)
        .unwrap_or_else(|violations| panic!("real schema baseline: {violations:#?}"));
    let frozen = fs::read_to_string(root.join(BASELINE_FILE))
        .unwrap_or_else(|error| panic!("read frozen baseline: {error}"));
    assert_eq!(
        frozen, manifest,
        "baseline must match the deliberate refresh"
    );
    let report = human_api_schema(&root)
        .unwrap_or_else(|violations| panic!("real schema conformance: {violations:#?}"));
    assert_eq!(report.additions_beyond_baseline, 0);
}

#[test]
fn binding_statement_has_complete_retired_failure() {
    retired_binding_goldens::assert_retired_binding_failure("binding.statement");
}

#[test]
fn binding_submit_has_complete_retired_failure() {
    retired_binding_goldens::assert_retired_binding_failure("binding.submit");
}

#[test]
fn binding_status_has_complete_retired_failure() {
    retired_binding_goldens::assert_retired_binding_failure("binding.status");
}

#[test]
fn binding_rebind_action_has_complete_retired_failure() {
    retired_binding_goldens::assert_retired_binding_failure("binding.rebind.action");
}

#[test]
fn binding_rebind_has_complete_retired_failure() {
    retired_binding_goldens::assert_retired_binding_failure("binding.rebind");
}

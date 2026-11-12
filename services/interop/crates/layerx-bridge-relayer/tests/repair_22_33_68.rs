#[path = "outbound_isolation.rs"]
mod outbound_isolation;
#[allow(dead_code)]
#[path = "support/repair_22_33_68.rs"]
mod support;

use sha2::{Digest, Sha256};
use std::path::Path;

#[test]
fn attestation_policy_coverage_survives_restart_direction_changes_and_rotation() {
    support::policy_restart_and_rotation();
}

#[test]
fn operator_root_fixture_loads_and_posts_through_pinned_tls() {
    support::configured_root_transport();
}

#[test]
fn cosign_root_fixture_loads_and_posts_through_pinned_tls() {
    support::configured_root_transport();
}

#[test]
fn quorum_coverage_refuses_tls_authority_and_certificate_aliases() {
    support::tls_aliases_are_not_independent_votes();
}

#[test]
fn capture_prerequisite_reports_missing_inputs_and_refuses_required_missing_or_unpinned_artifacts()
{
    use outbound_isolation::capture_prerequisite::{prerequisite, Disposition, MISSING};
    assert_eq!(
        prerequisite(None, None, false),
        Ok(Disposition::MissingPrerequisite(MISSING))
    );
    assert!(MISSING.contains("NOT PROVEN"));
    assert!(prerequisite(None, None, true)
        .err()
        .unwrap_or_else(|| panic!("expected Err value"))
        .contains("LAYERX_BRIDGE_CAPTURE_MANIFEST"));
    let directory = support::Directory::new();
    let path = directory.0.join("capture.json");
    let pin = "a".repeat(64);
    assert!(prerequisite(Some(&path), None, false)
        .err()
        .unwrap_or_else(|| panic!("expected Err value"))
        .contains("SHA256"));
    assert!(prerequisite(None, Some(&pin), false)
        .err()
        .unwrap_or_else(|| panic!("expected Err value"))
        .contains("MANIFEST"));
    for required in [true, false] {
        assert!(prerequisite(Some(Path::new("relative.json")), Some(&pin), required).is_err());
        assert!(prerequisite(Some(&path), Some("bad-pin"), required).is_err());
        assert_eq!(
            prerequisite(Some(&path), Some(&pin), required),
            Ok(Disposition::Required {
                manifest: &path,
                pin: &pin
            })
        );
        assert!(outbound_isolation::validate_capture_manifest(&path, &pin).is_err());
    }
    let bytes = br#"{"schema":"layerx.bridge.outbound-isolation/v1"}"#;
    std::fs::write(&path, bytes).unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    assert!(outbound_isolation::validate_capture_manifest(&path, &pin)
        .err()
        .unwrap_or_else(|| panic!("expected Err value"))
        .to_string()
        .contains("SHA256"));
    let digest = layerx_bridge_relayer::hex::encode(&Sha256::digest(bytes));
    assert!(
        outbound_isolation::validate_capture_manifest(&path, &digest)
            .err()
            .unwrap_or_else(|| panic!("expected Err value"))
            .to_string()
            .contains("missing field")
    );
}

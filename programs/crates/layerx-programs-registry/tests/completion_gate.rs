use std::collections::BTreeMap;
use std::fs;
use std::io::ErrorKind;
use std::path::Path;

const ADAPTER_FFI: &str = "programs/crates/layerx-programs-protocol-adapter/src/ffi.rs";
const REGISTRY_EVIDENCE: &str = "programs/crates/layerx-programs-registry/src/protocol_evidence.rs";
const RUNTIME_FFI: &str = "programs/crates/layerx-programs-runtime/src/ffi_call.rs";

const RETIRED_SURFACES: &[(&str, &str, Option<&str>)] = &[
    ("interface.13.c-pad-lxp-programs-account-state-head-read", ADAPTER_FFI, None),
    ("interface.13.c-pad-lxp-programs-exit-route-iter", ADAPTER_FFI, None),
    ("interface.13.c-pad-lxp-programs-value-account-iter", ADAPTER_FFI, None),
    ("interface.13.c-pad-lxp-programs-wind-down-history-iter", ADAPTER_FFI, None),
    ("interface.13.c-pad-lxp-programs-wind-down-read", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-caccount", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-caccountstatehead", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-camount", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-cbinding", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-cexitroute", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-chistory", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-cproof", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-cvalueaccountview", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-cwinddownview", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocoladaptererror", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-account-profile2", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-account-snapshot", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-balances", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-canonical-encode", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-history", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-into-balances", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-program", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-restore-verified", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-restore-verified-profile2", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-protocolprogramstateread-routes", ADAPTER_FFI, None),
    ("interface.13.rs-pad-ffi-read-program-state", ADAPTER_FFI, None),
    ("interface.13.rs-reg-protocol-evidence-deploymentmaintenanceproof", REGISTRY_EVIDENCE, Some("DeploymentMaintenanceProof")),
    ("interface.13.rs-reg-protocol-evidence-protocoldeploymentverifier-from-protected-history", REGISTRY_EVIDENCE, Some("from_protected_history")),
    ("interface.13.rs-reg-protocol-evidence-protocoldeploymentverifier-verify-current-maintenance-head", REGISTRY_EVIDENCE, Some("verify_current_maintenance_head")),
    ("interface.13.rs-reg-protocol-evidence-protocoldeploymentverifier-verify-current-maintenance-head-typed", REGISTRY_EVIDENCE, Some("verify_current_maintenance_head_typed")),
    ("interface.13.rs-reg-protocol-evidence-protocoldeploymentverifier-verify-historical-maintenance-head", REGISTRY_EVIDENCE, Some("verify_historical_maintenance_head")),
    ("interface.13.rs-reg-protocol-evidence-protocoldeploymentverifier-verify-historical-maintenance-head-typed", REGISTRY_EVIDENCE, Some("verify_historical_maintenance_head_typed")),
    ("interface.13.rs-reg-protocol-evidence-protocoldeploymentverifier-with-signed-history", REGISTRY_EVIDENCE, Some("with_signed_history")),
    ("interface.13.rs-reg-protocol-evidence-protocolheadmaintenanceproof", REGISTRY_EVIDENCE, Some("ProtocolHeadMaintenanceProof")),
    ("interface.13.rs-reg-protocol-evidence-verifiedchainhead-batch-header-digest", REGISTRY_EVIDENCE, Some("batch_header_digest")),
    ("interface.13.rs-reg-protocol-evidence-verifiedchainhead-global-sequence", REGISTRY_EVIDENCE, Some("global_sequence")),
    ("interface.13.rs-reg-protocol-evidence-verifiedchainhead-sequencer-public-key", REGISTRY_EVIDENCE, Some("sequencer_public_key")),
    ("interface.13.rs-reg-protocol-evidence-verifieddeploymentevidence-batch-header-digest", REGISTRY_EVIDENCE, Some("batch_header_digest")),
    ("interface.13.rs-reg-protocol-evidence-verifiedmaintenancehead", REGISTRY_EVIDENCE, Some("VerifiedMaintenanceHead")),
    ("interface.13.rs-reg-protocol-evidence-verifiedmaintenancehead-account-state-head", REGISTRY_EVIDENCE, Some("VerifiedMaintenanceHead")),
    ("interface.13.rs-reg-protocol-evidence-verifiedmaintenancehead-batch-header-digest", REGISTRY_EVIDENCE, Some("VerifiedMaintenanceHead")),
    ("interface.13.rs-reg-protocol-evidence-verifiedmaintenancehead-freshness", REGISTRY_EVIDENCE, Some("VerifiedMaintenanceHead")),
    ("interface.13.rs-reg-protocol-evidence-verifiedmaintenancehead-maintenance-receipt-sha256", REGISTRY_EVIDENCE, Some("VerifiedMaintenanceHead")),
    ("interface.13.rs-reg-protocol-evidence-verifiedmaintenancehead-sequencer-public-key", REGISTRY_EVIDENCE, Some("VerifiedMaintenanceHead")),
    ("interface.13.rs-reg-protocol-evidence-verifiedmaintenancehead-state-root", REGISTRY_EVIDENCE, Some("VerifiedMaintenanceHead")),
    ("interface.13.rs-reg-protocol-evidence-verifiedprotocolhead-batch-header-digest", REGISTRY_EVIDENCE, Some("batch_header_digest")),
    ("interface.13.rs-reg-protocol-evidence-verifiedprotocolhead-sequencer-public-key", REGISTRY_EVIDENCE, Some("sequencer_public_key")),
    ("interface.13.rs-rt-ffi-call-raw-native-receipt-authority-v1-bytes", RUNTIME_FFI, Some("RAW_NATIVE_RECEIPT_AUTHORITY_V1_BYTES")),
    ("interface.13.rs-rt-ffi-call-rawnativereceiptauthorityv1", RUNTIME_FFI, Some("RawNativeReceiptAuthorityV1")),
    ("interface.13.rs-rt-ffi-call-rawnativereceiptauthorityv1-canonical-bytes", RUNTIME_FFI, Some("RawNativeReceiptAuthorityV1")),
];

const PLANNED_PATHS: &[&str] = &[
    "precompiles/layerxprograms/conservation_test.go",
    "precompiles/layerxprograms/occ_test.go",
    "programs/crates/layerx-programs-registry/tests/chain_fixture.rs",
    "programs/crates/layerx-programs-registry/tests/chain_verifier_switch.rs",
    "programs/fixtures/pay5/test_native_roundtrip.py",
    "programs/fixtures/test_workspace_graph.py",
];

#[test]
fn programs_completion_retired_surfaces_absent() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(3)
        .unwrap_or_else(|| panic!("{}", "registry crate must be inside the Programs workspace"));
    let mut failures = Vec::new();
    let mut sources = BTreeMap::new();

    for &(interface, relative_path, retired_identifier) in RETIRED_SURFACES {
        let path = root.join(relative_path);
        match fs::symlink_metadata(&path) {
            Err(error) if error.kind() == ErrorKind::NotFound => continue,
            Err(error) => {
                failures.push(format!(
                    "Cannot inspect {interface} at {relative_path}: {error}"
                ));
                continue;
            }
            Ok(_) => {}
        }
        let Some(identifier) = retired_identifier else {
            failures.push(format!(
                "Retired surface {interface} remains at {relative_path}"
            ));
            continue;
        };
        let source = sources
            .entry(relative_path)
            .or_insert_with(|| fs::read_to_string(&path));
        match source {
            Ok(source) => {
                if source
                    .split(|character: char| !character.is_alphanumeric() && character != '_')
                    .any(|token| token == identifier)
                {
                    failures.push(format!(
                        "Retired surface {interface} ({identifier}) remains at {relative_path}"
                    ));
                }
            }
            Err(error) => failures.push(format!(
                "Cannot read {interface} at {relative_path}: {error}"
            )),
        }
    }

    for relative_path in PLANNED_PATHS {
        match fs::metadata(root.join(relative_path)) {
            Ok(metadata) if metadata.is_file() => {}
            Ok(_) => failures.push(format!(
                "Planned path is not a regular file: {relative_path}"
            )),
            Err(error) => failures.push(format!("Missing planned path {relative_path}: {error}")),
        }
    }

    assert!(
        failures.is_empty(),
        "Domain 13 completion blocked:\n{}",
        failures.join("\n")
    );
}

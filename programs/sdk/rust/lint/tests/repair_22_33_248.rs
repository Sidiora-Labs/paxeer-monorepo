use layerx_program_lint::{lint_artifact_for_abi, DeterminismViolation};
use layerx_programs_runtime::{
    ValidationRefusal, WasmEngine, ABI_V1_VERSION, ABI_V2_VERSION, ABI_V3_VERSION, ABI_V4_MODULE,
    ABI_V4_VERSION, ABI_V5_MODULE, ABI_V5_VERSION,
};

#[path = "support/abi_v5.rs"]
mod abi_v5;

#[test]
fn finding_016_v5_market_guest_reaches_import_validation() {
    let wasm = abi_v5::program_importing(ABI_V5_MODULE, "market_step_adjudicate", 6);
    let engine = WasmEngine::declared().expect("declared runtime engine");
    assert!(engine.validate_v5(&wasm).is_ok());
    assert_eq!(lint_artifact_for_abi(&wasm, ABI_V5_VERSION), Vec::new());
}

#[test]
fn finding_016_market_import_is_refused_before_v5() {
    let wasm = abi_v5::program_importing(ABI_V5_MODULE, "market_step_adjudicate", 6);
    for version in [
        ABI_V1_VERSION,
        ABI_V2_VERSION,
        ABI_V3_VERSION,
        ABI_V4_VERSION,
    ] {
        let violations = lint_artifact_for_abi(&wasm, version);
        assert!(
            violations.contains(&DeterminismViolation::UndeclaredHostImport {
                import_module: ABI_V5_MODULE.to_string(),
                import_name: "market_step_adjudicate".to_string(),
            }),
            "v{version} admitted the v5 import: {violations:?}"
        );
    }
}

#[test]
fn finding_016_v5_rejects_wrong_market_signature() {
    let wasm = abi_v5::program_importing(ABI_V5_MODULE, "market_step_adjudicate", 4);
    let engine = WasmEngine::declared().expect("declared runtime engine");
    assert!(matches!(
        engine.validate_v5(&wasm),
        Err(ValidationRefusal::WrongImportSignature { import_name })
            if import_name == "market_step_adjudicate"
    ));
    let violations = lint_artifact_for_abi(&wasm, ABI_V5_VERSION);
    assert!(
        violations.contains(&DeterminismViolation::RejectedByEngine {
            reason: "invalid LayerX ABI import market_step_adjudicate".to_string(),
        }),
        "wrong signature did not reach import validation: {violations:?}"
    );
}

#[test]
fn finding_016_v5_preserves_determinism_and_namespace_refusals() {
    let engine = WasmEngine::declared().expect("declared runtime engine");
    for (module, name, expected) in [
        (
            ABI_V5_MODULE,
            "now",
            DeterminismViolation::ClockImport {
                import_module: ABI_V5_MODULE.to_string(),
                import_name: "now".to_string(),
            },
        ),
        (
            ABI_V5_MODULE,
            "random_get",
            DeterminismViolation::RandomnessImport {
                import_module: ABI_V5_MODULE.to_string(),
                import_name: "random_get".to_string(),
            },
        ),
        (
            ABI_V4_MODULE,
            "market_step_adjudicate",
            DeterminismViolation::UndeclaredHostImport {
                import_module: ABI_V4_MODULE.to_string(),
                import_name: "market_step_adjudicate".to_string(),
            },
        ),
        (
            ABI_V5_MODULE,
            "web_read",
            DeterminismViolation::UndeclaredHostImport {
                import_module: ABI_V5_MODULE.to_string(),
                import_name: "web_read".to_string(),
            },
        ),
    ] {
        let wasm = abi_v5::program_importing(module, name, 6);
        assert!(matches!(
            engine.validate_v5(&wasm),
            Err(ValidationRefusal::ForbiddenImport { import_module, import_name })
                if import_module == module && import_name == name
        ));
        let violations = lint_artifact_for_abi(&wasm, ABI_V5_VERSION);
        assert!(
            violations.contains(&expected),
            "{module}::{name} did not reach determinism validation: {violations:?}"
        );
    }
}

#[test]
fn finding_016_unsupported_versions_remain_explicit() {
    let wasm = abi_v5::program_importing(ABI_V5_MODULE, "market_step_adjudicate", 6);
    for version in [0, 6, u16::MAX] {
        assert_eq!(
            lint_artifact_for_abi(&wasm, version),
            vec![DeterminismViolation::RejectedByEngine {
                reason: format!("unsupported LayerX ABI version {version}"),
            }]
        );
    }
}

use std::fmt::Debug;
use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_program_lint::{
    abi_surface_violations,
    cli::{parse_version, project_violations},
    lint_artifact_for_abi, lint_project_for_abi, DeterminismViolation,
};
use layerx_program_sdk::abi::{V5_ABI_MODULE, V5_HOST_FUNCTIONS};
use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, import_section, module, padding_section,
    raw_section, type_section, unsigned_leb, OP_CALL, OP_DROP, OP_END, OP_F32_CONST, OP_I32_CONST,
    OP_LOCAL_GET, TYPE_F32, TYPE_F64, TYPE_I32, TYPE_I64, TYPE_V128,
};
use layerx_programs_runtime::{
    admit_abi_version, AbiRevision, HostFunction, ValidationRefusal, WasmEngine, ABI_V1_MODULE,
    ABI_V2_HOST_FUNCTIONS, ABI_V2_MODULE, ABI_V3_HOST_FUNCTIONS, ABI_V3_MODULE,
    ABI_V4_HOST_FUNCTIONS, ABI_V4_MODULE, ABI_V5_HOST_FUNCTIONS, ABI_V5_MODULE, HOST_FUNCTIONS,
};

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("real qualification path refused: {error:?}"))
}

fn exports(imports: u32) -> Vec<u8> {
    let mut payload = unsigned_leb(3);
    for (name, kind, index) in [
        (layerx_program_sdk::CALL_ENTRY_EXPORT, 0, imports),
        (layerx_program_sdk::CALL_RESERVE_EXPORT, 0, imports + 1),
        (layerx_program_sdk::MEMORY_EXPORT, 2, 0),
    ] {
        payload.extend(unsigned_leb(name.len() as u64));
        payload.extend_from_slice(name.as_bytes());
        payload.push(kind);
        payload.extend(unsigned_leb(u64::from(index)));
    }
    raw_section(7, &payload)
}

// Committed fixture recipe using the runtime's binary codec and the SDK's
// frozen declarations. Both validators consume exactly these emitted bytes.
fn guest(
    imports: &[(&str, &str, u32)],
    params: &[u8],
    result: u8,
    extra_type: &[u8],
    prefix: &[u8],
) -> Vec<u8> {
    let mut types = vec![
        (params, std::slice::from_ref(&result)),
        (&[TYPE_I32, TYPE_I32], &[TYPE_I32]),
        (&[TYPE_I32], &[TYPE_I32]),
    ];
    if !extra_type.is_empty() {
        types.push((extra_type, &[]));
    }
    let mut call = prefix.to_vec();
    if !imports.is_empty() {
        for param in params {
            call.extend_from_slice(&[
                if *param == TYPE_I64 {
                    0x42
                } else {
                    OP_I32_CONST
                },
                0,
            ]);
        }
        call.extend_from_slice(&[OP_CALL, 0, OP_DROP]);
    }
    call.extend_from_slice(&[OP_I32_CONST, 0, OP_END]);
    module(&[
        type_section(&types),
        import_section(imports),
        function_section(&[1, 2]),
        raw_section(5, &[1, 1, 1, 1]),
        exports(imports.len() as u32),
        code_section(&[
            func_body(&[], &call),
            func_body(&[], &[OP_LOCAL_GET, 0, OP_END]),
        ]),
    ])
}

fn signature_types(function: &HostFunction) -> (Vec<u8>, u8) {
    let (params, result) = checked(function.signature.split_once(")->").ok_or("signature"));
    let value_type = |name| match name {
        "i32" => TYPE_I32,
        "i64" => TYPE_I64,
        _ => panic!("unexpected frozen integer type {name}"),
    };
    let params = params
        .trim_start_matches('(')
        .split(',')
        .filter(|name| !name.is_empty())
        .map(value_type)
        .collect();
    (params, value_type(result))
}

fn v5_guest() -> Vec<u8> {
    let function = &V5_HOST_FUNCTIONS[0];
    let (params, result) = signature_types(&HostFunction {
        name: function.name,
        signature: function.signature,
    });
    guest(
        &[(V5_ABI_MODULE, function.name, 0)],
        &params,
        result,
        &[],
        &[],
    )
}

fn undeclared(module: &str, name: &str) -> DeterminismViolation {
    DeterminismViolation::UndeclaredHostImport {
        import_module: module.to_string(),
        import_name: name.to_string(),
    }
}

struct Project(PathBuf);

impl Project {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-247-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        checked(fs::create_dir(&path));
        checked(fs::write(
            path.join("Cargo.toml"),
            "[package]\nname = \"guest\"\n",
        ));
        checked(fs::write(
            path.join("layerx-program.json"),
            "{\"abi_version\":5}\n",
        ));
        checked(fs::write(path.join("guest.wasm"), v5_guest()));
        Self(path)
    }
}

impl Drop for Project {
    fn drop(&mut self) {
        checked(fs::remove_dir_all(&self.0));
    }
}

#[test]
fn finding_015_v5_sdk_import_reaches_the_real_validator_and_linker() {
    assert_eq!(V5_ABI_MODULE, ABI_V5_MODULE);
    assert_eq!(V5_HOST_FUNCTIONS[0].name, "market_step_adjudicate");
    assert_eq!(
        V5_HOST_FUNCTIONS[0].signature,
        "(i32,i32,i32,i32,i32,i32)->i32"
    );
    assert_eq!(abi_surface_violations(), Vec::new());
    let wasm = v5_guest();
    let engine = checked(WasmEngine::declared());
    let validated = checked(engine.validate_v5(&wasm));
    assert_eq!(validated.abi_revision(), AbiRevision::V5);
    assert_eq!(validated.byte_size(), wasm.len() as u64);
    assert!(validated.exports_callable_entrypoint(layerx_program_sdk::CALL_ENTRY_EXPORT));
    checked(validated.instantiate_for_qualification());
    assert_eq!(lint_artifact_for_abi(&wasm, 5), Vec::new());
    for version in 1..5 {
        assert!(lint_artifact_for_abi(&wasm, version)
            .contains(&undeclared(V5_ABI_MODULE, V5_HOST_FUNCTIONS[0].name)));
    }
}

#[test]
fn finding_015_import_admission_matches_every_runtime_namespace() {
    let engine = checked(WasmEngine::declared());
    for (introduced, namespace, functions) in [
        (1, ABI_V1_MODULE, HOST_FUNCTIONS.as_slice()),
        (2, ABI_V2_MODULE, ABI_V2_HOST_FUNCTIONS.as_slice()),
        (3, ABI_V3_MODULE, ABI_V3_HOST_FUNCTIONS.as_slice()),
        (4, ABI_V4_MODULE, ABI_V4_HOST_FUNCTIONS.as_slice()),
        (5, ABI_V5_MODULE, ABI_V5_HOST_FUNCTIONS.as_slice()),
    ] {
        for function in functions {
            let (params, result) = signature_types(function);
            let wasm = guest(&[(namespace, function.name, 0)], &params, result, &[], &[]);
            for version in 1..=5 {
                assert!(admit_abi_version(version).is_ok());
                let violations = lint_artifact_for_abi(&wasm, version);
                if version >= introduced {
                    let validated = checked(engine.validate_versioned(version, &wasm));
                    checked(validated.instantiate_for_qualification());
                    assert_eq!(
                        violations,
                        Vec::new(),
                        "ABI {version}: {namespace}::{function:?}"
                    );
                } else {
                    assert!(matches!(
                        engine.validate_versioned(version, &wasm),
                        Err(ValidationRefusal::ForbiddenImport { .. })
                    ));
                    assert!(violations.contains(&undeclared(namespace, function.name)));
                }
            }
        }
    }
}

#[test]
fn finding_015_v5_checks_signatures_kinds_duplicates_and_unknown_imports() {
    let engine = checked(WasmEngine::declared());
    let name = V5_HOST_FUNCTIONS[0].name;
    let wrong_signature = guest(
        &[(V5_ABI_MODULE, name, 0)],
        &[TYPE_I32; 5],
        TYPE_I32,
        &[],
        &[],
    );
    assert_eq!(
        checked(
            engine
                .validate_v5(&wrong_signature)
                .err()
                .ok_or("signature accepted")
        ),
        ValidationRefusal::WrongImportSignature {
            import_name: name.to_string()
        }
    );
    assert!(lint_artifact_for_abi(&wrong_signature, 5).contains(
        &DeterminismViolation::RejectedByEngine {
            reason: format!("invalid LayerX ABI import {name}"),
        }
    ));

    let duplicate = guest(
        &[(V5_ABI_MODULE, name, 0), (V5_ABI_MODULE, name, 0)],
        &[TYPE_I32; 6],
        TYPE_I32,
        &[],
        &[],
    );
    assert!(matches!(
        engine.validate_v5(&duplicate),
        Err(ValidationRefusal::DuplicateImport { .. })
    ));
    assert!(lint_artifact_for_abi(&duplicate, 5).contains(
        &DeterminismViolation::RejectedByEngine {
            reason: format!("duplicate LayerX ABI import {V5_ABI_MODULE}::{name}"),
        }
    ));

    let mut non_function = unsigned_leb(1);
    for field in [V5_ABI_MODULE, name] {
        non_function.extend(unsigned_leb(field.len() as u64));
        non_function.extend_from_slice(field.as_bytes());
    }
    non_function.extend_from_slice(&[3, TYPE_I32, 0]);
    let wrong_kind = module(&[raw_section(2, &non_function)]);
    assert!(matches!(
        engine.validate_v5(&wrong_kind),
        Err(ValidationRefusal::WrongImportKind { .. })
    ));
    assert!(lint_artifact_for_abi(&wrong_kind, 5).contains(
        &DeterminismViolation::RejectedByEngine {
            reason: format!("invalid LayerX ABI import {name}"),
        }
    ));

    for (namespace, import) in [
        (V5_ABI_MODULE, "web_read"),
        (ABI_V4_MODULE, name),
        ("layerx_v6", name),
        (V5_ABI_MODULE, "market_step_adjudicate_unchecked"),
    ] {
        let wasm = guest(
            &[(namespace, import, 0)],
            &[TYPE_I32; 6],
            TYPE_I32,
            &[],
            &[],
        );
        assert!(matches!(
            engine.validate_v5(&wasm),
            Err(ValidationRefusal::ForbiddenImport { .. })
        ));
        assert!(lint_artifact_for_abi(&wasm, 5).contains(&undeclared(namespace, import)));
    }
}

#[test]
fn finding_015_v5_preserves_named_determinism_and_export_refusals() {
    let engine = checked(WasmEngine::declared());
    for (name, expected) in [
        (
            "clock_time_get",
            DeterminismViolation::ClockImport {
                import_module: "wasi_snapshot_preview1".to_string(),
                import_name: "clock_time_get".to_string(),
            },
        ),
        (
            "random_get",
            DeterminismViolation::RandomnessImport {
                import_module: "wasi_snapshot_preview1".to_string(),
                import_name: "random_get".to_string(),
            },
        ),
    ] {
        let wasm = guest(
            &[("wasi_snapshot_preview1", name, 0)],
            &[],
            TYPE_I32,
            &[],
            &[],
        );
        assert!(matches!(
            engine.validate_v5(&wasm),
            Err(ValidationRefusal::ForbiddenImport { .. })
        ));
        assert!(lint_artifact_for_abi(&wasm, 5).contains(&expected));
    }
    for value_type in [TYPE_F32, TYPE_F64, TYPE_V128] {
        let wasm = guest(&[], &[], TYPE_I32, &[value_type], &[]);
        let (runtime, lint) = if value_type == TYPE_V128 {
            (
                ValidationRefusal::ForbiddenVectorType,
                DeterminismViolation::VectorType,
            )
        } else {
            (
                ValidationRefusal::ForbiddenFloatType,
                DeterminismViolation::FloatingPointType,
            )
        };
        assert_eq!(
            checked(
                engine
                    .validate_v5(&wasm)
                    .err()
                    .ok_or("forbidden type accepted")
            ),
            runtime
        );
        assert!(lint_artifact_for_abi(&wasm, 5).contains(&lint));
    }
    let float = guest(
        &[],
        &[],
        TYPE_I32,
        &[],
        &[OP_F32_CONST, 0, 0, 0, 0, OP_DROP],
    );
    assert!(matches!(
        engine.validate_v5(&float),
        Err(ValidationRefusal::ForbiddenFloatInstruction)
    ));
    assert!(
        lint_artifact_for_abi(&float, 5).contains(&DeterminismViolation::FloatingPointInstruction)
    );
    let empty = module(&[]);
    for export in [
        layerx_program_sdk::CALL_ENTRY_EXPORT,
        layerx_program_sdk::CALL_RESERVE_EXPORT,
        layerx_program_sdk::MEMORY_EXPORT,
    ] {
        assert!(
            lint_artifact_for_abi(&empty, 5).contains(&DeterminismViolation::MissingExport {
                export: export.to_string(),
            })
        );
    }
    assert!(lint_artifact_for_abi(b"invalid wasm", 5)
        .iter()
        .any(|violation| matches!(violation, DeterminismViolation::MalformedModule { .. })));
}

#[test]
fn finding_015_cli_metadata_and_project_artifact_use_shared_admission() {
    let project = Project::new();
    let artifact = project.0.join("guest.wasm");
    assert_eq!(
        project_violations(&project.0, Some(&artifact), None),
        Vec::new()
    );
    assert_eq!(
        project_violations(&project.0, Some(&artifact), Some("5")),
        Vec::new()
    );
    assert_eq!(
        lint_project_for_abi(&project.0, Some(&artifact), 5),
        Vec::new()
    );
    assert!(project_violations(&project.0, Some(&artifact), Some("4"))
        .iter()
        .any(
            |violation| matches!(violation, DeterminismViolation::UnreadablePath { reason, .. }
            if reason == "requested ABI version differs from layerx-program.json")
        ));
    for version in 1..=5 {
        assert!(admit_abi_version(version).is_ok());
        assert_eq!(parse_version(&version.to_string()), Some(version));
    }
    for version in [0, 6, u16::MAX] {
        assert!(admit_abi_version(version).is_err());
        assert_eq!(parse_version(&version.to_string()), None);
        assert_eq!(
            lint_artifact_for_abi(&v5_guest(), version),
            vec![DeterminismViolation::RejectedByEngine {
                reason: format!("unsupported LayerX ABI version {version}"),
            }]
        );
        checked(fs::write(
            project.0.join("layerx-program.json"),
            format!("{{\"abi_version\":{version}}}"),
        ));
        assert!(project_violations(&project.0, Some(&artifact), None)
            .iter()
            .any(
                |violation| matches!(violation, DeterminismViolation::UnreadablePath { reason, .. }
                if reason == &format!("unsupported LayerX ABI version {version}"))
            ));
    }
    for manifest in [
        "{\"abi_version\":5,\"abi_version\":5}",
        "{\"abi_version\":\"5\"}",
        "{\"abi_version\":5.0}",
        "{\"metadata\":{\"abi_version\":5}}",
        "{}",
    ] {
        checked(fs::write(project.0.join("layerx-program.json"), manifest));
        assert!(project_violations(&project.0, Some(&artifact), None)
            .iter()
            .any(|violation| matches!(violation, DeterminismViolation::UnreadablePath { .. })));
    }
}

fn padded_guest(bytes: usize) -> Vec<u8> {
    let mut wasm = v5_guest();
    let remaining = bytes
        .checked_sub(wasm.len())
        .unwrap_or_else(|| panic!("fixture size"));
    let padding = (remaining.saturating_sub(16)..=remaining)
        .find(|padding| padding_section(*padding).len() == remaining)
        .unwrap_or_else(|| panic!("unrepresentable fixture padding"));
    wasm.extend(padding_section(padding));
    assert_eq!(wasm.len(), bytes);
    wasm
}

#[test]
fn finding_015_deployment_limit_includes_the_104_byte_envelope() {
    let at_limit = padded_guest(524_288 - 104);
    let engine = checked(WasmEngine::declared());
    checked(engine.validate_v5(&at_limit));
    assert_eq!(lint_artifact_for_abi(&at_limit, 5), Vec::new());
    let over_limit = padded_guest(524_288 - 104 + 1);
    checked(engine.validate_v5(&over_limit));
    for version in 1..=5 {
        assert_eq!(
            lint_artifact_for_abi(&over_limit, version),
            vec![DeterminismViolation::ModuleTooLarge {
                byte_size: 524_185,
                limit: 524_184
            }]
        );
    }
    assert_eq!(at_limit.len() + 104, 524_288);
    assert_eq!(over_limit.len() + 104, 524_289);
}

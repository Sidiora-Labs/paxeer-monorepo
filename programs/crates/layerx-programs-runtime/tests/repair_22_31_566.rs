//! Exercise the production native result adapter with real executor captures.

use layerx_programs_runtime::native_result::from_captured;
use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, module, raw_section, type_section, unsigned_leb,
    TYPE_I32,
};
use layerx_programs_runtime::{
    ActivityBudgetBinding, AuthorizationContext, AuthorizedExecutionRequest,
    BudgetedAuthorizedExecutionRequest, CapabilitySet, CompositionContext, DeclaredBudget,
    ExecutionError, ExecutionFault, Executor, PrincipalId, ProgramId, ProgramReplayProfile,
    Storage, TracePolicy, UnavailableReceiptOracle, V2ActivityOutcome, V2AuthorizedExecutionRecord,
    WasmEngine,
};
use std::path::Path;
use std::process::Command;

fn guest(trapping_start: bool, trapping_call: bool) -> Vec<u8> {
    let mut exports = unsigned_leb(3);
    for (name, kind, index) in [
        ("layerx_reserve", 0, 0),
        ("layerx_call", 0, 1),
        ("memory", 2, 0),
    ] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.extend([kind, index]);
    }
    let mut sections = vec![
        type_section(&[
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
            (&[], &[]),
        ]),
        function_section(if trapping_start { &[0, 1, 2] } else { &[0, 1] }),
        raw_section(5, &[1, 1, 1, 1]),
        raw_section(7, &exports),
    ];
    if trapping_start {
        sections.push(raw_section(8, &[2]));
    }
    let mut bodies = vec![
        func_body(&[], &[0x41, 0, 0x0b]),
        func_body(
            &[],
            if trapping_call {
                &[0x00, 0x0b]
            } else {
                &[0x41, 0, 0x0b]
            },
        ),
    ];
    if trapping_start {
        bodies.push(func_body(&[], &[0x00, 0x0b]));
    }
    sections.push(code_section(&bodies));
    module(&sections)
}

fn run(
    wasm: &[u8],
    profile: Option<ProgramReplayProfile>,
    trace_policy: Option<TracePolicy>,
    entrypoint: &str,
) -> Result<V2AuthorizedExecutionRecord, ExecutionError> {
    let engine = WasmEngine::declared().unwrap();
    let validated = engine.validate_v2(wasm).unwrap();
    let mut executor = Executor::declared();
    if let Some(profile) = profile {
        executor = executor.with_program_replay_profile(profile);
    }
    if let Some(trace_policy) = trace_policy {
        executor = executor.with_trace_policy(trace_policy);
    }
    let payer = PrincipalId::new([2; 32]).unwrap();
    let binding = ActivityBudgetBinding::new([3; 32]).unwrap();
    let declared = DeclaredBudget::new(1_000_000, 65_536, 0, 0, 1, 0, 0).unwrap();
    let admitted = executor
        .admit_activity_budget_for_qualification(declared, payer, binding, u128::MAX)
        .unwrap();
    let mut storage = Storage::new();
    let result = executor.execute_authorized_v2_budgeted_for_qualification(
        &mut storage,
        BudgetedAuthorizedExecutionRequest::new(
            AuthorizedExecutionRequest {
                module: &validated,
                program: ProgramId::new([1; 32]).unwrap(),
                authorization: AuthorizationContext::new(payer, CapabilitySet::empty()),
                receipts: &UnavailableReceiptOracle,
                entrypoint,
                calldata: &[],
                composition: CompositionContext::isolated(),
                response_capacity: 0,
            },
            admitted,
            payer,
            binding,
        ),
    );
    assert_eq!(
        storage,
        Storage::new(),
        "refused CALL must not commit storage"
    );
    result
}

fn profile() -> ProgramReplayProfile {
    ProgramReplayProfile::new(128, 1_048_576).unwrap()
}

#[test]
fn profiled_capture_one_byte_above_its_bound_answers_replay_bound() {
    let wasm = guest(false, false);
    let full = run(&wasm, Some(profile()), None, "layerx_call").unwrap();
    let replay = full.replay_record().unwrap();
    let maximum = u32::try_from(replay.canonical_bytes().len() - 1).unwrap();
    let bounded = ProgramReplayProfile::new(128, maximum).unwrap();
    let result = run(&wasm, Some(bounded), None, "layerx_call");
    assert_eq!(
        result.as_ref().unwrap_err(),
        &ExecutionError::Fault(ExecutionFault::EngineFault {
            reason: "program replay record refused: Bounds".to_string(),
        }),
        "the real record producer must refuse serialization at the exact bound"
    );
    assert_eq!(from_captured(result), Err(-740));
}

#[test]
fn trapping_guest_initializer_retains_failure_and_replay_record() {
    let result = run(&guest(true, false), Some(profile()), None, "layerx_call");
    let original = result.as_ref().unwrap().clone();
    assert!(matches!(original.outcome(), V2ActivityOutcome::Failure(_)));
    assert!(original.replay_record().is_some());
    let record = from_captured(result).unwrap();
    assert_eq!(record, original);
}

#[test]
fn with_trace_policy() {
    let result = run(
        &guest(false, false),
        Some(profile()),
        Some(TracePolicy::new(2, 128).unwrap()),
        "layerx_call",
    );
    assert_eq!(
        result.as_ref().unwrap_err(),
        &ExecutionError::Fault(ExecutionFault::EngineFault {
            reason: "program replay profile and trace policy diverged".to_string(),
        })
    );
    assert_eq!(from_captured(result), Err(-741));
}

#[test]
fn unprofiled_call_and_retained_guest_failure_are_unchanged() {
    let result = run(&guest(false, false), None, None, "layerx_call");
    let original = result.as_ref().unwrap().clone();
    let record = from_captured(result).unwrap();
    assert_eq!(record, original);
    assert!(matches!(
        record.outcome(),
        V2ActivityOutcome::Success { .. }
    ));
    assert!(record.replay_record().is_none());
    assert_eq!(record.execution().usage().cpu_fuel, 39);

    let result = run(&guest(false, true), None, None, "layerx_call");
    let original = result.as_ref().unwrap().clone();
    let record = from_captured(result).unwrap();
    assert_eq!(record, original);
    assert!(matches!(record.outcome(), V2ActivityOutcome::Failure(_)));
    assert!(record.replay_record().is_none());
}

#[test]
fn other_execution_variants_keep_non_canonical() {
    let result = run(&guest(false, false), Some(profile()), None, "missing_call");
    assert!(matches!(result, Err(ExecutionError::Entrypoint(_))));
    assert_eq!(from_captured(result), Err(-3));
}

#[test]
fn native_kernel_resolves_both_status_names() {
    let executable = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../build/tests/kernel/codec/lxp_test_result_codes");
    // hf-done builds this producer before running the Rust target. Its name
    // mode checks both codes and transports the actual C registry names.
    let output = Command::new(executable)
        .arg("--replay-status-names")
        .output()
        .unwrap();
    assert!(
        output.status.success(),
        "native C registry: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(
        String::from_utf8(output.stdout).unwrap(),
        "LXP_ERR_REPLAY_BOUND\nLXP_ERR_ENGINE_FAULT\n"
    );
}

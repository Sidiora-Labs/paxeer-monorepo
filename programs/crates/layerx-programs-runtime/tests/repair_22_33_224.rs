// Task 22.33.224: finding 22.30.125/trace_refusal_atomicity is proved by
// trace_refusal_atomicity. No out-of-path wiring is required. Baseline verification
// was not run: the task's ordered gates require implementation before verification.

use layerx_programs_runtime::test_support::{
    code_section, export_section, func_body, function_section, module, type_section, OP_END,
    OP_I32_CONST, TYPE_I32,
};
use layerx_programs_runtime::{
    CommitmentError, ExecutionState, ExecutionTrace, Executor, StepCommitment, TracePolicy,
    WasmEngine, WasmValue, MAX_TRACE_STATE_BYTES,
};

fn execution_state() -> ExecutionState {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let wasm = module(&[
        type_section(&[(&[], &[TYPE_I32])]),
        function_section(&[0]),
        export_section(&[("run", 0)]),
        code_section(&[func_body(&[], &[OP_I32_CONST, 7, OP_END])]),
    ]);
    let validated = WasmEngine::declared()
        .assert_success("test value")
        .validate(&wasm)
        .assert_success("test value");
    let record = Executor::declared()
        .with_trace_policy(TracePolicy::new(1, 16).assert_success("test value"))
        .execute_traced(&validated, "run", &[])
        .assert_success("test value");
    assert_eq!(record.execution.outputs, vec![WasmValue::I32(7)]);
    record.trace.steps()[0].pre_state.as_ref().clone()
}

#[test]
fn trace_refusal_atomicity() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut state = execution_state();
    state.linear_memory.clear();
    let overhead = state.canonical_bytes().assert_success("test value").len();
    let chunk_bytes = 1_024 * 1_024;
    state.linear_memory.resize(chunk_bytes - overhead, 0x5a);
    assert_eq!(
        state.canonical_bytes().assert_success("test value").len(),
        chunk_bytes
    );

    let mut trace = ExecutionTrace::new(TracePolicy::new(1, 128).assert_success("test value"));
    let mut expected_fuel = 0;
    let mut expected_commitments = Vec::new();
    let chunk_count =
        MAX_TRACE_STATE_BYTES / u64::try_from(chunk_bytes).assert_success("test value");
    for index in 0..chunk_count - 1 {
        state.step_index = index;
        let expected = StepCommitment::from_state(&state).assert_success("test value");
        assert_eq!(
            trace.record(&state).assert_success("test value"),
            Some(expected)
        );
        expected_fuel += expected.commitment_fuel;
        expected_commitments.push(expected);
    }
    assert_eq!(
        trace.total_state_bytes(),
        MAX_TRACE_STATE_BYTES - u64::try_from(chunk_bytes).assert_success("test value")
    );
    assert_eq!(trace.total_commitment_fuel(), expected_fuel);
    assert_eq!(trace.commitments(), expected_commitments);

    let before = trace.clone();
    let canonical_before = trace.canonical_bytes().assert_success("test value");
    state.step_index = chunk_count - 1;
    state.linear_memory.push(0x5a);
    let refused = StepCommitment::from_state(&state).assert_success("test value");
    assert_eq!(
        u64::from(refused.encoded_state_bytes) + trace.total_state_bytes(),
        MAX_TRACE_STATE_BYTES + 1
    );
    for _ in 0..2 {
        assert_eq!(
            trace.record(&state),
            Err(CommitmentError::TraceByteLimit {
                attempted: MAX_TRACE_STATE_BYTES + 1,
                limit: MAX_TRACE_STATE_BYTES,
            })
        );
        assert_eq!(
            trace.canonical_bytes().assert_success("test value"),
            canonical_before
        );
        assert_eq!(trace.total_state_bytes(), before.total_state_bytes());
        assert_eq!(
            trace.total_commitment_fuel(),
            before.total_commitment_fuel()
        );
        assert_eq!(trace.commitments(), before.commitments());
        assert_eq!(trace, before);
    }

    state.linear_memory.pop();
    let admitted = StepCommitment::from_state(&state).assert_success("test value");
    assert_ne!(admitted.digest, refused.digest);
    assert_eq!(
        trace.record(&state).assert_success("test value"),
        Some(admitted)
    );
    expected_commitments.push(admitted);
    assert_eq!(trace.commitments(), expected_commitments);
    assert_eq!(trace.total_state_bytes(), MAX_TRACE_STATE_BYTES);
    assert_eq!(
        trace.total_commitment_fuel(),
        expected_fuel + admitted.commitment_fuel
    );
    assert_ne!(
        trace.canonical_bytes().assert_success("test value"),
        canonical_before
    );
    let mut expected = before;
    assert_eq!(
        expected.record(&state).assert_success("test value"),
        Some(admitted)
    );
    assert_eq!(
        trace.canonical_bytes().assert_success("test value"),
        expected.canonical_bytes().assert_success("test value")
    );
}

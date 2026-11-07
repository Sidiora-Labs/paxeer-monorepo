use layerx_programs_arbiter::bisect::CommittedTrace;
use layerx_programs_runtime::execute::{
    instantiate_market_sandbox_untrusted, market_sandbox_input_digest, observe_market_sandbox_step,
    MarketSandboxRequest,
};
use layerx_programs_runtime::portable_replay::PortableBoundary;
use layerx_programs_runtime::replay::{
    market_sandbox_baseline_root, market_sandbox_namespace, MarketSandboxReplayAuthority,
};
use layerx_programs_runtime::test_support::{
    code_section, export_section, func_body, function_section, module, type_section, TYPE_I32,
};
use layerx_programs_runtime::{
    ExecutionFault, FeeSchedule, PrincipalId, ProgramId, ProgramReplayProfile, ResourceBudget,
    Storage, WasmEngine, RUNTIME_VERSION,
};
use sha2::{Digest, Sha256};

#[test]
fn terminal_trap_replay_recomputes_the_claimed_trap_before_committing_the_trace() {
    const MAXIMUM: usize = 1_048_576;
    let wasm = module(&[
        type_section(&[(&[], &[TYPE_I32])]),
        function_section(&[0]),
        export_section(&[("compute", 0)]),
        code_section(&[func_body(&[], &[0, 0x0b])]),
    ]);
    let module = WasmEngine::declared().unwrap().validate_v2(&wasm).unwrap();
    let program = ProgramId::new(module.code_hash()).unwrap();
    let tenant = PrincipalId::new([2; 32]).unwrap();
    let lease = [3; 32];
    let baseline = Storage::new();
    let fees = FeeSchedule::declared();
    let authority = MarketSandboxReplayAuthority {
        profile_binding: Sha256::digest(b"terminal-trap-profile").into(),
        namespace: market_sandbox_namespace(program, lease).unwrap(),
        lease_id: lease,
        namespace_limit: 1024,
        program,
        tenant,
        payment_account: tenant.bytes(),
        code_hash: module.code_hash(),
        input_digest: market_sandbox_input_digest("compute", &[]).unwrap(),
        runtime_version: RUNTIME_VERSION,
        abi_version: 2,
        fee_schedule_version: fees.version(),
        metering_schedule_version: module.metering_schedule_version(),
        budget: ResourceBudget::new_complete(10_000_000, 65_536, 1024, 1024, 1, 64, 0),
        fees,
        fee_budget: 1_000_000_000,
        baseline_state_root: market_sandbox_baseline_root(&baseline).unwrap(),
        baseline_storage: baseline,
    };
    let execution = instantiate_market_sandbox_untrusted(&module, &authority)
        .unwrap()
        .call_market_sandbox_untrusted(MarketSandboxRequest {
            module: &module,
            entrypoint: "compute",
            args: &[],
            authority: &authority,
            replay_profile: ProgramReplayProfile::new(128, MAXIMUM as u32).unwrap(),
        })
        .unwrap();
    assert_eq!(
        execution.terminal_fault,
        Some(ExecutionFault::UnreachableExecuted)
    );
    assert_eq!(execution.record.terminal_status(), 1);
    let terminal =
        PortableBoundary::decode_untrusted(execution.boundary_leaves.last().unwrap(), MAXIMUM)
            .unwrap();
    let mut forged = terminal.clone();
    assert!(!forged.trap.as_ref().unwrap().host_trap);
    forged.trap.as_mut().unwrap().host_trap = true;
    let observed = observe_market_sandbox_step(&module, &forged, &authority, MAXIMUM).unwrap();
    assert_eq!(
        observed.reencode_untrusted(MAXIMUM).unwrap(),
        terminal.reencode_untrusted(MAXIMUM).unwrap()
    );
    assert_ne!(
        observed.reencode_untrusted(MAXIMUM).unwrap(),
        forged.reencode_untrusted(MAXIMUM).unwrap()
    );
    let trace = CommittedTrace::new(execution.boundary_leaves).unwrap();
    assert_eq!(trace.boundary_count(), execution.record.boundary_count());
}

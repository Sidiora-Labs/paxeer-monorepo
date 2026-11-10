mod market_tests {
    use layerx_programs_runtime::portable_replay::{decode_record, PortableBoundary};
    use layerx_programs_runtime::test_support::{
        code_section, func_body, function_section, module, raw_section, type_section, unsigned_leb,
        TYPE_I32,
    };
    use layerx_programs_runtime::{
        arbitration_step_commitment_fuel_with_host, step_commitment_fuel, ActivityBudgetBinding,
        AuthorizationContext, AuthorizedExecutionRequest, BudgetedAuthorizedExecutionRequest,
        CapabilitySet, CompositionContext, DeclaredBudget, ExecutionError, ExecutionFault,
        Executor, PrincipalId, ProgramId, ProgramReplayProfile, Storage, UnavailableReceiptOracle,
        V2ActivityOutcome, V2AuthorizedExecutionRecord, WasmEngine, MIN_ACTIVITY_CPU_FUEL,
    };

    const CPU_CAP: u64 = 1_000_000;
    const MAXIMUM_BYTES: u32 = 1_048_576;

    fn guest(instructions: &[u8]) -> Vec<u8> {
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
        module(&[
            type_section(&[(&[TYPE_I32], &[TYPE_I32]), (&[TYPE_I32; 2], &[TYPE_I32])]),
            function_section(&[0, 1]),
            raw_section(5, &[1, 1, 1, 1]),
            raw_section(7, &exports),
            code_section(&[
                func_body(&[], &[0x41, 0, 0x0b]),
                func_body(&[], instructions),
            ]),
        ])
    }

    fn run(
        instructions: &[u8],
        profile: Option<ProgramReplayProfile>,
    ) -> Result<V2AuthorizedExecutionRecord, ExecutionError> {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let engine = WasmEngine::declared().assert_success("test value");
        let validated = engine
            .validate_v2(&guest(instructions))
            .assert_success("test value");
        let mut executor = Executor::declared();
        if let Some(profile) = profile {
            executor = executor.with_program_replay_profile(profile);
        }
        let payer = PrincipalId::new([2; 32]).assert_success("test value");
        let binding = ActivityBudgetBinding::new([3; 32]).assert_success("test value");
        let declared =
            DeclaredBudget::new(CPU_CAP, 65_536, 0, 0, 1, 0, 0).assert_success("test value");
        let admitted = executor
            .admit_activity_budget_for_qualification(declared, payer, binding, u128::MAX)
            .assert_success("test value");
        let mut storage = Storage::new();
        let result = executor.execute_authorized_v2_budgeted_for_qualification(
            &mut storage,
            BudgetedAuthorizedExecutionRequest::new(
                AuthorizedExecutionRequest {
                    module: &validated,
                    program: ProgramId::new([1; 32]).assert_success("test value"),
                    authorization: AuthorizationContext::new(payer, CapabilitySet::empty()),
                    receipts: &UnavailableReceiptOracle,
                    entrypoint: "layerx_call",
                    calldata: &[],
                    composition: CompositionContext::isolated(),
                    response_capacity: 0,
                },
                admitted,
                payer,
                binding,
            ),
        );
        assert_eq!(storage, Storage::new());
        result
    }

    fn profile() -> ProgramReplayProfile {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        ProgramReplayProfile::new(128, MAXIMUM_BYTES).assert_success("test value")
    }

    fn assert_replay_fuel(record: &layerx_programs_runtime::ProgramReplayRecord) -> u64 {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let (decoded, leaves) = decode_record(record.canonical_bytes(), MAXIMUM_BYTES as usize)
            .assert_success("test value");
        assert_eq!(&decoded, record);
        assert_eq!(leaves.len(), record.boundary_count() as usize);
        let mut total = 0u64;
        for leaf in leaves {
            let boundary = PortableBoundary::decode_with_header(
                &leaf,
                MAXIMUM_BYTES as usize,
                decoded.header(),
            )
            .assert_success("test value");
            assert_eq!(&boundary.header, decoded.header());
            let supplement = &boundary.replay.snapshot.supplement;
            let legacy =
                step_commitment_fuel(supplement.canonical_state_bytes).assert_success("test value");
            let arbitration = arbitration_step_commitment_fuel_with_host(
                supplement.canonical_state_bytes,
                supplement.arbitration_engine_canonical_bytes,
                supplement.arbitration_host_state_bytes,
            )
            .assert_success("test value");
            assert_eq!(supplement.commitment_fuel, legacy);
            assert_eq!(supplement.arbitration_commitment_fuel, arbitration);
            total = total
                .checked_add(legacy)
                .assert_success("test value")
                .checked_add(arbitration)
                .assert_success("test value");
        }
        assert_eq!(record.replay_fuel(), total);
        assert_eq!(decoded.replay_fuel(), total);
        total
    }

    #[test]
    fn profiled_call_preserves_guest_cpu_and_meters_replay_separately() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let mut instructions = vec![0x01; 24];
        instructions.extend([0x41, 0, 0x0b]);
        let ordinary = run(&instructions, None).assert_success("test value");
        let profiled = run(&instructions, Some(profile())).assert_success("test value");
        assert!(matches!(
            ordinary.outcome(),
            V2ActivityOutcome::Success { .. }
        ));
        assert_eq!(profiled.outcome(), ordinary.outcome());
        assert!(ordinary.replay_record().is_none());
        assert_eq!(profiled.execution().usage(), ordinary.execution().usage());
        assert!(profiled.execution().usage().cpu_fuel < CPU_CAP);
        let replay = profiled.replay_record().assert_success("test value");
        assert_eq!(replay.terminal_status(), 0);
        assert!(assert_replay_fuel(replay) > CPU_CAP);
    }

    #[test]
    fn replay_fuel_is_recomputed_for_a_retained_guest_trap() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let ordinary = run(&[0x00, 0x0b], None).assert_success("test value");
        let profiled = run(&[0x00, 0x0b], Some(profile())).assert_success("test value");
        assert!(matches!(profiled.outcome(), V2ActivityOutcome::Failure(_)));
        assert_eq!(profiled.execution().usage(), ordinary.execution().usage());
        let replay = profiled.replay_record().assert_success("test value");
        assert_eq!(replay.terminal_status(), 1);
        assert!(assert_replay_fuel(replay) > 0);
    }

    #[test]
    fn replay_capture_still_refuses_byte_and_boundary_profile_limits() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let instructions = [0x41, 0, 0x0b];
        let bytes = ProgramReplayProfile::new(128, 512).assert_success("test value");
        assert_eq!(
        run(&instructions, Some(bytes)),
        Err(ExecutionError::Fault(ExecutionFault::EngineFault {
            reason: "deterministic execution observer refused: SupplementRejected at retain_boundary_capture.source".to_string(),
        })),
    );
        let boundaries = ProgramReplayProfile::new(2, MAXIMUM_BYTES).assert_success("test value");
        assert_eq!(
        run(&instructions, Some(boundaries)),
        Err(ExecutionError::Fault(ExecutionFault::EngineFault {
            reason: "deterministic execution commitment refused: execution trace exceeds commitment limit 2".to_string(),
        })),
    );
    }

    #[test]
    fn unprofiled_empty_call_keeps_the_frozen_39_cpu_fuel() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let record = run(&[0x41, 0, 0x0b], None).assert_success("test value");
        assert!(matches!(
            record.outcome(),
            V2ActivityOutcome::Success { .. }
        ));
        assert!(record.replay_record().is_none());
        assert_eq!(MIN_ACTIVITY_CPU_FUEL, 39);
        assert_eq!(record.execution().usage().cpu_fuel, 39);
    }
}

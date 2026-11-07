mod market_tests {
    use layerx_programs_runtime::abi::response::CANDIDATE_ABI_MODULE;
    use layerx_programs_runtime::abi::ABI_MODULE;
    use layerx_programs_runtime::portable_replay::{decode_record, PortableBoundary};
    use layerx_programs_runtime::test_support::{
        code_section, func_body, function_section, import_section, module, raw_section,
        type_section, unsigned_leb, TYPE_I32,
    };
    use layerx_programs_runtime::{
        ActivityBudgetBinding, AuthorizationContext, AuthorizedExecutionRequest,
        BudgetedAuthorizedExecutionRequest, Capability, CapabilitySet, CompositionContext,
        DeclaredBudget, ExecutionError, ExecutionFault, Executor, PrincipalId, ProgramId,
        ProgramReplayProfile, Storage, StorageNamespace, UnavailableReceiptOracle,
        V2ActivityOutcome, V2AuthorizedExecutionRecord, WasmEngine,
    };

    const MAXIMUM_BYTES: u32 = 1_048_576;
    const KEY: &[u8] = b"state";
    const VALUE: [u8; 8192] = {
        let mut value = [0; 8192];
        value[0] = 0x5a;
        value[8191] = 0x5a;
        value
    };

    fn i32_const(code: &mut Vec<u8>, mut value: i32) {
        code.push(0x41);
        loop {
            let byte = (value & 0x7f) as u8;
            value >>= 7;
            if (value == 0 && byte & 0x40 == 0) || (value == -1 && byte & 0x40 != 0) {
                code.push(byte);
                return;
            }
            code.push(byte | 0x80);
        }
    }

    fn host_call(code: &mut Vec<u8>, index: u8, arguments: &[i32]) {
        for argument in arguments {
            i32_const(code, *argument);
        }
        code.extend([0x10, index, 0x1a]);
    }

    fn assert_host_boundaries(
        leaves: &[Vec<u8>],
        header: &layerx_programs_runtime::replay_record::ReplayRecordHeader,
    ) {
        let expected = [0, 0, 8193, 8193, 8193];
        let mut calls = 0;
        let module = WasmEngine::declared()
            .unwrap()
            .validate_v2(&large_state_guest())
            .unwrap();
        let mut memory = module.initial_replay_memory().unwrap();
        for leaf in leaves {
            let mut boundary =
                PortableBoundary::decode_with_header(leaf, MAXIMUM_BYTES as usize, header).unwrap();
            memory = boundary.rebuild_memory(&memory).unwrap();
            let snapshot = &boundary.replay.snapshot;
            if snapshot.canonical_instruction != [0x1a] {
                continue;
            }
            assert!(calls < expected.len());
            assert_eq!(snapshot.value_stack.last().unwrap().bits, expected[calls]);
            if calls >= 2 {
                assert_eq!(&snapshot.linear_memory[12288..20480], VALUE.as_slice());
                assert_eq!(snapshot.linear_memory[12288], 0x5a);
                assert_eq!(snapshot.linear_memory[20479], 0x5a);
            }
            calls += 1;
        }
        assert_eq!(calls, expected.len());
    }

    fn large_state_guest() -> Vec<u8> {
        let mut exports = unsigned_leb(3);
        for (name, kind, index) in [
            ("layerx_reserve", 0, 3),
            ("layerx_call", 0, 4),
            ("memory", 2, 0),
        ] {
            exports.extend(unsigned_leb(name.len() as u64));
            exports.extend_from_slice(name.as_bytes());
            exports.extend([kind, index]);
        }
        let mut entry = Vec::new();
        host_call(&mut entry, 1, &[2, 8, 5, 1024, 8192]);
        host_call(&mut entry, 2, &[16, 5, 1024, 4]);
        for _ in 0..3 {
            host_call(&mut entry, 0, &[2, 8, 5, 12288, 8192]);
        }
        entry.extend([0x41, 0, 0x0b]);
        let mut reserve = Vec::new();
        i32_const(&mut reserve, 32768);
        reserve.push(0x0b);
        let mut data = unsigned_leb(3);
        for (offset, bytes) in [
            (8, KEY),
            (16, b"event".as_slice()),
            (1024, VALUE.as_slice()),
        ] {
            data.push(0);
            i32_const(&mut data, offset);
            data.push(0x0b);
            data.extend(unsigned_leb(bytes.len() as u64));
            data.extend_from_slice(bytes);
        }
        module(&[
            type_section(&[
                (&[TYPE_I32; 5], &[TYPE_I32]),
                (&[TYPE_I32; 4], &[TYPE_I32]),
                (&[TYPE_I32], &[TYPE_I32]),
                (&[TYPE_I32; 2], &[TYPE_I32]),
            ]),
            import_section(&[
                (CANDIDATE_ABI_MODULE, "storage_read_scoped", 0),
                (CANDIDATE_ABI_MODULE, "storage_write_scoped", 0),
                (ABI_MODULE, "event_emit", 1),
            ]),
            function_section(&[2, 3]),
            raw_section(5, &[1, 1, 1, 1]),
            raw_section(7, &exports),
            code_section(&[func_body(&[], &reserve), func_body(&[], &entry)]),
            raw_section(11, &data),
        ])
    }

    fn run(
        profile: ProgramReplayProfile,
    ) -> (Storage, Result<V2AuthorizedExecutionRecord, ExecutionError>) {
        let program = ProgramId::new([1; 32]).unwrap();
        let payer = PrincipalId::new([2; 32]).unwrap();
        let binding = ActivityBudgetBinding::new([3; 32]).unwrap();
        let engine = WasmEngine::declared().unwrap();
        let validated = engine.validate_v2(&large_state_guest()).unwrap();
        let executor = Executor::declared().with_program_replay_profile(profile);
        let declared = DeclaredBudget::new(
            1_000_000, 16_777_216, 1_048_576, 1_048_576, 64, 1_048_576, 4096,
        )
        .unwrap();
        let admitted = executor
            .admit_activity_budget_for_qualification(declared, payer, binding, u128::MAX)
            .unwrap();
        let capabilities = CapabilitySet::new([
            Capability::SharedStorageRead,
            Capability::SharedStorageWrite,
            Capability::EmitEvent,
        ])
        .unwrap();
        let mut storage = Storage::new();
        let mut seed = storage.transaction(StorageNamespace::shared(program));
        seed.write(KEY, &[0x33; 4096]).unwrap();
        assert_eq!(seed.commit(), 1);
        let result = executor.execute_authorized_authenticated(
            &mut storage,
            BudgetedAuthorizedExecutionRequest::new(
                AuthorizedExecutionRequest {
                    module: &validated,
                    program,
                    authorization: AuthorizationContext::new(payer, capabilities),
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
            1,
            1000,
        );
        (storage, result)
    }

    #[test]
    fn authenticated_profiled_call_admits_large_shared_state_and_retains_replay() {
        let (mut storage, result) = run(ProgramReplayProfile::new(128, MAXIMUM_BYTES).unwrap());
        let record = result.unwrap();
        assert!(matches!(
            record.outcome(),
            V2ActivityOutcome::Success { .. }
        ));
        let replay = record.replay_record().unwrap();
        assert_eq!(replay.terminal_status(), 0);
        let (decoded, leaves) =
            decode_record(replay.canonical_bytes(), MAXIMUM_BYTES as usize).unwrap();
        assert_eq!(&decoded, replay);
        assert_eq!(leaves.len(), replay.boundary_count() as usize);
        assert_host_boundaries(&leaves, decoded.header());
        assert!(replay.boundary_count() > 5);
        assert!(replay.replay_fuel() > 0);
        assert_eq!(
            record.execution().usage().storage_read_bytes,
            3 * (8192 + KEY.len() as u64)
        );
        assert!(record.execution().usage().cpu_fuel < 1_000_000);
        let effects = record.effects().unwrap();
        assert_eq!(effects.events.len(), 1);
        assert_eq!(effects.events[0].topic, b"event");
        assert_eq!(effects.events[0].data, VALUE[..4]);
        let transaction =
            storage.transaction(StorageNamespace::shared(ProgramId::new([1; 32]).unwrap()));
        assert_eq!(transaction.read(KEY).unwrap().unwrap(), VALUE);
    }

    #[test]
    #[allow(non_snake_case)]
    fn SupplementRejected() {
        let (mut storage, result) = run(ProgramReplayProfile::new(128, 512).unwrap());
        assert_eq!(
        result,
        Err(ExecutionError::Fault(ExecutionFault::EngineFault {
            reason: "deterministic execution observer refused: SupplementRejected at retain_boundary_capture.source".to_string(),
        })),
    );
        let transaction =
            storage.transaction(StorageNamespace::shared(ProgramId::new([1; 32]).unwrap()));
        assert_eq!(transaction.read(KEY).unwrap().unwrap(), [0x33; 4096]);
    }
}

mod market_tests {
    use layerx_programs_runtime as rt;
    use rt::terminal::{decode_terminal_payload, ExecutionTerminal, TerminalDetail};
    use rt::test_support::{
        code_section, func_body, function_section, import_section, module, raw_section,
        type_section, unsigned_leb, TYPE_I32,
    };
    use rt::{
        AbiEffects, AuthorizationContext, AuthorizedExecutionRequest, Capability, CapabilitySet,
        CompositionContext, Executor, FeeSchedule, PrincipalId, ProgramId, ResourceBudget, Storage,
        StorageNamespace, TracePolicy, UnavailableReceiptOracle, V2ActivityReceipt, WasmEngine,
        WasmValue,
    };
    use std::{
        fs,
        path::PathBuf,
        sync::atomic::{AtomicU64, Ordering},
    };

    fn program() -> ProgramId {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        ProgramId::new([0x41; 32]).assert_success("test value")
    }
    fn actor() -> PrincipalId {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        PrincipalId::new([0x42; 32]).assert_success("test value")
    }
    fn budget() -> ResourceBudget {
        ResourceBudget::new_complete(
            100_000_000,
            131_072,
            1_048_576,
            1_048_576,
            64,
            1_048_576,
            4096,
        )
    }
    fn executor() -> Executor {
        Executor::new(budget(), FeeSchedule::declared())
    }

    struct DurableFile(PathBuf);
    impl DurableFile {
        fn save(bytes: &[u8]) -> Self {
            use layerx_programs_runtime::test_support::AssertSuccess as _;
            use std::io::Write;

            static NEXT: AtomicU64 = AtomicU64::new(0);
            let path = std::env::temp_dir().join(format!(
                "repair-22-31-259-{}-{}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
            let file = fs::OpenOptions::new()
                .write(true)
                .create_new(true)
                .open(&path)
                .assert_success("test value");
            (&file).write_all(bytes).assert_success("test value");
            file.sync_all().assert_success("test value");
            Self(path)
        }
        fn reopen(&self) -> Vec<u8> {
            use layerx_programs_runtime::test_support::AssertSuccess as _;

            fs::read(&self.0).assert_success("test value")
        }
    }
    impl Drop for DurableFile {
        fn drop(&mut self) {
            use layerx_programs_runtime::test_support::AssertSuccess as _;

            fs::remove_file(&self.0).assert_success("test value");
        }
    }

    fn exports(entries: &[(&str, u8, u32)]) -> Vec<u8> {
        let mut bytes = unsigned_leb(entries.len() as u64);
        for (name, kind, index) in entries {
            bytes.extend(unsigned_leb(name.len() as u64));
            bytes.extend_from_slice(name.as_bytes());
            bytes.push(*kind);
            bytes.extend(unsigned_leb(u64::from(*index)));
        }
        raw_section(7, &bytes)
    }
    fn push_i32(bytes: &mut Vec<u8>, mut value: i32) {
        bytes.push(0x41);
        loop {
            let low = value.to_le_bytes()[0] & 0x7f;
            value >>= 7;
            let done = value == 0 && low & 0x40 == 0 || value == -1 && low & 0x40 != 0;
            bytes.push(if done { low } else { low | 0x80 });
            if done {
                break;
            }
        }
    }
    fn data(bytes: &[u8]) -> Vec<u8> {
        let mut payload = vec![1, 0, 0x41, 0, 0x0b];
        payload.extend(unsigned_leb(bytes.len() as u64));
        payload.extend_from_slice(bytes);
        raw_section(11, &payload)
    }

    // The guest writes a colliding key, emits the value, publishes a v2 response,
    // and has a separate entry that reads that exact key after storage recovery.
    fn guest() -> Vec<u8> {
        guest_with_initial_value(0)
    }

    fn guest_with_initial_value(value: u8) -> Vec<u8> {
        let mut call = Vec::new();
        for value in [0, 1, 1, 1] {
            push_i32(&mut call, value);
        }
        call.extend([0x10, 0, 0x1a]);
        for value in [2, 4, 1, 1] {
            push_i32(&mut call, value);
        }
        call.extend([0x10, 2, 0x1a]);
        for value in [7, 1, 1] {
            push_i32(&mut call, value);
        }
        call.extend([0x10, 3, 0x1a, 0x41, 7, 0x0b]);
        let mut read = Vec::new();
        for value in [0, 1, 16, 1] {
            push_i32(&mut read, value);
        }
        read.extend([0x10, 1, 0x1a]);
        for value in [7, 16, 1] {
            push_i32(&mut read, value);
        }
        read.extend([0x10, 3, 0x1a, 0x41, 7, 0x0b]);
        module(&[
            type_section(&[
                (&[TYPE_I32; 4], &[TYPE_I32]),
                (&[TYPE_I32; 3], &[TYPE_I32]),
                (&[TYPE_I32], &[TYPE_I32]),
                (&[TYPE_I32; 2], &[TYPE_I32]),
            ]),
            import_section(&[
                ("layerx_v1", "storage_write", 0),
                ("layerx_v1", "storage_read", 0),
                ("layerx_v1", "event_emit", 0),
                ("layerx_v2", "response_write", 1),
            ]),
            function_section(&[2, 3, 3]),
            raw_section(5, &[1, 1, 1, 1]),
            exports(&[
                ("layerx_reserve", 0, 4),
                ("layerx_call", 0, 5),
                ("read", 0, 6),
                ("memory", 2, 0),
            ]),
            code_section(&[
                func_body(&[], &[0x41, 1, 0x0b]),
                func_body(&[], &call),
                func_body(&[], &read),
            ]),
            data(&[b'k', value, b'k', b'i', b'n', b'd']),
        ])
    }
    fn simple_guest() -> Vec<u8> {
        module(&[
            type_section(&[(&[TYPE_I32], &[TYPE_I32]), (&[TYPE_I32; 2], &[TYPE_I32])]),
            function_section(&[0, 1]),
            raw_section(5, &[1, 1, 1, 1]),
            exports(&[
                ("layerx_reserve", 0, 0),
                ("layerx_call", 0, 1),
                ("memory", 2, 0),
            ]),
            code_section(&[
                func_body(&[], &[0x41, 0, 0x0b]),
                func_body(&[], &[0x41, 7, 0x0b]),
            ]),
        ])
    }
    fn request<'a>(
        module: &'a rt::ValidatedModule,
        principal: PrincipalId,
        entry: &'a str,
        input: &'a [u8],
    ) -> AuthorizedExecutionRequest<'a> {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        AuthorizedExecutionRequest {
            module,
            program: program(),
            authorization: AuthorizationContext::new(
                principal,
                CapabilitySet::new([
                    Capability::StorageRead,
                    Capability::StorageWrite,
                    Capability::EmitEvent,
                ])
                .assert_success("test value"),
            ),
            receipts: &UnavailableReceiptOracle,
            entrypoint: entry,
            calldata: input,
            composition: CompositionContext::isolated(),
            response_capacity: 32,
        }
    }
    fn run(
        storage: &mut Storage,
        principal: PrincipalId,
        entry: &str,
        input: &[u8],
    ) -> rt::V2AuthorizedExecutionRecord {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let module = WasmEngine::declared()
            .assert_success("test value")
            .validate_versioned(4, &guest())
            .assert_success("test value");
        let record = executor()
            .execute_authorized_versioned_for_qualification(
                storage,
                request(&module, principal, entry, input),
            )
            .assert_success("test value");
        assert!(record.response().is_some(), "{:?}", record.outcome());
        record
    }

    #[test]
    fn receipt_abi_real_guests_persist_and_decode() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        for abi in 2..=5 {
            let module = WasmEngine::declared()
                .assert_success("test value")
                .validate_versioned(abi, &simple_guest())
                .assert_success("test value");
            let record = executor()
                .execute_authorized_versioned_for_qualification(
                    &mut Storage::new(),
                    request(&module, actor(), "layerx_call", &[]),
                )
                .assert_success("test value");
            assert_eq!(record.response().assert_success("test value").code, 7);
            let receipt = record.receipt_projection();
            assert_eq!(receipt.abi_revision(), abi);
            let stored = DurableFile::save(&receipt.canonical_encode());
            drop(record);
            let reopened =
                V2ActivityReceipt::canonical_decode(&stored.reopen()).assert_success("test value");
            assert_eq!(reopened, receipt);
            let mut invalid = reopened.canonical_encode();
            let revision = invalid
                .windows(32)
                .position(|bytes| bytes == program().bytes())
                .assert_success("test value")
                + 32;
            for unsupported in [0u16, 1, 6, u16::MAX] {
                invalid[revision..revision + 2].copy_from_slice(&unsupported.to_be_bytes());
                assert!(V2ActivityReceipt::canonical_decode(&invalid).is_err());
            }
            let encoded = receipt.canonical_encode();
            for end in 0..encoded.len() {
                assert!(V2ActivityReceipt::canonical_decode(&encoded[..end]).is_err());
            }
            let mut trailing = encoded;
            trailing.push(0);
            assert!(V2ActivityReceipt::canonical_decode(&trailing).is_err());
        }
    }

    fn trace_bytes(decoded: &rt::terminal::DecodedTerminal) -> &[u8] {
        match &decoded.detail {
            TerminalDetail::Execution(
                ExecutionTerminal::Legacy {
                    trace: Some(trace), ..
                }
                | ExecutionTerminal::CandidateV4 {
                    trace: Some(trace), ..
                },
            ) => trace,
            other => panic!("missing versioned trace: {other:?}"),
        }
    }
    #[test]
    fn trace_encoding_public_and_native_writers_are_identical() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        for abi in 1..=5 {
            let module = WasmEngine::declared()
                .assert_success("test value")
                .validate_versioned(abi, &simple_guest())
                .assert_success("test value");
            let exec =
                executor().with_trace_policy(TracePolicy::new(1, 128).assert_success("test value"));
            let mut native = Vec::with_capacity(1_048_576);
            let public = if abi == 1 {
                let record = exec
                    .execute_authorized(
                        &mut Storage::new(),
                        request(&module, actor(), "layerx_call", &[]),
                    )
                    .assert_success("test value");
                record.execution.write_canonical_evidence(&mut native);
                record.execution.canonical_evidence()
            } else {
                let record = exec
                    .execute_authorized_versioned_for_qualification(
                        &mut Storage::new(),
                        request(&module, actor(), "layerx_call", &[]),
                    )
                    .assert_success("test value");
                record.write_canonical_evidence(&mut native, &mut Vec::new());
                let projected = record.receipt_projection();
                assert_eq!(
                    V2ActivityReceipt::canonical_decode(&projected.canonical_encode())
                        .assert_success("test value"),
                    projected
                );
                record.canonical_evidence()
            };
            assert_eq!(native, public, "ABI {abi}");
            let disk = DurableFile::save(&native);
            drop(native);
            let decoded =
                decode_terminal_payload(1, abi, &disk.reopen()).assert_success("test value");
            rt::ExecutionTrace::verify_canonical_arbitration_bytes(trace_bytes(&decoded))
                .assert_success("test value");
        }
    }

    fn continuation_guest(kind: u8) -> Vec<u8> {
        let mut sections = vec![
            type_section(&[(&[], &[TYPE_I32]), (&[], &[])]),
            function_section(&[0, 0, 1, 0]),
        ];
        if kind != 0 && kind != 2 {
            sections.push(raw_section(4, &[1, 0x70, 1, 2, 2]));
        }
        sections.push(raw_section(5, &[1, 1, 1, 1]));
        if kind == 0 {
            sections.push(raw_section(6, &[1, TYPE_I32, 1, 0x41, 0, 0x0b]));
        }
        let mut names = vec![("mutate", 0, 2), ("observe", 0, 3), ("memory", 2, 0)];
        if kind == 0 {
            names.push(("counter", 3, 0));
        }
        sections.push(exports(&names));
        if kind == 1 {
            sections.push(raw_section(9, &[1, 0, 0x41, 0, 0x0b, 2, 0, 1]));
        }
        if kind == 3 {
            sections.push(raw_section(9, &[1, 1, 0, 1, 0]));
        }
        if kind == 2 {
            sections.push(raw_section(12, &[1]));
        }
        let mutate: &[u8] = match kind {
            0 => &[0x41, 9, 0x24, 0, 0x41, 0, 0x41, 9, 0x36, 2, 0, 0x0b],
            1 => &[0x41, 0, 0x41, 1, 0x41, 1, 0xfc, 14, 0, 0, 0x0b],
            2 => &[0xfc, 9, 0, 0x0b],
            _ => &[0xfc, 13, 0, 0x0b],
        };
        let observe: &[u8] = match kind {
            0 => &[0x23, 0, 0x41, 0, 0x28, 2, 0, 0x6a, 0x0b],
            1 => &[0x41, 0, 0x11, 0, 0, 0x0b],
            2 => &[
                0x41, 0, 0x41, 0, 0x41, 1, 0xfc, 8, 0, 0, 0x41, 0, 0x2d, 0, 0, 0x0b,
            ],
            _ => &[
                0x41, 0, 0x41, 0, 0x41, 1, 0xfc, 12, 0, 0, 0x41, 0, 0x11, 0, 0, 0x0b,
            ],
        };
        sections.push(code_section(&[
            func_body(&[], &[0x41, 1, 0x0b]),
            func_body(&[], &[0x41, 2, 0x0b]),
            func_body(&[], mutate),
            func_body(&[], observe),
        ]));
        if kind == 2 {
            sections.push(raw_section(11, &[1, 1, 1, b'z']));
        }
        module(&sections)
    }
    #[test]
    fn continuation_state_refuses_tables_and_dropped_segments_before_capture() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        for kind in 1..=3 {
            let wasm = continuation_guest(kind);
            let module = WasmEngine::declared()
                .assert_success("test value")
                .validate(&wasm)
                .assert_success("test value");
            assert!(!module.supports_continuation());
            let mut instance = module.instantiate().assert_success("test value");
            assert!(instance.call("observe", &[]).is_ok());
            instance.call("mutate", &[]).assert_success("test value");
            if kind == 1 {
                assert_eq!(
                    instance.call("observe", &[]).assert_success("test value"),
                    [WasmValue::I32(2)]
                );
            } else {
                assert!(instance.call("observe", &[]).is_err());
            }
            assert!(instance.capture_continuation("observe", &[]).is_err());
            drop(instance);
            drop(module);
            let reopened = WasmEngine::declared()
                .assert_success("test value")
                .validate(&wasm)
                .assert_success("test value");
            let mut fresh = reopened.instantiate().assert_success("test value");
            assert!(fresh.call("observe", &[]).is_ok());
            assert!(fresh.capture_continuation("observe", &[]).is_err());
        }
    }
    #[test]
    fn continuation_supported_memory_and_globals_survive_fresh_instantiation() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let wasm = continuation_guest(0);
        let module = WasmEngine::declared()
            .assert_success("test value")
            .validate(&wasm)
            .assert_success("test value");
        assert!(module.supports_continuation());
        let mut instance = module.instantiate().assert_success("test value");
        instance.call("mutate", &[]).assert_success("test value");
        let captured = instance
            .capture_continuation("observe", &[])
            .assert_success("test value");
        let uninterrupted = instance.call("observe", &[]).assert_success("test value");
        assert_eq!(uninterrupted, [WasmValue::I32(18)]);
        drop(instance);
        drop(module);
        let reopened = WasmEngine::declared()
            .assert_success("test value")
            .validate(&wasm)
            .assert_success("test value");
        let mut fresh = reopened.instantiate().assert_success("test value");
        assert_eq!(
            fresh.call("observe", &[]).assert_success("test value"),
            [WasmValue::I32(0)]
        );
        assert_eq!(
            fresh
                .restore_continuation(&captured)
                .assert_success("test value"),
            uninterrupted
        );
    }

    #[test]
    fn sandbox_storage_alias_leases_and_ordinary_call_survive_store_restart() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let first = rt::native_contract::sandbox_execution_principal(program(), [1; 32])
            .assert_success("test value");
        let second = rt::native_contract::sandbox_execution_principal(program(), [2; 32])
            .assert_success("test value");
        assert_ne!(first, second);
        assert_ne!(first, actor());
        assert_ne!(second, actor());
        let mut native = [0; 32];
        assert_eq!(
            unsafe {
                rt::native_contract::layerx_programs_sandbox_namespace(
                    program().bytes().as_ptr(),
                    [1; 32].as_ptr(),
                    native.as_mut_ptr(),
                )
            },
            0
        );
        assert_eq!(native, first.bytes());
        let mut storage = Storage::new();
        for (principal, value) in [(first, b'a'), (second, b'b'), (actor(), b'c')] {
            let record = run(&mut storage, principal, "layerx_call", &[value]);
            assert_eq!(
                record.response().assert_success("test value").bytes,
                [value]
            );
        }
        let file = DurableFile::save(
            &storage
                .replay_state_bytes(1_048_576)
                .assert_success("test value"),
        );
        drop(storage);
        let mut recovered = Storage::decode_untrusted_replay_state(&file.reopen(), 1_048_576)
            .assert_success("test value");
        for (principal, expected) in [(first, b'a'), (second, b'b'), (actor(), b'c')] {
            let record = run(&mut recovered, principal, "read", &[]);
            assert_eq!(
                record.response().assert_success("test value").bytes,
                [expected]
            );
            assert_eq!(
                recovered
                    .protocol_namespace_entries(StorageNamespace::principal(program(), principal))
                    .assert_success("test value"),
                [(b"k".to_vec(), vec![expected])]
            );
        }
    }

    #[test]
    fn sandbox_event_principal_and_guest_log_bytes_survive_receipt_replay() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let principal = rt::native_contract::sandbox_execution_principal(program(), [1; 32])
            .assert_success("test value");
        let record = run(&mut Storage::new(), principal, "layerx_call", &[0xff]);
        let effects = record.effects().assert_success("test value");
        assert_eq!(effects.events.len(), 1);
        assert_eq!(effects.events[0].principal, principal);
        let envelope = effects
            .canonical_program_event_envelope()
            .assert_success("test value");
        assert_eq!(
            unsafe {
                rt::native_contract::layerx_programs_event_envelope_validate(
                    envelope.as_ptr(),
                    envelope.len(),
                    principal.bytes().as_ptr(),
                    1,
                )
            },
            0
        );
        assert_ne!(
            unsafe {
                rt::native_contract::layerx_programs_event_envelope_validate(
                    envelope.as_ptr(),
                    envelope.len(),
                    actor().bytes().as_ptr(),
                    1,
                )
            },
            0
        );
        let logs = effects
            .program_event_logs(program())
            .assert_success("test value");
        let file = DurableFile::save(&envelope);
        drop(record);
        let replayed =
            AbiEffects::decode_program_event_envelope(&file.reopen()).assert_success("test value");
        assert_eq!(replayed.events[0].principal, principal);
        let replayed_logs = replayed
            .program_event_logs(program())
            .assert_success("test value");
        assert_eq!(logs, replayed_logs);
        assert_eq!(
            logs[0].topics[0],
            rt::hash_bytes(
                rt::HashAlgorithm::Keccak256,
                b"ProgramGuestEvent(bytes32,bytes32,bytes)"
            )
            .assert_success("test value")
        );
        assert_eq!(logs[0].topics[1], program().bytes());
        assert_eq!(
            &logs[0].data[..32],
            rt::hash_bytes(rt::HashAlgorithm::Keccak256, b"kind").assert_success("test value")
        );
        assert_eq!(logs[0].data[63], 64);
        assert_eq!(logs[0].data[95], 1);
        assert_eq!(logs[0].data[96], 0xff);
        assert!(logs[0].data[97..].iter().all(|byte| *byte == 0));
        let mut bad = file.reopen();
        bad.push(0);
        assert!(AbiEffects::decode_program_event_envelope(&bad).is_err());
    }

    #[test]
    fn profiled_call_scheduler_binds_complete_activity_and_executes() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use rt::native_contract::{
            call_payload_layout, layerx_programs_call_payload_offset, CanonicalCall,
        };
        let capabilities = CapabilitySet::new([
            Capability::StorageRead,
            Capability::StorageWrite,
            Capability::EmitEvent,
        ])
        .assert_success("test value");
        let declaration = rt::AccessDeclaration::absent();
        let profile = rt::ProgramReplayProfile::new(128, 1_048_576).assert_success("test value");
        let call = CanonicalCall {
            program: program(),
            abi: 4,
            entrypoint: "layerx_call",
            calldata: &[],
            capabilities: &capabilities,
            declaration: &declaration,
            response_capacity: 32,
            budget: budget(),
        };
        let ordinary = call.canonical_bytes(None).assert_success("test value");
        let wrapped = call
            .canonical_bytes(Some(profile))
            .assert_success("test value");
        let layout = call_payload_layout(&wrapped).assert_success("test value");
        assert!(layout.call_offset > 0);
        assert_eq!(&wrapped[layout.call_offset..], ordinary);
        assert_eq!(
            unsafe { layerx_programs_call_payload_offset(wrapped.as_ptr(), wrapped.len()) },
            i32::try_from(layout.call_offset).assert_success("fixture fits i32")
        );
        let grants = &wrapped[layout.capabilities_offset..layout.declaration_offset];
        let reachable =
            CapabilitySet::admitted_schedule_accesses(grants, program(), actor(), &wrapped)
                .assert_success("test value");
        let unwrapped = CapabilitySet::admitted_schedule_accesses(
            &capabilities.canonical_encoding(),
            program(),
            actor(),
            &ordinary,
        )
        .assert_success("test value");
        assert_eq!(reachable, unwrapped);
        let calldata_call = CanonicalCall {
            calldata: b"v",
            ..call
        };
        let calldata_payload = calldata_call
            .canonical_bytes(Some(profile))
            .assert_success("test value");
        let calldata_layout = call_payload_layout(&calldata_payload).assert_success("test value");
        assert_eq!(
            calldata_layout.capabilities_offset,
            layout.capabilities_offset + 1
        );
        assert_eq!(
            unsafe {
                layerx_programs_call_payload_offset(
                    calldata_payload.as_ptr(),
                    calldata_payload.len(),
                )
            },
            i32::try_from(calldata_layout.call_offset).assert_success("fixture fits i32")
        );
        assert_eq!(
            CapabilitySet::admitted_schedule_accesses(
                &calldata_payload
                    [calldata_layout.capabilities_offset..calldata_layout.declaration_offset],
                program(),
                actor(),
                &calldata_payload,
            )
            .assert_success("test value"),
            reachable
        );
        assert_eq!(
            rt::AccessDeclaration::canonical_decode(&wrapped[layout.declaration_offset..])
                .assert_success("test value"),
            declaration
        );
        assert_eq!(reachable.storage_len(), 2);
        assert_profiled_scheduler_execution(profile);
        assert_call_payload_refusals(wrapped, &ordinary);
    }
    fn assert_profiled_scheduler_execution(profile: rt::ProgramReplayProfile) {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let module = WasmEngine::declared()
            .assert_success("test value")
            .validate_versioned(4, &guest_with_initial_value(b'v'))
            .assert_success("test value");
        let mut storage = Storage::new();
        let result = executor()
            .with_program_replay_profile(profile)
            .execute_authorized_versioned_for_qualification(
                &mut storage,
                request(&module, actor(), "layerx_call", &[]),
            )
            .assert_success("test value");
        assert_eq!(result.response().assert_success("test value").bytes, b"v");
        let replay = result.replay_record().assert_success("test value");
        assert_eq!(replay.terminal_status(), 0);
        assert!(replay.boundary_count() > 1);
        assert_eq!(
            storage
                .protocol_namespace_entries(StorageNamespace::principal(program(), actor()))
                .assert_success("test value"),
            [(b"k".to_vec(), b"v".to_vec())]
        );
        let mut calldata_storage = Storage::new();
        let calldata_result = run(&mut calldata_storage, actor(), "layerx_call", b"v");
        assert_eq!(
            calldata_result
                .response()
                .assert_success("test value")
                .bytes,
            b"v"
        );
        assert_eq!(
            calldata_storage
                .protocol_namespace_entries(StorageNamespace::principal(program(), actor()))
                .assert_success("test value"),
            [(b"k".to_vec(), b"v".to_vec())]
        );
    }

    fn assert_call_payload_refusals(wrapped: Vec<u8>, ordinary: &[u8]) {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use rt::native_contract::{call_payload_layout, layerx_programs_call_payload_offset};
        let layout = call_payload_layout(&wrapped).assert_success("payload layout");
        let grants = &wrapped[layout.capabilities_offset..layout.declaration_offset];
        assert_ne!(
            rt::hash_bytes(rt::HashAlgorithm::Sha256, &wrapped).assert_success("test value"),
            rt::hash_bytes(rt::HashAlgorithm::Sha256, ordinary).assert_success("test value")
        );
        assert!(CapabilitySet::admitted_schedule_accesses(
            grants,
            ProgramId::new([0x99; 32]).assert_success("test value"),
            actor(),
            &wrapped
        )
        .is_err());
        assert!(
            CapabilitySet::admitted_schedule_accesses(&[], program(), actor(), &wrapped).is_err()
        );
        let mut malformed = wrapped.clone();
        malformed[34] ^= 1;
        assert!(call_payload_layout(&malformed).is_none());
        assert_eq!(
            unsafe { layerx_programs_call_payload_offset(malformed.as_ptr(), malformed.len()) },
            -1
        );
        let mut trailing = wrapped.clone();
        trailing.push(0);
        assert!(call_payload_layout(&trailing).is_none());
        let mut bad_budget = wrapped;
        bad_budget[layout.call_offset - 12..layout.call_offset - 8]
            .copy_from_slice(&1u32.to_be_bytes());
        assert!(call_payload_layout(&bad_budget).is_none());
    }

    fn integer_guest(extra: u8) -> Vec<u8> {
        module(&[
            type_section(&[(&[TYPE_I32], &[TYPE_I32])]),
            function_section(&[0]),
            exports(&[("compute", 0, 0)]),
            code_section(&[func_body(
                &[],
                &[0x20, 0, 0x41, extra, 0x6a, 0x41, 3, 0x6a, 0x0b],
            )]),
        ])
    }

    #[test]
    fn pinned_dispute_versions_replay_after_fee_update_upgrade_and_archive_restart() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use layerx_program_sdk::arbiter::{
            MARKET_STEP_OPEN, MARKET_STEP_TERMINAL, MARKET_STEP_TRANSITION,
        };

        let sandbox = capture_pinned_sandbox();
        let PinnedSandbox {
            wasm,
            fees,
            baseline,
            ..
        } = &sandbox;
        let fees = *fees;
        let capture = &sandbox.capture;
        let commitments = pinned_commitments(&sandbox);
        let leaves = &capture.boundary_leaves;
        let mut archive = rt::dispute_artifacts::DisputeArtifacts::default();
        archive
            .retain(program(), wasm, 2, rt::FuelSchedule::WASMI_0_31_2, fees)
            .assert_success("test value");
        let disk = DurableFile::save(&archive.canonical_bytes().assert_success("test value"));
        drop(archive);
        let recovered =
            rt::dispute_artifacts::DisputeArtifacts::from_authenticated_bytes(&disk.reopen())
                .assert_success("test value");
        let mut catalog = rt::ProgramCatalog::new().with_dispute_artifacts(recovered);
        catalog.insert(
            program(),
            WasmEngine::declared()
                .assert_success("test value")
                .validate_versioned(2, &integer_guest(9))
                .assert_success("test value"),
        );
        let current_fees = FeeSchedule::new_complete(rt::FeeScheduleParameters {
            version: 2,
            fee_units_per_cpu_fuel: 7,
            fee_units_per_memory_byte: 3,
            fee_units_per_storage_read_byte: 5,
            fee_units_per_storage_write_byte: 11,
            fee_units_per_output_value: 13,
            fee_units_per_output_byte: 17,
            fee_units_per_occupancy_byte_batch: 19,
        });
        let baseline_bytes = baseline
            .replay_state_bytes(1_048_576)
            .assert_success("test value");
        let context = PinnedDisputeContext {
            sandbox: &sandbox,
            commitments: &commitments,
            baseline_bytes: &baseline_bytes,
            catalog: &catalog,
            current_fees,
        };
        for (mode, position) in [
            (MARKET_STEP_OPEN, 0),
            (MARKET_STEP_TRANSITION, 0),
            (
                MARKET_STEP_TERMINAL,
                u32::try_from(leaves.len()).assert_success("fixture fits u32") - 1,
            ),
        ] {
            assert_pinned_step(&context, mode, position);
        }
        let mut corrupted = disk.reopen();
        let last = corrupted.len() - 1;
        corrupted[last] ^= 1;
        assert!(
            rt::dispute_artifacts::DisputeArtifacts::from_authenticated_bytes(&corrupted).is_err()
        );
    }
    struct PinnedSandbox {
        wasm: Vec<u8>,
        module: rt::ValidatedModule,
        fees: FeeSchedule,
        baseline: Storage,
        limits: ResourceBudget,
        lease: [u8; 32],
        authority: rt::replay::MarketSandboxReplayAuthority,
        capture: rt::execute::MarketSandboxExecution,
    }
    struct PinnedCommitments {
        levels: Vec<Vec<[u8; 32]>>,
        profile_bytes: Vec<u8>,
        billing_bytes: Vec<u8>,
    }
    struct PinnedDisputeContext<'a> {
        sandbox: &'a PinnedSandbox,
        commitments: &'a PinnedCommitments,
        baseline_bytes: &'a [u8],
        catalog: &'a rt::ProgramCatalog,
        current_fees: FeeSchedule,
    }

    fn capture_pinned_sandbox() -> PinnedSandbox {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use rt::execute::{
            instantiate_market_sandbox_untrusted, market_sandbox_input_digest, MarketSandboxRequest,
        };
        use rt::replay::{
            market_sandbox_baseline_root, market_sandbox_namespace, MarketSandboxReplayAuthority,
        };

        let wasm = integer_guest(2);
        let module = WasmEngine::declared()
            .assert_success("test value")
            .validate_versioned(2, &wasm)
            .assert_success("test value");
        let fees = FeeSchedule::declared();
        let baseline = Storage::new();
        let limits = ResourceBudget::new_complete(100_000, 65_536, 1024, 1024, 1, 64, 0);
        let lease = [5; 32];
        let authority = MarketSandboxReplayAuthority {
            profile_binding: [0x31; 32],
            namespace: market_sandbox_namespace(program(), lease).assert_success("test value"),
            lease_id: lease,
            namespace_limit: 1024,
            program: program(),
            tenant: actor(),
            payment_account: actor().bytes(),
            code_hash: module.code_hash(),
            input_digest: market_sandbox_input_digest("compute", &[WasmValue::I32(1)])
                .assert_success("test value"),
            runtime_version: rt::RUNTIME_VERSION,
            abi_version: 2,
            fee_schedule_version: fees.version(),
            metering_schedule_version: module.metering_schedule_version(),
            budget: limits,
            fees,
            fee_budget: 1_000_000_000,
            baseline_state_root: market_sandbox_baseline_root(&baseline)
                .assert_success("test value"),
            baseline_storage: baseline.clone(),
        };
        let mut instance =
            instantiate_market_sandbox_untrusted(&module, &authority).assert_success("test value");
        let capture = instance
            .call_market_sandbox_untrusted(MarketSandboxRequest {
                module: &module,
                entrypoint: "compute",
                args: &[WasmValue::I32(1)],
                authority: &authority,
                replay_profile: rt::ProgramReplayProfile::new(128, 1_048_576)
                    .assert_success("test value"),
            })
            .assert_success("test value");
        assert!(capture.terminal_fault.is_none());
        PinnedSandbox {
            wasm,
            module,
            fees,
            baseline,
            limits,
            lease,
            authority,
            capture,
        }
    }

    fn pinned_profile_bytes(sandbox: &PinnedSandbox) -> Vec<u8> {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use layerx_program_sdk::arbiter::{
            MarketSandboxProfile, ProfileLimits, MARKET_SANDBOX_PROFILE_CAPACITY,
        };

        let module = &sandbox.module;
        let capture = &sandbox.capture;
        let authority = &sandbox.authority;
        let limits = sandbox.limits;
        let fees = sandbox.fees;
        let lease = sandbox.lease;
        let mut entrypoint_bytes = [0; 64];
        entrypoint_bytes[..7].copy_from_slice(b"compute");
        let profile = MarketSandboxProfile {
            network_id: 1,
            market_program: [0x55; 32],
            sandbox_program: program().bytes(),
            offer_id: [1; 32],
            lease_id: lease,
            claim_id: [6; 32],
            provider: [2; 32],
            tenant: actor().bytes(),
            code_hash: module.code_hash(),
            input_digest: authority.input_digest,
            attested_input_commitment: [9; 32],
            namespace: authority.namespace,
            baseline_state_root: authority.baseline_state_root,
            initial_execution_state_root: capture.initial_commitment.digest,
            entrypoint_bytes,
            entrypoint_length: 7,
            runtime_version: rt::RUNTIME_VERSION,
            abi_version: 2,
            fee_schedule_version: fees.version(),
            metering_schedule_version: module.metering_schedule_version(),
            limits: ProfileLimits {
                cpu_fuel: limits.cpu_fuel(),
                memory_bytes: limits.memory_bytes(),
                storage_read_bytes: limits.storage_read_bytes(),
                storage_write_bytes: limits.storage_write_bytes(),
                output_values: u64::from(limits.output_values()),
                output_bytes: limits.output_bytes(),
                table_elements: u64::from(limits.table_elements()),
                namespace_bytes: 1024,
            },
            fee_budget: authority.fee_budget,
            interval_start: 4,
            interval_end: 6,
            response_deadline: 16,
            maximum_boundaries: 128,
            maximum_bytes: 1_048_576,
        };
        let mut profile_bytes = vec![0; MARKET_SANDBOX_PROFILE_CAPACITY];
        let len = profile
            .encode(&mut profile_bytes)
            .assert_success("test value");
        profile_bytes.truncate(len);
        profile_bytes
    }

    fn pinned_commitments(sandbox: &PinnedSandbox) -> PinnedCommitments {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use layerx_program_sdk::arbiter::{
            MarketBillingCommitment, MARKET_SANDBOX_BILLING_CAPACITY,
        };
        use rt::portable_replay::{replay_leaf_hash, replay_node_hash};

        let capture = &sandbox.capture;
        let leaves = &capture.boundary_leaves;
        assert!(leaves.len() >= 4);
        let mut levels = vec![leaves
            .iter()
            .enumerate()
            .map(|(index, leaf)| {
                replay_leaf_hash(
                    u32::try_from(index).assert_success("fixture fits u32"),
                    leaf,
                )
                .assert_success("test value")
            })
            .collect::<Vec<_>>()];
        while levels.last().assert_success("test value").len() > 1 {
            let next = levels
                .last()
                .assert_success("test value")
                .chunks(2)
                .map(|pair| replay_node_hash(pair[0], *pair.get(1).unwrap_or(&pair[0])))
                .collect();
            levels.push(next);
        }
        assert_eq!(
            levels.last().assert_success("test value")[0],
            capture.record.boundary_root()
        );
        let profile_bytes = pinned_profile_bytes(sandbox);
        let billing = MarketBillingCommitment {
            profile_digest: rt::hash_bytes(rt::HashAlgorithm::Sha256, &profile_bytes)
                .assert_success("test value"),
            provider_trace_root: capture.record.boundary_root(),
            final_execution_state_root: capture.final_commitment.digest,
            output_digest: capture.output_digest,
            usage: capture.billing_usage(2).assert_success("test value"),
            payable: 10_000,
            challenger_stake: 250_000_000,
            challenge_window_batches: 10,
            boundary_count: u32::try_from(leaves.len()).assert_success("fixture fits u32"),
        };
        let mut billing_bytes = vec![0; MARKET_SANDBOX_BILLING_CAPACITY];
        let len = billing
            .encode(&mut billing_bytes)
            .assert_success("test value");
        billing_bytes.truncate(len);
        PinnedCommitments {
            levels,
            profile_bytes,
            billing_bytes,
        }
    }

    fn pinned_step_evidence(
        sandbox: &PinnedSandbox,
        levels: &[Vec<[u8; 32]>],
        baseline_bytes: &[u8],
        mode: u8,
        position: u32,
    ) -> Vec<u8> {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use layerx_program_sdk::arbiter::MARKET_STEP_TRANSITION;
        use rt::execute::{encode_market_step_evidence_with_header, MarketStepProof};

        let leaves = &sandbox.capture.boundary_leaves;
        let module = &sandbox.module;
        let capture = &sandbox.capture;
        let indices = if mode == MARKET_STEP_TRANSITION {
            vec![position, position + 1]
        } else {
            vec![position]
        };
        let siblings: Vec<Vec<[u8; 32]>> = indices
            .iter()
            .map(|index| {
                let mut pos = *index as usize;
                let mut proof = Vec::new();
                for level in &levels[..levels.len() - 1] {
                    proof.push(*level.get(pos ^ 1).unwrap_or(&level[pos]));
                    pos /= 2;
                }
                proof
            })
            .collect();
        let proofs: Vec<_> = indices
            .iter()
            .zip(&siblings)
            .map(|(index, siblings)| MarketStepProof {
                index: *index,
                leaf: &leaves[*index as usize],
                siblings,
            })
            .collect();
        let initial = module.initial_replay_memory().assert_success("test value");
        let mut memory = initial.clone();
        for leaf in &leaves[..=position as usize] {
            let mut boundary = rt::portable_replay::PortableBoundary::decode_with_header(
                leaf,
                1_048_576,
                capture.record.header(),
            )
            .assert_success("test value");
            memory = boundary
                .rebuild_memory(&memory)
                .assert_success("test value");
        }
        let chunks = rt::replay::ReplayMemory::capture(&memory, &initial)
            .assert_success("test value")
            .changed_chunks;
        encode_market_step_evidence_with_header(
            baseline_bytes,
            &proofs,
            &chunks,
            capture.record.header(),
        )
        .assert_success("test value")
    }

    fn assert_pinned_step(context: &PinnedDisputeContext<'_>, mode: u8, position: u32) {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use layerx_program_sdk::arbiter::{
            MarketStepOutcome, MarketStepRequest, MARKET_STEP_HEADER_CAPACITY,
        };
        use rt::execute::adjudicate_market_step;

        let profile_bytes = &context.commitments.profile_bytes;
        let billing_bytes = &context.commitments.billing_bytes;
        let current_fees = context.current_fees;
        let catalog = context.catalog;

        let evidence = pinned_step_evidence(
            context.sandbox,
            &context.commitments.levels,
            context.baseline_bytes,
            mode,
            position,
        );
        let mut header = vec![0; MARKET_STEP_HEADER_CAPACITY];
        let length = MarketStepRequest {
            mode,
            position,
            profile: profile_bytes,
            billing: billing_bytes,
        }
        .encode(&mut header)
        .assert_success("test value");
        header.truncate(length);
        let mut charged = 0;
        let result = adjudicate_market_step(
            &header,
            &evidence,
            ProgramId::new([0x55; 32]).assert_success("test value"),
            current_fees,
            catalog,
            &mut |fuel| {
                charged += fuel;
                Ok(())
            },
        )
        .assert_success("test value");
        assert!(
            MarketStepOutcome::decode(&result)
                .assert_success("test value")
                .holds,
            "mode {mode}"
        );
        assert!(charged > 0);
        let mut missing = rt::ProgramCatalog::new();
        missing.insert(
            program(),
            WasmEngine::declared()
                .assert_success("test value")
                .validate_versioned(2, &integer_guest(9))
                .assert_success("test value"),
        );
        assert!(adjudicate_market_step(
            &header,
            &evidence,
            ProgramId::new([0x55; 32]).assert_success("test value"),
            current_fees,
            &missing,
            &mut |_| Ok(())
        )
        .is_err());
    }

    #[test]
    fn sandbox_terminal_decoder_persists_usage_and_authenticates_receipt_and_transfer() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use rt::terminal::{attach_sandbox_settlement, SandboxReceiptBinding, SandboxSettlement};
        let principal = rt::native_contract::sandbox_execution_principal(program(), [1; 32])
            .assert_success("test value");
        let record = run(&mut Storage::new(), principal, "layerx_call", b"s");
        let usage = record.execution().usage();
        let settlement = sandbox_settlement_fixture(principal, usage);
        let activity_id = settlement.activity_id;
        let transfer_root = settlement.transfer_root;
        let canonical = settlement
            .canonical_usage_receipt()
            .assert_success("test value");
        let digest =
            rt::hash_bytes(rt::HashAlgorithm::Sha256, &canonical).assert_success("test value");
        let mut frame = canonical.clone();
        SandboxSettlement::append_receipt_digest(&mut frame, digest);
        let binding = SandboxReceiptBinding {
            activity_id,
            program: program().bytes(),
            kind: 1,
            fee_schedule_version: FeeSchedule::declared().version(),
            usage,
            usage_receipt_digest: digest,
            transfer_root,
        };
        let inner = record.canonical_evidence();
        let mut terminal = inner.clone();
        attach_sandbox_settlement(&mut terminal, &frame, transfer_root)
            .assert_success("test value");
        let disk = DurableFile::save(&terminal);
        drop(terminal);
        drop(record);
        let recovered = decode_terminal_payload(1, 4, &disk.reopen()).assert_success("test value");
        let attachment = recovered.sandbox_settlement().assert_success("test value");
        attachment
            .authenticate(binding)
            .assert_success("test value");
        assert_eq!(attachment.lease, [1; 32]);
        assert_eq!(attachment.sequence, 1);
        assert_eq!(
            attachment
                .canonical_usage_receipt()
                .assert_success("test value"),
            canonical
        );
        assert_eq!(
            recovered.detail,
            decode_terminal_payload(1, 4, &inner)
                .assert_success("test value")
                .detail
        );
        for index in 0..4 {
            let mut invalid = binding;
            match index {
                0 => invalid.activity_id[0] ^= 1,
                1 => invalid.usage_receipt_digest[0] ^= 1,
                2 => invalid.transfer_root[0] ^= 1,
                _ => invalid.usage.cpu_fuel += 1,
            }
            assert!(attachment.authenticate(invalid).is_err());
        }
        let mut bad_root = disk.reopen();
        let last = bad_root.len() - 1;
        bad_root[last] ^= 1;
        assert!(decode_terminal_payload(1, 4, &bad_root).is_err());
        let mut duplicate = disk.reopen();
        attach_sandbox_settlement(&mut duplicate, &frame, transfer_root)
            .assert_success("test value");
        assert!(decode_terminal_payload(1, 4, &duplicate).is_err());
        let mut wrong_usage = settlement.clone();
        wrong_usage.usage.cpu_fuel += 1;
        assert!(wrong_usage.canonical_usage_receipt().is_err());
        let mut trailing = canonical.clone();
        trailing.push(0);
        assert!(attach_sandbox_settlement(&mut inner.clone(), &trailing, transfer_root).is_err());
        for end in 0..disk.reopen().len() {
            assert!(decode_terminal_payload(1, 4, &disk.reopen()[..end]).is_err());
        }
    }
    fn sandbox_settlement_fixture(
        principal: PrincipalId,
        usage: rt::MeteredUsage,
    ) -> rt::terminal::SandboxSettlement {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use rt::terminal::SandboxSettlement;
        let activity_id = [2; 32];
        let expected = [3; 32];
        let destination = [4; 32];
        let mut escrow_seed = b"sandbox-lease-escrow/v1\0".to_vec();
        escrow_seed.extend_from_slice(&[1; 32]);
        let escrow = rt::derive_program_account(program(), &escrow_seed)
            .assert_success("test value")
            .bytes();
        let transfer_root = rt::sandbox_escrow_charge_root(&rt::transfer::SandboxEscrowCharge {
            host_program: program(),
            execution_principal: principal,
            invocation_authority: activity_id,
            lease_id: [1; 32],
            expected_lease_digest: expected,
            escrow_account: escrow,
            asset: [6; 32],
            fee_destination: destination,
            amount: usage.fee_units,
        })
        .assert_success("test value");
        SandboxSettlement {
            receipt: Vec::new(),
            receipt_digest: [0; 32],
            transfer_root,
            lease: [1; 32],
            sequence: 1,
            observed_batch: 7,
            activity_id,
            program: program().bytes(),
            kind: 1,
            usage,
            fees: FeeSchedule::declared(),
            charged: usage.fee_units,
            lease_terms_digest: [7; 32],
            expected_lease_digest: expected,
            resulting_lease_digest: [8; 32],
            fee_destination: destination,
            previous_receipt: [0; 32],
            previous_accumulator: [0; 32],
            cumulative_usage: [
                usage.cpu_fuel,
                usage.memory_bytes,
                usage.storage_read_bytes,
                usage.storage_write_bytes,
                u64::from(usage.output_values),
                usage.output_bytes,
                0,
                2,
            ],
            cumulative_spent: usage.fee_units,
        }
    }
}

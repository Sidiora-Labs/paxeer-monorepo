mod market_tests {
    use layerx_programs_runtime::test_support::{
        code_section, export_section, func_body, function_section, module, raw_section,
        type_section, OP_END, OP_I32_CONST, TYPE_I32,
    };
    use layerx_programs_runtime::{
        AuthorizationContext, AuthorizedExecutionRequest, CapabilitySet, CompositionContext,
        CompositionRules, Executor, FeeSchedule, PrincipalId, ProgramCatalog, ProgramId,
        ProgramReplayProfile, ProgramReplayRecord, ResourceBudget, Storage,
        UnavailableReceiptOracle, V2ActivityOutcome, WasmEngine,
    };
    use sha2::{Digest, Sha256};
    use std::rc::Rc;

    fn take<'a>(bytes: &mut &'a [u8], length: usize) -> &'a [u8] {
        let (value, remaining) = bytes.split_at(length);
        *bytes = remaining;
        value
    }

    fn u32_field(bytes: &mut &[u8]) -> u32 {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        u32::from_be_bytes(take(bytes, 4).try_into().assert_success("test value"))
    }

    fn actual_leaves(record: &ProgramReplayRecord) -> Vec<&[u8]> {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        use layerx_programs_runtime::replay_record::{
            PROGRAM_REPLAY_RECORD_DOMAIN, PROGRAM_REPLAY_WITNESS_DOMAIN,
        };
        let mut bytes = record.canonical_bytes();
        assert_eq!(
            take(&mut bytes, PROGRAM_REPLAY_RECORD_DOMAIN.len()),
            PROGRAM_REPLAY_RECORD_DOMAIN
        );
        assert_eq!(take(&mut bytes, 2), 2u16.to_be_bytes());
        let header_length = u32_field(&mut bytes) as usize;
        let header = layerx_programs_runtime::replay_record::ReplayRecordHeader::decode(
            take(&mut bytes, header_length),
            1_048_576,
        )
        .assert_success("test value");
        assert_eq!(&header, record.header());
        take(&mut bytes, 8 + 1 + 4 + 64);
        let witness_length = u32_field(&mut bytes) as usize;
        let witness = take(&mut bytes, witness_length);
        assert!(bytes.is_empty());
        assert_eq!(
            <[u8; 32]>::from(Sha256::digest(witness)),
            record.witness_digest()
        );
        let mut bytes = witness;
        assert_eq!(
            take(&mut bytes, PROGRAM_REPLAY_WITNESS_DOMAIN.len()),
            PROGRAM_REPLAY_WITNESS_DOMAIN
        );
        assert_eq!(
            take(&mut bytes, 32),
            header.digest().assert_success("test value")
        );
        assert_eq!(u32_field(&mut bytes), record.boundary_count());
        let mut leaves = Vec::new();
        let mut hashes = Vec::new();
        for index in 0..record.boundary_count() {
            let length = u32_field(&mut bytes);
            let leaf = take(&mut bytes, length as usize);
            let mut hash = Sha256::new();
            hash.update(b"LXP/program-replay-leaf/v1\0");
            hash.update(index.to_be_bytes());
            hash.update(length.to_be_bytes());
            hash.update(leaf);
            hashes.push(<[u8; 32]>::from(hash.finalize()));
            let _ = captured_functions(leaf, record);
            leaves.push(leaf);
        }
        assert!(bytes.is_empty());
        while hashes.len() > 1 {
            hashes = hashes
                .chunks(2)
                .map(|pair| {
                    let mut hash = Sha256::new();
                    hash.update(b"LXP/program-replay-node/v1\0");
                    hash.update(pair[0]);
                    hash.update(*pair.get(1).unwrap_or(&pair[0]));
                    <[u8; 32]>::from(hash.finalize())
                })
                .collect();
        }
        assert_eq!(hashes[0], record.boundary_root());
        leaves
    }

    fn captured_functions(leaf: &[u8], record: &ProgramReplayRecord) -> Vec<u32> {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let boundary =
            layerx_programs_runtime::portable_replay::PortableBoundary::decode_with_header(
                leaf,
                1_048_576,
                record.header(),
            )
            .assert_success("test value");
        assert_eq!(&boundary.header, record.header());
        assert!(!boundary.arbitration.engine_state.is_empty());
        boundary
            .replay
            .frames
            .iter()
            .map(|frame| frame.module_function_index)
            .collect()
    }

    fn guest(trap: bool, start: bool) -> Vec<u8> {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let mut sections = vec![
            type_section(&[
                (&[TYPE_I32], &[TYPE_I32]),
                (&[TYPE_I32, TYPE_I32], &[TYPE_I32]),
                (&[], &[]),
            ]),
            function_section(if start { &[0, 1, 2] } else { &[0, 1] }),
            raw_section(5, &[1, 1, 1, 1]),
            export_section(&[("layerx_reserve", 0), ("layerx_call", 1)]),
        ];
        let mut exports = sections.pop().assert_success("test value");
        exports[1] += 9;
        exports[2] += 1;
        exports.extend_from_slice(&[6, b'm', b'e', b'm', b'o', b'r', b'y', 2, 0]);
        sections.push(exports);
        if start {
            sections.push(raw_section(8, &[2]));
        }
        let reserve = func_body(&[], &[OP_I32_CONST, 0, OP_END]);
        let entry = func_body(
            &[],
            if trap {
                &[0, OP_END]
            } else {
                &[OP_I32_CONST, 0, OP_END]
            },
        );
        let initializer = func_body(&[], &[OP_I32_CONST, 7, 0x1a, OP_END]);
        let bodies = if start {
            vec![reserve, entry, initializer]
        } else {
            vec![reserve, entry]
        };
        sections.push(code_section(&bodies));
        module(&sections)
    }

    fn try_run(
        trap: bool,
        start: bool,
        profile: bool,
    ) -> Result<
        layerx_programs_runtime::V2AuthorizedExecutionRecord,
        layerx_programs_runtime::ExecutionError,
    > {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let engine = WasmEngine::declared().assert_success("test value");
        let module = engine
            .validate_v2(&guest(trap, start))
            .assert_success("test value");
        let executor = Executor::new(ResourceBudget::declared(), FeeSchedule::declared());
        let executor = if profile {
            executor.with_program_replay_profile(
                ProgramReplayProfile::new(128, 1_048_576).assert_success("test value"),
            )
        } else {
            executor
        };
        executor.execute_authorized_v2(
            &mut Storage::new(),
            AuthorizedExecutionRequest {
                module: &module,
                program: ProgramId::new([1; 32]).assert_success("test value"),
                authorization: AuthorizationContext::new(
                    PrincipalId::new([2; 32]).assert_success("test value"),
                    CapabilitySet::empty(),
                ),
                receipts: &UnavailableReceiptOracle,
                entrypoint: "layerx_call",
                calldata: &[],
                composition: CompositionContext::new(
                    Rc::new(ProgramCatalog::new()),
                    CompositionRules::declared(),
                ),
                response_capacity: 0,
            },
        )
    }

    #[test]
    fn composition_semantic_digest_authenticates_the_decoded_witness() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let result = run(false, false, true);
        let record = result.replay_record().assert_success("replay record");
        for leaf in actual_leaves(record) {
            CompositionWitnessCheck {
                leaf,
                header: record.header(),
            }
            .assert_leaf();
        }
    }
    const COMPOSITION_MAXIMUM: usize = 1_048_576;
    struct CompositionWitnessCheck<'a> {
        leaf: &'a [u8],
        header: &'a layerx_programs_runtime::replay_record::ReplayRecordHeader,
    }
    fn composition_field<'a>(bytes: &mut &'a [u8]) -> &'a [u8] {
        let length = u32_field(bytes) as usize;
        take(bytes, length)
    }

    impl CompositionWitnessCheck<'_> {
        fn assert_refused(&self, changed: &[u8]) {
            use layerx_programs_runtime::test_support::AssertSuccess as _;

            use layerx_programs_runtime::portable_replay::PortableBoundary;
            use layerx_programs_runtime::replay::ReplayWitnessError;

            assert!(matches!(
                PortableBoundary::decode_with_header(changed, COMPOSITION_MAXIMUM, self.header),
                Err(ReplayWitnessError::Binding)
            ));
            let envelope = self
                .header
                .boundary_envelope(changed, COMPOSITION_MAXIMUM)
                .assert_success("test value");
            assert!(matches!(
                PortableBoundary::decode_untrusted(&envelope, COMPOSITION_MAXIMUM),
                Err(ReplayWitnessError::Binding)
            ));
        }
        fn present_value(&self, flag: &[u8], value: &[u8], parents: &[usize]) {
            use layerx_programs_runtime::test_support::AssertSuccess as _;

            let leaf = self.leaf;

            assert_eq!(flag, [0]);
            let offset = flag.as_ptr() as usize - leaf.as_ptr() as usize;
            let mut replacement = vec![1];
            replacement.extend(
                (u32::try_from(value.len()).assert_success("fixture fits u32")).to_be_bytes(),
            );
            replacement.extend(value);
            let extra = replacement.len() - 1;
            let mut changed = leaf.to_vec();
            changed.splice(offset..=offset, replacement);
            for parent in parents {
                let size = u32::from_be_bytes(
                    leaf[*parent..*parent + 4]
                        .try_into()
                        .assert_success("test value"),
                );
                changed[*parent..*parent + 4].copy_from_slice(
                    &(size + u32::try_from(extra).assert_success("fixture fits u32")).to_be_bytes(),
                );
            }
            self.assert_refused(&changed);
        }
        fn assert_decoded_graph(
            &self,
            boundary: &layerx_programs_runtime::portable_replay::PortableBoundary,
            digest: &[u8],
        ) {
            use layerx_programs_runtime::test_support::AssertSuccess as _;

            use layerx_programs_runtime::replay::CompositionReplayWitnessV1;
            let mut expanded =
                &boundary.semantic_bytes[b"LayerX/programs/semantic-replay-source/v2\0".len()..];
            for _ in 0..3 {
                composition_field(&mut expanded);
            }
            let witness = CompositionReplayWitnessV1::decode(
                composition_field(&mut expanded),
                COMPOSITION_MAXIMUM,
            )
            .assert_success("test value");
            assert!(expanded.is_empty());
            assert_eq!(
                digest,
                witness
                    .proposal_digest(COMPOSITION_MAXIMUM)
                    .assert_success("test value")
            );
            let graphs = witness
                .decode_untrusted_state(COMPOSITION_MAXIMUM)
                .assert_success("test value");
            let (revision, graph) = graphs.composition.assert_success("test value");
            assert_eq!(revision, layerx_programs_runtime::AbiRevision::V2);
            assert_eq!(
                graph
                    .replay_state_bytes(COMPOSITION_MAXIMUM)
                    .assert_success("test value"),
                *self
                    .header
                    .composition_graph
                    .as_ref()
                    .assert_success("test value")
            );
            assert_eq!(graph.frames().len(), 1);
            assert!(graphs.failure_graph.is_none());
        }
        fn assert_leaf(&self) {
            use layerx_programs_runtime::test_support::AssertSuccess as _;

            use layerx_programs_runtime::portable_replay::PortableBoundary;
            use layerx_programs_runtime::replay::ReplayWitnessError;
            let leaf = self.leaf;

            let boundary =
                PortableBoundary::decode_with_header(leaf, COMPOSITION_MAXIMUM, self.header)
                    .assert_success("test value");
            let mut bytes = leaf;
            composition_field(&mut bytes);
            for _ in 0..u32_field(&mut bytes) {
                take(&mut bytes, 12);
                let count = u32_field(&mut bytes) as usize;
                take(&mut bytes, count);
            }
            composition_field(&mut bytes);
            take(&mut bytes, 16 + 48);
            let mut semantic = composition_field(&mut bytes);
            let semantic_length_offset = semantic.as_ptr() as usize - leaf.as_ptr() as usize - 4;
            let mut host = composition_field(&mut semantic);
            let host_length_offset = host.as_ptr() as usize - leaf.as_ptr() as usize - 4;
            composition_field(&mut semantic);
            let digest = composition_field(&mut semantic);
            assert!(semantic.is_empty());
            assert_eq!(digest.len(), 32);
            self.assert_decoded_graph(&boundary, digest);
            let mut changed = leaf.to_vec();
            changed[digest.as_ptr() as usize - leaf.as_ptr() as usize] ^= 1;
            self.assert_refused(&changed);
            let host_parents = [semantic_length_offset, host_length_offset];
            let code_hash = take(&mut host, 1);
            let mut wrong_hash = self.header.identity.module_code_hash;
            wrong_hash[0] ^= 1;
            self.present_value(code_hash, &wrong_hash, &host_parents);
            let abi_flag = take(&mut host, 1);
            let mut constants = self.header.semantic_constants.as_slice();
            let mut abi = composition_field(&mut constants).to_vec();
            abi[b"LayerX/programs/v2/host-state\0".len() + 2] ^= 1;
            self.present_value(abi_flag, &abi, &host_parents);
            let mut runtime = composition_field(&mut host);
            let runtime_length_offset = runtime.as_ptr() as usize - leaf.as_ptr() as usize - 4;
            composition_field(&mut host); // Mutable meter stays in this leaf.
            let baseline_flag = take(&mut host, 1);
            self.present_value(baseline_flag, &[0xff], &host_parents);
            assert!(host.is_empty());
            if take(&mut runtime, 1) == [1] {
                take(&mut runtime, 8);
            }
            take(&mut runtime, 8 + 1);
            if take(&mut runtime, 1) == [1] {
                take(&mut runtime, 24);
            }
            let runtime_parents = [
                semantic_length_offset,
                host_length_offset,
                runtime_length_offset,
            ];
            self.present_value(take(&mut runtime, 1), &wrong_hash, &runtime_parents);
            assert_eq!(take(&mut runtime, 1), [1]);
            let revision_flag = take(&mut runtime, 1);
            assert_eq!(revision_flag, [0]);
            assert_eq!(self.header.identity.abi_version, 2);
            self.present_value(revision_flag, &[1], &runtime_parents);
            let graph_flag = take(&mut runtime, 1);
            let mut wrong_graph = self
                .header
                .composition_graph
                .clone()
                .assert_success("test value");
            wrong_graph[0] ^= 1;
            self.present_value(graph_flag, &wrong_graph, &runtime_parents);
            assert_eq!(take(&mut runtime, 1), [0]); // Failure graph.
            assert_eq!(take(&mut runtime, 1), [0]); // Composition refusal.
            assert_eq!(take(&mut runtime, 1), [1]); // Response outcome.
            let capacity_flag = take(&mut runtime, 1);
            let capacity = self.header.response_capacity.assert_success("test value");
            assert_eq!(capacity, 0);
            self.present_value(
                capacity_flag,
                &(capacity + 1).to_be_bytes(),
                &runtime_parents,
            );
            assert_eq!(take(&mut runtime, 2), [0, 0]); // Publication and refusal.
            assert!(runtime.is_empty());
            let mut missing_capacity = self.header.clone();
            missing_capacity.response_capacity = None;
            assert!(matches!(
                PortableBoundary::decode_with_header(leaf, COMPOSITION_MAXIMUM, &missing_capacity),
                Err(ReplayWitnessError::Binding)
            ));
            let envelope = missing_capacity
                .boundary_envelope(leaf, COMPOSITION_MAXIMUM)
                .assert_success("test value");
            assert!(matches!(
                PortableBoundary::decode_untrusted(&envelope, COMPOSITION_MAXIMUM),
                Err(ReplayWitnessError::Binding)
            ));
        }
    }

    #[test]
    fn actual_production_capture_is_opt_in_and_bounded() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        assert!(run(false, false, false).replay_record().is_none());
        let result = run(false, false, true);
        let record = result
            .replay_record()
            .assert_success("actual opt-in boundary producer");
        assert_ne!(record.boundary_root(), [0; 32]);
        assert!(!record.canonical_bytes().is_empty());
        assert!(record.canonical_bytes().len() <= 1_048_576);
        assert_eq!(record.terminal_status(), 0);
        assert!(matches!(
            result.outcome(),
            V2ActivityOutcome::Success { .. }
        ));
        assert!(!actual_leaves(record).is_empty());
        assert_eq!(
            record.canonical_bytes(),
            run(false, false, true)
                .replay_record()
                .assert_success("test value")
                .canonical_bytes()
        );
    }
    #[test]
    fn actual_module_start_refuses_unobservable_start_state() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        // 3e660e358 (done task 104.35.20) enables capture before pre.start in
        // validate.rs::instantiate_state_replay_retained. wasmi's
        // engine/mod.rs::execute_wasm_func maps observer refusal to UnreachableCodeReached
        // before execute.rs::record_legacy_commitments can compare canonical accounting.
        assert_eq!(
            try_run(false, true, true),
            Err(layerx_programs_runtime::ExecutionError::Fault(
                layerx_programs_runtime::ExecutionFault::UnreachableExecuted
            ))
        );
        assert!(matches!(
            try_run(false, true, false)
                .assert_success("test value")
                .outcome(),
            V2ActivityOutcome::Success { .. }
        ));
    }

    #[test]
    fn actual_unreachable_has_a_distinct_retained_terminal_record() {
        use layerx_programs_runtime::test_support::AssertSuccess as _;

        let result = run(true, false, true);
        assert!(result.replay_record().is_some());
        assert_eq!(
            result
                .replay_record()
                .assert_success("test value")
                .terminal_status(),
            1
        );
        assert!(matches!(result.outcome(), V2ActivityOutcome::Failure(_)));
        assert!(result.execution().trace().is_none());
        actual_leaves(result.replay_record().assert_success("test value"));
        assert!(run(true, false, false).replay_record().is_none());
    }
    #[test]
    fn invalid_signed_profile_bounds_are_refused() {
        for bounds in [
            (0, 1),
            (1, 1),
            (128, 0),
            (128, 511),
            (4097, 1_048_576),
            (128, 1_048_577),
            (u32::MAX, 1_048_576),
        ] {
            assert!(ProgramReplayProfile::new(bounds.0, bounds.1).is_err());
        }
    }

    fn run(
        trap: bool,
        start: bool,
        profile: bool,
    ) -> layerx_programs_runtime::V2AuthorizedExecutionRecord {
        try_run(trap, start, profile)
            .unwrap_or_else(|error| panic!("real production producer: {error:?}"))
    }
}

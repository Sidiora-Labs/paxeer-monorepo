mod market_tests {
    use layerx_program_sdk::arbiter::*;
    use layerx_programs_runtime as rt;
    use rt::execute::{
        adjudicate_market_step, encode_market_step_evidence_with_header,
        instantiate_market_sandbox_untrusted, market_sandbox_input_digest, MarketSandboxRequest,
        MarketStepProof, MarketStepRefusal,
    };
    use rt::portable_replay::{
        decode_record, replay_leaf_hash, replay_node_hash, PortableBoundary,
    };
    use rt::replay::{
        market_sandbox_baseline_root, market_sandbox_namespace, replay_memory_root, ReplayMemory,
        ReplayMemoryChunk, REPLAY_MEMORY_CHUNK_BYTES,
    };
    use rt::test_support::{
        code_section, func_body, function_section, import_section, module, raw_section,
        type_section, unsigned_leb, TYPE_I32,
    };
    use rt::{
        AuthorizationContext, AuthorizedExecutionRequest, Capability, CapabilitySet,
        CompositionContext, CompositionRules, Executor, FeeSchedule, PrincipalId, ProgramCatalog,
        ProgramId, ProgramReplayProfile, ResourceBudget, Storage, StorageNamespace,
        UnavailableReceiptOracle, V2ActivityOutcome, WasmEngine, WasmValue,
    };
    use sha2::{Digest, Sha256};
    use std::{io::Write, rc::Rc};

    mod previous_format {
        use super::rt;
        use layerx_programs_runtime::replay::ReplayWitnessError;
        const MAX_PROGRAM_REPLAY_BYTES: u32 = 1_048_576;
        fn append(
            out: &mut Vec<u8>,
            value: &[u8],
            maximum: usize,
        ) -> Result<(), ReplayWitnessError> {
            if out
                .len()
                .checked_add(value.len())
                .is_none_or(|n| n > maximum)
            {
                return Err(ReplayWitnessError::Bounds);
            }
            out.extend_from_slice(value);
            Ok(())
        }
        struct ReplayCursor<'a>(&'a [u8]);
        impl<'a> ReplayCursor<'a> {
            fn new(value: &'a [u8]) -> Self {
                Self(value)
            }
            fn take(&mut self, n: usize) -> Result<&'a [u8], ReplayWitnessError> {
                if self.0.len() < n {
                    return Err(ReplayWitnessError::Encoding);
                }
                let (head, tail) = self.0.split_at(n);
                self.0 = tail;
                Ok(head)
            }
            fn u8(&mut self) -> Result<u8, ReplayWitnessError> {
                Ok(self.take(1)?[0])
            }
            fn u16(&mut self) -> Result<u16, ReplayWitnessError> {
                Ok(u16::from_be_bytes(self.take(2)?.try_into().unwrap()))
            }
            fn u32(&mut self) -> Result<u32, ReplayWitnessError> {
                Ok(u32::from_be_bytes(self.take(4)?.try_into().unwrap()))
            }
            fn done(&self) -> bool {
                self.0.is_empty()
            }
        }
        pub fn portable_state_bytes(
            state: &[u8],
            maximum: usize,
        ) -> Result<Vec<u8>, ReplayWitnessError> {
            portable_payload_bytes(state, &[], maximum)
        }

        pub(super) fn portable_payload_bytes(
            state: &[u8],
            dictionary: &[u8],
            maximum: usize,
        ) -> Result<Vec<u8>, ReplayWitnessError> {
            const WINDOW: usize = u16::MAX as usize;
            const SLOTS: usize = 8192;
            const MIN_MATCH: usize = 6;

            fn slot(bytes: &[u8]) -> usize {
                let word = u32::from_le_bytes([bytes[0], bytes[1], bytes[2], bytes[3]]);
                (word.wrapping_mul(0x9e37_79b1) >> 19) as usize
            }
            fn literal(
                out: &mut Vec<u8>,
                bytes: &[u8],
                maximum: usize,
            ) -> Result<(), ReplayWitnessError> {
                for chunk in bytes.chunks(u16::MAX as usize) {
                    append(out, &[0], maximum)?;
                    append(out, &(chunk.len() as u16).to_be_bytes(), maximum)?;
                    append(out, chunk, maximum)?;
                }
                Ok(())
            }

            let mut bytes = Vec::new();
            append(&mut bytes, &[2], maximum)?;
            append(
                &mut bytes,
                &u32::try_from(state.len())
                    .map_err(|_| ReplayWitnessError::Bounds)?
                    .to_be_bytes(),
                maximum,
            )?;
            let mut previous = Vec::new();
            previous
                .try_reserve_exact(SLOTS)
                .map_err(|_| ReplayWitnessError::Allocation)?;
            previous.resize(SLOTS, usize::MAX);
            let dictionary = &dictionary[dictionary.len().saturating_sub(WINDOW)..];
            let byte_at = |position: usize| {
                if position < dictionary.len() {
                    dictionary[position]
                } else {
                    state[position - dictionary.len()]
                }
            };
            for position in 0..dictionary.len().saturating_sub(3) {
                previous[slot(&dictionary[position..])] = position;
            }
            let mut position = 0;
            let mut literal_start = 0;
            while state.len() - position >= MIN_MATCH {
                let key = slot(&state[position..]);
                let candidate = previous[key];
                let current = dictionary.len() + position;
                previous[key] = current;
                let mut count = 0;
                if candidate != usize::MAX && current - candidate <= WINDOW {
                    let maximum_match = (state.len() - position).min(WINDOW);
                    while count < maximum_match
                        && byte_at(candidate + count) == state[position + count]
                    {
                        count += 1;
                    }
                }
                if count < MIN_MATCH {
                    position += 1;
                    continue;
                }
                literal(&mut bytes, &state[literal_start..position], maximum)?;
                append(&mut bytes, &[1], maximum)?;
                append(&mut bytes, &(count as u16).to_be_bytes(), maximum)?;
                append(
                    &mut bytes,
                    &((current - candidate) as u16).to_be_bytes(),
                    maximum,
                )?;
                let end = position + count;
                position += 1;
                while position < end {
                    if state.len() - position >= MIN_MATCH {
                        previous[slot(&state[position..])] = dictionary.len() + position;
                    }
                    position += 1;
                }
                literal_start = position;
            }
            literal(&mut bytes, &state[literal_start..], maximum)?;
            Ok(bytes)
        }

        pub fn decode_portable_state_bytes_untrusted(
            bytes: &[u8],
            maximum_decoded_bytes: usize,
        ) -> Result<Vec<u8>, ReplayWitnessError> {
            decode_portable_payload_bytes_untrusted(bytes, maximum_decoded_bytes, &[])
        }

        pub fn decode_portable_payload_bytes_untrusted(
            bytes: &[u8],
            maximum_decoded_bytes: usize,
            dictionary: &[u8],
        ) -> Result<Vec<u8>, ReplayWitnessError> {
            let dictionary = &dictionary[dictionary.len().saturating_sub(u16::MAX as usize)..];

            if bytes.len() > MAX_PROGRAM_REPLAY_BYTES as usize
                || maximum_decoded_bytes == 0
                || maximum_decoded_bytes > rt::MAX_ARBITRATION_STATE_BYTES
            {
                return Err(ReplayWitnessError::Bounds);
            }
            let mut cursor = ReplayCursor::new(bytes);
            if cursor.u8()? != 2 {
                return Err(ReplayWitnessError::Encoding);
            }
            let length = cursor.u32()? as usize;
            if length > maximum_decoded_bytes {
                return Err(ReplayWitnessError::Bounds);
            }
            let mut decoded = Vec::new();
            decoded
                .try_reserve_exact(length)
                .map_err(|_| ReplayWitnessError::Allocation)?;
            while decoded.len() < length {
                let tag = cursor.u8()?;
                let count = cursor.u16()? as usize;
                if count == 0 || count > length - decoded.len() {
                    return Err(ReplayWitnessError::Bounds);
                }
                match tag {
                    0 => decoded.extend_from_slice(cursor.take(count)?),
                    1 => {
                        let distance = cursor.u16()? as usize;
                        if count < 6 || distance == 0 || distance > dictionary.len() + decoded.len()
                        {
                            return Err(ReplayWitnessError::Encoding);
                        }
                        for _ in 0..count {
                            let source = dictionary.len() + decoded.len() - distance;
                            let byte = if source < dictionary.len() {
                                dictionary[source]
                            } else {
                                decoded[source - dictionary.len()]
                            };
                            decoded.push(byte);
                        }
                    }
                    _ => return Err(ReplayWitnessError::Encoding),
                }
            }
            if !cursor.done() {
                return Err(ReplayWitnessError::Encoding);
            }
            if portable_payload_bytes(&decoded, dictionary, MAX_PROGRAM_REPLAY_BYTES as usize)?
                != bytes
            {
                return Err(ReplayWitnessError::Encoding);
            }
            Ok(decoded)
        }

        #[cfg(test)]
        mod market_tests {
            use super::{
                decode_portable_payload_bytes_untrusted, decode_portable_state_bytes_untrusted,
                portable_payload_bytes, portable_state_bytes, MAX_PROGRAM_REPLAY_BYTES,
            };
            use layerx_programs_runtime::replay::ReplayWitnessError;
            use sha2::{Digest, Sha256};

            #[test]
            fn boundary_payload_codec_is_lossless_canonical_and_bounded() {
                let maximum = MAX_PROGRAM_REPLAY_BYTES as usize;
                let mut distinct = Vec::new();
                for index in 0u32..4096 {
                    distinct.extend_from_slice(&Sha256::digest(index.to_be_bytes()));
                }
                for original in [
                    Vec::new(),
                    vec![9; 32],
                    vec![0; 131_073],
                    (0u8..=255).cycle().take(131_073).collect(),
                    distinct,
                ] {
                    let encoded = portable_state_bytes(&original, maximum).unwrap();
                    assert_eq!(
                        decode_portable_state_bytes_untrusted(&encoded, maximum).unwrap(),
                        original
                    );
                    assert_eq!(
                        portable_state_bytes(&original, encoded.len()).unwrap(),
                        encoded
                    );
                    assert_eq!(
                        portable_state_bytes(&original, encoded.len() - 1),
                        Err(ReplayWitnessError::Bounds)
                    );
                    if !original.is_empty() {
                        assert_eq!(
                            decode_portable_state_bytes_untrusted(&encoded, original.len() - 1),
                            Err(ReplayWitnessError::Bounds)
                        );
                    }
                    let mut trailing = encoded;
                    trailing.push(0);
                    assert_eq!(
                        decode_portable_state_bytes_untrusted(&trailing, maximum),
                        Err(ReplayWitnessError::Encoding)
                    );
                }
                let dictionary: Vec<_> = (0u32..2048)
                    .flat_map(|index| Sha256::digest(index.to_be_bytes()))
                    .collect();
                let original = &dictionary[dictionary.len() - 4096..];
                let encoded = portable_payload_bytes(original, &dictionary, maximum).unwrap();
                assert!(encoded.len() < portable_state_bytes(original, maximum).unwrap().len());
                assert_eq!(
                    decode_portable_payload_bytes_untrusted(&encoded, maximum, &dictionary)
                        .unwrap(),
                    original
                );
                assert!(decode_portable_state_bytes_untrusted(&encoded, maximum).is_err());
                assert_eq!(
                    decode_portable_payload_bytes_untrusted(
                        &encoded,
                        original.len() - 1,
                        &dictionary
                    ),
                    Err(ReplayWitnessError::Bounds)
                );
                let mut literal = vec![2];
                literal.extend_from_slice(&32u32.to_be_bytes());
                literal.extend_from_slice(&[0, 0, 32]);
                literal.extend_from_slice(&[9; 32]);
                assert_eq!(
                    decode_portable_state_bytes_untrusted(&literal, maximum),
                    Err(ReplayWitnessError::Encoding)
                );
                for distance in [0u16, 1] {
                    let mut before_start = vec![2];
                    before_start.extend_from_slice(&6u32.to_be_bytes());
                    before_start.extend_from_slice(&[1, 0, 6]);
                    before_start.extend_from_slice(&distance.to_be_bytes());
                    assert_eq!(
                        decode_portable_state_bytes_untrusted(&before_start, maximum),
                        Err(ReplayWitnessError::Encoding)
                    );
                }
                let zero_count = [2, 0, 0, 0, 1, 0, 0, 0];
                assert_eq!(
                    decode_portable_state_bytes_untrusted(&zero_count, maximum),
                    Err(ReplayWitnessError::Bounds)
                );
                let overrun = [2, 0, 0, 0, 5, 1, 0, 6, 0, 1];
                assert_eq!(
                    decode_portable_state_bytes_untrusted(&overrun, maximum),
                    Err(ReplayWitnessError::Bounds)
                );
                let truncated = [2, 0, 0, 0, 1, 0, 0, 1];
                assert_eq!(
                    decode_portable_state_bytes_untrusted(&truncated, maximum),
                    Err(ReplayWitnessError::Encoding)
                );
            }
        }
    }

    const MAXIMUM: usize = 1_048_576;
    const CALLS: usize = 128;
    const MEMORY_BYTES: usize = 17 * 65_536;
    const KEY: &[u8] = b"state";
    const VALUE: [u8; 32] = [0x5a; 32];

    fn program() -> ProgramId {
        ProgramId::new([0x77; 32]).unwrap()
    }
    fn market() -> ProgramId {
        ProgramId::new([0x55; 32]).unwrap()
    }
    fn tenant() -> PrincipalId {
        PrincipalId::new([4; 32]).unwrap()
    }

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

    fn exports(entries: &[(&str, u8, u32)]) -> Vec<u8> {
        let mut bytes = unsigned_leb(entries.len() as u64);
        for (name, kind, index) in entries {
            bytes.extend(unsigned_leb(name.len() as u64));
            bytes.extend(name.as_bytes());
            bytes.push(*kind);
            bytes.extend(unsigned_leb(u64::from(*index)));
        }
        raw_section(7, &bytes)
    }

    fn data(entries: &[(i32, &[u8])]) -> Vec<u8> {
        let mut bytes = unsigned_leb(entries.len() as u64);
        for (offset, value) in entries {
            bytes.push(0);
            i32_const(&mut bytes, *offset);
            bytes.push(0x0b);
            bytes.extend(unsigned_leb(value.len() as u64));
            bytes.extend(*value);
        }
        raw_section(11, &bytes)
    }

    fn guest() -> Vec<u8> {
        let mut entry = Vec::new();
        for index in 0..CALLS {
            for argument in [
                1,
                8,
                KEY.len() as i32,
                ((index + 1) * REPLAY_MEMORY_CHUNK_BYTES) as i32,
                VALUE.len() as i32,
            ] {
                i32_const(&mut entry, argument);
            }
            entry.extend([0x10, 0, 0x1a]);
        }
        entry.extend([0x41, 0, 0x0b]);
        module(&[
            type_section(&[
                (&[TYPE_I32; 5], &[TYPE_I32]),
                (&[TYPE_I32], &[TYPE_I32]),
                (&[TYPE_I32; 2], &[TYPE_I32]),
            ]),
            import_section(&[("layerx_v2", "storage_read_scoped", 0)]),
            function_section(&[1, 2]),
            raw_section(5, &[1, 1, 17, 17]),
            exports(&[
                ("layerx_reserve", 0, 1),
                ("layerx_call", 0, 2),
                ("compute", 0, 2),
                ("memory", 2, 0),
            ]),
            code_section(&[func_body(&[], &[0x41, 0, 0x0b]), func_body(&[], &entry)]),
            data(&[(8, KEY)]),
        ])
    }

    fn budget() -> ResourceBudget {
        ResourceBudget::new_complete(
            1_000_000, 16_777_216, 1_048_576, 1_048_576, 64, 1_048_576, 4096,
        )
    }

    fn seed(principal: PrincipalId) -> Storage {
        let mut storage = Storage::new();
        let mut transaction =
            storage.transaction(StorageNamespace::principal(program(), principal));
        transaction.write(KEY, &VALUE).unwrap();
        assert_eq!(transaction.commit(), 1);
        storage
    }

    #[test]
    fn profiled_executor_admits_128_distinct_chunk_writes_in_17_wasm_pages() {
        let validated = WasmEngine::declared()
            .unwrap()
            .validate_v2(&guest())
            .unwrap();
        let executor = Executor::new(budget(), FeeSchedule::declared())
            .with_program_replay_profile(ProgramReplayProfile::new(4096, MAXIMUM as u32).unwrap());
        let mut storage = seed(tenant());
        let before = storage.clone();
        let result = executor
            .execute_authorized_v2(
                &mut storage,
                AuthorizedExecutionRequest {
                    module: &validated,
                    program: program(),
                    authorization: AuthorizationContext::new(
                        tenant(),
                        CapabilitySet::new([Capability::StorageRead]).unwrap(),
                    ),
                    receipts: &UnavailableReceiptOracle,
                    entrypoint: "layerx_call",
                    calldata: &[],
                    composition: CompositionContext::isolated(),
                    response_capacity: 0,
                },
            )
            .unwrap();
        assert!(
            matches!(result.outcome(), V2ActivityOutcome::Success { .. }),
            "{result:?}"
        );
        assert_eq!(storage, before);
        let record = result.replay_record().unwrap();
        let (decoded, leaves) = decode_record(record.canonical_bytes(), MAXIMUM).unwrap();
        assert_eq!(decoded, *record);
        assert!(leaves.iter().map(Vec::len).sum::<usize>() <= MAXIMUM);
        assert!(record.canonical_bytes().len() <= MAXIMUM);
        assert_eq!(
            result.execution().usage().storage_read_bytes,
            CALLS as u64 * (VALUE.len() + KEY.len()) as u64
        );
        let mut memory = validated.initial_replay_memory().unwrap();
        for leaf in &leaves {
            let mut boundary =
                PortableBoundary::decode_with_header(leaf, MAXIMUM, record.header()).unwrap();
            if [456, 734, 743, 900].contains(&boundary.replay.snapshot.step_index) {
                captured_leaf_breakdown(&boundary);
            }
            assert_eq!(boundary.arbitration.identity, record.header().identity);
            assert_eq!(
                boundary.arbitration.legacy.execution_parameters_digest,
                record.header().execution_parameters_digest
            );
            memory = boundary.rebuild_memory(&memory).unwrap();
        }
        assert_header_binding(record);
        for index in 0..CALLS {
            let offset = (index + 1) * REPLAY_MEMORY_CHUNK_BYTES;
            assert_eq!(&memory[offset..offset + VALUE.len()], VALUE.as_slice());
        }
        writeln!(
            std::io::stderr(),
            "repair_22_31_577 executor admitted_count={} leaf_total={} record_total={}",
            CALLS,
            leaves.iter().map(Vec::len).sum::<usize>(),
            record.canonical_bytes().len()
        )
        .unwrap();
    }

    fn fields(bytes: &[u8], prefix: usize, count: usize) -> Vec<&[u8]> {
        let mut bytes = &bytes[prefix..];
        let mut result = Vec::new();
        for _ in 0..count {
            let length = u32::from_be_bytes(bytes[..4].try_into().unwrap()) as usize;
            result.push(&bytes[4..4 + length]);
            bytes = &bytes[4 + length..];
        }
        assert!(bytes.is_empty());
        result
    }

    fn previous_format_breakdown(boundary: &PortableBoundary) {
        // Reconstruct the old canonical preimage before applying its former packing stage.
        let mut arbitration = boundary.arbitration.clone();
        let mut memory = Vec::new();
        memory.extend(boundary.memory.length.to_be_bytes());
        memory.extend(boundary.memory.root);
        memory.extend((boundary.memory.changed_chunks.len() as u32).to_be_bytes());
        for chunk in &boundary.memory.changed_chunks {
            memory.extend(chunk.index.to_be_bytes());
            memory.extend(chunk.bytes);
        }
        std::sync::Arc::make_mut(&mut arbitration.legacy).linear_memory = memory.clone();
        let legacy = arbitration.legacy.canonical_bytes().unwrap();
        let state = arbitration.canonical_bytes().unwrap();
        let packed = previous_format::portable_state_bytes(&state, MAXIMUM).unwrap();
        let semantic = &boundary.semantic_bytes;
        let semantic_parts = fields(
            semantic,
            b"LayerX/programs/semantic-replay-source/v2\0".len(),
            4,
        );
        let packed_semantic =
            previous_format::portable_payload_bytes(semantic, &state, MAXIMUM).unwrap();
        let frames = 4 + boundary
            .replay
            .frames
            .iter()
            .map(|frame| 16 + frame.operand_types.len())
            .sum::<usize>();
        let instruction = 4 + boundary.replay.snapshot.canonical_instruction.len();
        let prefix = b"LXP/program-replay-boundary/v2\0".len();
        let old_total = prefix
            + 4
            + packed.len()
            + frames
            + instruction
            + 16
            + 48
            + 4
            + packed_semantic.len()
            + 1;
        let identity = 218;
        assert_eq!(
            state.len(),
            identity + 4 + legacy.len() + 4 + arbitration.engine_state.len() + 40
        );
        writeln!(std::io::stderr(), "repair_22_31_577 before middle_step={} prefix={prefix} packed_arbitration={} arbitration_framing=4 raw_identity={identity} raw_legacy={} raw_engine={} raw_host_commitment=40 packed_semantic={} semantic_framing=4 raw_semantic_host={} raw_semantic_storage={} raw_semantic_payment={} raw_semantic_composition={} frames={frames} instruction={instruction} fuel_expansion=16 supplement=48 memory={} memory_in_legacy=true trap=1 leaf_total={old_total}", boundary.replay.snapshot.step_index, packed.len(), legacy.len(), arbitration.engine_state.len(), packed_semantic.len(), semantic_parts[0].len(), semantic_parts[1].len(), semantic_parts[2].len(), semantic_parts[3].len(), memory.len()).unwrap();
        let leaf = boundary.reencode_untrusted(MAXIMUM).unwrap();
        let state_length = u32::from_be_bytes(leaf[..4].try_into().unwrap()) as usize;
        let compact_state = &leaf[4..4 + state_length];
        let legacy_length = u32::from_be_bytes(compact_state[2..6].try_into().unwrap()) as usize;
        let compact_semantic_offset = 4 + state_length + frames + instruction + 16 + 48;
        let semantic_length = u32::from_be_bytes(
            leaf[compact_semantic_offset..compact_semantic_offset + 4]
                .try_into()
                .unwrap(),
        ) as usize;
        let compact =
            &leaf[compact_semantic_offset + 4..compact_semantic_offset + 4 + semantic_length];
        let parts = fields(compact, 0, 3);
        assert_eq!(parts[2].len(), 32);
        let composition =
            rt::replay::CompositionReplayWitnessV1::decode(semantic_parts[3], MAXIMUM).unwrap();
        assert_eq!(parts[2], composition.proposal_digest(MAXIMUM).unwrap());
        assert_eq!(
            leaf.len(),
            4 + state_length + frames + instruction + 16 + 48 + 4 + semantic_length + 1
        );
        assert_eq!(
            state_length,
            2 + 4 + legacy_length + 4 + arbitration.engine_state.len() + 40
        );
        writeln!(std::io::stderr(), "repair_22_31_577 after middle_step={} prefix=0 header_binding=0 arbitration={} arbitration_framing=4 leaf_identity=0 legacy={legacy_length} engine={} host_commitment=40 semantic={} semantic_framing=4 semantic_host={} semantic_storage={} semantic_payment=0 semantic_composition={} frames={frames} instruction={instruction} fuel_expansion=16 supplement=48 memory={} memory_in_legacy=true trap=1 leaf_total={} header_total={}", boundary.replay.snapshot.step_index, state_length, arbitration.engine_state.len(), semantic_length, parts[0].len(), parts[1].len(), parts[2].len(), memory.len(), leaf.len(), boundary.header.canonical_bytes(MAXIMUM).unwrap().len()).unwrap();
    }

    fn captured_leaf_breakdown(boundary: &PortableBoundary) {
        previous_format_breakdown(boundary);
        fn take<'a>(bytes: &mut &'a [u8], length: usize) -> &'a [u8] {
            let (value, tail) = bytes.split_at(length);
            *bytes = tail;
            value
        }
        fn field<'a>(bytes: &mut &'a [u8]) -> &'a [u8] {
            let length = u32::from_be_bytes(take(bytes, 4).try_into().unwrap()) as usize;
            take(bytes, length)
        }
        fn delta(bytes: &mut &[u8]) -> usize {
            let present = take(bytes, 1);
            assert!(present == [0] || present == [1]);
            1 + if present == [1] {
                4 + field(bytes).len()
            } else {
                0
            }
        }
        let leaf = boundary.reencode_untrusted(MAXIMUM).unwrap();
        let mut bytes = leaf.as_slice();
        field(&mut bytes);
        let frames = u32::from_be_bytes(take(&mut bytes, 4).try_into().unwrap());
        for _ in 0..frames {
            take(&mut bytes, 12);
            let count = u32::from_be_bytes(take(&mut bytes, 4).try_into().unwrap()) as usize;
            take(&mut bytes, count);
        }
        field(&mut bytes);
        take(&mut bytes, 16 + 48);
        let mut compact = field(&mut bytes);
        let mut host_bytes = field(&mut compact);
        let code = delta(&mut host_bytes);
        let abi = delta(&mut host_bytes);
        let mut runtime = field(&mut host_bytes);
        let runtime_length = runtime.len();
        let meter = field(&mut host_bytes).len();
        let baseline = delta(&mut host_bytes);
        assert!(host_bytes.is_empty());
        if take(&mut runtime, 1) == [1] {
            take(&mut runtime, 8);
        }
        take(&mut runtime, 8 + 1);
        if take(&mut runtime, 1) == [1] {
            take(&mut runtime, 24);
        }
        let composition_code = delta(&mut runtime);
        let mut revision = 0;
        let mut graphs = 2;
        if take(&mut runtime, 1) == [1] {
            revision = delta(&mut runtime);
            graphs += delta(&mut runtime);
        }
        if take(&mut runtime, 1) == [1] {
            graphs += 4 + field(&mut runtime).len();
        }
        // Both admitted proofs have no composition refusal before the outcome.
        assert_eq!(take(&mut runtime, 1), [0]);
        let outcome_encoding = runtime;
        let outcome_bytes = runtime.len();
        let outcome_tag = take(&mut runtime, 1)[0];
        let capacity_bytes = if outcome_tag == 1 {
            delta(&mut runtime)
        } else {
            0
        };
        writeln!(std::io::stderr(), "repair_22_31_577 captured_leaf_breakdown step={} host_code_hash={} host_abi={} host_runtime={} host_meter={} host_storage_baseline={} host_framing=8 runtime_composition_code_hash={} runtime_composition_revision={} runtime_composition_graph={} runtime_outcome={} runtime_outcome_tag={} response_capacity={} response_state={} outcome_encoding={:02x?}",
            boundary.replay.snapshot.step_index, code, abi, runtime_length, meter, baseline, composition_code, revision, graphs, outcome_bytes, outcome_tag, capacity_bytes, runtime.len(), outcome_encoding).unwrap();
        let semantic = fields(
            &boundary.semantic_bytes,
            b"LayerX/programs/semantic-replay-source/v2\0".len(),
            4,
        );
        let host_domain = b"LayerX/programs/replay-host-witness/v1\0";
        // The restored canonical host still contains every header-backed value.
        let host = rt::replay::ReplayHostWitnessV1::decode(semantic[0], MAXIMUM).unwrap();
        assert_eq!(host.code_hash(), boundary.header.identity.module_code_hash);
        let raw = &semantic[0][host_domain.len() + 32..];
        let abi_length = if raw[0] == 1 {
            4 + u32::from_be_bytes(raw[1..5].try_into().unwrap()) as usize
        } else {
            0
        };
        let parts = fields(&raw[1 + abi_length..], 0, 3);
        let composition_domain = b"LayerX/programs/replay-composition-witness/v1\0";
        let composition = &semantic[3][composition_domain.len() + 32..];
        let revision = usize::from(composition[0] == 1);
        let restored_outcome_bytes = if outcome_tag == 1 {
            outcome_bytes - capacity_bytes + 8 + 8
        } else {
            outcome_bytes
        };
        writeln!(std::io::stderr(), "repair_22_31_577 restored_host step={} code_hash=32 abi={} runtime={} meter={} storage_baseline={} composition_code_hash=32 composition_revision={} composition_graph={} runtime_outcome={}",
            boundary.replay.snapshot.step_index, 1 + abi_length, parts[0].len(), parts[1].len(), parts[2].len(), revision, composition.len() - revision, restored_outcome_bytes).unwrap();
    }

    fn assert_header_binding(record: &rt::ProgramReplayRecord) {
        use rt::replay_record::{PROGRAM_REPLAY_RECORD_DOMAIN, PROGRAM_REPLAY_WITNESS_DOMAIN};
        let header = record.header();
        let canonical = header.canonical_bytes(MAXIMUM).unwrap();
        let header_offset = PROGRAM_REPLAY_RECORD_DOMAIN.len() + 2 + 4;
        let baseline_offset = 32 + 32 + 2 + 2 + 4 + 4 + 32 + 12 + 32 * 3 + 4;
        let constants_offset = baseline_offset + header.storage_baseline.len() + 4;
        let value_offset = header
            .storage_baseline
            .windows(VALUE.len())
            .position(|bytes| bytes == VALUE)
            .unwrap();
        // The witness must bind every record constant even when the leaves do not change.
        for offset in [
            32,  // input digest
            152, // receipt oracle root
            184, // balance oracle root
            76,  // execution parameters digest
            constants_offset + 4,
            baseline_offset + value_offset,
            108, // trace policy
            120, // host base state root
        ] {
            let mut changed = record.canonical_bytes().to_vec();
            changed[header_offset + offset] ^= 1;
            assert!(matches!(
                decode_record(&changed, MAXIMUM),
                Err(rt::replay::ReplayWitnessError::Binding)
            ));
        }
        let digest_offset = header_offset + canonical.len() + 8 + 1 + 4 + 32;
        let witness_offset = digest_offset + 32 + 4;
        let mut changed = record.canonical_bytes().to_vec();
        assert_eq!(
            &changed[witness_offset..witness_offset + PROGRAM_REPLAY_WITNESS_DOMAIN.len()],
            PROGRAM_REPLAY_WITNESS_DOMAIN
        );
        let binding_offset = witness_offset + PROGRAM_REPLAY_WITNESS_DOMAIN.len();
        assert_eq!(
            &changed[binding_offset..binding_offset + 32],
            header.digest().unwrap()
        );
        changed[binding_offset] ^= 1;
        let digest = Sha256::digest(&changed[witness_offset..]);
        changed[digest_offset..digest_offset + 32].copy_from_slice(&digest);
        assert!(matches!(
            decode_record(&changed, MAXIMUM),
            Err(rt::replay::ReplayWitnessError::Binding)
        ));
        assert_eq!(
            rt::replay_record::ReplayRecordHeader::decode(&canonical, MAXIMUM).unwrap(),
            *header
        );
        assert_eq!(
            header.canonical_bytes(canonical.len() - 1),
            Err(rt::replay::ReplayWitnessError::Bounds)
        );
        for length in 0..canonical.len() {
            assert!(
                rt::replay_record::ReplayRecordHeader::decode(&canonical[..length], MAXIMUM)
                    .is_err()
            );
        }
        let mut trailing = canonical;
        trailing.push(0);
        assert!(rt::replay_record::ReplayRecordHeader::decode(&trailing, MAXIMUM).is_err());
    }

    fn adjudicator(header: &[u8], evidence: &[u8], expected: i32) -> Vec<u8> {
        const HEADER: i32 = 8;
        const EVIDENCE: i32 = 4096;
        const OUTPUT: i32 = 1_048_576;
        let mut entry = Vec::new();
        for argument in [
            HEADER,
            header.len() as i32,
            EVIDENCE,
            evidence.len() as i32,
            OUTPUT,
            MARKET_STEP_OUTCOME_BYTES as i32,
        ] {
            i32_const(&mut entry, argument);
        }
        entry.extend([0x10, 0]);
        i32_const(&mut entry, expected);
        entry.extend([0x47, 0x04, 0x40, 0x00, 0x0b]);
        if expected == MARKET_STEP_OUTCOME_BYTES as i32 {
            i32_const(&mut entry, OUTPUT);
            entry.extend([0x2d, 0, 0]);
            i32_const(&mut entry, i32::from(MARKET_STEP_HOLDS));
            entry.extend([0x47, 0x04, 0x40, 0x00, 0x0b]);
        }
        entry.extend([0x41, 0, 0x0b]);
        module(&[
            type_section(&[
                (&[TYPE_I32; 6], &[TYPE_I32]),
                (&[TYPE_I32], &[TYPE_I32]),
                (&[TYPE_I32; 2], &[TYPE_I32]),
            ]),
            import_section(&[("layerx_v5", "market_step_adjudicate", 0)]),
            function_section(&[1, 2]),
            raw_section(5, &[1, 1, 17, 17]),
            exports(&[
                ("layerx_reserve", 0, 1),
                ("layerx_call", 0, 2),
                ("memory", 2, 0),
            ]),
            code_section(&[func_body(&[], &[0x41, 0, 0x0b]), func_body(&[], &entry)]),
            data(&[(HEADER, header), (EVIDENCE, evidence)]),
        ])
    }

    fn through_host(catalog: &Rc<ProgramCatalog>, header: &[u8], evidence: &[u8], expected: i32) {
        let engine = WasmEngine::declared().unwrap();
        let validated = engine
            .validate_v5(&adjudicator(header, evidence, expected))
            .unwrap();
        let result = Executor::new(
            ResourceBudget::new_complete(
                100_000_000,
                16_777_216,
                1_048_576,
                1_048_576,
                64,
                1_048_576,
                4096,
            ),
            FeeSchedule::declared(),
        )
        .execute_authorized_versioned_for_qualification(
            &mut Storage::new(),
            AuthorizedExecutionRequest {
                module: &validated,
                program: market(),
                authorization: AuthorizationContext::new(tenant(), CapabilitySet::empty()),
                receipts: &UnavailableReceiptOracle,
                entrypoint: "layerx_call",
                calldata: &[],
                composition: CompositionContext::new(catalog.clone(), CompositionRules::declared()),
                response_capacity: 0,
            },
        )
        .unwrap();
        assert!(
            matches!(result.outcome(), V2ActivityOutcome::Success { .. }),
            "host expected status {expected}: {result:?}"
        );
    }

    #[test]
    fn boundary_roots_rebuild_every_live_snapshot_and_authenticate_middle_step_memory() {
        let module = WasmEngine::declared()
            .unwrap()
            .validate_v2(&guest())
            .unwrap();
        let lease = [5; 32];
        let namespace = market_sandbox_namespace(program(), lease).unwrap();
        let baseline = seed(PrincipalId::new(namespace).unwrap());
        let fees = FeeSchedule::declared();
        let args = [WasmValue::I32(0), WasmValue::I32(0)];
        let authority = rt::replay::MarketSandboxReplayAuthority {
            profile_binding: [0x31; 32],
            namespace,
            lease_id: lease,
            namespace_limit: 1024,
            program: program(),
            tenant: tenant(),
            payment_account: tenant().bytes(),
            code_hash: module.code_hash(),
            input_digest: market_sandbox_input_digest("compute", &args).unwrap(),
            runtime_version: rt::RUNTIME_VERSION,
            abi_version: 2,
            fee_schedule_version: fees.version(),
            metering_schedule_version: module.metering_schedule_version(),
            budget: budget(),
            fees,
            fee_budget: 1_000_000_000,
            baseline_state_root: market_sandbox_baseline_root(&baseline).unwrap(),
            baseline_storage: baseline.clone(),
        };
        let mut memory = module.initial_replay_memory().unwrap();
        assert_eq!(memory.len(), MEMORY_BYTES);
        assert_eq!(&memory[8..13], KEY);
        let initial_memory = memory.clone();
        let mut snapshots = 0;
        let mut admitted = 0;
        let mut changed_chunks = 0;
        let mut instance = instantiate_market_sandbox_untrusted(&module, &authority).unwrap();
        let capture = instance
            .call_market_sandbox_untrusted_observed(
                MarketSandboxRequest {
                    module: &module,
                    entrypoint: "compute",
                    args: &args,
                    authority: &authority,
                    replay_profile: ProgramReplayProfile::new(4096, MAXIMUM as u32).unwrap(),
                },
                &mut |leaf, snapshot| {
                    assert!(leaf.starts_with(b"LXP/program-replay-boundary-context/v2\0"));
                    let mut portable = PortableBoundary::decode_untrusted(leaf, MAXIMUM).unwrap();
                    if admitted == CALLS / 2 && snapshot.snapshot.canonical_instruction == [0x1a] {
                        captured_leaf_breakdown(&portable);
                    }
                    assert_eq!(
                        portable.memory,
                        ReplayMemory::capture(&snapshot.snapshot.linear_memory, &memory).unwrap()
                    );
                    assert!(portable.memory.changed_chunks.len() <= 1);
                    changed_chunks += portable.memory.changed_chunks.len();
                    memory = portable.rebuild_memory(&memory).unwrap();
                    assert_eq!(memory, snapshot.snapshot.linear_memory);
                    assert_eq!(
                        portable.replay.snapshot.linear_memory,
                        snapshot.snapshot.linear_memory
                    );
                    assert_eq!(
                        portable.replay.snapshot.arbitration_instances,
                        snapshot.snapshot.arbitration_instances
                    );
                    assert_eq!(
                        portable.memory.root,
                        replay_memory_root(&snapshot.snapshot.linear_memory).unwrap()
                    );
                    let canonical_leaf = portable.reencode_untrusted(MAXIMUM).unwrap();
                    assert_eq!(
                        PortableBoundary::decode_with_header(
                            &canonical_leaf,
                            MAXIMUM,
                            &portable.header
                        )
                        .unwrap()
                        .header,
                        portable.header
                    );
                    assert_eq!(
                        portable
                            .header
                            .boundary_envelope(&canonical_leaf, MAXIMUM)
                            .unwrap(),
                        leaf
                    );
                    if snapshot.snapshot.canonical_instruction == [0x1a] {
                        assert_eq!(
                            snapshot.snapshot.value_stack.last().unwrap().bits,
                            VALUE.len() as u64 + 1
                        );
                        admitted += 1;
                    }
                    snapshots += 1;
                },
            )
            .unwrap();
        assert!(
            capture.terminal_fault.is_none(),
            "{:?}",
            capture.terminal_fault
        );
        assert_eq!(capture.values, [WasmValue::I32(0)]);
        assert_eq!(admitted, CALLS);
        assert_eq!(changed_chunks, CALLS);
        assert_eq!(snapshots, capture.boundary_leaves.len());
        assert_eq!(capture.record.terminal_status(), 0);
        let (decoded, leaves) = decode_record(capture.record.canonical_bytes(), MAXIMUM).unwrap();
        assert_eq!(decoded, capture.record);
        assert_header_binding(&capture.record);
        assert_eq!(leaves, capture.boundary_leaves);
        let leaf_total: usize = leaves.iter().map(Vec::len).sum();
        assert!(leaf_total <= MAXIMUM, "{leaf_total}");
        assert!(capture.record.canonical_bytes().len() <= MAXIMUM);
        assert_eq!(
            capture.usage.storage_read_bytes,
            CALLS as u64 * (KEY.len() + VALUE.len()) as u64
        );
        writeln!(std::io::stderr(), "repair_22_31_577 sandbox admitted_count={admitted} leaf_total={leaf_total} record_total={}", capture.record.canonical_bytes().len()).unwrap();
        for index in 0..CALLS {
            let start = (index + 1) * REPLAY_MEMORY_CHUNK_BYTES;
            assert_eq!(&memory[start..start + VALUE.len()], VALUE.as_slice());
        }

        let mut levels = vec![leaves
            .iter()
            .enumerate()
            .map(|(index, leaf)| replay_leaf_hash(index as u32, leaf).unwrap())
            .collect::<Vec<_>>()];
        while levels.last().unwrap().len() > 1 {
            levels.push(
                levels
                    .last()
                    .unwrap()
                    .chunks(2)
                    .map(|pair| replay_node_hash(pair[0], *pair.get(1).unwrap_or(&pair[0])))
                    .collect(),
            );
        }
        assert_eq!(levels.last().unwrap()[0], capture.record.boundary_root());
        let position = leaves.len() as u32 / 2;
        let mut pre_memory = initial_memory.clone();
        for leaf in &leaves[..=position as usize] {
            pre_memory =
                PortableBoundary::decode_with_header(leaf, MAXIMUM, capture.record.header())
                    .unwrap()
                    .rebuild_memory(&pre_memory)
                    .unwrap();
        }
        let chunks = ReplayMemory::capture(&pre_memory, &initial_memory)
            .unwrap()
            .changed_chunks;
        assert!(!chunks.is_empty());
        let indices = [position, position + 1];
        let paths: Vec<Vec<[u8; 32]>> = indices
            .iter()
            .map(|index| {
                let mut position = *index as usize;
                let mut path = Vec::new();
                for level in &levels[..levels.len() - 1] {
                    path.push(*level.get(position ^ 1).unwrap_or(&level[position]));
                    position /= 2;
                }
                path
            })
            .collect();
        let proofs: Vec<_> = indices
            .iter()
            .zip(&paths)
            .map(|(index, siblings)| MarketStepProof {
                index: *index,
                leaf: &leaves[*index as usize],
                siblings,
            })
            .collect();
        let baseline_bytes = baseline.replay_state_bytes(MAXIMUM).unwrap();
        let evidence = encode_market_step_evidence_with_header(
            &baseline_bytes,
            &proofs,
            &chunks,
            capture.record.header(),
        )
        .unwrap();
        let mut entrypoint_bytes = [0; 64];
        entrypoint_bytes[..7].copy_from_slice(b"compute");
        let limits = budget();
        let profile = MarketSandboxProfile {
            network_id: 1,
            market_program: market().bytes(),
            sandbox_program: program().bytes(),
            offer_id: [1; 32],
            lease_id: lease,
            claim_id: [6; 32],
            provider: [2; 32],
            tenant: tenant().bytes(),
            code_hash: module.code_hash(),
            input_digest: authority.input_digest,
            attested_input_commitment: [9; 32],
            namespace,
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
            maximum_boundaries: 4096,
            maximum_bytes: MAXIMUM as u32,
        };
        let mut profile_bytes = vec![0; MARKET_SANDBOX_PROFILE_CAPACITY];
        let length = profile.encode(&mut profile_bytes).unwrap();
        profile_bytes.truncate(length);
        let billing = MarketBillingCommitment {
            profile_digest: Sha256::digest(&profile_bytes).into(),
            provider_trace_root: capture.record.boundary_root(),
            final_execution_state_root: capture.final_commitment.digest,
            output_digest: capture.output_digest,
            usage: capture.billing_usage(2).unwrap(),
            payable: 10_000,
            challenger_stake: 250_000_000,
            challenge_window_batches: 10,
            boundary_count: leaves.len() as u32,
        };
        let mut billing_bytes = vec![0; MARKET_SANDBOX_BILLING_CAPACITY];
        let length = billing.encode(&mut billing_bytes).unwrap();
        billing_bytes.truncate(length);
        let mut header = vec![0; MARKET_STEP_HEADER_CAPACITY];
        let length = MarketStepRequest {
            mode: MARKET_STEP_TRANSITION,
            position,
            profile: &profile_bytes,
            billing: &billing_bytes,
        }
        .encode(&mut header)
        .unwrap();
        header.truncate(length);
        let mut catalog = ProgramCatalog::new();
        catalog.insert(program(), module);
        let catalog = Rc::new(catalog);
        through_host(
            &catalog,
            &header,
            &evidence,
            MARKET_STEP_OUTCOME_BYTES as i32,
        );
        let mut altered = chunks.clone();
        altered[0].bytes[17] ^= 1;
        let forged = encode_market_step_evidence_with_header(
            &baseline_bytes,
            &proofs,
            &altered,
            capture.record.header(),
        )
        .unwrap();
        assert_eq!(
            adjudicate_market_step(&header, &forged, market(), fees, &*catalog, &mut |_| Ok(())),
            Err(MarketStepRefusal::Evidence)
        );
        through_host(&catalog, &header, &forged, -5);
        for (mode, index) in [
            (MARKET_STEP_OPEN, position),
            (MARKET_STEP_TERMINAL, leaves.len() as u32 - 1),
        ] {
            let mut memory = initial_memory.clone();
            for leaf in &leaves[..=index as usize] {
                memory =
                    PortableBoundary::decode_with_header(leaf, MAXIMUM, capture.record.header())
                        .unwrap()
                        .rebuild_memory(&memory)
                        .unwrap();
            }
            let mut chunks = ReplayMemory::capture(&memory, &initial_memory)
                .unwrap()
                .changed_chunks;
            assert!(!chunks.is_empty());
            let mut pos = index as usize;
            let mut siblings = Vec::new();
            for level in &levels[..levels.len() - 1] {
                siblings.push(*level.get(pos ^ 1).unwrap_or(&level[pos]));
                pos /= 2;
            }
            let proofs = [MarketStepProof {
                index,
                leaf: &leaves[index as usize],
                siblings: &siblings,
            }];
            let evidence = encode_market_step_evidence_with_header(
                &baseline_bytes,
                &proofs,
                &chunks,
                capture.record.header(),
            )
            .unwrap();
            let length = MarketStepRequest {
                mode,
                position: index,
                profile: &profile_bytes,
                billing: &billing_bytes,
            }
            .encode(&mut header)
            .unwrap();
            header.truncate(length);
            through_host(
                &catalog,
                &header,
                &evidence,
                MARKET_STEP_OUTCOME_BYTES as i32,
            );
            chunks[0].bytes[17] ^= 1;
            let forged = encode_market_step_evidence_with_header(
                &baseline_bytes,
                &proofs,
                &chunks,
                capture.record.header(),
            )
            .unwrap();
            assert_eq!(
                adjudicate_market_step(
                    &header,
                    &forged,
                    market(),
                    fees,
                    &*catalog,
                    &mut |_| Ok(())
                ),
                Err(MarketStepRefusal::Evidence)
            );
            through_host(&catalog, &header, &forged, -5);
        }
    }

    #[test]
    fn memory_tree_binds_length_chunk_position_padding_and_canonical_changes() {
        let previous = vec![0; REPLAY_MEMORY_CHUNK_BYTES * 2 + 1];
        let mut current = previous.clone();
        current[7] = 3;
        current[8192] = 9;
        let memory = ReplayMemory::capture(&current, &previous).unwrap();
        assert_eq!(memory.changed_chunks.len(), 2);
        assert_eq!(memory.rebuild(&previous).unwrap(), current);
        assert_ne!(
            replay_memory_root(&current).unwrap(),
            replay_memory_root(&current[..8192]).unwrap()
        );
        let mut reordered = memory.clone();
        reordered.changed_chunks.reverse();
        assert!(reordered.rebuild(&previous).is_err());
        let mut padding = memory.clone();
        padding.changed_chunks[1].bytes[1] = 1;
        assert!(padding.rebuild(&previous).is_err());
        let mut duplicate = memory.clone();
        duplicate
            .changed_chunks
            .insert(1, duplicate.changed_chunks[0].clone());
        assert!(duplicate.rebuild(&previous).is_err());
        let mut redundant = memory.clone();
        redundant.changed_chunks.insert(
            1,
            ReplayMemoryChunk {
                index: 1,
                bytes: [0; REPLAY_MEMORY_CHUNK_BYTES],
            },
        );
        assert!(redundant.rebuild(&previous).is_err());
        let mut modified = memory.clone();
        modified.changed_chunks[0].bytes[8] ^= 1;
        assert!(modified.rebuild(&previous).is_err());
    }
}

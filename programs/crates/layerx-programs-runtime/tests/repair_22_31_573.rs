use layerx_programs_runtime::abi::response::CANDIDATE_ABI_MODULE;
use layerx_programs_runtime::abi::ABI_MODULE;
use layerx_programs_runtime::portable_replay::{decode_record, PortableBoundary};
use layerx_programs_runtime::replay_record::{ReplayRecordHeader, MAX_PROGRAM_REPLAY_BOUNDARIES};
use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, import_section, module, raw_section, type_section,
    unsigned_leb, TYPE_I32,
};
use layerx_programs_runtime::{
    ActivityBudgetBinding, AuthorizationContext, AuthorizedExecutionRequest,
    BudgetedAuthorizedExecutionRequest, Capability, CapabilitySet, CompositionContext,
    DeclaredBudget, ExecutionError, ExecutionFault, Executor, PrincipalId, ProgramId,
    ProgramReplayProfile, Storage, StorageNamespace, UnavailableReceiptOracle, V2ActivityOutcome,
    V2AuthorizedExecutionRecord, WasmEngine,
};

const MAXIMUM_BYTES: u32 = 1_048_576;
const KEY: &[u8] = b"state";
const VALUE: &[u8] = &[0x5a];
const HOST_CALLS: usize = 128;
const RETAINED_PAGE_BYTES: usize = 4096;
const BASELINE: &[u8] = &[0x33; 32];
const FIRST_OUTPUT: usize = 65536;

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

fn assert_host_boundaries(leaves: &[Vec<u8>], host_calls: usize, header: &ReplayRecordHeader) {
    let module = WasmEngine::declared()
        .unwrap()
        .validate_v2(&large_state_guest(host_calls))
        .unwrap();
    let mut memory = module.initial_replay_memory().unwrap();
    let mut calls = 0;
    for leaf in leaves {
        let mut boundary =
            PortableBoundary::decode_with_header(leaf, MAXIMUM_BYTES as usize, header).unwrap();
        memory = boundary.rebuild_memory(&memory).unwrap();
        let snapshot = &boundary.replay.snapshot;
        assert_eq!(snapshot.linear_memory.len(), 17 * 65536);
        if snapshot.canonical_instruction != [0x1a] {
            continue;
        }
        let status = snapshot.value_stack.last().unwrap().bits;
        if calls < 2 {
            assert_eq!(status, 0);
        } else {
            assert!(calls < host_calls + 2);
            assert_eq!(status, VALUE.len() as u64 + 1);
            for page in 0..=calls - 2 {
                let offset = FIRST_OUTPUT + page * RETAINED_PAGE_BYTES;
                assert_eq!(&snapshot.linear_memory[offset..offset + VALUE.len()], VALUE);
                assert_eq!(snapshot.linear_memory[offset + VALUE.len()], 0);
            }
        }
        calls += 1;
    }
    assert_eq!(calls, host_calls + 2);
}

fn leaf_sizes(leaf: &[u8]) -> (usize, usize, usize) {
    fn u32_field(bytes: &mut &[u8]) -> usize {
        let (value, rest) = bytes.split_at(4);
        *bytes = rest;
        u32::from_be_bytes(value.try_into().unwrap()) as usize
    }
    let mut bytes = leaf;
    let state = u32_field(&mut bytes);
    bytes = &bytes[state..];
    for _ in 0..u32_field(&mut bytes) {
        bytes = &bytes[12..];
        let operands = u32_field(&mut bytes);
        bytes = &bytes[operands..];
    }
    let instruction = u32_field(&mut bytes);
    bytes = &bytes[instruction + 16 + 48..];
    let semantic = u32_field(&mut bytes);
    (state, semantic, leaf.len() - state - semantic)
}

fn large_state_guest(host_calls: usize) -> Vec<u8> {
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
    host_call(&mut entry, 1, &[2, 8, 5, 1024, VALUE.len() as i32]);
    host_call(&mut entry, 2, &[16, 5, 1024, VALUE.len() as i32]);
    for page in 0..host_calls {
        let output = FIRST_OUTPUT + page * RETAINED_PAGE_BYTES;
        host_call(&mut entry, 0, &[2, 8, 5, output as i32, VALUE.len() as i32]);
    }
    entry.extend([0x41, 0, 0x0b]);
    let mut reserve = Vec::new();
    i32_const(&mut reserve, 32768);
    reserve.push(0x0b);
    let mut data = unsigned_leb(3);
    for (offset, bytes) in [(8, KEY), (16, b"event".as_slice()), (1024, VALUE)] {
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
        raw_section(5, &[1, 1, 17, 17]),
        raw_section(7, &exports),
        code_section(&[func_body(&[], &reserve), func_body(&[], &entry)]),
        raw_section(11, &data),
    ])
}

fn run(
    profile: ProgramReplayProfile,
    host_calls: usize,
) -> (Storage, Result<V2AuthorizedExecutionRecord, ExecutionError>) {
    let program = ProgramId::new([1; 32]).unwrap();
    let payer = PrincipalId::new([2; 32]).unwrap();
    let binding = ActivityBudgetBinding::new([3; 32]).unwrap();
    let engine = WasmEngine::declared().unwrap();
    let validated = engine.validate_v2(&large_state_guest(host_calls)).unwrap();
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
    seed.write(KEY, BASELINE).unwrap();
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

fn assert_witness_bound_refusal(
    mut storage: Storage,
    result: Result<V2AuthorizedExecutionRecord, ExecutionError>,
    host_calls: usize,
) {
    let error = result.unwrap_err();
    let ExecutionError::Fault(ExecutionFault::EngineFault { reason }) = &error else {
        panic!("{host_calls} calls must refuse at the witness byte bound: {error:?}");
    };
    if let Some(measurement) = reason.strip_prefix(
        "engine fault: program replay record refused: Bounds: check=capture witness bytes observed=",
    ) {
        let (observed, rest) = measurement.split_once(" permitted=").unwrap();
        let (permitted, rest) = rest.split_once(" leaf_length=").unwrap();
        let (leaf_length, site) = rest.split_once("; capture check at ").unwrap();
        let observed: usize = observed.parse().unwrap();
        let permitted: usize = permitted.parse().unwrap();
        let leaf_length: usize = leaf_length.parse().unwrap();
        assert_eq!(permitted, MAXIMUM_BYTES as usize);
        assert!(observed > permitted);
        assert!(leaf_length > 0 && leaf_length <= permitted);
        assert!(observed.checked_sub(leaf_length + 4).unwrap() <= permitted);
        assert!(site.starts_with("crates/layerx-programs-runtime/src/execute.rs:"));
        assert!(site.contains("; captured_leaf_breakdown: "));
        assert!(site.contains(&format!(" leaf_total={leaf_length} ")));
    } else if let Some(line) = reason.strip_prefix(
        "program replay record refused: Bounds; capture check at crates/layerx-programs-runtime/src/execute.rs:",
    ) {
        assert!(line.parse::<u32>().unwrap() > 0);
    } else {
        assert!(
            reason == "deterministic execution observer refused: SupplementRejected at retain_boundary_capture.source"
                || reason == "program replay record refused: Bounds",
            "{host_calls} calls must refuse at the witness byte bound: {error:?}"
        );
    }
    let transaction =
        storage.transaction(StorageNamespace::shared(ProgramId::new([1; 32]).unwrap()));
    assert_eq!(transaction.read(KEY).unwrap().unwrap(), BASELINE);
    use std::io::Write;
    writeln!(
        std::io::stdout().lock(),
        "repair_22_31_573 refused host_calls={host_calls} error={error:?}",
    )
    .unwrap();
}

fn largest_retained_host_call_count() -> (usize, Storage, V2AuthorizedExecutionRecord) {
    let profile = ProgramReplayProfile::new(MAX_PROGRAM_REPLAY_BOUNDARIES, MAXIMUM_BYTES).unwrap();
    let (storage, result) = run(profile, HOST_CALLS);
    match result {
        Ok(record) => (HOST_CALLS, storage, record),
        Err(error) => {
            assert_witness_bound_refusal(storage, Err(error), HOST_CALLS);
            let mut admitted = 65;
            let mut refused = HOST_CALLS;
            let (mut storage, result) = run(profile, admitted);
            let mut record = result.expect("more than 64 distinct-page host calls must fit");
            while admitted + 1 < refused {
                let candidate = admitted + (refused - admitted) / 2;
                let (candidate_storage, result) = run(profile, candidate);
                match result {
                    Ok(candidate_record) => {
                        admitted = candidate;
                        storage = candidate_storage;
                        record = candidate_record;
                    }
                    Err(error) => {
                        assert_witness_bound_refusal(candidate_storage, Err(error), candidate);
                        refused = candidate;
                    }
                }
            }
            assert_eq!(refused, admitted + 1);
            assert!(admitted > 64);
            (admitted, storage, record)
        }
    }
}

#[test]
fn authenticated_default_stack_guest_retains_replay_for_128_distinct_page_host_calls() {
    let (host_calls, mut storage, record) = largest_retained_host_call_count();
    assert!(host_calls > 64);
    assert!(host_calls <= HOST_CALLS);
    assert!(matches!(
        record.outcome(),
        V2ActivityOutcome::Success { .. }
    ));
    assert!(record.execution().trace().is_none());
    let replay = record.replay_record().unwrap();
    assert_eq!(replay.terminal_status(), 0);
    let (decoded, leaves) =
        decode_record(replay.canonical_bytes(), MAXIMUM_BYTES as usize).unwrap();
    assert_eq!(&decoded, replay);
    assert_eq!(leaves.len(), replay.boundary_count() as usize);
    let leaf_bytes: usize = leaves.iter().map(Vec::len).sum();
    let framed_leaf_bytes = leaf_bytes + 4 * leaves.len();
    assert!(framed_leaf_bytes <= MAXIMUM_BYTES as usize);
    assert!(replay.canonical_bytes().len() <= MAXIMUM_BYTES as usize);
    // Write directly so the bounded gate records measurements for the repair record.
    use std::io::Write;
    writeln!(
        std::io::stdout().lock(),
        "repair_22_31_573 host_calls={host_calls} boundaries={} leaf_bytes={leaf_bytes} framed_leaf_bytes={framed_leaf_bytes} record_bytes={}",
        leaves.len(),
        replay.canonical_bytes().len(),
    )
    .unwrap();
    for (index, leaf) in leaves.iter().enumerate() {
        let boundary =
            PortableBoundary::decode_with_header(leaf, MAXIMUM_BYTES as usize, replay.header())
                .unwrap();
        let (state_bytes, semantic_bytes, metadata_bytes) = leaf_sizes(leaf);
        writeln!(
            std::io::stdout().lock(),
            "repair_22_31_573 boundary={index} leaf_bytes={} portable_state_bytes={state_bytes} semantic_bytes={semantic_bytes} metadata_bytes={metadata_bytes} changed_chunks={}",
            leaf.len(),
            boundary.memory.changed_chunks.len(),
        )
        .unwrap();
    }
    assert_host_boundaries(&leaves, host_calls, replay.header());
    assert!(replay.boundary_count() > 128);
    assert!(replay.boundary_count() <= MAX_PROGRAM_REPLAY_BOUNDARIES);
    assert!(replay.replay_fuel() > 0);
    assert_eq!(
        record.execution().usage().storage_read_bytes,
        host_calls as u64 * (VALUE.len() + KEY.len()) as u64
    );
    assert!(record.execution().usage().cpu_fuel < 1_000_000);
    let effects = record.effects().unwrap();
    assert_eq!(effects.events.len(), 1);
    assert_eq!(effects.events[0].topic, b"event");
    assert_eq!(effects.events[0].data, *VALUE);
    let transaction =
        storage.transaction(StorageNamespace::shared(ProgramId::new([1; 32]).unwrap()));
    assert_eq!(transaction.read(KEY).unwrap().unwrap(), VALUE);
}

#[test]
fn tiny_profile_refuses_at_boundary_bound_without_retained_limit() {
    let (mut storage, result) = run(
        ProgramReplayProfile::new(2, MAXIMUM_BYTES).unwrap(),
        HOST_CALLS,
    );
    assert_eq!(
        result,
        Err(ExecutionError::Fault(ExecutionFault::EngineFault {
            reason: "deterministic execution commitment refused: execution trace exceeds commitment limit 2".to_string(),
        })),
    );
    let transaction =
        storage.transaction(StorageNamespace::shared(ProgramId::new([1; 32]).unwrap()));
    assert_eq!(transaction.read(KEY).unwrap().unwrap(), BASELINE);
}

#[test]
fn default_stack_guest_retains_more_than_64_mib_of_full_boundary_states_within_profile() {
    let host_calls = 8;
    let (_, result) = run(
        ProgramReplayProfile::new(128, MAXIMUM_BYTES).unwrap(),
        host_calls,
    );
    let record = result.unwrap();
    assert!(matches!(
        record.outcome(),
        V2ActivityOutcome::Success { .. }
    ));
    assert!(record.execution().trace().is_none());
    let replay = record.replay_record().unwrap();
    let (decoded, leaves) =
        decode_record(replay.canonical_bytes(), MAXIMUM_BYTES as usize).unwrap();
    assert_eq!(&decoded, replay);
    assert_eq!(leaves.len(), replay.boundary_count() as usize);
    assert!(leaves.len() * 17 * 65536 > 64 * 1024 * 1024);
    assert!(replay.boundary_count() <= 128);
    assert_host_boundaries(&leaves, host_calls, replay.header());
    let module = WasmEngine::declared()
        .unwrap()
        .validate_v2(&large_state_guest(host_calls))
        .unwrap();
    let mut rebuilt = module.initial_replay_memory().unwrap();
    let mut previous = Vec::new();
    let mut changed_pages = 0;
    for (index, leaf) in leaves.iter().enumerate() {
        let mut boundary =
            PortableBoundary::decode_with_header(leaf, MAXIMUM_BYTES as usize, replay.header())
                .unwrap();
        rebuilt = boundary.rebuild_memory(&rebuilt).unwrap();
        let memory = &boundary.replay.snapshot.linear_memory;
        assert_eq!(memory.len(), 17 * 65536);
        let retained = wasmi::ObservationCharge::retained_memory_bytes(&previous, memory).unwrap();
        if index == 0 {
            assert_eq!(retained, 17 * 65536);
        } else {
            assert!(retained == 0 || retained == RETAINED_PAGE_BYTES as u64);
            changed_pages += usize::from(retained != 0);
        }
        previous.clone_from(memory);
    }
    assert_eq!(changed_pages, host_calls);
}

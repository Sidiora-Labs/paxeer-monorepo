//! Repair record 22.33.226 for audit 22.30.126: receipt-trace-bound.
//! Proving tests: receipt_trace_real_producer_round_trips,
//! receipt_trace_maximum_malformed_is_not_copied,
//! receipt_trace_one_past_limit_is_rejected_before_verification,
//! receipt_trace_truncated_length_is_rejected,
//! receipt_response_keeps_its_own_bound, and receipt_graph_keeps_its_own_bound.
//! Prerequisite 22.31.259 already installed the pre-copy trace length check.
//! This repair shares bounded field reads and spells out the v2 encoding bound.
//! Baseline verification was not run: the ordered implementation-first gates
//! prohibit invoking verify_cmd by hand, overriding the conflicting audit step 5.
//! No pre-change test failure is claimed. No out-of-path wiring is required.
//! The gate runner records the verified revision and private evidence location.

use std::alloc::{GlobalAlloc, Layout, System};
use std::cell::Cell;

use layerx_programs_runtime as rt;
use rt::commit::MAX_CANONICAL_ARBITRATION_TRACE_BYTES;
use rt::test_support::{
    code_section, func_body, function_section, import_section, module, raw_section, type_section,
    unsigned_leb, TYPE_I32,
};
use rt::{
    AuthorizationContext, AuthorizedExecutionRequest, CapabilitySet, CompositionContext, Executor,
    FailureEncodingError, FeeSchedule, PrincipalId, ProgramId, ResourceBudget, Storage,
    TracePolicy, UnavailableReceiptOracle, V2ActivityReceipt, V2ReceiptOutcome, WasmEngine,
};

#[derive(Clone, Copy, Default, Debug)]
struct Allocations {
    calls: usize,
    largest: usize,
}

thread_local! {
    static ALLOCATIONS: Cell<Option<Allocations>> = const { Cell::new(None) };
}

struct ObservedAllocator;

fn observe_allocation(size: usize) {
    let _ = ALLOCATIONS.try_with(|slot| {
        if let Some(mut allocations) = slot.get() {
            allocations.calls += 1;
            allocations.largest = allocations.largest.max(size);
            slot.set(Some(allocations));
        }
    });
}

// Forward every operation to the real system allocator. Only allocations made
// by this test thread during the public decoder call are observed.
unsafe impl GlobalAlloc for ObservedAllocator {
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        observe_allocation(layout.size());
        unsafe { System.alloc(layout) }
    }

    unsafe fn alloc_zeroed(&self, layout: Layout) -> *mut u8 {
        observe_allocation(layout.size());
        unsafe { System.alloc_zeroed(layout) }
    }

    unsafe fn dealloc(&self, pointer: *mut u8, layout: Layout) {
        unsafe { System.dealloc(pointer, layout) }
    }

    unsafe fn realloc(&self, pointer: *mut u8, layout: Layout, size: usize) -> *mut u8 {
        observe_allocation(size);
        unsafe { System.realloc(pointer, layout, size) }
    }
}

#[global_allocator]
static ALLOCATOR: ObservedAllocator = ObservedAllocator;

fn decode_observed(bytes: &[u8]) -> (Result<V2ActivityReceipt, FailureEncodingError>, Allocations) {
    ALLOCATIONS.with(|slot| slot.set(Some(Allocations::default())));
    let decoded = V2ActivityReceipt::canonical_decode(bytes);
    let allocations = ALLOCATIONS.with(|slot| slot.replace(None).unwrap());
    (decoded, allocations)
}

fn push_i32(bytes: &mut Vec<u8>, mut value: i32) {
    bytes.push(0x41);
    loop {
        let low = (value & 0x7f) as u8;
        value >>= 7;
        let done = value == 0 && low & 0x40 == 0 || value == -1 && low & 0x40 != 0;
        bytes.push(if done { low } else { low | 0x80 });
        if done {
            break;
        }
    }
}

fn responder(response_length: usize) -> Vec<u8> {
    let pages = response_length.div_ceil(65_536).max(1);
    let mut memory = vec![1, 1];
    memory.extend(unsigned_leb(pages as u64));
    memory.extend(unsigned_leb(pages as u64));
    let mut exports = unsigned_leb(3);
    for (name, kind, index) in [
        ("layerx_reserve", 0, 1),
        ("layerx_call", 0, 2),
        ("memory", 2, 0),
    ] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.extend([kind, index]);
    }
    let mut body = Vec::new();
    for value in [7, 0, i32::try_from(response_length).unwrap()] {
        push_i32(&mut body, value);
    }
    body.extend([0x10, 0, 0x1a, 0x41, 7, 0x0b]);
    module(&[
        type_section(&[
            (&[TYPE_I32; 3], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
        ]),
        import_section(&[("layerx_v2", "response_write", 0)]),
        function_section(&[1, 2]),
        raw_section(5, &memory),
        raw_section(7, &exports),
        code_section(&[func_body(&[], &[0x41, 0, 0x0b]), func_body(&[], &body)]),
    ])
}

fn execute(abi: u16, response_length: usize, traced: bool) -> rt::V2AuthorizedExecutionRecord {
    let module = WasmEngine::declared()
        .unwrap()
        .validate_versioned(abi, &responder(response_length))
        .unwrap();
    let mut executor = Executor::new(
        ResourceBudget::new_complete(
            100_000_000,
            2 * 1_048_576,
            1_048_576,
            1_048_576,
            64,
            rt::MAX_CALL_RESPONSE_BYTES as u64,
            4096,
        ),
        FeeSchedule::declared(),
    );
    if traced {
        executor = executor.with_trace_policy(TracePolicy::new(1, 128).unwrap());
    }
    let record = executor
        .execute_authorized_versioned_for_qualification(
            &mut Storage::new(),
            AuthorizedExecutionRequest {
                module: &module,
                program: ProgramId::new([0x41; 32]).unwrap(),
                authorization: AuthorizationContext::new(
                    PrincipalId::new([0x42; 32]).unwrap(),
                    CapabilitySet::empty(),
                ),
                receipts: &UnavailableReceiptOracle,
                entrypoint: "layerx_call",
                calldata: &[],
                composition: CompositionContext::isolated(),
                response_capacity: response_length,
            },
        )
        .unwrap();
    let response = record.response().unwrap();
    assert_eq!(response.code, 7);
    assert_eq!(response.bytes, vec![0; response_length]);
    record
}

fn graph_length_offset() -> usize {
    b"LXP/program-activity-receipt/v4\0".len() + 32 + 2 + 2 + 4 + 4 + 5 * 8 + 4 + 16
}

fn read_length(bytes: &[u8], offset: usize) -> usize {
    u32::from_be_bytes(bytes[offset..offset + 4].try_into().unwrap()) as usize
}

fn trace_length_offset(receipt: &V2ActivityReceipt) -> usize {
    graph_length_offset() + 4 + receipt.graph_evidence().len() + 1
}

fn replace_trace(receipt: &V2ActivityReceipt, length: usize) -> Vec<u8> {
    let mut encoded = receipt.canonical_encode();
    let offset = trace_length_offset(receipt);
    let old_length = read_length(&encoded, offset);
    let start = offset + 4;
    let mut trace = encoded[start..start + old_length].to_vec();
    trace.resize(length, 0);
    encoded.splice(start..start + old_length, trace);
    encoded[offset..offset + 4].copy_from_slice(&u32::try_from(length).unwrap().to_be_bytes());
    encoded
}

#[test]
fn receipt_trace_real_producer_round_trips() {
    for abi in 2..=5 {
        let record = execute(abi, 3, true);
        let trace = record.execution().trace().unwrap();
        let legacy_length = trace.canonical_bytes().unwrap().len();
        let arbitration_length = trace.arbitration_commitments().len() * (2 + 8 + 32 + 4 + 8);
        let receipt = record.receipt_projection();
        let encoded_trace = receipt.trace_evidence().unwrap();
        assert_eq!(
            encoded_trace.len(),
            2 + 4 + legacy_length + 4 + arbitration_length + 8 + 8
        );
        assert_eq!(
            MAX_CANONICAL_ARBITRATION_TRACE_BYTES,
            60 + rt::MAX_TRACE_COMMITMENTS * (52 + 54)
        );
        assert!(encoded_trace.len() <= MAX_CANONICAL_ARBITRATION_TRACE_BYTES);
        rt::ExecutionTrace::verify_canonical_arbitration_bytes(encoded_trace).unwrap();
        let transport = receipt.canonical_encode();
        let decoded = V2ActivityReceipt::canonical_decode(&transport).unwrap();
        assert_eq!(decoded, receipt);
        assert_eq!(decoded.canonical_encode(), transport);
    }
}

#[test]
fn receipt_trace_maximum_malformed_is_not_copied() {
    let receipt = execute(4, 3, true).receipt_projection();
    let malformed = replace_trace(&receipt, MAX_CANONICAL_ARBITRATION_TRACE_BYTES);
    let offset = trace_length_offset(&receipt);
    assert_eq!(
        read_length(&malformed, offset),
        MAX_CANONICAL_ARBITRATION_TRACE_BYTES
    );
    let (decoded, allocations) = decode_observed(&malformed);
    assert_eq!(decoded, Err(FailureEncodingError::Malformed));
    // At the permitted length the real verifier examines the valid chain
    // prefix and rejects its padded suffix without cloning the large field.
    assert!(allocations.calls > 1, "{allocations:?}");
    assert!(
        allocations.largest < MAX_CANONICAL_ARBITRATION_TRACE_BYTES,
        "{allocations:?}"
    );
}

#[test]
fn receipt_trace_one_past_limit_is_rejected_before_verification() {
    let receipt = execute(4, 3, true).receipt_projection();
    let malformed = replace_trace(&receipt, MAX_CANONICAL_ARBITRATION_TRACE_BYTES + 1);
    let (decoded, allocations) = decode_observed(&malformed);
    assert_eq!(decoded, Err(FailureEncodingError::Malformed));
    // Only the preceding graph is copied: neither trace verification nor a
    // trace-sized allocation is permitted for an oversized field.
    assert_eq!(allocations.calls, 1, "{allocations:?}");
    assert_eq!(allocations.largest, receipt.graph_evidence().len());
}

#[test]
fn receipt_trace_truncated_length_is_rejected() {
    let receipt = execute(4, 3, true).receipt_projection();
    let offset = trace_length_offset(&receipt);
    for length in [MAX_CANONICAL_ARBITRATION_TRACE_BYTES as u32, u32::MAX] {
        let mut truncated = receipt.canonical_encode();
        truncated[offset..offset + 4].copy_from_slice(&length.to_be_bytes());
        truncated.truncate(offset + 4);
        let (decoded, allocations) = decode_observed(&truncated);
        assert_eq!(decoded, Err(FailureEncodingError::Malformed));
        assert_eq!(allocations.calls, 1, "{allocations:?}");
        assert_eq!(allocations.largest, receipt.graph_evidence().len());
    }
}

#[test]
fn receipt_response_keeps_its_own_bound() {
    let receipt = execute(4, rt::MAX_CALL_RESPONSE_BYTES, false).receipt_projection();
    assert!(receipt.trace_evidence().is_none());
    let mut transport = receipt.canonical_encode();
    let decoded = V2ActivityReceipt::canonical_decode(&transport).unwrap();
    assert_eq!(decoded, receipt);
    let V2ReceiptOutcome::Success(response) = decoded.outcome() else {
        panic!("expected real guest response");
    };
    assert_eq!(response.bytes.len(), rt::MAX_CALL_RESPONSE_BYTES);
    let length_offset = trace_length_offset(&receipt) + 1 + 4;
    assert_eq!(
        read_length(&transport, length_offset),
        rt::MAX_CALL_RESPONSE_BYTES
    );
    transport[length_offset..length_offset + 4].copy_from_slice(
        &u32::try_from(rt::MAX_CALL_RESPONSE_BYTES + 1)
            .unwrap()
            .to_be_bytes(),
    );
    transport.push(0);
    let (decoded, allocations) = decode_observed(&transport);
    assert_eq!(decoded, Err(FailureEncodingError::Malformed));
    assert_eq!(allocations.calls, 1, "{allocations:?}");
    assert_eq!(allocations.largest, receipt.graph_evidence().len());
}

#[test]
fn receipt_graph_keeps_its_own_bound() {
    let receipt = execute(4, 3, true).receipt_projection();
    let maximum_graph = b"LayerX/programs/call-graph/v1\0".len()
        + 32
        + 16
        + 8
        + rt::DEFAULT_MAX_CALL_GRAPH_EDGES as usize * 68;
    let mut transport = receipt.canonical_encode();
    let offset = graph_length_offset();
    let old_length = read_length(&transport, offset);
    assert_eq!(old_length, receipt.graph_evidence().len());
    let mut graph = receipt.graph_evidence().to_vec();
    graph.resize(maximum_graph + 1, 0);
    transport.splice(offset + 4..offset + 4 + old_length, graph);
    transport[offset..offset + 4]
        .copy_from_slice(&u32::try_from(maximum_graph + 1).unwrap().to_be_bytes());
    let (decoded, allocations) = decode_observed(&transport);
    assert_eq!(decoded, Err(FailureEncodingError::Malformed));
    assert_eq!(allocations.calls, 0, "{allocations:?}");
}

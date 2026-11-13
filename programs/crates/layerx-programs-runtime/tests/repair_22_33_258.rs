//! Repair record 22.33.258, audit 22.30.130, finding scan-copying-cost.
//! `scan_copying_cost_maximum_page` and `scan_copying_cost_rejected_values` prove
//! bounded allocation through the production store, planner and metered ABI.
//! `scan_copying_cost_cursor_overhead` and `scan_copying_cost_host_wire` prove
//! exact cursor/page bytes, continuation and unchanged host refusal/charging.
//! Baseline verification was not run: the explicit implementation-first order
//! and single verification budget prohibit the conflicting baseline instruction.
//! Wiring request (domain 13): `src/abi/storage_ops.rs` and `src/host/scan.rs` under
//! `programs/crates/layerx-programs-runtime` must pass output capacity and remaining
//! storage-read budget into borrowed planning, preserving refusal precedence.
//! Qualification repair: private failure finalization is observed through an exact
//! full-budget public charge, which succeeds only if the refused scan charged zero.
//! Qualification repair: `ScanCeilingExceeded` has the documented bounds status -3;
//! a malformed producer-issued cursor retains the distinct invalid status -2.
//! The gate runner records the candidate revision and private evidence.

use std::alloc::{GlobalAlloc, Layout, System};
use std::cell::Cell;

use layerx_programs_runtime::storage::{
    ScanLimits, StorageError, MAX_STORAGE_KEY_BYTES, MAX_STORAGE_VALUE_BYTES,
};
use layerx_programs_runtime::{
    Abi, AbiError, AuthorizationContext, Capability, CapabilitySet, FeeSchedule, Meter,
    MeterRefusal, PrincipalId, ProgramId, ResourceBudget, ResourceKind, Storage, StorageNamespace,
    StorageSelector, UnavailableReceiptOracle, ABI_VERSION,
};

// Observe real allocations without replacing any runtime component or transport.
struct AllocationCounter;
thread_local! {
    static ALLOCATED: Cell<Option<usize>> = const { Cell::new(None) };
}

fn count_allocation(bytes: usize) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let _ = ALLOCATED.try_with(|counter| {
        if let Some(total) = counter.get() {
            counter.set(Some(total.checked_add(bytes).assert_success("test value")));
        }
    });
}

unsafe impl GlobalAlloc for AllocationCounter {
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        count_allocation(layout.size());
        unsafe { System.alloc(layout) }
    }

    unsafe fn alloc_zeroed(&self, layout: Layout) -> *mut u8 {
        count_allocation(layout.size());
        unsafe { System.alloc_zeroed(layout) }
    }

    unsafe fn realloc(&self, ptr: *mut u8, layout: Layout, size: usize) -> *mut u8 {
        count_allocation(size);
        unsafe { System.realloc(ptr, layout, size) }
    }

    unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
        unsafe { System.dealloc(ptr, layout) }
    }
}

#[global_allocator]
static ALLOCATOR: AllocationCounter = AllocationCounter;

fn measured<T>(run: impl FnOnce() -> T) -> (T, usize) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    ALLOCATED.with(|counter| assert_eq!(counter.replace(Some(0)), None));
    let result = run();
    let bytes = ALLOCATED.with(|counter| counter.replace(None).assert_success("test value"));
    (result, bytes)
}

fn owner() -> ProgramId {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    ProgramId::new([1; 32]).assert_success("test value")
}

fn actor() -> PrincipalId {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    PrincipalId::new([2; 32]).assert_success("test value")
}

fn abi(storage: Storage, shared: bool) -> Abi {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    Abi::new(
        ABI_VERSION,
        owner(),
        AuthorizationContext::new(
            actor(),
            CapabilitySet::new([if shared {
                Capability::SharedStorageRead
            } else {
                Capability::StorageRead
            }])
            .assert_success("test value"),
        ),
        storage,
        &UnavailableReceiptOracle,
    )
    .assert_success("test value")
}

fn meter(read_bytes: u64) -> Meter {
    Meter::new(
        ResourceBudget::new(1, 1, read_bytes, 1, 1, 1),
        FeeSchedule::declared(),
    )
}

#[test]
fn scan_copying_cost_maximum_page() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let namespace = StorageNamespace::principal(owner(), actor());
    let prefix = vec![b'p'; MAX_STORAGE_KEY_BYTES - 1];
    let mut storage = Storage::new();
    let mut transaction = storage.transaction(namespace);
    let mut key = prefix.clone();
    key.push(0);
    let mut value = vec![0; MAX_STORAGE_VALUE_BYTES];
    for index in 0..65_u8 {
        key[MAX_STORAGE_KEY_BYTES - 1] = index;
        value.fill(index);
        transaction.write(&key, &value).assert_success("test value");
    }
    assert_eq!(transaction.commit(), 65);
    let mut runtime = abi(storage, false);
    let limits = ScanLimits::declared();
    let cursor_bytes = 14 + 65 + prefix.len() + MAX_STORAGE_KEY_BYTES;
    let expected_bytes =
        5 + 64 * (6 + MAX_STORAGE_KEY_BYTES + MAX_STORAGE_VALUE_BYTES) + cursor_bytes;
    let mut budget = meter(u64::try_from(expected_bytes).assert_success("test value"));
    let (result, allocated) = measured(|| {
        runtime.storage_scan_selected(
            &mut budget,
            StorageSelector::Principal,
            &prefix,
            b"",
            limits,
        )
    });
    let page = result.assert_success("test value");
    assert_eq!(page.entries().len(), 64);
    assert_eq!(
        page.metered_bytes(),
        u64::try_from(expected_bytes).assert_success("test value")
    );
    assert!(page.metered_bytes() <= u64::from(limits.max_bytes()));
    // At most one owned copy of each accepted key/value, plus bounded metadata.
    assert!(
        allocated <= expected_bytes + 16_384,
        "allocated {allocated} for {expected_bytes} bytes"
    );
    for (index, entry) in page.entries().iter().enumerate() {
        assert_eq!(&entry.key[..MAX_STORAGE_KEY_BYTES - 1], prefix);
        assert_eq!(usize::from(entry.key[MAX_STORAGE_KEY_BYTES - 1]), index);
        assert_eq!(entry.value.len(), MAX_STORAGE_VALUE_BYTES);
        assert!(entry.value.iter().all(|byte| usize::from(*byte) == index));
    }
    let cursor = page.cursor().assert_success("test value");
    assert_eq!(cursor.len(), cursor_bytes);
    assert_eq!(&cursor[..2], &[1, 65]);
    assert_eq!(&cursor[2..67], namespace.canonical_bytes());
    assert_eq!(&cursor[67..69], &255_u16.to_be_bytes());
    assert_eq!(&cursor[69..324], prefix);
    assert_eq!(&cursor[324..328], &limits.max_entries().to_be_bytes());
    assert_eq!(&cursor[328..332], &limits.max_bytes().to_be_bytes());
    assert_eq!(&cursor[332..334], &256_u16.to_be_bytes());
    assert_eq!(&cursor[334..], page.entries()[63].key);
    let usage = budget.finish().assert_success("test value");
    assert_eq!(
        usage.storage_read_bytes,
        u64::try_from(expected_bytes).assert_success("test value")
    );
    assert_eq!(usage.output_bytes, 0);

    let terminal_bytes = 5 + 6 + MAX_STORAGE_KEY_BYTES + MAX_STORAGE_VALUE_BYTES;
    let mut budget = meter(u64::try_from(terminal_bytes).assert_success("test value"));
    let next = runtime
        .storage_scan_selected(
            &mut budget,
            StorageSelector::Principal,
            &prefix,
            cursor,
            limits,
        )
        .assert_success("test value");
    assert_eq!(next.entries().len(), 1);
    assert!(next.entries()[0].value.iter().all(|byte| *byte == 64));
    assert_eq!(next.cursor(), None);
    assert_eq!(
        next.metered_bytes(),
        u64::try_from(terminal_bytes).assert_success("test value")
    );
    assert_eq!(
        budget
            .finish()
            .assert_success("test value")
            .storage_read_bytes,
        next.metered_bytes()
    );
}

#[test]
fn scan_copying_cost_rejected_values() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let namespace = StorageNamespace::principal(owner(), actor());
    let large = vec![0xa5; MAX_STORAGE_VALUE_BYTES];
    let mut storage = Storage::new();
    let mut transaction = storage.transaction(namespace);
    transaction.write(b"a", &large).assert_success("test value");
    assert_eq!(transaction.commit(), 1);
    let mut runtime = abi(storage, false);
    for ceiling in [
        5,
        12,
        u32::try_from(11 + MAX_STORAGE_VALUE_BYTES).assert_success("test value"),
    ] {
        let mut budget = meter(u64::MAX);
        let (result, allocated) = measured(|| {
            runtime.storage_scan_selected(
                &mut budget,
                StorageSelector::Principal,
                b"",
                b"",
                ScanLimits::new(64, ceiling).assert_success("test value"),
            )
        });
        assert_eq!(
            result,
            Err(AbiError::Storage(StorageError::ScanCeilingExceeded))
        );
        assert!(
            allocated < 16_384,
            "rejected value allocated {allocated} bytes"
        );
        assert_eq!(
            budget
                .finish()
                .assert_success("test value")
                .storage_read_bytes,
            0
        );
    }

    let mut storage = Storage::new();
    let mut transaction = storage.transaction(namespace);
    transaction.write(b"a", b"x").assert_success("test value");
    transaction.write(b"b", &large).assert_success("test value");
    assert_eq!(transaction.commit(), 2);
    let mut runtime = abi(storage, false);
    let mut budget = meter(93);
    let (result, allocated) = measured(|| {
        runtime.storage_scan_selected(
            &mut budget,
            StorageSelector::Principal,
            b"",
            b"",
            ScanLimits::new(64, 93).assert_success("test value"),
        )
    });
    let page = result.assert_success("test value");
    assert_eq!(page.entries().len(), 1);
    assert_eq!(page.entries()[0].key, b"a");
    assert_eq!(page.entries()[0].value, b"x");
    assert_eq!(page.cursor().assert_success("test value").len(), 80);
    assert_eq!(page.metered_bytes(), 93);
    assert!(
        allocated < 16_384,
        "rejected second value allocated {allocated} bytes"
    );
    assert_eq!(
        budget
            .finish()
            .assert_success("test value")
            .storage_read_bytes,
        93
    );
}

#[test]
fn scan_copying_cost_cursor_overhead() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    for shared in [false, true] {
        let namespace = if shared {
            StorageNamespace::shared(owner())
        } else {
            StorageNamespace::principal(owner(), actor())
        };
        let selector = if shared {
            StorageSelector::Shared
        } else {
            StorageSelector::Principal
        };
        let prefix = vec![b'p'; 255];
        let mut key = prefix.clone();
        key.push(0);
        let first_key = key.clone();
        let mut storage = Storage::new();
        let mut transaction = storage.transaction(namespace);
        transaction.write(&key, b"").assert_success("test value");
        key[255] = 1;
        transaction.write(&key, b"").assert_success("test value");
        assert_eq!(transaction.commit(), 2);
        let mut runtime = abi(storage, shared);
        let cursor_bytes = 14 + namespace.canonical_bytes().len() + 255 + 256;
        let bytes = u32::try_from(5 + 262 + cursor_bytes).assert_success("test value");
        let limits = ScanLimits::new(1, bytes).assert_success("test value");
        let mut budget = meter(u64::from(bytes));
        let page = runtime
            .storage_scan_selected(&mut budget, selector, &prefix, b"", limits)
            .assert_success("test value");
        assert_eq!(page.entries()[0].key, first_key);
        assert!(page.entries()[0].value.is_empty());
        assert_eq!(
            page.cursor().assert_success("test value").len(),
            cursor_bytes
        );
        assert_eq!(page.metered_bytes(), u64::from(bytes));
        assert_eq!(
            budget
                .finish()
                .assert_success("test value")
                .storage_read_bytes,
            u64::from(bytes)
        );
        let mut budget = meter(u64::from(bytes));
        assert_eq!(
            runtime.storage_scan_selected(
                &mut budget,
                selector,
                &prefix,
                b"",
                ScanLimits::new(1, bytes - 1).assert_success("test value"),
            ),
            Err(AbiError::Storage(StorageError::ScanCeilingExceeded))
        );
        assert_eq!(
            budget
                .finish()
                .assert_success("test value")
                .storage_read_bytes,
            0
        );
        let mut budget = meter(267);
        let terminal = runtime
            .storage_scan_selected(
                &mut budget,
                selector,
                &prefix,
                page.cursor().assert_success("test value"),
                limits,
            )
            .assert_success("test value");
        assert_eq!(terminal.entries()[0].key, key);
        assert_eq!(terminal.cursor(), None);
        assert_eq!(terminal.metered_bytes(), 267);
        assert_eq!(
            budget
                .finish()
                .assert_success("test value")
                .storage_read_bytes,
            267
        );
        let mut budget = meter(u64::from(bytes - 1));
        let refusal = MeterRefusal::BudgetExceeded {
            resource: ResourceKind::StorageRead,
            limit: u64::from(bytes - 1),
            attempted: u64::from(bytes),
        };
        assert_eq!(
            runtime.storage_scan_selected(&mut budget, selector, &prefix, b"", limits),
            Err(AbiError::Meter(refusal))
        );
        assert_eq!(budget.finish(), Err(refusal));
        // A successful charge of the entire original allowance proves the refused
        // scan left the admitted storage-read counter at exactly zero.
        assert_eq!(budget.charge_storage_read(u64::from(bytes - 1)), Ok(()));
        assert_eq!(budget.finish(), Err(refusal));
    }
}

fn push_i32(body: &mut Vec<u8>, mut value: i32) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    body.push(0x41);
    loop {
        let byte = u8::try_from(value & 0x7f).assert_success("test value");
        value >>= 7;
        let done = (value == 0 && byte & 0x40 == 0) || (value == -1 && byte & 0x40 != 0);
        body.push(if done { byte } else { byte | 0x80 });
        if done {
            break;
        }
    }
}

fn host_guest(
    cursor: &[u8],
    ceiling: i32,
    capacity: i32,
    response_bytes: i32,
    status: bool,
) -> Vec<u8> {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    use layerx_programs_runtime::test_support::{
        code_section, func_body, function_section, import_section, module, raw_section,
        type_section, unsigned_leb, TYPE_I32,
    };
    let mut exports = vec![3];
    for (name, kind, index) in [
        ("layerx_reserve", 0, 2),
        ("layerx_call", 0, 3),
        ("memory", 2, 0),
    ] {
        exports.extend(unsigned_leb(
            u64::try_from(name.len()).assert_success("test value"),
        ));
        exports.extend_from_slice(name.as_bytes());
        exports.extend([kind, index]);
    }
    let mut body = Vec::new();
    for argument in [
        1,
        128,
        0,
        512,
        i32::try_from(cursor.len()).assert_success("test value"),
        64,
        ceiling,
        1024,
        capacity,
    ] {
        push_i32(&mut body, argument);
    }
    body.extend([0x10, 0, 0x21, 0]); // scan -> local 0
    push_i32(&mut body, 64);
    body.extend([0x20, 0, 0x36, 2, 0]); // preserve scan result beside sentinel
    for argument in [0, if status { 64 } else { 1024 }, response_bytes] {
        push_i32(&mut body, argument);
    }
    body.extend([0x10, 1, 0x1a]);
    push_i32(&mut body, 0);
    body.push(0x0b);
    let mut data = vec![2];
    for (offset, bytes) in [(68, b"keep".as_slice()), (512, cursor)] {
        data.push(0);
        push_i32(&mut data, offset);
        data.push(0x0b);
        data.extend(unsigned_leb(
            u64::try_from(bytes.len()).assert_success("test value"),
        ));
        data.extend_from_slice(bytes);
    }
    module(&[
        type_section(&[
            (&[TYPE_I32; 9], &[TYPE_I32]),
            (&[TYPE_I32; 3], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
        ]),
        import_section(&[
            ("layerx_v2", "storage_scan_scoped", 0),
            ("layerx_v2", "response_write", 1),
        ]),
        function_section(&[2, 3]),
        raw_section(5, &[1, 1, 1, 1]),
        raw_section(7, &exports),
        code_section(&[
            func_body(&[], &[0x41, 0, 0x0b]),
            func_body(&[(1, TYPE_I32)], &body),
        ]),
        raw_section(11, &data),
    ])
}

fn host_scan(
    storage: &mut Storage,
    cursor: &[u8],
    ceiling: i32,
    capacity: i32,
    response_bytes: i32,
    status: bool,
) -> (Vec<u8>, u64) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    use layerx_programs_runtime::{
        AuthorizedExecutionRequest, CompositionContext, Executor, WasmEngine, WasmValue,
        CALL_ENTRY_EXPORT,
    };
    let engine = WasmEngine::declared().assert_success("test value");
    let wasm = host_guest(cursor, ceiling, capacity, response_bytes, status);
    let module = engine
        .validate_candidate_v2(&wasm)
        .assert_success("test value");
    let record = Executor::new(ResourceBudget::declared(), FeeSchedule::declared())
        .execute_authorized_candidate(
            storage,
            AuthorizedExecutionRequest {
                module: &module,
                program: owner(),
                authorization: AuthorizationContext::new(
                    actor(),
                    CapabilitySet::new([Capability::StorageRead]).assert_success("test value"),
                ),
                receipts: &UnavailableReceiptOracle,
                entrypoint: CALL_ENTRY_EXPORT,
                calldata: &[],
                composition: CompositionContext::isolated(),
                response_capacity: usize::try_from(response_bytes).assert_success("test value"),
            },
        )
        .assert_success("test value");
    assert_eq!(record.execution().outputs(), [WasmValue::I32(0)]);
    (
        record.response().assert_success("test value").bytes.clone(),
        record.execution().usage().storage_read_bytes,
    )
}

fn expected_small_page(keys: &[u8], ceiling: u32, continuation: bool) -> Vec<u8> {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut page = u16::try_from(keys.len())
        .assert_success("test value")
        .to_be_bytes()
        .to_vec();
    for key in keys {
        page.extend_from_slice(&[0, 1, *key, 0, 0, 0, 1, *key]);
    }
    if continuation {
        page.extend([1, 0, 80, 1, 65]);
        page.extend_from_slice(&owner().bytes());
        page.push(0);
        page.extend_from_slice(&actor().bytes());
        page.extend_from_slice(&0_u16.to_be_bytes());
        page.extend_from_slice(&64_u32.to_be_bytes());
        page.extend_from_slice(&ceiling.to_be_bytes());
        page.extend([0, 1, *keys.last().assert_success("test value")]);
    } else {
        page.extend([0, 0, 0]);
    }
    page
}

#[test]
fn scan_copying_cost_host_wire() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let namespace = StorageNamespace::principal(owner(), actor());
    let mut storage = Storage::new();
    let mut transaction = storage.transaction(namespace);
    for key in [b'd', b'b', b'a', b'c'] {
        transaction
            .write(&[key], &[key])
            .assert_success("test value");
    }
    assert_eq!(transaction.commit(), 4);
    let before = storage.clone();
    let (page, read) = host_scan(&mut storage, b"", 101, 101, 101, false);
    assert_eq!(page, expected_small_page(b"ab", 101, true));
    assert_eq!(read, 101);
    // Feed the real producer's encoded cursor back through guest memory and host.
    let cursor = &page[21..];
    let (terminal, read) = host_scan(&mut storage, cursor, 101, 21, 21, false);
    assert_eq!(terminal, expected_small_page(b"cd", 101, false));
    assert_eq!(read, 21);
    let (page, read) = host_scan(&mut storage, b"", 100, 93, 93, false);
    assert_eq!(page, expected_small_page(b"a", 100, true));
    assert_eq!(read, 93);
    for (ceiling, capacity, code) in [(101, 100, -3_i32), (5, 5, -3)] {
        let (refusal, read) = host_scan(&mut storage, b"", ceiling, capacity, 8, true);
        let mut expected = code.to_le_bytes().to_vec();
        expected.extend_from_slice(b"keep");
        assert_eq!(refusal, expected);
        assert_eq!(read, 0);
    }
    let mut malformed_cursor = cursor.to_vec();
    malformed_cursor.push(0);
    let (refusal, read) = host_scan(&mut storage, &malformed_cursor, 101, 101, 8, true);
    let mut expected = (-2_i32).to_le_bytes().to_vec();
    expected.extend_from_slice(b"keep");
    assert_eq!(refusal, expected);
    assert_eq!(read, 0);
    assert_eq!(storage, before);
    let mut empty = Storage::new();
    let (page, read) = host_scan(&mut empty, b"", 5, 5, 5, false);
    assert_eq!(page, [0; 5]);
    assert_eq!(read, 5);
}

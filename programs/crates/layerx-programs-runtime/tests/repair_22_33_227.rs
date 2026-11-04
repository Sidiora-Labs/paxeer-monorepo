//! Repair record 22.33.227 for audit 22.30.126.
//! hash-preflight: hash_preflight_oversized_valid_memory,
//! hash_preflight_invalid_input_and_output_ranges, hash_preflight_exhausted_fuel,
//! and hash_preflight_valid_vectors_and_ceiling.
//! guest-read-allocation: guest_read_invalid_large_ranges_do_not_allocate and
//! guest_read_valid_range_still_reaches_storage.
//! Baseline verification was not run: the ordered implementation-first gates
//! prohibit a manual verify run, overriding the conflicting audit step 6.
//! No pre-change test failure is claimed. No out-of-path wiring is required.
//! The gate runner records the verified revision and private evidence location.

use std::alloc::{GlobalAlloc, Layout, System};
use std::cell::Cell;

use layerx_programs_runtime as rt;
use rt::test_support::{
    code_section, func_body, function_section, import_section, module, raw_section, type_section,
    unsigned_leb, TYPE_I32,
};
use rt::{
    Abi, AuthorizationContext, Capability, CapabilitySet, FeeSchedule, HashAlgorithm, Meter,
    PrincipalId, ProgramId, ProgramInstance, ResourceBudget, Storage, UnavailableReceiptOracle,
    WasmEngine, WasmValue,
};

const HASH_LIMIT: i32 = 1_048_576;
const MEMORY_BYTES: i32 = 2 * 1_048_576;
const OUTPUT: i32 = MEMORY_BYTES - 32;
const STATUS_INVALID: i32 = -2;
const STATUS_BOUNDS: i32 = -3;
const STATUS_METER: i32 = -4;

thread_local! {
    static LARGEST_ALLOCATION: Cell<Option<usize>> = const { Cell::new(None) };
}

struct ObservedAllocator;

fn observe_allocation(size: usize) {
    let _ = LARGEST_ALLOCATION.try_with(|slot| {
        if let Some(largest) = slot.get() {
            slot.set(Some(largest.max(size)));
        }
    });
}

// Observe the real allocator only during a warmed guest call on this thread.
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

fn guest() -> Vec<u8> {
    let mut exports = unsigned_leb(4);
    for (name, kind, index) in [
        ("hash", 0, 2),
        ("store", 0, 3),
        ("load", 0, 4),
        ("memory", 2, 0),
    ] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.extend([kind, index]);
    }
    let mut data = vec![2, 0];
    push_i32(&mut data, 0);
    data.push(0x0b);
    data.extend(unsigned_leb(3));
    data.extend_from_slice(b"abc");
    data.push(0);
    push_i32(&mut data, OUTPUT);
    data.push(0x0b);
    data.extend(unsigned_leb(32));
    data.extend([0xa5; 32]);
    let forward = |index| {
        func_body(
            &[],
            &[0x20, 0, 0x20, 1, 0x20, 2, 0x20, 3, 0x10, index, 0x0b],
        )
    };
    module(&[
        type_section(&[(&[TYPE_I32; 4], &[TYPE_I32]), (&[TYPE_I32], &[TYPE_I32])]),
        import_section(&[("layerx_v2", "hash", 0), ("layerx_v1", "storage_write", 0)]),
        function_section(&[0, 0, 1]),
        raw_section(5, &[1, 1, 32, 32]),
        raw_section(7, &exports),
        code_section(&[
            forward(0),
            forward(1),
            func_body(&[], &[0x20, 0, 0x28, 2, 0, 0x0b]),
        ]),
        raw_section(11, &data),
    ])
}

fn instance(cpu: u64) -> ProgramInstance {
    let module = WasmEngine::declared()
        .unwrap()
        .validate_v2(&guest())
        .unwrap();
    let budget = ResourceBudget::new_complete(
        cpu,
        MEMORY_BYTES as u64,
        HASH_LIMIT as u64,
        HASH_LIMIT as u64,
        128,
        HASH_LIMIT as u64,
        4096,
    );
    let abi = Abi::new(
        rt::ABI_V2_VERSION,
        ProgramId::new([0x41; 32]).unwrap(),
        AuthorizationContext::new(
            PrincipalId::new([0x42; 32]).unwrap(),
            CapabilitySet::new([Capability::StorageWrite]).unwrap(),
        ),
        Storage::new(),
        &UnavailableReceiptOracle,
    )
    .unwrap();
    let mut instance = module
        .instantiate_sandbox(Meter::new(budget, FeeSchedule::declared()), abi)
        .unwrap();
    assert_eq!(
        call(&mut instance, "load", &[OUTPUT]),
        i32::from_le_bytes([0xa5; 4])
    );
    instance
}

fn call(instance: &mut ProgramInstance, name: &str, args: &[i32]) -> i32 {
    let args: Vec<_> = args.iter().copied().map(WasmValue::I32).collect();
    let result = instance.call(name, &args).unwrap();
    match result.as_slice() {
        [WasmValue::I32(value)] => *value,
        other => panic!("unexpected guest result: {other:?}"),
    }
}

fn observed_call(instance: &mut ProgramInstance, name: &str, args: &[i32]) -> (i32, usize, u64) {
    let before = instance.meter().cpu_total();
    LARGEST_ALLOCATION.with(|slot| slot.set(Some(0)));
    let result = call(instance, name, args);
    let largest = LARGEST_ALLOCATION.with(|slot| slot.replace(None).unwrap());
    (result, largest, instance.meter().cpu_total() - before)
}

fn assert_output(instance: &mut ProgramInstance, expected: &[u8]) {
    assert_eq!(expected.len(), 32);
    for (index, word) in expected.chunks_exact(4).enumerate() {
        assert_eq!(
            call(instance, "load", &[OUTPUT + (index as i32) * 4]),
            i32::from_le_bytes(word.try_into().unwrap()),
            "digest word {index}"
        );
    }
}

fn assert_preflight_rejection(instance: &mut ProgramInstance, args: &[i32], expected: i32) {
    let (status, largest, fuel) = observed_call(instance, "hash", args);
    assert_eq!(status, expected, "hash arguments {args:?}");
    assert!(largest < 65_536, "rejected hash allocated {largest} bytes");
    assert!(
        fuel < 1024,
        "rejected hash charged proportional fuel: {fuel}"
    );
    assert_output(instance, &[0xa5; 32]);
}

fn stored_value(instance: &ProgramInstance) -> Option<Vec<u8>> {
    let mut storage = instance.storage_snapshot().unwrap();
    let namespace = rt::StorageNamespace::principal(
        ProgramId::new([0x41; 32]).unwrap(),
        PrincipalId::new([0x42; 32]).unwrap(),
    );
    storage.transaction(namespace).read(b"a").unwrap()
}

#[test]
fn hash_preflight_oversized_valid_memory() {
    for algorithm in 1..=3 {
        for cpu in [1024, 10_000_000] {
            let mut instance = instance(cpu);
            assert_preflight_rejection(
                &mut instance,
                &[algorithm, 0, HASH_LIMIT + 1, OUTPUT],
                STATUS_BOUNDS,
            );
            assert!(instance.meter().finish().is_ok());
        }
    }
}

#[test]
fn hash_preflight_invalid_input_and_output_ranges() {
    for args in [
        [1, i32::MAX, HASH_LIMIT, OUTPUT],
        [2, MEMORY_BYTES - 16, 32, OUTPUT],
        [3, 0, HASH_LIMIT, MEMORY_BYTES - 31],
        [1, 0, HASH_LIMIT, i32::MAX],
        [2, MEMORY_BYTES + 1, 0, OUTPUT],
    ] {
        let mut instance = instance(10_000_000);
        assert_preflight_rejection(&mut instance, &args, STATUS_BOUNDS);
        assert!(instance.meter().finish().is_ok());
    }
    for args in [[1, -1, 3, OUTPUT], [2, 0, -1, OUTPUT], [3, 0, 3, -1]] {
        let mut instance = instance(10_000_000);
        assert_preflight_rejection(&mut instance, &args, STATUS_INVALID);
    }
}

#[test]
fn hash_preflight_exhausted_fuel() {
    for algorithm in 1..=3 {
        let mut instance = instance(1024);
        let (status, largest, _) =
            observed_call(&mut instance, "hash", &[algorithm, 0, HASH_LIMIT, OUTPUT]);
        assert_eq!(status, STATUS_METER);
        assert!(largest < 65_536, "unfunded hash allocated {largest} bytes");
        assert!(matches!(
            instance.meter().finish(),
            Err(rt::MeterRefusal::BudgetExceeded {
                resource: rt::ResourceKind::Cpu,
                ..
            })
        ));
    }
}

#[test]
fn hash_preflight_valid_vectors_and_ceiling() {
    for (id, algorithm, abc) in [
        (
            1,
            HashAlgorithm::Sha256,
            "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
        ),
        (
            2,
            HashAlgorithm::Keccak256,
            "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45",
        ),
        (
            3,
            HashAlgorithm::Blake3,
            "6437b3ac38465133ffb63b75273a8db548c558465d79db03fd359c6cd5bd9d85",
        ),
    ] {
        let mut instance = instance(10_000_000);
        let (status, _, empty_fuel) =
            observed_call(&mut instance, "hash", &[id, MEMORY_BYTES, 0, OUTPUT]);
        assert_eq!(status, 0);
        assert_output(&mut instance, &rt::hash_bytes(algorithm, b"").unwrap());
        let (status, _, abc_fuel) = observed_call(&mut instance, "hash", &[id, 0, 3, OUTPUT]);
        assert_eq!(status, 0);
        assert_eq!(abc_fuel - empty_fuel, 3 * algorithm.fuel_per_byte());
        assert_output(&mut instance, &hex::decode(abc).unwrap());

        let mut input = vec![0; HASH_LIMIT as usize];
        input[..3].copy_from_slice(b"abc");
        let (status, largest, fuel) =
            observed_call(&mut instance, "hash", &[id, 0, HASH_LIMIT, OUTPUT]);
        assert_eq!(status, 0);
        assert!(
            largest >= HASH_LIMIT as usize,
            "allocation observer missed the valid copy"
        );
        assert_eq!(
            fuel - empty_fuel,
            HASH_LIMIT as u64 * algorithm.fuel_per_byte()
        );
        assert_output(&mut instance, &rt::hash_bytes(algorithm, &input).unwrap());
    }
}

#[test]
fn guest_read_invalid_large_ranges_do_not_allocate() {
    for length in [262_144, HASH_LIMIT] {
        for pointer in [i32::MAX, MEMORY_BYTES - length + 1, MEMORY_BYTES] {
            let mut instance = instance(10_000_000);
            let (status, largest, _) =
                observed_call(&mut instance, "store", &[0, 1, pointer, length]);
            assert_eq!(status, STATUS_BOUNDS);
            assert!(
                largest < 65_536,
                "invalid read of {length} bytes allocated {largest}"
            );
            assert_eq!(instance.meter().finish().unwrap().storage_write_bytes, 0);
            assert_eq!(stored_value(&instance), None);
        }
    }
    for (pointer, length, expected) in [
        (-1, HASH_LIMIT, STATUS_INVALID),
        (0, -1, STATUS_INVALID),
        (0, HASH_LIMIT + 1, STATUS_BOUNDS),
        (MEMORY_BYTES + 1, 0, STATUS_BOUNDS),
    ] {
        let mut instance = instance(10_000_000);
        let (status, largest, _) = observed_call(&mut instance, "store", &[0, 1, pointer, length]);
        assert_eq!(status, expected);
        assert!(largest < 65_536);
        assert_eq!(stored_value(&instance), None);
    }
}

#[test]
fn guest_read_valid_range_still_reaches_storage() {
    let mut instance = instance(10_000_000);
    assert_eq!(call(&mut instance, "store", &[0, 1, 0, 3]), 0);
    assert_eq!(stored_value(&instance), Some(b"abc".to_vec()));
    let usage = instance.meter().finish().unwrap();
    assert!(usage.storage_write_bytes >= 3);
    assert_eq!(
        call(&mut instance, "store", &[0, 1, MEMORY_BYTES - 3, 3]),
        0
    );
    assert_eq!(stored_value(&instance), Some(vec![0xa5; 3]));
    assert_eq!(call(&mut instance, "store", &[0, 1, MEMORY_BYTES, 0]), 0);
    assert_eq!(stored_value(&instance), Some(Vec::new()));
}

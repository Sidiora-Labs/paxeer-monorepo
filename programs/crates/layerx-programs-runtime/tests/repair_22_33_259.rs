//! Repair record 22.33.259, audit 22.30.130.
//! pre-entry-capabilities: `pre_entry_start_event`, `pre_entry_start_storage`,
//! `pre_entry_reserve_event`, `pre_entry_reserve_storage` and
//! `pre_entry_start_and_reserve_union` prove descriptor binding through the native
//! byte interface and real execution, including missing-grant refusal/rollback.
//! linker-timing-assertion: `linker_strategies_preserve_complete_composition`
//! proves equivalent nine-frame executions and exact linker counts; the existing
//! validate.rs test also counts actual per-frame host registrations deterministically.
//! Wiring request: domain 13 owns programs/benches/interpreter.rs; benchmark both
//! complete linker composition strategies and report sample distributions, without
//! correctness assertions about universal wall-clock ordering.
//! Baseline verification was not run: implementation-first order and the explicit
//! prohibition on running `verify_cmd` by hand override the conflicting audit step 6.
//! The gate runner records the verified revision and private evidence location.
//! Qualification repair: canonical interface collection counts are big-endian u16.

use std::cell::RefCell;

use layerx_program_sdk::bindgen::BindingGenerator;
use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, host_linker_constructions_on_current_thread,
    import_section, module, raw_section, type_section, unsigned_leb, TYPE_I32,
};
pub use layerx_programs_runtime::{
    abi, abi_policy, dynamic_spend, validate, WasmEngine, MAX_PROGRAM_ACCOUNT_SEED_BYTES,
};
use layerx_programs_runtime::{
    AuthorizationContext, AuthorizedExecutionRecord, AuthorizedExecutionRequest, Capability,
    CapabilitySet, CompositionContext, CompositionRules, EntrypointRefusal, ExecutionError,
    ExecutionFault, Executor, FeeSchedule, PrincipalId, ProgramCatalog, ProgramId, ResourceBudget,
    Storage, StorageNamespace, UnavailableReceiptOracle, ValidatedModule, WasmValue, ABI_MODULE,
    CALL_ENTRY_EXPORT,
};
use sha2::{Digest, Sha256};

// Compile the production native descriptor consumer into this gate, which does
// not enable host-ffi or require a native host binary. Its byte source below is
// an in-memory activity store, the native interface's committed-fixture peer.
#[allow(dead_code)]
#[path = "../src/ffi_interface.rs"]
mod native_interface;

std::thread_local! {
    static ACTIVITY: RefCell<Option<(Vec<u8>, Vec<u8>)>> = const { RefCell::new(None) };
}

#[no_mangle]
extern "C" fn layerx_programs_interface_activity_byte(
    token: u64,
    section: u16,
    offset: u32,
) -> i32 {
    ACTIVITY.with(|slot| {
        let activity = slot.borrow();
        let Some((wasm, interface)) = activity.as_ref().filter(|_| token == 1) else {
            return -3;
        };
        let bytes = match section {
            0 => wasm,
            2 => interface,
            _ => return -3,
        };
        bytes
            .get(offset as usize)
            .map_or(-3, |byte| i32::from(*byte))
    })
}

fn program(byte: u8) -> ProgramId {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    ProgramId::new([byte; 32]).assert_success("test value")
}

fn principal() -> PrincipalId {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    PrincipalId::new([0x51; 32]).assert_success("test value")
}

fn exports(entries: &[(&str, u8, u8)]) -> Vec<u8> {
    let mut bytes = unsigned_leb(entries.len() as u64);
    for (name, kind, index) in entries {
        bytes.extend(unsigned_leb(name.len() as u64));
        bytes.extend_from_slice(name.as_bytes());
        bytes.extend([*kind, *index]);
    }
    raw_section(7, &bytes)
}

fn data(entries: &[(u8, &[u8])]) -> Vec<u8> {
    let mut bytes = unsigned_leb(entries.len() as u64);
    for (offset, value) in entries {
        bytes.extend([0, 0x41, *offset, 0x0b]);
        bytes.extend(unsigned_leb(value.len() as u64));
        bytes.extend_from_slice(value);
    }
    raw_section(11, &bytes)
}

#[derive(Clone, Copy, Eq, PartialEq)]
enum Effect {
    Storage,
    Event,
}

impl Effect {
    fn helper(self) -> u8 {
        match self {
            Self::Storage => 2,
            Self::Event => 3,
        }
    }

    fn mask(self) -> u16 {
        match self {
            Self::Storage => 1 << 1,
            Self::Event => 1 << 4,
        }
    }

    fn grant(self) -> Capability {
        match self {
            Self::Storage => Capability::StorageWrite,
            Self::Event => Capability::EmitEvent,
        }
    }
}

fn pre_entry_guest(start: Option<Effect>, reserve: Option<Effect>) -> Vec<u8> {
    // Both effects are indirect through a defined helper; neither callable
    // entry invokes any host import. Unreachable imports must remain excluded.
    let effect = |host| func_body(&[], &[0x41, 0, 0x41, 1, 0x41, 1, 0x41, 1, 0x10, host, 0x0b]);
    let reserve_body = reserve.map_or_else(
        || func_body(&[], &[0x41, 32, 0x0b]),
        |effect| {
            func_body(
                &[(1, TYPE_I32)],
                &[
                    0x10,
                    effect.helper(),
                    0x22,
                    1,
                    0x04,
                    TYPE_I32,
                    0x20,
                    1,
                    0x05,
                    0x41,
                    32,
                    0x0b,
                    0x0b,
                ],
            )
        },
    );
    let start_body = start.map_or_else(
        || func_body(&[], &[0x0b]),
        |effect| func_body(&[], &[0x10, effect.helper(), 0x04, 0x40, 0x00, 0x0b, 0x0b]),
    );
    let mut sections = vec![
        type_section(&[
            (&[TYPE_I32; 4], &[TYPE_I32]),
            (&[], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
            (&[], &[]),
        ]),
        import_section(&[
            (ABI_MODULE, "storage_write", 0),
            (ABI_MODULE, "event_emit", 0),
        ]),
        function_section(&[1, 1, 2, 3, 3, 4]),
        raw_section(5, &[1, 0, 1]),
        exports(&[
            ("layerx_reserve", 0, 4),
            (CALL_ENTRY_EXPORT, 0, 5),
            ("other", 0, 6),
            ("memory", 2, 0),
        ]),
    ];
    if start.is_some() {
        sections.push(raw_section(8, &[7]));
    }
    sections.push(code_section(&[
        effect(0),
        effect(1),
        reserve_body,
        func_body(&[], &[0x41, 0, 0x0b]),
        func_body(&[], &[0x41, 7, 0x0b]),
        start_body,
    ]));
    sections.push(data(&[(0, b"kv")]));
    module(&sections)
}

// A canonical, single-entry descriptor fixture. The native binder authenticates
// its code hash against the guest bytes and consumes its capability descriptors.
fn descriptor_fixture(wasm: &[u8], mask: u16) -> Vec<u8> {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut bytes = b"LayerX/program-interface/v1\0".to_vec();
    bytes.extend_from_slice(&Sha256::digest(wasm));
    bytes.extend_from_slice(&1_u16.to_be_bytes());
    bytes.extend_from_slice(&1_u16.to_be_bytes());
    bytes.extend_from_slice(
        &(u16::try_from(CALL_ENTRY_EXPORT.len()).assert_success("fixture fits u16")).to_be_bytes(),
    );
    bytes.extend_from_slice(CALL_ENTRY_EXPORT.as_bytes());
    bytes.extend([1, 2, 3, 4]);
    bytes.extend([1, 0x10, 1, 0x10]); // LayerX u8 input and output schemas.
    bytes.extend_from_slice(
        &(u16::try_from(mask.count_ones()).assert_success("fixture fits u16")).to_be_bytes(),
    );
    for bit in 0..12 {
        if mask & (1 << bit) != 0 {
            bytes.push(bit);
        }
    }
    bytes.extend_from_slice(&0_u16.to_be_bytes()); // Event topics.
    bytes.extend_from_slice(&0_u16.to_be_bytes()); // Typed failures.
    bytes
}

fn bind_descriptor(wasm: &[u8], descriptor: &[u8]) -> i32 {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let hash: [u8; 32] = Sha256::digest(wasm).into();
    let words: Vec<_> = hash
        .chunks_exact(8)
        .map(|word| u64::from_be_bytes(word.try_into().assert_success("test value")))
        .collect();
    ACTIVITY.with(|slot| *slot.borrow_mut() = Some((wasm.to_vec(), descriptor.to_vec())));
    let result = native_interface::layerx_programs_interface_validate(
        1,
        u32::try_from(wasm.len()).assert_success("fixture fits u32"),
        u32::try_from(descriptor.len()).assert_success("fixture fits u32"),
        0,
        1,
        0,
        words[0],
        words[1],
        words[2],
        words[3],
    );
    ACTIVITY.with(|slot| *slot.borrow_mut() = None);
    result
}

fn execute(
    module: &ValidatedModule,
    storage: &mut Storage,
    grants: CapabilitySet,
    input: &[u8],
    entry: &str,
    composition: CompositionContext,
) -> Result<AuthorizedExecutionRecord, ExecutionError> {
    Executor::new(
        ResourceBudget::new(
            5_000_000,
            16 * 1_024 * 1_024,
            1_048_576,
            1_048_576,
            64,
            4096,
        ),
        FeeSchedule::declared(),
    )
    .execute_authorized(
        storage,
        AuthorizedExecutionRequest {
            module,
            program: program(1),
            authorization: AuthorizationContext::new(principal(), grants),
            receipts: &UnavailableReceiptOracle,
            entrypoint: entry,
            calldata: input,
            composition,
            response_capacity: 0,
        },
    )
}

fn prove_pre_entry(start: Option<Effect>, reserve: Option<Effect>) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let wasm = pre_entry_guest(start, reserve);
    let engine = WasmEngine::declared().assert_success("test value");
    let module = engine.validate(&wasm).assert_success("test value");
    let required = start
        .into_iter()
        .chain(reserve)
        .fold(0, |mask, effect| mask | effect.mask());
    assert_pre_entry_descriptor(&wasm, &module, required);
    let grants = CapabilitySet::new(start.into_iter().chain(reserve).map(Effect::grant))
        .assert_success("test value");
    let mut storage = Storage::new();
    let record = execute(
        &module,
        &mut storage,
        grants.clone(),
        &[1, 2, 3, 4, 1, 0x10, 9],
        CALL_ENTRY_EXPORT,
        CompositionContext::isolated(),
    )
    .assert_success("test value");
    assert_eq!(record.execution.outputs, vec![WasmValue::I32(0)]);
    let events =
        usize::from(start == Some(Effect::Event)) + usize::from(reserve == Some(Effect::Event));
    assert_eq!(record.effects.events.len(), events);
    for event in &record.effects.events {
        assert_eq!(event.topic, b"k");
        assert_eq!(event.data, b"v");
        assert_eq!(event.program, program(1));
        assert_eq!(event.principal, principal());
    }
    let namespace = StorageNamespace::principal(program(1), principal());
    assert_eq!(
        storage
            .transaction(namespace)
            .read(b"k")
            .assert_success("test value"),
        (required & Effect::Storage.mask() != 0).then(|| b"v".to_vec())
    );
    for missing in start.into_iter().chain(reserve) {
        let reduced = CapabilitySet::new(
            start
                .into_iter()
                .chain(reserve)
                .filter(|effect| *effect != missing)
                .map(Effect::grant),
        )
        .assert_success("test value");
        let mut denied = Storage::new();
        let before = denied.clone();
        let Err(error) = execute(
            &module,
            &mut denied,
            reduced,
            &[1],
            CALL_ENTRY_EXPORT,
            CompositionContext::isolated(),
        ) else {
            panic!("missing capability must refuse pre-entry execution");
        };
        if start == Some(missing) {
            assert_eq!(
                error,
                ExecutionError::Fault(ExecutionFault::UnreachableExecuted)
            );
        } else {
            assert_eq!(
                error,
                ExecutionError::Entrypoint(EntrypointRefusal::AllocationRefused { code: -1 })
            );
        }
        assert_eq!(denied, before);
        assert_eq!(
            bind_descriptor(
                &wasm,
                &descriptor_fixture(&wasm, required & !missing.mask())
            ),
            -3
        );
    }
    let mut other_storage = Storage::new();
    let other = execute(
        &module,
        &mut other_storage,
        grants,
        &[1],
        "other",
        CompositionContext::isolated(),
    )
    .assert_success("test value");
    assert_eq!(other.execution.outputs, vec![WasmValue::I32(7)]);
    assert_eq!(other.effects.events.len(), events);
}
fn assert_pre_entry_descriptor(
    wasm: &[u8],
    module: &layerx_programs_runtime::ValidatedModule,
    required: u16,
) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    for entry in [CALL_ENTRY_EXPORT, "other"] {
        assert!(module.supports_interface_entrypoint(entry));
        assert_eq!(
            module.required_interface_capability_mask(entry),
            Some(required)
        );
        assert!(module.interface_capability_mask_matches(entry, required));
    }
    assert_eq!(module.required_interface_capability_mask("absent"), None);
    let descriptor = descriptor_fixture(wasm, required);
    let bindings = BindingGenerator::from_interface(&descriptor).assert_success("test value");
    bindings
        .require_code_hash(Sha256::digest(wasm).into())
        .assert_success("test value");
    assert_eq!(bind_descriptor(wasm, &descriptor), 0);
    assert_eq!(bind_descriptor(wasm, &descriptor_fixture(wasm, 0)), -3);
    assert_eq!(
        bind_descriptor(wasm, &descriptor_fixture(wasm, required | 1)),
        -3
    );
    let mut altered = wasm.to_vec();
    altered.extend(raw_section(0, &[1, b'x']));
    assert_eq!(bind_descriptor(&altered, &descriptor), -213);
}

#[test]
fn pre_entry_start_event() {
    prove_pre_entry(Some(Effect::Event), None);
}

#[test]
fn pre_entry_start_storage() {
    prove_pre_entry(Some(Effect::Storage), None);
}

#[test]
fn pre_entry_reserve_event() {
    prove_pre_entry(None, Some(Effect::Event));
}

#[test]
fn pre_entry_reserve_storage() {
    prove_pre_entry(None, Some(Effect::Storage));
}

#[test]
fn pre_entry_start_and_reserve_union() {
    prove_pre_entry(Some(Effect::Storage), Some(Effect::Event));
}

#[test]
fn pre_entry_empty_input_skips_reserve_effects() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let engine = WasmEngine::declared().assert_success("test value");
    let module = engine
        .validate(&pre_entry_guest(None, Some(Effect::Event)))
        .assert_success("test value");
    assert_eq!(
        module.required_interface_capability_mask(CALL_ENTRY_EXPORT),
        Some(1 << 4)
    );
    let record = execute(
        &module,
        &mut Storage::new(),
        CapabilitySet::empty(),
        &[],
        CALL_ENTRY_EXPORT,
        CompositionContext::isolated(),
    )
    .assert_success("test value");
    assert_eq!(record.execution.outputs, vec![WasmValue::I32(0)]);
    assert!(record.effects.events.is_empty());
}

fn call_grants(ids: &[ProgramId]) -> CapabilitySet {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    CapabilitySet::new(ids.iter().map(|id| Capability::Call { program: *id }))
        .assert_success("test value")
}

fn push_i32(body: &mut Vec<u8>, value: i32) {
    body.push(0x41);
    let mut value = i64::from(value);
    loop {
        let byte = value.to_le_bytes()[0] & 0x7f;
        value >>= 7;
        let done = (value == 0 && byte & 0x40 == 0) || (value == -1 && byte & 0x40 != 0);
        body.push(if done { byte } else { byte | 0x80 });
        if done {
            break;
        }
    }
}

fn composition_frame(callee: Option<ProgramId>, requested: &CapabilitySet) -> Vec<u8> {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let Some(callee) = callee else {
        return module(&[
            type_section(&[(&[TYPE_I32; 2], &[TYPE_I32])]),
            function_section(&[0]),
            exports(&[(CALL_ENTRY_EXPORT, 0, 0)]),
            code_section(&[func_body(&[], &[0x41, 0, 0x0b])]),
        ]);
    };
    let encoding = requested.canonical_encoding();
    let mut body = Vec::new();
    for value in [
        0,
        32,
        32,
        0,
        32,
        i32::try_from(encoding.len()).assert_success("fixture fits i32"),
    ] {
        push_i32(&mut body, value);
    }
    body.extend([0x10, 0, 0x0b]);
    module(&[
        type_section(&[(&[TYPE_I32; 6], &[TYPE_I32]), (&[TYPE_I32; 2], &[TYPE_I32])]),
        import_section(&[(ABI_MODULE, "program_call", 0)]),
        function_section(&[1]),
        raw_section(5, &[1, 0, 1]),
        exports(&[(CALL_ENTRY_EXPORT, 0, 1), ("memory", 2, 0)]),
        code_section(&[func_body(&[], &body)]),
        data(&[(0, &callee.bytes()), (32, &encoding)]),
    ])
}

fn complete_composition(separate_linkers: bool) -> (AuthorizedExecutionRecord, usize) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let before = host_linker_constructions_on_current_thread();
    let ids: Vec<_> = (1..=9).map(program).collect();
    let frames: Vec<_> = (0..ids.len())
        .map(|i| {
            composition_frame(
                ids.get(i + 1).copied(),
                &call_grants(&ids[(i + 2).min(ids.len())..]),
            )
        })
        .collect();
    let engine = WasmEngine::declared().assert_success("test value");
    let root = engine.validate(&frames[0]).assert_success("test value");
    let mut catalog = ProgramCatalog::new();
    let mut constructions = engine.host_linker_construction_count();
    for (id, wasm) in ids[1..].iter().zip(&frames[1..]) {
        let child = if separate_linkers {
            let independent = WasmEngine::declared().assert_success("test value");
            constructions += independent.host_linker_construction_count();
            independent.validate(wasm).assert_success("test value")
        } else {
            engine.validate(wasm).assert_success("test value")
        };
        assert_eq!(child.host_linker_construction_count(), 1);
        assert_eq!(
            child.host_function_registration_count(),
            root.host_function_registration_count()
        );
        assert!(catalog.insert(*id, child).is_none());
    }
    let mut storage = Storage::new();
    let record = execute(
        &root,
        &mut storage,
        call_grants(&ids[1..]),
        &[],
        CALL_ENTRY_EXPORT,
        CompositionContext::catalog(catalog, CompositionRules::declared()),
    )
    .assert_success("test value");
    assert_eq!(storage, Storage::new());
    assert_eq!(engine.host_linker_construction_count(), 1);
    assert_eq!(
        host_linker_constructions_on_current_thread() - before,
        constructions
    );
    (record, constructions)
}

#[test]
fn linker_strategies_preserve_complete_composition() {
    let (shared, shared_count) = complete_composition(false);
    let (rebuilt, rebuilt_count) = complete_composition(true);
    assert_eq!(shared.execution.outputs, vec![WasmValue::I32(0)]);
    assert_eq!(shared.call_graph.edges().len(), 8);
    assert_eq!(shared_count, 1);
    assert_eq!(rebuilt_count, 9);
    assert_eq!(shared, rebuilt);
    assert_eq!(complete_composition(false).0, shared);
}

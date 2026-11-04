use layerx_program_sdk::{BindingGenerator, CallInput};
use layerx_programs::{
    InterfaceCapability, InterfaceEntryPoint, ProgramInterface, TypedFailure, ValueSchema,
    ValueType,
};
use layerx_programs_runtime::abi::codec::Calldata;
use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, import_section, module, raw_section, type_section,
    unsigned_leb, TYPE_I32,
};
use layerx_programs_runtime::{
    AuthorizationContext, AuthorizedExecutionRequest, Capability, CapabilitySet,
    CompositionContext, CompositionRules, Executor, FeeSchedule, PrincipalId, ProgramCatalog,
    ProgramId, ResourceBudget, Storage, StorageNamespace, UnavailableReceiptOracle, WasmEngine,
    WasmValue, CALL_ENTRY_EXPORT,
};

#[allow(dead_code)]
mod client {
    include!("../fixtures/repair_22_33_242/client.inc");
}

const LIMIT: usize = 1_048_576;
const MAX_PAYLOAD: usize = LIMIT - 10;
const SCHEMA_MAX: u32 = 1_048_570;
const INTERFACE: &[u8] = include_bytes!("../fixtures/repair_22_33_242/bytes.interface");
const SINK: &[u8] = include_bytes!("../fixtures/repair_22_33_242/sink.wasm");

fn generator() -> BindingGenerator {
    let schema = || {
        ValueSchema::layerx(ValueType::Bytes {
            max_len: SCHEMA_MAX,
        })
    };
    let published = ProgramInterface::bind(
        SINK,
        1,
        vec![InterfaceEntryPoint {
            name: "blob".to_owned(),
            discriminator: [1, 2, 3, 4],
            calldata: schema(),
            response: schema(),
            capabilities: vec![InterfaceCapability::StorageWrite],
            event_topics: vec![],
            failures: vec![TypedFailure {
                code: 7,
                name: "refused".to_owned(),
                detail: schema(),
            }],
        }],
    )
    .unwrap_or_else(|error| panic!("registry Bytes interface: {error}"));
    assert_eq!(published.canonical_encoding(), INTERFACE);
    let generator = BindingGenerator::from_interface(published.canonical_encoding())
        .unwrap_or_else(|error| panic!("published interface consumer: {error}"));
    assert_eq!(generator.code_hash(), client::CODE_HASH);
    assert_eq!(generator.interface_digest(), client::INTERFACE_DIGEST);
    assert_eq!(
        generator.generate_rust(),
        include_str!("../fixtures/repair_22_33_242/client.inc"),
        "the consumer compiled by Cargo must be the production generator's complete output"
    );
    generator
}

fn input(length: usize) -> client::blob::Input {
    client::blob::Input::new((0..length).map(|index| index.to_le_bytes()[0]).collect())
        .unwrap_or_else(|error| panic!("schema-admissible Bytes value: {error:?}"))
}

fn forwarder(child: ProgramId) -> Vec<u8> {
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
    let mut data = vec![1, 0, 0x41, 0, 0x0b, 35];
    data.extend_from_slice(&child.bytes());
    // The production capability encoding for one StorageWrite grant.
    data.extend([0, 1, 2]);
    module(&[
        type_section(&[
            (&[TYPE_I32; 6], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
        ]),
        import_section(&[("layerx_v1", "program_call", 0)]),
        function_section(&[1, 2]),
        raw_section(5, &[1, 1, 17, 17]),
        raw_section(7, &exports),
        code_section(&[
            func_body(&[], &[0x41, 0x80, 0x08, 0x0b]),
            func_body(
                &[],
                &[
                    0x41, 0, 0x41, 32, 0x20, 0, 0x20, 1, 0x41, 32, 0x41, 3, 0x10, 0, 0x0b,
                ],
            ),
        ]),
        raw_section(11, &data),
    ])
}

#[test]
fn finding_010_exact_limit_generated_call_survives_runtime_transport() {
    let _generator = generator();
    assert_eq!(client::MAX_CALLDATA_BYTES, LIMIT);
    assert_eq!(layerx_programs_runtime::abi::MAX_CALL_INPUT_BYTES, LIMIT);
    let value = input(MAX_PAYLOAD);
    let call = client::blob::call(&value, client::CODE_HASH, client::INTERFACE_DIGEST)
        .unwrap_or_else(|error| panic!("exact-limit call construction: {error:?}"));
    let bytes = call.as_bytes();
    assert_eq!(bytes.len(), LIMIT);
    assert_eq!(&bytes[..6], &[1, 2, 3, 4, 1, 0x20]);
    assert_eq!(&bytes[6..10], &(MAX_PAYLOAD as u32).to_be_bytes());
    assert_eq!(&bytes[10..], value.as_slice());
    assert_eq!(CallInput::new(bytes).map(CallInput::bytes), Ok(bytes));
    let frame = Calldata::from_bytes(&bytes[4..])
        .unwrap_or_else(|error| panic!("runtime canonical codec: {error}"));
    assert_eq!(frame.as_bytes(), &bytes[4..]);

    let engine = WasmEngine::declared().unwrap_or_else(|error| panic!("engine: {error}"));
    let child = ProgramId::new([3; 32]).unwrap_or_else(|error| panic!("child: {error}"));
    let root = ProgramId::new([4; 32]).unwrap_or_else(|error| panic!("root: {error}"));
    let principal = PrincipalId::new([2; 32]).unwrap_or_else(|error| panic!("principal: {error}"));
    let sink = engine
        .validate(SINK)
        .unwrap_or_else(|error| panic!("sink: {error}"));
    let mut catalog = ProgramCatalog::new();
    assert!(catalog.insert(child, sink).is_none());
    let forwarder = forwarder(child);
    let forwarder = engine
        .validate(&forwarder)
        .unwrap_or_else(|error| panic!("forwarder: {error}"));
    let capabilities = CapabilitySet::new([
        Capability::Call { program: child },
        Capability::StorageWrite,
    ])
    .unwrap_or_else(|error| panic!("explicit call capabilities: {error}"));
    let mut storage = Storage::new();
    let record = Executor::new(
        ResourceBudget::new(16_777_216, 16_777_216, 2_097_152, 2_097_152, 64, 4_096),
        FeeSchedule::declared(),
    )
    .execute_authorized(
        &mut storage,
        AuthorizedExecutionRequest {
            module: &forwarder,
            program: root,
            authorization: AuthorizationContext::new(principal, capabilities),
            receipts: &UnavailableReceiptOracle,
            entrypoint: CALL_ENTRY_EXPORT,
            calldata: bytes,
            composition: CompositionContext::catalog(catalog, CompositionRules::declared()),
            response_capacity: 0,
        },
    )
    .unwrap_or_else(|error| panic!("exact-limit program_call transport: {error}"));
    assert_eq!(record.execution.outputs, vec![WasmValue::I32(0)]);
    assert_eq!(record.call_graph.edges().len(), 1);
    assert_eq!(record.call_graph.edges()[0].caller(), root);
    assert_eq!(record.call_graph.edges()[0].callee(), child);
    let received = storage
        .transaction(StorageNamespace::principal(child, principal))
        .read(b"key")
        .unwrap_or_else(|error| panic!("real sink storage: {error}"))
        .unwrap_or_else(|| panic!("callee did not store the transported call"));
    assert_eq!(received, bytes, "the actual callee must receive every byte");
    let decoded = client::blob::decode_output(&received[4..])
        .unwrap_or_else(|error| panic!("compiled generated consumer: {error:?}"));
    assert_eq!(decoded.as_slice(), value.as_slice());
}

#[test]
fn finding_010_one_byte_over_and_selector_overflow_refused_at_construction() {
    let _generator = generator();
    for extra in 1..=4 {
        // These values satisfy the schema, and the frame alone fits the codec.
        // Only the complete selector-plus-frame call exceeds the transport bound.
        let value = input(MAX_PAYLOAD + extra);
        let mut frame = vec![1];
        client::CanonicalEncode::encode(&value, &mut frame)
            .unwrap_or_else(|error| panic!("valid standalone frame: {error:?}"));
        assert_eq!(frame.len(), LIMIT - 4 + extra);
        assert!(Calldata::from_bytes(&frame).is_ok());
        assert_eq!(
            client::blob::decode_output(&frame)
                .unwrap_or_else(|error| panic!("standalone generated decode: {error:?}"))
                .as_slice(),
            value.as_slice()
        );
        assert_eq!(
            client::blob::call(&value, client::CODE_HASH, client::INTERFACE_DIGEST),
            Err(client::BindingRefusal::Encode(client::CodecError::TooLong)),
            "selector-plus-frame overflow by {extra} bytes must fail in the builder"
        );
    }
}

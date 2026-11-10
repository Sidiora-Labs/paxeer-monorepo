//! Deterministic WASM binary construction for validation and execution tests.

/// Returns every ABI version admitted by the production policy.
#[must_use]
pub fn supported_abi_versions() -> Vec<u16> {
    (0..=u16::MAX)
        .filter(|version| crate::admit_abi_version(*version).is_ok())
        .collect()
}

/// An explicit success assertion shared by runtime qualification fixtures.
/// Both implementations fail immediately on the same cases as the original checks.
pub trait AssertSuccess {
    type Value;

    /// Extracts the asserted successful value.
    ///
    /// # Panics
    /// Panics when a result is an error or an option is absent.
    #[track_caller]
    fn assert_success(self, message: &str) -> Self::Value;
}

impl<T, E: core::fmt::Debug> AssertSuccess for Result<T, E> {
    type Value = T;

    #[track_caller]
    fn assert_success(self, message: &str) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("{message}: {error:?}"),
        }
    }
}

impl<T> AssertSuccess for Option<T> {
    type Value = T;

    #[track_caller]
    fn assert_success(self, message: &str) -> T {
        if let Some(value) = self {
            value
        } else {
            panic!("{message}: absent value")
        }
    }
}

/// Counts actual host-linker registrations on this test thread.
#[must_use]
pub fn host_linker_constructions_on_current_thread() -> usize {
    crate::host::linker_constructions_on_current_thread()
}

/// The `i32` value type code.
pub const TYPE_I32: u8 = 0x7f;
/// The `i64` value type code.
pub const TYPE_I64: u8 = 0x7e;
/// The 32-bit float value type code.
pub const TYPE_F32: u8 = 0x7d;
/// The 64-bit float value type code.
pub const TYPE_F64: u8 = 0x7c;
/// The `v128` value type code.
pub const TYPE_V128: u8 = 0x7b;

/// The `end` opcode.
pub const OP_END: u8 = 0x0b;
/// The `call` opcode.
pub const OP_CALL: u8 = 0x10;
/// The `drop` opcode.
pub const OP_DROP: u8 = 0x1a;
/// The `local.get` opcode.
pub const OP_LOCAL_GET: u8 = 0x20;
/// The `i32.const` opcode.
pub const OP_I32_CONST: u8 = 0x41;
/// The 32-bit float constant opcode.
pub const OP_F32_CONST: u8 = 0x43;
/// The `i32.add` opcode.
pub const OP_I32_ADD: u8 = 0x6a;
/// The `i32.div_s` opcode.
pub const OP_I32_DIV_S: u8 = 0x6d;

const SECTION_TYPE: u8 = 1;
const SECTION_IMPORT: u8 = 2;
const SECTION_FUNCTION: u8 = 3;
const SECTION_EXPORT: u8 = 7;
const SECTION_CODE: u8 = 10;
const SECTION_CUSTOM: u8 = 0;
const FUNC_TYPE: u8 = 0x60;
const KIND_FUNC: u8 = 0x00;

/// Encodes an unsigned integer as LEB128.
#[must_use]
pub fn unsigned_leb(mut value: u64) -> Vec<u8> {
    let mut bytes = Vec::new();
    loop {
        let byte = u8::try_from(value & 0x7f).unwrap_or(0);
        value >>= 7;
        if value == 0 {
            bytes.push(byte);
            return bytes;
        }
        bytes.push(byte | 0x80);
    }
}

fn name(text: &str) -> Vec<u8> {
    let mut bytes = unsigned_leb(text.len() as u64);
    bytes.extend_from_slice(text.as_bytes());
    bytes
}

fn section(id: u8, payload: &[u8]) -> Vec<u8> {
    let mut bytes = vec![id];
    bytes.extend(unsigned_leb(payload.len() as u64));
    bytes.extend_from_slice(payload);
    bytes
}

/// Encodes a section from its numeric identifier and already assembled payload.
///
/// This is intentionally low-level so committed execution corpora can exercise
/// proposal instructions without adding production dependencies on a WAT parser.
#[must_use]
pub fn raw_section(id: u8, payload: &[u8]) -> Vec<u8> {
    section(id, payload)
}

/// Encodes a type section from `(params, results)` value type code pairs.
#[must_use]
pub fn type_section(types: &[(&[u8], &[u8])]) -> Vec<u8> {
    let mut payload = unsigned_leb(types.len() as u64);
    for (params, results) in types {
        payload.push(FUNC_TYPE);
        payload.extend(unsigned_leb(params.len() as u64));
        payload.extend_from_slice(params);
        payload.extend(unsigned_leb(results.len() as u64));
        payload.extend_from_slice(results);
    }
    section(SECTION_TYPE, &payload)
}

/// Encodes an import section of function imports as `(module, name, type index)`.
#[must_use]
pub fn import_section(imports: &[(&str, &str, u32)]) -> Vec<u8> {
    let mut payload = unsigned_leb(imports.len() as u64);
    for (import_module, import_name, type_index) in imports {
        payload.extend(name(import_module));
        payload.extend(name(import_name));
        payload.push(KIND_FUNC);
        payload.extend(unsigned_leb(u64::from(*type_index)));
    }
    section(SECTION_IMPORT, &payload)
}

/// Encodes a function section from type indices.
#[must_use]
pub fn function_section(type_indices: &[u32]) -> Vec<u8> {
    let mut payload = unsigned_leb(type_indices.len() as u64);
    for type_index in type_indices {
        payload.extend(unsigned_leb(u64::from(*type_index)));
    }
    section(SECTION_FUNCTION, &payload)
}

/// Encodes an export section of function exports as `(name, function index)`.
#[must_use]
pub fn export_section(exports: &[(&str, u32)]) -> Vec<u8> {
    let mut payload = unsigned_leb(exports.len() as u64);
    for (export_name, function_index) in exports {
        payload.extend(name(export_name));
        payload.push(KIND_FUNC);
        payload.extend(unsigned_leb(u64::from(*function_index)));
    }
    section(SECTION_EXPORT, &payload)
}

/// Encodes one function body from local declarations and raw instructions.
#[must_use]
pub fn func_body(locals: &[(u32, u8)], instructions: &[u8]) -> Vec<u8> {
    let mut body = unsigned_leb(locals.len() as u64);
    for (count, value_type) in locals {
        body.extend(unsigned_leb(u64::from(*count)));
        body.push(*value_type);
    }
    body.extend_from_slice(instructions);
    body
}

/// Encodes a code section from assembled function bodies.
#[must_use]
pub fn code_section(bodies: &[Vec<u8>]) -> Vec<u8> {
    let mut payload = unsigned_leb(bodies.len() as u64);
    for body in bodies {
        payload.extend(unsigned_leb(body.len() as u64));
        payload.extend_from_slice(body);
    }
    section(SECTION_CODE, &payload)
}

/// Encodes a custom section padded with zero bytes to the given length.
#[must_use]
pub fn padding_section(padding: usize) -> Vec<u8> {
    let mut payload = name("padding");
    payload.extend(core::iter::repeat_n(0u8, padding));
    section(SECTION_CUSTOM, &payload)
}

/// Assembles a module from encoded sections.
#[must_use]
pub fn module(sections: &[Vec<u8>]) -> Vec<u8> {
    let mut bytes = vec![0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00];
    for encoded in sections {
        bytes.extend_from_slice(encoded);
    }
    bytes
}

/// Builds a module exporting `add`, summing two `i32` parameters.
#[must_use]
pub fn add_module() -> Vec<u8> {
    module(&[
        type_section(&[(&[TYPE_I32, TYPE_I32], &[TYPE_I32])]),
        function_section(&[0]),
        export_section(&[("add", 0)]),
        code_section(&[func_body(
            &[],
            &[OP_LOCAL_GET, 0, OP_LOCAL_GET, 1, OP_I32_ADD, OP_END],
        )]),
    ])
}

/// Invokes a real composed guest through its versioned host linker without publishing effects.
///
/// # Errors
///
/// Returns the actual ABI, entrypoint, instantiation or nested-call refusal.
pub fn invoke_composed_for_qualification(
    storage: &crate::Storage,
    request: crate::AuthorizedExecutionRequest<'_>,
) -> Result<i32, crate::ExecutionError> {
    use crate::{abi::Abi, calls::Composition, meter::Meter, ExecutionError};

    crate::entrypoint::preflight(request.calldata).map_err(ExecutionError::Entrypoint)?;
    let revision = request.module.abi_revision();
    let principal = request.authorization.principal();
    let abi = Abi::new(
        crate::abi_policy::abi_version(revision),
        request.program,
        request.authorization,
        storage.clone(),
        request.receipts,
    )
    .map_err(ExecutionError::Abi)?;
    let composition = Composition::new(
        request
            .composition
            .claim_resolver(None)
            .map_err(ExecutionError::Composition)?,
        crate::CallGraph::root(request.composition.rules(), request.program, principal),
        revision,
    );
    let meter = Meter::new(
        crate::ResourceBudget::declared(),
        crate::FeeSchedule::declared(),
    );
    let mut instance = if revision == crate::AbiRevision::V1 {
        request
            .module
            .instantiate_composed(meter, abi, composition)
            .map_err(|(fault, _)| ExecutionError::Fault(fault))?
    } else {
        request
            .module
            .instantiate_composed_response_context_retained(
                meter,
                abi,
                composition,
                request.response_capacity,
                None,
            )
            .map_err(ExecutionError::Response)?
            .map_err(|failure| ExecutionError::Fault(failure.0))?
    };
    let outcome = crate::entrypoint::invoke(&mut instance, request.entrypoint, request.calldata);
    if let Some(refusal) = instance.state().refusal() {
        return Err(ExecutionError::Composition(refusal.clone()));
    }
    outcome.map_err(ExecutionError::Entrypoint)
}

/// Executes a validated revision through the production executor with recorded fees.
///
/// # Errors
/// Returns the actual execution refusal without fabricating evidence.
pub fn execute_versioned_for_qualification(
    module: &crate::ValidatedModule,
    prices: crate::FeeSchedule,
    export: &str,
    args: &[crate::WasmValue],
) -> Result<crate::ExecutionRecord, crate::ExecutionError> {
    crate::Executor::new_versioned(
        crate::ResourceBudget::declared(),
        prices,
        crate::RUNTIME_VERSION,
        crate::abi_policy::abi_version(module.abi_revision()),
    )
    .execute(module, export, args)
}

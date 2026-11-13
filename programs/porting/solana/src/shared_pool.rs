//! Reference port demonstrating shared state: a Solana program-owned pool.
//!
//! The program tracks a pool reserve that every participant can read and
//! contribute to through the executable [`SharedPoolPort`]. Deposits stage both
//! account writes and an authorized 402LXP request in one runtime transaction.
//! The enclosing kernel activity settles that request before publishing its
//! storage candidate, just as for the mint reference. The account helpers below
//! describe the byte-preserving schema; they do not execute a deposit themselves.

use layerx_programs_runtime::abi::manifest::ABI_V2_MODULE;
use layerx_programs_runtime::{
    AbiError, AuthorizedExecutionRequest, Capability, CapabilitySet, CompositionRefusal,
    ExecutionError, Executor, Storage, V2AuthorizedExecutionRecord, ValidatedModule, WasmEngine,
    ABI_MODULE,
};

use crate::account::{AccountMapping, AccountRole, AccountSchema, Field, FieldType, FieldValue};
use crate::anchor::{account_discriminator, instruction_discriminator, InstructionAbi};
use crate::error::PortRefusal;
use crate::monetary::Transfer402Plan;
use crate::pubkey::{Pubkey, SeedPath, PUBKEY_BYTES};
use crate::wasm::{
    Code, ModuleBuilder, ELSE, I32, I32_ADD, I32_EQZ, I32_LOAD, I32_LT_S, I32_NE, I32_STORE, I64,
    I64_ADD, I64_EQ, I64_EQZ, I64_GT_U, I64_LOAD, I64_NE, I64_STORE, IF, RETURN, VOID_BLOCK,
};

/// Seed positions supplied by the authenticated `LayerX` invocation envelope.
pub const PARTICIPANT_ENVELOPE_SEEDS: [usize; 1] = [2];

/// Schema for the shared reserve account; execution lives in [`SharedPoolPort`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PoolReserve;

impl PoolReserve {
    /// Returns the account schema for the program-owned pool.
    ///
    /// # Errors
    ///
    /// Refuses schema construction failures.
    pub fn schema() -> Result<AccountSchema, PortRefusal> {
        AccountSchema::new(
            "PoolReserve",
            vec![
                Field {
                    name: "total_deposited".to_owned(),
                    kind: FieldType::U64,
                },
                Field {
                    name: "participant_count".to_owned(),
                    kind: FieldType::U32,
                },
                Field {
                    name: "pool_authority".to_owned(),
                    kind: FieldType::Pubkey,
                },
            ],
        )
    }

    /// Returns the seed path for the pool account.
    ///
    /// # Errors
    ///
    /// Refuses seed paths the runtime cannot admit.
    pub fn seeds() -> Result<SeedPath, PortRefusal> {
        SeedPath::new(vec![b"pool".to_vec(), b"reserve".to_vec(), vec![0u8]])
    }

    /// Returns the role of the pool account in a deposit instruction.
    #[must_use]
    pub fn role() -> AccountRole {
        AccountRole::ProgramOwnedShared
    }

    /// Translates the pool account into its `LayerX` form.
    ///
    /// # Errors
    ///
    /// Refuses translation failures.
    pub fn translate() -> Result<AccountMapping, PortRefusal> {
        Self::role().translate()
    }

    /// Returns the storage key the pool reserve occupies in shared namespace.
    ///
    /// # Errors
    ///
    /// Refuses key derivation failures.
    pub fn storage_key() -> Result<Vec<u8>, PortRefusal> {
        // No envelope seeds need to be dropped since this is program-owned
        Self::seeds()?.collapse(&[])?.storage_key()
    }

    /// Encodes initial pool state.
    ///
    /// # Errors
    ///
    /// Refuses encoding failures.
    pub fn encode_initial(authority: [u8; PUBKEY_BYTES]) -> Result<Vec<u8>, PortRefusal> {
        let schema = Self::schema()?;
        schema.encode(&[
            FieldValue::U64(0),            // total_deposited
            FieldValue::U32(0),            // participant_count
            FieldValue::Pubkey(authority), // pool_authority
        ])
    }

    /// Decodes pool state from storage.
    ///
    /// # Errors
    ///
    /// Refuses decoding failures.
    pub fn decode(data: &[u8]) -> Result<(u64, u32, [u8; PUBKEY_BYTES]), PortRefusal> {
        let schema = Self::schema()?;
        let values = schema.decode(data)?;
        match &values[..] {
            [FieldValue::U64(total), FieldValue::U32(count), FieldValue::Pubkey(auth)] => {
                Ok((*total, *count, *auth))
            }
            _ => Err(PortRefusal::SchemaMismatch),
        }
    }
}

/// Schema for a principal-scoped participant record.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ParticipantDeposit;

impl ParticipantDeposit {
    /// Returns the account schema for per-participant deposits.
    ///
    /// # Errors
    ///
    /// Refuses schema construction failures.
    pub fn schema() -> Result<AccountSchema, PortRefusal> {
        AccountSchema::new(
            "ParticipantDeposit",
            vec![
                Field {
                    name: "amount".to_owned(),
                    kind: FieldType::U64,
                },
                Field {
                    name: "deposit_count".to_owned(),
                    kind: FieldType::U32,
                },
            ],
        )
    }

    /// Returns the seed path for a participant's deposit record.
    ///
    /// The signer's pubkey seed will collapse, making this principal-scoped.
    ///
    /// # Errors
    ///
    /// Refuses seed paths the runtime cannot admit.
    pub fn seeds(signer: Pubkey) -> Result<SeedPath, PortRefusal> {
        SeedPath::new(vec![
            b"deposit".to_vec(),
            b"participant".to_vec(),
            signer.bytes().to_vec(),
        ])
    }

    /// Returns the role of the participant deposit account.
    #[must_use]
    pub fn role() -> AccountRole {
        AccountRole::ProgramState
    }

    /// Returns the storage key with the signer seed collapsed.
    ///
    /// # Errors
    ///
    /// Refuses key derivation failures.
    pub fn storage_key() -> Result<Vec<u8>, PortRefusal> {
        SeedPath::new(vec![b"deposit".to_vec(), b"participant".to_vec()])?.storage_key()
    }

    /// Decodes one participant's byte-preserving account data.
    ///
    /// # Errors
    /// Refuses a discriminator or length that differs from the schema.
    pub fn decode(data: &[u8]) -> Result<(u64, u32), PortRefusal> {
        match &Self::schema()?.decode(data)?[..] {
            [FieldValue::U64(amount), FieldValue::U32(count)] => Ok((*amount, *count)),
            _ => Err(PortRefusal::SchemaMismatch),
        }
    }
}

const RESERVE_KEY: u32 = 64;
const PARTICIPANT_KEY: u32 = 128;
const ASSET: u32 = 192;
const DESTINATION: u32 = 224;
const RESERVE: u32 = 256;
const RESERVE_BYTES: i32 = 52;
const PARTICIPANT: u32 = RESERVE + RESERVE_BYTES as u32;
const PARTICIPANT_BYTES: i32 = 20;
const INPUT: u32 = 1024;
const INPUT_BYTES: i32 = 16;
/// Size of a query response: shared reserve followed by the caller's deposit.
pub const POOL_QUERY_BYTES: usize = 72;

/// The shared pool and the invoking principal's private contribution.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PoolView {
    pub total_deposited: u64,
    pub participant_count: u32,
    pub pool_authority: [u8; PUBKEY_BYTES],
    pub deposit_amount: u64,
    pub deposit_count: u32,
}

impl PoolView {
    /// Decodes the real guest's bounded query response using both schemas.
    ///
    /// # Errors
    /// Refuses truncated, trailing or malformed account bytes.
    pub fn decode(response: &[u8]) -> Result<Self, PortRefusal> {
        if response.len() != POOL_QUERY_BYTES {
            return Err(PortRefusal::AccountBounds);
        }
        let (total_deposited, participant_count, pool_authority) =
            PoolReserve::decode(&response[..RESERVE_BYTES as usize])?;
        let (deposit_amount, deposit_count) =
            ParticipantDeposit::decode(&response[RESERVE_BYTES as usize..])?;
        Ok(Self {
            total_deposited,
            participant_count,
            pool_authority,
            deposit_amount,
            deposit_count,
        })
    }
}

/// An executable pool with immutable asset, destination, authority and per-call
/// deposit bound. Stored totals remain checked `u64` values and counts checked
/// `u32` values. Value moves only through the kernel's 402LXP request path.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SharedPoolPort {
    authority: [u8; PUBKEY_BYTES],
    asset: [u8; PUBKEY_BYTES],
    destination: [u8; PUBKEY_BYTES],
    maximum_deposit: u64,
}

impl SharedPoolPort {
    /// Pins the pool's monetary mapping and a nonzero per-call bound.
    ///
    /// # Errors
    /// Refuses reserved monetary keys, zero authority or a zero bound.
    pub fn new(
        authority: [u8; PUBKEY_BYTES],
        asset: [u8; PUBKEY_BYTES],
        destination: [u8; PUBKEY_BYTES],
        maximum_deposit: u64,
    ) -> Result<Self, PortRefusal> {
        Pubkey::new(authority)?;
        Transfer402Plan::new(asset, destination, u128::from(maximum_deposit))?;
        Ok(Self {
            authority,
            asset,
            destination,
            maximum_deposit,
        })
    }

    /// Returns the source-compatible deposit instruction codec.
    ///
    /// # Errors
    /// Refuses an invalid instruction schema.
    pub fn deposit_instruction() -> Result<InstructionAbi, PortRefusal> {
        InstructionAbi::new("pool_deposit", vec![FieldType::U64])
    }

    /// Returns the shared-reserve and caller-deposit query codec.
    ///
    /// # Errors
    /// Refuses an invalid instruction schema.
    pub fn query_instruction() -> Result<InstructionAbi, PortRefusal> {
        InstructionAbi::new("pool_query", vec![])
    }

    /// Declares exactly the scopes and monetary leg a deposit requires.
    /// The caller must authenticate these grants in its activity envelope.
    ///
    /// # Errors
    /// Refuses an amount outside the pinned bound or an invalid grant.
    pub fn deposit_capabilities(&self, amount: u64) -> Result<CapabilitySet, PortRefusal> {
        if amount == 0 || amount > self.maximum_deposit {
            return Err(PortRefusal::OutOfRange);
        }
        Ok(CapabilitySet::new([
            Capability::StorageRead,
            Capability::StorageWrite,
            Capability::SharedStorageRead,
            Capability::SharedStorageWrite,
            Transfer402Plan::new(self.asset, self.destination, u128::from(amount))?.capability(),
        ])?)
    }

    /// Grants read access to the shared reserve and the caller's own deposit.
    ///
    /// # Errors
    /// Refuses invalid capability encoding.
    pub fn query_capabilities() -> Result<CapabilitySet, PortRefusal> {
        Ok(CapabilitySet::new([
            Capability::StorageRead,
            Capability::SharedStorageRead,
        ])?)
    }

    /// Invokes this pool's emitted program with precise authority and response
    /// refusals before entering WASM. The guest independently enforces every
    /// operation, so direct host callers also get atomic rollback on refusal.
    /// The supplied storage is the enclosing activity's settlement candidate.
    ///
    /// # Errors
    /// Refuses another module, missing or insufficient grants, an undersized
    /// query response, or any refusal returned by the real executor.
    pub fn execute(
        &self,
        storage: &mut Storage,
        request: AuthorizedExecutionRequest<'_>,
    ) -> Result<V2AuthorizedExecutionRecord, ExecutionError> {
        let code = self
            .code()
            .map_err(|_| ExecutionError::Abi(AbiError::InvalidEncoding))?;
        if request.module.code_hash() != crate::hash::sha256(&code) {
            return Err(ExecutionError::Abi(AbiError::InvalidEncoding));
        }
        let query_tag = instruction_discriminator("pool_query");
        let deposit_tag = instruction_discriminator("pool_deposit");
        let required = if request.calldata == query_tag {
            if request.response_capacity < POOL_QUERY_BYTES {
                return Err(ExecutionError::Response(
                    layerx_programs_runtime::ResponseRefusal::CapacityExceeded {
                        bytes: POOL_QUERY_BYTES,
                        capacity: request.response_capacity,
                    },
                ));
            }
            vec![Capability::StorageRead, Capability::SharedStorageRead]
        } else if request.calldata.len() == INPUT_BYTES as usize
            && request.calldata[..8] == deposit_tag
        {
            let amount = u64::from_le_bytes(
                request.calldata[8..16]
                    .try_into()
                    .map_err(|_| ExecutionError::Abi(AbiError::InvalidEncoding))?,
            );
            // Invalid amounts remain guest refusals with the actual metered
            // execution evidence, as do malformed instructions.
            if amount == 0 || amount > self.maximum_deposit {
                return Executor::declared().execute_authorized_v2(storage, request);
            }
            vec![
                Capability::StorageRead,
                Capability::StorageWrite,
                Capability::SharedStorageRead,
                Capability::SharedStorageWrite,
                Capability::Transfer402 {
                    asset: self.asset,
                    to: self.destination,
                    maximum_amount: u128::from(amount),
                },
            ]
        } else {
            return Executor::declared().execute_authorized_v2(storage, request);
        };
        request
            .authorization
            .capabilities()
            .narrow(required)
            .map_err(|error| ExecutionError::Composition(CompositionRefusal::Authority(error)))?;
        Executor::declared().execute_authorized_v2(storage, request)
    }

    /// Emits the actual deterministic ABI-v2 deposit and query program.
    /// Every failed host call traps, rolling back all staged writes and effects.
    ///
    /// # Errors
    /// Refuses an invalid schema, key or module size.
    pub fn code(&self) -> Result<Vec<u8>, PortRefusal> {
        let reserve_key = PoolReserve::storage_key()?;
        let participant_key = ParticipantDeposit::storage_key()?;
        let reserve_key_len =
            i32::try_from(reserve_key.len()).map_err(|_| PortRefusal::InvalidSeeds)?;
        let participant_key_len =
            i32::try_from(participant_key.len()).map_err(|_| PortRefusal::InvalidSeeds)?;
        let query_bytes =
            i32::try_from(POOL_QUERY_BYTES).map_err(|_| PortRefusal::AccountBounds)?;
        let pubkey_bytes = i32::try_from(PUBKEY_BYTES).map_err(|_| PortRefusal::AccountBounds)?;
        let mut builder = ModuleBuilder::new(1);
        let scoped_type = builder.signature(&[I32; 5], &[I32]);
        let transfer_type = builder.signature(&[I64, I64, I32, I32, I32, I32], &[I32]);
        let response_type = builder.signature(&[I32; 3], &[I32]);
        let read_type = builder.signature(&[], &[]);
        let deposit_type = builder.signature(&[I64], &[]);
        let reserve_type = builder.signature(&[I32], &[I32]);
        let entry_type = builder.signature(&[I32, I32], &[I32]);
        let read = builder.import(ABI_V2_MODULE, "storage_read_scoped", scoped_type);
        let write = builder.import(ABI_V2_MODULE, "storage_write_scoped", scoped_type);
        let transfer = builder.import(ABI_MODULE, "transfer_402", transfer_type);
        let response = builder.import(ABI_V2_MODULE, "response_write", response_type);
        builder.segment(RESERVE_KEY, &reserve_key);
        builder.segment(PARTICIPANT_KEY, &participant_key);
        builder.segment(ASSET, &self.asset);
        builder.segment(DESTINATION, &self.destination);
        builder.segment(RESERVE, &PoolReserve::encode_initial(self.authority)?);
        builder.segment(
            PARTICIPANT,
            &ParticipantDeposit::schema()?.encode(&[FieldValue::U64(0), FieldValue::U32(0)])?,
        );
        let load = self.emit_load(
            &mut builder,
            read_type,
            read,
            reserve_key_len,
            participant_key_len,
        );
        let deposit = self.emit_deposit(
            &mut builder,
            deposit_type,
            load,
            write,
            transfer,
            reserve_key_len,
            participant_key_len,
            pubkey_bytes,
        );
        let mut reserve = Code::new();
        reserve.local_get(0);
        reserve.i32_const(0);
        reserve.op(I32_LT_S);
        reserve.trap_if();
        reserve.local_get(0);
        reserve.i32_const(INPUT_BYTES);
        reserve.op(crate::wasm::I32_GT_U);
        reserve.trap_if();
        reserve.pointer(INPUT);
        reserve.end();
        let reserve = builder.function(reserve_type, &[], &reserve);
        let entry = Self::emit_entry(
            &mut builder,
            entry_type,
            load,
            deposit,
            response,
            query_bytes,
        );
        builder.export_memory("memory");
        builder.export_function("layerx_reserve", reserve);
        builder.export_function(layerx_programs_runtime::CALL_ENTRY_EXPORT, entry);
        let wasm = builder.finish();
        if wasm.len() as u64 > layerx_programs_runtime::limits::DEFAULT_MAX_MODULE_BYTES {
            return Err(PortRefusal::ModuleTooLarge);
        }
        Ok(wasm)
    }

    fn emit_entry(
        builder: &mut ModuleBuilder,
        signature: u32,
        load: u32,
        deposit: u32,
        response: u32,
        query_bytes: i32,
    ) -> u32 {
        let mut entry = Code::new();
        entry.local_get(0);
        entry.pointer(INPUT);
        entry.op(I32_NE);
        entry.trap_if();
        entry.local_get(1);
        entry.i32_const(8);
        entry.op(I32_NE);
        entry.block(IF, VOID_BLOCK);
        entry.local_get(1);
        entry.i32_const(INPUT_BYTES);
        entry.op(I32_NE);
        entry.trap_if();
        entry.end();
        entry.pointer(INPUT);
        entry.memory(I64_LOAD, 0);
        entry.i64_const(i64::from_le_bytes(instruction_discriminator(
            "pool_deposit",
        )));
        entry.op(I64_EQ);
        entry.block(IF, VOID_BLOCK);
        entry.local_get(1);
        entry.i32_const(INPUT_BYTES);
        entry.op(I32_NE);
        entry.trap_if();
        entry.pointer(INPUT);
        entry.memory(I64_LOAD, 8);
        entry.call(deposit);
        entry.i32_const(0);
        entry.op(RETURN);
        entry.end();
        entry.pointer(INPUT);
        entry.memory(I64_LOAD, 0);
        entry.i64_const(i64::from_le_bytes(instruction_discriminator("pool_query")));
        entry.op(I64_NE);
        entry.trap_if();
        entry.local_get(1);
        entry.i32_const(8);
        entry.op(I32_NE);
        entry.trap_if();
        entry.call(load);
        entry.i32_const(0);
        entry.pointer(RESERVE);
        entry.i32_const(query_bytes);
        entry.call(response);
        entry.trap_unless_ok();
        entry.i32_const(0);
        entry.end();
        builder.function(signature, &[], &entry)
    }

    /// Validates the emitted pool against the production ABI-v2 engine.
    ///
    /// # Errors
    /// Refuses any deterministic-subset or engine validation error.
    pub fn validated_module(&self) -> Result<ValidatedModule, PortRefusal> {
        Ok(WasmEngine::declared()?.validate_v2(&self.code()?)?)
    }

    fn emit_load(
        &self,
        builder: &mut ModuleBuilder,
        signature: u32,
        read: u32,
        reserve_key_len: i32,
        participant_key_len: i32,
    ) -> u32 {
        let mut code = Code::new();
        for (scope, key, key_len, output, size, name) in [
            (
                2,
                RESERVE_KEY,
                reserve_key_len,
                RESERVE,
                RESERVE_BYTES,
                "PoolReserve",
            ),
            (
                1,
                PARTICIPANT_KEY,
                participant_key_len,
                PARTICIPANT,
                PARTICIPANT_BYTES,
                "ParticipantDeposit",
            ),
        ] {
            code.i32_const(scope);
            code.pointer(key);
            code.i32_const(key_len);
            code.pointer(output);
            code.i32_const(size);
            code.call(read);
            code.local_set(0);
            code.local_get(0);
            code.i32_const(0);
            code.op(I32_LT_S);
            code.trap_if();
            code.local_get(0);
            code.op(I32_EQZ);
            code.block(IF, VOID_BLOCK);
            // Missing cells use the schema-encoded initial data segment.
            code.op(ELSE);
            code.local_get(0);
            code.i32_const(size + 1);
            code.op(I32_NE);
            code.trap_if();
            code.pointer(output);
            code.memory(I64_LOAD, 0);
            code.i64_const(i64::from_le_bytes(account_discriminator(name)));
            code.op(I64_NE);
            code.trap_if();
            code.end();
        }
        for (index, chunk) in (0_u32..).zip(self.authority.chunks_exact(8)) {
            code.pointer(RESERVE);
            code.memory(I64_LOAD, 20 + index * 8);
            code.i64_const(i64::from_le_bytes(
                chunk
                    .try_into()
                    .unwrap_or_else(|_| unreachable!("eight-byte chunk")),
            ));
            code.op(I64_NE);
            code.trap_if();
        }
        code.end();
        builder.function(signature, &[(1, I32)], &code)
    }

    #[allow(clippy::too_many_arguments)]
    fn emit_deposit(
        &self,
        builder: &mut ModuleBuilder,
        signature: u32,
        load: u32,
        write: u32,
        transfer: u32,
        reserve_key_len: i32,
        participant_key_len: i32,
        pubkey_bytes: i32,
    ) -> u32 {
        let mut code = Code::new();
        code.local_get(0);
        code.op(I64_EQZ);
        code.trap_if();
        code.local_get(0);
        code.i64_const(i64::from_le_bytes(self.maximum_deposit.to_le_bytes()));
        code.op(I64_GT_U);
        code.trap_if();
        code.call(load);
        for output in [RESERVE, PARTICIPANT] {
            // Refuse unsigned overflow before storing the incremented total.
            code.pointer(output);
            code.memory(I64_LOAD, 8);
            code.local_set(1);
            code.local_get(1);
            code.local_get(0);
            code.op(I64_ADD);
            code.local_set(2);
            code.local_get(1);
            code.local_get(2);
            code.op(I64_GT_U);
            code.trap_if();
            code.pointer(output);
            code.local_get(2);
            code.memory(I64_STORE, 8);
        }
        code.pointer(PARTICIPANT);
        code.memory(I32_LOAD, 16);
        code.op(I32_EQZ);
        code.block(IF, VOID_BLOCK);
        emit_increment_count(&mut code, RESERVE);
        code.end();
        emit_increment_count(&mut code, PARTICIPANT);
        for (scope, key, len, output, size) in [
            (2, RESERVE_KEY, reserve_key_len, RESERVE, RESERVE_BYTES),
            (
                1,
                PARTICIPANT_KEY,
                participant_key_len,
                PARTICIPANT,
                PARTICIPANT_BYTES,
            ),
        ] {
            code.i32_const(scope);
            code.pointer(key);
            code.i32_const(len);
            code.pointer(output);
            code.i32_const(size);
            code.call(write);
            code.trap_unless_ok();
        }
        // The exact principal-funded transfer is part of the same ABI
        // transaction; denied authority discards both staged account writes.
        code.i64_const(0);
        code.local_get(0);
        code.pointer(ASSET);
        code.i32_const(pubkey_bytes);
        code.pointer(DESTINATION);
        code.i32_const(pubkey_bytes);
        code.call(transfer);
        code.trap_unless_ok();
        code.end();
        builder.function(signature, &[(2, I64)], &code)
    }
}

fn emit_increment_count(code: &mut Code, output: u32) {
    code.pointer(output);
    code.memory(I32_LOAD, 16);
    code.i32_const(-1);
    code.op(crate::wasm::I32_EQ);
    code.trap_if();
    code.pointer(output);
    code.pointer(output);
    code.memory(I32_LOAD, 16);
    code.i32_const(1);
    code.op(I32_ADD);
    code.memory(I32_STORE, 16);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn pool_reserve_is_program_owned_shared() {
        let role = PoolReserve::role();
        assert_eq!(role, AccountRole::ProgramOwnedShared);

        let mapping =
            PoolReserve::translate().unwrap_or_else(|error| panic!("shared pool: {error}"));
        assert_eq!(mapping, AccountMapping::SharedCell);
    }

    #[test]
    fn participant_deposit_is_principal_scoped() {
        let role = ParticipantDeposit::role();
        assert_eq!(role, AccountRole::ProgramState);

        let mapping = role
            .translate()
            .unwrap_or_else(|error| panic!("shared pool: {error}"));
        assert_eq!(mapping, AccountMapping::NamespacedCell);
    }

    #[test]
    fn pool_schema_has_correct_space() {
        let schema = PoolReserve::schema().unwrap_or_else(|error| panic!("shared pool: {error}"));
        // Discriminator (8) + u64 (8) + u32 (4) + Pubkey (32) = 52
        assert_eq!(schema.space(), 52);
    }

    #[test]
    fn pool_state_round_trips() {
        let authority = [42u8; PUBKEY_BYTES];
        let encoded = PoolReserve::encode_initial(authority)
            .unwrap_or_else(|error| panic!("shared pool: {error}"));
        let (total, count, auth) =
            PoolReserve::decode(&encoded).unwrap_or_else(|error| panic!("shared pool: {error}"));

        assert_eq!(total, 0);
        assert_eq!(count, 0);
        assert_eq!(auth, authority);
    }

    #[test]
    fn participant_schema_has_correct_space() {
        let schema =
            ParticipantDeposit::schema().unwrap_or_else(|error| panic!("shared pool: {error}"));
        // Discriminator (8) + u64 (8) + u32 (4) = 20
        assert_eq!(schema.space(), 20);
    }

    #[test]
    fn pool_storage_key_derivation() {
        // Demonstrates that the pool key can be derived and will land in
        // shared namespace when the runtime executes it
        let key = PoolReserve::storage_key().unwrap_or_else(|error| panic!("shared pool: {error}"));
        assert!(!key.is_empty());
    }

    #[test]
    fn participant_storage_key_collapses_signer() {
        // Demonstrates that the participant key collapses the signer seed
        // and will land in principal-scoped namespace
        let first = ParticipantDeposit::seeds(
            Pubkey::new([1; PUBKEY_BYTES]).unwrap_or_else(|error| panic!("shared pool: {error}")),
        )
        .unwrap_or_else(|error| panic!("shared pool: {error}"))
        .collapse(&PARTICIPANT_ENVELOPE_SEEDS)
        .unwrap_or_else(|error| panic!("shared pool: {error}"))
        .storage_key()
        .unwrap_or_else(|error| panic!("shared pool: {error}"));
        let second = ParticipantDeposit::seeds(
            Pubkey::new([2; PUBKEY_BYTES]).unwrap_or_else(|error| panic!("shared pool: {error}")),
        )
        .unwrap_or_else(|error| panic!("shared pool: {error}"))
        .collapse(&PARTICIPANT_ENVELOPE_SEEDS)
        .unwrap_or_else(|error| panic!("shared pool: {error}"))
        .storage_key()
        .unwrap_or_else(|error| panic!("shared pool: {error}"));
        assert_eq!(first, second);
        assert_eq!(
            first,
            ParticipantDeposit::storage_key()
                .unwrap_or_else(|error| panic!("shared pool: {error}"))
        );
    }
}

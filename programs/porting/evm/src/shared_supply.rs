//! Reference port demonstrating shared state: an ERC-20 total supply.
//!
//! The contract tracks a global `totalSupply` that every holder can read,
//! demonstrating the program-shared namespace. Individual balances remain
//! principal-scoped through the caller-indexed pattern.

use layerx_programs_runtime::{
    Capability, CapabilitySet, Deploy, ProgramId, StorageSelector, UpgradePolicy, ABI_MODULE,
    ABI_V2_MODULE, ABI_V2_VERSION, CALL_RESERVE_EXPORT,
};

use crate::error::PortRefusal;
use crate::layout::{caller_indexed_key, shared_key, value_slot};
use crate::wasm::{
    Code, ModuleBuilder, I32, I32_EQZ, I32_LOAD8_U, I32_NE, I32_STORE8, I32_WRAP_I64, I64, I64_ADD,
    I64_EXTEND_I32_U, I64_GT_U, I64_LOAD, I64_LT_S, I64_MUL, I64_NE, I64_OR, I64_SHL, I64_SHR_U,
    I64_STORE, I64_SUB, IF, VOID_BLOCK,
};

/// Declaration-order slot of the total supply in shared namespace.
pub const TOTAL_SUPPLY_SLOT: u64 = 0;
/// Declaration-order slot of per-holder balances, caller-indexed.
pub const BALANCES_SLOT: u64 = 1;
/// Maximum supply the contract may mint.
pub const MAX_SUPPLY: u64 = 1_000_000;
/// Recipient pinned by the fixed reference deployment and `new` convenience constructor.
pub const DEFAULT_RECIPIENT: [u8; 32] = [7; 32];
/// Asset pinned by the fixed reference deployment.
pub const DEFAULT_ASSET: [u8; 32] = [7; 32];

/// Terms a token deployment pins into its module.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TokenTerms {
    /// The identifier of the 402LXP asset standing in for the ERC-20.
    pub asset: [u8; 32],
    /// The per-token price in the asset's smallest unit.
    pub price_per_token: u64,
}

/// The ported token contract descriptor.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SharedSupplyPort {
    terms: TokenTerms,
    recipient: [u8; 32],
}

impl SharedSupplyPort {
    /// Constructs a port descriptor paying the fixed reference recipient, `DEFAULT_RECIPIENT`.
    ///
    /// # Errors
    ///
    /// Refuses zero price or oversized bounds.
    pub fn new(terms: TokenTerms) -> Result<Self, PortRefusal> {
        Self::new_with_recipient(terms, DEFAULT_RECIPIENT)
    }

    /// Pins explicit payment terms, including a recipient independent of the asset.
    ///
    /// # Errors
    /// Refuses zero identifiers, zero price, or a price whose maximum mint exceeds `u64`.
    pub fn new_with_recipient(terms: TokenTerms, recipient: [u8; 32]) -> Result<Self, PortRefusal> {
        if terms.asset == [0; 32]
            || recipient == [0; 32]
            || terms.price_per_token == 0
            || terms.price_per_token > u64::MAX / MAX_SUPPLY
        {
            return Err(PortRefusal::OutOfRange);
        }
        Ok(Self { terms, recipient })
    }

    /// Returns the storage key for the shared total supply.
    #[must_use]
    pub fn total_supply_key() -> [u8; 32] {
        shared_key(value_slot(TOTAL_SUPPLY_SLOT))
    }

    /// Returns the storage key for one holder's balance.
    #[must_use]
    pub fn balance_key() -> [u8; 32] {
        caller_indexed_key(BALANCES_SLOT)
    }

    /// Returns the capabilities a mint operation requires.
    ///
    /// # Errors
    ///
    /// Refuses invalid capability encoding.
    pub fn mint_capabilities(&self, amount: u64) -> Result<CapabilitySet, PortRefusal> {
        if amount == 0 || amount > MAX_SUPPLY {
            return Err(PortRefusal::OutOfRange);
        }
        let transfer = Capability::Transfer402 {
            asset: self.terms.asset,
            to: self.recipient,
            maximum_amount: u128::from(self.terms.price_per_token)
                .checked_mul(u128::from(amount))
                .ok_or(PortRefusal::OutOfRange)?,
        };
        CapabilitySet::new([
            Capability::StorageRead,
            Capability::SharedStorageRead,
            Capability::StorageWrite,
            Capability::SharedStorageWrite,
            Capability::EmitEvent,
            transfer,
        ])
        .map_err(PortRefusal::from)
    }

    /// Returns the capabilities a balance query requires.
    ///
    /// # Errors
    ///
    /// Refuses invalid capability encoding.
    pub fn balance_query_capabilities() -> Result<CapabilitySet, PortRefusal> {
        CapabilitySet::new([Capability::StorageRead]).map_err(PortRefusal::from)
    }

    /// Returns the capabilities a total supply query requires.
    ///
    /// # Errors
    ///
    /// Refuses invalid capability encoding.
    pub fn supply_query_capabilities() -> Result<CapabilitySet, PortRefusal> {
        CapabilitySet::new([Capability::SharedStorageRead]).map_err(PortRefusal::from)
    }

    /// Builds the fixed reference deployment: `DEFAULT_ASSET`, price one, `DEFAULT_RECIPIENT`.
    ///
    /// # Errors
    ///
    /// Refuses an invalid or oversized module construction.
    pub fn deployment(program: ProgramId) -> Result<Deploy, PortRefusal> {
        Self::fixed_reference()?.deployment_for_terms(program)
    }

    /// Builds a deployment pinned to this descriptor's validated payment terms.
    ///
    /// # Errors
    /// Refuses an invalid or oversized module construction.
    pub fn deployment_for_terms(&self, program: ProgramId) -> Result<Deploy, PortRefusal> {
        let wasm = self.code_for_terms()?;
        Ok(Deploy {
            program,
            code_hash: crate::hash::sha256(&wasm),
            wasm,
            abi_version: ABI_V2_VERSION,
            upgrade_policy: UpgradePolicy::Immutable,
        })
    }

    /// Emits the fixed reference module with the same terms as `deployment`.
    ///
    /// # Errors
    ///
    /// Refuses oversized modules and invalid constructions.
    pub fn code() -> Result<Vec<u8>, PortRefusal> {
        Self::fixed_reference()?.code_for_terms()
    }

    fn fixed_reference() -> Result<Self, PortRefusal> {
        Self::new_with_recipient(
            TokenTerms {
                asset: DEFAULT_ASSET,
                price_per_token: 1,
            },
            DEFAULT_RECIPIENT,
        )
    }

    /// Emits code pinned to this descriptor's asset, price and explicit recipient.
    ///
    /// # Errors
    /// Refuses oversized modules and invalid constructions.
    pub fn code_for_terms(&self) -> Result<Vec<u8>, PortRefusal> {
        let mut builder = ModuleBuilder::new(MEMORY_PAGES);
        let host_type = builder.signature(&[I32, I32, I32, I32, I32], &[I32]);
        let load_type = builder.signature(&[I32], &[I64]);
        let store_type = builder.signature(&[I32, I64], &[]);
        let read_type = builder.signature(&[I32, I32], &[I64]);
        let mint_type = builder.signature(&[I64], &[I64]);
        let query_type = builder.signature(&[], &[I64]);
        let entry_type = builder.signature(&[I32, I32], &[I32]);
        let reserve_type = builder.signature(&[I32], &[I32]);
        let response_type = builder.signature(&[I32, I32, I32], &[I32]);
        let event_type = builder.signature(&[I32, I32, I32, I32], &[I32]);
        let transfer_type = builder.signature(&[I64, I64, I32, I32, I32, I32], &[I32]);
        let storage_read = builder.import(ABI_V2_MODULE, "storage_read_scoped", host_type);
        let storage_write = builder.import(ABI_V2_MODULE, "storage_write_scoped", host_type);
        let response_write = builder.import(ABI_V2_MODULE, "response_write", response_type);
        let transfer = builder.import(ABI_MODULE, "transfer_402", transfer_type);
        let event = builder.import(ABI_MODULE, "event_emit", event_type);
        builder.segment(ASSET_POINTER, &self.terms.asset);
        builder.segment(RECIPIENT_POINTER, &self.recipient);
        builder.segment(
            TOPIC_POINTER,
            &crate::semantics::EventAbi::envelope_derived("Transfer(address,address,uint256)", 2)?
                .topic(),
        );
        builder.segment(SUPPLY_KEY_POINTER, &Self::total_supply_key());
        builder.segment(BALANCE_KEY_POINTER, &Self::balance_key());
        let load_be64 = emit_load_be64(&mut builder, load_type);
        let store_word = emit_store_word(&mut builder, store_type);
        let read_word = emit_read_word(&mut builder, read_type, storage_read, load_be64);
        let mint = emit_mint(
            &mut builder,
            mint_type,
            storage_write,
            read_word,
            store_word,
            transfer,
            event,
            self.terms.price_per_token,
        )?;
        let balance_of = emit_query(
            &mut builder,
            query_type,
            StorageSelector::Principal,
            BALANCE_KEY_POINTER,
            read_word,
        );
        let total_supply = emit_query(
            &mut builder,
            query_type,
            StorageSelector::Shared,
            SUPPLY_KEY_POINTER,
            read_word,
        );
        let reserve = emit_reserve(&mut builder, reserve_type);
        for (name, target, argument) in [
            (MINT_ENTRY_EXPORT, mint, Some(load_be64)),
            (BALANCE_OF_ENTRY_EXPORT, balance_of, None),
            (TOTAL_SUPPLY_ENTRY_EXPORT, total_supply, None),
        ] {
            let entry = emit_entry(
                &mut builder,
                entry_type,
                target,
                argument,
                store_word,
                response_write,
            );
            builder.export_function(name, entry);
        }
        builder.export_function(CALL_RESERVE_EXPORT, reserve);
        builder.export_memory(MEMORY_EXPORT);
        builder.export_function(MINT_EXPORT, mint);
        builder.export_function(BALANCE_OF_EXPORT, balance_of);
        builder.export_function(TOTAL_SUPPLY_EXPORT, total_supply);
        let wasm = builder.finish();
        if u64::try_from(wasm.len()).unwrap_or(u64::MAX)
            > layerx_programs_runtime::limits::DEFAULT_MAX_MODULE_BYTES
        {
            return Err(PortRefusal::ModuleTooLarge);
        }
        Ok(wasm)
    }
}

/// Exported name of the mint entry point.
pub const MINT_EXPORT: &str = "mint";
/// Exported name of the balance query.
pub const BALANCE_OF_EXPORT: &str = "balanceOf";
/// Exported name of the total supply query.
pub const TOTAL_SUPPLY_EXPORT: &str = "totalSupply";
/// Byte entry point accepting one big-endian `u64` mint amount and returning a word.
pub const MINT_ENTRY_EXPORT: &str = "mint_call";
/// Byte entry point returning the invoking principal's balance as a word.
pub const BALANCE_OF_ENTRY_EXPORT: &str = "balanceOf_call";
/// Byte entry point returning the shared total supply as a word.
pub const TOTAL_SUPPLY_ENTRY_EXPORT: &str = "totalSupply_call";
/// Exported name of the module's linear memory.
pub const MEMORY_EXPORT: &str = "memory";

const MEMORY_PAGES: u32 = 1;
const SUPPLY_KEY_POINTER: u32 = 0;
const BALANCE_KEY_POINTER: u32 = 32;
const VALUE_POINTER: u32 = 64;
const INPUT_POINTER: u32 = 96;
const KEY_LENGTH: i32 = 32;
const ASSET_POINTER: u32 = 128;
const RECIPIENT_POINTER: u32 = 160;
const TOPIC_POINTER: u32 = 192;

fn emit_load_be64(builder: &mut ModuleBuilder, signature: u32) -> u32 {
    let mut code = Code::new();
    code.i64_const(0);
    code.local_set(1);
    for offset in 0..8_u32 {
        code.local_get(1);
        code.i64_const(8);
        code.op(I64_SHL);
        code.local_get(0);
        code.memory(I32_LOAD8_U, offset);
        code.op(I64_EXTEND_I32_U);
        code.op(I64_OR);
        code.local_set(1);
    }
    code.local_get(1);
    code.end();
    builder.function(signature, &[(1, I64)], &code)
}

fn emit_store_word(builder: &mut ModuleBuilder, signature: u32) -> u32 {
    let mut code = Code::new();
    for offset in [0_u32, 8, 16] {
        code.local_get(0);
        code.i64_const(0);
        code.memory(I64_STORE, offset);
    }
    for index in 0..8_u32 {
        code.local_get(0);
        code.local_get(1);
        code.i64_const(i64::from(56 - index * 8));
        code.op(I64_SHR_U);
        code.op(I32_WRAP_I64);
        code.memory(I32_STORE8, 24 + index);
    }
    code.end();
    builder.function(signature, &[], &code)
}

fn emit_read_word(
    builder: &mut ModuleBuilder,
    signature: u32,
    storage_read: u32,
    load_be64: u32,
) -> u32 {
    let mut code = Code::new();
    for offset in [0_u32, 8, 16, 24] {
        code.pointer(VALUE_POINTER);
        code.i64_const(0);
        code.memory(I64_STORE, offset);
    }
    code.local_get(0);
    code.local_get(1);
    code.i32_const(KEY_LENGTH);
    code.pointer(VALUE_POINTER);
    code.i32_const(KEY_LENGTH);
    code.call(storage_read);
    code.local_set(2);
    // A missing cell is zero; any present cell must be a complete word.
    code.local_get(2);
    code.op(I32_EQZ);
    code.block(IF, VOID_BLOCK);
    code.op(crate::wasm::ELSE);
    code.local_get(2);
    code.i32_const(KEY_LENGTH + 1);
    code.op(I32_NE);
    code.trap_if();
    code.end();
    for offset in [0_u32, 8, 16] {
        code.pointer(VALUE_POINTER);
        code.memory(I64_LOAD, offset);
        code.i64_const(0);
        code.op(I64_NE);
        code.trap_if();
    }
    code.pointer(VALUE_POINTER + 24);
    code.call(load_be64);
    code.end();
    builder.function(signature, &[(1, I32)], &code)
}

fn emit_mint(
    builder: &mut ModuleBuilder,
    signature: u32,
    storage_write: u32,
    read_word: u32,
    store_word: u32,
    transfer: u32,
    event: u32,
    price: u64,
) -> Result<u32, PortRefusal> {
    let mut code = Code::new();
    code.local_get(0);
    code.i64_const(1);
    code.op(I64_LT_S);
    code.trap_if();
    code.i32_const(StorageSelector::Shared as i32);
    code.i32_const(i32::try_from(SUPPLY_KEY_POINTER).map_err(|_| PortRefusal::OutOfRange)?);
    code.call(read_word);
    code.local_set(1);
    code.local_get(1);
    code.i64_const(MAX_SUPPLY as i64);
    code.op(I64_GT_U);
    code.trap_if();
    code.local_get(0);
    code.i64_const(MAX_SUPPLY as i64);
    code.local_get(1);
    code.op(I64_SUB);
    code.op(I64_GT_U);
    code.trap_if();
    code.local_get(1);
    code.local_get(0);
    code.op(I64_ADD);
    code.local_set(2);
    code.pointer(VALUE_POINTER);
    code.local_get(2);
    code.call(store_word);
    code.i32_const(StorageSelector::Shared as i32);
    code.pointer(SUPPLY_KEY_POINTER);
    code.i32_const(KEY_LENGTH);
    code.pointer(VALUE_POINTER);
    code.i32_const(KEY_LENGTH);
    code.call(storage_write);
    code.trap_unless_ok();
    code.i32_const(StorageSelector::Principal as i32);
    code.i32_const(i32::try_from(BALANCE_KEY_POINTER).map_err(|_| PortRefusal::OutOfRange)?);
    code.call(read_word);
    code.local_set(3);
    code.local_get(0);
    code.i64_const(-1);
    code.local_get(3);
    code.op(I64_SUB);
    code.op(I64_GT_U);
    code.trap_if();
    code.local_get(3);
    code.local_get(0);
    code.op(I64_ADD);
    code.local_set(4);
    code.pointer(VALUE_POINTER);
    code.local_get(4);
    code.call(store_word);
    code.i32_const(StorageSelector::Principal as i32);
    code.pointer(BALANCE_KEY_POINTER);
    code.i32_const(KEY_LENGTH);
    code.pointer(VALUE_POINTER);
    code.i32_const(KEY_LENGTH);
    code.call(storage_write);
    code.trap_unless_ok();
    code.i64_const(0);
    code.local_get(0);
    code.i64_const(price as i64);
    code.op(I64_MUL);
    code.pointer(ASSET_POINTER);
    code.i32_const(KEY_LENGTH);
    code.pointer(RECIPIENT_POINTER);
    code.i32_const(KEY_LENGTH);
    code.call(transfer);
    code.trap_unless_ok();
    code.pointer(VALUE_POINTER);
    code.local_get(0);
    code.call(store_word);
    code.pointer(TOPIC_POINTER);
    code.i32_const(KEY_LENGTH);
    code.pointer(VALUE_POINTER);
    code.i32_const(KEY_LENGTH);
    code.call(event);
    code.trap_unless_ok();
    code.local_get(2);
    code.end();
    Ok(builder.function(signature, &[(4, I64)], &code))
}

fn emit_query(
    builder: &mut ModuleBuilder,
    signature: u32,
    selector: StorageSelector,
    key_pointer: u32,
    read_word: u32,
) -> u32 {
    let mut code = Code::new();
    code.i32_const(selector as i32);
    code.pointer(key_pointer);
    code.call(read_word);
    code.end();
    builder.function(signature, &[], &code)
}

fn emit_reserve(builder: &mut ModuleBuilder, signature: u32) -> u32 {
    let mut code = Code::new();
    code.local_get(0);
    code.i32_const(8);
    code.op(I32_NE);
    code.trap_if();
    code.pointer(INPUT_POINTER);
    code.end();
    builder.function(signature, &[], &code)
}

fn emit_entry(
    builder: &mut ModuleBuilder,
    signature: u32,
    target: u32,
    argument: Option<u32>,
    store_word: u32,
    response_write: u32,
) -> u32 {
    let mut code = Code::new();
    code.local_get(1);
    code.i32_const(if argument.is_some() { 8 } else { 0 });
    code.op(I32_NE);
    code.trap_if();
    code.pointer(VALUE_POINTER);
    if let Some(load_be64) = argument {
        code.local_get(0);
        code.call(load_be64);
    }
    code.call(target);
    code.call(store_word);
    code.i32_const(0);
    code.pointer(VALUE_POINTER);
    code.i32_const(KEY_LENGTH);
    code.call(response_write);
    code.trap_unless_ok();
    code.i32_const(0);
    code.end();
    builder.function(signature, &[], &code)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn shared_supply_key_addresses_shared_namespace() {
        let terms = TokenTerms {
            asset: [1u8; 32],
            price_per_token: 100,
        };
        let _port =
            SharedSupplyPort::new(terms).unwrap_or_else(|error| panic!("shared supply: {error}"));

        // The total supply key uses shared_key, demonstrating shared namespace
        let supply_key = SharedSupplyPort::total_supply_key();
        assert_eq!(supply_key, shared_key(value_slot(TOTAL_SUPPLY_SLOT)));
    }

    #[test]
    fn balance_key_is_caller_indexed() {
        let terms = TokenTerms {
            asset: [1u8; 32],
            price_per_token: 100,
        };
        let _port =
            SharedSupplyPort::new(terms).unwrap_or_else(|error| panic!("shared supply: {error}"));

        // Balances remain principal-scoped
        let balance_key = SharedSupplyPort::balance_key();
        assert_eq!(balance_key, caller_indexed_key(BALANCES_SLOT));
    }

    #[test]
    fn mint_requires_both_namespaces() {
        let terms = TokenTerms {
            asset: [1u8; 32],
            price_per_token: 100,
        };
        let port = SharedSupplyPort::new_with_recipient(terms, [1; 32])
            .unwrap_or_else(|error| panic!("shared supply: {error}"));
        let caps = port
            .mint_capabilities(10)
            .unwrap_or_else(|error| panic!("shared supply: {error}"));

        // Mint needs principal-scoped read/write for balances
        // and shared read/write for total supply
        let expected = CapabilitySet::new([
            Capability::StorageRead,
            Capability::SharedStorageRead,
            Capability::StorageWrite,
            Capability::SharedStorageWrite,
            Capability::EmitEvent,
            Capability::Transfer402 {
                asset: [1u8; 32],
                to: [1u8; 32],
                maximum_amount: 1_000,
            },
        ])
        .unwrap_or_else(|error| panic!("shared supply: {error}"));
        assert_eq!(caps, expected);
    }

    #[test]
    fn supply_query_requires_only_shared_read() {
        let terms = TokenTerms {
            asset: [1u8; 32],
            price_per_token: 100,
        };
        let _port =
            SharedSupplyPort::new(terms).unwrap_or_else(|error| panic!("shared supply: {error}"));
        let caps = SharedSupplyPort::supply_query_capabilities()
            .unwrap_or_else(|error| panic!("shared supply: {error}"));

        // Total supply query only needs shared read
        let expected = CapabilitySet::new([Capability::SharedStorageRead])
            .unwrap_or_else(|error| panic!("shared supply: {error}"));
        assert_eq!(caps, expected);
    }
}

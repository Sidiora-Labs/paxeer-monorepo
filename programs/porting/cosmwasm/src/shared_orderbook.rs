//! Reference port demonstrating shared state: a `CosmWasm` order book.
//!
//! [`OrderBook`] exposes the key schema. [`BoundedOrderBook`] executes a
//! single sell order between two registered traders through ABI-v2 scoped host
//! operations. Its finite base/quote lots are an internal settlement ledger,
//! allocated once by the trusted migration/bootstrap path; they are not 402LXP
//! balances and have no mint, deposit or withdrawal operation. Matching conserves
//! both lots, escrows resting base quantity, and commits the order, both shared
//! balances and the taker's principal position in one runtime transaction.

use crate::error::PortRefusal;
use crate::storage::{map_key, StateBinding};
use crate::wasm::*;
use layerx_programs_runtime::{
    Capability, CapabilitySet, Deploy, PrincipalId, ProgramId, Storage, StorageNamespace,
    StorageSelector, UpgradePolicy, ABI_V2_MODULE, ABI_V2_VERSION,
};

/// The global order count, shared across all traders.
pub const ORDER_COUNT: &str = "order_count";
/// The order book map, shared across all traders.
pub const ORDERS: &str = "orders";
/// Per-trader position, principal-scoped.
pub const POSITIONS: &str = "positions";

/// An order book entry visible to every trader.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Order {
    /// The order identifier.
    pub id: u64,
    /// Whether the order is buying or selling.
    pub is_buy: bool,
    /// The price per unit.
    pub price: u128,
    /// The remaining quantity.
    pub quantity: u64,
}

/// A trader's position, principal-scoped.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Position {
    /// The trader's buy-side quantity.
    pub buy_quantity: u64,
    /// The trader's sell-side quantity.
    pub sell_quantity: u64,
}

/// Demonstrates the storage bindings for a shared order book.
pub struct OrderBook;

impl OrderBook {
    /// Returns the binding for the global order count.
    #[must_use]
    pub fn order_count_binding() -> StateBinding {
        StateBinding::Shared
    }

    /// Returns the binding for the order book map.
    #[must_use]
    pub fn orders_binding() -> StateBinding {
        StateBinding::Shared
    }

    /// Returns the binding for per-trader positions.
    #[must_use]
    pub fn positions_binding() -> StateBinding {
        StateBinding::SenderIndexed
    }

    /// Returns the storage key for the global order count in shared namespace.
    ///
    /// # Errors
    ///
    /// Refuses invalid namespace or key construction.
    pub fn order_count_key() -> Result<Vec<u8>, PortRefusal> {
        Self::order_count_binding().layerx_key(ORDER_COUNT, &[])
    }

    /// Returns the storage key for one order in the shared order book.
    ///
    /// # Errors
    ///
    /// Refuses invalid namespace or key construction.
    pub fn order_key(order_id: u64) -> Result<Vec<u8>, PortRefusal> {
        let id_bytes = order_id.to_le_bytes();
        map_key(ORDERS, &id_bytes)
    }

    /// Returns the storage key for one trader's position in principal-scoped
    /// namespace.
    ///
    /// # Errors
    ///
    /// Refuses invalid namespace or key construction.
    pub fn position_key() -> Result<Vec<u8>, PortRefusal> {
        Self::positions_binding().layerx_key(POSITIONS, &[])
    }

    /// Returns whether the order count addresses shared namespace.
    #[must_use]
    pub fn order_count_is_shared() -> bool {
        Self::order_count_binding().shared()
    }

    /// Returns whether the order book addresses shared namespace.
    #[must_use]
    pub fn orders_is_shared() -> bool {
        Self::orders_binding().shared()
    }

    /// Returns whether positions address shared namespace.
    #[must_use]
    pub fn positions_is_shared() -> bool {
        Self::positions_binding().shared()
    }

    /// Returns whether the order count is portable.
    #[must_use]
    pub fn order_count_portable() -> bool {
        Self::order_count_binding().portable()
    }

    /// Returns whether the order book is portable.
    #[must_use]
    pub fn orders_portable() -> bool {
        Self::orders_binding().portable()
    }

    /// Returns whether positions are portable.
    #[must_use]
    pub fn positions_portable() -> bool {
        Self::positions_binding().portable()
    }
}

/// Maximum base lots allocated by one bounded market.
pub const MAX_ORDER_LOTS: u64 = 1_000_000;
/// ABI entry point accepting a little-endian `u64` sell quantity.
pub const PLACE_ORDER: &str = "place_order";
/// ABI entry point accepting a little-endian `u64` match quantity.
pub const MATCH_ORDER: &str = "match_order";
/// ABI entry point returning the canonical 33-byte [`Order`] record.
pub const READ_ORDER: &str = "read_order";

const ROLE_KEY: &[u8] = b"orderbook_role";
const SETTLEMENT: &str = "orderbook_settlement";
const INPUT: u32 = 1024;
const ROLE: u32 = 1100;
const COUNT: u32 = 1120;
const ORDER: u32 = 1140;
const MAKER: u32 = 1200;
const TAKER: u32 = 1220;
const POSITION: u32 = 1240;
const ORDER_BYTES: i32 = 33;

/// Immutable market terms and initial allocation of internal base/quote lots.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct OrderBookTerms {
    /// Principal allowed to place the one sell order.
    pub maker: PrincipalId,
    /// Principal allowed to buy lots from that order.
    pub taker: PrincipalId,
    /// Initial maker base inventory, bounded by [`MAX_ORDER_LOTS`].
    pub base_lots: u64,
    /// Initial taker quote inventory, bounded by the signed ABI integer domain.
    pub quote_lots: u64,
    /// Quote lots paid per base lot, fixed at deployment.
    pub price: u64,
}

/// Executable, finite two-trader market with one lifetime resting order.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct BoundedOrderBook {
    terms: OrderBookTerms,
}

impl BoundedOrderBook {
    /// Validates immutable terms, including the maximum price/quantity product.
    ///
    /// # Errors
    /// Refuses identical traders, zero allocations/prices and arithmetic bounds.
    pub fn new(terms: OrderBookTerms) -> Result<Self, PortRefusal> {
        if terms.maker == terms.taker
            || terms.base_lots == 0
            || terms.base_lots > MAX_ORDER_LOTS
            || terms.quote_lots == 0
            || terms.quote_lots > i64::MAX as u64
            || terms.price == 0
            || terms
                .price
                .checked_mul(terms.base_lots)
                .is_none_or(|amount| amount > i64::MAX as u64)
        {
            return Err(PortRefusal::OutOfRange);
        }
        Ok(Self { terms })
    }

    /// Shared Map key of a trader's conserved `(base, quote)` lot balance.
    ///
    /// # Errors
    /// Refuses a key outside the storage bounds.
    pub fn settlement_key(trader: PrincipalId) -> Result<Vec<u8>, PortRefusal> {
        map_key(SETTLEMENT, &trader.bytes())
    }

    /// Allocates the finite market once, through the trusted native bootstrap
    /// or migration boundary. Guest authority cannot change allocations/roles.
    /// The entire setup is atomic, including when a destination is frozen.
    ///
    /// # Errors
    /// Refuses an existing destination or any storage refusal.
    pub fn initialize(&self, storage: &mut Storage, program: ProgramId) -> Result<(), PortRefusal> {
        let mut candidate = storage.clone();
        let shared = StorageNamespace::shared(program);
        let mut order = Vec::new();
        order.extend_from_slice(&1_u64.to_le_bytes());
        order.push(0);
        order.extend_from_slice(&u128::from(self.terms.price).to_le_bytes());
        order.extend_from_slice(&0_u64.to_le_bytes());
        for (key, value) in [
            (OrderBook::order_count_key()?, 0_u64.to_le_bytes().to_vec()),
            (OrderBook::order_key(1)?, order),
            (
                Self::settlement_key(self.terms.maker)?,
                lot_balance(self.terms.base_lots, 0),
            ),
            (
                Self::settlement_key(self.terms.taker)?,
                lot_balance(0, self.terms.quote_lots),
            ),
        ] {
            initialize_cell(&mut candidate, shared, &key, &value)?;
        }
        for (principal, role) in [(self.terms.maker, 1_u8), (self.terms.taker, 2_u8)] {
            let namespace = StorageNamespace::principal(program, principal);
            initialize_cell(&mut candidate, namespace, ROLE_KEY, &[role])?;
            initialize_cell(
                &mut candidate,
                namespace,
                &OrderBook::position_key()?,
                &lot_balance(0, 0),
            )?;
        }
        *storage = candidate;
        Ok(())
    }

    /// Authority required to place or match. Read-only queries need only
    /// `SharedStorageRead`.
    ///
    /// # Errors
    /// Refuses invalid capability encoding.
    pub fn capabilities() -> Result<CapabilitySet, PortRefusal> {
        Ok(CapabilitySet::new([
            Capability::StorageRead,
            Capability::StorageWrite,
            Capability::SharedStorageRead,
            Capability::SharedStorageWrite,
        ])?)
    }

    /// Produces the request bytes consumed by either executable order entry.
    ///
    /// # Errors
    /// Refuses zero or out-of-market quantities before transport.
    pub fn quantity_message(&self, quantity: u64) -> Result<Vec<u8>, PortRefusal> {
        if quantity == 0 || quantity > self.terms.base_lots {
            return Err(PortRefusal::OutOfRange);
        }
        Ok(quantity.to_le_bytes().to_vec())
    }

    /// Decodes a guest query response using the public order schema.
    ///
    /// # Errors
    /// Refuses malformed, foreign or out-of-bound order state.
    pub fn decode_order(&self, bytes: &[u8]) -> Result<Order, PortRefusal> {
        if bytes.len() != ORDER_BYTES as usize {
            return Err(PortRefusal::SchemaMismatch);
        }
        let word = |range: core::ops::Range<usize>| -> Result<[u8; 8], PortRefusal> {
            bytes
                .get(range)
                .and_then(|slice| slice.try_into().ok())
                .ok_or(PortRefusal::SchemaMismatch)
        };
        let id = u64::from_le_bytes(word(0..8)?);
        let price = u128::from_le_bytes(
            bytes[9..25]
                .try_into()
                .map_err(|_| PortRefusal::SchemaMismatch)?,
        );
        let quantity = u64::from_le_bytes(word(25..33)?);
        if id != 1
            || bytes[8] != 0
            || price != u128::from(self.terms.price)
            || quantity > self.terms.base_lots
        {
            return Err(PortRefusal::SchemaMismatch);
        }
        Ok(Order {
            id,
            is_buy: false,
            price,
            quantity,
        })
    }

    /// Immutable ABI-v2 deployment for this market.
    ///
    /// # Errors
    /// Refuses invalid key construction or an oversized module.
    pub fn deployment(&self, program: ProgramId) -> Result<Deploy, PortRefusal> {
        let wasm = self.code()?;
        Ok(Deploy {
            program,
            code_hash: crate::hash::sha256(&wasm),
            wasm,
            abi_version: ABI_V2_VERSION,
            upgrade_policy: UpgradePolicy::Immutable,
        })
    }

    /// Emits real scoped reads/writes. All host statuses and stored widths are
    /// checked; any trap rolls back every staged settlement cell.
    ///
    /// # Errors
    /// Refuses invalid key construction or an oversized module.
    pub fn code(&self) -> Result<Vec<u8>, PortRefusal> {
        let mut builder = ModuleBuilder::new(1);
        let host = builder.signature(&[I32; 5], &[I32]);
        let response = builder.signature(&[I32; 3], &[I32]);
        let entry = builder.signature(&[I32; 2], &[I32]);
        let reserve_type = builder.signature(&[I32], &[I32]);
        let read = builder.import(ABI_V2_MODULE, "storage_read_scoped", host);
        let write = builder.import(ABI_V2_MODULE, "storage_write_scoped", host);
        let respond = builder.import(ABI_V2_MODULE, "response_write", response);
        let cells = [
            (StorageSelector::Principal, ROLE_KEY.to_vec(), ROLE, 1),
            (
                StorageSelector::Shared,
                OrderBook::order_count_key()?,
                COUNT,
                8,
            ),
            (
                StorageSelector::Shared,
                OrderBook::order_key(1)?,
                ORDER,
                ORDER_BYTES,
            ),
            (
                StorageSelector::Shared,
                Self::settlement_key(self.terms.maker)?,
                MAKER,
                16,
            ),
            (
                StorageSelector::Shared,
                Self::settlement_key(self.terms.taker)?,
                TAKER,
                16,
            ),
            (
                StorageSelector::Principal,
                OrderBook::position_key()?,
                POSITION,
                16,
            ),
        ];
        let mut keys = Vec::new();
        let mut key_pointer = 0_u32;
        for (_, key, _, _) in &cells {
            builder.segment(key_pointer, key);
            keys.push((key_pointer, key.len() as i32));
            key_pointer += key.len() as u32;
        }
        let mut reserve = Code::new();
        reserve.local_get(0);
        reserve.i32_const(8);
        reserve.op(I32_GT_U);
        reserve.trap_if();
        reserve.pointer(INPUT);
        reserve.end();
        let reserve = builder.function(reserve_type, &[], &reserve);
        builder.export_function("layerx_reserve", reserve);
        for (name, matching) in [(PLACE_ORDER, false), (MATCH_ORDER, true)] {
            let mut code = Code::new();
            code.local_get(1);
            code.i32_const(8);
            code.op(I32_NE);
            code.trap_if();
            code.local_get(0);
            code.memory(I64_LOAD, 0);
            code.local_set(2);
            code.local_get(2);
            code.op(I64_EQZ);
            code.trap_if();
            code.local_get(2);
            code.i64_const(self.terms.base_lots as i64);
            code.op(I64_GT_U);
            code.trap_if();
            for ((selector, _, buffer, width), (key, length)) in cells.iter().zip(&keys) {
                emit_storage(&mut code, read, *selector, *key, *length, *buffer, *width);
                // A present scoped value returns its byte length plus one;
                // zero denotes absence. Buffer capacity is the value width.
                code.i32_const(*width + 1);
                code.op(I32_NE);
                code.trap_if();
            }
            code.pointer(ROLE);
            code.memory(I32_LOAD8_U, 0);
            code.i32_const(if matching { 2 } else { 1 });
            code.op(I32_NE);
            code.trap_if();
            // Validate persisted quantities before any arithmetic, including
            // the unaligned u128 price and complete order identity.
            check_word(&mut code, ORDER, 0, 1, true);
            code.pointer(ORDER);
            code.memory(I32_LOAD8_U, 8);
            code.trap_if();
            check_word(&mut code, ORDER, 9, self.terms.price, true);
            check_word(&mut code, ORDER, 17, 0, true);
            check_word(&mut code, ORDER, 25, self.terms.base_lots, false);
            for (buffer, offset, bound) in [
                (MAKER, 0, self.terms.base_lots),
                (MAKER, 8, self.terms.quote_lots),
                (TAKER, 0, self.terms.base_lots),
                (TAKER, 8, self.terms.quote_lots),
                (POSITION, 0, self.terms.base_lots),
                (POSITION, 8, self.terms.base_lots),
            ] {
                check_word(&mut code, buffer, offset, bound, false);
            }
            check_word(&mut code, COUNT, 0, u64::from(matching), true);
            emit_conservation(&mut code, self.terms);
            if matching {
                // Quantity cannot exceed resting inventory or the quote
                // balance. The immutable product bound prevents overflow.
                code.local_get(2);
                code.pointer(ORDER);
                code.memory(I64_LOAD, 25);
                code.op(I64_GT_U);
                code.trap_if();
                code.local_get(2);
                code.i64_const(self.terms.price as i64);
                code.op(I64_MUL);
                code.local_set(3);
                code.local_get(3);
                code.pointer(TAKER);
                code.memory(I64_LOAD, 8);
                code.op(I64_GT_U);
                code.trap_if();
                update_word(&mut code, ORDER, 25, 2, I64_SUB);
                update_word(&mut code, TAKER, 0, 2, I64_ADD);
                update_word(&mut code, TAKER, 8, 3, I64_SUB);
                update_word(&mut code, MAKER, 8, 3, I64_ADD);
                update_word(&mut code, POSITION, 0, 2, I64_ADD);
            } else {
                check_word(&mut code, ORDER, 25, 0, true);
                code.local_get(2);
                code.pointer(MAKER);
                code.memory(I64_LOAD, 0);
                code.op(I64_GT_U);
                code.trap_if();
                update_word(&mut code, MAKER, 0, 2, I64_SUB);
                code.pointer(ORDER);
                code.local_get(2);
                code.memory(I64_STORE, 25);
                code.pointer(COUNT);
                code.i64_const(1);
                code.memory(I64_STORE, 0);
                update_word(&mut code, POSITION, 8, 2, I64_ADD);
            }
            // Check all additions against their conserved allocation ceilings.
            for (buffer, offset, bound) in [
                (MAKER, 8, self.terms.quote_lots),
                (TAKER, 0, self.terms.base_lots),
                (POSITION, 0, self.terms.base_lots),
                (POSITION, 8, self.terms.base_lots),
            ] {
                check_word(&mut code, buffer, offset, bound, false);
            }
            emit_conservation(&mut code, self.terms);
            // Principal position is deliberately last: a denied final write
            // must roll back the order and both shared settlement balances.
            for index in [1_usize, 2, 3, 4, 5] {
                let (selector, _, buffer, width) = &cells[index];
                let (key, length) = keys[index];
                emit_storage(&mut code, write, *selector, key, length, *buffer, *width);
                code.trap_unless_ok();
            }
            emit_order_response(&mut code, respond);
            let function = builder.function(entry, &[(2, I64)], &code);
            builder.export_function(name, function);
        }
        let mut query = Code::new();
        query.local_get(1);
        query.trap_if();
        let (key, length) = keys[2];
        emit_storage(
            &mut query,
            read,
            StorageSelector::Shared,
            key,
            length,
            ORDER,
            ORDER_BYTES,
        );
        query.i32_const(ORDER_BYTES + 1);
        query.op(I32_NE);
        query.trap_if();
        check_word(&mut query, ORDER, 0, 1, true);
        query.pointer(ORDER);
        query.memory(I32_LOAD8_U, 8);
        query.trap_if();
        check_word(&mut query, ORDER, 9, self.terms.price, true);
        check_word(&mut query, ORDER, 17, 0, true);
        check_word(&mut query, ORDER, 25, self.terms.base_lots, false);
        emit_order_response(&mut query, respond);
        let query = builder.function(entry, &[], &query);
        builder.export_function(READ_ORDER, query);
        builder.export_memory("memory");
        let wasm = builder.finish();
        if wasm.len() as u64 > layerx_programs_runtime::limits::DEFAULT_MAX_MODULE_BYTES {
            return Err(PortRefusal::ModuleTooLarge);
        }
        Ok(wasm)
    }
}

fn lot_balance(base: u64, quote: u64) -> Vec<u8> {
    [base.to_le_bytes(), quote.to_le_bytes()].concat()
}

fn initialize_cell(
    storage: &mut Storage,
    namespace: StorageNamespace,
    key: &[u8],
    value: &[u8],
) -> Result<(), PortRefusal> {
    let mut transaction = storage.transaction(namespace);
    if transaction.read(key)?.is_some() {
        return Err(PortRefusal::OutOfRange);
    }
    transaction.write(key, value)?;
    let _ = transaction.commit();
    Ok(())
}

fn emit_storage(
    code: &mut Code,
    host: u32,
    selector: StorageSelector,
    key: u32,
    length: i32,
    buffer: u32,
    width: i32,
) {
    code.i32_const(selector as i32);
    code.pointer(key);
    code.i32_const(length);
    code.pointer(buffer);
    code.i32_const(width);
    code.call(host);
}

fn check_word(code: &mut Code, buffer: u32, offset: u32, bound: u64, exact: bool) {
    code.pointer(buffer);
    code.memory(I64_LOAD, offset);
    code.i64_const(bound as i64);
    code.op(if exact { I64_NE } else { I64_GT_U });
    code.trap_if();
}

fn update_word(code: &mut Code, buffer: u32, offset: u32, local: u32, operation: u8) {
    code.pointer(buffer);
    code.pointer(buffer);
    code.memory(I64_LOAD, offset);
    code.local_get(local);
    code.op(operation);
    code.memory(I64_STORE, offset);
}

fn emit_conservation(code: &mut Code, terms: OrderBookTerms) {
    code.pointer(MAKER);
    code.memory(I64_LOAD, 0);
    code.pointer(TAKER);
    code.memory(I64_LOAD, 0);
    code.op(I64_ADD);
    code.pointer(ORDER);
    code.memory(I64_LOAD, 25);
    code.op(I64_ADD);
    code.i64_const(terms.base_lots as i64);
    code.op(I64_NE);
    code.trap_if();
    code.pointer(MAKER);
    code.memory(I64_LOAD, 8);
    code.pointer(TAKER);
    code.memory(I64_LOAD, 8);
    code.op(I64_ADD);
    code.i64_const(terms.quote_lots as i64);
    code.op(I64_NE);
    code.trap_if();
}

fn emit_order_response(code: &mut Code, respond: u32) {
    code.i32_const(0);
    code.pointer(ORDER);
    code.i32_const(ORDER_BYTES);
    code.call(respond);
    code.trap_unless_ok();
    code.i32_const(0);
    code.end();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn order_count_is_shared() {
        assert_eq!(OrderBook::order_count_binding(), StateBinding::Shared);
        assert!(OrderBook::order_count_is_shared());
        assert!(OrderBook::order_count_portable());
    }

    #[test]
    fn orders_are_shared() {
        assert_eq!(OrderBook::orders_binding(), StateBinding::Shared);
        assert!(OrderBook::orders_is_shared());
        assert!(OrderBook::orders_portable());
    }

    #[test]
    fn positions_are_sender_indexed() {
        assert_eq!(OrderBook::positions_binding(), StateBinding::SenderIndexed);
        assert!(!OrderBook::positions_is_shared());
        assert!(OrderBook::positions_portable());
    }

    #[test]
    fn order_count_key_derivation() {
        let key = OrderBook::order_count_key()
            .unwrap_or_else(|error| panic!("shared order book: {error}"));
        // Item key is the namespace verbatim
        assert_eq!(key, ORDER_COUNT.as_bytes());
    }

    #[test]
    fn order_key_derivation() {
        let key =
            OrderBook::order_key(42).unwrap_or_else(|error| panic!("shared order book: {error}"));
        // Map key is length-prefixed namespace + key
        assert!(!key.is_empty());
        // Verify it uses map_key composition
        let expected = map_key(ORDERS, &42u64.to_le_bytes())
            .unwrap_or_else(|error| panic!("shared order book: {error}"));
        assert_eq!(key, expected);
    }

    #[test]
    fn position_key_collapses() {
        let key =
            OrderBook::position_key().unwrap_or_else(|error| panic!("shared order book: {error}"));
        // SenderIndexed collapses onto the map prefix
        // which is length-prefixed namespace
        assert!(!key.is_empty());
    }

    #[test]
    fn all_bindings_portable() {
        // All three bindings are now portable, including shared state
        assert!(OrderBook::order_count_portable());
        assert!(OrderBook::orders_portable());
        assert!(OrderBook::positions_portable());
    }

    #[test]
    fn shared_binding_addresses_shared_namespace() {
        // Shared bindings explicitly indicate shared namespace
        let binding = StateBinding::Shared;
        assert!(binding.shared());
        assert!(binding.portable());

        // And produce valid keys
        let key = binding
            .layerx_key(ORDER_COUNT, &[])
            .unwrap_or_else(|error| panic!("shared order book: {error}"));
        assert_eq!(key, ORDER_COUNT.as_bytes());
    }
}

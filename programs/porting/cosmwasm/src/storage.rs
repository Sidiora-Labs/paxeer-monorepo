//! `cw-storage-plus` raw keys carried onto namespaced storage.
//!
//! `cw-storage-plus` composes a raw key from a namespace and the typed key:
//! an [`Item`] writes its namespace bytes and nothing else, and a [`Map`]
//! writes a two-byte big-endian length, then the namespace, then the key. Both
//! rules are reproduced here exactly, so an exported contract state dump can be
//! located key for key.
//!
//! A `LayerX` program has two disjoint scopes: `(program, principal)` for
//! caller-owned state, and `(program)` for state shared by all callers. Key
//! bytes alone do not select a scope. Preserve a global `Item` by using
//! `StateBinding::Shared.layerx_key("order_count", &[])` in shared scope;
//! preserve a shared `Map` with `map_key("orders", &id.to_le_bytes())` in that
//! same scope. `StateBinding::Item` deliberately selects a per-principal copy;
//! it is appropriate only when that isolation is intended. Sender-indexed and
//! sender-suffixed maps use principal scope and omit the authenticated sender
//! from their raw keys.
//!
//! Guests select `StorageSelector::Shared` (selector 2) with ABI-v2
//! `storage_read_scoped`, `storage_write_scoped` and `storage_delete_scoped`,
//! under separate `SharedStorageRead` and `SharedStorageWrite` grants. Principal
//! scope uses `StorageSelector::Principal` (selector 1) and `StorageRead` /
//! `StorageWrite` grants. The host fixes the owning program and principal;
//! neither can be supplied in key bytes. See [`crate::shared_orderbook`] for
//! executable shared reads and atomic matching alongside principal positions.
//!
//! [`Item`]: https://docs.rs/cw-storage-plus
//! [`Map`]: https://docs.rs/cw-storage-plus

use layerx_programs_runtime::storage::MAX_STORAGE_KEY_BYTES;

use crate::error::PortRefusal;

/// Longest namespace the two-byte length prefix can carry.
pub const MAX_NAMESPACE_BYTES: usize = 65_535;

/// Returns the raw key an [`Item`] occupies, which is its namespace verbatim.
///
/// # Errors
///
/// Refuses an empty or oversized namespace and a key beyond the storage bound.
///
/// [`Item`]: https://docs.rs/cw-storage-plus
pub fn item_key(namespace: &str) -> Result<Vec<u8>, PortRefusal> {
    check_namespace(namespace)?;
    bounded(namespace.as_bytes().to_vec())
}

/// Returns the length-prefixed namespace a [`Map`] writes before every key.
///
/// # Errors
///
/// Refuses an empty or oversized namespace and a prefix beyond the storage
/// bound.
///
/// [`Map`]: https://docs.rs/cw-storage-plus
pub fn map_prefix(namespace: &str) -> Result<Vec<u8>, PortRefusal> {
    check_namespace(namespace)?;
    let mut prefix = Vec::with_capacity(namespace.len().saturating_add(2));
    push_length_prefixed(namespace.as_bytes(), &mut prefix)?;
    bounded(prefix)
}

/// Returns the raw key a [`Map`] entry occupies.
///
/// # Errors
///
/// Refuses an empty or oversized namespace and a key beyond the storage bound.
///
/// [`Map`]: https://docs.rs/cw-storage-plus
pub fn map_key(namespace: &str, key: &[u8]) -> Result<Vec<u8>, PortRefusal> {
    let mut composed = map_prefix(namespace)?;
    composed.extend_from_slice(key);
    bounded(composed)
}

/// Returns the raw key a [`Map`] with a composite key occupies. Every element
/// but the last is written length-prefixed, exactly as `cw-storage-plus`
/// nests namespaces, and the last element is written raw.
///
/// # Errors
///
/// Refuses an empty or oversized namespace, an oversized key element and a key
/// beyond the storage bound.
///
/// [`Map`]: https://docs.rs/cw-storage-plus
pub fn composite_map_key(
    namespace: &str,
    leading: &[&[u8]],
    last: &[u8],
) -> Result<Vec<u8>, PortRefusal> {
    let mut composed = map_prefix(namespace)?;
    for element in leading {
        push_length_prefixed(element, &mut composed)?;
    }
    composed.extend_from_slice(last);
    bounded(composed)
}

/// What one piece of contract state becomes after the port.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum StateBinding {
    /// An intentionally principal-scoped `Item`. Use `Shared` instead to
    /// preserve a source-global cell; the donation reference pins its global
    /// deployment configuration into the module.
    Item,
    /// A `Map` whose key is `info.sender`. The key half is already carried by
    /// the namespace, so the entry collapses onto the map's namespace prefix.
    SenderIndexed,
    /// A `Map` whose key is a composite ending in `info.sender`. The trailing
    /// key half collapses; the leading elements stay in the key.
    SenderSuffixed,
    /// A global `Item` or a `Map` any account must be able to read. Selects
    /// the program-shared namespace `(program)`. `layerx_key` returns an Item
    /// key; compose Map entries with `map_key` in that same scope.
    Shared,
}

impl StateBinding {
    /// Returns the namespaced-storage key the binding occupies.
    ///
    /// # Errors
    ///
    /// Refuses whatever the key composition refuses.
    pub fn layerx_key(self, namespace: &str, leading: &[&[u8]]) -> Result<Vec<u8>, PortRefusal> {
        match self {
            Self::Item | Self::Shared => item_key(namespace),
            Self::SenderIndexed => map_prefix(namespace),
            Self::SenderSuffixed => composite_map_key(namespace, leading, &[]),
        }
    }

    /// Returns whether the binding addresses the shared namespace instead of
    /// the principal-scoped namespace.
    #[must_use]
    pub const fn shared(self) -> bool {
        matches!(self, Self::Shared)
    }

    /// Returns whether the binding can be carried over at all.
    #[must_use]
    pub const fn portable(self) -> bool {
        matches!(
            self,
            Self::Item | Self::SenderIndexed | Self::SenderSuffixed | Self::Shared
        )
    }
}

/// One holder of sender-indexed state in a migration plan.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StateHolder {
    /// The address bytes `cw-storage-plus` wrote into the raw key, which is
    /// the canonical form of the bech32 address.
    pub address: Vec<u8>,
    /// The principal whose namespace holds the ported cell.
    pub principal: [u8; 32],
}

/// One live entry located in a `CosmWasm` state dump and the namespaced cell it
/// becomes.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MigrationCell {
    /// The raw key the entry occupies in the contract's own store.
    pub cosmwasm_key: Vec<u8>,
    /// The key the ported entry occupies in `LayerX` namespaced storage.
    pub layerx_key: Vec<u8>,
    /// The principal whose namespace holds the ported entry.
    pub principal: [u8; 32],
}

/// Builds the import plan for a `Map` keyed by `info.sender`: one cell per
/// holder, naming the raw key to read from a state dump and the collapsed key
/// to write, in that holder's own namespace.
///
/// Every holder's entry collapses onto the *same* key, because the part of the
/// key that distinguished them is the sender address and the namespace already
/// carries the principal. The cells do not collide: each one is written in a
/// different namespace.
///
/// # Errors
///
/// Refuses an empty address, the reserved zero principal, and any namespace or
/// key the declared bounds reject.
pub fn sender_indexed_import(
    namespace: &str,
    holders: &[StateHolder],
) -> Result<Vec<MigrationCell>, PortRefusal> {
    let layerx_key = map_prefix(namespace)?;
    let mut plan = Vec::with_capacity(holders.len());
    for holder in holders {
        if holder.address.is_empty() {
            return Err(PortRefusal::EmptyAddress);
        }
        if holder.principal == [0u8; 32] {
            return Err(PortRefusal::EmptyAddress);
        }
        plan.push(MigrationCell {
            cosmwasm_key: map_key(namespace, &holder.address)?,
            layerx_key: layerx_key.clone(),
            principal: holder.principal,
        });
    }
    Ok(plan)
}

fn check_namespace(namespace: &str) -> Result<(), PortRefusal> {
    if namespace.is_empty() || namespace.len() > MAX_NAMESPACE_BYTES {
        return Err(PortRefusal::InvalidNamespace);
    }
    Ok(())
}

fn push_length_prefixed(element: &[u8], out: &mut Vec<u8>) -> Result<(), PortRefusal> {
    let length = u16::try_from(element.len()).map_err(|_| PortRefusal::InvalidNamespace)?;
    out.extend_from_slice(&length.to_be_bytes());
    out.extend_from_slice(element);
    Ok(())
}

fn bounded(key: Vec<u8>) -> Result<Vec<u8>, PortRefusal> {
    if key.is_empty() || key.len() > MAX_STORAGE_KEY_BYTES {
        return Err(PortRefusal::KeyTooLong);
    }
    Ok(key)
}

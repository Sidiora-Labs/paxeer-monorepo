<!--
order: 2
-->

# State

The capability keeper separates durable owner identities from process-local
capability references. The persistent store key is `capability`; the memory
store key is `mem_capability`. Key constructors are defined in
[types/keys.go](../types/keys.go), and reads and writes are implemented in
[keeper/keeper.go](../keeper/keeper.go).

## Index

| Persistent key | Value |
| --- | --- |
| UTF-8 `index` | Next capability index, an eight-byte big-endian `uint64` |

The default genesis index is `1`. `InitializeIndex` requires a nonzero value and
an unset index. `NewCapability` uses the current index and increments it after
recording the initial owner. Releasing a capability does not decrement the
index or reuse its number.

## CapabilityOwners

| Persistent key | Value |
| --- | --- |
| UTF-8 `capability_index` concatenated with the eight-byte big-endian index | Protobuf `CapabilityOwners` |

`CapabilityOwners.Owners` contains `Owner{Module, Name}` pairs. The runtime
`Set` helper maintains lexicographic ordering by `Owner.Key()`, the string
`module/name`, and rejects an already present key with `ErrOwnerClaimed`.
`Get` and `Remove` use that ordering for binary search.

`NewCapability` rejects an existing name in its module's reverse mapping.
Owner-set insertion rejects duplicate owner keys within that capability;
`ClaimCapability` itself does not check whether the requested module/name is
already mapped to a different index before writing the reverse mapping.
Callers must preserve the one-name-to-one-index mapping within their scope.
Names can be shared across different module scopes.

[GenesisState.Validate](../types/genesis.go) requires a nonzero next index,
nonempty owner lists, existing indices in `[1, Index)`, and nonblank owner
module and name strings. It does not validate owner sorting, duplicate genesis
indices, duplicate owner keys, or module/name uniqueness across indices.
`SetOwners` stores the supplied list directly; it does not sort or deduplicate
it. Imported genesis therefore needs canonical, unambiguous owner lists before
initialization; basic genesis validation alone does not establish those rules.

## Capability

`Capability` contains an `Index`, but authentication uses the process-local
reference's forward mapping. The keeper's shared `capMap` interns one canonical
`*Capability` per index for its lifetime. All owners of an index retrieve that
same reference in the running process.

| Memory key | Value |
| --- | --- |
| `FwdCapabilityKey(module, capability)` | Capability name as UTF-8 bytes |
| `RevCapabilityKey(module, name)`, formatted as `module/rev/name` | Eight-byte big-endian index |
| UTF-8 `mem_initialized` | Byte `0x01` once reconstruction has completed |

The forward key has the `module/fwd/0x` prefix followed by the pointer-derived
string encoded by `FwdCapabilityKey`. That helper formats the pointer with
`%#010p` and keeps its final ten characters when longer. Use the constructor
rather than persisting or recreating this process-dependent key.
`GetCapability` resolves the reverse index through `capMap`;
`AuthenticateCapability` compares the forward lookup's name with the requested
name. A serialized index is not a replacement for an authenticated reference.

### Reconstruction and release

`InitMemStore` requires an SDK memory store. On its first call it scans the
persistent `capability_index` prefix, decodes each owner set, interns the
capability by index, and installs both mappings for every owner. It then sets
`mem_initialized`, so subsequent calls are no-ops. Initialization runs from
`InitGenesis` and from `BeginBlocker` after a restart; see the
[overview](README.md#initialization) for application setup.

`ReleaseCapability` deletes the releasing scope's forward and reverse mappings
and removes its owner pair from persistent state. It deletes the persistent
owner record when no owners remain. The index may remain interned in `capMap`,
but that entry alone grants no ownership: the owner record and memory mappings
control lookup and authentication. A restart rebuilds references from the
remaining persisted owners; pointer addresses themselves are never exported.

### Genesis import and export

[genesis.go](../genesis.go) imports `GenesisState.Index` and each
`GenesisOwners{Index, IndexOwners}` record before rebuilding memory state.
Export reads the next index, visits indices from `1` up to that value exclusively,
and includes only existing, nonempty owner sets. It preserves gaps left by
released capabilities and exports neither memory keys nor pointer identities.

<!--
order: 0
title: Capability Overview
parent:
  title: "capability"
-->

# `capability`

## Overview

`x/capability` implements multi-owner capabilities for modules in the merged
Paxeer node, following the Cosmos SDK ADR 003 model. A scoped keeper lets a module
create capabilities, claim references shared with it, authenticate a reference
against a local name, retrieve owned capabilities and release ownership.

Persistent state holds a monotonically allocated index and the owner tuples
`(module, name)` for each capability. Memory state holds forward mappings from
module and capability reference to name, reverse mappings from module and name
to index, and a shared map from index to a canonical `*Capability`. References
are local to a running keeper; persisted owner identities allow their mappings
to be reconstructed after restart. See [State](02_state.md) for the exact keys,
owner ordering and genesis validation boundary.

The application creates a scoped keeper for each participating module and
passes it to that module. Lookup and authentication use the scoped module name.
The global keeper's ability to create scopes stays with the application.
The module provides no CLI transaction or query commands and no query service,
but it does implement genesis defaults, validation, import and export through
[types/genesis.go](../types/genesis.go), [genesis.go](../genesis.go) and
[module.go](../module.go).

## Initialization

Create the keeper with `keeper.NewKeeper`, a codec, a persistent store key, and
a memory store key. The actual setup in [node/app.go](../../../../node/app.go)
uses the `capabilitykeeper` package alias:

```go
import (
  capabilitykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/keeper"
  capabilitytypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/types"
)

// With appCodec, keys and memKeys supplied by application initialization:
app.CapabilityKeeper = capabilitykeeper.NewKeeper(
  appCodec,
  keys[capabilitytypes.StoreKey],
  memKeys[capabilitytypes.MemStoreKey],
)
```

The node creates scopes for IBC, IBC transfer and Wasm with `ScopeToModule` and
passes them to their module keepers. Scoping panics on a blank module name, a
duplicate module name or an already sealed keeper. Applications can call
`Seal()` after creating all scopes to prevent further scope creation; the
current node setup does not call it. Calling `Seal()` twice also panics.

Construction cannot populate the memory store because it has no SDK context.
The application must mount an SDK memory store for `mem_capability` and initialize
it before capability operations:

1. At genesis, `InitGenesis` installs the next index and persisted owner sets,
   then calls `InitMemStore`. Default genesis has index `1` and no owners.
2. On restart, `BeginBlocker` calls `InitMemStore` to reconstruct references and
   mappings from persistent owners loaded from the committed store. Recreate the
   keeper and module scopes, then initialize memory before using those scopes;
   do not reimport genesis into an already initialized persistent store. The node
   invokes capability initialization before downstream IBC begin-block processing in
   [legacyabci/begin_block.go](../../../../node/legacyabci/begin_block.go).
3. Later calls return without reconstructing state once the memory store's
   initialization flag is present.

`ExportGenesis` emits `GenesisState{Index, Owners}`: the next allocation index and
nonempty `GenesisOwners{Index, IndexOwners}` records. Each owner is a persisted
`Owner{Module, Name}` identity. Export preserves capability indices and gaps left
by released capabilities; it exports neither memory mappings nor process-local
references. Importing the exported state through `AppModule.InitGenesis` decodes
the JSON with the application codec and delegates to `InitGenesis` above.

`AppModuleBasic.ValidateGenesis` decodes the JSON and calls `GenesisState.Validate`.
Validation requires a nonzero next index, nonempty owner sets, unique capability
indices in `[1, Index)`, nonblank module/name identities, unique module/name
bindings across all indices, and owners strictly ordered by `Owner.Key()`.
`InitGenesis` itself does not call validation: validate external genesis before
importing it, since `SetOwners` stores the supplied lists directly.

After import or restart, modules must use `ScopedKeeper.GetCapability` to obtain
the reconstructed canonical `*Capability`. Owners of the same index receive the
same pointer in that keeper. An old pointer or a newly allocated `Capability`
with the same index does not carry the reconstructed forward mapping and cannot
authenticate in place of that reference. Durable identity is the index and owner
tuple; pointer addresses belong only to the running keeper.

## Contents

1. **[Concepts](01_concepts.md)**
2. **[State](02_state.md)**

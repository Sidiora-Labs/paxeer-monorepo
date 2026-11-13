<!-- Source: https://docs.paxeer.app/modules/store/ -->

# Store Module

A single cached-context helper in `modules/store/` — not a registered chain module.

**Source:** `Paxeer-X-Network/modules/store/handler.go`

## Overview

`modules/store/` contains exactly one file, `handler.go` (11 lines), exporting one function. It has no `module.go`, keeper, types, genesis, messages, or queries, and it is not registered in `node/app.go`. `modules/README.md` lists it only as "module-level store integration helpers".

## GetCachedContext

```
package store

import (
    sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func GetCachedContext(ctx sdk.Context) (sdk.Context, sdk.CacheMultiStore) {
    ms := ctx.MultiStore()
    msCache := ms.CacheMultiStore()
    return ctx.WithMultiStore(msCache), msCache
}
```

`GetCachedContext` wraps the context's multistore in a `CacheMultiStore` and returns both the derived context and the cache. Writes made through the derived context stay in the cache until the caller writes the cache through; discarding the cache discards the writes. This is the standard SDK pattern for speculative execution.

## What it is not

- No key-formatting or prefix constants, iterator utilities, or codec helpers are defined here. Modules define their own prefixes and call the SDK `KVStore` and codec directly.

- No state-migration utilities or test helpers (in-memory stores, mock contexts, snapshots) live in this package.

- It is not a layer over [PaxDB storage](https://docs.paxeer.app/storage); module keepers access state through the SDK multistore without going through this package.

## Next Steps

- [Understand PaxDB storage architecture](https://docs.paxeer.app/storage)

- [Review other Paxeer modules](https://docs.paxeer.app/modules)

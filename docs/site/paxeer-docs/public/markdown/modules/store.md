<!-- Source: https://docs.paxeer.app/modules/store/ -->

# Store Module

A single cached-context helper in `modules/store/` — not a registered chain module.

**Source:** `modules/store/handler.go`

## Overview

`modules/store/` provides `handler.go`, which exports the cached-context helper. Keepers and module lifecycle registration live in `node/app.go`. `modules/README.md` describes this package as "module-level store integration helpers".

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

## Using the cached context

- Modules define their own prefixes and use the SDK `KVStore` and codec through the returned context.

- Retain the returned cache when the caller needs to commit the buffered writes.

- The SDK multistore supplies the cache used for speculative execution over [PaxDB storage](https://docs.paxeer.app/storage).

## Next Steps

- [Understand PaxDB storage architecture](https://docs.paxeer.app/storage)

- [Review other Paxeer modules](https://docs.paxeer.app/modules)

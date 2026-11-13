# Asset read authority

[`px_listAssets`](../../../rpc/px_assets.go) reads one registration-ordered page
from `/v1/assets` on the indexer explicitly configured by `[evm].px_indexer_url`
or `rpc.WithPxIndexerURL`. Each response joins at most sixteen records against
one committed node height and returns `order: registration`, `joined_limit: 16`
and `next_cursor`. Pass a non-null cursor to fetch the next page; cursors are
canonical positive decimal row IDs.

Bank denoms obtain metadata and exact supply through the bank precompile, with
display decimals read from the bank metadata at the same height. Native ledger
records obtain `getAsset` through the ledger precompile, whose keeper
authenticates committed kernel state produced by this module's
[`lx_asset_registry.c`](lx_asset_registry.c) and
[`lx_asset_state.c`](lx_asset_state.c). A successful ledger join includes
`paused` and `verification=authenticated_committed_snapshot`. A failed metadata
read leaves `name`, `symbol`, `decimals` and `supply` null and omits `paused` and
`verification`. A missing or invalid indexer returns an error.

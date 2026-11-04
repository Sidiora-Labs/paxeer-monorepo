# Asset read authority

`px_listAssets` enumerates `/v1/assets` on the indexer explicitly configured by
`[evm].px_indexer_url` or `rpc.WithPxIndexerURL`. It sorts the complete paginated enumeration by asset ID
and joins at most sixteen records against one committed node height.

Bank denoms obtain metadata and exact supply through the bank precompile.
Native ledger records obtain `getAsset` through the ledger precompile, whose
keeper authenticates committed kernel state produced by this module's
`lx_asset_registry.c` and `lx_asset_state.c`. A successful ledger join includes
`paused` and `verification=authenticated_committed_snapshot`. A failed metadata
read leaves all four metadata fields null and omits those optional fields.
A missing or invalid indexer is an explicit error, never an empty registry.

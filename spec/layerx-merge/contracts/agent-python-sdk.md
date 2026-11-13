# Agent Python SDK contract

## px

[`layerx_sdk.px`](../../../services/agent/sdk/python/layerx_sdk/px.py) provides
joined account, asset, network and history reads through `PxEndpoint`.

`decode_px_network_head` reads the network ID, Paxeer chain ID and latest block,
and the consensus latest height as `latest_committed_height`. The consensus height must
not exceed the node's latest block. Missing consensus evidence is unavailable.
The decoder refuses an `anchor` member; `PxAnchorHead` is not exported.

`history_cursor(block, transaction, log)` encodes a block position as 32 lowercase
hexadecimal characters: 16 for the block height and 8 each for transaction and
log indexes. `history_params` passes that cursor in the history request.
For block 7, transaction 2 and log 1 the cursor is
`00000000000000070000000200000001`.

Paxeer mainnet-beta uses EVM chain ID `125` (`0x7d`). Block commit establishes
finality; joined history and network labels do not replace receipt verification.

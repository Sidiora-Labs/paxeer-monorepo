# CosmWasm bindings

This package connects CosmWasm contracts to native modules in the merged Paxeer network through custom queries and messages. [`RegisterCustomPlugins`](wasm.go) installs the query and message handlers in the Wasm keeper.

Queries ([`queries.go`](queries.go), [`query_plugin.go`](query_plugin.go)):

- Oracle: exchange rates and TWAPs
- Epoch: the current epoch
- Token Factory: denom authority metadata (`denom_authority_metadata`), denoms from a creator (`denoms_from_creator`), and pause state (`denom_paused`)
- EVM: static calls; ERC-20, ERC-721 and ERC-1155 helpers; Pax and EVM address mapping; interface support checks
- Staking extension: unbonding delegations

Custom queries use a JSON wrapper with `route` and `query_data`. The supported route values are `oracle`, `epoch`, `tokenfactory`, `evm`, and `stakingext`. Select one query variant inside `query_data`.

Messages ([`encoder.go`](encoder.go), [`message_plugin.go`](message_plugin.go)):

- Token Factory: `create_denom`, `mint_tokens`, `burn_tokens`, `change_admin`, `set_metadata`, and `set_denom_paused` (pause or unpause)
- EVM: `call_evm` and `delegate_call_evm`, dispatched as `MsgInternalEVMCall` and `MsgInternalEVMDelegateCall`

Custom messages select one operation by its top-level JSON key, with the operation's fields inside its value. Multiple non-null operation keys are rejected. An empty object or an object containing only null operation values returns an unsupported-message error.

## Token Factory pause bindings

Query a denom's pause state through the `tokenfactory` route:

```json
{
  "route": "tokenfactory",
  "query_data": {
    "denom_paused": { "denom": "factory/<creator>/<subdenom>" }
  }
}
```

The response uses `QueryDenomPausedResponse`: `paused` is a boolean and `issuer_did` identifies the bound issuer, if any. The handler marshals with Go's `encoding/json`, so `paused: false` and an empty `issuer_did` are omitted. An unpaused denom without an issuer therefore returns `{}`. An empty denom returns gRPC `InvalidArgument`; missing authority metadata returns `NotFound`. See the [query bindings](../../modules/tokenfactory/client/wasm/bindings/queries.go) and [query handler](../../modules/tokenfactory/client/wasm/query.go).

Pause a denom with a custom message:

```json
{
  "set_denom_paused": {
    "denom": "factory/<creator>/<subdenom>",
    "paused": true
  }
}
```

Set `paused` to `false` to unpause. Both a non-empty `denom` and an explicit boolean `paused` are required by the [message binding](../../modules/tokenfactory/client/wasm/bindings/msgs.go). The [encoder](../../modules/tokenfactory/client/wasm/encoder.go) creates `MsgSetDenomPaused` with the calling contract's address as `Sender`; a payload cannot override that sender.

The [Token Factory message handler](../../modules/tokenfactory/keeper/msg_server.go) requires the sender to be the denom's admin and rejects issuer-bound denoms with `ErrIssuerMismatch`. Issuer-bound pause changes use the keeper's `SetDenomPausedByIssuer` path. Encoding a message does not bypass these module checks.

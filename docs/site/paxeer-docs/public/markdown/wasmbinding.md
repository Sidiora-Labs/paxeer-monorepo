<!-- Source: https://docs.paxeer.app/wasmbinding/ -->

# WASM Bindings

Use Paxeer custom messages and queries from CosmWasm, including token creation, EVM calls, address association, and native module reads.

**Source:** `node/wasmbinding/` at the repository root and each module's `client/wasm/` package.

## How bindings are registered

`RegisterCustomPlugins` installs a custom message encoder and query router in the CosmWasm keeper. These bindings extend standard bank, staking, distribution, and Wasm messages. The package is Go node code; it is not a Rust crate to add to `Cargo.toml`.

Contract-side types must serialize to the JSON forms below and implement the appropriate CosmWasm custom message or query traits. Use the schemas in this source revision when generating types or evaluating a client library.

## Custom messages

`node/wasmbinding/encoder.go` dispatches these top-level message variants. Supply one operation per message.

| Variant | Payload and behavior |
| --- | --- |
| `create_denom` | `subdenom`; creates a tokenfactory denomination under the calling contract. |
| `mint_tokens` | `amount` coin object; subject to the denomination's mint authority. |
| `burn_tokens` | `amount` coin object; executes the native tokenfactory burn path. |
| `change_admin` | `denom` and `new_admin_address`; requires the current denomination administrator. |
| `set_metadata` | Bank `metadata` object for the denomination. |
| `set_denom_paused` | `denom` and an explicit `paused` boolean; a missing or null boolean is refused. |
| `call_evm` | `to`, decimal-string integer `value`, and base64 `data`; dispatches an internal EVM call. |
| `delegate_call_evm` | `to` and base64 `data`; carries calling-contract and code-hash context into the EVM delegate-call path. |

```
// JSON payload of a custom message, emitted by a CosmWasm contract:
{ "create_denom": { "subdenom": "credits" } }
```

The tokenfactory encoder and ordinary EVM-call encoder supply the contract as sender. The delegate-call encoder uses the incoming message sender when it is a valid Paxeer address and carries the emitting contract and code hash separately. ABI calldata bytes use base64 in the custom-message JSON; Ethereum `0x` hex strings must be decoded and re-encoded before use.

## Custom queries

`PaxQueryWrapper` selects a route and embeds the route-specific JSON query in `query_data`. Unknown routes and unknown variants return errors.

| Route | Implemented query families |
| --- | --- |
| `evm` | Static calls; ERC-20, ERC-721, and ERC-1155 reads; transfer and approval payload generation; interface support; EVM/Paxeer address lookups. |
| `tokenfactory` | `denom_authority_metadata`, `denoms_from_creator`, and `denom_paused` (which reports the pause state and issuer DID for a denomination). |
| `epoch` | `epoch` state. |
| `stakingext` | `unbonding_delegations` for a supplied delegator. |
| `oracle` | `exchange_rates` and `oracle_twaps` through the retained native Oracle handler. |

```
// Query the current epoch from inside a CosmWasm contract:
{
  "route": "epoch",
  "query_data": { "epoch": {} }
}
```

This is a custom querier payload, not an HTTP JSON-RPC request. It must be issued through the contract's CosmWasm query interface.

## Address lookup and token queries

The EVM route exposes `get_pax_address` with `evm_address`, and `get_evm_address` with `pax_address`. Responses contain the resolved address and an `associated` flag. Check that flag before using the result as a linked account.

ERC query fields are defined in `modules/evm/client/wasm/bindings/queries.go`. A transfer-payload query only produces encoded calldata; it does not transfer assets. Send the resulting calldata through a supported execution path when a transaction is intended.

Keep native coin integers, ERC token decimals, and EVM wei values separate. Do not format a bank base-denomination amount using an unrelated ERC token's display precision.

## Execution boundaries

CosmWasm-to-EVM calls are implemented. The corresponding [wasmd precompile](https://docs.paxeer.app/wasm) supports EVM-to-CosmWasm calls, with context and pointer checks. A non-query CosmWasm → EVM → CosmWasm loop is rejected; delegate-call messages should not be treated as unrestricted arbitrary execution.

The native Oracle querier exposes exchange rates and time-weighted prices through this route. Check available denominations and observation freshness before using price data. See the [Oracle module guide](https://docs.paxeer.app/modules/oracle).

## Implementation references

- `node/wasmbinding/wasm.go`: plugin registration.

- `node/wasmbinding/encoder.go` and `message_plugin.go`: custom variants and module dispatch.

- `node/wasmbinding/query_plugin.go` and `queries.go`: query wrapper, route selection, and handlers.

- `modules/evm/client/wasm/encoder.go`: base64 decoding and internal EVM message construction.

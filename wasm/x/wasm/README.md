# Wasm Module

The CosmWasm smart contract module of the Paxeer X chain, vendored from upstream [wasmd](https://github.com/CosmWasm/wasmd).
`node/app.go` wires it into `paxd`, and EVM contracts reach it through the `wasmd` precompile in
[`precompiles/wasmd`](../../../precompiles/wasmd).

## Configuration

`paxd` reads the following keys from the `[wasm]` section of `config/app.toml` (see `ReadWasmConfig` in `module.go`):

```toml
[wasm]
# This is the maximum sdk gas (wasm and storage) that we allow for any x/wasm "smart" queries
query_gas_limit = 300000
# This defines the memory size for Wasm modules that we can keep cached to speed-up instantiation
# The value is in MiB not bytes
memory_cache_size = 100
# A quoted positive decimal SDK gas limit, or empty for the consensus block limit
simulation_gas_limit = ""
```

When a key is not set, `query_gas_limit` defaults to 3000000 and `memory_cache_size` to 100 MiB.
The node template uses a query limit of 300000 and a memory cache of 100 MiB
(see [`app_config.go`](../../../daemon/paxd/cmd/app_config.go)).

`simulation_gas_limit` is a string in both the node template and the start flag.
For example, `simulation_gas_limit = "987654"` sets the simulation gas meter to
987654 SDK gas. An absent key or `""` uses the consensus maximum block gas when
positive; without a positive block maximum, the existing simulation meter is
retained. `"0"` is rejected, as are negative, fractional, overflowing or malformed
values and TOML integers such as `simulation_gas_limit = 987654`. Quote the
number to apply the bound. This option affects simulations only.

## Messages

`MsgStoreCode`, `MsgInstantiateContract`, `MsgExecuteContract`, `MsgMigrateContract`, `MsgUpdateAdmin`,
`MsgClearAdmin`, `MsgIBCSend`, and `MsgIBCCloseChannel` (see [`types/tx.go`](types/tx.go)).

## Events

Contract transactions emit the following events (types in [`types/events.go`](types/events.go)):

| Event | When | Attributes |
| ----- | ---- | ---------- |
| `store_code` | code upload | `code_id`, one `feature` per required capability |
| `instantiate` | contract instantiation | `_contract_address`, `code_id` |
| `execute` | contract execution | `_contract_address` |
| `migrate`, `sudo`, `reply`, `pin_code`, ... | the matching operation | `_contract_address` and/or `code_id` |
| `wasm` | the contract returned attributes | the contract's attributes, plus `_contract_address` |
| `wasm-<type>` | the contract returned a custom event of `<type>` | the event's attributes, plus `_contract_address` |

The `_contract_address` attribute is always added by the module, so a contract cannot spoof which contract
emitted an event. Contract-supplied attribute keys must not start with `_`.

If funds are transferred to or from the contract as part of the message, the bank module emits its usual
`transfer` events as well.

## CLI

Transactions live under `paxd tx wasm` (`store`, `instantiate`, `execute`, `migrate`, `set-contract-admin`,
`clear-contract-admin`) and queries under `paxd q wasm` (`list-code`, `list-contract-by-code`, `code`, `code-info`,
`contract`, `contract-state all|raw|smart`, `contract-history`, `pinned`, `libwasmvm-version`).

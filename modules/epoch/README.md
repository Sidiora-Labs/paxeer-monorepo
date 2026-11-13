# Epoch module

The `modules/epoch` directory holds the Go implementation of time-based epochs
inside `paxd` on the single Paxeer network. Epochs follow a fixed duration, one
minute by default, from the epoch genesis time. The module checks consensus block
time in `BeginBlock` and calls registered module hooks when it advances an epoch.

## Layout

- [client/](client/): CLI queries and transaction command grouping, plus Wasm query bindings.
- [keeper/](keeper/): Epoch persistence, block transitions, hooks, gRPC queries, metrics and keeper tests.
- [simulation/](simulation/): Simulation account lookup helper and its tests.
- [spec/](spec/): Module specification.
- [types/](types/): Epoch and genesis types, validation, hook interfaces, events, store keys and generated protobuf bindings.
- [genesis.go](genesis.go): Genesis initialization and export.
- [genesis_test.go](genesis_test.go): Genesis round-trip and consensus-time initialization tests.
- [handler.go](handler.go): Message handler that rejects unrecognized messages.
- [handler_test.go](handler_test.go): Message rejection test.
- [module.go](module.go): Module registration, query services and genesis integration.
- [module_simulation.go](module_simulation.go): Randomized simulation genesis and simulation interfaces.
- [module_test.go](module_test.go): Module registration, simulation genesis, export and transition tests.

## Build and test

This directory belongs to the root [Go module](../../go.mod) and has no separate
manifest or Makefile. From the repository root, use the chain targets in
[chain.mk](../../chain.mk):

```sh
make -f chain.mk build
make -f chain.mk test
```

The build target builds `paxd`, including the epoch module, and its native library
dependencies. The test target builds those dependencies and runs the root Go
package tests in groups, including this directory and its subpackages, with race
detection and coverage.

### Package verification

The package tests cover genesis initialization and export, message rejection,
module registration, and epoch transitions. They require no external inputs.

## Configuration

The module reads no environment variables or configuration files directly. The
node passes its `epoch` genesis state from `genesis.json` to
[InitGenesis](genesis.go). The state fields are `genesis_time`, `epoch_duration`,
`current_epoch`, `current_epoch_start_time` and `current_epoch_height`.

The module's `Params` type is empty; `paxd q epoch params` queries it. The epoch
duration belongs to epoch state, not a module parameter.

Epoch transitions occur strictly after a scheduled boundary and advance at most
one elapsed period per block. See [Hooks](#hooks) for timing and catch-up behavior,
and the [module specification](spec/README.md) for an overview.

The [mint module](../mint/keeper/hooks.go) implements `AfterEpochEnd` to release
scheduled tokens to the fee collector using the ending epoch's start date in UTC.

### State

The module stores an `Epoch` under the `epoch` key in its KV store. Query it with:

```sh
paxd q epoch epoch --output json
```

The response has this shape (timestamps below are placeholders):

```json
{
  "epoch": {
    "genesis_time": "<genesis time, RFC 3339>",
    "epoch_duration": "60s",
    "current_epoch": "0",
    "current_epoch_start_time": "<epoch start time, RFC 3339>",
    "current_epoch_height": "0"
  }
}
```

| JSON field | Go field | Meaning |
| --- | --- | --- |
| `genesis_time` | `GenesisTime` | Epoch schedule's genesis time. |
| `epoch_duration` | `EpochDuration` | Fixed positive duration, rendered as a duration string such as `60s`. |
| `current_epoch` | `CurrentEpoch` | Current epoch number, initially zero in default genesis. |
| `current_epoch_start_time` | `CurrentEpochStartTime` | Scheduled start time of the current epoch. |
| `current_epoch_height` | `CurrentEpochHeight` | Block height at which this epoch was entered; initially zero in default genesis. |

[Default genesis](types/genesis.go) sets the duration to one minute. During
[genesis initialization](genesis.go), when both `GenesisTime` and
`CurrentEpochStartTime` equal the default placeholder, both are replaced
with the consensus block time; explicitly configured timestamps are preserved.
Later transitions preserve `GenesisTime` and `EpochDuration`.

### Messages

The module defines no transaction messages. State is read through queries;
transitions are driven by `BeginBlock`, with hooks and events for other modules.

### Hooks

In [keeper/abci.go](keeper/abci.go), `BeginBlock` advances an epoch only when:

```text
block time - current epoch start time > epoch duration
```

A block exactly at the boundary does not advance the epoch, call the hooks, or
emit `new_epoch`. On a qualifying block, the order is:

1. Call `AfterEpochEnd(ctx, lastEpoch)` with the ending epoch.
2. Store the new epoch: increment its number by one, advance its scheduled start
   by exactly one duration, and set its height to the current block height.
3. Call `BeforeEpochStart(ctx, newEpoch)` with the newly stored epoch.
4. Emit the `new_epoch` event.

Both callbacks execute in the same `BeginBlock` and block context. Each registered
hook runs with a cached context; successful changes and events are propagated to
the parent context. A hook panic propagates and aborts block processing rather
than silently skipping that hook and advancing the epoch.

After downtime, each block processes at most one elapsed period. The scheduled
start advances from the previous start, rather than resetting to the block time,
so elapsed periods remain available for catch-up in subsequent blocks. For
example, with one-minute epochs starting at `00:00:00`, a block at `00:05:01`
advances the start only to `00:01:00`. Four more blocks at that timestamp advance
it successively to `00:05:00`; another block at that timestamp makes no transition.
This behavior is covered by
[`TestEpochCatchupPreservesElapsedPeriods`](keeper/abci_catchup_test.go).

Modules implement the [EpochHooks interface](types/hooks.go):

```go
type EpochHooks interface {
    AfterEpochEnd(ctx sdk.Context, epoch Epoch)
    BeforeEpochStart(ctx sdk.Context, epoch Epoch)
}
```

[`node/app.go`](../../node/app.go) registers the mint keeper's hooks when
constructing the epoch keeper:

```go
app.EpochKeeper = *epochmodulekeeper.NewKeeper(
    appCodec,
    keys[epochmoduletypes.StoreKey],
    keys[epochmoduletypes.MemStoreKey],
    app.GetSubspace(epochmoduletypes.ModuleName),
).SetHooks(epochmoduletypes.NewMultiEpochHooks(
    app.MintKeeper.Hooks()))
```

`NewMultiEpochHooks` invokes hooks in registration order. A keeper with no hooks
registered still advances epoch state and emits its transition event.

### Events

Each successful transition emits one `new_epoch` event with these attributes:

- `epoch_number`: The new epoch's number as a decimal string.
- `epoch_time`: The new epoch's scheduled start time, formatted with Go's `time.Time.String()`.
- `epoch_height`: The block height at which the new epoch was entered, as a decimal string.

## Documentation

- [Module specification](spec/README.md).
- [Chain module declarations](../../docs/system/chain-modules.md#epoch).
- [Epoch documentation site page](../../docs/site/paxeer-docs/src/app/modules/epoch/page.tsx).
- [Epoch documentation in Markdown](../../docs/site/paxeer-docs/public/markdown/modules/epoch.md).
- [Mint documentation](../../docs/site/paxeer-docs/public/markdown/modules/mint.md).

<!--
order: 1
-->

# Concepts

The `x/upgrade` module schedules application upgrades for the single Paxeer
network running `paxd`. The kernel participates in that network and proposes
blocks. Upgrade handlers migrate the chain's application state at a block
boundary; receipts become final when their blocks are committed.

## Plan

A `Plan` identifies an upgrade handler by its exact `Name`, sets its activation
`Height` and carries optional `Info` metadata. The relevant fields are:

```go
type Plan struct {
    Name   string
    Height int64
    Info   string
}
```

This is the scheduling subset of the generated
[Plan type](../types/upgrade.pb.go). Its deprecated `Time` and
`UpgradedClientState` fields must be unset:
[ValidateBasic](../types/plan.go) rejects time-based upgrades and embedded IBC
client upgrade state. The name must be nonempty and the height positive.

[Keeper.ScheduleUpgrade](../keeper/keeper.go) rejects heights below the current
block height and names that have already completed. It stores one pending plan;
scheduling a replacement clears the previous plan's IBC upgrade state. An
accepted software upgrade governance proposal calls this method. `Info` can
identify the release, commit or download metadata; it does not register a handler
or migrate state by itself.

### Sidecar Process

An operator can use the repository's [Cosmovisor](../../../cosmovisor/README.md)
to manage the `paxd` binary. This optional local process watches the application's
upgrade output and switches to the binary associated with the plan name. It is
not a network consensus component. The old `cosmosd` links are replaced by the
in-tree Cosmovisor implementation and its configuration documentation.

The upgrade module writes `<node-home>/data/upgrade-info.json` with `name`,
`height` and optional `info` before halting for a missing handler. It also emits
an unescaped line on stderr so Cosmovisor's scanner can recognize it:

```text
UPGRADE "<name>" NEEDED at height: <height>: <info>
```

Set `DAEMON_NAME=paxd` and an absolute `DAEMON_HOME` for Cosmovisor's binary tree.
The replacement binary belongs in
`$DAEMON_HOME/cosmovisor/upgrades/<name>/bin/paxd`, with the upgrade name
URI-encoded as described in the Cosmovisor guide. Automatic download requires
`DAEMON_ALLOW_DOWNLOAD_BINARIES=true`; automatic restart requires
`DAEMON_RESTART_AFTER_UPGRADE=true`. Both options default to false. A staged
binary still needs the matching handler and any required store-loader changes.

## Handler

An upgrade handler is application-specific. Register it with
`Keeper.SetUpgradeHandler(planName, handler)`. The exact type in
[handler.go](../types/handler.go) is:

```go
type UpgradeHandler func(ctx sdk.Context, plan Plan, fromVM module.VersionMap) (module.VersionMap, error)
```

The version map records module names and their consensus versions. The handler
performs the required migrations and returns the updated map. Paxeer registers
its handlers in [node/upgrades.go](../../../../node/upgrades.go).

### BeginBlock timing

The scheduled-plan halt and handler decision runs in
[`upgrade.BeginBlocker`](../abci.go), before transaction execution at the upgrade
height. The node calls it through
[node/legacyabci/begin_block.go](../../../../node/legacyabci/begin_block.go): epoch
processing runs first, then upgrade, capability, distribution, slashing,
evidence, staking, IBC client and EVM begin-block processing. This is the node's
explicit call order, rather than a module-manager `EndBlock` upgrade hook.
The halt/restart/commit sequence below is exercised against the real keeper and
reopened store by `TestRepairM_610_BeginBlockRestart` in
[repair_m610_test.go](../repair_m610_test.go).

`Plan.ShouldExecute` becomes true when the current block height is at least the
plan height. For a normal scheduled upgrade at height `H`:

1. The old binary commits through `H - 1`. At `BeginBlock` for `H`, a missing
   handler causes it to write the upgrade info file, print the upgrade-needed
   message and panic. It does not commit block `H`.
2. The operator or Cosmovisor starts the replacement binary on the same node
   state. Its store loader prepares any store changes while loading the last
   committed version, before block `H` is processed again.
3. At `BeginBlock` for `H`, the matching handler runs. `Keeper.ApplyUpgrade`
   persists the returned module version map, increments the application protocol
   version, clears IBC upgrade state and the pending plan, and records the done
   height. A handler error panics. The migrated state and completion record become
   durable with the block commit.

For a future plan whose handler is already present, a non-minor upgrade panics
with `BINARY UPDATED BEFORE TRIGGER!`. If `Info` is a JSON object whose
`upgradeType` is `minor` (case-insensitive), the binary is allowed to run before
the scheduled height, but the handler still waits until the plan is due. For
example, `Info` can contain `{"upgradeType":"minor"}`. An invalid JSON `Info` is
logged as a parse error and does not grant the minor-release exception.

The begin blocker also checks compatibility with the last completed upgrade on
its first execution in a process. If the applicable check finds no registered
handler for that completed upgrade, it panics rather than continuing with an
incompatible binary. A configured skip height clears a due plan without running
its handler; this is distinct from a successful migration.

### Node-specific named upgrades

The node also has an application path,
[`App.applyNamedUpgrade`](../../../../node/upgrades.go), for known pending plans
named by the local upgrade info file. [App.BeginBlock](../../../../node/abci.go)
calls it before the module begin blockers. It only applies a recognized,
uncompleted plan when the file height equals the current block height, and checks
that required stores are mounted. The activation plan is also refused if the
module version map already contains all of its activation modules. Tracing
contexts do not apply it.

This path uses `Keeper.ApplyUpgrade` and its persisted completion record; keeping
the file on disk does not repeat a committed migration. The local file is an
operator input, distinct from the governance plan stored on-chain. For plans that
add stores, its height must match the last committed height plus one when the
replacement process starts.

## StoreLoader

Store layout changes and module state migrations occur at different points.
The application configures a `baseapp.StoreLoader` with `app.SetStoreLoader`
before loading its database. The helper in
[storeloader.go](../types/storeloader.go) has this signature:

```go
func UpgradeStoreLoader(upgradeHeight int64, storeUpgrades *store.StoreUpgrades) baseapp.StoreLoader
```

`store.StoreUpgrades` describes added, deleted and renamed stores. The helper
calls `LoadLatestVersionAndUpgrade` only if there are layout changes and
`upgradeHeight == ms.LastCommitID().Version + 1`; otherwise it uses the default
store loader. After a successful commit advances the stored version, restarting
with the same info file does not reapply those layout changes. If the process
stops before committing the upgrade block, recovery resumes from the last
committed state.

Paxeer's [App.SetStoreUpgradeHandlers](../../../../node/app.go) reads the upgrade
info file, selects store changes by name and checks skip-height configuration
before installing this loader. The file reader returns the store package's
`UpgradeInfo` containing `Name` and `Height`; the on-disk writer additionally
includes the plan's `Info` when nonempty. A missing file produces an empty info
value, while an unreadable or malformed file returns an error that startup treats
as fatal. The filename alone is not proof that migrations have committed; use
the on-chain applied-upgrade record.

## Proposal

A `SoftwareUpgradeProposal` carries `Title`, `Description` and `Plan`. When it
passes governance, the [proposal handler](../handler.go) schedules the plan.
Scheduling a new approved plan replaces the current pending plan, so a later
proposal can change its activation height before it executes. It cannot reuse a
name that has already completed.

The CLI submission entry point is
`paxd tx gov submit-proposal software-upgrade <name>` with `--upgrade-height`,
optional `--upgrade-info` and the normal proposal and transaction flags. The
selected release must contain a handler registered under that exact name.
Use the Cosmos chain ID for signing; the network's EVM chain ID is `125`
(`0x7d`). Native client configuration and address conventions are described in
[the staking client reference](../../staking/spec/09_client.md#cli).

Read the scheduled plan with `paxd query upgrade plan`. The command
`paxd query upgrade applied <name>` looks up the applied height and prints that
block's metadata, so the node must also retain the corresponding block header.
Use `--node` for your node's Cosmos consensus RPC. These commands are declared in
[query.go](../client/cli/query.go).

### Cancelling Upgrade Proposals

`CancelSoftwareUpgradeProposal` carries `Title` and `Description`. If it passes,
the handler calls `Keeper.ClearUpgradePlan`, removing the pending plan. The CLI
entry point is `paxd tx gov submit-proposal cancel-software-upgrade`. Cancellation
does not undo a completed migration or remove its done record.

Leave enough time for the actual governance deposit and voting periods,
operator coordination and a possible cancellation vote before activation. These
periods are durations, while `Plan.Height` is a block height; convert the timing
budget using the chain's block cadence. A cancellation can be proposed while the
upgrade proposal is under voting, but it must execute after scheduling and before
the plan becomes due. A fixed arithmetic formula alone cannot guarantee that
ordering.

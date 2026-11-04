<!--
order: 0
title: Bank Overview
parent:
  title: "bank"
-->

# `x/bank`

## Abstract

This document describes the Cosmos SDK bank fork used by the merged Paxeer
network. The module is wired into the node application in
[node/app.go](../../../../node/app.go).

The bank module handles multi-asset transfers between accounts, tracks total
supply by denomination, and supports module accounts. Delegation and
undelegation also track vesting-account balances. Its view, send and full keeper
interfaces provide different levels of access to other modules.

The fork additionally tracks fractional base-denomination balances in wei.
One base-denomination unit is `1000000000000` wei, as defined by
`OneUhpxInWei` in [keeper/send.go](../keeper/send.go). Whole coins and fractional
balances jointly represent the base-denomination holdings; see [Events](04_events.md)
for their distinct event units and carry/borrow behavior.

## Supply

The bank keeper tracks supply per denomination, holds tokens in module
accounts, and exposes supply queries. It also provides an SDK invariant helper;
its existence and the node's runtime conservation check have different coverage
and wiring.

### Total Supply

Minting increases the stored supply for each affected denomination; burning
reduces it. Transfers move balances without changing supply. Base-denomination
accounting includes both whole coin balances and fractional wei balances, with
10^12 wei converted to one whole unit.

[keeper/invariants.go](../keeper/invariants.go) defines `TotalSupply`, which
compares stored supply with all account balances, deferred balances, and the
base-denomination equivalent of aggregated wei balances. It also checks that
the aggregate wei remainder is zero. However,
`AppModule.RegisterInvariants` in [module.go](../module.go) is empty: the bank
module does not register this all-denomination helper as a runtime invariant.

The node instead calls `LightInvarianceChecks` during block finalization.
[LightInvarianceTotalSupply](../../../../node/invariance.go) compares changes in
whole base-denomination balances, wei balances and base-denomination supply.
It requires the total wei change to be a multiple of 10^12 and checks that the
combined balance change equals the supply change. Negative changed balances
and conservation mismatches panic. This is an incremental check of the
configured base denomination, not a full scan of every denomination.

The application option `light_invariance.supply_enabled` defaults to `true`.
Setting it to `false` disables this supply check. The check is also bypassed
when `MockBalancesEnabled` is active, and it logs and returns if the bank store
is not a MemIAVL commitment store. Malformed changed keys or balance values are
logged and skipped in several read paths, so the light check should not be
interpreted as comprehensive state validation.

## Module Accounts

Module accounts hold coins directly and can transfer them to other module
accounts or ordinary accounts. They avoid implementing token escrow as a pair
of burn and mint operations.

The actual interface is `ModuleAccountI` in
[x/auth/types/account.go](../../auth/types/account.go):

```go
type ModuleAccountI interface {
  AccountI

  GetName() string
  GetPermissions() []string
  HasPermission(string) bool
}
```

The auth keeper manages module accounts by name. The bank keeper provides
account-to-module, module-to-account and module-to-module transfers, plus
permission-controlled minting, burning, delegation and undelegation.

Message handlers must enforce recipient restrictions. The bank message server
checks blocked recipient addresses and denomination allow lists; module callers
must use the appropriate keeper methods and recipient policy for their flow.

### Permissions

The application's `maccPerms` map in [node/app.go](../../../../node/app.go) is
passed to the auth keeper and defines module account permissions. Bank wrapper
methods check those permissions for privileged operations:

- `Minter` (`"minter"`): mint coins, subject to the keeper's mint restriction.
- `Burner` (`"burner"`): burn coins.
- `Staking` (`"staking"`): receive delegated coins and release undelegated coins.

## Contents

1. **[State](01_state.md)**
2. **[Keepers](02_keepers.md)**
   - [Common Types](02_keepers.md#common-types)
   - [BaseKeeper](02_keepers.md#basekeeper)
   - [SendKeeper](02_keepers.md#sendkeeper)
   - [ViewKeeper](02_keepers.md#viewkeeper)
3. **[Messages](03_messages.md)**
   - [MsgSend](03_messages.md#msgsend)
4. **[Events](04_events.md)**
   - [Handlers](04_events.md#handlers)
   - [Keeper events](04_events.md#keeper-events)
5. **[Parameters](05_params.md)**

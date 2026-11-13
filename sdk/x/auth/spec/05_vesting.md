<!--
order: 5
-->

# Vesting

- [Intro and Requirements](#intro-and-requirements)
- [Note](#note)
- [Vesting Account Types](#vesting-account-types)
- [Vesting Account Specification](#vesting-account-specification)
- [Keepers & Handlers](#keepers--handlers)
- [Genesis Initialization](#genesis-initialization)
- [Examples](#examples)
- [Glossary](#glossary)

## Intro and Requirements

Vesting accounts in Paxeer attach transfer restrictions to an original coin
allocation. They retain the identity fields of an auth account, while `x/bank`
stores their current balances. Locked coins can be delegated through the bank
and staking paths; they cannot be sent as ordinary spendable coins. Additional
coins received by the account are outside the original allocation and do not
extend its vesting schedule.

Schedules use committed block time, expressed as Unix seconds. Vesting is
calculated on demand when a caller asks for vested or locked coins; no per-block
mutation is needed merely to advance the schedule.

## Note

Continuous and delayed accounts can be created after genesis by
`cosmos.vesting.v1beta1.Msg/CreateVestingAccount`. The module registers this
message service in [`vesting/module.go`](../vesting/module.go), and
[`msg_server.go`](../vesting/msg_server.go) creates the account and transfers
funds from the sender. See [Client](07_client.md#vesting) for the `paxd` command.

The message carries `from_address`, `to_address`, `amount`, `end_time`, `delayed`
and optional `admin`. Basic validation requires valid sender/recipient addresses,
valid positive coin amounts and a positive `end_time`. The handler additionally
checks send-enabled coins, denomination allowlists, blocked recipients and that
the recipient account does not already exist. A supplied administrator address
must parse and identify an existing account. The sender signs the message.

With `delayed = false`, the handler sets the continuous schedule's `start_time`
to the current block's Unix time. With `delayed = true`, it creates a delayed
account. Choose an end time after creation for a useful future schedule; the
message's basic validation checks positivity, while the handler rejects a
continuous end time at or before the creation block. A delayed account may have
an already elapsed positive end time and is immediately vested in that case.
Periodic and permanently locked types exist, but this message service and CLI
do not create them.

The fork's schema includes `BaseVestingAccount.Admin` (`admin`) and
`CancelledTime` (`cancelled_time`). Creation stores `admin`; `cancelled_time`
must be zero: account validation rejects nonzero markers, including genesis
imports. `Admin` is reserved metadata without cancellation authority. There is
no registered cancellation message or implemented
administrator cancellation lifecycle, and the vesting calculations do not read
these fields. Their presence in the schema does not grant an active ability to
cancel, claw back or stop a schedule.

## Vesting Account Types

The interface is defined in
[`vesting/exported/exported.go`](../vesting/exported/exported.go):

```go
type VestingAccount interface {
  types.AccountI

  LockedCoins(blockTime time.Time) sdk.Coins
  TrackDelegation(blockTime time.Time, balance, amount sdk.Coins)
  TrackUndelegation(amount sdk.Coins)

  GetVestedCoins(blockTime time.Time) sdk.Coins
  GetVestingCoins(blockTime time.Time) sdk.Coins
  GetStartTime() int64
  GetEndTime() int64
  GetOriginalVesting() sdk.Coins
  GetDelegatedFree() sdk.Coins
  GetDelegatedVesting() sdk.Coins
}
```

The [local protobuf schema](../../../proto/cosmos/vesting/v1beta1/vesting.proto)
and [account implementations](../vesting/types/vesting_account.go) define these
representations.

### BaseVestingAccount

This embeds `BaseAccount` and adds `original_vesting`, `delegated_free`,
`delegated_vesting`, `end_time`, `admin` and `cancelled_time`. It holds common
bookkeeping; concrete account types supply the schedule. None of these fields
replaces the bank balance.

### ContinuousVestingAccount

This embeds `BaseVestingAccount` and adds `start_time`. Coins vest linearly
between start and end, with the rounding described below.

### DelayedVestingAccount

This embeds `BaseVestingAccount`. All original coins vest at `end_time`.
`GetStartTime()` returns zero because there is no start-time field.

### Period

A period has an `int64 length` in seconds and a coin `amount`. The Go `Periods`
type is a slice of `Period`; periods are sequential, measured from the schedule's
start time and then from the preceding period's end.

### PeriodicVestingAccount

This embeds `BaseVestingAccount` and adds `start_time` and `vesting_periods`.
Validation requires positive period lengths and valid positive amounts, an end
time equal to start plus all period lengths, and a sum of period amounts equal
to `original_vesting`.

### PermanentLockedAccount

This embeds `BaseVestingAccount`, never vests the original allocation, and has
no scheduled release. Its constructor sets `end_time` to zero, and both time
getters return zero. Its locked coins may still be delegated through the vesting
accounting paths.

## Vesting Account Specification

The following quantities apply separately to each denomination:

| Symbol | Meaning |
| --- | --- |
| `OV` | Original vesting allocation stored in the auth account |
| `V` | Original coins still vesting at the current block time |
| `V'` | Original coins already vested: `OV - V` |
| `DV` | Delegated coins classified as vesting when delegated |
| `DF` | Delegated coins classified as free when delegated |
| `BC` | Current coin balance in `x/bank`, excluding coins moved into delegation |

`OV`, `DV` and `DF` are account bookkeeping. `BC` is a separate bank balance;
receipts, transfers and delegation change it independently of schedule time.

### Determining Vesting & Vested Amounts

#### Continuously Vesting Accounts

For a valid continuous schedule and block time `T`:

1. If `T <= StartTime`, `V' = 0`.
2. Otherwise, if `T >= EndTime`, `V' = OV`.
3. Otherwise compute the SDK decimal scalar
   `(T - StartTime) / (EndTime - StartTime)` and, per denomination, multiply by
   `OV` and use `RoundInt()` to obtain `V'`.
4. Compute `V = OV - V'`.

The implementation rounds to the nearest integer coin amount, rather than
simply truncating a rational result. Validation requires `StartTime < EndTime`.

#### Periodic Vesting Accounts

At or before `StartTime`, nothing has vested. At or after `EndTime`, the entire
original allocation has vested. Between those boundaries, start at `StartTime`
and accumulate a period's amount whenever its full length has elapsed. Advance
the period start by that length and stop at the first incomplete period.
Then `V = OV - V'`. A period releases its tranche at its end, not at its start.

#### Delayed/Discrete Vesting Accounts

Before `EndTime`, `V' = 0` and `V = OV`. At or after `EndTime`, `V' = OV` and
`V = 0`. Coins received outside the original allocation are unaffected by this
release boundary.

### Transferring/Sending

The concrete account's `LockedCoins` calls `LockedCoinsFromVesting`, which
calculates:

```text
locked = V - min(V, DV) = max(V - DV, 0)
spendable = BC - locked
```

The bank keeper reads the balance and consults the auth vesting account at
`ctx.BlockTime()`. Its `SpendableCoins` returns an empty coin set if subtracting
locks would produce a negative amount. Bank sends enforce unlocked funds; they
do not modify `original_vesting` to make a transfer possible.

#### Keepers/Handlers

[`bank/keeper/view.go`](../../bank/keeper/view.go) supplies `LockedCoins` and
`SpendableCoins`. [`bank/keeper/send.go`](../../bank/keeper/send.go) enforces the
unlocked balance during coin subtraction. Use those keeper methods rather than
editing a balance on the embedded `BaseAccount`, which has no balance field.

### Delegating

For a positive delegation `D` covered by the bank balance:

1. Require `BC >= D > 0`.
2. Compute `X = min(max(V - DV, 0), D)` as the vesting portion.
3. Compute `Y = D - X` as the free portion.
4. Set `DV = DV + X` and `DF = DF + Y`.

`TrackDelegation(blockTime, balance, amount)` updates these counters only. It
requires sorted coin sets and panics on a zero delegation or insufficient
balance. Moving the coins and persisting the changed account are the caller's
responsibility.

#### Keepers/Handlers

The bank keeper's `DelegateCoins` path tracks the vesting delegation and moves
funds to the staking module account. Its internal `trackDelegation` calls the
vesting method and saves the account through `AccountKeeper.SetAccount`. An
ordinary `SendCoins` call does not replace this accounting.

### Undelegating

For positive returned coins `D`, free delegation is reduced first:

1. Compute `X = min(DF, D)`.
2. Compute `Y = min(DV, D - X)`.
3. Set `DF = DF - X` and `DV = DV - Y`.

`TrackUndelegation(amount)` changes counters only. The bounds on both reductions
matter when the refund exceeds tracked delegation because of rounding. Slashing
can also leave a residual `DV`: returned free coins are accounted for before
vesting coins, even if the slashed delegation originally used vesting coins.

#### Keepers/Handlers

The bank keeper's `UndelegateCoins` path returns funds and calls
`trackUndelegation`, which updates and persists the auth account. It is distinct
from the staking unbonding delay: scheduling an undelegation does not itself
return the bank balance immediately.

## Keepers & Handlers

Vesting account types live under `sdk/x/auth/vesting`. The
[vesting message server](../vesting/msg_server.go) handles account creation, the
[auth keeper](../keeper/account.go) persists the typed account, and the
[bank keeper](../../bank/keeper/keeper.go) owns transfers and delegation balance
changes. Other modules must use the bank's delegation/undelegation operations
when using locked coins, so that `DV` and `DF` remain consistent.

Query the auth account for its schedule and the bank's `SpendableBalances` query
for transferable coin balances. See [State](02_state.md#bank-balances) and
[Client](07_client.md) for these distinct interfaces.

## Genesis Initialization

Vesting accounts can also be imported at genesis. The current
[`auth.GenesisState`](../../../proto/cosmos/auth/v1beta1/genesis.proto) contains
`params` and a repeated `google.protobuf.Any accounts` field. Each account carries
its concrete type URL and nested fields. There is no `Vesting` boolean or
historical `ToAccount` conversion based on optional time fields.

For example, this is the shape of one continuous account entry in
`app_state.auth.accounts`. Replace the address placeholder with a valid Paxeer
address; the example times are illustrative Unix seconds:

```json
{
  "@type": "/cosmos.vesting.v1beta1.ContinuousVestingAccount",
  "base_vesting_account": {
    "base_account": {
      "address": "<pax-address>",
      "pub_key": null,
      "account_number": "0",
      "sequence": "0"
    },
    "original_vesting": [{"denom": "uhpx", "amount": "1000000"}],
    "delegated_free": [],
    "delegated_vesting": [],
    "end_time": "2000003600",
    "admin": "",
    "cancelled_time": "0"
  },
  "start_time": "2000000000"
}
```

The corresponding current balance belongs in `app_state.bank.balances`, for
example:

```json
{
  "address": "<pax-address>",
  "coins": [{"denom": "uhpx", "amount": "1000000"}]
}
```

These are entries, not a complete genesis file. Bank genesis validation requires
balances to cover the original vesting allocation. Periodic and permanent
accounts use their own registered type URLs and fields; they are not inferred
from the continuous example.

[`auth.InitGenesis`](../genesis.go) unpacks the typed accounts, sorts them by
account number, allocates account numbers through `NewAccount`, and stores them.
Auth genesis validation checks duplicates and each account's `Validate` method;
the bank performs its own balance validation. `auth.ExportGenesis` packs the
concrete accounts back into `Any` values. The vesting module's own genesis
initialization/export is empty because its accounts are held by auth and its
balances by bank. Preserve the protobuf representation when exporting or
importing; the human-readable account `String()`/YAML rendering is not a
complete serialization of the administrator and cancellation fields.

## Examples

All amounts below are integer units of one denomination. Balances are bank
balances, and delegation removes coins from that balance.

### Simple

Start with `OV = BC = V = 10` and `DV = DF = 0` in a continuous account.

| Action | `BC` | `V` | `DV` | Locked | Spendable |
| --- | --- | --- | --- | --- | --- |
| Receive 1 additional coin | 11 | 10 | 0 | 10 | 1 |
| 2 original coins vest | 11 | 8 | 0 | 8 | 3 |
| Delegate 4 coins | 7 | 8 | 4 | 4 | 3 |
| Send 3 coins | 4 | 8 | 4 | 4 | 0 |
| 2 more original coins vest | 4 | 6 | 4 | 2 | 2 |
| Send 2 coins | 2 | 6 | 4 | 2 | 0 |

The remaining bank coins are locked but can still be delegated.

### Slashing

Start again with 10 original coins. Once 5 vest, delegate 5 to validator A
(`DV = 5`) and 5 to validator B (`DF = 5`), leaving `BC = 0` and `V = 5`.
If A is slashed by half, an eventual return from A first reduces `DF`. A
subsequent return from B reduces any remaining `DF` before `DV`.

Ignoring integer denomination rounding to illustrate the accounting, returns of
2.5 and 5 would leave `BC = 7.5`, `DF = 0` and `DV = 2.5`. The remaining lock is
`max(5 - 2.5, 0) = 2.5`, so only 5 is spendable at that point. Once the schedule
fully vests, the lock is zero even if `DV` remains positive. Real coin amounts
and staking refunds use integer units and the staking exchange-rate rules.

### Periodic Vesting

Suppose a periodic account has `OV = BC = 100`, starts at `ST` and releases 25
coins at the end of each of four periods of 7,884,000 seconds. Its `end_time`
is `ST + 31536000`; each period has amount `25` of the chosen denomination.

After receiving one extra coin, `BC = 101`. At the end of the first period,
`V = 75` and 26 coins are spendable. Sending 5 and delegating 5 leaves `BC = 91`
and `DV = 5`, so locked coins are 70 and spendable coins are 21. At the end of
the second period, `V = 50`; the lock becomes 45 and spendable coins become 46.
This illustrates the account type's schedule, not a live periodic-creation CLI.

## Glossary

- `OriginalVesting`: original scheduled allocation per denomination, recorded at
  creation or genesis import.
- `StartTime`: Unix-second start for a continuous or periodic schedule.
- `EndTime`: Unix-second full-release time for a finite schedule.
- `DelegatedFree`: tracked delegation classified as free when delegated.
- `DelegatedVesting`: tracked delegation classified as vesting when delegated.
- `ContinuousVestingAccount`: linear vesting with SDK decimal rounding.
- `DelayedVestingAccount`: full release at the end time.
- `PeriodicVestingAccount`: release in sequential tranches.
- `PermanentLockedAccount`: no scheduled release of the original allocation.
- `Admin`: reserved metadata without cancellation authority.
- `CancelledTime`: reserved persisted field that must be zero; cancellation is
  unsupported and nonzero markers fail account validation.

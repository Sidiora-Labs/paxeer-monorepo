<!--
order: 2
-->

# State

## Accounts

Auth accounts store identity and signing metadata: address, public key, account
number and sequence. Coin balances belong to `x/bank`, including balances held by
module accounts. A vesting account adds a schedule and delegation bookkeeping;
its `original_vesting` is an allocation used to calculate locks, not its current
bank balance.

The auth keeper uses the `acc` store, as defined in
[`types/keys.go`](../types/keys.go):

| Key within `acc` | Value |
| --- | --- |
| `0x01 \| address bytes` | Account encoded through the protobuf interface codec |
| `globalAccountNumber` (literal bytes) | Protobuf `UInt64Value` for the next account number |

[`AccountKeeper.SetAccount`](../keeper/account.go) persists an account; mutating
an in-memory account alone does not update the store. `NewAccount` allocates an
account number using `GetNextAccountNumber`, which advances the global counter.
Accounts may be base, module, vesting, or another registered implementation of
`AccountI`.

### Account Interface

[`AccountI`](../types/account.go) exposes authentication metadata. Its methods do
not read or write bank balances. The interface is:

```go
type AccountI interface {
  proto.Message

  GetAddress() sdk.AccAddress
  SetAddress(sdk.AccAddress) error

  GetPubKey() cryptotypes.PubKey
  SetPubKey(cryptotypes.PubKey) error

  GetAccountNumber() uint64
  SetAccountNumber(uint64) error

  GetSequence() uint64
  SetSequence(uint64) error

  String() string
}
```

A public key can be unset until the signer supplies it. `BaseAccount.SetAddress`
rejects replacing an already-set address. Sequence checking and increments occur
in the ante paths described in [AnteHandlers](03_antehandlers.md). The
[DisableSeqnoCheck parameter](07_params.md#sequence-checking) controls an explicit
sequence comparison, so replay behavior depends on more than these stored fields.

#### Base Account

The wire fields in
[`auth.proto`](../../../proto/cosmos/auth/v1beta1/auth.proto) are:

```protobuf
message BaseAccount {
  string address = 1;
  google.protobuf.Any pub_key = 2;
  uint64 account_number = 3;
  uint64 sequence = 4;
}
```

The account type URL is `/cosmos.auth.v1beta1.BaseAccount`. A `ModuleAccount`
embeds a base account and adds `name` and `permissions`. Its coins remain in the
bank store. Query responses use protobuf `Any` to preserve the concrete account
type, including vesting types. See [Client](07_client.md) for the account query
shape and supported routes. The [auth query schema](../../../proto/cosmos/auth/v1beta1/query.proto)
defines `QueryAccountResponse.account` as `Any`, without a balance field.

### Bank Balances

The [bank key definitions](../../bank/types/key.go) use a separate store named
`bank`:

| Key within `bank` | Contents |
| --- | --- |
| `0x02 \| length-prefixed address \| denomination bytes` | Coin balance for that denomination |
| `0x04 \| address bytes` | Fractional wei balance used by the fork |

`BaseViewKeeper.GetBalance` and `GetAllBalances` read coin balances;
`GetWeiBalance` reads the separate wei balance. The bank keeper's `LockedCoins`
consults the auth account's vesting implementation at the current block time.
`SpendableCoins` subtracts those locks from bank balances, returning an empty
coin set if the subtraction would be negative. These methods are implemented in
[`bank/keeper/view.go`](../../bank/keeper/view.go).

The [bank query schema](../../../proto/cosmos/bank/v1beta1/query.proto) exposes
`Balance`, `AllBalances` and `SpendableBalances` for coin balances. Use these
rather than looking for a balance in an auth account response. `AllBalances`
does not include the separately stored fractional wei balance.

### Vesting Account

See [Vesting](05_vesting.md) for supported types, live creation, lock accounting
and the typed genesis representation.

<!--
order: 2
-->

# Keepers

Paxeer's bank module exposes `Keeper`, `SendKeeper`, and `ViewKeeper`. Pass another
module the narrowest interface it needs, or define a smaller expected-keeper
interface. Even `SendKeeper` includes direct balance mutations and policy setters;
its name does not make it safe to hand to arbitrary callers.

The declarations below reflect [keeper.go](../keeper/keeper.go),
[send.go](../keeper/send.go), and [view.go](../keeper/view.go). In these declarations,
`sdk`, `codec`, and `query` refer to this repository's SDK packages, `types` to
`x/bank/types`, and `abci` to the consensus ABCI package. The extended `GenesisState`
is defined in the keeper package.

## Blocklisting Addresses

The keeper accepts a map keyed by Bech32 account address for recipients blocked
from direct transfers. `BlockedAddr` also recognizes synthetic addresses formed
from `evm_coinbase` plus eight bytes. The bank `MsgSend` and `MsgMultiSend` handlers
check recipients with `BlockedAddr`; `SendCoinsFromModuleToAccount` also enforces
that check. Module accounts commonly appear in the blocklist so funds reach them
through their intended module operations.

Do not assume every keeper operation applies the message handler's policies.
`SendCoins`, `SendCoinsWithoutAccCreation`, and `InputOutputCoins` do not themselves
check the blocklist, send-enabled parameters, or denomination allowlists. A caller
using those APIs must enforce the policies required by its own operation. In
particular, the bank message handlers check allowlists for both senders and
recipients. See [Messages](03_messages.md).

## Common Types

### Input

An input identifies an account and the coins to debit:

```protobuf
message Input {
  string address = 1;
  repeated cosmos.base.v1beta1.Coin coins = 2;
}
```

### Output

An output identifies an account and the coins to credit:

```protobuf
message Output {
  string address = 1;
  repeated cosmos.base.v1beta1.Coin coins = 2;
}
```

Both types are defined in [bank.proto](../../../proto/cosmos/bank/v1beta1/bank.proto).
Input/output validation requires valid addresses, positive coin sets, and equal
aggregate input and output amounts for every denomination.

## BaseKeeper

`BaseKeeper` implements the full `Keeper` interface, including module transfers,
staking transfers, minting, burning, supply and metadata writes, genesis, and
queries. Minting and burning require the corresponding module-account permissions.
`WithMintCoinsRestriction(check MintingRestrictionFn) BaseKeeper` returns a keeper
copy with an additional mint restriction; chained restrictions all apply. The
function type is `func(ctx sdk.Context, coins sdk.Coins) error`.

```go
type Keeper interface {
    SendKeeper

    InitGenesis(sdk.Context, *types.GenesisState)
    ExportGenesis(sdk.Context) *types.GenesisState
    InitGenesisWithAllowLists(sdk.Context, *GenesisState)
    ExportGenesisWithAllowLists(sdk.Context) *GenesisState

    GetSupply(ctx sdk.Context, denom string) sdk.Coin
    HasSupply(ctx sdk.Context, denom string) bool
    SetSupply(ctx sdk.Context, coin sdk.Coin)
    GetPaginatedTotalSupply(ctx sdk.Context, pagination *query.PageRequest) (sdk.Coins, *query.PageResponse, error)
    IterateTotalSupply(ctx sdk.Context, cb func(sdk.Coin) bool)
    GetDenomMetaData(ctx sdk.Context, denom string) (types.Metadata, bool)
    SetDenomMetaData(ctx sdk.Context, denomMetaData types.Metadata)
    IterateAllDenomMetaData(ctx sdk.Context, cb func(types.Metadata) bool)

    SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
    SendCoinsFromModuleToModule(ctx sdk.Context, senderModule, recipientModule string, amt sdk.Coins) error
    SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
    DelegateCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
    UndelegateCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error

    MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
    BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error

    DeferredSendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
    WriteDeferredBalances(ctx sdk.Context) []abci.Event
    IterateDeferredBalances(ctx sdk.Context, cb func(addr sdk.AccAddress, coin sdk.Coin) bool)

    DelegateCoins(ctx sdk.Context, delegatorAddr, moduleAccAddr sdk.AccAddress, amt sdk.Coins) error
    UndelegateCoins(ctx sdk.Context, moduleAccAddr, delegatorAddr sdk.AccAddress, amt sdk.Coins) error

    GetStoreKey() sdk.StoreKey
    GetCdc() codec.BinaryCodec

    types.QueryServer
}
```

`NewBaseKeeper` creates a keeper without deferred storage.
`NewBaseKeeperWithDeferredCache` takes the same codec, bank store key, account
keeper, parameter subspace, and blocklist, followed by a
`deferredCacheStoreKey sdk.StoreKey`. Use that constructor for deferred APIs; they
panic if no deferred cache was configured.

`DeferredSendCoinsFromAccountToModule` debits immediately and stages the module
credit under the current transaction index. `IterateDeferredBalances` can visit
the same module and denomination more than once. `WriteDeferredBalances` aggregates
credits, writes them in sorted module order, clears the cache, and returns events.
The node calls it in `CompleteBlock`. The [State page](01_state.md) describes this
lifecycle and the current limitations of module genesis recovery for allowlists.

## SendKeeper

`BaseSendKeeper` implements transfers and direct balance changes without exposing
minting or burning. Direct `AddCoins` / `SubUnlockedCoins` calls do not adjust total
supply; callers must preserve the relevant accounting invariants.

```go
type SendKeeper interface {
    ViewKeeper

    InputOutputCoins(ctx sdk.Context, inputs []types.Input, outputs []types.Output) error
    SendCoins(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
    SendCoinsWithoutAccCreation(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
    SendCoinsAndWei(ctx sdk.Context, from sdk.AccAddress, to sdk.AccAddress, amt sdk.Int, wei sdk.Int) error
    SubUnlockedCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins, checkNeg bool) error
    AddCoins(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coins, checkNeg bool) error
    SubWei(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Int) error
    AddWei(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Int) error

    GetParams(ctx sdk.Context) types.Params
    SetParams(ctx sdk.Context, params types.Params)

    IsSendEnabledCoin(ctx sdk.Context, coin sdk.Coin) bool
    IsSendEnabledCoins(ctx sdk.Context, coins ...sdk.Coin) error
    SetDenomAllowList(ctx sdk.Context, denom string, allowList types.AllowList)
    GetDenomAllowList(ctx sdk.Context, denom string) types.AllowList
    IsInDenomAllowList(ctx sdk.Context, addr sdk.AccAddress, coins sdk.Coins, cache map[string]AllowedAddresses) bool

    BlockedAddr(addr sdk.AccAddress) bool
    RegisterRecipientChecker(RecipientChecker)
    RegisterSendEnabledChecker(SendEnabledChecker)
    CanSendTo(ctx sdk.Context, recipient sdk.AccAddress) bool
}
```

### Coin and wei mutations

`SubUnlockedCoins(..., checkNeg bool)` always validates the input coin set. With
`checkNeg = true`, it checks the balance minus locked vesting coins, rejects an
insufficient spendable balance, and validates the resulting stored coin. With
`false`, it uses unsafe subtraction and skips resulting-balance validity checks.
This is a privileged accounting path, not a way to authorize overspending.

`AddCoins(..., checkNeg bool)` also validates the input coins. Its flag controls
resulting-balance validation at the write. Normal transfer methods pass `true`.
Callers must propagate errors and use the transaction/cache context so a later
failure cannot leave earlier writes committed on their own.

`SendCoins` creates a missing recipient account; `SendCoinsWithoutAccCreation`
does not. `SendCoinsAndWei` accepts separate whole native base units (`amt`) and
wei (`wei`), validates both as initialized and non-negative, and performs the
combined transfer in a cache context. It writes that context only after every
step succeeds, and does not create the recipient account.

`AddWei` and `SubWei` normalize fractional native balances against whole `uhpx`
using `10^12` wei per `uhpx`. These methods do not accept a `checkNeg` bypass.
Ordinary coin and fractional balances must both be considered when integrating
native EVM amounts.

### Policy checks and hooks

`IsSendEnabledCoin` combines the parameter policy with every registered
`SendEnabledChecker`; any checker returning false disables the denomination.
`IsSendEnabledCoins` returns `types.ErrSendDisabled` when any coin is disabled.
`SetDenomAllowList`, `GetDenomAllowList`, and `IsInDenomAllowList` handle the
separate denomination policy. Supply a non-nil per-operation
`map[string]AllowedAddresses` cache to `IsInDenomAllowList`.

The registration callback types are:

```go
type RecipientChecker = func(ctx sdk.Context, recipient sdk.AccAddress) bool
type SendEnabledChecker = func(ctx sdk.Context, denom string) bool
type BalanceChangeHook = func(ctx sdk.Context, addr sdk.AccAddress, denom string, before func() sdk.Int) error
```

`RegisterRecipientChecker` appends a checker. `CanSendTo` requires every checker to
accept, independently of the blocklist and allowlists. `AddCoins` and non-zero
`AddWei` invoke it and return `sdkerrors.ErrInvalidRecipient` on rejection.

The concrete `BaseSendKeeper` also exposes
`RegisterBalanceChangeHook(name string, hook BalanceChangeHook)`; this method is
not a member of `SendKeeper`. Register hooks during keeper construction. An empty
name, nil callback, or duplicate name panics. Registrations are shared by keeper
copies and callbacks run in registration order before `setBalance` writes or
deletes an ordinary coin balance. `before()` lazily reads the prior amount from
the supplied context. A hook error aborts that write. Direct initialization and
writes to the fractional wei store do not call this hook; wei operations that
carry or borrow whole coins invoke it for that coin write.

## ViewKeeper

`BaseViewKeeper` provides reads and validation without balance mutation methods.

```go
type ViewKeeper interface {
    ValidateBalance(ctx sdk.Context, addr sdk.AccAddress) error
    HasBalance(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coin) bool

    GetAllBalances(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
    GetAccountsBalances(ctx sdk.Context) []types.Balance
    GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
    LockedCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
    SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
    GetWeiBalance(ctx sdk.Context, addr sdk.AccAddress) sdk.Int

    IterateAccountBalances(ctx sdk.Context, addr sdk.AccAddress, cb func(coin sdk.Coin) (stop bool))
    IterateAllBalances(ctx sdk.Context, cb func(address sdk.AccAddress, coin sdk.Coin) (stop bool))
    IterateAllWeiBalances(ctx sdk.Context, cb func(sdk.AccAddress, sdk.Int) bool)
}
```

`GetBalance` performs one keyed lookup for an account and denomination;
`GetWeiBalance` does the same in the fractional store. Their cost depends on the
underlying KV store; there is no blanket `O(1)` guarantee. `GetAllBalances` and
`SpendableCoins` scan an account's denominations, while account/all-balance and
all-wei iterators visit the relevant stored entries. `GetAccountsBalances` scans
all ordinary balances. Iterator callbacks returning true stop iteration.

`LockedCoins` reads vesting locks at the context's block time, and
`SpendableCoins` subtracts locked amounts from ordinary balances. Ordinary balance
queries do not include fractional wei or pending deferred module credits.

The interface blocks above are checked against their source declarations by
`TestRepairM_669_KeeperCapabilities` in
[repair_m669_test.go](../repair_m669_test.go). Its keeper examples also demonstrate
that `SubUnlockedCoins` with `checkNeg = false` can persist a negative balance,
and that `AddCoins` with `checkNeg = true` rejects a still-negative result. Neither
flag disables input validation, recipient checkers, or balance-change hooks.
Do not use the unsafe path as an alternative to normal transfer validation.

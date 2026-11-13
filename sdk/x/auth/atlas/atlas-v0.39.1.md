# x/auth

The `x/auth` module is responsible for specifying the base transaction and
account types for an application, as well as AnteHandler and authentication logic.

## Usage

1. Import the module.

   ```go
   import (
       "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth"
       "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
       authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
       authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
       authsims "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/simulation"
   )
   ```

2. Add `AppModuleBasic` to your `ModuleBasics`.

    ```go
    var (
      ModuleBasics = module.NewBasicManager(
        // ...
        auth.AppModuleBasic{},
      )
    )
    ```

3. Create the module's parameter subspace in your application constructor.

   ```go
   func NewApp(...) *App {
     // ...
     app.ParamsKeeper.Subspace(authtypes.ModuleName)
   }
   ```

4. Create the keeper.

   ```go
   func NewApp(...) *App {
      // ...
      app.AccountKeeper = authkeeper.NewAccountKeeper(
       appCodec, keys[authtypes.StoreKey], app.GetSubspace(authtypes.ModuleName),
       authtypes.ProtoBaseAccount, maccPerms,
      )
   }
   ```

5. Add the `x/auth` module to the app's `ModuleManager`.

   ```go
   func NewApp(...) *App {
     // ...
     app.mm = module.NewManager(
       // ...
       auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts),
       // ...
     )
   }
   ```

6. Set the `x/auth` module genesis order.

   ```go
   func NewApp(...) *App {
     // ...
     app.mm.SetOrderInitGenesis(..., authtypes.ModuleName, ...)
   }
   ```

7. Add the `x/auth` module to the simulation manager (if you have one set).

   ```go
   func NewApp(...) *App {
     // ...
     app.sm = module.NewSimulationManager(
       // ...
       auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts),
       // ...
     )
   }
   ```

8. Set the `AnteHandler` using `ante.HandlerOptions`.

   ```go
   func NewApp(...) *App {
     handler, err := ante.NewAnteHandler(ante.HandlerOptions{
       AccountKeeper: app.AccountKeeper,
       BankKeeper: app.BankKeeper,
       FeegrantKeeper: app.FeeGrantKeeper,
       ParamsKeeper: app.ParamsKeeper,
       SignModeHandler: app.GetTxConfig().SignModeHandler(),
       SigGasConsumer: ante.DefaultSigVerificationGasConsumer,
     })
     if err != nil {
       panic(err)
     }
     app.SetAnteHandler(handler)
   }
   ```

### Vesting Accounts

The `x/auth` modules also defines a few standard vesting account types under the
`vesting` sub-package. In order to get your application to automatically support
these in terms of encoding and decoding, you must register the types with your
application Amino codec.

Wherever you define the application `Codec`, be sure to register types via:

```go
import (
    "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
    vestingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
)

func MakeCodec() *codec.LegacyAmino {
  cdc := codec.NewLegacyAmino()

  // ...
  vestingtypes.RegisterLegacyAminoCodec(cdc)
  // ...

  return cdc
}
```

## Genesis

The `x/auth` module defines its genesis state as follows:

```go
type GenesisState struct {
  Params   Params                   `json:"params" yaml:"params"`
  Accounts exported.GenesisAccounts `json:"accounts" yaml:"accounts"`
}
```

Which relies on the following types:

```go
type Account interface {
  GetAddress() sdk.AccAddress
  SetAddress(sdk.AccAddress) error
  GetPubKey() crypto.PubKey
  SetPubKey(crypto.PubKey) error
  GetAccountNumber() uint64
  SetAccountNumber(uint64) error
  GetSequence() uint64
  SetSequence(uint64) error
  GetCoins() sdk.Coins
  SetCoins(sdk.Coins) error
  SpendableCoins(blockTime time.Time) sdk.Coins
  String() string
}

type Params struct {
  MaxMemoCharacters      uint64 `json:"max_memo_characters" yaml:"max_memo_characters"`
  TxSigLimit             uint64 `json:"tx_sig_limit" yaml:"tx_sig_limit"`
  TxSizeCostPerByte      uint64 `json:"tx_size_cost_per_byte" yaml:"tx_size_cost_per_byte"`
  SigVerifyCostED25519   uint64 `json:"sig_verify_cost_ed25519" yaml:"sig_verify_cost_ed25519"`
  SigVerifyCostSecp256k1 uint64 `json:"sig_verify_cost_secp256k1" yaml:"sig_verify_cost_secp256k1"`
}
```

## Client

### CLI

The `x/auth` module provides various auxiliary CLI commands and a few that are
part of the module itself via the `ModuleManager`. The commands that are part of
the module itself are defined below:

1. Query an account.

   ```shell
   paxd query auth account [address] [...flags]
   ```

2. Sign an unsigned transaction using a single signature.

   ```shell
   paxd tx sign [file]
   ```

3. Sign an unsigned transaction using a multisig.

   ```shell
   paxd tx multisign [file] [name] [[signature]...]
   ```

### REST

The `x/auth` module provides various auxiliary REST handlers and a few that are
part of the module itself via the `ModuleManager`. The endpoints that are part of
the module itself are defined below:

1. Query an account.

   | Method | Path                     |
   | :----- | :----------------------- |
   | `GET` | `/auth/accounts/{address}` |

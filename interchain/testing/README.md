# IBC Testing Package

This package exercises IBC clients, connections, channels and packets against SDK
applications in process. Its chains are test fixtures. Paxeer runs one network
through `paxd`, with the kernel participating in block proposal and finality at
block commit. Creating two `TestChain` instances does not provision production
networks or tenant domains.

The default application is [simapp](simapp/README.md); the [mock application](mock/README.md)
supports core IBC and middleware tests without depending on the transfer application.

## Components

The testing package has four cooperating components:

- `Coordinator` owns the test chains and their shared clock. `TimeIncrement`
  defaults to five seconds. Use its commit and time helpers to keep chain headers
  synchronized when creating proofs and updating clients.
- `TestChain` wraps a `TestingApp`, validator set, headers and funded sender
  accounts. `NewTestChain` creates four validators; `NewTestChainWithValSet`
  accepts a supplied validator set and matching, correctly ordered signers.
  `SendMsgs` signs and delivers SDK messages, commits successful execution and
  advances the headers and coordinator time.
- `Path` connects two endpoints. `NewPath(chainA, chainB)` creates `EndpointA` and
  `EndpointB` and sets their counterparty pointers. `RelayPacket` receives and
  acknowledges an already sent packet using its commitment and emitted acknowledgement.
- `Endpoint` holds one chain's client, connection and channel identifiers and
  configuration. Its methods perform handshakes, client updates and packet
  operations on that side of the path.

See [coordinator.go](coordinator.go), [chain.go](chain.go), [path.go](path.go) and
[endpoint.go](endpoint.go) for the implementations.

## Integration

### TestingApp

[app.go](app.go) declares the application interface:

```go
type TestingApp interface {
	abci.Application

	GetBaseApp() *baseapp.BaseApp
	GetStakingKeeper() stakingkeeper.Keeper
	GetIBCKeeper() *keeper.Keeper
	GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper
	GetTxConfig() client.TxConfig
	AppCodec() codec.Codec
	LastCommitID() sdk.CommitID
	LastBlockHeight() int64
}
```

Here `abci` comes from `consensus/abci/types`, `keeper` from
`interchain/modules/core/keeper`, and the remaining types from the corresponding
packages under `sdk/`. Use this repository's types when implementing the interface.

[`SimApp`](simapp/app.go) embeds `BaseApp` and implements the keeper, codec and
transaction-configuration accessors. For example:

```go
func (app *SimApp) GetIBCKeeper() *ibckeeper.Keeper {
	return app.IBCKeeper
}

func (app *SimApp) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper {
	return app.ScopedIBCKeeper
}

func (app *SimApp) GetTxConfig() client.TxConfig {
	return MakeTestEncodingConfig().TxConfig
}
```

The current harness also uses `TestingAppDecorator` in [chain.go](chain.go) to
expose simapp-specific test support. In particular, `SendMsgs` asserts that
`chain.App` is a `*TestingAppDecorator`. Implementing `TestingApp` alone does not
make an arbitrary application compatible with every harness helper.

### Initialize TestingApp

The standalone `SetupTestingApp` helper in [app.go](app.go) constructs an
application and returns its default module genesis state:

```go
func SetupTestingApp() (TestingApp, map[string]json.RawMessage) {
	db := dbm.NewMemDB()
	encCdc := simapp.MakeTestEncodingConfig()
	app := simapp.NewSimApp(db, nil, true, map[int64]bool{}, simapp.DefaultNodeHome, 5, nil, encCdc, simapp.EmptyAppOptions{})
	return app, simapp.NewDefaultGenesisState(encCdc.Marshaler)
}
```

This snippet uses `encoding/json`, `github.com/tendermint/tm-db` as `dbm`, and
`github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp` as `simapp`.
The returned genesis map still needs to be passed to application initialization.

`DefaultTestingAppInit` is declared with `SetupTestingApp` as its initial value,
but the current `NewCoordinator` → `NewTestChain` → `NewTestChainWithValSet` path
does not call it. Instead, the constructor calls `simapp.SetupWithGenesisValSet`
and wraps the result with `NewTestingAppDecorator`. Reassigning
`DefaultTestingAppInit` therefore does not replace the coordinator's application.
For the supplied harness, use its constructors and configure the resulting simapp;
a different application needs construction and delivery code that supports it.

## Example

This complete test creates a default mock path and relays a packet:

```go
package ibctesting_test

import (
	"testing"

	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/mock"
	"github.com/stretchr/testify/require"
)

func TestMockPacketRelay(t *testing.T) {
	coord := ibctesting.NewCoordinator(t, 2)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))
	path := ibctesting.NewPath(chainA, chainB)

	// NewPath leaves all three identifiers empty until the handshakes run.
	coord.Setup(path)
	require.Equal(t, "07-tendermint-0", path.EndpointA.ClientID)
	require.Equal(t, "connection-0", path.EndpointA.ConnectionID)
	require.Equal(t, "channel-0", path.EndpointA.ChannelID)

	packet := channeltypes.NewPacket(
		mock.MockPacketData, 1,
		path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID,
		chainB.GetTimeoutHeight(), 0,
	)
	require.NoError(t, path.EndpointA.SendPacket(packet))
	require.NoError(t, path.RelayPacket(packet))
}
```

The literal identifiers above apply to the first client, connection and channel
on fresh chains. They belong to three different fields and may differ when a
chain already has IBC state.

Configure the endpoints before starting their handshakes. The defaults in
[config.go](config.go) select Tendermint clients, the mock port, `mock-version` and
an unordered channel. `ClientID`, `ConnectionID` and `ChannelID` begin empty;
`ClientConfig`, `ConnectionConfig` and `ChannelConfig` are already populated.

The coordinator offers the following setup levels:

- `SetupClients(path)` creates clients on both endpoints.
- `SetupConnections(path)` creates clients and completes the connection handshake.
- `Setup(path)` creates clients, connections and channels using the path's configuration.

These helpers require success. To assert a handshake failure, call individual
endpoint methods and check the returned error instead.

For separate receive and acknowledgement steps, replace `RelayPacket` with:

```go
result, err := path.EndpointB.RecvPacketWithResult(packet)
require.NoError(t, err)
ack, err := ibctesting.ParseAckFromEvents(result.GetEvents())
require.NoError(t, err)
require.NoError(t, path.EndpointA.AcknowledgePacket(packet, ack))
```

`RelayPacket` expects a synchronous acknowledgement event. For asynchronous
receipt, use the endpoint methods and call `WriteAcknowledgement` on the
receiving endpoint when the application is ready, then `AcknowledgePacket` on the
sending endpoint. Client updates can also be requested explicitly with
`Endpoint.UpdateClient()`.

### Transfer Testing Example

The default simapp already includes the ICS 20 transfer application. A transfer
path must set both the port and version on both endpoints before `Setup`:

```go
func NewTransferPath(chainA, chainB *ibctesting.TestChain) *ibctesting.Path {
	path := ibctesting.NewPath(chainA, chainB)
	path.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
	path.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort
	path.EndpointA.ChannelConfig.Version = transfertypes.Version
	path.EndpointB.ChannelConfig.Version = transfertypes.Version
	return path
}
```

Here `transfertypes` is
`github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types`,
whose `Version` is `ics20-1`. Use transfer messages and transfer packet data for
this path. The mock packet payload in the preceding example is specific to the
mock application.

Use `chain.GetSimApp()` to access the underlying simapp's public keepers and
modules. The current constructor stores a decorator in `chain.App`, so a direct
`chain.App.(*simapp.SimApp)` assertion is incorrect.

### Middleware Testing

Wire a middleware stack in the application and expose its base application when
tests need to change callbacks. The [mock module](mock/README.md) supplies
`MockIBCApp`, with an optional function field for each IBC callback. Configure
its port and scoped capability keeper before registering it with
`mock.NewIBCModule`.

The simapp exposes `ICAAuthModule`, the mock authentication application under
the interchain accounts controller middleware. For example, a test can inject a
channel-open-try refusal using the current callback signature:

```go
chainA.GetSimApp().ICAAuthModule.IBCApp.OnChanOpenTry = func(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID, channelID string,
	chanCap *capabilitytypes.Capability,
	counterparty channeltypes.Counterparty,
	counterpartyVersion string,
) (string, error) {
	return "", fmt.Errorf("mock authentication application rejected OnChanOpenTry")
}
```

This snippet uses `fmt`, `sdk/types` as `sdk`, `sdk/x/capability/types` as
`capabilitytypes`, and the `channeltypes` import from the packet example.
An `OnChanOpenInit` override receives the same arguments but ends with
`version string` and returns only `error`.

Install an override before the operation under test, then assert the outcome
through the middleware path being exercised. A non-nil callback replaces the
mock's default handling, including its default channel-capability claim for
open-init and open-try; a successful override must perform any capability work
its test needs.

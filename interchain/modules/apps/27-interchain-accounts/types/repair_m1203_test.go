package types_test

import (
	"bytes"
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ica "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts"
	icahost "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/host"
	hostkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/host/keeper"
	icatypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/types"
	ibc "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	ibchost "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability"
	capabilitytypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// Produce every ICA mapping and account through the real controller/core/host
// handshake; the exported fixture is never assembled with keeper setters.
func repairM1203Handshake(t *testing.T) *ibctesting.Path {
	t.Helper()
	coordinator := ibctesting.NewCoordinator(t, 2)
	path := ibctesting.NewPath(coordinator.GetChain(ibctesting.GetChainID(1)), coordinator.GetChain(ibctesting.GetChainID(2)))
	path.SetChannelOrdered()
	path.EndpointB.ChannelConfig.PortID = icatypes.PortID
	coordinator.SetupConnections(path)
	owner := path.EndpointA.Chain.SenderAccount.GetAddress().String()
	port, err := icatypes.NewControllerPortID(owner)
	require.NoError(t, err)
	path.EndpointA.ChannelConfig.PortID = port
	repairM1203Register(t, path, owner)
	path.EndpointB.ChannelConfig.Version = path.EndpointA.ChannelConfig.Version
	require.NoError(t, path.EndpointB.ChanOpenTry())
	require.NoError(t, path.EndpointA.ChanOpenAck())
	require.NoError(t, path.EndpointB.ChanOpenConfirm())
	return path
}

func repairM1203Register(t *testing.T, path *ibctesting.Path, owner string) {
	t.Helper()
	chain := path.EndpointA.Chain
	ctx := chain.GetContext()
	sequence := chain.App.GetIBCKeeper().ChannelKeeper.GetNextChannelSequence(ctx)
	require.NoError(t, chain.GetSimApp().ICAControllerKeeper.RegisterInterchainAccount(ctx, path.EndpointA.ConnectionID, owner))
	path.EndpointA.ChannelID = channeltypes.FormatChannelIdentifier(sequence)
	path.EndpointA.ChannelConfig.Version = path.EndpointA.GetChannel().Version
	chain.Coordinator.CommitBlock(chain)
}

func TestRepairM_1203_MalformedHostGenesisRefused(t *testing.T) {
	path := repairM1203Handshake(t)
	app := path.EndpointB.Chain.GetSimApp()
	exported := hostkeeper.ExportGenesis(path.EndpointB.Chain.GetContext(), app.ICAHostKeeper)
	require.Len(t, exported.ActiveChannels, 1)
	require.Len(t, exported.InterchainAccounts, 1)
	cdc := app.AppCodec()
	encoded, err := cdc.MarshalAsJSON(&exported)
	require.NoError(t, err)
	require.NoError(t, exported.Validate())
	foreign, err := bech32.ConvertAndEncode("foreign", bytes.Repeat([]byte{7}, 20))
	require.NoError(t, err)
	address := exported.InterchainAccounts[0].AccountAddress
	last := "q"
	if strings.HasSuffix(address, last) {
		last = "p"
	}
	cases := []struct {
		name    string
		mutate  func(*icatypes.HostGenesisState)
		message string
	}{
		{"alphanumeric address", func(gs *icatypes.HostGenesisState) { gs.InterchainAccounts[0].AccountAddress = "a" }, "invalid host account address"},
		{"empty address", func(gs *icatypes.HostGenesisState) { gs.InterchainAccounts[0].AccountAddress = "" }, "invalid host account address"},
		{"foreign address prefix", func(gs *icatypes.HostGenesisState) { gs.InterchainAccounts[0].AccountAddress = foreign }, "invalid host account address"},
		{"bad address checksum", func(gs *icatypes.HostGenesisState) {
			gs.InterchainAccounts[0].AccountAddress = address[:len(address)-1] + last
		}, "invalid host account address"},
		{"empty channel connection", func(gs *icatypes.HostGenesisState) { gs.ActiveChannels[0].ConnectionId = "" }, "identifier"},
		{"malformed channel connection", func(gs *icatypes.HostGenesisState) { gs.ActiveChannels[0].ConnectionId = "invalid|connection" }, "identifier"},
		{"channel connection separator", func(gs *icatypes.HostGenesisState) { gs.ActiveChannels[0].ConnectionId = "connection/0" }, "identifier"},
		{"empty account connection", func(gs *icatypes.HostGenesisState) { gs.InterchainAccounts[0].ConnectionId = "" }, "identifier"},
		{"malformed account connection", func(gs *icatypes.HostGenesisState) { gs.InterchainAccounts[0].ConnectionId = "invalid|connection" }, "identifier"},
		{"account connection separator", func(gs *icatypes.HostGenesisState) { gs.InterchainAccounts[0].ConnectionId = "connection/0" }, "identifier"},
		{"invalid channel", func(gs *icatypes.HostGenesisState) { gs.ActiveChannels[0].ChannelId = "invalid|channel" }, "identifier"},
		{"invalid channel port", func(gs *icatypes.HostGenesisState) { gs.ActiveChannels[0].PortId = "invalid|port" }, "identifier"},
		{"invalid account port", func(gs *icatypes.HostGenesisState) { gs.InterchainAccounts[0].PortId = "invalid|port" }, "identifier"},
		{"duplicate channel", func(gs *icatypes.HostGenesisState) {
			gs.ActiveChannels = append(gs.ActiveChannels, gs.ActiveChannels[0])
		}, "duplicate host active channel mapping"},
		{"conflicting channel", func(gs *icatypes.HostGenesisState) {
			ch := gs.ActiveChannels[0]
			ch.ChannelId = "channel-999"
			gs.ActiveChannels = append(gs.ActiveChannels, ch)
		}, "duplicate host active channel mapping"},
		{"duplicate account", func(gs *icatypes.HostGenesisState) {
			gs.InterchainAccounts = append(gs.InterchainAccounts, gs.InterchainAccounts[0])
		}, "duplicate host account mapping"},
		{"conflicting account", func(gs *icatypes.HostGenesisState) {
			acc := gs.InterchainAccounts[0]
			acc.AccountAddress = sdk.AccAddress(bytes.Repeat([]byte{8}, 20)).String()
			gs.InterchainAccounts = append(gs.InterchainAccounts, acc)
		}, "duplicate host account mapping"},
		{"invalid host port", func(gs *icatypes.HostGenesisState) { gs.Port = "invalid|port" }, "identifier"},
		{"wrong host port", func(gs *icatypes.HostGenesisState) { gs.Port = "transfer" }, "expected icahost"},
		{"invalid params", func(gs *icatypes.HostGenesisState) { gs.Params.AllowMessages = []string{" "} }, "parameter must not contain empty strings"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gs icatypes.HostGenesisState
			require.NoError(t, cdc.UnmarshalAsJSON(encoded, &gs))
			tc.mutate(&gs)
			data, err := cdc.MarshalAsJSON(icatypes.NewGenesisState(icatypes.DefaultControllerGenesis(), gs))
			require.NoError(t, err)
			err = (ica.AppModuleBasic{}).ValidateGenesis(cdc, nil, data)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.message)
		})
	}
	// Distinct connection/port pairs are independent, including the same port
	// on another connection and another owner port on the same connection.
	for _, changeConnection := range []bool{false, true} {
		var gs icatypes.HostGenesisState
		require.NoError(t, cdc.UnmarshalAsJSON(encoded, &gs))
		ch, acc := gs.ActiveChannels[0], gs.InterchainAccounts[0]
		if changeConnection {
			ch.ConnectionId, acc.ConnectionId = "connection-2", "connection-2"
		} else {
			ch.PortId, acc.PortId = "icacontroller-other", "icacontroller-other"
		}
		ch.ChannelId = "channel-999"
		acc.AccountAddress = sdk.AccAddress(bytes.Repeat([]byte{9}, 20)).String()
		gs.ActiveChannels = append(gs.ActiveChannels, ch)
		gs.InterchainAccounts = append(gs.InterchainAccounts, acc)
		require.NoError(t, gs.Validate())
	}
	require.NoError(t, icatypes.DefaultHostGenesis().Validate())
}

func TestRepairM_1203_ControllerRemoteAddresses(t *testing.T) {
	for _, remote := range []string{"a", "0x0123456789abcdef", strings.Repeat("x", icatypes.DefaultMaxAddrLength)} {
		gs := icatypes.DefaultControllerGenesis()
		gs.InterchainAccounts = []icatypes.RegisteredInterchainAccount{{ConnectionId: "connection-0", PortId: "icacontroller-owner", AccountAddress: remote}}
		require.NoError(t, gs.Validate(), remote)
		host := icatypes.DefaultHostGenesis()
		host.InterchainAccounts = gs.InterchainAccounts
		require.Error(t, host.Validate(), remote)
	}
}

// Fresh production keepers and a disk-backed multistore on every open. Memory
// capabilities are reconstructed from the committed capability owner records.
func repairM1203Open(t *testing.T, dir string, header tmproto.Header) (*simapp.SimApp, *rootmulti.Store, sdk.Context, func()) {
	t.Helper()
	db, err := dbm.NewGoLevelDB("ica", dir)
	require.NoError(t, err)
	ms := rootmulti.NewStore(db)
	options := viper.New()
	options.Set(baseapp.FlagChainID, header.ChainID)
	app := simapp.NewSimApp(db, nil, false, map[int64]bool{}, dir, 0, nil,
		simapp.MakeTestEncodingConfig(), options, func(b *baseapp.BaseApp) { b.SetCMS(ms) })
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, header, false)
	if ms.LastCommitID().Version > 0 {
		app.CapabilityKeeper.InitMemStore(ctx)
	}
	closed := false
	stop := func() {
		if !closed {
			require.NoError(t, ms.Close())
			closed = true
		}
	}
	t.Cleanup(stop)
	return app, ms, ctx, stop
}

func TestRepairM_1203_PersistedExportImportReopenHandshake(t *testing.T) {
	path := repairM1203Handshake(t)
	controller, source := path.EndpointA.Chain, path.EndpointB.Chain
	port := path.EndpointA.ChannelConfig.PortID
	owner := controller.SenderAccount.GetAddress().String()
	address, found := source.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(source.GetContext(), path.EndpointB.ConnectionID, port)
	require.True(t, found)
	require.NoError(t, path.EndpointA.SetChannelClosed())
	require.NoError(t, path.EndpointB.SetChannelClosed())

	producer := source.GetSimApp()
	ctx := source.GetContext()
	cdc := producer.AppCodec()
	hostState := hostkeeper.ExportGenesis(ctx, producer.ICAHostKeeper)
	require.NoError(t, hostState.Validate())
	hostBytes, err := cdc.MarshalAsJSON(&hostState)
	require.NoError(t, err)
	coreBytes, err := cdc.MarshalAsJSON(ibc.ExportGenesis(ctx, *producer.IBCKeeper))
	require.NoError(t, err)
	authBytes, err := cdc.MarshalAsJSON(auth.ExportGenesis(ctx, producer.AccountKeeper))
	require.NoError(t, err)
	capBytes, err := cdc.MarshalAsJSON(capability.ExportGenesis(ctx, *producer.CapabilityKeeper))
	require.NoError(t, err)

	// Produce the next handshake request on the controller, rather than
	// inventing metadata that the real producer might never send.
	repairM1203Register(t, path, owner)
	request := path.EndpointA.GetChannel()
	header := ctx.BlockHeader()
	dir := t.TempDir()
	consumer, store, importedCtx, stop := repairM1203Open(t, dir, header)
	var restoredHost icatypes.HostGenesisState
	var restoredCore ibctypes.GenesisState
	var restoredAuth authtypes.GenesisState
	var restoredCaps capabilitytypes.GenesisState
	consumerCodec := consumer.AppCodec()
	require.NoError(t, consumerCodec.UnmarshalAsJSON(hostBytes, &restoredHost))
	require.NoError(t, consumerCodec.UnmarshalAsJSON(coreBytes, &restoredCore))
	require.NoError(t, consumerCodec.UnmarshalAsJSON(authBytes, &restoredAuth))
	require.NoError(t, consumerCodec.UnmarshalAsJSON(capBytes, &restoredCaps))
	require.NoError(t, restoredHost.Validate())
	capability.InitGenesis(importedCtx, *consumer.CapabilityKeeper, restoredCaps)
	auth.InitGenesis(importedCtx, consumer.AccountKeeper, restoredAuth)
	ibc.InitGenesis(importedCtx, *consumer.IBCKeeper, false, &restoredCore)
	hostkeeper.InitGenesis(importedCtx, consumer.ICAHostKeeper, restoredHost)
	commit := store.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	stop()

	reopened, recoveredStore, recoveredCtx, _ := repairM1203Open(t, dir, header)
	require.Equal(t, commit, recoveredStore.LastCommitID())
	recoveredHost := hostkeeper.ExportGenesis(recoveredCtx, reopened.ICAHostKeeper)
	recoveredBytes, err := reopened.AppCodec().MarshalAsJSON(&recoveredHost)
	require.NoError(t, err)
	require.JSONEq(t, string(hostBytes), string(recoveredBytes))
	require.NoError(t, recoveredHost.Validate())
	require.True(t, reopened.ICAHostKeeper.IsBound(recoveredCtx, icatypes.PortID))
	activeID, found := reopened.ICAHostKeeper.GetActiveChannelID(recoveredCtx, path.EndpointB.ConnectionID, port)
	require.True(t, found)
	require.Equal(t, path.EndpointB.ChannelID, activeID)
	channel, found := reopened.IBCKeeper.ChannelKeeper.GetChannel(recoveredCtx, icatypes.PortID, activeID)
	require.True(t, found)
	require.Equal(t, channeltypes.CLOSED, channel.State)
	require.Equal(t, []string{path.EndpointB.ConnectionID}, channel.ConnectionHops)
	require.Equal(t, port, channel.Counterparty.PortId)
	acc := reopened.AccountKeeper.GetAccount(recoveredCtx, sdk.MustAccAddressFromBech32(address))
	require.IsType(t, &icatypes.InterchainAccount{}, acc)
	require.Equal(t, port, acc.(*icatypes.InterchainAccount).AccountOwner)
	accountNumber := acc.GetAccountNumber()

	nextID := channeltypes.FormatChannelIdentifier(reopened.IBCKeeper.ChannelKeeper.GetNextChannelSequence(recoveredCtx))
	chanCap, err := reopened.ScopedIBCKeeper.NewCapability(recoveredCtx, ibchost.ChannelCapabilityPath(icatypes.PortID, nextID))
	require.NoError(t, err)
	version, err := icahost.NewIBCModule(reopened.ICAHostKeeper).OnChanOpenTry(recoveredCtx,
		request.Ordering, channel.ConnectionHops, icatypes.PortID, nextID, chanCap,
		channeltypes.NewCounterparty(port, path.EndpointA.ChannelID), request.Version)
	require.NoError(t, err)
	var response icatypes.Metadata
	require.NoError(t, reopened.AppCodec().UnmarshalAsJSON([]byte(version), &response))
	require.Equal(t, address, response.Address, "reopening must reuse the recovered account")
	require.NoError(t, controller.GetSimApp().ICAControllerKeeper.OnChanOpenAck(controller.GetContext(), port, path.EndpointA.ChannelID, version))
	remote, found := controller.GetSimApp().ICAControllerKeeper.GetInterchainAccountAddress(controller.GetContext(), path.EndpointA.ConnectionID, port)
	require.True(t, found)
	require.Equal(t, address, remote)
	require.Equal(t, accountNumber, reopened.AccountKeeper.GetAccount(recoveredCtx, sdk.MustAccAddressFromBech32(address)).GetAccountNumber())
	require.Len(t, reopened.ICAHostKeeper.GetAllInterchainAccounts(recoveredCtx), 1)
	require.True(t, reopened.ICAHostKeeper.AuthenticateCapability(recoveredCtx, chanCap, ibchost.ChannelCapabilityPath(icatypes.PortID, nextID)))
}

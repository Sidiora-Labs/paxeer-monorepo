package keeper_test

import (
	"fmt"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ica "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/host/keeper"
	hosttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability"
	capabilitytypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM1204Host(t *testing.T) (*simapp.SimApp, sdk.Context) {
	t.Helper()
	db := dbm.NewMemDB()
	store := rootmulti.NewStore(db)
	options := viper.New()
	options.Set(baseapp.FlagChainID, "ica-port-roundtrip")
	app := simapp.NewSimApp(db, nil, false, map[int64]bool{}, t.TempDir(), 0, nil,
		simapp.MakeTestEncodingConfig(), options, func(b *baseapp.BaseApp) { b.SetCMS(store) })
	require.NoError(t, store.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	ctx := sdk.NewContext(store, tmproto.Header{ChainID: "ica-port-roundtrip", Height: 1}, false)
	capability.InitGenesis(ctx, *app.CapabilityKeeper, *capabilitytypes.DefaultGenesis())
	return app, ctx
}

func TestRepairM_1204_NoncanonicalPortRefused(t *testing.T) {
	for _, port := range []string{"custom-ica-host", "transfer", "ICAHOST", "", "invalid|port"} {
		t.Run(port, func(t *testing.T) {
			app, ctx := repairM1204Host(t)
			state := icatypes.DefaultHostGenesis()
			state.Port = port
			err := state.Validate()
			require.Error(t, err)
			if host.PortIdentifierValidator(port) == nil {
				require.ErrorIs(t, err, icatypes.ErrInvalidHostPort)
			}
			data, err := app.AppCodec().MarshalAsJSON(icatypes.NewGenesisState(icatypes.DefaultControllerGenesis(), state))
			require.NoError(t, err)
			require.Error(t, (ica.AppModuleBasic{}).ValidateGenesis(app.AppCodec(), nil, data))
			index := app.CapabilityKeeper.GetLatestIndex(ctx)
			require.PanicsWithValue(t, fmt.Sprintf("invalid interchain accounts host genesis: %v", state.Validate()), func() {
				keeper.InitGenesis(ctx, app.ICAHostKeeper, state)
			})
			require.Equal(t, index, app.CapabilityKeeper.GetLatestIndex(ctx))
			require.False(t, app.ICAHostKeeper.IsBound(ctx, port))
			require.False(t, app.IBCKeeper.PortKeeper.IsBound(ctx, port))
			require.False(t, ctx.KVStore(app.GetKey(hosttypes.StoreKey)).Has(icatypes.KeyPort(port)))
			require.Empty(t, app.ICAHostKeeper.GetAllActiveChannels(ctx))
			require.Empty(t, app.ICAHostKeeper.GetAllInterchainAccounts(ctx))
			keeper.InitGenesis(ctx, app.ICAHostKeeper, icatypes.DefaultHostGenesis())
			require.True(t, app.ICAHostKeeper.IsBound(ctx, icatypes.PortID))
			require.Equal(t, index+1, app.CapabilityKeeper.GetLatestIndex(ctx))
			canonical := keeper.ExportGenesis(ctx, app.ICAHostKeeper)
			require.PanicsWithValue(t, fmt.Sprintf("invalid interchain accounts host genesis: %v", state.Validate()), func() {
				keeper.InitGenesis(ctx, app.ICAHostKeeper, state)
			})
			require.Equal(t, index+1, app.CapabilityKeeper.GetLatestIndex(ctx))
			require.Equal(t, canonical, keeper.ExportGenesis(ctx, app.ICAHostKeeper))
			require.False(t, app.IBCKeeper.PortKeeper.IsBound(ctx, port))
			require.False(t, ctx.KVStore(app.GetKey(hosttypes.StoreKey)).Has(icatypes.KeyPort(port)))
		})
	}
}

func TestRepairM_1204_CanonicalExportImport(t *testing.T) {
	coordinator := ibctesting.NewCoordinator(t, 2)
	path := ibctesting.NewPath(coordinator.GetChain(ibctesting.GetChainID(1)), coordinator.GetChain(ibctesting.GetChainID(2)))
	path.SetChannelOrdered()
	path.EndpointB.ChannelConfig.PortID = icatypes.PortID
	coordinator.SetupConnections(path)
	controller := path.EndpointA.Chain
	owner := controller.SenderAccount.GetAddress().String()
	port, err := icatypes.NewControllerPortID(owner)
	require.NoError(t, err)
	path.EndpointA.ChannelConfig.PortID = port
	sequence := controller.App.GetIBCKeeper().ChannelKeeper.GetNextChannelSequence(controller.GetContext())
	require.NoError(t, controller.GetSimApp().ICAControllerKeeper.RegisterInterchainAccount(controller.GetContext(), path.EndpointA.ConnectionID, owner))
	path.EndpointA.ChannelID = channeltypes.FormatChannelIdentifier(sequence)
	path.EndpointA.ChannelConfig.Version = path.EndpointA.GetChannel().Version
	coordinator.CommitBlock(controller)
	path.EndpointB.ChannelConfig.Version = path.EndpointA.ChannelConfig.Version
	require.NoError(t, path.EndpointB.ChanOpenTry())
	require.NoError(t, path.EndpointA.ChanOpenAck())
	require.NoError(t, path.EndpointB.ChanOpenConfirm())

	source := path.EndpointB.Chain
	producer := source.GetSimApp()
	params := hosttypes.NewParams(false, []string{"/cosmos.bank.v1beta1.MsgSend"})
	producer.ICAHostKeeper.SetParams(source.GetContext(), params)
	exported := keeper.ExportGenesis(source.GetContext(), producer.ICAHostKeeper)
	require.Equal(t, icatypes.PortID, exported.Port)
	require.Equal(t, params, exported.Params)
	require.NoError(t, exported.Validate())
	require.Len(t, exported.ActiveChannels, 1)
	require.Len(t, exported.InterchainAccounts, 1)
	require.Equal(t, path.EndpointB.ChannelID, exported.ActiveChannels[0].ChannelId)
	remote, found := controller.GetSimApp().ICAControllerKeeper.GetInterchainAccountAddress(controller.GetContext(), path.EndpointA.ConnectionID, port)
	require.True(t, found)
	require.Equal(t, remote, exported.InterchainAccounts[0].AccountAddress)
	encoded, err := producer.AppCodec().MarshalAsJSON(&exported)
	require.NoError(t, err)

	consumer, ctx := repairM1204Host(t)
	var imported icatypes.HostGenesisState
	require.NoError(t, consumer.AppCodec().UnmarshalAsJSON(encoded, &imported))
	require.NoError(t, imported.Validate())
	require.False(t, consumer.ICAHostKeeper.IsBound(ctx, icatypes.PortID))
	keeper.InitGenesis(ctx, consumer.ICAHostKeeper, imported)
	require.True(t, consumer.ICAHostKeeper.IsBound(ctx, icatypes.PortID))
	require.True(t, consumer.IBCKeeper.PortKeeper.IsBound(ctx, icatypes.PortID))
	cap, found := consumer.ScopedICAHostKeeper.GetCapability(ctx, host.PortPath(icatypes.PortID))
	require.True(t, found)
	require.True(t, consumer.ICAHostKeeper.AuthenticateCapability(ctx, cap, host.PortPath(icatypes.PortID)))
	require.Equal(t, exported.ActiveChannels, consumer.ICAHostKeeper.GetAllActiveChannels(ctx))
	require.Equal(t, exported.InterchainAccounts, consumer.ICAHostKeeper.GetAllInterchainAccounts(ctx))
	require.Equal(t, exported.Params, consumer.ICAHostKeeper.GetParams(ctx))
	roundtrip := keeper.ExportGenesis(ctx, consumer.ICAHostKeeper)
	require.NoError(t, roundtrip.Validate())
	encodedAgain, err := consumer.AppCodec().MarshalAsJSON(&roundtrip)
	require.NoError(t, err)
	require.JSONEq(t, string(encoded), string(encodedAgain))
	index := consumer.CapabilityKeeper.GetLatestIndex(ctx)
	keeper.InitGenesis(ctx, consumer.ICAHostKeeper, roundtrip)
	require.Equal(t, index, consumer.CapabilityKeeper.GetLatestIndex(ctx))
	rebound, found := consumer.ScopedICAHostKeeper.GetCapability(ctx, host.PortPath(icatypes.PortID))
	require.True(t, found)
	require.Same(t, cap, rebound)
}

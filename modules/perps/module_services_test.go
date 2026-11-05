package perps_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func TestAppModuleRoutesPerpsMsgs(t *testing.T) {
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := sdktestutil.DefaultContext(key, sdk.NewTransientStoreKey("transient_perps"))
	k := keeper.NewKeeper(key, nil, nil)
	am := perps.NewAppModule(k)
	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(registry)
	queryRouter := baseapp.NewGRPCQueryRouter()
	am.RegisterServices(module.NewConfigurator(cdc, msgRouter, queryRouter))

	for _, typeURL := range []string{
		"/paxprotocol.paxchain.perps.MsgUpdateParams",
		"/paxprotocol.paxchain.perps.MsgSetMarket",
		"/paxprotocol.paxchain.perps.MsgSetPaused",
	} {
		require.NotNil(t, msgRouter.HandlerByTypeURL(typeURL), typeURL)
	}

	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	require.Equal(t, authority, k.Authority())
	msg := &types.MsgSetPaused{Authority: authority, Paused: true}
	amino := codec.NewLegacyAmino()
	am.RegisterLegacyAminoCodec(amino)
	encoded, err := amino.MarshalAsJSON(msg)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"perps/MsgSetPaused","value":{"authority":"`+authority+`","paused":true}}`, string(encoded))

	before, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.False(t, before.Paused)
	handler := msgRouter.HandlerByTypeURL("/paxprotocol.paxchain.perps.MsgSetPaused")
	result, err := handler(ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, result)
	after, err := k.GetParams(ctx)
	require.NoError(t, err)
	want := before
	want.Paused = true
	require.Equal(t, want, after)
}

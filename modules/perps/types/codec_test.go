package types_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestRegisterInterfacesResolvesPerpsMsgs(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)

	for _, tc := range []struct {
		typeURL string
		msg     sdk.Msg
	}{
		{"/paxprotocol.paxchain.perps.MsgUpdateParams", &types.MsgUpdateParams{}},
		{"/paxprotocol.paxchain.perps.MsgSetMarket", &types.MsgSetMarket{}},
		{"/paxprotocol.paxchain.perps.MsgSetPaused", &types.MsgSetPaused{}},
	} {
		t.Run(tc.typeURL, func(t *testing.T) {
			resolved, err := registry.Resolve(tc.typeURL)
			require.NoError(t, err)
			require.IsType(t, tc.msg, resolved)
			require.Implements(t, (*sdk.Msg)(nil), resolved)

			packed, err := cdctypes.NewAnyWithValue(tc.msg)
			require.NoError(t, err)
			require.Equal(t, tc.typeURL, packed.TypeUrl)
			var unpacked sdk.Msg
			require.NoError(t, registry.UnpackAny(&cdctypes.Any{
				TypeUrl: packed.TypeUrl,
				Value:   packed.Value,
			}, &unpacked))
			require.IsType(t, tc.msg, unpacked)
		})
	}
}

func TestLegacyAminoNamesPerpsMsgs(t *testing.T) {
	cdc := codec.NewLegacyAmino()
	types.RegisterLegacyAminoCodec(cdc)

	for _, tc := range []struct {
		name string
		msg  sdk.Msg
	}{
		{"perps/MsgUpdateParams", &types.MsgUpdateParams{Authority: "governance", Params: types.DefaultParams()}},
		{"perps/MsgSetMarket", &types.MsgSetMarket{Authority: "governance", Market: types.Market{
			ID: 7, Symbol: "BTC", ImpactNotionalUsd: sdk.NewInt(20_000), OiCapUsd: sdk.NewInt(100_000_000),
		}}},
		{"perps/MsgSetPaused", &types.MsgSetPaused{Authority: "governance", Paused: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := cdc.MarshalAsJSON(tc.msg)
			require.NoError(t, err)
			var envelope struct {
				Type  string `json:"type"`
				Value struct {
					Authority string `json:"authority"`
				} `json:"value"`
			}
			require.NoError(t, json.Unmarshal(raw, &envelope))
			require.Equal(t, tc.name, envelope.Type)
			require.Equal(t, "governance", envelope.Value.Authority)
		})
	}
}

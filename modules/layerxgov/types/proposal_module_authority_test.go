package types

import (
	"encoding/json"
	"testing"

	agent "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	pay "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	perps "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeed "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	spot "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func assertModuleAuthorityMessage(t *testing.T, msg sdk.Msg, authority, name, typeURL string) {
	t.Helper()
	got, err := governanceMessage(msg)
	require.NoError(t, err)
	require.Equal(t, authority, got)
	require.Equal(t, typeURL, sdk.MsgTypeURL(msg))

	registry := cdctypes.NewInterfaceRegistry()
	for _, register := range []func(cdctypes.InterfaceRegistry){
		pay.RegisterInterfaces, agent.RegisterInterfaces, perps.RegisterInterfaces,
		pricefeed.RegisterInterfaces, spot.RegisterInterfaces,
	} {
		register(registry)
	}
	resolved, err := registry.Resolve(typeURL)
	require.NoError(t, err)
	require.IsType(t, msg, resolved)

	proposal, err := NewLayerXProposal("Configure module", "Update module authority settings", msg)
	require.NoError(t, err)
	raw, err := govtypes.ModuleCdc.MarshalAsJSON(proposal)
	require.NoError(t, err)
	var encoded struct {
		Value struct {
			Messages []struct {
				Type string `json:"type"`
			} `json:"messages"`
		} `json:"value"`
	}
	require.NoError(t, json.Unmarshal(raw, &encoded))
	require.Len(t, encoded.Value.Messages, 1)
	require.Equal(t, name, encoded.Value.Messages[0].Type)

	decoder := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(decoder)
	govtypes.RegisterLegacyAminoCodec(decoder)
	for _, register := range []func(*codec.LegacyAmino){
		RegisterCodec, pay.RegisterCodec, agent.RegisterCodec, perps.RegisterLegacyAminoCodec,
		pricefeed.RegisterCodec, spot.RegisterCodec,
	} {
		register(decoder)
	}
	var decoded LayerXProposal
	require.NoError(t, decoder.UnmarshalAsJSON(raw, &decoded))
	messages, err := decoded.GetMessages()
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.IsType(t, msg, messages[0])
	require.Equal(t, typeURL, decoded.Messages[0].TypeUrl)
	roundAuthority, err := governanceMessage(messages[0])
	require.NoError(t, err)
	require.Equal(t, authority, roundAuthority)
}

func TestGovernanceMessagePayUpdateParams(t *testing.T) {
	authority := GovernanceAuthority()
	assertModuleAuthorityMessage(t, &pay.MsgUpdateParams{Authority: authority}, authority,
		"layerxpay/MsgUpdateParams", "/paxprotocol.paxchain.layerxpay.MsgUpdateParams")
}

func TestGovernanceMessageAgentUpdateParams(t *testing.T) {
	authority := GovernanceAuthority()
	assertModuleAuthorityMessage(t, &agent.MsgUpdateParams{Authority: authority}, authority,
		"layerxagent/MsgUpdateParams", "/paxprotocol.paxchain.layerxagent.MsgUpdateParams")
}

func TestGovernanceMessagePerpsAuthority(t *testing.T) {
	authority := GovernanceAuthority()
	for _, test := range []struct {
		name string
		msg  sdk.Msg
	}{
		{"MsgUpdateParams", &perps.MsgUpdateParams{Authority: authority, Params: perps.DefaultParams()}},
		{"MsgSetMarket", &perps.MsgSetMarket{Authority: authority}},
		{"MsgSetPaused", &perps.MsgSetPaused{Authority: authority, Paused: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertModuleAuthorityMessage(t, test.msg, authority,
				"perps/"+test.name, "/paxprotocol.paxchain.perps."+test.name)
		})
	}
}

func TestGovernanceMessagePricefeedSetParams(t *testing.T) {
	authority := GovernanceAuthority()
	assertModuleAuthorityMessage(t, &pricefeed.MsgSetParams{Authority: authority}, authority,
		"pricefeed/MsgSetParams", "/paxprotocol.paxchain.pricefeed.MsgSetParams")
}

func TestGovernanceMessageSpotAuthority(t *testing.T) {
	authority := GovernanceAuthority()
	for _, test := range []struct {
		name string
		msg  sdk.Msg
	}{
		{"MsgUpdateParams", &spot.MsgUpdateParams{Authority: authority, Params: spot.DefaultParams()}},
		{"MsgSyncReserves", &spot.MsgSyncReserves{Authority: authority, Tokens: []string{"0x0000000000000000000000000000000000000001"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertModuleAuthorityMessage(t, test.msg, authority,
				"spot/"+test.name, "/paxprotocol.paxchain.spot."+test.name)
		})
	}
}

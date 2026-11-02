package types_test

import (
	"bytes"
	"encoding/json"
	"testing"

	bridge "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func TestLayerXProposalBridgeAminoJSON(t *testing.T) {
	decoder := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(decoder)
	govtypes.RegisterLegacyAminoCodec(decoder)
	types.RegisterCodec(decoder)
	bridge.RegisterCodec(decoder)

	authority := types.GovernanceAuthority()
	proposer := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
	for _, test := range []struct {
		name string
		msg  sdk.Msg
	}{
		{"layerxbridge/MsgRegisterChain", &bridge.MsgRegisterChain{
			Authority: authority,
			Chain:     bridge.Chain{ChainID: 1, Vault: bridge.Address20{1}, FinalityDepth: 12, Enabled: true},
		}},
		{"layerxbridge/MsgSetAttestors", &bridge.MsgSetAttestors{
			Authority: authority,
			Set: bridge.AttestorSet{
				Attestors: []bridge.Attestor{{Signer: bridge.Address20{2}, Bond: sdk.NewInt(100)}},
				Threshold: 1,
			},
		}},
		{"layerxbridge/MsgSetCap", &bridge.MsgSetCap{
			Authority: authority, ChainID: 1, Asset: bridge.Address20{3},
			MaxInFlight: sdk.NewInt(1000), MaxPerTx: sdk.NewInt(100),
		}},
		{"layerxbridge/MsgPause", &bridge.MsgPause{Authority: authority}},
		{"layerxbridge/MsgUnpause", &bridge.MsgUnpause{Authority: authority}},
		{"layerxbridge/MsgRegisterSidioraPair", &bridge.MsgRegisterSidioraPair{
			Authority: authority, ChainID: bridge.SidioraHomeChainID,
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			proposal, err := types.NewLayerXProposal("Configure bridge", "Update bridge governance settings", test.msg)
			require.NoError(t, err)
			submit, err := govtypes.NewMsgSubmitProposal(proposal, sdk.NewCoins(sdk.NewInt64Coin("stake", 10)), proposer)
			require.NoError(t, err)
			require.NoError(t, submit.ValidateBasic())

			raw, err := govtypes.ModuleCdc.MarshalAsJSON(submit)
			require.NoError(t, err)
			var encoded struct {
				Type  string `json:"type"`
				Value struct {
					Content struct {
						Type  string `json:"type"`
						Value struct {
							Messages []struct {
								Type string `json:"type"`
							} `json:"messages"`
						} `json:"value"`
					} `json:"content"`
				} `json:"value"`
			}
			require.NoError(t, json.Unmarshal(raw, &encoded))
			require.Equal(t, "cosmos-sdk/MsgSubmitProposal", encoded.Type)
			require.Equal(t, "layerxgov/LayerXProposal", encoded.Value.Content.Type)
			require.Len(t, encoded.Value.Content.Value.Messages, 1)
			require.Equal(t, test.name, encoded.Value.Content.Value.Messages[0].Type)
			require.Equal(t, sdk.MustSortJSON(raw), submit.GetSignBytes())

			var decoded govtypes.MsgSubmitProposal
			require.NoError(t, decoder.UnmarshalAsJSON(raw, &decoded))
			require.NoError(t, decoded.ValidateBasic())
			require.Equal(t, submit.Proposer, decoded.Proposer)
			require.Equal(t, submit.InitialDeposit, decoded.InitialDeposit)
			require.IsType(t, &types.LayerXProposal{}, decoded.GetContent())
			round := decoded.GetContent().(*types.LayerXProposal)
			require.Equal(t, proposal.Title, round.Title)
			require.Equal(t, proposal.Description, round.Description)
			messages, err := round.GetMessages()
			require.NoError(t, err)
			require.Len(t, messages, 1)
			require.Equal(t, test.msg, messages[0])
		})
	}
}

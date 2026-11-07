package types_test

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	pay "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func TestLayerXProposalRoutesPayPause(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	govtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	pay.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	govtypes.RegisterLegacyAminoCodec(amino)
	types.RegisterCodec(amino)
	pay.RegisterCodec(amino)

	authority := types.GovernanceAuthority()
	proposer := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
	for _, tc := range []struct {
		name   string
		paused bool
	}{
		{"pause", true},
		{"unpause", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := pay.DefaultParams()
			params.Paused = tc.paused
			msg := &pay.MsgUpdateParams{Authority: authority, Params: params}
			proposal, err := types.NewLayerXProposal("Set pay pause", "Govern pay pause while preserving exits", msg)
			require.NoError(t, err)
			require.NoError(t, proposal.ValidateBasic())
			require.Equal(t, sdk.MsgTypeURL(msg), proposal.Messages[0].TypeUrl)

			raw, err := cdc.MarshalInterface(proposal)
			require.NoError(t, err)
			var content govtypes.Content
			require.NoError(t, cdc.UnmarshalInterface(raw, &content))
			require.IsType(t, &types.LayerXProposal{}, content)

			submit, err := govtypes.NewMsgSubmitProposal(proposal, sdk.NewCoins(), proposer)
			require.NoError(t, err)
			require.NoError(t, submit.ValidateBasic())
			raw, err = govtypes.ModuleCdc.MarshalAsJSON(submit)
			require.NoError(t, err)
			require.Contains(t, string(raw), `"layerxpay/MsgUpdateParams"`)
			require.Equal(t, sdk.MustSortJSON(raw), submit.GetSignBytes())
			var decoded govtypes.MsgSubmitProposal
			require.NoError(t, amino.UnmarshalAsJSON(raw, &decoded))
			require.NoError(t, decoded.ValidateBasic())
			require.IsType(t, &types.LayerXProposal{}, decoded.GetContent())

			for _, round := range []*types.LayerXProposal{proposal, content.(*types.LayerXProposal), decoded.GetContent().(*types.LayerXProposal)} {
				require.NoError(t, round.ValidateBasic())
				messages, err := round.GetMessages()
				require.NoError(t, err)
				require.Len(t, messages, 1)
				require.IsType(t, &pay.MsgUpdateParams{}, messages[0])
				carried := messages[0].(*pay.MsgUpdateParams)
				require.Equal(t, msg, carried)
				for _, method := range []string{"escrowRelease", "escrowTimeout", "budgetClose", "budgetDefund", "budgetRevoke", "streamPause", "streamClose", "serviceOfferWithdraw", "serviceCommitAbandon"} {
					require.NoError(t, types.CheckPauseScope(pay.ModuleName, carried.Params.Paused, method), method)
				}
				if tc.paused {
					require.ErrorIs(t, types.CheckPauseScope(pay.ModuleName, carried.Params.Paused, "budgetSpend"), types.ErrPausedScope)
				} else {
					require.NoError(t, types.CheckPauseScope(pay.ModuleName, carried.Params.Paused, "budgetSpend"))
				}
			}
		})
	}

	outsider := sdk.AccAddress(bytes.Repeat([]byte{2}, 20)).String()
	proposal, err := types.NewLayerXProposal("Set pay pause", "Refuse non-governance authority", &pay.MsgUpdateParams{Authority: outsider, Params: pay.DefaultParams()})
	require.NoError(t, err)
	require.ErrorIs(t, proposal.ValidateBasic(), govtypes.ErrInvalidProposalContent)
	_, err = types.NewLayerXProposal("Set pay pause", "Refuse nil pay message", (*pay.MsgUpdateParams)(nil))
	require.ErrorIs(t, err, govtypes.ErrInvalidProposalContent)
}

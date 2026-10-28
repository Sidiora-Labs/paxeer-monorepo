package keeper

import (
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	wasmvmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/stretchr/testify/require"
)

func TestEncodeGovMsgVoteWeightedIsRefused(t *testing.T) {
	sender := RandomAccountAddress(t)
	msg := &wasmvmtypes.GovMsg{
		VoteWeighted: &wasmvmtypes.VoteWeightedMsg{
			ProposalId: 42,
			Options: []wasmvmtypes.WeightedVoteOption{
				{Option: wasmvmtypes.Yes, Weight: "0.75"},
				{Option: wasmvmtypes.No, Weight: "0.25"},
			},
		},
	}
	encoders := map[string]func(sdk.AccAddress, *wasmvmtypes.GovMsg) ([]sdk.Msg, error){
		"direct": EncodeGovMsg,
		"default": func(sender sdk.AccAddress, msg *wasmvmtypes.GovMsg) ([]sdk.Msg, error) {
			return DefaultEncoders(nil, nil).Encode(sdk.Context{}, sender, "", wasmvmtypes.CosmosMsg{Gov: msg}, wasmvmtypes.MessageInfo{}, types.CodeInfo{})
		},
	}
	for name, encode := range encoders {
		t.Run(name, func(t *testing.T) {
			msgs, err := encode(sender, msg)
			require.ErrorIs(t, err, types.ErrUnknownMsg)
			require.Contains(t, err.Error(), "Unknown variant of Gov")
			require.Nil(t, msgs)
		})
	}
}

func TestEncodeGovMsgWithoutAVariantIsRefused(t *testing.T) {
	sender := RandomAccountAddress(t)
	cases := []struct {
		name   string
		msg    wasmvmtypes.GovMsg
		option govtypes.VoteOption
	}{
		{name: "empty"},
		{
			name: "unset option",
			msg:  wasmvmtypes.GovMsg{Vote: &wasmvmtypes.VoteMsg{ProposalId: 42, Vote: wasmvmtypes.UnsetVoteOption}},
		},
		{
			name: "unknown option",
			msg:  wasmvmtypes.GovMsg{Vote: &wasmvmtypes.VoteMsg{ProposalId: 42, Vote: 99}},
		},
		{
			name:   "yes",
			msg:    wasmvmtypes.GovMsg{Vote: &wasmvmtypes.VoteMsg{ProposalId: 42, Vote: wasmvmtypes.Yes}},
			option: govtypes.OptionYes,
		},
		{
			name:   "no",
			msg:    wasmvmtypes.GovMsg{Vote: &wasmvmtypes.VoteMsg{ProposalId: 42, Vote: wasmvmtypes.No}},
			option: govtypes.OptionNo,
		},
		{
			name:   "no with veto",
			msg:    wasmvmtypes.GovMsg{Vote: &wasmvmtypes.VoteMsg{ProposalId: 42, Vote: wasmvmtypes.NoWithVeto}},
			option: govtypes.OptionNoWithVeto,
		},
		{
			name:   "abstain",
			msg:    wasmvmtypes.GovMsg{Vote: &wasmvmtypes.VoteMsg{ProposalId: 42, Vote: wasmvmtypes.Abstain}},
			option: govtypes.OptionAbstain,
		},
	}
	encoders := map[string]func(sdk.AccAddress, *wasmvmtypes.GovMsg) ([]sdk.Msg, error){
		"direct": EncodeGovMsg,
		"default": func(sender sdk.AccAddress, msg *wasmvmtypes.GovMsg) ([]sdk.Msg, error) {
			return DefaultEncoders(nil, nil).Encode(sdk.Context{}, sender, "", wasmvmtypes.CosmosMsg{Gov: msg}, wasmvmtypes.MessageInfo{}, types.CodeInfo{})
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for name, encode := range encoders {
				t.Run(name, func(t *testing.T) {
					msgs, err := encode(sender, &tc.msg)
					if tc.option == govtypes.OptionEmpty {
						require.ErrorIs(t, err, types.ErrUnknownMsg)
						require.Contains(t, err.Error(), "Unknown variant of Gov")
						require.Nil(t, msgs)
						return
					}
					require.NoError(t, err)
					require.Equal(t, []sdk.Msg{&govtypes.MsgVote{
						ProposalId: 42,
						Voter:      sender.String(),
						Option:     tc.option,
					}}, msgs)
				})
			}
		})
	}
}

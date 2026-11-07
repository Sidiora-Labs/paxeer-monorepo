package types_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func repairM570Submit(t *testing.T, proposal *types.UpdateMinterProposal, transport string) *govtypes.MsgSubmitProposal {
	t.Helper()
	registry := cdctypes.NewInterfaceRegistry()
	govtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	msg, err := govtypes.NewMsgSubmitProposal(proposal,
		sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1)),
		sdk.AccAddress(bytes.Repeat([]byte{1}, 20)))
	require.NoError(t, err)

	var decoded govtypes.MsgSubmitProposal
	if transport == "protobuf" {
		wire, err := cdc.Marshal(msg)
		require.NoError(t, err)
		require.NoError(t, cdc.Unmarshal(wire, &decoded))
	} else {
		var wire []byte
		if transport == "json_omitted" {
			marshaler := &jsonpb.Marshaler{OrigName: true, AnyResolver: registry}
			require.NoError(t, cdctypes.UnpackInterfaces(msg, cdctypes.ProtoJSONPacker{JSONPBMarshaler: marshaler}))
			var buffer bytes.Buffer
			require.NoError(t, marshaler.Marshal(&buffer, msg))
			wire = buffer.Bytes()
		} else {
			wire, err = cdc.MarshalAsJSON(msg)
			require.NoError(t, err)
		}
		if proposal.Minter == nil {
			var envelope struct {
				Content map[string]json.RawMessage `json:"content"`
			}
			require.NoError(t, json.Unmarshal(wire, &envelope))
			if transport == "json_omitted" {
				require.NotContains(t, envelope.Content, "minter")
			} else {
				require.JSONEq(t, "null", string(envelope.Content["minter"]))
			}
		}
		require.NoError(t, cdc.UnmarshalAsJSON(wire, &decoded))
	}
	content, ok := decoded.GetContent().(*types.UpdateMinterProposal)
	require.True(t, ok, "transport must unpack the production mint proposal")
	require.Equal(t, proposal, content)
	return &decoded
}

func TestRepairM_570_MissingMinter(t *testing.T) {
	for _, transport := range []string{"protobuf", "json", "json_omitted"} {
		t.Run(transport, func(t *testing.T) {
			proposal := &types.UpdateMinterProposal{Title: "Update minter", Description: "Set the mint schedule"}
			msg := repairM570Submit(t, proposal, transport)
			require.Nil(t, msg.GetContent().(*types.UpdateMinterProposal).Minter)
			for attempt := 0; attempt < 2; attempt++ {
				var err error
				require.NotPanics(t, func() { err = msg.ValidateBasic() })
				require.ErrorIs(t, err, govtypes.ErrInvalidProposalContent)
				require.ErrorContains(t, err, "minter cannot be nil")
			}
			require.Nil(t, msg.GetContent().(*types.UpdateMinterProposal).Minter)
		})
	}
}

func TestRepairM_570_AbstractFields(t *testing.T) {
	cases := []struct {
		name, title, description, want string
	}{
		{"empty_title", "", "Schedule", "proposal title cannot be blank"},
		{"whitespace_title", " \t\n", "Schedule", "proposal title cannot be blank"},
		{"long_title", strings.Repeat("t", govtypes.MaxTitleLength+1), "Schedule", "proposal title is longer than max length"},
		{"empty_description", "Update minter", "", "proposal description cannot be blank"},
		{"long_description", "Update minter", strings.Repeat("d", govtypes.MaxDescriptionLength+1), "proposal description is longer than max length"},
	}
	for _, tc := range cases {
		for _, transport := range []string{"protobuf", "json", "json_omitted"} {
			for _, minterState := range []string{"valid", "invalid", "missing"} {
				t.Run(tc.name+"/"+transport+"/"+minterState, func(t *testing.T) {
					proposal := types.NewUpdateMinterProposalHandler(tc.title, tc.description, types.DefaultInitialMinter())
					if minterState == "invalid" {
						proposal.Minter.StartDate = "invalid"
					} else if minterState == "missing" {
						proposal.Minter = nil
					}
					err := repairM570Submit(t, proposal, transport).ValidateBasic()
					require.ErrorIs(t, err, govtypes.ErrInvalidProposalContent)
					require.ErrorContains(t, err, tc.want)
				})
			}
		}
	}
}

func TestRepairM_570_MinterValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*types.Minter)
	}{
		{"date", func(m *types.Minter) { m.StartDate = "invalid" }},
		{"amount", func(m *types.Minter) { m.RemainingMintAmount = m.TotalMintAmount + 1 }},
		{"denom", func(m *types.Minter) { m.Denom = "" }},
	}
	for _, tc := range cases {
		for _, transport := range []string{"protobuf", "json", "json_omitted"} {
			t.Run(tc.name+"/"+transport, func(t *testing.T) {
				proposal := types.NewUpdateMinterProposalHandler("Update minter", "Set the mint schedule", types.DefaultInitialMinter())
				tc.mutate(proposal.Minter)
				want := types.ValidateMinter(*proposal.Minter)
				require.Error(t, want)
				require.EqualError(t, repairM570Submit(t, proposal, transport).ValidateBasic(), want.Error())
			})
		}
	}
}

func TestRepairM_570_ValidProposal(t *testing.T) {
	for _, transport := range []string{"protobuf", "json", "json_omitted"} {
		for _, boundary := range []bool{false, true} {
			title, description := "Update minter", "Set the mint schedule"
			if boundary {
				title = strings.Repeat("t", govtypes.MaxTitleLength)
				description = strings.Repeat("d", govtypes.MaxDescriptionLength)
			}
			t.Run(transport+"/"+title[:1], func(t *testing.T) {
				proposal := types.NewUpdateMinterProposalHandler(title, description, types.DefaultInitialMinter())
				require.NoError(t, repairM570Submit(t, proposal, transport).ValidateBasic())
			})
		}
	}
}

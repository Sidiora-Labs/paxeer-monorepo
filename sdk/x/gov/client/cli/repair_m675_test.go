package cli_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/cli"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func repair675CastBatch(t *testing.T, chain *repair308Chain, msgs ...sdk.Msg) {
	t.Helper()
	txs := make([]tmtypes.Tx, 0, len(msgs))
	results := make([]*abci.TxResultV2, 0, len(msgs))
	for index, msg := range msgs {
		builder := chain.encoding.TxConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		txBytes, err := chain.encoding.TxConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		decoded, err := chain.encoding.TxConfig.TxDecoder()(txBytes)
		require.NoError(t, err)
		res := chain.app.DeliverTx(chain.ctx, abci.RequestDeliverTxV2{Tx: txBytes}, decoded, sha256.Sum256(txBytes))
		require.Zero(t, res.Code, res.Log)
		txs = append(txs, txBytes)
		results = append(results, &abci.TxResultV2{Height: 1, Index: uint32(index), Tx: txBytes, Result: abci.ExecTxResult{
			Code: res.Code, Codespace: res.Codespace, Data: res.Data, Log: res.Log,
			Info: res.Info, GasWanted: res.GasWanted, GasUsed: res.GasUsed, Events: res.Events,
		}})
	}
	env := chain.node.RPCEnvironment()
	block := tmtypes.MakeBlock(1, txs, &tmtypes.Commit{}, nil)
	block.ChainID = "repair-308"
	block.Time = chain.ctx.BlockTime()
	pubKey, err := cryptocodec.ToTmPubKeyInterface(ed25519.GenPrivKeyFromSecret([]byte("repair-308-validator")).PubKey())
	require.NoError(t, err)
	validators := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(pubKey, 1)})
	block.ProposerAddress = pubKey.Address()
	block.ValidatorsHash = validators.Hash()
	block.NextValidatorsHash = validators.Hash()
	require.NoError(t, block.ValidateBasic(tmtypes.ConsensusPolicy{}))
	parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
	require.NoError(t, err)
	env.BlockStore.SaveBlock(block, parts, &tmtypes.Commit{Height: 1})
	for _, sink := range env.EventSinks {
		require.NoError(t, sink.IndexTxEvents(results))
	}
}

func TestRepairM_675_QueryCommands(t *testing.T) {
	for _, name := range []string{"params", "param", "proposer", "tally", "proposal", "proposals", "deposits", "deposit", "votes", "vote"} {
		t.Run(name, func(t *testing.T) { repair675QueryCase(t, name) })
	}
}

func TestRepairM_675_FinalizedHistoryPaginationAndRecovery(t *testing.T) {
	chain := newRepair308Chain(t, "kv")
	voters := []sdk.AccAddress{
		sdk.AccAddress(bytes.Repeat([]byte{21}, 20)),
		sdk.AccAddress(bytes.Repeat([]byte{22}, 20)),
		sdk.AccAddress(bytes.Repeat([]byte{23}, 20)),
	}
	options := []govtypes.WeightedVoteOptions{
		govtypes.NewNonSplitVoteOption(govtypes.OptionYes),
		{{Option: govtypes.OptionYes, Weight: sdk.MustNewDecFromStr("0.7")}, {Option: govtypes.OptionAbstain, Weight: sdk.MustNewDecFromStr("0.3")}},
		govtypes.NewNonSplitVoteOption(govtypes.OptionNo),
	}
	repair675CastBatch(t, chain,
		govtypes.NewMsgVoteWeighted(voters[0], chain.proposal.ProposalId, options[0]),
		govtypes.NewMsgVoteWeighted(voters[1], chain.proposal.ProposalId, options[1]),
		govtypes.NewMsgVoteWeighted(voters[2], chain.proposal.ProposalId, options[2]),
	)
	chain.commit()
	oldClient := chain.client(t)
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	output, err := repair675Execute(ctx, oldClient, cli.GetCmdQueryVotes(), "1", "--page=2", "--limit=1", "--output=json")
	require.NoError(t, err)
	var live govtypes.QueryVotesResponse
	require.NoError(t, oldClient.Codec.UnmarshalAsJSON([]byte(output), &live))
	require.Len(t, live.Votes, 1)
	require.Equal(t, voters[1].String(), live.Votes[0].Voter)
	require.Equal(t, options[1], govtypes.WeightedVoteOptions(live.Votes[0].Options))
	stop()
	chain.tally(t)
	for _, voter := range voters {
		_, found := chain.keeper.GetVote(chain.ctx, chain.proposal.ProposalId, voter)
		require.False(t, found, "finalization must remove live votes before history is queried")
	}
	fresh := chain.client(t)
	for _, tc := range []struct {
		page, limit int
		indices     []int
	}{
		{1, 1, []int{0}}, {2, 1, []int{1}}, {3, 1, []int{2}}, {2, 2, []int{2}}, {4, 1, nil}, {1, 0, []int{0, 1, 2}},
	} {
		t.Run(fmt.Sprintf("page=%d/limit=%d", tc.page, tc.limit), func(t *testing.T) {
			output, err := repair675Execute(t.Context(), fresh, cli.GetCmdQueryVotes(), "1", fmt.Sprintf("--page=%d", tc.page), fmt.Sprintf("--limit=%d", tc.limit), "--output=json")
			require.NoError(t, err)
			var recovered govtypes.Votes
			require.NoError(t, fresh.LegacyAmino.UnmarshalAsJSON([]byte(output), &recovered))
			require.Len(t, recovered, len(tc.indices))
			for i, index := range tc.indices {
				require.Equal(t, chain.proposal.ProposalId, recovered[i].ProposalId)
				require.Equal(t, voters[index].String(), recovered[i].Voter)
				require.Equal(t, options[index], govtypes.WeightedVoteOptions(recovered[i].Options))
			}
		})
	}
	output, err = repair675Execute(t.Context(), fresh, cli.GetCmdQueryVote(), "1", voters[1].String(), "--output=json")
	require.NoError(t, err)
	var single govtypes.Vote
	require.NoError(t, fresh.Codec.UnmarshalAsJSON([]byte(output), &single))
	require.Equal(t, voters[1].String(), single.Voter)
	require.Equal(t, chain.proposal.ProposalId, single.ProposalId)
	require.Equal(t, options[1], govtypes.WeightedVoteOptions(single.Options))
}

func TestRepairM_675_HistoryPaginationErrors(t *testing.T) {
	chain := newRepair308Chain(t, "null")
	chain.commit()
	chain.tally(t)
	clientCtx := chain.client(t)
	maxInt := uint64(^uint(0) >> 1)
	for _, args := range [][]string{
		{"--page=" + strconv.FormatUint(maxInt+1, 10)},
		{"--limit=" + strconv.FormatUint(maxInt+1, 10)},
		{"--page=2", "--limit=" + strconv.FormatUint(maxInt, 10)},
		{"--page=" + strconv.FormatUint(maxInt, 10), "--limit=0"},
	} {
		output, err := repair675Execute(t.Context(), clientCtx, cli.GetCmdQueryVotes(), append([]string{"1"}, args...)...)
		require.ErrorContains(t, err, "supported history pagination range")
		require.Empty(t, output)
	}
	for _, flag := range []string{flags.FlagPage, flags.FlagLimit} {
		cmd := cli.GetCmdQueryVotes()
		wrong := pflag.NewFlagSet("wrong-type", pflag.ContinueOnError)
		wrong.Int(flag, 1, "")
		cmd.Flags().Lookup(flag).Value = wrong.Lookup(flag).Value
		output, err := repair675Execute(t.Context(), clientCtx, cmd, "1", "--node=")
		require.ErrorContains(t, err, "uint64")
		require.Empty(t, output)
	}
	output, err := repair675Execute(t.Context(), clientCtx, cli.GetCmdQueryVotes(), "1", "--page=2", "--limit=1")
	require.ErrorContains(t, err, "transaction searching is disabled")
	require.Empty(t, output)
}

func TestRepairM_675_QueryTransportFailure(t *testing.T) {
	enc := repair308Encoding()
	clientCtx := client.Context{}.WithCodec(enc.Codec).WithLegacyAmino(enc.Amino).WithOutputFormat("json")
	for _, cmd := range []*cobra.Command{cli.GetCmdQueryProposal(), cli.GetCmdQueryVotes(), cli.GetCmdQueryDeposits(), cli.GetCmdQueryTally()} {
		output, err := repair675Execute(t.Context(), clientCtx, cmd, "1", "--node=")
		require.ErrorContains(t, err, "no RPC client")
		require.Empty(t, output)
	}
}

func TestRepairM_675_ExpeditedSubmission(t *testing.T) {
	enc := repair308Encoding()
	proposer := sdk.AccAddress(bytes.Repeat([]byte{31}, 20))
	clientCtx := client.Context{}.WithCodec(enc.Codec).WithLegacyAmino(enc.Amino).
		WithInterfaceRegistry(enc.InterfaceRegistry).WithTxConfig(enc.TxConfig).
		WithChainID("repair-675").WithOutputFormat("json").WithKeyring(keyring.NewInMemory())
	proposalFile := filepath.Join(t.TempDir(), "proposal.json")
	require.NoError(t, os.WriteFile(proposalFile, []byte(`{"title":"Expedited proposal","description":"Preserve the expedited intent","type":"Text","is_expedited":true}`), 0600))
	paths := [][]string{
		{"--title=Expedited proposal", "--description=Preserve the expedited intent", "--type=Text", "--is-expedited=true"},
		{"--proposal=" + proposalFile},
	}
	var first []byte
	for index, args := range paths {
		output, err := repair675Execute(t.Context(), clientCtx, cli.NewCmdSubmitProposal(), append(args, "--generate-only", "--offline", "--from="+proposer.String())...)
		require.NoError(t, err)
		tx, err := enc.TxConfig.TxJSONDecoder()([]byte(output))
		require.NoError(t, err)
		require.Len(t, tx.GetMsgs(), 1)
		msg, ok := tx.GetMsgs()[0].(*govtypes.MsgSubmitProposal)
		require.True(t, ok)
		require.NoError(t, msg.ValidateBasic())
		require.True(t, msg.IsExpedited)
		require.Equal(t, proposer.String(), msg.Proposer)
		text, ok := msg.GetContent().(*govtypes.TextProposal)
		require.True(t, ok)
		require.True(t, text.IsExpedited)
		require.Equal(t, "Expedited proposal", text.Title)
		encoded, err := enc.Codec.Marshal(msg)
		require.NoError(t, err)
		if index == 0 {
			first = encoded
		} else {
			require.Equal(t, first, encoded)
		}
		chain := newRepair308Chain(t, "null")
		chain.cast(t, msg, false)
		chain.commit()
		submitted, found := chain.keeper.GetProposal(chain.ctx, 2)
		require.True(t, found)
		require.True(t, submitted.IsExpedited)
		require.True(t, submitted.GetContent().(*govtypes.TextProposal).IsExpedited)
	}
	for _, flag := range []string{"--is-expedited=true", "--is-expedited=false", "--title=", "--description=x", "--type=Text", "--deposit=1stake"} {
		output, err := repair675Execute(t.Context(), clientCtx, cli.NewCmdSubmitProposal(), "--proposal="+proposalFile, flag, "--generate-only", "--offline", "--from="+proposer.String())
		require.ErrorContains(t, err, "provided alongside --proposal")
		require.Empty(t, output)
	}
	output, err := repair675Execute(t.Context(), clientCtx, cli.NewCmdSubmitProposal(), "--is-expedited=invalid", "--generate-only", "--from="+proposer.String())
	require.ErrorContains(t, err, "invalid")
	require.Empty(t, output)
}

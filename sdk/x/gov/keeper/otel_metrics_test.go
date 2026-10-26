package keeper_test

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func proposalTotal(reader *oteltest.Reader) int64 {
	return reader.CounterSum("paxcosmos_x_gov_keeper", "proposal")
}

func voteTotal(reader *oteltest.Reader) int64 {
	return reader.CounterSum("paxcosmos_x_gov_keeper", "vote")
}

func depositTotal(reader *oteltest.Reader) int64 {
	return reader.CounterSum("paxcosmos_x_gov_keeper", "deposit")
}

func TestMsgServerRecordsProposalVoteAndDepositTotals(t *testing.T) {
	reader := oteltest.Install()
	app := paxapp.Setup(t, false, false, false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})
	addrs := paxapp.AddTestAddrsIncremental(app, ctx, 3, sdk.NewInt(30000000))
	server := keeper.NewMsgServerImpl(app.GovKeeper)
	goCtx := sdk.WrapSDKContext(ctx)
	initialDeposit := app.GovKeeper.GetDepositParams(ctx).MinDeposit
	msg, err := types.NewMsgSubmitProposal(TestProposal, initialDeposit, addrs[0])
	require.NoError(t, err)

	proposalsBefore := proposalTotal(reader)
	votesBefore := voteTotal(reader)
	depositsBefore := depositTotal(reader)

	proposalResponse, err := server.SubmitProposal(goCtx, msg)
	require.NoError(t, err)
	require.NotNil(t, proposalResponse)
	require.Equal(t, int64(1), proposalTotal(reader)-proposalsBefore, "SubmitProposal must record exactly one proposal")
	require.Equal(t, votesBefore, voteTotal(reader))
	require.Equal(t, depositsBefore, depositTotal(reader))
	proposal, found := app.GovKeeper.GetProposal(ctx, proposalResponse.ProposalId)
	require.True(t, found)
	require.Equal(t, types.StatusVotingPeriod, proposal.Status)

	voteResponse, err := server.Vote(goCtx, types.NewMsgVote(addrs[1], proposal.ProposalId, types.OptionYes))
	require.NoError(t, err)
	require.NotNil(t, voteResponse)
	require.Equal(t, int64(1), voteTotal(reader)-votesBefore, "Vote must record exactly one vote")
	require.Equal(t, proposalsBefore+1, proposalTotal(reader))
	require.Equal(t, depositsBefore, depositTotal(reader))

	votesAfterVote := voteTotal(reader)
	options := types.WeightedVoteOptions{
		{Option: types.OptionYes, Weight: sdk.MustNewDecFromStr("0.6")},
		{Option: types.OptionAbstain, Weight: sdk.MustNewDecFromStr("0.4")},
	}
	weightedResponse, err := server.VoteWeighted(goCtx, types.NewMsgVoteWeighted(addrs[2], proposal.ProposalId, options))
	require.NoError(t, err)
	require.NotNil(t, weightedResponse)
	require.Equal(t, int64(1), voteTotal(reader)-votesAfterVote, "VoteWeighted must record exactly one vote")
	require.Equal(t, int64(2), voteTotal(reader)-votesBefore)
	require.Equal(t, proposalsBefore+1, proposalTotal(reader))
	require.Equal(t, depositsBefore, depositTotal(reader))

	amount := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1))
	depositResponse, err := server.Deposit(goCtx, types.NewMsgDeposit(addrs[1], proposal.ProposalId, amount))
	require.NoError(t, err)
	require.NotNil(t, depositResponse)
	require.Equal(t, int64(1), depositTotal(reader)-depositsBefore, "Deposit must record exactly one deposit")
	require.Equal(t, int64(1), proposalTotal(reader)-proposalsBefore)
	require.Equal(t, int64(2), voteTotal(reader)-votesBefore)
	deposit, found := app.GovKeeper.GetDeposit(ctx, proposal.ProposalId, addrs[1])
	require.True(t, found)
	require.Equal(t, amount, deposit.Amount)
}

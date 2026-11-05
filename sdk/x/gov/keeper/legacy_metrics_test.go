package keeper_test

import (
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func TestGovEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	app := paxapp.Setup(t, false, false, false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: 1, Time: time.Now()})
	addrs := paxapp.AddTestAddrsIncremental(app, ctx, 3, sdk.NewInt(30000000))
	server := keeper.NewMsgServerImpl(app.GovKeeper)
	goCtx := sdk.WrapSDKContext(ctx)
	initialDeposit := app.GovKeeper.GetDepositParams(ctx).MinDeposit
	msg, err := types.NewMsgSubmitProposal(TestProposal, initialDeposit, addrs[0])
	require.NoError(t, err)

	sink := oteltest.InstallLegacySink(t)
	require.Empty(t, sink.Keys())
	assertNoLegacyGovMetrics := func() {
		t.Helper()
		keys := sink.Keys()
		for _, key := range keys {
			require.False(t, strings.HasPrefix(key, "oteltest.gov."), "legacy governance metric emitted: %s", key)
		}
		require.NotContains(t, keys, "oteltest.end_blocker", "EndBlocker must not emit a legacy duration")
	}

	proposalsBefore := proposalTotal(reader)
	votesBefore := voteTotal(reader)
	depositsBefore := depositTotal(reader)
	proposalResponse, err := server.SubmitProposal(goCtx, msg)
	require.NoError(t, err)
	require.NotNil(t, proposalResponse)
	require.Equal(t, proposalsBefore+1, proposalTotal(reader))
	proposal, found := app.GovKeeper.GetProposal(ctx, proposalResponse.ProposalId)
	require.True(t, found)
	require.Equal(t, types.StatusVotingPeriod, proposal.Status)
	assertNoLegacyGovMetrics()

	voteResponse, err := server.Vote(goCtx, types.NewMsgVote(addrs[1], proposal.ProposalId, types.OptionYes))
	require.NoError(t, err)
	require.NotNil(t, voteResponse)
	require.Equal(t, votesBefore+1, voteTotal(reader))
	assertNoLegacyGovMetrics()

	options := types.WeightedVoteOptions{
		{Option: types.OptionYes, Weight: sdk.MustNewDecFromStr("0.6")},
		{Option: types.OptionAbstain, Weight: sdk.MustNewDecFromStr("0.4")},
	}
	weightedResponse, err := server.VoteWeighted(goCtx, types.NewMsgVoteWeighted(addrs[2], proposal.ProposalId, options))
	require.NoError(t, err)
	require.NotNil(t, weightedResponse)
	require.Equal(t, votesBefore+2, voteTotal(reader))
	assertNoLegacyGovMetrics()

	amount := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1))
	depositResponse, err := server.Deposit(goCtx, types.NewMsgDeposit(addrs[1], proposal.ProposalId, amount))
	require.NoError(t, err)
	require.NotNil(t, depositResponse)
	require.Equal(t, depositsBefore+1, depositTotal(reader))
	deposit, found := app.GovKeeper.GetDeposit(ctx, proposal.ProposalId, addrs[1])
	require.True(t, found)
	require.Equal(t, amount, deposit.Amount)
	assertNoLegacyGovMetrics()

	durationsBefore := reader.HistogramCount("paxcosmos_x_gov", "gov_end_blocker_duration")
	gov.EndBlocker(ctx, app.GovKeeper)
	require.Equal(t, durationsBefore+1, reader.HistogramCount("paxcosmos_x_gov", "gov_end_blocker_duration"))
	assertNoLegacyGovMetrics()

	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(ctx.BlockTime().Add(time.Second))
	gov.EndBlocker(ctx, app.GovKeeper)
	require.Equal(t, durationsBefore+2, reader.HistogramCount("paxcosmos_x_gov", "gov_end_blocker_duration"))
	require.Equal(t, proposalsBefore+1, proposalTotal(reader))
	require.Equal(t, votesBefore+2, voteTotal(reader))
	require.Equal(t, depositsBefore+1, depositTotal(reader))
	assertNoLegacyGovMetrics()
}

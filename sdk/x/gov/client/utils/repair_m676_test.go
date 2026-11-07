package utils_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func TestRepairM_676_HistoricalMultiMessage(t *testing.T) {
	f := newGovHistoryFixture(t, 5)
	firstID, err := f.app.GovKeeper.GetProposalID(f.govCtx)
	require.NoError(t, err)
	target := firstID + 1
	// Both submission events match this transaction. Only the second message
	// produced the requested proposal, its proposer and its initial deposit.
	f.execute(false, f.submission(0, "unrelated", 100), f.submission(1, "target", 120))
	f.execute(false,
		types.NewMsgDeposit(f.addrs[2], firstID, historyCoins(500)),
		types.NewMsgDeposit(f.addrs[2], target, historyCoins(7)),
		types.NewMsgDeposit(f.addrs[3], target, historyCoins(9)),
		types.NewMsgDeposit(f.addrs[2], target, historyCoins(11)),
		types.NewMsgDeposit(f.addrs[1], target, historyCoins(3)),
	)
	f.execute(true,
		types.NewMsgVote(f.addrs[2], firstID, types.OptionNoWithVeto),
		types.NewMsgVote(f.addrs[3], target, types.OptionYes),
		types.NewMsgVote(f.addrs[2], target, types.OptionNo),
	)
	// This transaction occurs in two protobuf vote event streams. The last
	// matching message must win exactly once, independently of stream order.
	f.execute(false,
		types.NewMsgVote(f.addrs[2], target, types.OptionYes),
		types.NewMsgVoteWeighted(f.addrs[2], firstID, types.NewNonSplitVoteOption(types.OptionNoWithVeto)),
		types.NewMsgVoteWeighted(f.addrs[3], target, types.NewNonSplitVoteOption(types.OptionNo)),
		types.NewMsgVoteWeighted(f.addrs[2], target, types.WeightedVoteOptions{
			{Option: types.OptionYes, Weight: sdk.MustNewDecFromStr("0.4")},
			{Option: types.OptionAbstain, Weight: sdk.MustNewDecFromStr("0.6")},
		}),
	)
	f.index()
	proposer, err := utils.QueryProposerByTxQuery(f.ctx, f.clientCtx, target)
	require.NoError(t, err)
	require.Equal(t, utils.NewProposer(target, f.addrs[1].String()), proposer)
	proposer, err = utils.QueryProposerByTxQuery(f.ctx, f.clientCtx, firstID)
	require.NoError(t, err)
	require.Equal(t, utils.NewProposer(firstID, f.addrs[0].String()), proposer)

	data, err := utils.QueryDepositsByTxQuery(f.ctx, f.clientCtx, types.NewQueryProposalParams(target))
	require.NoError(t, err)
	var deposits []types.Deposit
	require.NoError(t, f.clientCtx.LegacyAmino.UnmarshalAsJSON(data, &deposits))
	wantDeposits := []types.Deposit{
		types.NewDeposit(target, f.addrs[1], historyCoins(123)),
		types.NewDeposit(target, f.addrs[2], historyCoins(18)),
		types.NewDeposit(target, f.addrs[3], historyCoins(9)),
	}
	require.Equal(t, wantDeposits, deposits)
	for i, owner := range []int{1, 2, 3} {
		live, found := f.app.GovKeeper.GetDeposit(f.govCtx, target, f.addrs[owner])
		require.True(t, found)
		require.Equal(t, wantDeposits[i], live)
		data, err := utils.QueryDepositByTxQuery(f.ctx, f.clientCtx, types.NewQueryDepositParams(target, f.addrs[owner]))
		require.NoError(t, err)
		var deposit types.Deposit
		require.NoError(t, f.clientCtx.Codec.UnmarshalAsJSON(data, &deposit))
		require.Equal(t, live, deposit)
	}
	wantVotes := []types.Vote{f.liveVote(target, 3), f.liveVote(target, 2)}
	require.Equal(t, wantVotes, f.votes(target, 1, 10))
	for _, owner := range []int{2, 3} {
		data, err := utils.QueryVoteByTxQuery(f.ctx, f.clientCtx, types.NewQueryVoteParams(target, f.addrs[owner]))
		require.NoError(t, err)
		var vote types.Vote
		require.NoError(t, f.clientCtx.Codec.UnmarshalAsJSON(data, &vote))
		require.Equal(t, f.liveVote(target, owner), vote)
	}
	data, err = utils.QueryVoteByTxQuery(f.ctx, f.clientCtx, types.NewQueryVoteParams(target, f.addrs[4]))
	require.ErrorContains(t, err, "did not vote")
	require.Nil(t, data)
	_, err = utils.QueryProposerByTxQuery(f.ctx, f.clientCtx, target+100)
	require.Error(t, err)
}

func TestRepairM_676_HistoricalPagination(t *testing.T) {
	exerciseHistoricalVotePagination(t)
}

func TestRepairM_676_GovHistoryTestShape(t *testing.T) {
	f := newGovHistoryFixture(t, 5)
	target := f.submit(0, "target")
	other := f.submit(1, "other")
	// Neither account 2's deposit nor account 2's vote is on the target,
	// but transaction-wide event predicates match sender 2 and the target.
	f.execute(false,
		types.NewMsgDeposit(f.addrs[2], other, historyCoins(7)),
		types.NewMsgDeposit(f.addrs[3], target, historyCoins(5)),
		types.NewMsgVote(f.addrs[2], other, types.OptionYes),
		types.NewMsgVote(f.addrs[3], target, types.OptionNo),
	)
	for i := 0; i < 35; i++ {
		f.execute(true, types.NewMsgDeposit(f.addrs[3], target, historyCoins(1)))
		f.execute(false, types.NewMsgDeposit(f.addrs[3], target, historyCoins(2)))
	}
	f.index()
	node, err := f.clientCtx.GetNode()
	require.NoError(t, err)
	for _, action := range []string{types.TypeMsgDeposit, sdk.MsgTypeURL(&types.MsgDeposit{})} {
		query := fmt.Sprintf("message.action='%s' AND proposal_deposit.proposal_id='%d'", action, target)
		page, limit := 1, 30
		first, err := node.TxSearch(f.ctx, query, true, &page, &limit, "asc")
		require.NoError(t, err)
		wantCount := 35
		if action == sdk.MsgTypeURL(&types.MsgDeposit{}) {
			wantCount++
		}
		require.Equal(t, wantCount, first.TotalCount)
		require.Len(t, first.Txs, 30)
		page = 2
		second, err := node.TxSearch(f.ctx, query, true, &page, &limit, "asc")
		require.NoError(t, err)
		require.Equal(t, wantCount, second.TotalCount)
		require.Len(t, second.Txs, wantCount-30)
		require.Less(t, first.Txs[29].Index, second.Txs[0].Index)
	}
	page, limit := 1, 30
	crossMessageQuery := fmt.Sprintf("message.action='%s' AND proposal_vote.proposal_id='%d' AND message.sender='%s'",
		sdk.MsgTypeURL(&types.MsgVote{}), target, f.addrs[2])
	matched, err := node.TxSearch(f.ctx, crossMessageQuery, true, &page, &limit, "asc")
	require.NoError(t, err)
	require.Equal(t, 1, matched.TotalCount)
	data, err := utils.QueryVoteByTxQuery(f.ctx, f.clientCtx, types.NewQueryVoteParams(target, f.addrs[2]))
	require.ErrorContains(t, err, "did not vote")
	require.Nil(t, data)
	data, err = utils.QueryDepositByTxQuery(f.ctx, f.clientCtx, types.NewQueryDepositParams(target, f.addrs[2]))
	require.ErrorContains(t, err, "did not deposit")
	require.Nil(t, data)
	data, err = utils.QueryDepositsByTxQuery(f.ctx, f.clientCtx, types.NewQueryProposalParams(target))
	require.NoError(t, err)
	var deposits []types.Deposit
	require.NoError(t, f.clientCtx.LegacyAmino.UnmarshalAsJSON(data, &deposits))
	require.Equal(t, []types.Deposit{
		types.NewDeposit(target, f.addrs[0], historyCoins(100)),
		types.NewDeposit(target, f.addrs[3], historyCoins(110)),
	}, deposits)
	live, found := f.app.GovKeeper.GetDeposit(f.govCtx, target, f.addrs[3])
	require.True(t, found)
	require.Equal(t, deposits[1], live)
	require.Equal(t, []types.Vote{f.liveVote(target, 3)}, f.votes(target, 1, 30))
}

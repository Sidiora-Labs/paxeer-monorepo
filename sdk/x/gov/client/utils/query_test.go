package utils_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/export"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

type govHistoryFixture struct {
	t         *testing.T
	ctx       context.Context
	app       *paxapp.App
	legacy    *baseapp.BaseApp
	govCtx    sdk.Context
	clientCtx client.Context
	addrs     []sdk.AccAddress
	txs       []tmtypes.Tx
	results   []*abci.TxResultV2
	height    int64
	index     func()
}

func newGovHistoryFixture(t *testing.T, accounts int) *govHistoryFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	conf.Mode = config.ModeValidator
	conf.RPC.ListenAddress = ""
	conf.Consensus.CreateEmptyBlocks = false
	genesis, err := tmtypes.GenesisDocFromFile(conf.GenesisFile())
	require.NoError(t, err)
	consensusApp := kvstore.NewApplication()
	consensusApp.SetValidators(genesis.ValidatorUpdates())
	t.Cleanup(func() { require.NoError(t, consensusApp.Close()) })
	n, err := tmnode.New(ctx, conf, func() {}, consensusApp, nil, nil, tmnode.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	require.NoError(t, n.Start(ctx))
	t.Cleanup(func() { cancel(); n.Wait() })

	env := *n.RPCEnvironment()
	env.Router = nil
	blockDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, blockDB.Close()) })
	blocks := export.NewBlockStore(blockDB)
	env.BlockStore = blocks
	rpcClient := &local.Local{Environment: &env}
	require.Len(t, env.EventSinks, 1)
	require.Equal(t, "kv", string(env.EventSinks[0].Type()))
	app := paxapp.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	enc := paxapp.MakeEncodingConfig()
	legacyDB := dbm.NewMemDB()
	legacy := baseapp.NewBaseApp("historical-legacy-gov", legacyDB, enc.TxConfig.TxDecoder(), conf, paxapp.TestAppOpts{})
	legacy.Router().AddRoute(sdk.NewRoute(types.RouterKey, gov.NewHandler(app.GovKeeper)))
	t.Cleanup(func() { require.NoError(t, legacy.Close()) })
	now := time.Now().UTC()
	govCtx := app.BaseApp.NewContext(false, tmproto.Header{Height: 1, Time: now})
	addrs := paxapp.AddTestAddrsIncremental(app, govCtx, accounts, sdk.NewInt(30000000))
	params := app.GovKeeper.GetDepositParams(govCtx)
	params.MinDeposit = historyCoins(100)
	app.GovKeeper.SetDepositParams(govCtx, params)
	f := &govHistoryFixture{
		t: t, ctx: ctx, app: app, legacy: legacy, govCtx: govCtx, addrs: addrs, height: 1,
		clientCtx: client.Context{}.WithClient(rpcClient).WithCodec(enc.Marshaler).WithLegacyAmino(enc.Amino).WithTxConfig(enc.TxConfig),
	}
	f.index = func() {
		t.Helper()
		vals, err := env.StateStore.LoadValidators(1)
		require.NoError(t, err)
		block := tmtypes.MakeBlock(f.height, f.txs, &tmtypes.Commit{}, nil)
		block.Time = now.Add(time.Duration(f.height) * time.Second)
		block.ChainID = env.GenDoc.ChainID
		block.ValidatorsHash = vals.Hash()
		block.NextValidatorsHash = vals.Hash()
		block.ProposerAddress = vals.Proposer.Address
		parts, err := block.MakePartSet(65536)
		require.NoError(t, err)
		blocks.SaveBlock(block, parts, &tmtypes.Commit{Height: f.height})
		require.NoError(t, env.EventSinks[0].IndexTxEvents(f.results))
		f.height++
		f.txs = nil
		f.results = nil
		f.govCtx = f.govCtx.WithBlockHeight(f.height)
	}
	return f
}

func historyCoins(amount int64) sdk.Coins {
	return sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, amount))
}

func (f *govHistoryFixture) execute(legacy bool, msgs ...sdk.Msg) {
	f.t.Helper()
	builder := f.clientCtx.TxConfig.NewTxBuilder()
	require.NoError(f.t, builder.SetMsgs(msgs...))
	builder.SetMemo(fmt.Sprintf("history-%d-%d", f.height, len(f.txs)))
	encoded, err := f.clientCtx.TxConfig.TxEncoder()(builder.GetTx())
	require.NoError(f.t, err)
	decoded, err := f.clientCtx.TxConfig.TxDecoder()(encoded)
	require.NoError(f.t, err)
	runner := f.app.BaseApp
	if legacy {
		runner = f.legacy
	}
	result, err := runner.RunMsgs(f.govCtx, decoded.GetMsgs())
	require.NoError(f.t, err)
	f.results = append(f.results, &abci.TxResultV2{
		Height: f.height, Index: uint32(len(f.txs)), Tx: encoded,
		Result: abci.ExecTxResult{Data: result.Data, Log: result.Log, Events: sdk.MarkEventsToIndex(result.Events, f.app.IndexEvents)},
	})
	f.txs = append(f.txs, encoded)
}

func (f *govHistoryFixture) submission(owner int, title string, amount int64) *types.MsgSubmitProposal {
	f.t.Helper()
	msg, err := types.NewMsgSubmitProposal(types.NewTextProposal(title, "governance history proof", false), historyCoins(amount), f.addrs[owner])
	require.NoError(f.t, err)
	return msg
}

func (f *govHistoryFixture) submit(owner int, title string) uint64 {
	f.t.Helper()
	id, err := f.app.GovKeeper.GetProposalID(f.govCtx)
	require.NoError(f.t, err)
	f.execute(false, f.submission(owner, title, 100))
	return id
}

func (f *govHistoryFixture) votes(proposalID uint64, page, limit int) []types.Vote {
	f.t.Helper()
	data, err := utils.QueryVotesByTxQuery(f.ctx, f.clientCtx, types.NewQueryProposalVotesParams(proposalID, page, limit))
	require.NoError(f.t, err)
	var votes []types.Vote
	require.NoError(f.t, f.clientCtx.LegacyAmino.UnmarshalAsJSON(data, &votes))
	return votes
}

func (f *govHistoryFixture) liveVote(proposalID uint64, owner int) types.Vote {
	f.t.Helper()
	vote, found := f.app.GovKeeper.GetVote(f.govCtx, proposalID, f.addrs[owner])
	require.True(f.t, found)
	return types.NewVote(proposalID, f.addrs[owner], vote.Options)
}

func TestGetPaginatedVotes(t *testing.T) {
	exerciseHistoricalVotePagination(t)
}

func exerciseHistoricalVotePagination(t *testing.T) {
	t.Helper()
	f := newGovHistoryFixture(t, 140)
	target := f.submit(0, "target")
	other := f.submit(0, "other")
	var owners []int
	owner := 1
	// Unequal stream lengths require a second page for every action, even when
	// another stream is exhausted. Interleave the streams within each block.
	for i := 0; i < 35; i++ {
		for stream, count := range []int{35, 34, 33, 32} {
			if i >= count {
				continue
			}
			var vote sdk.Msg = types.NewMsgVote(f.addrs[owner], target, types.OptionYes)
			if stream >= 2 {
				vote = types.NewMsgVoteWeighted(f.addrs[owner], target, types.NewNonSplitVoteOption(types.OptionAbstain))
			}
			f.execute(stream%2 == 0, vote)
			owners = append(owners, owner)
			owner++
		}
		if i == 17 {
			f.index()
		}
	}
	// The latest vote is on the second index page and in a later block.
	f.execute(false,
		types.NewMsgVote(f.addrs[1], other, types.OptionNoWithVeto),
		types.NewMsgVote(f.addrs[1], target, types.OptionNo),
		types.NewMsgVoteWeighted(f.addrs[1], target, types.NewNonSplitVoteOption(types.OptionAbstain)),
	)
	f.index()
	// Account 1's final vote moves to the end of chronological final-vote order.
	owners = append(owners[1:], 1)
	want := make([]types.Vote, 0, len(owners))
	for _, owner := range owners {
		want = append(want, f.liveVote(target, owner))
	}
	require.Len(t, want, 134)
	for _, tc := range []struct {
		name        string
		page, limit int
		want        []types.Vote
	}{
		{"first", 1, 30, want[:30]},
		{"second", 2, 30, want[30:60]},
		{"third", 3, 30, want[60:90]},
		{"fourth", 4, 30, want[90:120]},
		{"partial-last", 5, 30, want[120:]},
		{"exhausted", 6, 30, nil},
		{"invalid", -1, 30, nil},
		{"default-limit", 1, 0, want[:100]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := f.votes(target, tc.page, tc.limit)
			require.Len(t, got, len(tc.want))
			for i := range got {
				require.Equal(t, tc.want[i], got[i])
			}
		})
	}
	data, err := utils.QueryVoteByTxQuery(f.ctx, f.clientCtx, types.NewQueryVoteParams(target, f.addrs[1]))
	require.NoError(t, err)
	var single types.Vote
	require.NoError(t, f.clientCtx.Codec.UnmarshalAsJSON(data, &single))
	require.Equal(t, want[len(want)-1], single)
}

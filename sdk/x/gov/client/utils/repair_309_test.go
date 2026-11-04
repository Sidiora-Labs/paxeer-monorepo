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
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func TestRepair_309_HistoricalDepositOwner(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	conf.Mode = config.ModeValidator
	conf.RPC.ListenAddress = ""
	conf.Consensus.CreateEmptyBlocks = false
	genesis, err := tmtypes.GenesisDocFromFile(conf.GenesisFile())
	require.NoError(t, err)
	consensusApp := kvstore.NewApplication()
	consensusApp.SetValidators(genesis.ValidatorUpdates())
	defer func() { require.NoError(t, consensusApp.Close()) }()
	n, err := tmnode.New(ctx, conf, func() {}, consensusApp, nil, nil, tmnode.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	require.NoError(t, n.Start(ctx))
	defer func() { cancel(); n.Wait() }()

	// Use the node's real event sink and RPC consumer, with a separate real
	// block store holding the governance transactions executed below.
	env := *n.RPCEnvironment()
	env.Router = nil
	blockDB := dbm.NewMemDB()
	defer func() { require.NoError(t, blockDB.Close()) }()
	blocks := export.NewBlockStore(blockDB)
	env.BlockStore = blocks
	rpcClient := &local.Local{Environment: &env}
	require.Len(t, env.EventSinks, 1)
	require.Equal(t, "kv", string(env.EventSinks[0].Type()))

	app := paxapp.Setup(t, false, false, false)
	defer func() { require.NoError(t, app.Close()) }()
	now := time.Now().UTC()
	govCtx := app.BaseApp.NewContext(false, tmproto.Header{Height: 1, Time: now})
	addrs := paxapp.AddTestAddrsIncremental(app, govCtx, 3, sdk.NewInt(30000000))
	coins := func(amount int64) sdk.Coins {
		return sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, amount))
	}
	depositParams := app.GovKeeper.GetDepositParams(govCtx)
	depositParams.MinDeposit = coins(100)
	app.GovKeeper.SetDepositParams(govCtx, depositParams)
	enc := paxapp.MakeEncodingConfig()
	clientCtx := client.Context{}.WithClient(rpcClient).WithCodec(enc.Marshaler).
		WithLegacyAmino(enc.Amino).WithTxConfig(enc.TxConfig)

	var txs []tmtypes.Tx
	var results []*abci.TxResultV2
	execute := func(msgs ...sdk.Msg) {
		t.Helper()
		builder := enc.TxConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msgs...))
		builder.SetMemo(fmt.Sprintf("repair-309-%d", len(txs)))
		encoded, err := enc.TxConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		decoded, err := enc.TxConfig.TxDecoder()(encoded)
		require.NoError(t, err)
		result, err := app.BaseApp.RunMsgs(govCtx, decoded.GetMsgs())
		require.NoError(t, err)
		results = append(results, &abci.TxResultV2{
			Height: 1, Index: uint32(len(txs)), Tx: encoded,
			Result: abci.ExecTxResult{Data: result.Data, Events: sdk.MarkEventsToIndex(result.Events, app.IndexEvents)},
		})
		txs = append(txs, encoded)
	}
	submit := func(title string, initial int64) uint64 {
		t.Helper()
		id, err := app.GovKeeper.GetProposalID(govCtx)
		require.NoError(t, err)
		msg, err := types.NewMsgSubmitProposal(types.NewTextProposal(title, "deposit owner proof", false), coins(initial), addrs[0])
		require.NoError(t, err)
		execute(msg)
		return id
	}
	proposalID := submit("target", 100)
	otherID := submit("other", 100)
	execute(types.NewMsgDeposit(addrs[0], proposalID, coins(7)))
	for i := 0; i < 31; i++ {
		execute(types.NewMsgDeposit(addrs[1], proposalID, coins(1)))
	}
	// Transaction predicates match across messages: the consumer must check
	// each message's owner and proposal, and add both matching messages.
	execute(
		types.NewMsgDeposit(addrs[1], otherID, coins(9)),
		types.NewMsgDeposit(addrs[0], proposalID, coins(3)),
		types.NewMsgDeposit(addrs[1], proposalID, coins(5)),
		types.NewMsgDeposit(addrs[1], proposalID, coins(2)),
	)
	vals, err := env.StateStore.LoadValidators(1)
	require.NoError(t, err)
	block := tmtypes.MakeBlock(1, txs, &tmtypes.Commit{}, nil)
	block.Time = now
	block.ChainID = env.GenDoc.ChainID
	block.ValidatorsHash = vals.Hash()
	block.NextValidatorsHash = vals.Hash()
	block.ProposerAddress = vals.Proposer.Address
	parts, err := block.MakePartSet(65536)
	require.NoError(t, err)
	blocks.SaveBlock(block, parts, &tmtypes.Commit{Height: 1})
	require.NoError(t, env.EventSinks[0].IndexTxEvents(results))

	want := []types.Deposit{
		types.NewDeposit(proposalID, addrs[0], coins(110)),
		types.NewDeposit(proposalID, addrs[1], coins(38)),
	}
	for i, expected := range want {
		live, found := app.GovKeeper.GetDeposit(govCtx, proposalID, addrs[i])
		require.True(t, found)
		require.Equal(t, expected, live)
	}
	checkHistory := func() {
		t.Helper()
		for i, expected := range want {
			data, err := utils.QueryDepositByTxQuery(ctx, clientCtx, types.NewQueryDepositParams(proposalID, addrs[i]))
			require.NoError(t, err)
			var recovered types.Deposit
			require.NoError(t, enc.Marshaler.UnmarshalAsJSON(data, &recovered))
			require.Equal(t, expected, recovered)
		}
		data, err := utils.QueryDepositByTxQuery(ctx, clientCtx, types.NewQueryDepositParams(proposalID, addrs[2]))
		require.ErrorContains(t, err, fmt.Sprintf("address '%s' did not deposit to proposalID %d", addrs[2], proposalID))
		require.Nil(t, data)
	}
	checkHistory()
	proposal, found := app.GovKeeper.GetProposal(govCtx, proposalID)
	require.True(t, found)
	require.Equal(t, types.StatusVotingPeriod, proposal.Status)
	govCtx = govCtx.WithBlockTime(proposal.VotingEndTime.Add(time.Second))
	gov.EndBlocker(govCtx, app.GovKeeper)
	proposal, found = app.GovKeeper.GetProposal(govCtx, proposalID)
	require.True(t, found)
	require.Equal(t, types.StatusRejected, proposal.Status)
	for _, addr := range addrs[:2] {
		_, found := app.GovKeeper.GetDeposit(govCtx, proposalID, addr)
		require.False(t, found)
	}
	checkHistory()
}

package keeper_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transfer "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paxdbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func (suite *KeeperTestSuite) TestGenesis() {
	transferStateGenesisRecovery(suite.T())
}

func transferStateReopen(t *testing.T, app *simapp.SimApp) *simapp.SimApp {
	t.Helper()
	info, err := app.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(info))
	version := app.LastBlockHeight()
	require.Positive(t, version)
	require.NoError(t, app.Close())
	sc := paxdbconfig.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	sc.MemIAVLConfig.SnapshotMinTimeInterval = 0
	sc.HistoricalProofRateLimit = 0
	sc.HistoricalProofMaxInFlight = 100
	cms := storev2rootmulti.NewStore(home, sc, paxdbconfig.StateStoreConfig{}, nil)
	reopened := simapp.NewSimApp(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 5, nil, simapp.MakeTestEncodingConfig(), simapp.EmptyAppOptions{}, func(b *baseapp.BaseApp) { b.SetCMS(cms) })
	require.Equal(t, version, reopened.LastBlockHeight())
	return reopened
}

func transferStateGenesisRecovery(t *testing.T) {
	t.Helper()
	s := transferStateSuite(t)
	traces := types.Traces{
		transferStateProduce(t, s, sdk.DefaultBondDenom),
		transferStateProduce(t, s, "factory/creator/subdenom"),
	}
	producer := s.chainC.GetSimApp()
	receiver := s.chainC.SenderAccount.GetAddress()
	genesis := transfer.NewAppModule(producer.TransferKeeper).ExportGenesis(s.chainC.GetContext(), producer.AppCodec())
	require.True(t, json.Valid(genesis))
	require.NoError(t, (transfer.AppModuleBasic{}).ValidateGenesis(producer.AppCodec(), producer.GetTxConfig(), genesis))
	var decoded types.GenesisState
	producer.AppCodec().MustUnmarshalJSON(genesis, &decoded)
	require.Equal(t, types.PortID, decoded.PortId)
	require.Equal(t, traces.Sort(), decoded.DenomTraces)
	balances := producer.BankKeeper.ExportGenesis(s.chainC.GetContext())
	bankJSON := producer.AppCodec().MustMarshalJSON(balances)
	// Close the actual channel producer and reopen its persisted commitment store.
	producer = transferStateReopen(t, producer)
	s.chainC.App = ibctesting.NewTestingAppDecorator(t, producer)
	pctx := producer.NewUncachedContext(true, tmproto.Header{Height: producer.LastBlockHeight()})
	require.Equal(t, genesis, transfer.NewAppModule(producer.TransferKeeper).ExportGenesis(pctx, producer.AppCodec()))
	require.Equal(t, balances, producer.BankKeeper.ExportGenesis(pctx))

	consumer := simapp.Setup(false)
	cctx := consumer.NewContext(false, tmproto.Header{Height: 1})
	require.Empty(t, consumer.TransferKeeper.GetAllDenomTraces(cctx))
	require.True(t, consumer.BankKeeper.GetAllBalances(cctx, receiver).Empty())
	var imported banktypes.GenesisState
	consumer.AppCodec().MustUnmarshalJSON(bankJSON, &imported)
	require.NoError(t, imported.Validate())
	consumer.BankKeeper.InitGenesis(cctx, &imported)
	transfer.NewAppModule(consumer.TransferKeeper).InitGenesis(cctx, consumer.AppCodec(), genesis)
	consumer.SetDeliverStateToCommit()
	_, err := consumer.Commit(context.Background())
	require.NoError(t, err)
	consumer = transferStateReopen(t, consumer)
	t.Cleanup(func() { require.NoError(t, consumer.Close()) })
	cctx = consumer.NewUncachedContext(true, tmproto.Header{Height: consumer.LastBlockHeight()})
	require.Equal(t, balances, consumer.BankKeeper.ExportGenesis(cctx))
	require.Equal(t, genesis, transfer.NewAppModule(consumer.TransferKeeper).ExportGenesis(cctx, consumer.AppCodec()))
	for _, trace := range traces {
		result, err := consumer.TransferKeeper.DenomTrace(sdk.WrapSDKContext(cctx), &types.QueryDenomTraceRequest{Hash: trace.Hash().String()})
		require.NoError(t, err)
		require.Equal(t, &trace, result.DenomTrace)
		require.Equal(t, sdk.NewInt(17), consumer.BankKeeper.GetBalance(cctx, receiver, trace.IBCDenom()).Amount)
	}
}

package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/code"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func newAutobahnIndexingEnvironment(t *testing.T, rng utils.Rng) (*Environment, *p2p.GigaRouter, eventbus.Subscription, eventbus.Subscription) {
	t.Helper()
	ctx := t.Context()
	app := proxy.New(kvstore.NewApplication(), proxy.NopMetrics())
	stateStore := sm.NewStore(dbm.NewMemDB())
	router, giga := newAutobahnTestRouter(t, rng, "autobahn-indexing-test", app, stateStore, 1, 1_000)
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(ctx))
	t.Cleanup(bus.Wait)
	giga.SetEventBus(bus)

	sinks := []indexer.EventSink{kv.NewEventSink(dbm.NewMemDB())}
	service := indexer.NewService(indexer.ServiceArgs{Sinks: sinks, EventBus: bus})
	require.NoError(t, service.Start(ctx))
	t.Cleanup(service.Wait)

	headers, err := bus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: t.Name(), Query: types.EventQueryNewBlockHeader, Limit: 16,
	})
	require.NoError(t, err)
	txs, err := bus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: t.Name(), Query: types.EventQueryTx, Limit: 16,
	})
	require.NoError(t, err)

	return &Environment{
		App: app, StateStore: stateStore, BlockStore: store.NewBlockStore(dbm.NewMemDB()),
		Router: router, EventBus: bus, EventSinks: sinks,
	}, giga, headers, txs
}

func nextAutobahnIndexedHeader(t *testing.T, ctx context.Context, headers, txs eventbus.Subscription) types.EventDataNewBlockHeader {
	t.Helper()
	msg, err := headers.Next(ctx)
	require.NoError(t, err, "waiting for an executed Autobahn block")
	header, ok := msg.Data().(types.EventDataNewBlockHeader)
	require.True(t, ok)
	for i := range header.NumTxs {
		msg, err := txs.Next(ctx)
		require.NoError(t, err, "waiting for the indexer to process the block's transactions")
		tx, ok := msg.Data().(types.EventDataTx)
		require.True(t, ok)
		require.Equal(t, header.Header.Height, tx.Height)
		require.EqualValues(t, i, tx.Index)
	}
	return header
}

func TestAutobahnTxSearchFindsFinalizedTx(t *testing.T) {
	rng := utils.TestRng()
	env, giga, headers, txs := newAutobahnIndexingEnvironment(t, rng)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	tx := types.Tx("indexed-key=indexed-value")

	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(env.Router.Run(ctx)) })
		for _, tx := range []types.Tx{tx, types.Tx("seal-key=seal-value")} {
			if _, err := giga.InsertTx(ctx, tx); err != nil {
				return fmt.Errorf("giga.InsertTx(): %w", err)
			}
		}

		header := nextAutobahnIndexedHeader(t, ctx, headers, txs)
		for header.NumTxs == 0 {
			header = nextAutobahnIndexedHeader(t, ctx, headers, txs)
		}
		require.Equal(t, int64(1), header.NumTxs)
		res, err := env.TxSearch(ctx, &coretypes.RequestTxSearch{
			Query: fmt.Sprintf("tx.height=%d", header.Header.Height),
		})
		require.NoError(t, err)
		require.Equal(t, 1, res.TotalCount)
		require.Len(t, res.Txs, 1)
		require.Equal(t, tx, res.Txs[0].Tx)
		require.Equal(t, tx.Hash().Bytes(), res.Txs[0].Hash)
		require.Equal(t, header.Header.Height, res.Txs[0].Height)
		require.Equal(t, uint32(0), res.Txs[0].Index)
		require.Equal(t, code.CodeTypeOK, res.Txs[0].TxResult.Code)
		return nil
	})
	require.NoError(t, err)
}

func TestAutobahnBlockSearchCountsFinalizedBlock(t *testing.T) {
	rng := utils.TestRng()
	env, giga, headers, txs := newAutobahnIndexingEnvironment(t, rng)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(env.Router.Run(ctx)) })
		for i := range 3 {
			if _, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value%d", i, i))); err != nil {
				return fmt.Errorf("giga.InsertTx(): %w", err)
			}
		}

		for height := int64(1); height <= 2; height++ {
			header := nextAutobahnIndexedHeader(t, ctx, headers, txs)
			require.Equal(t, height, header.Header.Height)
			_, err := env.StateStore.LoadFinalizeBlockResponses(height)
			require.NoError(t, err)
		}
		res, err := env.BlockSearch(ctx, &coretypes.RequestBlockSearch{Query: "block.height=2"})
		require.NoError(t, err)
		require.Equal(t, 1, res.TotalCount)
		return nil
	})
	require.NoError(t, err)
}

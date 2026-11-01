package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestBlockSearchUnderAutobahnReturnsBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	rng := utils.TestRng()
	appKV := &statusAutobahnObservedApplication{
		Application: kvstore.NewApplication(),
		committed:   utils.NewAtomicSend[int64](0),
	}
	appKV.RetainBlocks = 2
	t.Cleanup(func() { require.NoError(t, appKV.Close()) })
	app := proxy.New(appKV, proxy.NopMetrics())
	stateStore := &blockchainAutobahnObservedStore{
		Store:  sm.NewStore(dbm.NewMemDB()),
		pruned: utils.NewAtomicSend[int64](0),
	}
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	sink := kv.NewEventSink(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, sink.Stop()) })
	router, giga := newAutobahnTestRouter(t, rng, "autobahn-block-search-test", app, stateStore, 1, 1_000)
	env := &Environment{
		App: app, StateStore: stateStore, BlockStore: blockStore, Router: router,
		EventSinks: []indexer.EventSink{sink},
	}

	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for i := range 4 {
			resp, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value%d", i, i)))
			if err != nil {
				return fmt.Errorf("giga.InsertTx(): %w", err)
			}
			require.False(t, resp.IsErr())
		}
		_, err := appKV.committed.Subscribe().Wait(ctx, func(height int64) bool { return height >= 3 })
		if err != nil {
			return err
		}
		require.Zero(t, blockStore.Base())
		require.Zero(t, blockStore.Height())
		for height := int64(2); height <= 3; height++ {
			block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(height))
			require.NoError(t, err)
			response, err := stateStore.LoadFinalizeBlockResponses(height)
			require.NoError(t, err)
			require.NoError(t, sink.IndexBlockEvents(types.EventDataNewBlockHeader{
				Header: block.Block.Header, NumTxs: int64(len(block.Block.Txs)),
				ResultFinalizeBlock: *response,
			}))
		}

		for _, tc := range []struct {
			name    string
			order   string
			page    coretypes.Int64
			perPage coretypes.Int64
			heights []int64
		}{
			{name: "default", page: 1, perPage: 10, heights: []int64{3, 2}},
			{name: "descending", order: "desc", page: 1, perPage: 10, heights: []int64{3, 2}},
			{name: "ascending", order: "asc", page: 1, perPage: 10, heights: []int64{2, 3}},
			{name: "first_page", page: 1, perPage: 1, heights: []int64{3}},
			{name: "second_page", page: 2, perPage: 1, heights: []int64{2}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				result, err := env.BlockSearch(ctx, &coretypes.RequestBlockSearch{
					Query: "block.height>=2", OrderBy: tc.order, Page: &tc.page, PerPage: &tc.perPage,
				})
				require.NoError(t, err)
				require.Equal(t, 2, result.TotalCount)
				require.Len(t, result.Blocks, len(tc.heights))
				for i, height := range tc.heights {
					block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(height))
					require.NoError(t, err)
					require.NotEmpty(t, block.BlockID.Hash)
					require.Equal(t, block.BlockID.Hash, result.Blocks[i].BlockID.Hash)
					require.Equal(t, block, result.Blocks[i])
					require.Equal(t, height, result.Blocks[i].Block.Height)
				}
			})
		}
		result, err := env.BlockSearch(ctx, &coretypes.RequestBlockSearch{Query: "block.height>3"})
		require.NoError(t, err)
		require.Zero(t, result.TotalCount)
		require.Empty(t, result.Blocks)

		for i := 4; i < 6; i++ {
			resp, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value%d", i, i)))
			if err != nil {
				return fmt.Errorf("giga.InsertTx(): %w", err)
			}
			require.False(t, resp.IsErr())
		}
		_, err = stateStore.pruned.Subscribe().Wait(ctx, func(height int64) bool { return height >= 4 })
		if err != nil {
			return err
		}
		require.Equal(t, int64(3), giga.EarliestBlockNumber())
		_, err = giga.BlockByNumber(ctx, 2)
		require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable)
		for _, order := range []string{"asc", "desc"} {
			result, err := env.BlockSearch(ctx, &coretypes.RequestBlockSearch{Query: "block.height>=2", OrderBy: order})
			require.NoError(t, err)
			require.Equal(t, 2, result.TotalCount)
			require.Len(t, result.Blocks, 1)
			block, err := giga.BlockByNumber(ctx, 3)
			require.NoError(t, err)
			require.Equal(t, block, result.Blocks[0])
		}
		result, err = env.BlockSearch(ctx, &coretypes.RequestBlockSearch{Query: "block.height=2"})
		require.NoError(t, err)
		require.Equal(t, 1, result.TotalCount)
		require.Empty(t, result.Blocks)
		return nil
	})
	require.NoError(t, err)
}

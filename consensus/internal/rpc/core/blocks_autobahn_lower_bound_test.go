package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestAutobahnHeightBelowRetainedRangeRejected(t *testing.T) {
	const blocks = 5
	const earliest = int64(3)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	rng := utils.TestRng()
	kv := &statusAutobahnObservedApplication{
		Application: kvstore.NewApplication(),
		committed:   utils.NewAtomicSend[int64](0),
	}
	kv.RetainBlocks = 2
	t.Cleanup(func() { require.NoError(t, kv.Close()) })
	app := proxy.New(kv, proxy.NopMetrics())
	stateStore := &blockchainAutobahnObservedStore{
		Store:  sm.NewStore(dbm.NewMemDB()),
		pruned: utils.NewAtomicSend[int64](0),
	}
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	router, giga := newAutobahnTestRouter(t, rng, "autobahn-lower-bound-test", app, stateStore, 1, 1_000)
	env := &Environment{App: app, StateStore: stateStore, BlockStore: blockStore, Router: router}

	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for i := range blocks + 1 {
			resp, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value%d", i, i)))
			if err != nil {
				return fmt.Errorf("giga.InsertTx(): %w", err)
			}
			require.False(t, resp.IsErr())
		}
		_, err := stateStore.pruned.Subscribe().Wait(ctx, func(height int64) bool { return height >= earliest+1 })
		return err
	})
	require.NoError(t, err)
	require.Zero(t, blockStore.Base())
	require.Zero(t, blockStore.Height())
	require.Equal(t, earliest, giga.EarliestBlockNumber())
	info, err := env.ABCIInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(blocks), info.Response.LastBlockHeight)

	for requested := int64(1); requested < earliest; requested++ {
		height, err := env.autobahnCheckAndGetHeight(ctx, &requested)
		require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable)
		require.ErrorContains(t, err, fmt.Sprintf("requested height: %d, base height: %d", requested, earliest))
		require.Zero(t, height)

		result, err := env.Block(ctx, &coretypes.RequestBlockInfo{Height: (*coretypes.Int64)(&requested)})
		require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable)
		require.ErrorContains(t, err, fmt.Sprintf("base height: %d", earliest))
		require.Nil(t, result)
	}
	for requested := earliest; requested <= blocks; requested++ {
		height, err := env.autobahnCheckAndGetHeight(ctx, &requested)
		require.NoError(t, err)
		require.Equal(t, requested, height)
		result, err := env.Block(ctx, &coretypes.RequestBlockInfo{Height: (*coretypes.Int64)(&requested)})
		require.NoError(t, err)
		require.NotNil(t, result.Block)
		require.Equal(t, requested, result.Block.Height)
	}
	height, err := env.autobahnCheckAndGetHeight(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, int64(blocks), height)
	for _, requested := range []int64{-1, 0, blocks + 1} {
		height, err := env.autobahnCheckAndGetHeight(ctx, &requested)
		if requested <= 0 {
			require.ErrorIs(t, err, coretypes.ErrZeroOrNegativeHeight)
		} else {
			require.ErrorIs(t, err, coretypes.ErrHeightExceedsChainHead)
		}
		require.Zero(t, height)
	}
}

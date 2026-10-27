package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// statusAutobahnObservedApplication publishes notifications after real kvstore commits.
type statusAutobahnObservedApplication struct {
	*kvstore.Application
	committed utils.AtomicSend[int64]
}

func (app *statusAutobahnObservedApplication) Commit(ctx context.Context) (*abci.ResponseCommit, error) {
	resp, err := app.Application.Commit(ctx)
	if err != nil {
		return nil, err
	}
	info, err := app.Application.Info(ctx, &abci.RequestInfo{})
	if err != nil {
		return nil, err
	}
	app.committed.Store(info.LastBlockHeight)
	return resp, nil
}

func TestStatusUnderAutobahnReportsLatestBlockHashAndTime(t *testing.T) {
	const blocks = 3
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	rng := utils.TestRng()
	kv := &statusAutobahnObservedApplication{
		Application: kvstore.NewApplication(),
		committed:   utils.NewAtomicSend[int64](0),
	}
	t.Cleanup(func() { require.NoError(t, kv.Close()) })
	app := proxy.New(kv, proxy.NopMetrics())
	stateStore := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	router, giga := newAutobahnTestRouter(t, rng, "autobahn-status-test", app, stateStore, 1, 1_000)
	env := &Environment{App: app, StateStore: stateStore, BlockStore: blockStore, Router: router}

	startup, err := env.Status(ctx)
	require.NoError(t, err)
	require.Zero(t, startup.SyncInfo.LatestBlockHeight)
	require.Empty(t, startup.SyncInfo.LatestBlockHash)
	require.True(t, startup.SyncInfo.LatestBlockTime.Equal(time.Unix(0, 0)))

	err = scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for i := range blocks + 1 {
			resp, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value%d", i, i)))
			if err != nil {
				return fmt.Errorf("giga.InsertTx(): %w", err)
			}
			require.False(t, resp.IsErr())
		}
		_, err := kv.committed.Subscribe().Wait(ctx, func(height int64) bool { return height >= blocks })
		require.NoError(t, err)

		info, err := env.ABCIInfo(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(blocks), info.Response.LastBlockHeight)
		block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(info.Response.LastBlockHeight))
		require.NoError(t, err)
		require.NotEmpty(t, block.BlockID.Hash)
		require.False(t, block.Block.Header.Time.IsZero())
		status, err := env.Status(ctx)
		require.NoError(t, err)
		require.Equal(t, info.Response.LastBlockHeight, status.SyncInfo.LatestBlockHeight)
		require.Equal(t, info.Response.LastBlockAppHash, []byte(status.SyncInfo.LatestAppHash))
		require.Equal(t, block.BlockID.Hash, status.SyncInfo.LatestBlockHash)
		require.True(t, block.Block.Header.Time.Equal(status.SyncInfo.LatestBlockTime))
		require.Zero(t, blockStore.Height(), "Autobahn does not populate the CometBFT block store")
		return nil
	})
	require.NoError(t, err)

	// A fresh router has no cached blocks for the app's already executed height.
	restarted, emptyGiga := newAutobahnTestRouter(t, rng, "autobahn-status-test", app, stateStore, 1, 1_000)
	env.Router = restarted
	lookupCtx, cancelLookup := context.WithCancel(ctx)
	cancelLookup()
	_, err = emptyGiga.BlockByNumber(lookupCtx, blocks)
	require.ErrorIs(t, err, context.Canceled)
	info, err := env.ABCIInfo(ctx)
	require.NoError(t, err)
	status, err := env.Status(lookupCtx)
	require.NoError(t, err)
	require.Equal(t, int64(blocks), status.SyncInfo.LatestBlockHeight)
	require.Equal(t, info.Response.LastBlockAppHash, []byte(status.SyncInfo.LatestAppHash))
	require.Empty(t, status.SyncInfo.LatestBlockHash)
	require.True(t, status.SyncInfo.LatestBlockTime.Equal(time.Unix(0, 0)))
}

func TestStatusUnderAutobahnReportsEarliestRetainedBlock(t *testing.T) {
	for _, retainBlocks := range []int64{1, 2} {
		t.Run(fmt.Sprintf("retain%d", retainBlocks), func(t *testing.T) {
			const blocks = 4
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			rng := utils.TestRng()
			kv := &statusAutobahnObservedApplication{
				Application: kvstore.NewApplication(),
				committed:   utils.NewAtomicSend[int64](0),
			}
			kv.RetainBlocks = retainBlocks
			t.Cleanup(func() { require.NoError(t, kv.Close()) })
			app := proxy.New(kv, proxy.NopMetrics())
			stateStore := sm.NewStore(dbm.NewMemDB())
			t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
			blockStore := store.NewBlockStore(dbm.NewMemDB())
			t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
			router, giga := newAutobahnTestRouter(t, rng, "autobahn-status-retention-test", app, stateStore, 1, 1_000)
			env := &Environment{App: app, StateStore: stateStore, BlockStore: blockStore, Router: router}

			startup, err := env.Status(ctx)
			require.NoError(t, err)
			require.Equal(t, giga.EarliestBlockNumber(), startup.SyncInfo.EarliestBlockHeight)
			require.Empty(t, startup.SyncInfo.EarliestBlockHash)
			require.Empty(t, startup.SyncInfo.EarliestAppHash)
			require.True(t, startup.SyncInfo.EarliestBlockTime.Equal(time.Unix(0, 0)))

			err = scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
				s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
				for i := range blocks + 1 {
					resp, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value%d", i, i)))
					if err != nil {
						return fmt.Errorf("giga.InsertTx(): %w", err)
					}
					require.False(t, resp.IsErr())
				}
				_, err := kv.committed.Subscribe().Wait(ctx, func(height int64) bool { return height >= blocks })
				if err != nil {
					return err
				}
				require.Eventually(t, func() bool {
					return giga.EarliestBlockNumber() == int64(blocks)-retainBlocks
				}, time.Minute, 10*time.Millisecond, "blocks below the retention boundary were not pruned")
				return nil
			})
			require.NoError(t, err)

			info, err := env.ABCIInfo(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(blocks), info.Response.LastBlockHeight)
			earliest := giga.EarliestBlockNumber()
			// Autobahn retains one block below the app's response-retention height.
			retainHeight := info.Response.LastBlockHeight - retainBlocks + 1
			require.Equal(t, retainHeight-1, earliest)
			require.Greater(t, earliest, int64(1))
			_, err = giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(earliest-1))
			require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable)
			_, err = stateStore.LoadFinalizeBlockResponses(earliest - 1)
			require.ErrorIs(t, err, sm.ErrNoFinalizeBlockResponsesForHeight{Height: earliest - 1})
			_, err = stateStore.LoadFinalizeBlockResponses(earliest)
			require.ErrorIs(t, err, sm.ErrNoFinalizeBlockResponsesForHeight{Height: earliest})
			_, err = stateStore.LoadFinalizeBlockResponses(retainHeight)
			require.NoError(t, err)
			block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(earliest))
			require.NoError(t, err)
			require.NotEmpty(t, block.BlockID.Hash)
			require.False(t, block.Block.Header.Time.IsZero())

			status, err := env.Status(ctx)
			require.NoError(t, err)
			require.Equal(t, earliest, status.SyncInfo.EarliestBlockHeight)
			require.Equal(t, block.BlockID.Hash, status.SyncInfo.EarliestBlockHash)
			require.Equal(t, block.Block.Header.AppHash, status.SyncInfo.EarliestAppHash)
			require.True(t, block.Block.Header.Time.Equal(status.SyncInfo.EarliestBlockTime))
			require.Zero(t, blockStore.Height(), "Autobahn does not populate the CometBFT block store")
			require.Nil(t, blockStore.LoadBaseMeta())
		})
	}
}

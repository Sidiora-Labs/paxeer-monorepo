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

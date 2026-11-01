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
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type lagStatusAutobahnApplication struct {
	*statusAutobahnObservedApplication
	executing chan struct{}
	resume    <-chan struct{}
}

func (app *lagStatusAutobahnApplication) FinalizeBlock(ctx context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	if req.Header.Height == 4 {
		close(app.executing)
		select {
		case <-app.resume:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return app.Application.FinalizeBlock(ctx, req)
}

func TestLagStatusUnderAutobahnUsesExecutedAndCommittedHeights(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	resume := make(chan struct{})
	kv := &lagStatusAutobahnApplication{
		statusAutobahnObservedApplication: &statusAutobahnObservedApplication{
			Application: kvstore.NewApplication(),
			committed:   utils.NewAtomicSend[int64](0),
		},
		executing: make(chan struct{}),
		resume:    resume,
	}
	t.Cleanup(func() { require.NoError(t, kv.Close()) })
	app := proxy.New(kv, proxy.NopMetrics())
	stateStore := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	router, giga := newAutobahnTestRouter(t, utils.TestRng(), "autobahn-lag-status-test", app, stateStore, 1, 1_000)
	env := &Environment{App: app, StateStore: stateStore, BlockStore: blockStore, Router: router}

	startup, err := env.LagStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, &coretypes.ResultLagStatus{}, startup)

	err = scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for i := range 5 {
			resp, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value%d", i, i)))
			if err != nil {
				return fmt.Errorf("giga.InsertTx(): %w", err)
			}
			require.False(t, resp.IsErr())
		}
		select {
		case <-kv.executing:
		case <-ctx.Done():
			return ctx.Err()
		}

		info, err := env.ABCIInfo(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(3), info.Response.LastBlockHeight)
		committed := giga.LastCommittedBlockNumber()
		require.GreaterOrEqual(t, committed, int64(4))
		gap := committed - info.Response.LastBlockHeight
		env.Config.LagThreshold = gap
		result, err := env.LagStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, &coretypes.ResultLagStatus{
			CurrentHeight: info.Response.LastBlockHeight,
			MaxPeerHeight: committed,
			Lag:           gap,
		}, result)
		require.Zero(t, blockStore.Height())

		env.Config.LagThreshold = gap - 1
		lagging, err := env.LagStatus(ctx)
		require.ErrorIs(t, err, coretypes.ErrLagIsTooHigh)
		require.Equal(t, result, lagging)

		close(resume)
		_, err = kv.committed.Subscribe().Wait(ctx, func(height int64) bool { return height >= committed })
		require.NoError(t, err)
		env.Config.LagThreshold = 0
		caughtUp, err := env.LagStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, committed, caughtUp.CurrentHeight)
		require.Equal(t, committed, caughtUp.MaxPeerHeight)
		require.Zero(t, caughtUp.Lag)
		return nil
	})
	require.NoError(t, err)

	env.Router, _ = newAutobahnTestRouter(t, utils.TestRng(), "autobahn-lag-status-test", app, stateStore, 1, 1_000)
	result, err := env.LagStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(4), result.CurrentHeight)
	require.Zero(t, result.MaxPeerHeight)
	require.Zero(t, result.Lag)

	env.Router = nil
	result, err = env.LagStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, &coretypes.ResultLagStatus{}, result)
}

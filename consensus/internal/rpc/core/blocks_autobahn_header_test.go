package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestHeaderUnderAutobahnMatchesBlock(t *testing.T) {
	const blocks = 6
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	rng := utils.TestRng()
	kv := &statusAutobahnObservedApplication{
		Application: kvstore.NewApplication(),
		committed:   utils.NewAtomicSend[int64](0),
	}
	kv.RetainBlocks = 3
	t.Cleanup(func() { require.NoError(t, kv.Close()) })
	app := proxy.New(kv, proxy.NopMetrics())
	stateStore := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	router, giga := newAutobahnTestRouter(t, rng, "autobahn-header-test", app, stateStore, 1, 1_000)
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
		_, err := kv.committed.Subscribe().Wait(ctx, func(height int64) bool { return height >= blocks })
		require.NoError(t, err)
		info, err := env.ABCIInfo(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(blocks), info.Response.LastBlockHeight)
		require.Zero(t, blockStore.Height())
		require.Zero(t, blockStore.Base())

		for _, tc := range []struct {
			name   string
			height coretypes.Int64
			latest bool
		}{
			{name: "historical", height: 3},
			{name: "head", height: blocks},
			{name: "latest", height: blocks, latest: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				req := &coretypes.RequestBlockInfo{}
				if !tc.latest {
					req.Height = &tc.height
				}
				block, err := env.Block(ctx, req)
				require.NoError(t, err)
				require.NotNil(t, block.Block)
				header, err := env.Header(ctx, req)
				require.NoError(t, err)
				require.NotNil(t, header.Header)
				require.Equal(t, block.Block.Header, *header.Header)
				require.Equal(t, int64(tc.height), header.Header.Height)
				require.Equal(t, "autobahn-header-test", header.Header.ChainID)
				require.False(t, header.Header.Time.IsZero())
			})
		}

		for _, tc := range []struct {
			name   string
			height coretypes.Int64
			want   error
		}{
			{name: "zero", height: 0, want: coretypes.ErrZeroOrNegativeHeight},
			{name: "negative", height: -1, want: coretypes.ErrZeroOrNegativeHeight},
			{name: "minimum", height: math.MinInt64, want: coretypes.ErrZeroOrNegativeHeight},
			{name: "above_head", height: blocks + 1, want: coretypes.ErrHeightExceedsChainHead},
			{name: "maximum", height: math.MaxInt64, want: coretypes.ErrHeightExceedsChainHead},
		} {
			t.Run(tc.name, func(t *testing.T) {
				header, err := env.Header(ctx, &coretypes.RequestBlockInfo{Height: &tc.height})
				require.ErrorIs(t, err, tc.want)
				require.Nil(t, header)
			})
		}

		t.Run("pruned", func(t *testing.T) {
			_, routerErr := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(1))
			require.ErrorIs(t, routerErr, coretypes.ErrHeightNotAvailable)
			height := coretypes.Int64(1)
			header, err := env.Header(ctx, &coretypes.RequestBlockInfo{Height: &height})
			require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable)
			require.IsType(t, routerErr, err)
			require.Same(t, errors.Unwrap(routerErr), errors.Unwrap(err))
			require.Nil(t, header)
		})
		return nil
	})
	require.NoError(t, err)
}

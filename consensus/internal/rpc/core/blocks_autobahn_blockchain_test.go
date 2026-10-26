package core

import (
	"context"
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

type blockchainAutobahnObservedStore struct {
	sm.Store
	pruned utils.AtomicSend[int64]
}

func (store *blockchainAutobahnObservedStore) PruneFinalizeBlockResponses(height int64) error {
	if err := store.Store.PruneFinalizeBlockResponses(height); err != nil {
		return err
	}
	store.pruned.Store(height)
	return nil
}

func TestBlockchainInfoUnderAutobahnListsFinalizedBlocks(t *testing.T) {
	const blocks = 25
	for _, retention := range []struct {
		name         string
		retainBlocks int64
		first        int64
	}{
		{name: "archive", first: 1},
		// PruneBefore keeps the block immediately preceding RetainHeight.
		{name: "pruned", retainBlocks: 21, first: 4},
	} {
		t.Run(retention.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			rng := utils.TestRng()
			kv := &statusAutobahnObservedApplication{
				Application: kvstore.NewApplication(),
				committed:   utils.NewAtomicSend[int64](0),
			}
			kv.RetainBlocks = retention.retainBlocks
			t.Cleanup(func() { require.NoError(t, kv.Close()) })
			app := proxy.New(kv, proxy.NopMetrics())
			stateStore := &blockchainAutobahnObservedStore{
				Store:  sm.NewStore(dbm.NewMemDB()),
				pruned: utils.NewAtomicSend[int64](0),
			}
			t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
			blockStore := store.NewBlockStore(dbm.NewMemDB())
			t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
			router, giga := newAutobahnTestRouter(t, rng, "autobahn-blockchain-test", app, stateStore, 1, 1_000)
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
				if err != nil {
					return err
				}
				if retention.first > 1 {
					// The router prunes its blocks and stored responses after Commit.
					_, err = stateStore.pruned.Subscribe().Wait(ctx, func(height int64) bool { return height >= retention.first+1 })
				}
				return err
			})
			require.NoError(t, err)
			info, err := env.ABCIInfo(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(blocks), info.Response.LastBlockHeight)
			require.Zero(t, blockStore.Base())
			require.Zero(t, blockStore.Height())
			require.Equal(t, retention.first, giga.EarliestBlockNumber())
			if retention.first > 1 {
				_, err := giga.BlockByNumber(ctx, 1)
				require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable)
			}

			for _, tc := range []struct {
				name        string
				min, max    coretypes.Int64
				first, last int64
			}{
				{name: "defaults", first: blocks, last: 6},
				{name: "limit", min: 1, max: blocks, first: blocks, last: 6},
				{name: "clamp_max", min: 24, max: math.MaxInt64, first: blocks, last: 24},
				{name: "bounded", min: 1, max: 5, first: 5, last: retention.first},
				{name: "default_min", max: 5, first: 5, last: retention.first},
				{name: "single", min: blocks, max: blocks, first: blocks, last: blocks},
			} {
				t.Run(tc.name, func(t *testing.T) {
					res, err := env.BlockchainInfo(ctx, &coretypes.RequestBlockchainInfo{MinHeight: tc.min, MaxHeight: tc.max})
					require.NoError(t, err)
					require.Equal(t, info.Response.LastBlockHeight, res.LastHeight)
					require.Len(t, res.BlockMetas, int(tc.first-tc.last+1))
					for i, meta := range res.BlockMetas {
						height := tc.first - int64(i)
						block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(height))
						require.NoError(t, err)
						require.NotNil(t, meta)
						require.Equal(t, height, meta.Header.Height)
						require.Equal(t, block.BlockID, meta.BlockID)
						require.NotEmpty(t, meta.BlockID.Hash)
						require.Equal(t, block.Block.Header, meta.Header)
						require.Equal(t, block.Block.Size(), meta.BlockSize)
						require.Positive(t, meta.BlockSize)
						require.Equal(t, len(block.Block.Data.Txs), meta.NumTxs)
						require.Equal(t, 1, meta.NumTxs)
						require.Nil(t, blockStore.LoadBlockMeta(height))
					}
				})
			}

			if retention.first > 1 {
				res, err := env.BlockchainInfo(ctx, &coretypes.RequestBlockchainInfo{MinHeight: 1, MaxHeight: 3})
				require.NoError(t, err)
				require.Equal(t, int64(blocks), res.LastHeight)
				require.NotNil(t, res.BlockMetas)
				require.Empty(t, res.BlockMetas)
			}
			for _, tc := range []struct {
				name     string
				min, max coretypes.Int64
				want     error
			}{
				{name: "negative_min", min: -1, want: coretypes.ErrZeroOrNegativeHeight},
				{name: "negative_max", max: -1, want: coretypes.ErrZeroOrNegativeHeight},
				{name: "above_head", min: blocks + 1, want: coretypes.ErrInvalidRequest},
				{name: "reversed", min: 5, max: 4, want: coretypes.ErrInvalidRequest},
			} {
				t.Run(tc.name, func(t *testing.T) {
					res, err := env.BlockchainInfo(ctx, &coretypes.RequestBlockchainInfo{MinHeight: tc.min, MaxHeight: tc.max})
					require.ErrorIs(t, err, tc.want)
					require.Nil(t, res)
				})
			}

			restarted, _ := newAutobahnTestRouter(t, rng, "autobahn-blockchain-test", app, stateStore, 1, 1_000)
			env.Router = restarted
			lookupCtx, cancelLookup := context.WithCancel(ctx)
			cancelLookup()
			res, err := env.BlockchainInfo(lookupCtx, &coretypes.RequestBlockchainInfo{MinHeight: blocks, MaxHeight: blocks})
			require.ErrorIs(t, err, context.Canceled)
			require.Nil(t, res)
		})
	}
}

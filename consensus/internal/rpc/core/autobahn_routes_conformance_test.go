package core

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
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

func TestAutobahnRoutesAnswerWithoutPanic(t *testing.T) {
	const blocks = 3
	const chainID = "autobahn-routes-conformance-test"
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	rng := utils.TestRng()
	application := &statusAutobahnObservedApplication{
		Application: kvstore.NewApplication(),
		committed:   utils.NewAtomicSend[int64](0),
	}
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	app := proxy.New(application, proxy.NopMetrics())
	stateStore := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	router, giga := newAutobahnTestRouter(t, rng, chainID, app, stateStore, 1, 1_000)
	genesis := &types.GenesisDoc{ChainID: chainID, InitialHeight: 1}
	require.NoError(t, genesis.ValidateAndComplete())
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(ctx))
	defer bus.Wait()
	defer bus.Stop()
	giga.SetEventBus(bus)
	sinks := []indexer.EventSink{kv.NewEventSink(dbm.NewMemDB())}
	service := indexer.NewService(indexer.ServiceArgs{Sinks: sinks, EventBus: bus})
	require.NoError(t, service.Start(ctx))
	defer service.Wait()
	defer service.Stop()
	headers, err := bus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: t.Name(), Query: types.EventQueryNewBlockHeader, Limit: 16,
	})
	require.NoError(t, err)
	txs, err := bus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: t.Name(), Query: types.EventQueryTx, Limit: 16,
	})
	require.NoError(t, err)
	env := &Environment{
		App: app, StateStore: stateStore, BlockStore: blockStore,
		Router: router, GenDoc: genesis, Config: *config.DefaultRPCConfig(),
		EventBus: bus, EventSinks: sinks,
	}

	err = scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for i := range blocks + 1 {
			response, err := giga.InsertTx(ctx, types.Tx(fmt.Sprintf("key%d=value%d", i, i)))
			if err != nil {
				return err
			}
			require.True(t, response.IsOK())
		}
		for height := int64(1); height <= blocks; height++ {
			header := nextAutobahnIndexedHeader(t, ctx, headers, txs)
			require.Equal(t, height, header.Header.Height)
		}
		_, err := application.committed.Subscribe().Wait(ctx, func(height int64) bool { return height >= blocks })
		if err != nil {
			return err
		}
		info, err := app.Info(ctx, &abci.RequestInfo{})
		require.NoError(t, err)
		require.Equal(t, int64(blocks), info.LastBlockHeight)
		require.Zero(t, blockStore.Height())
		for height := int64(1); height <= blocks; height++ {
			response, err := stateStore.LoadFinalizeBlockResponses(height)
			require.NoError(t, err)
			require.NotNil(t, response)
		}
		block, err := giga.BlockByNumber(ctx, blocks)
		require.NoError(t, err)
		require.Len(t, block.Block.Data.Txs, 1)
		tx := block.Block.Data.Txs[0]
		height := coretypes.Int64(blocks)
		blockRequest := &coretypes.RequestBlockInfo{Height: &height}
		hashRequest := &coretypes.RequestBlockByHash{Hash: block.BlockID.Hash}

		for _, route := range []struct {
			name      string
			call      func() (any, error)
			wantError error
		}{
			{"Status", func() (any, error) { return env.Status(ctx) }, nil},
			{"LagStatus", func() (any, error) { return env.LagStatus(ctx) }, nil},
			{"BlockchainInfo", func() (any, error) {
				return env.BlockchainInfo(ctx, &coretypes.RequestBlockchainInfo{MinHeight: 1, MaxHeight: blocks})
			}, nil},
			{"Block", func() (any, error) { return env.Block(ctx, blockRequest) }, nil},
			{"BlockByHash", func() (any, error) { return env.BlockByHash(ctx, hashRequest) }, nil},
			{"BlockResults", func() (any, error) { return env.BlockResults(ctx, blockRequest) }, nil},
			{"Header", func() (any, error) { return env.Header(ctx, blockRequest) }, nil},
			{"HeaderByHash", func() (any, error) { return env.HeaderByHash(ctx, hashRequest) }, nil},
			{"Commit", func() (any, error) { return env.Commit(ctx, blockRequest) }, coretypes.ErrCommitNotAvailable},
			{"Validators", func() (any, error) {
				return env.Validators(ctx, &coretypes.RequestValidators{Height: &height})
			}, nil},
			{"ConsensusParams", func() (any, error) {
				return env.ConsensusParams(ctx, &coretypes.RequestConsensusParams{Height: &height})
			}, nil},
			{"DumpConsensusState", func() (any, error) { return env.DumpConsensusState(ctx) }, coretypes.ErrConsensusStateNotAvailable},
			{"GetConsensusState", func() (any, error) { return env.GetConsensusState(ctx) }, coretypes.ErrConsensusStateNotAvailable},
			{"BroadcastEvidence", func() (any, error) {
				return env.BroadcastEvidence(ctx, &coretypes.RequestBroadcastEvidence{})
			}, coretypes.ErrEvidenceNotAccepted},
			{"Tx", func() (any, error) {
				return env.Tx(ctx, &coretypes.RequestTx{Hash: tx.Hash().Bytes(), Prove: true})
			}, nil},
			{"TxSearch", func() (any, error) {
				return env.TxSearch(ctx, &coretypes.RequestTxSearch{Query: "tx.height=3", Prove: true})
			}, nil},
			{"BlockSearch", func() (any, error) {
				return env.BlockSearch(ctx, &coretypes.RequestBlockSearch{Query: "block.height=3"})
			}, nil},
		} {
			t.Run(route.name, func(t *testing.T) {
				defer func() {
					if panicValue := recover(); panicValue != nil {
						t.Fatalf("%s panicked: %v", route.name, panicValue)
					}
				}()
				result, err := route.call()
				if err != nil {
					require.True(t,
						errors.Is(err, coretypes.ErrCommitNotAvailable) ||
							errors.Is(err, coretypes.ErrConsensusStateNotAvailable) ||
							errors.Is(err, coretypes.ErrEvidenceNotAccepted) ||
							errors.Is(err, coretypes.ErrHeightNotAvailable),
						"unexpected error: %v", err)
					require.Nil(t, result)
					if route.wantError != nil {
						require.ErrorIs(t, err, route.wantError)
					}
					return
				}
				require.Nil(t, route.wantError, "route must return its unavailable sentinel")
				require.NotNil(t, result, "route returned no result and no error")
				switch result := result.(type) {
				case *coretypes.ResultStatus:
					require.Equal(t, int64(blocks), result.SyncInfo.LatestBlockHeight)
				case *coretypes.ResultLagStatus:
					require.Equal(t, int64(blocks), result.CurrentHeight)
				case *coretypes.ResultBlockchainInfo:
					require.Equal(t, int64(blocks), result.LastHeight)
					require.Len(t, result.BlockMetas, blocks)
				case *coretypes.ResultBlock:
					require.NotNil(t, result.Block)
					require.Equal(t, int64(blocks), result.Block.Height)
					require.Equal(t, block.BlockID, result.BlockID)
				case *coretypes.ResultBlockResults:
					require.Equal(t, int64(blocks), result.Height)
					require.Len(t, result.TxsResults, 1)
				case *coretypes.ResultHeader:
					require.NotNil(t, result.Header)
					require.Equal(t, block.Block.Header, *result.Header)
				case *coretypes.ResultValidators:
					require.Equal(t, int64(blocks), result.BlockHeight)
				case *coretypes.ResultConsensusParams:
					require.Equal(t, int64(blocks), result.BlockHeight)
					require.Equal(t, *genesis.ConsensusParams, result.ConsensusParams)
				case *coretypes.ResultTx:
					require.Equal(t, tx, result.Tx)
					require.Equal(t, int64(blocks), result.Height)
					require.NoError(t, result.Proof.Validate(block.Block.Data.Txs.Hash()))
				case *coretypes.ResultTxSearch:
					require.Equal(t, 1, result.TotalCount)
					require.Len(t, result.Txs, 1)
					require.Equal(t, tx, result.Txs[0].Tx)
				case *coretypes.ResultBlockSearch:
					require.Equal(t, 1, result.TotalCount)
					require.Len(t, result.Blocks, 1)
					require.Equal(t, int64(blocks), result.Blocks[0].Block.Height)
				default:
					t.Fatalf("unexpected response type %T", result)
				}
			})
		}
		return nil
	})
	require.NoError(t, err)
}

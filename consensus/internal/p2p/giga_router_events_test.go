package p2p

import (
	"context"
	"testing"
	"time"

	dbm "github.com/tendermint/tm-db"
	"golang.org/x/time/rate"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/consensus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/producer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/conn"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	tmpubsub "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	tmquery "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func runGigaRouterEventTest(t *testing.T, rng utils.Rng, query *tmquery.Query, stopBus bool,
	check func(context.Context, testAppState, *testResponseStore, eventbus.Subscription),
) {
	t.Helper()
	const maxTxsPerBlock = 3
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(ctx))
	defer bus.Stop()
	sub, err := bus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{
		ClientID: t.Name(), Query: query, Limit: 128,
	})
	require.NoError(t, err)
	if stopBus {
		bus.Stop()
		bus.Wait()
	}

	_, keys := atypes.GenCommittee(rng, 1)
	cfg := &testNodeCfg{
		validatorKey: keys[0], nodeKey: makeKey(rng), addr: tcp.TestReserveAddr(),
	}
	genDoc := &types.GenesisDoc{
		ChainID: "giga-router-events-test", InitialHeight: rng.Int63n(100000) + 1,
		AppState: testAppStateJSON(rng),
	}
	require.NoError(t, genDoc.ValidateAndComplete())
	app := newTestApp()
	store := newTestResponseStore()
	nodeInfo := makeInfo(cfg.nodeKey)
	nodeInfo.ListenAddr = cfg.addr.String()
	nodeInfo.Network = genDoc.ChainID
	e := Endpoint{AddrPort: cfg.addr}
	router, err := NewRouter(NopMetrics(), cfg.nodeKey,
		func() *types.NodeInfo { return &nodeInfo }, dbm.NewMemDB(), &RouterOptions{
			SelfAddress: utils.Some(e.NodeAddress(cfg.nodeKey.Public().NodeID())),
			Endpoint:    e, Connection: conn.DefaultMConnConfig(),
			IncomingConnectionWindow: utils.Some(time.Duration(0)),
			MaxAcceptRate:            utils.Some(rate.Inf), MaxDialRate: utils.Some(rate.Limit(30)),
			Giga: utils.Some(&GigaRouterConfig{
				DialInterval:   100 * time.Millisecond,
				ValidatorAddrs: map[atypes.PublicKey]GigaNodeAddr{cfg.validatorKey.Public(): cfg.GigaNodeAddr()},
				Consensus: &consensus.Config{
					Key: cfg.validatorKey, ViewTimeout: func(atypes.View) time.Duration { return time.Hour },
					PersistentStateDir: utils.None[string](),
				},
				Producer: &producer.Config{
					App:                  proxy.New(app, proxy.NopMetrics()),
					MaxGasWantedPerBlock: maxTxsPerBlock, MaxGasEstimatedPerBlock: maxTxsPerBlock,
					MaxTxsPerBlock: maxTxsPerBlock, MaxTxsPerSecond: utils.None[uint64](),
					BlockInterval: time.Hour,
				},
				GenDoc: genDoc, StateStore: store,
			}),
		})
	require.NoError(t, err)
	giga := router.Giga().OrPanic("non-giga router")
	giga.SetEventBus(bus)

	err = scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		var txs [][]byte
		for range 2*maxTxsPerBlock + 1 {
			tx := utils.GenBytes(rng, 100)
			if _, err := giga.InsertTx(ctx, tx); err != nil {
				return err
			}
			txs = append(txs, tx)
		}
		for _, tx := range txs[:2*maxTxsPerBlock] {
			if err := app.WaitForTx(ctx, tx); err != nil {
				return err
			}
		}
		snapshot := app.Snapshot()
		var executed [][]byte
		for _, req := range snapshot.Blocks {
			executed = append(executed, req.Txs...)
		}
		require.Equal(t, txs[:2*maxTxsPerBlock], executed)
		require.Equal(t, len(snapshot.Blocks), len(snapshot.Responses))
		last := snapshot.Blocks[len(snapshot.Blocks)-1]
		proposal, err := giga.data.AppProposal(ctx, atypes.GlobalBlockNumber(last.Header.Height))
		if err != nil {
			return err
		}
		require.Equal(t, snapshot.AppHash[:], []byte(proposal.AppHash()))
		check(ctx, snapshot, store, sub)
		return nil
	})
	require.NoError(t, err)
}

func TestGigaRouterPublishesNewBlockHeaderEvents(t *testing.T) {
	rng := utils.TestRng()
	runGigaRouterEventTest(t, rng, types.EventQueryNewBlockHeader, false,
		func(ctx context.Context, snapshot testAppState, store *testResponseStore, sub eventbus.Subscription) {
			for i, req := range snapshot.Blocks {
				msg, err := sub.Next(ctx)
				require.NoError(t, err)
				event, ok := msg.Data().(types.EventDataNewBlockHeader)
				require.True(t, ok)
				require.Equal(t, req.Header, event.Header.ToProto())
				require.Equal(t, int64(len(req.Txs)), event.NumTxs)
				got, err := event.ResultFinalizeBlock.Marshal()
				require.NoError(t, err)
				require.Equal(t, snapshot.Responses[i], got)
				stored, err := store.WaitFor(ctx, req.Header.Height)
				require.NoError(t, err)
				require.Equal(t, stored, got)
			}
		})

	t.Run("publish errors preserve execution and app hash", func(t *testing.T) {
		rng := utils.TestRng()
		runGigaRouterEventTest(t, rng, types.EventQueryNewBlockHeader, true,
			func(ctx context.Context, snapshot testAppState, store *testResponseStore, _ eventbus.Subscription) {
				for i, req := range snapshot.Blocks {
					stored, err := store.WaitFor(ctx, req.Header.Height)
					require.NoError(t, err)
					require.Equal(t, snapshot.Responses[i], stored)
				}
			})
	})
}

func TestGigaRouterPublishesTxEvents(t *testing.T) {
	rng := utils.TestRng()
	runGigaRouterEventTest(t, rng, types.EventQueryTx, false,
		func(ctx context.Context, snapshot testAppState, store *testResponseStore, sub eventbus.Subscription) {
			for _, req := range snapshot.Blocks {
				stored, err := store.WaitFor(ctx, req.Header.Height)
				require.NoError(t, err)
				var resp abci.ResponseFinalizeBlock
				require.NoError(t, resp.Unmarshal(stored))
				require.Len(t, resp.TxResults, len(req.Txs))
				for i, tx := range req.Txs {
					msg, err := sub.Next(ctx)
					require.NoError(t, err)
					event, ok := msg.Data().(types.EventDataTx)
					require.True(t, ok)
					require.Equal(t, req.Header.Height, event.Height)
					require.Equal(t, uint32(i), event.Index)
					require.Equal(t, tx, event.Tx)
					require.Equal(t, *resp.TxResults[i], event.Result)
				}
			}
		})
}

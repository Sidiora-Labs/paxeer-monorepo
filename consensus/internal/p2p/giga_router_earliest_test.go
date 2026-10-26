package p2p_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbm "github.com/tendermint/tm-db"
	"golang.org/x/time/rate"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/consensus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/producer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/conn"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestGigaRouterEarliestBlockNumberFollowsPruning(t *testing.T) {
	const txsPerNode = 4
	const retainBlocks = 3
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	rng := utils.TestRng()
	_, validatorKeys := atypes.GenCommittee(rng, 4)
	nodeKeys := make([]p2p.NodeSecretKey, len(validatorKeys))
	endpoints := make([]p2p.Endpoint, len(validatorKeys))
	addrs := map[atypes.PublicKey]p2p.GigaNodeAddr{}
	for i, key := range validatorKeys {
		nodeKeys[i] = p2p.NodeSecretKey(ed25519.TestSecretKey(utils.GenBytes(rng, 32)))
		addr := tcp.TestReserveAddr()
		endpoints[i] = p2p.Endpoint{AddrPort: addr}
		addrs[key.Public()] = p2p.GigaNodeAddr{
			Key:      nodeKeys[i].Public(),
			HostPort: tcp.HostPort{Hostname: addr.Addr().String(), Port: addr.Port()},
		}
	}
	genDoc := &types.GenesisDoc{ChainID: "giga-router-earliest-test", InitialHeight: 1}
	require.NoError(t, genDoc.ValidateAndComplete())
	var routers []*p2p.GigaRouter
	var apps []*kvstore.Application
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		for i, validatorKey := range validatorKeys {
			app := kvstore.NewApplication()
			app.RetainBlocks = retainBlocks
			t.Cleanup(func() { require.NoError(t, app.Close()) })
			apps = append(apps, app)
			db := dbm.NewMemDB()
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			nodeInfo := types.NodeInfo{
				NodeID:          nodeKeys[i].Public().NodeID(),
				ListenAddr:      endpoints[i].AddrPort.String(),
				Network:         genDoc.ChainID,
				Moniker:         fmt.Sprintf("earliest-%d", i),
				ProtocolVersion: types.ProtocolVersion{P2P: 1, Block: 2, App: 3},
			}
			router, err := p2p.NewRouter(p2p.NopMetrics(), nodeKeys[i],
				func() *types.NodeInfo { return &nodeInfo }, db,
				&p2p.RouterOptions{
					SelfAddress:              utils.Some(endpoints[i].NodeAddress(nodeKeys[i].Public().NodeID())),
					Endpoint:                 endpoints[i],
					Connection:               conn.DefaultMConnConfig(),
					IncomingConnectionWindow: utils.Some(time.Duration(0)),
					MaxAcceptRate:            utils.Some(rate.Inf),
					MaxDialRate:              utils.Some(rate.Limit(30)),
					Giga: utils.Some(&p2p.GigaRouterConfig{
						DialInterval:   100 * time.Millisecond,
						ValidatorAddrs: addrs,
						Consensus: &consensus.Config{
							Key:                validatorKey,
							ViewTimeout:        func(atypes.View) time.Duration { return time.Hour },
							PersistentStateDir: utils.None[string](),
						},
						Producer: &producer.Config{
							App:                     proxy.New(app, proxy.NopMetrics()),
							MaxGasWantedPerBlock:    21_000,
							MaxGasEstimatedPerBlock: 21_000,
							MaxTxsPerBlock:          1,
							MaxTxsPerSecond:         utils.None[uint64](),
							BlockInterval:           100 * time.Millisecond,
						},
						GenDoc:     genDoc,
						StateStore: state.NewStore(db),
					}),
				})
			if err != nil {
				return fmt.Errorf("NewRouter[%d]: %w", i, err)
			}
			giga := router.Giga().OrPanic("non-giga router")
			require.Equal(t, genDoc.InitialHeight, giga.EarliestBlockNumber())
			routers = append(routers, giga)
			s.SpawnBgNamed(fmt.Sprintf("router[%d]", i), func() error {
				return utils.IgnoreCancel(router.Run(ctx))
			})
		}
		for i, router := range routers {
			for j := range txsPerNode {
				resp, err := router.InsertTx(ctx, []byte(fmt.Sprintf("key-%d-%d=value", i, j)))
				if err != nil {
					return err
				}
				require.Equal(t, abci.CodeTypeOK, resp.Code)
			}
		}
		for _, app := range apps {
			for {
				info, err := app.Info(ctx, &abci.RequestInfo{})
				if err != nil {
					return err
				}
				if info.LastBlockHeight == int64(len(routers)*txsPerNode) {
					break
				}
				if err := utils.Sleep(ctx, 10*time.Millisecond); err != nil {
					return err
				}
			}
		}
		return nil
	})
	require.NoError(t, err)
	for i, router := range routers {
		earliest := router.EarliestBlockNumber()
		require.Greater(t, earliest, genDoc.InitialHeight, "router[%d] must prune executed blocks", i)
		for height := int64(0); height <= int64(len(routers)*txsPerNode); height++ {
			block, err := router.BlockByNumber(ctx, atypes.GlobalBlockNumber(height)) //nolint:gosec // height is nonnegative.
			if height < earliest {
				require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable, "router[%d] height %d", i, height)
				require.Nil(t, block)
				continue
			}
			require.NoError(t, err, "router[%d] height %d", i, height)
			require.NotNil(t, block)
			require.NotNil(t, block.Block)
			require.Equal(t, height, block.Block.Height)
		}
	}
}

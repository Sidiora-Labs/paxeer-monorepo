package node

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestGigaRouterReceivesEventBus(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "without Autobahn"
		if enabled {
			name = "with Autobahn"
		}
		t.Run(name, func(t *testing.T) {
			cfg, err := config.ResetTestRoot(t.TempDir(), "node_giga_eventbus_test")
			require.NoError(t, err)
			cfg.RPC.ListenAddress = ""
			gen, err := types.GenesisDocFromFile(cfg.GenesisFile())
			require.NoError(t, err)
			gen.ConsensusParams.Block.MaxGas = testGenesisMaxGas
			if enabled {
				pv, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
				require.NoError(t, err)
				nodeKey, err := types.LoadOrGenNodeKey(cfg.NodeKeyFile())
				require.NoError(t, err)
				addr, err := tcp.ParseHostPort(cfg.P2P.ListenAddress)
				require.NoError(t, err)
				fc := defaultFileConfig(t, []config.AutobahnValidator{{
					ValidatorKey: atypes.SecretKeyFromED25519(pv.Key.PrivKey).Public(),
					NodeKey:      p2p.NodeSecretKey(nodeKey).Public(),
					Address:      addr,
				}})
				fc.MaxTxsPerBlock = 1
				cfg.AutobahnConfigFile = writeAutobahnConfig(t, fc)
			}

			app := kvstore.NewApplication()
			app.SetValidators(gen.ValidatorUpdates())
			ns, err := New(t.Context(), cfg, func() {}, app, gen, nil,
				DefaultMetricsProvider(cfg.Instrumentation)(cfg.ChainID()), types.DefaultConsensusPolicy())
			require.NoError(t, err)
			n, ok := ns.(*nodeImpl)
			require.True(t, ok)
			started := false
			t.Cleanup(func() {
				if started {
					n.Stop()
					n.Wait()
				} else {
					n.OnStop()
				}
			})
			giga, ok := n.router.Giga().Get()
			require.Equal(t, enabled, ok)
			if !enabled {
				require.True(t, n.mempool.IsPresent())
				return
			}

			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			blocks, err := n.EventBus().SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
				ClientID: t.Name() + "/blocks", Query: types.EventQueryNewBlockHeader, Limit: 10,
			})
			require.NoError(t, err)
			txs, err := n.EventBus().SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
				ClientID: t.Name() + "/txs", Query: types.EventQueryTx, Limit: 10,
			})
			require.NoError(t, err)
			require.NoError(t, n.Start(ctx))
			started = true
			tx := types.Tx("eventbus=autobahn")
			check, err := giga.InsertTx(ctx, tx)
			require.NoError(t, err)
			require.Zero(t, check.Code)

			blockMsg, err := blocks.Next(ctx)
			require.NoError(t, err, "Autobahn must publish block events on the node event bus")
			block, ok := blockMsg.Data().(types.EventDataNewBlockHeader)
			require.True(t, ok)
			require.Equal(t, gen.ChainID, block.Header.ChainID)
			require.Equal(t, gen.InitialHeight, block.Header.Height)
			require.Equal(t, int64(1), block.NumTxs)
			txMsg, err := txs.Next(ctx)
			require.NoError(t, err, "Autobahn must publish transaction events on the node event bus")
			txEvent, ok := txMsg.Data().(types.EventDataTx)
			require.True(t, ok)
			require.Equal(t, block.Header.Height, txEvent.Height)
			require.Equal(t, []byte(tx), txEvent.Tx)
			require.Zero(t, txEvent.Result.Code)
		})
	}
}

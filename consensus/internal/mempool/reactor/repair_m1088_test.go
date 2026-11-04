package reactor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/mempool"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
)

func TestRepairM_1088_ShutdownBeforeReadiness(t *testing.T) {
	for _, broadcast := range []bool{true, false} {
		for _, cancelParent := range []bool{false, true} {
			t.Run(fmt.Sprintf("broadcast=%v/cancelParent=%v", broadcast, cancelParent), func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				t.Cleanup(cancel)
				network := p2p.MakeTestNetwork(t, p2p.TestNetworkOptions{NumNodes: 1})
				cfg := config.TestMempoolConfig()
				cfg.Broadcast = broadcast
				txmp := mempool.NewTxMempool(cfg.ToMempoolConfig(), kvstore.NewProxy(), mempool.NopMetrics(), mempool.NopTxConstraintsFetcher)
				reactor, err := NewReactor(cfg, txmp, network.Nodes()[0].Router)
				require.NoError(t, err)
				require.NoError(t, reactor.Start(ctx))
				t.Cleanup(func() {
					cancel()
					if reactor.IsRunning() {
						// Release a regressed readiness wait so a failed assertion can finish cleanup.
						reactor.MarkReadyToStart()
					}
					reactor.Stop()
				})
				require.True(t, reactor.IsRunning())

				stopped := make(chan struct{})
				go func() {
					if cancelParent {
						cancel()
						reactor.Wait()
					} else {
						reactor.Stop()
					}
					close(stopped)
				}()

				select {
				case <-stopped:
				case <-time.After(2 * time.Second):
					t.Fatal("reactor shutdown blocked before readiness")
				}
				require.False(t, reactor.IsRunning())
				require.Equal(t, 0, txmp.Size())
			})
		}
	}
}

func TestRepairM_1088_ReadyReceivesTransactions(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	rng := utils.TestRng()
	rts := setupReactors(ctx, t, 2)
	rts.start(t)

	sender := rts.nodes[0]
	receiver := rts.nodes[1]
	txs := checkTxs(ctx, t, rng, rts.mempools[sender], 1)
	received, err := rts.mempools[receiver].WaitForReadyTx(ctx)
	require.NoError(t, err)
	require.Equal(t, txs[0].tx, received.Value())
	require.Equal(t, 1, rts.mempools[receiver].Size())
}

//go:build gofuzz

package tests

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/mempool"
)

func FuzzMempool(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("key=value"))
	f.Add([]byte{0xff, 0x00, 0x01})

	f.Fuzz(func(t *testing.T, data []byte) {
		cfg := config.DefaultMempoolConfig()
		cfg.Broadcast = false
		mp := mempool.NewTxMempool(cfg.ToMempoolConfig(), kvstore.NewProxy(), mempool.NopMetrics(), mempool.NopTxConstraintsFetcher)
		_, _ = mp.CheckTx(t.Context(), data)
	})
}

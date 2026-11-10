package receipt_test

import (
	"testing"
	"time"

	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/stretchr/testify/require"
)

func TestRepair2239367CloseIdleReceiptStore(t *testing.T) {
	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	cfg.KeepRecent = 1
	cfg.PruneIntervalSeconds = 3600
	key := storetypes.NewKVStoreKey("evm")
	store, err := receipt.NewReceiptStore(cfg, key)
	require.NoError(t, err)
	for range 2 {
		done := make(chan error, 1)
		go func() { done <- store.Close() }()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("idle receipt store close must interrupt the pruning sleep and join its database workers")
		}
	}
	reopened, err := receipt.NewReceiptStore(cfg, key)
	require.NoError(t, err, "receipt ownership must be released after closing the database")
	require.NoError(t, reopened.Close())
}

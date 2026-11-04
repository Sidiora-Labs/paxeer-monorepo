package receipt

import (
	"os"
	"path/filepath"
	"testing"

	dbutils "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/utils"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/wal"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestRepair_104_ZeroWALEntries(t *testing.T) {
	repair104Restart(t, nil, 0, 0, 0)
}

func TestRepair_104_SingleUnappliedEntry(t *testing.T) {
	repair104Restart(t, []int64{7}, 0, 0, 0)
}

func TestRepair_104_MultipleUnappliedEntries(t *testing.T) {
	repair104Restart(t, []int64{7, 11, 19}, 0, 0, 0)
}

func TestRepair_104_SingleUnappliedTail(t *testing.T) {
	repair104Restart(t, []int64{7, 11}, 1, 0, 0)
}

func TestRepair_104_MultipleUnappliedTail(t *testing.T) {
	repair104Restart(t, []int64{7, 11, 19}, 1, 0, 0)
}

func TestRepair_104_FullyAppliedTail(t *testing.T) {
	repair104Restart(t, []int64{7, 11, 19}, 3, 0, 0)
}

func TestRepair_104_StoreAheadOfWAL(t *testing.T) {
	repair104Restart(t, []int64{7, 11, 19}, 3, 23, 0)
}

func TestRepair_104_PrunedWALSingleton(t *testing.T) {
	repair104Restart(t, []int64{7, 11, 19}, 0, 0, 2)
}

func repair104Restart(t *testing.T, versions []int64, applied int, watermark int64, truncated int) {
	t.Helper()
	ctx, storeKey := newTestContext()
	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	cfg.KeepRecent = 0
	producer, err := NewReceiptStore(cfg, storeKey)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, producer.Close()) })

	recoveryCfg := cfg
	recoveryCfg.DBDirectory = t.TempDir()
	consumer, err := NewReceiptStore(recoveryCfg, storeKey)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, consumer.Close()) })

	records := make([]ReceiptRecord, len(versions))
	for i, version := range versions {
		txHash := common.BytesToHash([]byte{byte(i + 1)})
		records[i] = ReceiptRecord{
			TxHash: txHash,
			Receipt: makeTestReceipt(txHash, uint64(version), uint32(i),
				common.HexToAddress("0x104"), []common.Hash{common.HexToHash("0x52")}),
		}
		require.NoError(t, producer.SetReceipts(ctx.WithBlockHeight(version), records[i:i+1]))
		if i < applied {
			require.NoError(t, consumer.SetReceipts(ctx.WithBlockHeight(version), records[i:i+1]))
		}
	}
	require.NoError(t, producer.Close())
	require.NoError(t, consumer.Close())

	if watermark > 0 {
		consumer, err = NewReceiptStore(recoveryCfg, storeKey)
		require.NoError(t, err)
		require.NoError(t, consumer.SetLatestVersion(watermark))
		require.NoError(t, consumer.Close())
	}

	producerWALPath := dbutils.GetChangelogPath(cfg.DBDirectory)
	if truncated > 0 {
		stream, err := wal.NewChangelogWAL(producerWALPath, wal.Config{})
		require.NoError(t, err)
		require.NoError(t, stream.TruncateBefore(uint64(truncated+1)))
		require.NoError(t, stream.Close())
	}
	// Transfer only the real producer's durable WAL, leaving the consumer's
	// receipt data and applied version at their pre-recovery state.
	files, err := os.ReadDir(producerWALPath)
	require.NoError(t, err)
	consumerWALPath := dbutils.GetChangelogPath(recoveryCfg.DBDirectory)
	oldFiles, err := os.ReadDir(consumerWALPath)
	require.NoError(t, err)
	for _, file := range oldFiles {
		require.False(t, file.IsDir())
		require.NoError(t, os.Remove(filepath.Join(consumerWALPath, file.Name())))
	}
	for _, file := range files {
		require.False(t, file.IsDir())
		data, err := os.ReadFile(filepath.Join(producerWALPath, file.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(consumerWALPath, file.Name()), data, 0o600))
	}

	expectedVersion := watermark
	if len(versions) > 0 && versions[len(versions)-1] > expectedVersion {
		expectedVersion = versions[len(versions)-1]
	}
	for restart := 0; restart < 2; restart++ {
		reopened, err := NewReceiptStore(recoveryCfg, storeKey)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, reopened.Close()) })
		require.Equal(t, expectedVersion, reopened.LatestVersion())
		for i, record := range records {
			if i < truncated && i >= applied {
				_, err := reopened.GetReceiptFromStore(ctx, record.TxHash)
				require.ErrorIs(t, err, ErrNotFound)
				continue
			}
			got, err := reopened.GetReceipt(ctx, record.TxHash)
			require.NoError(t, err)
			require.Equal(t, record.Receipt, got)
			stored, err := reopened.GetReceiptFromStore(ctx, record.TxHash)
			require.NoError(t, err)
			require.Equal(t, record.Receipt, stored)
		}
		_, err = reopened.GetReceiptFromStore(ctx, common.HexToHash("0xffff"))
		require.ErrorIs(t, err, ErrNotFound)
		require.NoError(t, reopened.Close())
	}
}

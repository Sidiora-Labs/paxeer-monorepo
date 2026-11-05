package receipt

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	types2 "github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/wal"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// RecoverReceiptStore exposes recoverReceiptStore for testing.
func RecoverReceiptStore(changelogPath string, db types2.StateStore) error {
	entries, err := recoverReceiptStore(changelogPath)
	if err != nil {
		return err
	}
	return replayReceiptStore(entries, db)
}

func ReadReceiptChangelog(stream wal.ChangelogWAL) ([]proto.ChangelogEntry, error) {
	return readReceiptChangelog(stream)
}

// GetLogsForTx exposes getLogsForTx for testing.
func GetLogsForTx(receipt *types.Receipt, logStartIndex uint) []*ethtypes.Log {
	return getLogsForTx(receipt, logStartIndex)
}

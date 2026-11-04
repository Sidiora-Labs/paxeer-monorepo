package rootmulti

import (
	"testing"

	paxdbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

func TestRepairM_608_SnapshotHeightHash(t *testing.T) {
	for _, mode := range []paxdbconfig.WriteMode{
		paxdbconfig.MemiavlOnly,
		paxdbconfig.MigrateEVM,
		paxdbconfig.EVMMigrated,
		paxdbconfig.MigrateAllButBank,
		paxdbconfig.AllMigratedButBank,
		paxdbconfig.MigrateBank,
		paxdbconfig.FlatKVOnly,
		paxdbconfig.TestOnlyDualWrite,
	} {
		t.Run(string(mode), func(t *testing.T) { testSnapshotHeightHashMode(t, mode) })
	}
}

func TestRepairM_608_DualWriteContinuation(t *testing.T) {
	TestFlatKVSnapshotRestoreWithLatticeHash(t)
}

func TestRepairM_608_EVMMigratedContinuation(t *testing.T) {
	TestFlatKVEVMMigratedSnapshotRestore(t)
}

func TestRepairM_608_FlatKVOnlyContinuation(t *testing.T) {
	TestFlatKVOnlySnapshotRestoreAppHashParity(t)
}

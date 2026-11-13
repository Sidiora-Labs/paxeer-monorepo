package rootmulti

import (
	"testing"

	paxdbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

func TestSnapshotHeightHashAcrossWriteModes(t *testing.T) {
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

func TestSnapshotDualWriteContinuation(t *testing.T) {
	TestFlatKVSnapshotRestoreWithLatticeHash(t)
}

func TestSnapshotEVMMigratedContinuation(t *testing.T) {
	TestFlatKVEVMMigratedSnapshotRestore(t)
}

func TestSnapshotFlatKVOnlyContinuation(t *testing.T) {
	TestFlatKVOnlySnapshotRestoreAppHashParity(t)
}

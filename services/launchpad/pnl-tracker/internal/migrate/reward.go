package migrate

import (
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

// LoadRewardRegistry binds the reward registration to its checked-in approval
// and frozen reader interface. Other registry entries retain their own admission
// requirements; this approval covers only migration 180001.
func LoadRewardRegistry(source, approval, readerInterface []byte) (production.MigrationRegistry, error) {
	registry, err := production.LoadMigrationRegistry(source, production.Checksum(source))
	if err != nil {
		return registry, err
	}
	var admitted struct {
		MigrationID      int64            `json:"migration_id"`
		SourceSHA256     string           `json:"source_sha256"`
		Predecessors     map[int64]string `json:"predecessors"`
		InterfaceDigest  string           `json:"interface_digest"`
		ReaderMinVersion int64            `json:"reader_min_version"`
		EstimatedLockMS  int64            `json:"estimated_lock_ms"`
		TransactionMode  string           `json:"transaction_mode"`
		RollbackStrategy string           `json:"rollback_strategy"`
		BackfillStrategy string           `json:"backfill_strategy"`
		Destructive      bool             `json:"destructive"`
	}
	if err := json.Unmarshal(approval, &admitted); err != nil {
		return production.MigrationRegistry{}, err
	}
	catalog, err := registry.ValidateOrder()
	if err != nil {
		return production.MigrationRegistry{}, err
	}
	m, ok := catalog[180001]
	if !ok || admitted.MigrationID != m.MigrationID || m.Service != "core/pnl-tracker" || m.Schema != "pnl" || m.OwnerLane != "L18" || m.SHA256 != admitted.SourceSHA256 || registry.ApprovalDigest != production.Checksum(approval) || m.ApprovalManifestDigest != registry.ApprovalDigest || admitted.InterfaceDigest != production.Checksum(readerInterface) || m.InterfaceDigest != admitted.InterfaceDigest || m.ReaderMinVersion != admitted.ReaderMinVersion || m.EstimatedLockMS != admitted.EstimatedLockMS || m.TransactionMode != admitted.TransactionMode || m.RollbackStrategy != admitted.RollbackStrategy || m.BackfillStrategy != admitted.BackfillStrategy || m.Destructive != admitted.Destructive {
		return production.MigrationRegistry{}, fmt.Errorf("reward migration differs from its source-bound approval")
	}
	if len(admitted.Predecessors) != 3 || admitted.Predecessors[80003] == "" || admitted.Predecessors[160000] == "" || admitted.Predecessors[180000] == "" || len(m.PredecessorIDs) != 1 || m.PredecessorIDs[0] != 180000 {
		return production.MigrationRegistry{}, fmt.Errorf("reward prerequisite approval incomplete")
	}
	for id, sum := range admitted.Predecessors {
		if catalog[id].SHA256 != sum || catalog[id].Service != m.Service || catalog[id].Schema != m.Schema {
			return production.MigrationRegistry{}, fmt.Errorf("reward predecessor %d differs from approval", id)
		}
	}
	return registry, nil
}

package evidence_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/evidence"
)

func TestRepairM_1154_RepeatedStartAndRestart(t *testing.T) {
	evidence.RepairM1154RepeatedStartAndRestart(t)
}

func TestRepairM_1154_FailedStartRetry(t *testing.T) {
	evidence.RepairM1154FailedStartRetry(t)
}

func TestRepairM_1154_ExpiredRecovery(t *testing.T) {
	evidence.RepairM1154ExpiredRecovery(t)
}

func TestRepairM_1154_ShutdownJoinsUsers(t *testing.T) {
	evidence.RepairM1154ShutdownJoinsUsers(t)
}

func TestRepairM_1154_LightClientLifecycle(t *testing.T) {
	TestLightClientAttackEvidenceLifecycle(t)
}

package evidence_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/evidence"
)

func TestPoolLifecycleRepeatedStartAndRestart(t *testing.T) {
	evidence.PoolLifecycleRepeatedStartAndRestart(t)
}

func TestPoolLifecycleFailedStartRetry(t *testing.T) {
	evidence.PoolLifecycleFailedStartRetry(t)
}

func TestPoolLifecycleExpiredRecovery(t *testing.T) {
	evidence.PoolLifecycleExpiredRecovery(t)
}

func TestPoolLifecycleShutdownJoinsUsers(t *testing.T) {
	evidence.PoolLifecycleShutdownJoinsUsers(t)
}

func TestPoolLifecycleLightClientLifecycle(t *testing.T) {
	TestLightClientAttackEvidenceLifecycle(t)
}

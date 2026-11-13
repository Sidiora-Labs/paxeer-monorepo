package data

import "testing"

func TestDataRecoveryFirstBatchPersistenceInterruption(t *testing.T) {
	testFirstBatchPersistenceInterruption(t)
}

func TestDataRecoveryUncertifiedTailReconciliation(t *testing.T) {
	TestReconcileCase7BlocksPastQCs(t)
	TestReconcileCase7BlocksTail(t)
}

func TestDataRecoveryCertificateLossStillRejected(t *testing.T) {
	TestReconcileCase2Corrupted(t)
}

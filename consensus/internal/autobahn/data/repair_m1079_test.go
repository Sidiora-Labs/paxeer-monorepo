package data

import "testing"

func TestRepairM_1079_FirstBatchPersistenceInterruption(t *testing.T) {
	testFirstBatchPersistenceInterruption(t)
}

func TestRepairM_1079_UncertifiedTailReconciliation(t *testing.T) {
	TestReconcileCase7BlocksPastQCs(t)
	TestReconcileCase7BlocksTail(t)
}

func TestRepairM_1079_CertificateLossStillRejected(t *testing.T) {
	TestReconcileCase2Corrupted(t)
}

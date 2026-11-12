package auth

import "testing"

func TestRegisteredAuditDeliveryAndReplay(t *testing.T) {
	TestPhase2AuditDurableBridgeAndRealBroker(t)
}

func TestRegisteredAuditImmutability(t *testing.T) {
	TestPhase2AuditRuntimeCannotRewriteHistory(t)
}

func TestRegisteredStoredReplayIntegrity(t *testing.T) {
	TestPhase3StoredReplayIntegrityAndCanceledTransaction(t)
}

func TestRegisteredContractRevocation(t *testing.T) {
	TestPhase2CommittedContractRevocation(t)
}

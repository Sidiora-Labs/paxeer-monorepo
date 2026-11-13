package app

import "testing"

func TestDevelopmentTransactionReads(t *testing.T) {
	t.Run("transaction status", TestDevelopmentTransactionStatusFromChainRPC)
	t.Run("receipt without logs", TestRepair_27_log_free_receipts)
}

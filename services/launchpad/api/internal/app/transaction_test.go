package app

import "testing"

func TestDevelopmentTransactionReads(t *testing.T) {
	t.Run("transaction status", TestDevelopmentTransactionStatusFromChainRPC)
	t.Run("receipt without logs", TestLogFreeReceipts)
}

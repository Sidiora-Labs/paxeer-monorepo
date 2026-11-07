package persist

import "testing"

func TestRepairM_1084_FailedWritePreservesSequenceAndBackup(t *testing.T) {
	t.Run("write_error", TestPersistWriteErrorReturnsError)
	t.Run("sequence_and_backup", TestPersistWriteErrorDoesNotAdvanceSeq)
}

func TestRepairM_1084_ReadErrorsPropagate(t *testing.T) {
	t.Run("read_errors", TestLoadPersistedOSErrorPropagates)
}

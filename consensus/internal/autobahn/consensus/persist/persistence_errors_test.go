package persist

import "testing"

func TestPersistenceErrorsFailedWritePreservesSequenceAndBackup(t *testing.T) {
	t.Run("write_error", TestPersistWriteErrorReturnsError)
	t.Run("sequence_and_backup", TestPersistWriteErrorDoesNotAdvanceSeq)
}

func TestPersistenceErrorsReadErrorsPropagate(t *testing.T) {
	t.Run("read_errors", TestLoadPersistedOSErrorPropagates)
}

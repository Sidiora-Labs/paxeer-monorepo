package keyring

import "github.com/pkg/errors"

var (
	// ErrUnsupportedSigningAlgo is raised when the caller tries to use a
	// signing scheme outside the configured supported algorithms.
	ErrUnsupportedSigningAlgo = errors.New("unsupported signing algo")

	// ErrUnsupportedLanguage is raised when the caller tries to use a
	// different language than english for creating a mnemonic sentence.
	ErrUnsupportedLanguage = errors.New("unsupported language: only english is supported")
)

// ErrCorruptKey identifies unusable persisted key information.
var ErrCorruptKey = errors.New("corrupt keyring entry")

// EntryError identifies an entry without returning invalid public information.
type EntryError struct {
	Entry string
	Err   error
}

func (e *EntryError) Error() string        { return "corrupt keyring entry " + e.Entry + ": " + e.Err.Error() }
func (e *EntryError) Unwrap() error        { return e.Err }
func (e *EntryError) Is(target error) bool { return target == ErrCorruptKey }

// CorruptionError accompanies List's healthy records, which remain recoverable.
type CorruptionError struct{ Entries []*EntryError }

func (e *CorruptionError) Error() string {
	message := "corrupt keyring entries:"
	for _, entry := range e.Entries {
		message += " " + entry.Entry
	}
	return message
}
func (e *CorruptionError) Unwrap() []error {
	causes := make([]error, len(e.Entries))
	for i, entry := range e.Entries {
		causes[i] = entry
	}
	return causes
}

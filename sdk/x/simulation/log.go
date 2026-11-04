package simulation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LogWriter records operations and returns the actual diagnostic path and any export error.
type LogWriter interface {
	AddEntry(OperationEntry)
	PrintLogs() (string, error)
}

func NewLogWriter(testingmode bool) LogWriter {
	if !testingmode {
		return &DummyLogWriter{}
	}
	return &StandardLogWriter{}
}

type StandardLogWriter struct {
	OpEntries []OperationEntry `json:"op_entries" yaml:"op_entries"`
	// Directory overrides the default user simulation directory.
	Directory string `json:"-" yaml:"-"`
}

func (lw *StandardLogWriter) AddEntry(opEntry OperationEntry) {
	lw.OpEntries = append(lw.OpEntries, opEntry)
}

func (lw *StandardLogWriter) PrintLogs() (string, error) {
	f, err := createLogFile(lw.Directory)
	if err != nil {
		return "", err
	}
	return f.Name(), writeLogEntries(f, lw.OpEntries)
}

func writeLogEntries(w io.WriteCloser, entries []OperationEntry) (err error) {
	defer func() { err = errors.Join(err, w.Close()) }()
	encoder := json.NewEncoder(w)
	for _, entry := range entries {
		if err = encoder.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}

func createLogFile(directory string) (*os.File, error) {
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		directory = filepath.Join(home, ".simapp", "simulations")
	}
	if err := os.MkdirAll(directory, 0750); err != nil {
		return nil, err
	}
	// CreateTemp uses exclusive creation; its random suffix identifies this export.
	return os.CreateTemp(directory, fmt.Sprintf("%s-*.log", time.Now().UTC().Format("2006-01-02_15-04-05")))
}

type DummyLogWriter struct{}

func (lw *DummyLogWriter) AddEntry(_ OperationEntry)  {}
func (lw *DummyLogWriter) PrintLogs() (string, error) { return "", nil }

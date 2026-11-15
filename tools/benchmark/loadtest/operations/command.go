// Package operations implements the benchmark's administrative script operations.
package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const CommandTimeout = 30 * time.Second

func closeWithError(closer io.Closer, err error) error {
	closeErr := closer.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func effectiveUID() (uint64, error) {
	uid := os.Geteuid()
	if uid < 0 || uint64(uid) > uint64(^uint32(0)) {
		return 0, errors.New("effective user ID exceeds uint32")
	}
	return uint64(uid), nil
}

type Command struct {
	Args  []string `json:"args"`
	Input string   `json:"input,omitempty"`
}

type Commands interface {
	Run(context.Context, Command) ([]byte, error)
}

type Executables struct{}

func (Executables) Run(ctx context.Context, c Command) ([]byte, error) {
	if len(c.Args) == 0 {
		return nil, errors.New("empty command")
	}
	ctx, cancel := context.WithTimeout(ctx, CommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Args[0], c.Args[1:]...)
	cmd.Stdin = os.Stdin
	if c.Input != "" {
		cmd.Stdin = strings.NewReader(c.Input)
	}
	// Keep stderr out of JSON and of errors, which may otherwise expose key material.
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", filepath.Base(c.Args[0]), err)
	}
	return out, nil
}

func Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func decodeJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}

func writeJSONAtomic(path string, data any, mode os.FileMode) (err error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".loadtest-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() {
		if file != nil {
			err = closeWithError(file, err)
		}
		removeErr := os.Remove(name)
		if err == nil && removeErr != nil && !os.IsNotExist(removeErr) {
			err = removeErr
		}
	}()
	if err = file.Chmod(mode); err != nil {
		return err
	}
	if err = json.NewEncoder(file).Encode(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	err = file.Close()
	file = nil
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	directory, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { err = closeWithError(directory, err) }()
	dir, err := directory.Open(".")
	if err != nil {
		return err
	}
	defer func() { err = closeWithError(dir, err) }()
	return dir.Sync()
}

// SignalContext is the controller lifecycle used by the executable.
func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

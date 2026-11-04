package scripts

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
)

type StateManifest struct {
	Interface     string   `json:"interface"`
	Height        int64    `json:"height"`
	InitialHeight int64    `json:"initial_height"`
	Stores        []string `json:"stores"`
	Scope         string   `json:"scope"`
}

func ExportArguments(home string, height int64) ([]string, error) {
	if height <= 0 || height == int64(^uint64(0)>>1) {
		return nil, errors.New("export height must be positive and leave room for the genesis initial height")
	}
	return []string{"export", "--home", home, "--height", strconv.FormatInt(height, 10), "--streaming=false"}, nil
}

func ExportNodeState(binary, home string, height int64, archive string, stderr io.Writer) error {
	args, err := ExportArguments(home, height)
	if err != nil {
		return err
	}
	// Export to disk rather than buffering a potentially large node state in RAM.
	stage, err := os.CreateTemp("", "pax-export-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(stage.Name())
	defer stage.Close()
	command := exec.Command(binary, args...)
	command.Stdout = stage
	command.Stderr = stderr
	exportErr := command.Run()
	if _, err = stage.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return PublishExportResult(stage, exportErr, height, archive)
}

// PublishExportResult consumes the node command's stdout and completion status.
func PublishExportResult(export io.Reader, exportErr error, height int64, archive string) error {
	if exportErr != nil {
		return fmt.Errorf("node state export failed; archive not published: %w", exportErr)
	}
	return PublishStateExport(export, height, archive)
}

func PublishStateExport(export io.Reader, height int64, archive string) error {
	if _, err := ExportArguments("", height); err != nil {
		return err
	}
	var state struct {
		ChainID       string                     `json:"chain_id"`
		InitialHeight string                     `json:"initial_height"`
		AppState      map[string]json.RawMessage `json:"app_state"`
	}
	// Retain the original complete export, including validators and consensus parameters.
	stage, err := os.CreateTemp("", "pax-export-validated-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(stage.Name())
	defer stage.Close()
	decoder := json.NewDecoder(io.TeeReader(export, stage))
	if err = decoder.Decode(&state); err != nil {
		return fmt.Errorf("invalid node export: %w", err)
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("node export must contain exactly one genesis document")
	}
	initial, err := strconv.ParseInt(state.InitialHeight, 10, 64)
	if err != nil || initial != height+1 {
		return fmt.Errorf("export initial_height %q does not match requested state height %d", state.InitialHeight, height)
	}
	if state.ChainID == "" || len(state.AppState) == 0 {
		return errors.New("incomplete node state export")
	}
	// This is a logical genesis export, never a raw/native database snapshot.
	// Refuse legacy module-only output that cannot describe the merged execution domain.
	for _, required := range []string{"auth", "bank", "evm"} {
		value, exists := state.AppState[required]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("unsupported export layout: missing %s state", required)
		}
	}
	stores := make([]string, 0, len(state.AppState))
	for name, value := range state.AppState {
		if name == "" || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("incomplete exported store %q", name)
		}
		stores = append(stores, name)
	}
	sort.Strings(stores)
	manifest, err := json.MarshalIndent(StateManifest{Interface: "paxd export", Height: height, InitialHeight: initial, Stores: stores, Scope: "logical application genesis modules; not a physical/native database snapshot"}, "", "  ")
	if err != nil {
		return err
	}
	output, err := os.CreateTemp(filepath.Dir(archive), ".pax-state-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	defer output.Close()
	writer := zip.NewWriter(output)
	entry, err := writer.Create("genesis.json")
	if err != nil {
		return err
	}
	if _, err = stage.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err = io.Copy(entry, stage); err != nil {
		return err
	}
	entry, err = writer.Create("manifest.json")
	if err != nil {
		return err
	}
	if _, err = entry.Write(manifest); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	if err = output.Sync(); err != nil {
		return err
	}
	if err = output.Close(); err != nil {
		return err
	}
	return os.Rename(output.Name(), archive)
}

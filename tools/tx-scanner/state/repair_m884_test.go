package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func repairM884Read(t *testing.T, dir string, want State) {
	t.Helper()
	got, err := ReadState(dir)
	if err != nil {
		t.Fatalf("reopen checkpoint: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened state = %#v, want %#v", got, want)
	}
}

func repairM884Backup(t *testing.T, dir string, want State) {
	t.Helper()
	// Reopen the backup through the production reader in a recovery directory.
	data, err := os.ReadFile(filepath.Join(dir, checkpointName+".previous"))
	if err != nil {
		t.Fatal(err)
	}
	recovery := t.TempDir()
	if err := os.WriteFile(filepath.Join(recovery, checkpointName), data, 0600); err != nil {
		t.Fatal(err)
	}
	repairM884Read(t, recovery, want)
}

func TestRepairM_884_RestartAtomicReplacement(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "checkpoints")
	previous := State{LastProcessedHeight: 128, BlocksMissingTxs: []int64{7, 39, 127}}
	current := State{LastProcessedHeight: 256, BlocksMissingTxs: []int64{7, 39, 127, 191}}
	if err := WriteState(dir, previous); err != nil {
		t.Fatal(err)
	}
	repairM884Read(t, dir, previous)
	filename := filepath.Join(dir, checkpointName)
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	openBeforeReplace, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer openBeforeReplace.Close()
	if err := WriteState(dir, current); err != nil {
		t.Fatal(err)
	}
	// A descriptor opened before replacement must still see the old inode.
	// Truncating and rewriting the live file fails this assertion.
	stillPrevious, err := io.ReadAll(openBeforeReplace)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stillPrevious, before) {
		t.Fatalf("live checkpoint was rewritten in place: %s", stillPrevious)
	}
	if err := openBeforeReplace.Close(); err != nil {
		t.Fatal(err)
	}
	repairM884Read(t, dir, current)
	repairM884Backup(t, dir, previous)
	next := State{LastProcessedHeight: 512, BlocksMissingTxs: []int64{7, 39, 127, 191, 300}}
	if err := WriteState(dir, next); err != nil {
		t.Fatal(err)
	}
	repairM884Read(t, dir, next)
	repairM884Backup(t, dir, current)
	matches, err := filepath.Glob(filepath.Join(dir, ".tx-scanner-state-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary checkpoints after success: %v, %v", matches, err)
	}
}

func TestRepairM_884_InterruptedCheckpointRestart(t *testing.T) {
	dir := t.TempDir()
	previous := State{LastProcessedHeight: 70, BlocksMissingTxs: []int64{9, 55}}
	current := State{LastProcessedHeight: 90, BlocksMissingTxs: []int64{9, 55, 81}}
	for _, s := range []State{previous, current} {
		if err := WriteState(dir, s); err != nil {
			t.Fatal(err)
		}
	}
	// Leave the same disk residue as an interrupted temporary-file write.
	pending, err := os.CreateTemp(dir, ".tx-scanner-state-*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pending.Write(data[:len(data)/2]); err != nil {
		t.Fatal(err)
	}
	if err := pending.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := pending.Close(); err != nil {
		t.Fatal(err)
	}
	repairM884Read(t, dir, current)
	repairM884Backup(t, dir, previous)
	// Corruption after restart must be explicit, leaving operator recovery intact.
	filename := filepath.Join(dir, checkpointName)
	if err := os.WriteFile(filename, data[:len(data)/2], 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadState(dir); !errors.Is(err, ErrRecoveryRequired) || errors.Is(err, os.ErrNotExist) || !reflect.DeepEqual(got, State{}) {
		t.Fatalf("corrupt restart returned %#v, %v", got, err)
	}
	if err := WriteState(dir, State{LastProcessedHeight: 999}); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("write over corrupt state = %v", err)
	}
	repairM884Backup(t, dir, previous)
	if err := os.Rename(filename+".previous", filename); err != nil {
		t.Fatal(err)
	}
	repairM884Read(t, dir, previous)
	if err := WriteState(dir, current); err != nil {
		t.Fatal(err)
	}
	repairM884Read(t, dir, current)
	repairM884Backup(t, dir, previous)
}

func TestRepairM_884_PublicationFailurePreservesCheckpoint(t *testing.T) {
	dir := t.TempDir()
	previous := State{LastProcessedHeight: 120, BlocksMissingTxs: []int64{10, 100}}
	if err := WriteState(dir, previous); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(dir, checkpointName)
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	// A real filesystem refusal at backup publication must abort the update.
	if err := os.Mkdir(filename+".previous", 0700); err != nil {
		t.Fatal(err)
	}
	current := State{LastProcessedHeight: 240, BlocksMissingTxs: []int64{10, 100, 220}}
	if err := WriteState(dir, current); err == nil {
		t.Fatal("checkpoint update succeeded despite backup publication failure")
	}
	after, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("failed update changed current checkpoint: %s, %v", after, err)
	}
	repairM884Read(t, dir, previous)
	matches, err := filepath.Glob(filepath.Join(dir, ".tx-scanner-state-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary checkpoints after failure: %v, %v", matches, err)
	}
	if err := os.Remove(filename + ".previous"); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(dir, current); err != nil {
		t.Fatal(err)
	}
	repairM884Read(t, dir, current)
	repairM884Backup(t, dir, previous)
}

func TestRepairM_884_AbsentVersusLostCheckpoint(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new")
	if _, err := ReadState(dir); !errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("first start = %v", err)
	}
	previous := State{LastProcessedHeight: 40, BlocksMissingTxs: []int64{11, 31}}
	current := State{LastProcessedHeight: 80, BlocksMissingTxs: []int64{11, 31, 51}}
	for _, s := range []State{previous, current} {
		if err := WriteState(dir, s); err != nil {
			t.Fatal(err)
		}
	}
	filename := filepath.Join(dir, checkpointName)
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadState(dir); !errors.Is(err, ErrRecoveryRequired) || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lost primary misclassified as first start: %v", err)
	}
	if err := WriteState(dir, State{}); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("lost checkpoint overwritten: %v", err)
	}
	if _, err := os.Stat(filename); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed write published a fresh checkpoint: %v", err)
	}
	repairM884Backup(t, dir, previous)
	if err := os.Rename(filename+".previous", filename); err != nil {
		t.Fatal(err)
	}
	repairM884Read(t, dir, previous)
}

func TestRepairM_884_CorruptCheckpointRefused(t *testing.T) {
	for _, payload := range []string{"", "{", "null", "{}", `{"last_processed_height":42}`, `{"blocks_missing_txs":[4]}`, `{"last_processed_height":null,"blocks_missing_txs":[4]}`, `{"last_processed_height":42,"blocks_missing_txs":"lost"}`, `{"last_processed_height":42,"blocks_missing_txs":[4]} trailing`} {
		t.Run(payload, func(t *testing.T) {
			dir := t.TempDir()
			filename := filepath.Join(dir, checkpointName)
			if err := os.WriteFile(filename, []byte(payload), 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := ReadState(dir); !errors.Is(err, ErrRecoveryRequired) || errors.Is(err, os.ErrNotExist) || !reflect.DeepEqual(got, State{}) {
				t.Fatalf("corrupt restart returned %#v, %v", got, err)
			}
			if err := WriteState(dir, State{LastProcessedHeight: 100}); !errors.Is(err, ErrRecoveryRequired) {
				t.Fatalf("corrupt checkpoint replaced: %v", err)
			}
			got, err := os.ReadFile(filename)
			if err != nil || string(got) != payload {
				t.Fatalf("corrupt evidence changed: %q, %v", got, err)
			}
		})
	}
	for _, s := range []State{{}, {LastProcessedHeight: 1, BlocksMissingTxs: []int64{}}} {
		dir := t.TempDir()
		if err := WriteState(dir, s); err != nil {
			t.Fatal(err)
		}
		repairM884Read(t, dir, s)
	}
}

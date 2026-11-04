package persist

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
)

func TestPersisterAlternates(t *testing.T) {
	dir := t.TempDir()

	w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)

	// Both files should be pre-created (empty) by NewPersister for dir-sync optimization.
	_, errA := os.Stat(filepath.Join(dir, "test"+suffixA))
	_, errB := os.Stat(filepath.Join(dir, "test"+suffixB))
	require.NoError(t, errA, "A should be pre-created")
	require.NoError(t, errB, "B should be pre-created")

	// First write: goes to A (seq=1)
	require.NoError(t, w.Persist(wrapperspb.String("data1")))

	// Second write: goes to B (seq=2)
	require.NoError(t, w.Persist(wrapperspb.String("data2")))

	// Third write: goes to A (seq=3, overwrite)
	require.NoError(t, w.Persist(wrapperspb.String("data3")))

	// loadPersisted should return data3 (highest seq)
	ds, err := loadPersisted(dir, "test")
	require.NoError(t, err)
	var inner wrapperspb.StringValue
	require.NoError(t, proto.Unmarshal(ds.data, &inner))
	require.Equal(t, "data3", inner.GetValue())
}

func TestPersisterPicksHigherSeq(t *testing.T) {
	dir := t.TempDir()

	w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)

	// Write three times: A(seq=1), B(seq=2), A(seq=3)
	require.NoError(t, w.Persist(wrapperspb.String("first")))
	require.NoError(t, w.Persist(wrapperspb.String("second")))
	require.NoError(t, w.Persist(wrapperspb.String("third")))

	// Should load "third" (seq=3 > seq=2)
	w2, loaded, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	msg, ok := loaded.Get()
	require.True(t, ok)
	require.Equal(t, "third", msg.GetValue())
	_ = w2
}

func TestLoadPersistedOneCorruptFileSucceeds(t *testing.T) {
	dir := t.TempDir()

	// Write to both files: A(seq=1), B(seq=2)
	w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w.Persist(wrapperspb.String("first")))  // seq=1, A
	require.NoError(t, w.Persist(wrapperspb.String("second"))) // seq=2, B

	// Corrupt B (the winner) — should fall back to A
	err = os.WriteFile(filepath.Join(dir, "test"+suffixB), []byte("corrupt"), 0600)
	require.NoError(t, err)

	_, loaded, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	msg, ok := loaded.Get()
	require.True(t, ok)
	require.Equal(t, "first", msg.GetValue())
}

func TestLoadPersistedBothCorruptError(t *testing.T) {
	dir := t.TempDir()

	// Write valid data to A only
	w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w.Persist(wrapperspb.String("valid"))) // seq=1, A

	// Corrupt A (B is empty, treated as non-existent) — both files fail
	err = os.WriteFile(filepath.Join(dir, "test"+suffixA), []byte("corrupt"), 0600)
	require.NoError(t, err)

	_, _, err = NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.Error(t, err)
}

func TestNewPersisterOneCorruptFileSucceeds(t *testing.T) {
	dir := t.TempDir()

	// Write to both files: A(seq=1), B(seq=2)
	w1, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w1.Persist(wrapperspb.String("first")))  // seq=1, A
	require.NoError(t, w1.Persist(wrapperspb.String("second"))) // seq=2, B

	// Corrupt B (the winner) — NewPersister should still succeed using A
	err = os.WriteFile(filepath.Join(dir, "test"+suffixB), []byte("corrupt"), 0600)
	require.NoError(t, err)

	w2, loaded, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	msg, ok := loaded.Get()
	require.True(t, ok)
	require.Equal(t, "first", msg.GetValue())

	// A won (seq=1), so next write goes to B (the corrupt/loser slot)
	require.NoError(t, w2.Persist(wrapperspb.String("recovered"))) // seq=2, B

	_, loaded2, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	msg2, ok := loaded2.Get()
	require.True(t, ok)
	require.Equal(t, "recovered", msg2.GetValue())
}

func TestLoadPersistedEmptyDir(t *testing.T) {
	dir := t.TempDir()

	// No files exist
	_, err := loadPersisted(dir, "test")
	require.True(t, errors.Is(err, ErrNoData), "should return ErrNoData when no files exist")
}

func TestNewPersisterInvalidDirError(t *testing.T) {
	// State dir must already exist; invalid (nonexistent or not a directory) returns error
	_, _, err := NewPersister[*wrapperspb.StringValue](utils.Some("/nonexistent/path/that/does/not/exist"), "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid state dir")
}

func TestPersistWriteErrorReturnsError(t *testing.T) {
	// When the directory is replaced with a regular file,
	// Persist returns error from OpenFile.
	dir := filepath.Join(t.TempDir(), "state")
	require.NoError(t, os.Mkdir(dir, 0700))
	w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w.Persist(wrapperspb.String("data1")))
	require.NoError(t, os.Rename(dir, dir+"-backup"))
	require.NoError(t, os.WriteFile(dir, nil, 0600))
	err = w.Persist(wrapperspb.String("data2"))
	require.Error(t, err)
	var pathErr *os.PathError
	require.True(t, errors.As(err, &pathErr))
}

func TestPersistWriteErrorDoesNotAdvanceSeq(t *testing.T) {
	// When Persist fails, seq must not advance.
	// Otherwise the next successful write would target the same file as the
	// last good write, destroying the only valid backup.
	for _, tc := range []struct {
		name         string
		writes       uint64
		backupSuffix string
		targetSuffix string
	}{
		{"write_B", 1, suffixA, suffixB},
		{"write_A", 2, suffixB, suffixA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			require.NoError(t, os.Mkdir(dir, 0700))
			w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
			require.NoError(t, err)
			for range tc.writes {
				require.NoError(t, w.Persist(wrapperspb.String("good")))
			}
			backupName := "test" + tc.backupSuffix
			targetName := "test" + tc.targetSuffix
			backupBytes, err := os.ReadFile(filepath.Join(dir, backupName))
			require.NoError(t, err)
			targetBytes, err := os.ReadFile(filepath.Join(dir, targetName))
			require.NoError(t, err)

			// A regular file in place of the state directory fails even as root.
			savedDir := dir + "-backup"
			require.NoError(t, os.Rename(dir, savedDir))
			require.NoError(t, os.WriteFile(dir, nil, 0600))
			err = w.Persist(wrapperspb.String("fail"))
			require.Error(t, err)
			var pathErr *os.PathError
			require.True(t, errors.As(err, &pathErr))
			require.Equal(t, filepath.Join(dir, targetName), pathErr.Path)
			require.Equal(t, tc.writes, w.(*abPersister[*wrapperspb.StringValue]).seq)
			for name, want := range map[string][]byte{backupName: backupBytes, targetName: targetBytes} {
				got, err := os.ReadFile(filepath.Join(savedDir, name))
				require.NoError(t, err)
				require.Equal(t, want, got, "failed write must leave both snapshots intact")
			}
			require.NoError(t, os.Remove(dir))
			require.NoError(t, os.Rename(savedDir, dir))

			// Reopening after the failure must still recover the last good state.
			_, loaded, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
			require.NoError(t, err)
			msg, ok := loaded.Get()
			require.True(t, ok)
			require.Equal(t, "good", msg.GetValue())

			// Retry the original writer: the loser slot gets the next sequence,
			// while the valid backup remains byte-for-byte unchanged.
			require.NoError(t, w.Persist(wrapperspb.String("recovered")))
			written, err := loadFile(dir, targetName)
			require.NoError(t, err)
			require.Equal(t, tc.writes+1, written.seq)
			var decoded wrapperspb.StringValue
			require.NoError(t, proto.Unmarshal(written.data, &decoded))
			require.Equal(t, "recovered", decoded.GetValue())
			gotBackup, err := os.ReadFile(filepath.Join(dir, backupName))
			require.NoError(t, err)
			require.Equal(t, backupBytes, gotBackup)
			backup, err := loadFile(dir, backupName)
			require.NoError(t, err)
			require.Equal(t, tc.writes, backup.seq)

			_, loaded, err = NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
			require.NoError(t, err)
			msg, ok = loaded.Get()
			require.True(t, ok)
			require.Equal(t, "recovered", msg.GetValue())
		})
	}
}

func TestLoadPersistedOSErrorPropagates(t *testing.T) {
	// When one A/B file has an OS-level error (not corrupt data),
	// loadPersisted should propagate the error instead of silently
	// falling back to the other file.
	for _, suffix := range []string{suffixA, suffixB} {
		t.Run(suffix, func(t *testing.T) {
			dir := t.TempDir()

			// Write to both files: A(seq=1), B(seq=2).
			w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
			require.NoError(t, err)
			require.NoError(t, w.Persist(wrapperspb.String("first")))
			require.NoError(t, w.Persist(wrapperspb.String("second")))

			// Reading a directory as a snapshot is an OS error even as root.
			// Test both the older slot and the winner: neither may be ignored.
			filename := "test" + suffix
			path := filepath.Join(dir, filename)
			savedPath := path + "-backup"
			require.NoError(t, os.Rename(path, savedPath))
			require.NoError(t, os.Mkdir(path, 0700))
			_, readErr := os.ReadFile(path)
			require.Error(t, readErr)
			var readPathErr *os.PathError
			require.True(t, errors.As(readErr, &readPathErr))

			assertReadError := func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				require.False(t, errors.Is(err, ErrCorrupt), "OS error should not be wrapped as ErrCorrupt")
				require.False(t, errors.Is(err, ErrNoData), "should not be ErrNoData")
				require.False(t, errors.Is(err, os.ErrNotExist), "the conflicting path exists")
				var pathErr *os.PathError
				require.True(t, errors.As(err, &pathErr))
				require.Equal(t, path, pathErr.Path)
				require.ErrorIs(t, err, readPathErr.Err)
			}
			_, err = loadFile(dir, filename)
			assertReadError(t, err)
			_, err = loadPersisted(dir, "test")
			assertReadError(t, err)
			failedWriter, loaded, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
			assertReadError(t, err)
			require.Nil(t, failedWriter)
			require.False(t, loaded.IsPresent())

			// Removing only the obstruction restores the real persisted state.
			require.NoError(t, os.Remove(path))
			require.NoError(t, os.Rename(savedPath, path))
			_, loaded, err = NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
			require.NoError(t, err)
			msg, ok := loaded.Get()
			require.True(t, ok)
			require.Equal(t, "second", msg.GetValue())
		})
	}
}

func TestNewPersisterResumeSeq(t *testing.T) {
	dir := t.TempDir()

	// Create persister and write some data
	w1, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w1.Persist(wrapperspb.String("data1"))) // seq=1, A
	require.NoError(t, w1.Persist(wrapperspb.String("data2"))) // seq=2, B
	require.NoError(t, w1.Persist(wrapperspb.String("data3"))) // seq=3, A (winner)

	// Create new persister (simulates restart)
	// A has seq=3 (winner), so new persister should write to B first to preserve A
	w2, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w2.Persist(wrapperspb.String("data4"))) // seq=4, B (preserves A)

	// Verify data4 is the latest (seq=4)
	_, loaded, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	msg, ok := loaded.Get()
	require.True(t, ok)
	require.Equal(t, "data4", msg.GetValue())
}

func TestNewPersisterPreservesWinner(t *testing.T) {
	dir := t.TempDir()

	// Write to both files: A=seq1, B=seq2 (B wins)
	w1, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w1.Persist(wrapperspb.String("old")))    // seq=1, A
	require.NoError(t, w1.Persist(wrapperspb.String("winner"))) // seq=2, B (winner)

	// New persister should write to A first (preserve B)
	w2, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w2.Persist(wrapperspb.String("new"))) // seq=3, A (preserves B)

	// Verify "new" is the latest (seq=3)
	_, loaded, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	msg, ok := loaded.Get()
	require.True(t, ok)
	require.Equal(t, "new", msg.GetValue())
}

// --- CRC32 and file header validation tests ---

func TestLoadFileDataCorruption(t *testing.T) {
	dir := t.TempDir()

	w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w.Persist(wrapperspb.String("data"))) // seq=1, A

	// Flip a byte in the proto payload; CRC should catch it.
	path := filepath.Join(dir, "test"+suffixA)
	bz, err := os.ReadFile(path)
	require.NoError(t, err)
	bz[len(bz)-1] ^= 0xFF
	require.NoError(t, os.WriteFile(path, bz, 0600))

	_, err = loadFile(dir, "test"+suffixA)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCorrupt))
	require.Contains(t, err.Error(), "crc32 mismatch")
}

func TestLoadFileTruncatedHeader(t *testing.T) {
	dir := t.TempDir()

	// A file shorter than the header is rejected.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test"+suffixA), []byte("short"), 0600))

	_, err := loadFile(dir, "test"+suffixA)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCorrupt))
	require.Contains(t, err.Error(), "truncated")
}

func TestLoadFileZeroSeq(t *testing.T) {
	dir := t.TempDir()

	// Build a valid-CRC file with seq=0.
	payload := make([]byte, seqSize)
	binary.LittleEndian.PutUint64(payload, 0)
	crc := crc32.Checksum(payload, crc32c)
	buf := make([]byte, crcSize+len(payload))
	binary.BigEndian.PutUint32(buf[:crcSize], crc)
	copy(buf[crcSize:], payload)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test"+suffixA), buf, 0600))

	_, err := loadFile(dir, "test"+suffixA)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCorrupt))
	require.Contains(t, err.Error(), "zero seq")
}

func TestLoadFileSeqCorruption(t *testing.T) {
	dir := t.TempDir()

	w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w.Persist(wrapperspb.String("data")))

	// Flip a bit in the seq portion of the file; CRC should catch it.
	path := filepath.Join(dir, "test"+suffixA)
	bz, err := os.ReadFile(path)
	require.NoError(t, err)
	bz[crcSize] ^= 0x01 // first byte of seq
	require.NoError(t, os.WriteFile(path, bz, 0600))

	_, err = loadFile(dir, "test"+suffixA)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCorrupt))
	require.Contains(t, err.Error(), "crc32 mismatch")
}

func TestPersistFileFormat(t *testing.T) {
	dir := t.TempDir()
	w, _, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.NoError(t, w.Persist(wrapperspb.String("hello")))

	bz, err := os.ReadFile(filepath.Join(dir, "test"+suffixA))
	require.NoError(t, err)
	require.True(t, len(bz) >= headerSize)

	// Verify CRC covers the payload (seq + proto data).
	wantCRC := crc32.Checksum(bz[crcSize:], crc32c)
	gotCRC := binary.BigEndian.Uint32(bz[:crcSize])
	require.Equal(t, wantCRC, gotCRC)

	// Verify seq.
	seq := binary.LittleEndian.Uint64(bz[crcSize : crcSize+seqSize])
	require.Equal(t, uint64(1), seq)
}

// TestNewPersisterNoneDirIsNoOp pins the behaviour that makes an absent state
// dir unsafe for a validator: NewPersister accepts None, every Persist call
// reports success, nothing reaches the disk, and a restart loads no state at
// all. Config validation refuses that combination outside an explicit
// test-only opt-out; this test is the reason it must.
func TestNewPersisterNoneDirIsNoOp(t *testing.T) {
	dir := t.TempDir()

	w, loaded, err := NewPersister[*wrapperspb.StringValue](utils.None[string](), "test")
	require.NoError(t, err)
	require.False(t, loaded.IsPresent(), "no-op persister must load nothing")

	require.NoError(t, w.Persist(wrapperspb.String("safety-state")))
	require.NoError(t, w.Persist(wrapperspb.String("safety-state-2")))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "no-op persister must not write any file")

	// A restart through the same code path recovers nothing, so the consensus
	// state would come back with no memory of the votes it already cast.
	_, reloaded, err := NewPersister[*wrapperspb.StringValue](utils.None[string](), "test")
	require.NoError(t, err)
	require.False(t, reloaded.IsPresent())
}

// TestNewPersisterSomeDirSurvivesRestart is the contrasting case: with a state
// dir the persisted message is recovered by the next NewPersister call, which
// is the property the config-level requirement protects.
func TestNewPersisterSomeDirSurvivesRestart(t *testing.T) {
	dir := t.TempDir()

	w, loaded, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	require.False(t, loaded.IsPresent())
	require.NoError(t, w.Persist(wrapperspb.String("safety-state")))

	_, reloaded, err := NewPersister[*wrapperspb.StringValue](utils.Some(dir), "test")
	require.NoError(t, err)
	msg, ok := reloaded.Get()
	require.True(t, ok)
	require.Equal(t, "safety-state", msg.GetValue())
}

package wal

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/wal"
)

func TestRepairM_1073_BinaryRestartRecovery(t *testing.T) {
	repairM1073RestartRecovery(t, wal.Binary)
}

func TestRepairM_1073_JSONRestartRecovery(t *testing.T) {
	repairM1073RestartRecovery(t, wal.JSON)
}

func repairM1073RestartRecovery(t *testing.T, format wal.LogFormat) {
	t.Helper()
	for _, tc := range []struct {
		name      string
		entries   int
		tailDelta int
		healthy   bool
	}{
		{name: "equal_prefix_and_suffix", entries: 1},
		{name: "shorter_suffix", entries: 1, tailDelta: -1},
		{name: "longer_suffix", entries: 1, tailDelta: 1},
		{name: "multiple_entries_equal_suffix", entries: 3},
		{name: "healthy", entries: 3, healthy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			opts := &wal.Options{LogFormat: format}
			producer, err := wal.Open(dir, opts)
			require.NoError(t, err)
			producerClosed := false
			t.Cleanup(func() {
				if !producerClosed {
					require.NoError(t, producer.Close())
				}
			})
			entries := [][]byte{[]byte("a"), []byte("second entry"), {0x00, 0xff, '\n'}}[:tc.entries]
			for i, entry := range entries {
				require.NoError(t, producer.Write(uint64(i+1), entry))
			}
			require.NoError(t, producer.Close())
			producerClosed = true

			files, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, files, 1)
			segmentPath := filepath.Join(dir, files[0].Name())
			prefix, err := os.ReadFile(segmentPath)
			require.NoError(t, err)
			require.NotEmpty(t, prefix)
			if format == wal.Binary && tc.entries == 1 {
				require.Equal(t, []byte{0x01, 'a'}, prefix, "produce the audit's exact binary prefix")
			}
			if !tc.healthy {
				corruptByte := byte(0x80) // An unterminated binary length varint.
				if format == wal.JSON {
					corruptByte = '{' // An incomplete JSON record without a newline.
				}
				suffix := bytes.Repeat([]byte{corruptByte}, len(prefix)+tc.tailDelta)
				require.NotEmpty(t, suffix)
				if tc.tailDelta == 0 {
					require.Len(t, suffix, len(prefix))
				}
				file, err := os.OpenFile(segmentPath, os.O_WRONLY|os.O_APPEND, 0)
				require.NoError(t, err)
				n, writeErr := file.Write(suffix)
				closeErr := file.Close()
				require.NoError(t, writeErr)
				require.Equal(t, len(suffix), n)
				require.NoError(t, closeErr)

				broken, err := wal.Open(dir, opts)
				if broken != nil {
					t.Cleanup(func() { require.NoError(t, broken.Close()) })
				}
				require.ErrorIs(t, err, wal.ErrCorrupt, "the real reader must detect the injected torn tail")
			}

			recovered, err := open(dir, opts)
			require.NoError(t, err, "restart must recover even when valid and corrupt lengths are equal")
			recoveredClosed := false
			t.Cleanup(func() {
				if !recoveredClosed {
					require.NoError(t, recovered.Close())
				}
			})
			retained, err := os.ReadFile(segmentPath)
			require.NoError(t, err)
			require.Equal(t, prefix, retained, "retain exactly the producer's validated bytes")
			first, err := recovered.FirstIndex()
			require.NoError(t, err)
			require.Equal(t, uint64(1), first)
			last, err := recovered.LastIndex()
			require.NoError(t, err)
			require.Equal(t, uint64(len(entries)), last)
			for i, want := range entries {
				got, err := recovered.Read(uint64(i + 1))
				require.NoError(t, err)
				require.Equal(t, want, got)
			}
			_, err = recovered.Read(last + 1)
			require.ErrorIs(t, err, wal.ErrNotFound)
			next := []byte("written after recovery")
			require.NoError(t, recovered.Write(last+1, next))
			require.NoError(t, recovered.Close())
			recoveredClosed = true

			restarted, err := open(dir, opts)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, restarted.Close()) })
			last, err = restarted.LastIndex()
			require.NoError(t, err)
			require.Equal(t, uint64(len(entries)+1), last)
			for i, want := range append(entries, next) {
				got, err := restarted.Read(uint64(i + 1))
				require.NoError(t, err)
				require.Equal(t, want, got, "recovery and subsequent writes must survive another restart")
			}
		})
	}
}

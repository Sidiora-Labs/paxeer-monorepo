package wal

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func repairM1158DescriptorCount(t *testing.T, dir string) int {
	t.Helper()
	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", fd.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue // ReadDir's own descriptor has already closed.
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(target, dir+string(os.PathSeparator)) {
			count++
		}
	}
	return count
}

func repairM1158AssertDescriptors(t *testing.T, dir string, want int) {
	t.Helper()
	if got := repairM1158DescriptorCount(t, dir); got != want {
		t.Fatalf("WAL descriptors: got %d, want %d", got, want)
	}
}

func TestRepairM_1158_RepeatedSegmentSearchEOF(t *testing.T) {
	previousGC := debug.SetGCPercent(-1)
	t.Cleanup(func() { debug.SetGCPercent(previousGC) })
	dir := t.TempDir()
	headPath := filepath.Join(dir, "wal")
	cfg := &Config{FileSizeLimit: 1}
	l, err := OpenLog(headPath, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { l.Close() }()
	repairM1158AssertDescriptors(t, dir, 2)
	for range 64 {
		entries, err := l.ReadFile(0)
		if err != nil || len(entries) != 0 {
			t.Fatalf("empty head: entries=%q, err=%v", entries, err)
		}
		repairM1158AssertDescriptors(t, dir, 2)
	}
	want := [][]byte{[]byte("first"), {}, []byte("third"), []byte("last")}
	for _, entry := range want {
		if err := l.Append(entry); err != nil {
			t.Fatal(err)
		}
	}
	for phase := range 2 {
		if got := l.MinOffset(); got != -3 {
			t.Fatalf("phase %d: minimum offset %d, want -3", phase, got)
		}
		for search := range 64 {
			for offset := 0; offset >= l.MinOffset(); offset-- {
				entries, err := l.ReadFile(offset)
				if err != nil {
					t.Fatalf("phase %d search %d offset %d: %v", phase, search, offset, err)
				}
				if len(entries) != 1 || !bytes.Equal(entries[0], want[offset+3]) {
					t.Fatalf("offset %d: got %q, want %q", offset, entries, want[offset+3])
				}
				repairM1158AssertDescriptors(t, dir, 2)
			}
		}
		l.Close()
		repairM1158AssertDescriptors(t, dir, 0)
		if phase == 0 {
			reopened, err := OpenLog(headPath, cfg)
			if err != nil {
				t.Fatal(err)
			}
			l = reopened
			repairM1158AssertDescriptors(t, dir, 2)
		}
	}
}

func TestRepairM_1158_RepeatedSegmentSearchCorruption(t *testing.T) {
	previousGC := debug.SetGCPercent(-1)
	t.Cleanup(func() { debug.SetGCPercent(previousGC) })
	for _, segment := range []string{"head", "tail"} {
		for _, damage := range []string{"checksum", "short-header", "short-payload"} {
			t.Run(segment+"/"+damage, func(t *testing.T) {
				dir := t.TempDir()
				headPath := filepath.Join(dir, "wal")
				entry := []byte("payload")
				entrySize := headerSize + int64(len(entry))
				l, err := OpenLog(headPath, &Config{FileSizeLimit: 2 * entrySize})
				if err != nil {
					t.Fatal(err)
				}
				defer l.Close()
				for range 2 {
					if err := l.Append(entry); err != nil {
						t.Fatal(err)
					}
				}
				segmentPath, offset := headPath, 0
				if segment == "tail" {
					if err := l.Append([]byte("healthy head")); err != nil {
						t.Fatal(err)
					}
					segmentPath, offset = headPath+".000", -1
				}
				if err := l.Sync(); err != nil {
					t.Fatal(err)
				}
				encoded, err := os.ReadFile(segmentPath)
				if err != nil {
					t.Fatal(err)
				}
				if int64(len(encoded)) != 2*entrySize {
					t.Fatalf("segment size %d, want %d", len(encoded), 2*entrySize)
				}
				switch damage {
				case "checksum":
					encoded[entrySize] ^= 0xff
				case "short-header":
					encoded = encoded[:entrySize+headerSize-1]
				case "short-payload":
					encoded = encoded[:len(encoded)-1]
				}
				if err := os.WriteFile(segmentPath, encoded, filePerms); err != nil {
					t.Fatal(err)
				}
				repairM1158AssertDescriptors(t, dir, 2)
				for search := range 64 {
					entries, err := l.ReadFile(offset)
					if !errors.Is(err, errCorrupted) {
						t.Fatalf("search %d: got %v, want corruption", search, err)
					}
					if entries != nil {
						t.Fatalf("search %d: returned partial entries %q", search, entries)
					}
					repairM1158AssertDescriptors(t, dir, 2)
					if segment == "tail" {
						headEntries, err := l.ReadFile(0)
						if err != nil || len(headEntries) != 1 || !bytes.Equal(headEntries[0], []byte("healthy head")) {
							t.Fatalf("healthy head: entries=%q, err=%v", headEntries, err)
						}
						repairM1158AssertDescriptors(t, dir, 2)
					}
				}
				l.Close()
				repairM1158AssertDescriptors(t, dir, 0)
			})
		}
	}
}

package archive

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var repairM176Passphrase = []byte("correct horse battery staple ceremony")

func repairM176Fixture(t *testing.T, id string) ([]json.RawMessage, []byte) {
	t.Helper()
	row, err := json.Marshal(map[string]string{"id": id, "encrypted_private_key": "private envelope " + id})
	if err != nil {
		t.Fatal(err)
	}
	rows := []json.RawMessage{row}
	plain, err := json.Marshal(payload{Table: "wallets", Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := Seal(plain, repairM176Passphrase)
	zero(plain)
	if err != nil {
		t.Fatal(err)
	}
	return rows, sealed
}

func repairM176Verify(rows []json.RawMessage) func(string) error {
	return func(path string) error {
		got, err := Open(path, repairM176Passphrase)
		if err != nil {
			return err
		}
		return sameRows(rows, got)
	}
}

func repairM176Absent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("path %s should be absent: %v", path, err)
	}
}

// Closing every descriptor and dropping the writer models process exit without
// running its deferred cleanup. The next writer must reconstruct state from disk.
func repairM176Stop(t *testing.T, w *archiveWrite) {
	t.Helper()
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.dir.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_176_KDFCost(t *testing.T) {
	rows, sealed := repairM176Fixture(t, "kdf")
	path := filepath.Join(t.TempDir(), "funded.archive")
	if err := writeExclusive(path, sealed, repairM176Verify(rows)); err != nil {
		t.Fatal(err)
	}
	if err := repairM176Verify(rows)(path); err != nil {
		t.Fatalf("writer profile must remain readable: %v", err)
	}
	cases := []struct{ logN, r, p byte }{
		{22, 32, 16},                        // formerly accepted: 16 GiB plus excessive work
		{22, 1, 1}, {16, 8, 1}, {15, 32, 1}, // memory budget
		{14, 8, 16}, {15, 8, 2}, {14, 16, 2}, // work budget
		{0, 8, 1}, {13, 8, 1}, {23, 8, 1}, {255, 255, 255},
		{15, 0, 1}, {15, 33, 1}, {15, 8, 0}, {15, 8, 17},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d_%d_%d", tc.logN, tc.r, tc.p), func(t *testing.T) {
			bad := bytes.Clone(sealed)
			bad[len(magic)], bad[len(magic)+1], bad[len(magic)+2] = tc.logN, tc.r, tc.p
			if err := os.WriteFile(path, bad, 0o600); err != nil {
				t.Fatal(err)
			}
			// ErrFormat, rather than a failed authentication, proves refusal
			// before expensive scrypt derivation of the unauthenticated header.
			if _, err := Open(path, repairM176Passphrase); !errors.Is(err, ErrFormat) {
				t.Fatalf("untrusted KDF costs were not refused: %v", err)
			}
		})
	}
	// Exhaustively check the formerly accepted header space against independent
	// memory/work arithmetic, including combinations where only CPU is excessive.
	for logN := byte(14); logN <= 22; logN++ {
		for r := byte(1); r <= 32; r++ {
			for p := byte(1); p <= 16; p++ {
				n := uint64(1) << logN
				memory := 128 * uint64(r) * (n + uint64(p) + 2)
				work := n * uint64(r) * uint64(p)
				want := memory <= 33557504 && work <= 262144
				if got := supportedScryptCost(logN, r, p); got != want {
					t.Fatalf("profile %d/%d/%d: supported=%v, want %v", logN, r, p, got, want)
				}
			}
		}
	}
	// The previously readable cheaper profile still reaches authentication.
	cheaper := bytes.Clone(sealed)
	cheaper[len(magic)] = 14
	if _, err := Unseal(cheaper, repairM176Passphrase); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("cheaper profile was not admitted to authentication: %v", err)
	}
	// Emit a correctly authenticated cheaper-profile fixture with the production
	// cipher, then read it through the production archive decoder.
	plain, err := Unseal(sealed, repairM176Passphrase)
	if err != nil {
		t.Fatal(err)
	}
	defer zero(plain)
	header := bytes.Clone(cheaper[:headerLen])
	gcm, err := newGCM(repairM176Passphrase, header[len(magic)+3:len(magic)+3+saltLen], 14, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	legacy := gcm.Seal(header, header[headerLen-nonceLen:], plain, header)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := repairM176Verify(rows)(path); err != nil {
		t.Fatalf("supported cheaper archive no longer opens: %v", err)
	}
	if _, err := Unseal(sealed, []byte("a different sufficiently long password")); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("wrong passphrase: %v", err)
	}
}

func TestRepairM_176_StaleRowsRegenerated(t *testing.T) {
	_, stale := repairM176Fixture(t, "stale database rows")
	currentRows, current := repairM176Fixture(t, "current database rows")
	path := filepath.Join(t.TempDir(), "funded.archive")
	w, err := beginArchiveWrite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.stage(stale); err != nil {
		t.Fatal(err)
	}
	repairM176Stop(t, w)
	w = nil
	if err := writeExclusive(path, current, repairM176Verify(currentRows)); err != nil {
		t.Fatalf("restart reused stale rows instead of regenerating: %v", err)
	}
	if err := repairM176Verify(currentRows)(path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, current) {
		t.Fatalf("restart did not publish current ciphertext: %v", err)
	}
}

func TestRepairM_176_RestartRecovery(t *testing.T) {
	rows, sealed := repairM176Fixture(t, "recovery")
	for _, boundary := range []string{"created", "partial", "written", "synced", "verified", "published"} {
		t.Run(boundary, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "funded.archive")
			w, err := beginArchiveWrite(path)
			if err != nil {
				t.Fatal(err)
			}
			info, err := w.file.Stat()
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("staging file must be private: %v, %v", info, err)
			}
			switch boundary {
			case "partial":
				if _, err := w.file.Write(sealed[:headerLen]); err != nil {
					t.Fatal(err)
				}
			case "written":
				if _, err := w.file.Write(sealed); err != nil {
					t.Fatal(err)
				}
			case "synced", "verified", "published":
				if err := w.stage(sealed); err != nil {
					t.Fatal(err)
				}
			}
			if boundary == "verified" || boundary == "published" {
				if err := repairM176Verify(rows)(w.temp); err != nil {
					t.Fatal(err)
				}
			}
			if boundary == "published" {
				if err := w.publish(); err != nil {
					t.Fatal(err)
				}
			} else {
				repairM176Absent(t, path)
			}
			temp := w.temp
			repairM176Stop(t, w)
			w = nil

			// A new writer reopens the directory and reconciles the previous
			// writer's staging file, then uses the real verifier and publisher.
			err = writeExclusive(path, sealed, repairM176Verify(rows))
			if boundary == "published" {
				if !errors.Is(err, os.ErrExist) {
					t.Fatalf("published archive must not be replaced on restart: %v", err)
				}
			} else if err != nil {
				t.Fatalf("restart could not recover an interrupted %s write: %v", boundary, err)
			}
			repairM176Absent(t, temp)
			if err := repairM176Verify(rows)(path); err != nil {
				t.Fatalf("reopened archive did not recover rows: %v", err)
			}
			raw, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(raw, sealed) {
				t.Fatalf("recovered ciphertext differs: %v", err)
			}
			if bytes.Contains(raw, []byte("private envelope")) {
				t.Fatal("archive contains a cleartext row")
			}
			info, err = os.Stat(path)
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("published file must be private: %v, %v", info, err)
			}
		})
	}
}

func TestRepairM_176_VerifyBeforePublication(t *testing.T) {
	rows, sealed := repairM176Fixture(t, "verified")
	otherRows, other := repairM176Fixture(t, "different")
	corrupt := bytes.Clone(sealed)
	corrupt[len(corrupt)-1] ^= 1
	for _, tc := range []struct {
		name string
		data []byte
		want error
	}{
		{"truncated", sealed[:headerLen], ErrFormat},
		{"unauthenticated", corrupt, ErrDecrypt},
		{"different_rows", other, ErrVerify},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "funded.archive")
			if err := writeExclusive(path, tc.data, repairM176Verify(rows)); !errors.Is(err, tc.want) {
				t.Fatalf("verification refusal: got %v, want %v", err, tc.want)
			}
			repairM176Absent(t, path)
			if err := writeExclusive(path, sealed, func(staged string) error {
				repairM176Absent(t, path)
				return repairM176Verify(rows)(staged)
			}); err != nil {
				t.Fatalf("retry after failed verification: %v", err)
			}
			if err := repairM176Verify(rows)(path); err != nil {
				t.Fatal(err)
			}
		})
	}
	// A pre-existing authenticated archive with different rows is preserved.
	path := filepath.Join(t.TempDir(), "funded.archive")
	if err := writeExclusive(path, other, repairM176Verify(otherRows)); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusive(path, sealed, repairM176Verify(rows)); !errors.Is(err, os.ErrExist) {
		t.Fatalf("replacement was not refused: %v", err)
	}
	if err := repairM176Verify(otherRows)(path); err != nil {
		t.Fatalf("existing archive changed: %v", err)
	}
}

func TestRepairM_176_PublicationRace(t *testing.T) {
	rows, sealed := repairM176Fixture(t, "racing")
	path := filepath.Join(t.TempDir(), "funded.archive")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- writeExclusive(path, sealed, repairM176Verify(rows))
		}()
	}
	wg.Wait()
	close(results)
	success, exists := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, os.ErrExist) {
			exists++
		} else {
			t.Fatalf("concurrent writer failed unexpectedly: %v", err)
		}
	}
	if success != 1 || exists != 1 {
		t.Fatalf("expected one publication and one refusal, got %d and %d", success, exists)
	}
	if err := repairM176Verify(rows)(path); err != nil {
		t.Fatal(err)
	}

	// A publisher that does not use the cooperative lock can win after the
	// initial absence check. Link must still refuse to replace its final file.
	racePath := filepath.Join(t.TempDir(), "funded.archive")
	if err := writeExclusive(racePath, sealed, func(staged string) error {
		if err := repairM176Verify(rows)(staged); err != nil {
			return err
		}
		return os.Link(path, racePath)
	}); !errors.Is(err, os.ErrExist) {
		t.Fatalf("publication race did not preserve existing final file: %v", err)
	}
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(racePath)
	if err != nil || !os.SameFile(first, second) {
		t.Fatalf("publication replaced the winning inode: %v", err)
	}
}

func TestRepairM_176_StagingSymlinkRefused(t *testing.T) {
	rows, sealed := repairM176Fixture(t, "symlink")
	dir := t.TempDir()
	path := filepath.Join(dir, "funded.archive")
	target := filepath.Join(dir, "unrelated")
	if err := os.WriteFile(target, []byte("preserve this file"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(dir, ".funded.archive.archive-tmp")
	if err := os.Symlink(target, staged); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusive(path, sealed, repairM176Verify(rows)); !errors.Is(err, ErrFormat) {
		t.Fatalf("staging symlink was not refused: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "preserve this file" {
		t.Fatalf("staging reconciliation changed symlink target: %v", err)
	}
	repairM176Absent(t, path)
	if _, err := os.Readlink(staged); err != nil {
		t.Fatalf("staging symlink was removed: %v", err)
	}
}

func TestRepairM_176_FilesystemFailures(t *testing.T) {
	rows, sealed := repairM176Fixture(t, "filesystem")
	for _, boundary := range []string{"write", "file_sync", "directory_sync"} {
		t.Run(boundary, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "funded.archive")
			w, err := beginArchiveWrite(path)
			if err != nil {
				t.Fatal(err)
			}
			temp := w.temp
			if boundary == "directory_sync" {
				if err := w.stage(sealed); err != nil {
					t.Fatal(err)
				}
				if err := repairM176Verify(rows)(w.temp); err != nil {
					t.Fatal(err)
				}
				// The real link succeeds, but Sync on a closed directory must
				// report a durability error rather than a successful archive.
				if err := w.dir.Close(); err != nil {
					t.Fatal(err)
				}
				if err := w.publish(); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("directory sync failure was lost: %v", err)
				}
			} else {
				if boundary == "file_sync" {
					if err := w.write(sealed); err != nil {
						t.Fatal(err)
					}
				}
				if err := w.file.Close(); err != nil {
					t.Fatal(err)
				}
				if boundary == "write" {
					err = w.stage(sealed)
				} else {
					err = w.syncAndClose()
				}
				if !errors.Is(err, os.ErrClosed) {
					t.Fatalf("%s failure was lost: %v", boundary, err)
				}
				repairM176Absent(t, path)
			}
			if err := w.close(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("cleanup must retain the real descriptor error: %v", err)
			}
			w = nil
			repairM176Absent(t, temp)
			err = writeExclusive(path, sealed, repairM176Verify(rows))
			if boundary == "directory_sync" {
				if !errors.Is(err, os.ErrExist) {
					t.Fatalf("retry must preserve the already published archive: %v", err)
				}
			} else if err != nil {
				t.Fatalf("retry after %s error: %v", boundary, err)
			}
			if err := repairM176Verify(rows)(path); err != nil {
				t.Fatalf("archive reopened after %s error: %v", boundary, err)
			}
		})
	}
}

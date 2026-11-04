package archive

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/scrypt"
	"golang.org/x/sys/unix"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
)

const (
	EnvPassphrase    = "CEREMONY_ARCHIVE_PASSPHRASE"
	EnvPath          = "CEREMONY_ARCHIVE_PATH"
	MinPassphraseLen = 16
	magic            = "PXWARCH1"
	saltLen          = 16
	nonceLen         = 12
	keyLen           = 32
	scryptLogN       = 15
	scryptR          = 8
	scryptP          = 1
	headerLen        = len(magic) + 3 + saltLen + nonceLen
	maxArchiveSize   = 1 << 30
)

var (
	ErrPassphrase = errors.New("archive: passphrase is missing or shorter than 16 bytes")
	ErrPath       = errors.New("archive: archive path is not set")
	ErrFormat     = errors.New("archive: malformed archive file")
	ErrDecrypt    = errors.New("archive: archive does not decrypt under the passphrase")
	ErrVerify     = errors.New("archive: archive rows differ from the database rows")
)

type Result struct {
	Path string
	Rows int
}

type payload struct {
	Table string            `json:"table"`
	Rows  []json.RawMessage `json:"rows"`
}

func LoadPassphrase(getenv func(string) string) ([]byte, error) {
	p := getenv(EnvPassphrase)
	if len(p) < MinPassphraseLen {
		return nil, fmt.Errorf("%w: %s", ErrPassphrase, EnvPassphrase)
	}
	return []byte(p), nil
}

func LoadPath(getenv func(string) string) (string, error) {
	p := strings.TrimSpace(getenv(EnvPath))
	if p == "" {
		return "", fmt.Errorf("%w: %s", ErrPath, EnvPath)
	}
	return p, nil
}

func ReadFundedRows(ctx context.Context, db *sql.DB) ([]json.RawMessage, error) {
	rows, err := db.QueryContext(ctx, `select row_to_json(w)::text from wallets w where `+migrate.FundedPredicate()+` order by w.id`)
	if err != nil {
		return nil, fmt.Errorf("archive: read funded wallets: %w", err)
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("archive: scan funded wallet: %w", err)
		}
		out = append(out, json.RawMessage(s))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("archive: read funded wallets: %w", err)
	}
	return out, nil
}

func Archive(ctx context.Context, db *sql.DB, path string, passphrase []byte) (Result, error) {
	if len(passphrase) < MinPassphraseLen {
		return Result{}, ErrPassphrase
	}
	if path == "" {
		return Result{}, ErrPath
	}
	rows, err := ReadFundedRows(ctx, db)
	if err != nil {
		return Result{}, err
	}
	plain, err := json.Marshal(payload{Table: "wallets", Rows: rows})
	if err != nil {
		return Result{}, err
	}
	sealed, err := Seal(plain, passphrase)
	zero(plain)
	if err != nil {
		return Result{}, err
	}
	if err := writeExclusive(path, sealed, func(staged string) error {
		back, err := Open(staged, passphrase)
		if err != nil {
			return err
		}
		if err := sameRows(rows, back); err != nil {
			return err
		}
		_, err = Verify(ctx, db, staged, passphrase)
		return err
	}); err != nil {
		return Result{}, err
	}
	return Result{Path: path, Rows: len(rows)}, nil
}

func Verify(ctx context.Context, db *sql.DB, path string, passphrase []byte) (Result, error) {
	back, err := Open(path, passphrase)
	if err != nil {
		return Result{}, err
	}
	fresh, err := ReadFundedRows(ctx, db)
	if err != nil {
		return Result{}, err
	}
	if err := sameRows(fresh, back); err != nil {
		return Result{}, err
	}
	return Result{Path: path, Rows: len(back)}, nil
}

func Seal(plain, passphrase []byte) ([]byte, error) {
	header := make([]byte, headerLen)
	copy(header, magic)
	header[len(magic)] = scryptLogN
	header[len(magic)+1] = scryptR
	header[len(magic)+2] = scryptP
	salt := header[len(magic)+3 : len(magic)+3+saltLen]
	nonce := header[len(magic)+3+saltLen:]
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	gcm, err := newGCM(passphrase, salt, scryptLogN, scryptR, scryptP)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(header, nonce, plain, header), nil
}

func Unseal(data, passphrase []byte) ([]byte, error) {
	if len(data) < headerLen+16 || string(data[:len(magic)]) != magic {
		return nil, ErrFormat
	}
	logN, r, p := data[len(magic)], data[len(magic)+1], data[len(magic)+2]
	if !supportedScryptCost(logN, r, p) {
		return nil, ErrFormat
	}
	header := data[:headerLen]
	salt := header[len(magic)+3 : len(magic)+3+saltLen]
	nonce := header[len(magic)+3+saltLen:]
	gcm, err := newGCM(passphrase, salt, int(logN), int(r), int(p))
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, data[headerLen:], header)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plain, nil
}

func supportedScryptCost(logN, r, p byte) bool {
	if logN < 14 || logN > 22 || r == 0 || r > 32 || p == 0 || p > 16 {
		return false
	}
	// Bound both allocations (V, B and XY) and work by the writer's profile.
	// This also retains older, cheaper profiles without trusting header fields.
	n, block, parallel := uint64(1)<<logN, uint64(r), uint64(p)
	memory := 128 * block * (n + parallel + 2)
	const maxMemory = 128 * scryptR * ((1 << scryptLogN) + scryptP + 2)
	const maxWork = (1 << scryptLogN) * scryptR * scryptP
	return memory <= maxMemory && n*block*parallel <= maxWork
}

func Open(path string, passphrase []byte) ([]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("archive: open: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxArchiveSize+1))
	if err != nil {
		return nil, fmt.Errorf("archive: read: %w", err)
	}
	if len(data) > maxArchiveSize {
		return nil, ErrFormat
	}
	plain, err := Unseal(data, passphrase)
	if err != nil {
		return nil, err
	}
	defer zero(plain)
	var p payload
	if err := json.Unmarshal(plain, &p); err != nil || p.Table != "wallets" {
		return nil, ErrFormat
	}
	return p.Rows, nil
}

func newGCM(passphrase, salt []byte, logN, r, p int) (cipher.AEAD, error) {
	key, err := scrypt.Key(passphrase, salt, 1<<logN, r, p, keyLen)
	if err != nil {
		return nil, err
	}
	defer zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithNonceSize(block, nonceLen)
}

type archiveWrite struct {
	path     string
	temp     string
	dir      *os.File
	file     *os.File
	ownsTemp bool
}

func beginArchiveWrite(path string) (_ *archiveWrite, err error) {
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("archive: open directory: %w", err)
	}
	w := &archiveWrite{path: path, temp: filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".archive-tmp"), dir: dir}
	defer func() {
		if err != nil {
			err = errors.Join(err, w.close())
		}
	}()
	// A directory lock survives neither close nor process exit. It serializes
	// reconciliation and publication, including independent writers after restart.
	if err = unix.Flock(int(dir.Fd()), unix.LOCK_EX); err != nil {
		return nil, fmt.Errorf("archive: lock directory: %w", err)
	}
	// The reserved staging name is never committed state. Discard any regular
	// orphan and regenerate from current rows; never follow a staged symlink.
	if info, statErr := os.Lstat(w.temp); statErr == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: staging path is not a regular file", ErrFormat)
		}
		if err = os.Remove(w.temp); err != nil {
			return nil, fmt.Errorf("archive: reconcile staging file: %w", err)
		}
		if err = dir.Sync(); err != nil {
			return nil, fmt.Errorf("archive: sync reconciliation: %w", err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("archive: inspect staging file: %w", statErr)
	}
	if _, statErr := os.Lstat(path); statErr == nil {
		return nil, fmt.Errorf("archive: create: %w", os.ErrExist)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("archive: inspect final file: %w", statErr)
	}
	w.file, err = os.OpenFile(w.temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("archive: create: %w", err)
	}
	w.ownsTemp = true
	return w, nil
}

func (w *archiveWrite) stage(data []byte) error {
	if err := w.write(data); err != nil {
		return err
	}
	return w.syncAndClose()
}

func (w *archiveWrite) write(data []byte) error {
	n, err := w.file.Write(data)
	if err != nil {
		return fmt.Errorf("archive: write: %w", err)
	}
	if n != len(data) {
		return fmt.Errorf("archive: write: %w", io.ErrShortWrite)
	}
	return nil
}

func (w *archiveWrite) syncAndClose() error {
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("archive: sync: %w", err)
	}
	err := w.file.Close()
	w.file = nil
	if err != nil {
		return fmt.Errorf("archive: close: %w", err)
	}
	return nil
}

func (w *archiveWrite) publish() error {
	// Link is an atomic, non-replacing publication on the same filesystem.
	if err := os.Link(w.temp, w.path); err != nil {
		return fmt.Errorf("archive: publish: %w", err)
	}
	if err := w.dir.Sync(); err != nil {
		return fmt.Errorf("archive: sync publication: %w", err)
	}
	return nil
}

func (w *archiveWrite) close() error {
	var err error
	if w.file != nil {
		err = w.file.Close()
		w.file = nil
	}
	if w.ownsTemp {
		if removeErr := os.Remove(w.temp); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("archive: remove staging file: %w", removeErr))
		} else {
			err = errors.Join(err, w.dir.Sync())
		}
		w.ownsTemp = false
	}
	return errors.Join(err, w.dir.Close())
}

func writeExclusive(path string, data []byte, verify func(string) error) (err error) {
	w, err := beginArchiveWrite(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, w.close()) }()
	if err := w.stage(data); err != nil {
		return err
	}
	if err := verify(w.temp); err != nil {
		return err
	}
	return w.publish()
}

func sameRows(want, got []json.RawMessage) error {
	if len(want) != len(got) {
		return fmt.Errorf("%w: %d rows in the database, %d in the archive", ErrVerify, len(want), len(got))
	}
	for i := range want {
		var a, b bytes.Buffer
		if err := json.Compact(&a, want[i]); err != nil {
			return fmt.Errorf("%w: row %d", ErrVerify, i)
		}
		if err := json.Compact(&b, got[i]); err != nil {
			return fmt.Errorf("%w: row %d", ErrVerify, i)
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			return fmt.Errorf("%w: row %d", ErrVerify, i)
		}
	}
	return nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

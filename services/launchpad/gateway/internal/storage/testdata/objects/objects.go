// Package objects provides immutable, content-addressed files for the repair's
// real metadata publisher and gateway reader, sharing the same on-disk bytes.
package objects

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
)

type Store struct{ Root string }

func validKey(key string) bool {
	p := strings.Split(key, "/")
	if !fs.ValidPath(key) || len(p) != 4 || p[0] != "approved" || (p[2] != "256" && p[2] != "512" && p[2] != "1024") || !strings.HasSuffix(p[3], ".png") {
		return false
	}
	for _, value := range []string{p[1], strings.TrimSuffix(p[3], ".png")} {
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != sha256.Size || value != strings.ToLower(value) {
			return false
		}
	}
	return true
}

func (s Store) PutImmutable(ctx context.Context, key string, body []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	if !validKey(key) || len(body) == 0 || len(body) > 6<<20 || path.Base(key) != hex.EncodeToString(digest[:])+".png" {
		return errors.New("invalid immutable object")
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = root.MkdirAll(path.Dir(key), 0700); err != nil {
		return err
	}
	file, err := root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, fs.ErrExist) {
		previous, readErr := root.ReadFile(key)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(previous, body) {
			return errors.New("immutable object conflict")
		}
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := file.Write(body)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return errors.Join(err, root.Remove(key))
	}
	return nil
}

func (s Store) Get(ctx context.Context, key string) (*storage.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validKey(key) {
		return nil, storage.ErrNotFound
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	body, err := root.ReadFile(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(body)
	etag := hex.EncodeToString(digest[:])
	if len(body) == 0 || len(body) > 6<<20 || path.Base(key) != etag+".png" {
		return nil, errors.New("corrupt immutable object")
	}
	return &storage.Object{Body: io.NopCloser(bytes.NewReader(body)), Size: int64(len(body)), ContentType: "image/png", ETag: etag}, nil
}

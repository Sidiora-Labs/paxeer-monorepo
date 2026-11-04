package tasks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func recoveryHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readRecoveryFile(home, name string) ([]byte, error) {
	dir, err := openConfinedDir(home, false)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return confinedRead(dir, name, 64<<20)
}

func saveRecovery(home, name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return durableJSONFile(home, name, data)
}

func ensureRecoveryDir(path string) error {
	dir, err := openConfinedDir(path, true)
	if err != nil {
		return err
	}
	return dir.Close()
}

func lockRecovery(ctx context.Context, home, name string) (func(), error) {
	dir, err := openConfinedDir(home, true)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	for {
		if err := ctx.Err(); err != nil {
			f.Close()
			return nil, err
		}
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() { _ = unix.Flock(fd, unix.LOCK_UN); _ = f.Close() }, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			f.Close()
			return nil, err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			f.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

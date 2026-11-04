package bringup

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// WithMutation serializes a complete render/validate/reload transaction using
// the same stable inode as every shell writer and secondary import owner.
func (s *EdgeState) WithMutation(action func() error) error {
	if s.Root == "" {
		return errors.New("edge state closed")
	}
	f, e := os.OpenFile(filepath.Join(s.Root, ".edge-operation.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	if e = s.Recover(); e != nil {
		return e
	}
	return action()
}

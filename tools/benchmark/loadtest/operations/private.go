package operations

import (
	"errors"
	"os"
	"syscall"
)

func openExclusiveAt(dir *os.File, name string) (*os.File, error) {
	fd, err := syscall.Openat(int(dir.Fd()), name, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	if err = file.Chmod(0600); err != nil {
		return nil, closeWithError(file, err)
	}
	return file, nil
}

func openPrivateFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		return nil, closeWithError(file, err)
	}
	uid, err := effectiveUID()
	if err != nil {
		return nil, closeWithError(file, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || uint64(info.Sys().(*syscall.Stat_t).Uid) != uid {
		return nil, closeWithError(file, errors.New("file must be an owner-only regular file"))
	}
	return file, nil
}

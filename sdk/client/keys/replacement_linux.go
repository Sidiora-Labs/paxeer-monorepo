package keys

import "golang.org/x/sys/unix"

func exchangeKeyringDirectories(liveDir, stageDir string) error {
	return unix.Renameat2(unix.AT_FDCWD, liveDir, unix.AT_FDCWD, stageDir, unix.RENAME_EXCHANGE)
}

//go:build !linux

package keys

import "errors"

func exchangeKeyringDirectories(liveDir, stageDir string) error {
	return errors.New("atomic key replacement requires directory exchange support on this platform; the existing key has been preserved")
}

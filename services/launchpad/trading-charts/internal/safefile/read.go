package safefile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReadRegular reads a bounded regular file within its containing directory.
func ReadRegular(path string, maximum int64) (raw []byte, err error) {
	file, err := os.OpenInRoot(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, fmt.Errorf("bounded regular file required")
	}
	raw, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maximum {
		return nil, fmt.Errorf("bounded regular file required")
	}
	return raw, nil
}

package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func readSocialBoundFile(path string) (data []byte, result error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	file, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 1048576 {
		return nil, fmt.Errorf("social bound file unavailable")
	}
	data, err = io.ReadAll(io.LimitReader(file, 1048577))
	if err != nil || len(data) < 1 || len(data) > 1048576 {
		return nil, fmt.Errorf("social bound file unavailable")
	}
	return data, nil
}

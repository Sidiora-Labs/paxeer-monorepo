package config

import (
	"errors"
	"os"
	"path/filepath"
)

func readFileWithinPath(path string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	data, readErr := root.ReadFile(filepath.Base(path))
	if err := errors.Join(readErr, root.Close()); err != nil {
		return nil, err
	}
	return data, nil
}

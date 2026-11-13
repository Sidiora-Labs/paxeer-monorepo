package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func readRegularFile(path string, minSize, maxSize int64) (data []byte, result error) {
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
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < minSize || info.Size() > maxSize {
		return nil, fmt.Errorf("file size or type invalid")
	}
	data, err = io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) < minSize || int64(len(data)) > maxSize {
		return nil, fmt.Errorf("file size invalid")
	}
	return data, nil
}

package util

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func OpenPath(path string) (*os.File, error) {
	return OpenFilePath(path, os.O_RDONLY, 0)
}

func OpenFilePath(path string, flag int, perm os.FileMode) (*os.File, error) {
	directory, name := filepath.Split(path)
	if name == "" {
		return nil, errors.New("file name required")
	}
	if directory == "" {
		directory = "."
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	file, openErr := root.OpenFile(name, flag, perm)
	closeErr := root.Close()
	if openErr != nil {
		return nil, errors.Join(openErr, closeErr)
	}
	if closeErr != nil {
		return nil, errors.Join(closeErr, file.Close())
	}
	return file, nil
}

func ReadFilePath(path string) ([]byte, error) {
	file, err := OpenPath(path)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(file)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, err
	}
	return data, nil
}

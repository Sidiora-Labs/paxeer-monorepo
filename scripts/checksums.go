package scripts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func PublishChecksums(directory, output string) (string, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", root)
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return "", err
	}
	outputParent, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return "", err
	}
	output = filepath.Join(outputParent, filepath.Base(output))
	// A sibling staging file is outside the input tree even when TMPDIR is inside it.
	if filepath.Dir(root) == root {
		return "", errors.New("cannot stage a manifest outside the filesystem root")
	}
	stage, err := os.CreateTemp(filepath.Dir(root), ".pax-checksums-")
	if err != nil {
		return "", err
	}
	defer os.Remove(stage.Name())
	defer stage.Close()
	var paths []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == output {
			return nil
		}
		if entry.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, path := range paths {
		file, openErr := os.Open(path)
		if openErr != nil {
			return "", openErr
		}
		before, statErr := file.Stat()
		if statErr != nil {
			file.Close()
			return "", statErr
		}
		if !before.Mode().IsRegular() {
			file.Close()
			return "", fmt.Errorf("input is no longer a regular file: %s", path)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		after, afterErr := file.Stat()
		closeErr := file.Close()
		if err = errors.Join(copyErr, afterErr, closeErr); err != nil {
			return "", err
		}
		current, currentErr := os.Stat(path)
		if currentErr != nil {
			return "", currentErr
		}
		if !os.SameFile(before, current) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return "", fmt.Errorf("input changed while hashing: %s", path)
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return "", relErr
		}
		name := "./" + filepath.ToSlash(relative)
		escaped := strings.ContainsAny(name, "\\\n\r")
		prefix := ""
		if escaped {
			prefix = "\\"
			name = strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "\\r").Replace(name)
		}
		if _, err = fmt.Fprintf(stage, "%s%x  %s\n", prefix, hash.Sum(nil), name); err != nil {
			return "", err
		}
	}
	if err = stage.Sync(); err != nil {
		return "", err
	}
	if _, err = stage.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, stage); err != nil {
		return "", err
	}
	if _, err = stage.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if err = publishFile(output, stage, 0600); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

package meshpki

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// resolveDirectory resolves links through the closest existing ancestor,
// including when the requested output directory does not yet exist.
func resolveDirectory(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("output directory required")
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	current := absolute
	var missing []string
	for {
		real, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				real = filepath.Join(real, missing[i])
			}
			return real, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func outsideWorktree(dir string) error {
	for current := dir; ; current = filepath.Dir(current) {
		_, err := os.Lstat(filepath.Join(current, ".git"))
		if err == nil {
			return errors.New("output directory must be outside a Git working copy")
		}
		if !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

func openStore(dir string, create bool) (*os.Root, error) {
	real, err := resolveDirectory(dir)
	if err != nil {
		return nil, err
	}
	if err = outsideWorktree(real); err != nil {
		return nil, err
	}
	// Also check the supplied path, which may enter a worktree through a link.
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = outsideWorktree(absolute); err != nil {
		return nil, err
	}
	if create {
		if err = os.MkdirAll(real, 0o700); err != nil {
			return nil, err
		}
	}
	return os.OpenRoot(real)
}

func readFile(root *os.Root, name string) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 1048576 {
		return nil, errors.New("regular nonempty material file of at most 1 MiB required")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 1048577))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw) > 1048576 {
		return nil, errors.New("material file size outside bounds")
	}
	return raw, nil
}

type outputFile struct {
	name    string
	data    []byte
	private bool
}

// Reserve every filename before writing any bytes. O_EXCL also refuses links.
func writeFiles(root *os.Root, outputs []outputFile) (err error) {
	handles := make([]*os.File, 0, len(outputs))
	defer func() {
		for i, file := range handles {
			_ = file.Close()
			if err != nil {
				_ = root.Remove(outputs[i].name)
			}
		}
	}()
	for _, output := range outputs {
		mode := os.FileMode(0o644)
		if output.private {
			mode = 0o600
		}
		file, openErr := root.OpenFile(output.name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if openErr != nil {
			return fmt.Errorf("create %s: %w", output.name, openErr)
		}
		handles = append(handles, file)
		if err = file.Chmod(mode); err != nil {
			return err
		}
	}
	for i, output := range outputs {
		if _, err = handles[i].Write(output.data); err != nil {
			return err
		}
		if err = handles[i].Sync(); err != nil {
			return err
		}
	}
	for _, file := range handles {
		if err = file.Close(); err != nil {
			return err
		}
	}
	return nil
}

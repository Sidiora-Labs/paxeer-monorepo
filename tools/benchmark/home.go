package benchmark

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const benchmarkHomeMarker = ".pax-benchmark-home"

// RunHome prepares the dedicated home consumed by benchmark.sh. Reset is an
// exact path confirmation and is accepted only for a previously marked home.
func RunHome(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("benchmark-home", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	target := flags.String("home", "", "dedicated benchmark directory")
	userHome := flags.String("user-home", "", "user home to protect")
	protected := flags.String("protected", "", "repository directory to protect")
	reset := flags.String("reset", "", "exact benchmark path to reset")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected benchmark-home arguments")
	}
	if !filepath.IsAbs(*target) || strings.ContainsAny(*target, "\r\n") {
		return errors.New("benchmark home must be an absolute, nonempty path")
	}
	clean := filepath.Clean(*target)
	if clean != strings.TrimRight(*target, string(os.PathSeparator)) {
		return errors.New("benchmark home must not contain dot or parent components")
	}
	if *reset != "" && *reset != clean {
		return errors.New("reset must confirm the exact benchmark home path")
	}
	if !filepath.IsAbs(*userHome) || !filepath.IsAbs(*protected) {
		return errors.New("user home and protected directory must be absolute paths")
	}
	home, err := filepath.EvalSymlinks(*userHome)
	if err != nil {
		return fmt.Errorf("resolve user home: %w", err)
	}
	repo, err := filepath.EvalSymlinks(*protected)
	if err != nil {
		return fmt.Errorf("resolve protected directory: %w", err)
	}
	defaultHome := filepath.Join(home, ".pax")
	if containsPath(clean, home) || containsPath(clean, repo) || containsPath(defaultHome, clean) || containsPath(clean, defaultHome) {
		return errors.New("benchmark home overlaps a protected directory")
	}
	// Also protect the destination of an existing default-home symlink.
	resolvedDefault, err := filepath.EvalSymlinks(defaultHome)
	if err == nil {
		if containsPath(resolvedDefault, clean) || containsPath(clean, resolvedDefault) {
			return errors.New("benchmark home overlaps the default node directory")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("resolve default node home: %w", err)
	}

	root, err := openBenchmarkDirectory(clean)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := benchmarkDirectoryEntries(root)
	if err != nil {
		return err
	}
	marker := "pax-benchmark-home-v1\n" + clean + "\n"
	if len(entries) != 0 {
		if *reset == "" {
			return errors.New("benchmark home is not empty; BENCHMARK_RESET_HOME must confirm its exact path")
		}
		info, err := root.Lstat(benchmarkHomeMarker)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("refusing reset of an unmarked benchmark home")
		}
		f, err := root.Open(benchmarkHomeMarker)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(f, int64(len(marker)+1)))
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if string(data) != marker {
			return errors.New("benchmark home ownership marker does not match its path")
		}
		// All removal is relative to this open directory, including symlink entries.
		// Never remove the directory itself or resolve a child link to its target.
		for _, entry := range entries {
			if entry.Name() == benchmarkHomeMarker {
				continue
			}
			if err := root.RemoveAll(entry.Name()); err != nil {
				return fmt.Errorf("reset benchmark state: %w", err)
			}
		}
	} else {
		f, err := root.OpenFile(benchmarkHomeMarker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := io.WriteString(f, marker)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	// Refuse to hand a replaced path back to the shell after preparing an open fd.
	opened, err := root.Stat(".")
	if err != nil {
		return err
	}
	current, err := os.Lstat(clean)
	if err != nil {
		return err
	}
	if !os.SameFile(opened, current) {
		return errors.New("benchmark directory changed during preparation")
	}
	_, err = fmt.Fprintln(output, clean)
	return err
}

func containsPath(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

// Walk with directory handles and reject links before opening each component.
// Checking inode identity after open also detects replacement during the walk.
func openBenchmarkDirectory(path string) (*os.Root, error) {
	volume := filepath.VolumeName(path) + string(os.PathSeparator)
	root, err := os.OpenRoot(volume)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(strings.TrimPrefix(path, volume), string(os.PathSeparator)) {
		info, err := root.Lstat(component)
		if errors.Is(err, os.ErrNotExist) {
			err = root.Mkdir(component, 0700)
			if err == nil {
				info, err = root.Lstat(component)
			}
		}
		if err != nil {
			root.Close()
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, errors.New("benchmark home components must be directories, never symlinks")
		}
		next, err := root.OpenRoot(component)
		if err != nil {
			root.Close()
			return nil, err
		}
		opened, err := next.Stat(".")
		root.Close()
		if err != nil || !os.SameFile(info, opened) {
			next.Close()
			return nil, errors.New("benchmark home component changed during preparation")
		}
		root = next
	}
	return root, nil
}

func benchmarkDirectoryEntries(root *os.Root) ([]os.DirEntry, error) {
	dir, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if readErr != nil {
		return nil, readErr
	}
	return entries, closeErr
}

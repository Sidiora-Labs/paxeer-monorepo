package qualification

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var ClockRoots = []string{"src/protocol", "src/state", "src/ledger", "src/modules"}
var ClockPatterns = []string{
	`(^|[^[:alnum:]_])(time|gettimeofday|clock_gettime|rand)[[:space:]]*\(`,
	`#[[:space:]]*include[[:space:]]*<[[:space:]]*time\.h[[:space:]]*>`,
}

// ScanClockSources checks every declared input before accepting a negative scan.
// It also scans in process so the prerequisite and content checks share one path.
func ScanClockSources(root string) error {
	for _, name := range ClockRoots {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("clock scan input %s: %w", name, err)
		}
		if !info.IsDir() || info.Mode().Perm()&0444 == 0 || info.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("clock scan input %s is not a readable, searchable directory", name)
		}
		err = filepath.WalkDir(path, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			// rg does not follow symlinks by default; an uninspected source must not
			// silently count as clean, even when qualification runs as root.
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("uninspected symlink %s", path)
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if info.Mode().Perm()&0444 == 0 || info.Mode().Perm()&0111 == 0 {
					return fmt.Errorf("unreadable directory %s", path)
				}
				return nil
			}
			if !strings.HasSuffix(path, ".c") {
				return nil
			}
			if !info.Mode().IsRegular() || info.Mode().Perm()&0444 == 0 {
				return fmt.Errorf("unreadable source %s", path)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, pattern := range ClockPatterns {
				if regexp.MustCompile(pattern).Match(content) {
					return fmt.Errorf("%s references a forbidden clock or entropy API", path)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("clock scan input %s: %w", name, err)
		}
	}
	return nil
}

// CheckRGExit distinguishes a match, an exhaustive negative result and a tool error.
func CheckRGExit(status int, stderr string) error {
	switch status {
	case 0:
		return fmt.Errorf("consensus transition source references a forbidden clock or entropy API")
	case 1:
		if stderr != "" {
			return fmt.Errorf("clock scan diagnostic: %s", stderr)
		}
		return nil
	default:
		return fmt.Errorf("rg clock scan failed (exit %d): %s", status, stderr)
	}
}

func FindRG(path string) (string, error) {
	for _, directory := range filepath.SplitList(path) {
		if directory == "" {
			continue
		}
		candidate, err := filepath.Abs(filepath.Join(directory, "rg"))
		if err != nil {
			return "", err
		}
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("clock scan requires an executable rg on PATH")
}

func CheckClock(root string) error {
	rg, err := FindRG(os.Getenv("PATH"))
	if err != nil {
		return err
	}
	if err := ScanClockSources(root); err != nil {
		return err
	}
	for _, pattern := range ClockPatterns {
		args := []string{"--no-config", "--hidden", "--no-ignore", "-n", "--glob", "*.c", pattern}
		args = append(args, ClockRoots...)
		cmd := exec.Command(rg, args...)
		cmd.Dir = root
		var stderr strings.Builder
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		status := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				status = exit.ExitCode()
			} else {
				return fmt.Errorf("cannot run rg: %w", err)
			}
		}
		if err := CheckRGExit(status, stderr.String()); err != nil {
			return fmt.Errorf("%w\n%s", err, output)
		}
	}
	return nil
}

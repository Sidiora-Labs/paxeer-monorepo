package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type StressConfig struct{ Binary, Home string }

func ValidateLocalHome(userHome, nodeHome string) (string, error) {
	root, err := filepath.EvalSymlinks(userHome)
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(nodeHome)
	if err != nil {
		return "", err
	}
	// Resolve the existing prefix as well as a possibly new final directory.
	prefix := candidate
	var suffix []string
	for {
		_, err = os.Lstat(prefix)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(prefix)
		if parent == prefix {
			return "", fmt.Errorf("cannot resolve %s", candidate)
		}
		suffix = append(suffix, filepath.Base(prefix))
		prefix = parent
	}
	prefix, err = filepath.EvalSymlinks(prefix)
	if err != nil {
		return "", err
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		prefix = filepath.Join(prefix, suffix[i])
	}
	rel, err := filepath.Rel(root, prefix)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("PAX_HOME must resolve to a directory below HOME")
	}
	return prefix, nil
}

func NewStressConfig(userHome, binary, requestedHome string) (StressConfig, error) {
	if binary == "" {
		binary = filepath.Join(userHome, "go", "bin", "paxd")
	}
	var err error
	binary, err = filepath.Abs(binary)
	if err != nil {
		return StressConfig{}, err
	}
	if requestedHome == "" {
		requestedHome, err = os.MkdirTemp(userHome, ".pax-evm-stress-")
		if err != nil {
			return StressConfig{}, err
		}
	} else {
		// A supplied target must still be fresh: stress never acknowledges erasing
		// an existing operator home on the caller's behalf.
		if _, statErr := os.Lstat(requestedHome); !os.IsNotExist(statErr) {
			if statErr != nil {
				return StressConfig{}, statErr
			}
			return StressConfig{}, fmt.Errorf("stress PAX_HOME must be a new disposable directory: %s", requestedHome)
		}
	}
	home, err := ValidateLocalHome(userHome, requestedHome)
	if err != nil {
		return StressConfig{}, err
	}
	return StressConfig{Binary: binary, Home: home}, nil
}

func (cfg StressConfig) ShellEnvironment() string {
	var output strings.Builder
	fmt.Fprintf(&output, "export PAXD_BIN=%s\nexport PAX_HOME=%s\n", shellQuote(cfg.Binary), shellQuote(cfg.Home))
	output.WriteString("PAX_START_ARGS=(")
	for _, arg := range cfg.StartArgs() {
		output.WriteString(shellQuote(arg) + " ")
	}
	output.WriteString(")\n")
	return output.String()
}

func (cfg StressConfig) StartArgs() []string {
	return []string{"start", "--trace", "--chain-id", "pax-chain", "--home", cfg.Home}
}

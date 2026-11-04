package scripts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RunMaintenance is shared by the script entrypoints and the in-process gate.
func RunMaintenance(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "missing maintenance operation")
		return 1
	}
	var err error
	switch args[0] {
	case "duration":
		if len(args) != 2 {
			err = errors.New("duration requires one value")
			break
		}
		seconds, parseErr := DurationSeconds(args[1])
		result := struct {
			Valid   bool    `json:"valid"`
			Seconds float64 `json:"seconds"`
			Error   string  `json:"error,omitempty"`
		}{Valid: parseErr == nil, Seconds: seconds}
		if parseErr != nil {
			result.Error = parseErr.Error()
		}
		err = json.NewEncoder(out).Encode(result)
	case "checksums":
		if len(args) != 2 {
			err = errors.New("checksums requires a directory")
			break
		}
		var digest string
		digest, err = PublishChecksums(args[1], "checksum.list")
		if err == nil {
			_, err = fmt.Fprintf(out, "Checksum of all files in %s:\n\t%s\nPlease view checksum.list for individual file checksums.\n", args[1], digest)
		}
	case "changelog":
		if len(args) != 4 {
			err = errors.New("changelog requires repository, base and head")
			break
		}
		var result string
		result, err = GenerateChangelog(".", args[1], args[2], args[3])
		if err == nil {
			if result == "" {
				_, err = fmt.Fprintf(errOut, "No PRs found between %s and %s.\n", args[2], args[3])
			} else {
				_, err = fmt.Fprintf(errOut, "Found %d PRs.\n", strings.Count(result, "\n")-2)
				if err == nil {
					_, err = io.WriteString(out, result)
				}
			}
		}
	case "consensus-config":
		if len(args) != 2 {
			err = errors.New("consensus-config requires config.toml")
			break
		}
		err = ConfigureConsensus(args[1])
	case "stress-env":
		if len(args) != 1 {
			err = errors.New("stress-env takes no arguments")
			break
		}
		var cfg StressConfig
		cfg, err = NewStressConfig(os.Getenv("HOME"), os.Getenv("PAXD_BIN"), os.Getenv("PAX_HOME"))
		if err == nil {
			_, err = io.WriteString(out, cfg.ShellEnvironment())
		}
	case "validate-local-home":
		if len(args) != 3 {
			err = errors.New("validate-local-home requires HOME and PAX_HOME")
			break
		}
		var home string
		home, err = ValidateLocalHome(args[1], args[2])
		if err == nil {
			_, err = fmt.Fprintln(out, home)
		}
	case "export":
		if len(args) != 5 {
			err = errors.New("export requires binary, home, height and archive")
			break
		}
		var height int64
		height, err = strconv.ParseInt(args[3], 10, 64)
		if err == nil {
			err = ExportNodeState(args[1], args[2], height, args[4], errOut)
		}
	default:
		err = fmt.Errorf("unknown maintenance operation %q", args[0])
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

func DurationSeconds(value string) (float64, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("configuration failure: invalid Go duration %q: %w", value, err)
	}
	return duration.Seconds(), nil
}

func publishFile(path string, data io.Reader, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".pax-publish-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(mode); err == nil {
		_, err = io.Copy(file, data)
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

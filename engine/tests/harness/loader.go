package harness

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// extractOnce ensures we only extract the archive once per test run
var extractOnce sync.Once

// LoadStateTest loads a state test from a JSON file
func LoadStateTest(filePath string) (map[string]*StateTestJSON, error) {
	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var tests map[string]StateTestJSON
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&tests)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*StateTestJSON)
	names := make([]string, 0, len(tests))
	for name := range tests {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		test := tests[name]
		result[name] = &test
	}
	return result, nil
}

// LoadStateTestsFromDir loads all state tests from a directory
func LoadStateTestsFromDir(dirPath string) (map[string]*StateTestJSON, error) {
	result := make(map[string]*StateTestJSON)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		tests, err := LoadStateTest(path)
		if err != nil {
			return err
		}

		names := make([]string, 0, len(tests))
		for name := range tests {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			// Use relative path + test name as key to avoid collisions
			relPath, _ := filepath.Rel(dirPath, path)
			fullName := filepath.Join(relPath, name)
			result[fullName] = tests[name]
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetStateTestsPath returns the path to GeneralStateTests, extracting from archive if needed
func GetStateTestsPath() (string, error) {
	dataPath := filepath.Join("data", "GeneralStateTests")
	if _, err := os.Stat(dataPath); err == nil {
		return dataPath, nil
	}

	archivePath := filepath.Join("data", "fixtures_general_state_tests.tgz")
	_, err := os.Stat(archivePath)
	if err != nil {
		return "", err
	}

	var extractErr error
	extractOnce.Do(func() {
		extractErr = extractArchive(archivePath, "data")
	})
	if extractErr != nil {
		return "", extractErr
	}

	// Always verify the directory exists after sync.Once completes,
	// regardless of whether this was the first invocation
	if _, err = os.Stat(dataPath); err != nil {
		return "", err
	}

	return dataPath, nil
}

// extractArchive extracts a .tgz archive to the destination directory
func extractArchive(archivePath, destDir string) error {
	file, err := os.Open(filepath.Clean(archivePath))
	if err != nil {
		return err
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer zipped.Close()
	archive := tar.NewReader(zipped)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(header.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe fixture archive path %q", header.Name)
		}
		path := filepath.Join(destDir, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(output, archive, header.Size)
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported fixture archive entry %q type %d", header.Name, header.Typeflag)
		}
	}
}

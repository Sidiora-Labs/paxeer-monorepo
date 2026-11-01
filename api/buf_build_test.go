package api_test

import (
	"io/fs"
	"path/filepath"
	"testing"
)

func TestBufBuildWorkspaceImage(t *testing.T) {
	imageCounts := make(map[string]int)
	for _, file := range bufImage(t).GetFile() {
		imageCounts[file.GetName()]++
	}
	sources := make(map[string]string)
	for _, module := range []struct {
		root     string
		baseline int
	}{
		{root: "../api", baseline: 54},
		{root: "../sdk/proto", baseline: 67},
	} {
		t.Run(filepath.ToSlash(module.root), func(t *testing.T) {
			sourceCount, imageCount := 0, 0
			err := filepath.WalkDir(module.root, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() || filepath.Ext(path) != ".proto" {
					return nil
				}
				relative, err := filepath.Rel(module.root, path)
				if err != nil {
					return err
				}
				name := filepath.ToSlash(relative)
				if previous, exists := sources[name]; exists {
					t.Errorf("%s: module-relative name shared by %s and %s", name, previous, path)
				}
				sources[name] = path
				sourceCount++
				count := imageCounts[name]
				imageCount += count
				if count != 1 {
					t.Errorf("%s (%s): buf workspace image has %d occurrences, want exactly 1", name, path, count)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walk proto module %s: %v", module.root, err)
			}
			if sourceCount < module.baseline {
				t.Errorf("%s: found %d proto sources, want at least the %d baseline files plus additions", module.root, sourceCount, module.baseline)
			}
			if imageCount != sourceCount {
				t.Errorf("%s: buf workspace image has %d module files, want %d sources (%d baseline plus %d additions)", module.root, imageCount, sourceCount, module.baseline, sourceCount-module.baseline)
			}
		})
	}
}

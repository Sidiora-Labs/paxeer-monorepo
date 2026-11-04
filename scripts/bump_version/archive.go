package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A pending manifest is published only after the entire payload is staged and
// synced. It is retained until every destination is durable and the completion
// marker has been published. Both records use the same immutable manifest.
type archiveManifest struct {
	Tag    string
	Folder string
	Files  []archiveFile
}

type archiveFile struct {
	Path         string
	SHA256       string
	Versions     bool
	BeforeSHA256 string // empty means the versions file did not exist
}

func archivePaths(folder string) (pending, marker string) {
	parent := filepath.Join(commonDir, "legacy")
	return filepath.Join(parent, "."+folder+".pending"), filepath.Join(parent, folder, ".complete.json")
}

func recoverArchive(tag, folder string) (bool, error) {
	pending, marker := archivePaths(folder)
	data, err := os.ReadFile(marker)
	if err == nil {
		manifest, err := decodeArchive(data, tag, folder)
		if err != nil {
			return false, err
		}
		if err := validatePublishedArchive(manifest); err != nil {
			return false, err
		}
		// A crash immediately after marker publication may leave the staging set.
		if err := os.RemoveAll(pending); err != nil {
			return false, err
		}
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("reading archive completion marker: %w", err)
	}
	data, err = os.ReadFile(filepath.Join(pending, "manifest.json"))
	if err == nil {
		manifest, err := decodeArchive(data, tag, folder)
		if err != nil {
			return false, err
		}
		if err := publishArchive(pending, manifest); err != nil {
			return false, err
		}
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("reading pending archive: %w", err)
	}
	if _, err := os.Stat(pending); err == nil {
		return false, fmt.Errorf("pending archive %s has no manifest; inspect and restore the saved release state", pending)
	} else if !os.IsNotExist(err) {
		return false, err
	}
	// Pre-marker archives cannot establish completeness. Never silently overwrite
	// them or advertise their registrations through regenerated setup files.
	modules, err := discoverModules()
	if err != nil {
		return false, err
	}
	for _, mod := range append([]string{"common"}, modules...) {
		path := filepath.Join(precompilesDir, mod, "legacy", folder)
		if _, err := os.Stat(path); err == nil {
			return false, fmt.Errorf("archive %s has no completion marker or pending manifest; inspect and restore the saved release state before retrying", path)
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}

func decodeArchive(data []byte, tag, folder string) (*archiveManifest, error) {
	var manifest archiveManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("reading archive manifest: %w", err)
	}
	if manifest.Tag != tag || manifest.Folder != folder {
		return nil, fmt.Errorf("archive manifest does not match %s (%s)", tag, folder)
	}
	seen := make(map[string]bool)
	for _, file := range manifest.Files {
		clean := filepath.Clean(file.Path)
		parts := strings.Split(filepath.ToSlash(clean), "/")
		valid := len(parts) == 5 && parts[0] == precompilesDir && parts[2] == "legacy" && parts[3] == folder && !file.Versions
		if file.Versions {
			valid = len(parts) == 3 && parts[0] == precompilesDir && parts[1] != "common" && parts[2] == "versions"
		}
		if clean != file.Path || !valid || seen[clean] || len(file.SHA256) != 64 {
			return nil, fmt.Errorf("invalid archive manifest path or digest: %q", file.Path)
		}
		seen[clean] = true
	}
	return &manifest, nil
}

func fileDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func stageArchive(modules []string, tag, folder, legacyCommon string) (string, *archiveManifest, error) {
	pending, _ := archivePaths(folder)
	parent := filepath.Dir(pending)
	if err := os.MkdirAll(parent, 0750); err != nil {
		return "", nil, err
	}
	stage, err := os.MkdirTemp(parent, "."+folder+".staging-")
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	manifest := &archiveManifest{Tag: tag, Folder: folder}
	add := func(path string, versions bool, before string) error {
		hash, err := fileDigest(filepath.Join(stage, "files", path))
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, archiveFile{Path: path, SHA256: hash, Versions: versions, BeforeSHA256: before})
		return nil
	}
	stageGo := func(src, dst, oldImport, newImport string) error {
		target := filepath.Join(stage, "files", dst)
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}
		if err := rewriteGoFile(src, target, folder, oldImport, newImport); err != nil {
			return fmt.Errorf("staging %s: %w", src, err)
		}
		return add(dst, false, "")
	}
	// No modules selected still produces a completed empty archive transaction.
	if len(modules) > 0 {
		for _, name := range []string{"precompiles.go", "evm_events.go"} {
			src := filepath.Join(commonDir, name)
			if _, err := os.Stat(src); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return "", nil, err
			}
			if err := stageGo(src, filepath.Join(commonDir, "legacy", folder, name), "", ""); err != nil {
				return "", nil, err
			}
		}
	}
	for _, mod := range modules {
		dir := filepath.Join(precompilesDir, mod)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", nil, err
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "setup.go" {
				continue
			}
			if err := stageGo(filepath.Join(dir, name), filepath.Join(dir, "legacy", folder, name), commonPkgPath, legacyCommon); err != nil {
				return "", nil, err
			}
		}
		abi := filepath.Join(dir, "abi.json")
		if _, err := os.Stat(abi); err == nil {
			dst := filepath.Join(dir, "legacy", folder, "abi.json")
			target := filepath.Join(stage, "files", dst)
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return "", nil, err
			}
			if err := copyFile(abi, target); err != nil {
				return "", nil, err
			}
			if err := add(dst, false, ""); err != nil {
				return "", nil, err
			}
		} else if !os.IsNotExist(err) {
			return "", nil, fmt.Errorf("reading %s: %w", abi, err)
		}
	}
	// Validate every registration before publishing even the common archive.
	for _, mod := range modules {
		path := filepath.Join(precompilesDir, mod, "versions")
		before, err := fileDigest(path)
		if err != nil && !os.IsNotExist(err) {
			return "", nil, err
		}
		existing, err := readNonEmptyLines(path)
		if err != nil && !os.IsNotExist(err) {
			return "", nil, err
		}
		if err := validateVersions(mod, existing); err != nil {
			return "", nil, err
		}
		if pos, match := semverIndexOf(existing, tag); pos != -1 {
			return "", nil, fmt.Errorf("%s: %s (semver-equal to %s) already exists at line %d", path, tag, match, pos+1)
		}
		target := filepath.Join(stage, "files", path)
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return "", nil, err
		}
		if err := writeLines(target, append(existing, tag)); err != nil {
			return "", nil, err
		}
		if err := add(path, true, before); err != nil {
			return "", nil, err
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), data, 0600); err != nil {
		return "", nil, err
	}
	if err := syncArchiveTree(stage); err != nil {
		return "", nil, err
	}
	if err := os.Rename(stage, pending); err != nil {
		return "", nil, err
	}
	if err := syncDirectory(parent); err != nil {
		return "", nil, err
	}
	return pending, manifest, nil
}

func validatePublishedArchive(manifest *archiveManifest) error {
	for _, file := range manifest.Files {
		hash, err := fileDigest(file.Path)
		if err != nil {
			return fmt.Errorf("validating completed archive %s: %w", file.Path, err)
		}
		if hash != file.SHA256 {
			return fmt.Errorf("completed archive %s differs from its completion marker; inspect before retrying", file.Path)
		}
	}
	return nil
}

func publishArchive(pending string, manifest *archiveManifest) error {
	// Validate the complete staged set and all destinations before any publication.
	for _, file := range manifest.Files {
		hash, err := fileDigest(filepath.Join(pending, "files", file.Path))
		if err != nil {
			return fmt.Errorf("reading staged archive %s: %w", file.Path, err)
		}
		if hash != file.SHA256 {
			return fmt.Errorf("staged archive %s failed digest validation", file.Path)
		}
		current, err := fileDigest(file.Path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspecting archive destination %s: %w", file.Path, err)
		}
		if file.Versions {
			if current != file.BeforeSHA256 && current != file.SHA256 {
				return fmt.Errorf("%s changed since staging; inspect before resuming", file.Path)
			}
		} else if err == nil && current != file.SHA256 {
			return fmt.Errorf("archive destination %s conflicts with the staged release; inspect before resuming", file.Path)
		}
	}
	for _, file := range manifest.Files {
		if err := publishArchiveFile(pending, file); err != nil {
			return err
		}
	}
	if err := validatePublishedArchive(manifest); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	_, marker := archivePaths(manifest.Folder)
	if err := atomicArchiveWrite(marker, data); err != nil {
		return fmt.Errorf("publishing completion marker: %w", err)
	}
	if err := os.RemoveAll(pending); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(pending))
}

func publishArchiveFile(pending string, file archiveFile) error {
	data, err := os.ReadFile(filepath.Join(pending, "files", file.Path))
	if err != nil {
		return err
	}
	return atomicArchiveWrite(file.Path, data)
}

func atomicArchiveWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".bump-version-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	// Sync new directory entries as well as the file, up to precompiles/.
	for current := dir; current != "."; current = filepath.Dir(current) {
		if err := syncDirectory(current); err != nil {
			return err
		}
	}
	return nil
}

func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

func syncArchiveTree(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		return errors.Join(f.Sync(), f.Close())
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := syncDirectory(dirs[i]); err != nil {
			return err
		}
	}
	return nil
}

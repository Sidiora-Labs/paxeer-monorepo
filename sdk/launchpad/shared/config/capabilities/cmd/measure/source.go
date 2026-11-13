package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/util"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func requiredSourcePaths() []string {
	return []string{
		"sdk/launchpad/shared/config/capabilities/measurement.go",
		"sdk/launchpad/shared/config/capabilities/cmd/measure/main.go",
		"sdk/launchpad/shared/config/capabilities/cmd/measure/source.go",
		"sdk/launchpad/shared/config/capabilities/catalog.go",
		"sdk/launchpad/shared/config/capabilities/preflight.go",
		"sdk/launchpad/shared/config/deployment/attest.go",
		"sdk/launchpad/shared/config/deployment/manifest.go",
		"sdk/launchpad/shared/config/deployment/revision.go",
		"sdk/launchpad/shared/config/deployment/public.go",
		"sdk/launchpad/shared/config/deployment/current.json",
		"sdk/launchpad/shared/config/capabilities/frozen.json",
		"sdk/launchpad/shared/config/capabilities/contract-support.json",
		"sdk/launchpad/shared/chain/readpolicy/policy.go",
		"sdk/launchpad/shared/go.mod",
		"sdk/launchpad/shared/go.sum",
		"spec/layerx-merge/spec.kvx",
		"spec/layerx-merge/contracts/launchpad.md",
		"sdk/launchpad/protocol/addresses.go",
		"apps/launchpad/web/src/core/network/deployment.ts",
	}
}

func writeSourceIndex(root, output string, additional []string) error {
	if root == "" || output == "" {
		return deployment.Issue{Code: valueCandidateSourceIndexInvalid}
	}
	paths := append(requiredSourcePaths(), additional...)
	if len(paths) > 10000 {
		return deployment.Issue{Code: valueCandidateSourceIndexInvalid}
	}
	index := sourceIndex{SchemaVersion: 1}
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || path == ".." || strings.HasPrefix(path, "../") || seen[path] {
			return deployment.Issue{Code: "candidate_source_path_invalid"}
		}
		seen[path] = true
		file, err := util.OpenPath(filepath.Join(root, path))
		if err != nil {
			return deployment.Issue{Code: valueCandidateSourceChanged}
		}
		hash := sha256.New()
		count, readErr := io.Copy(hash, io.LimitReader(file, (64<<20)+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || count > 64<<20 {
			return deployment.Issue{Code: valueCandidateSourceChanged}
		}
		index.Files = append(index.Files, sourceFile{Path: path, SHA256: hex.EncodeToString(hash.Sum(nil))})
	}
	if err := validateSource(root, index); err != nil {
		return err
	}
	file, err := util.OpenFilePath(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return deployment.Issue{Code: "candidate_source_index_output_unavailable"}
	}
	defer func() {
		if cleanupErr := file.Close(); cleanupErr != nil {
			log.Printf("cleanup file.Close: %v", cleanupErr)
		}
	}()
	if err := json.NewEncoder(file).Encode(index); err != nil {
		return deployment.Issue{Code: "candidate_source_index_write_failed"}
	}
	if err := file.Sync(); err != nil {
		return deployment.Issue{Code: "candidate_source_index_sync_failed"}
	}
	return nil
}

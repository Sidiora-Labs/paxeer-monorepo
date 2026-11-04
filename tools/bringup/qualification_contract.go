package bringup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
)

// FoundationInvocation is the provenance emitted by Foundation.producer. argv
// remains an array: shell quoting must never change the recorded invocation.
type FoundationInvocation struct {
	Argv     []string `json:"argv"`
	ExitCode *int     `json:"exit_code"`
	Stdout   string   `json:"stdout_sha256"`
	Stderr   string   `json:"stderr_sha256"`
}

type FoundationArtifact struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// QualifyFoundation is shared by the producer and both material consumers.
// It checks the caller's independently calculated source closure and every
// produced byte before the consumer is allowed to install any material.
func QualifyFoundation(directory, revision, binding, consumer string, raw []byte) error {
	if consumer != "foundation" && consumer != "registry-material" && consumer != "policy-graph" {
		return errors.New("unknown foundation consumer")
	}
	if uniqueJSON(raw) != nil {
		return errors.New("foundation JSON refused")
	}
	var manifest struct {
		Version          int                           `json:"version"`
		Stage            string                        `json:"stage"`
		Purpose          string                        `json:"purpose"`
		Revision         string                        `json:"source_revision"`
		Binding          string                        `json:"source_binding"`
		Paths            []string                      `json:"source_paths"`
		Producers        []json.RawMessage             `json:"producers"`
		Inputs           map[string]string             `json:"inputs"`
		Artifacts        map[string]FoundationArtifact `json:"generated_artifacts"`
		ArtifactManifest string                        `json:"artifact_manifest"`
		ArtifactDigest   string                        `json:"artifact_manifest_sha256"`
	}
	hex40 := regexp.MustCompile(`^[0-9a-f]{40}$`)
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if json.Unmarshal(raw, &manifest) != nil || manifest.Version != 1 || manifest.Stage != "dependency-foundation" || manifest.Purpose != "disposable-test-only" || !hex40.MatchString(revision) || manifest.Revision != revision || !hex64.MatchString(binding) || manifest.Binding != binding || len(manifest.Paths) == 0 || len(manifest.Producers) == 0 || len(manifest.Artifacts) == 0 || len(manifest.Inputs) == 0 {
		return errors.New("foundation source or producer provenance refused")
	}
	paths := map[string]bool{}
	for _, path := range manifest.Paths {
		if !safeFoundationPath(path) || paths[path] {
			return errors.New("foundation source path refused")
		}
		paths[path] = true
	}
	for _, record := range manifest.Producers {
		var invocation FoundationInvocation
		decoder := json.NewDecoder(bytes.NewReader(record))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&invocation) != nil || invocation.ExitCode == nil || *invocation.ExitCode != 0 || len(invocation.Argv) == 0 || invocation.Argv[0] == "" || !hex64.MatchString(invocation.Stdout) || !hex64.MatchString(invocation.Stderr) {
			return errors.New("foundation invocation refused")
		}
		for _, arg := range invocation.Argv {
			if strings.ContainsRune(arg, 0) {
				return errors.New("foundation argv refused")
			}
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	for path, record := range manifest.Artifacts {
		if !safeFoundationPath(path) || !hex64.MatchString(record.SHA256) || record.Size <= 0 {
			return errors.New("foundation artifact record refused")
		}
		// OpenRoot confines traversal; Lstat additionally refuses linked artifacts.
		parts := strings.Split(path, "/")
		for i := range parts {
			info, e := root.Lstat(strings.Join(parts[:i+1], "/"))
			if e != nil || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("foundation artifact path refused")
			}
		}
		file, e := root.Open(path)
		if e != nil {
			return e
		}
		info, e := file.Stat()
		if e != nil || !info.Mode().IsRegular() || info.Size() != record.Size {
			file.Close()
			return errors.New("foundation artifact size refused")
		}
		digest := sha256.New()
		_, e = io.Copy(digest, file)
		file.Close()
		if e != nil || hex.EncodeToString(digest.Sum(nil)) != record.SHA256 {
			return errors.New("foundation artifact digest refused")
		}
	}
	for _, path := range manifest.Inputs {
		if _, ok := manifest.Artifacts[path]; !ok {
			return errors.New("foundation input has no digest binding")
		}
	}
	if !filepath.IsAbs(manifest.ArtifactManifest) || !hex64.MatchString(manifest.ArtifactDigest) {
		return errors.New("foundation artifact manifest binding refused")
	}
	artifact, err := os.Open(manifest.ArtifactManifest)
	if err != nil {
		return err
	}
	digest := sha256.New()
	_, err = io.Copy(digest, artifact)
	artifact.Close()
	if err != nil || hex.EncodeToString(digest.Sum(nil)) != manifest.ArtifactDigest {
		return errors.New("foundation artifact manifest changed")
	}
	if consumer != "foundation" {
		proof, err := root.ReadFile("qualification.json")
		if err != nil {
			return err
		}
		var qualification struct {
			Revision string `json:"source_revision"`
			Exit     *int   `json:"exit_code"`
			Tests    int    `json:"tests"`
			Skipped  *int   `json:"skipped"`
		}
		if uniqueJSON(proof) != nil || json.Unmarshal(proof, &qualification) != nil || qualification.Revision != revision || qualification.Exit == nil || *qualification.Exit != 0 || qualification.Tests <= 0 || qualification.Skipped == nil || *qualification.Skipped != 0 {
			return errors.New("foundation qualification refused")
		}
	}
	return nil
}

func safeFoundationPath(path string) bool {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || (path == "." || path == "..") || strings.HasPrefix(path, "../") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".env" || strings.HasPrefix(part, ".env.") || strings.Contains(part, ".env.") || strings.HasSuffix(part, ".env") {
			return false
		}
	}
	return true
}

type XwebReceiptKey struct {
	ID     string `json:"key_id"`
	Public string `json:"public_key"`
	First  uint64 `json:"first_height"`
	Last   uint64 `json:"last_height"`
}

// XwebReceiptHistory transports the node's complete governed key history into
// the configuration loader's encoding without discarding rotation ranges.
func XwebReceiptHistory(raw []byte) ([]XwebReceiptKey, error) {
	var history struct {
		Keys []struct {
			ID     string  `json:"keyId"`
			Public string  `json:"publicKey"`
			First  *uint64 `json:"firstHeight"`
			Last   *uint64 `json:"lastHeight"`
		} `json:"keys"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if uniqueJSON(raw) != nil || decoder.Decode(&history) != nil || len(history.Keys) == 0 {
		return nil, errors.New("receipt history unavailable or malformed")
	}
	keys := []XwebReceiptKey{}
	seen := map[string]bool{}
	for _, key := range history.Keys {
		public, e := hex.DecodeString(strings.TrimPrefix(key.Public, "0x"))
		id := strings.ToLower(strings.TrimPrefix(key.ID, "0x"))
		if !isHexWord(key.ID) || !isHexWord(key.Public) || id == strings.Repeat("0", 64) || seen[id] || e != nil || len(public) != 32 || !verify.PublicKeyIsCanonical([32]byte(public)) || key.First == nil || key.Last == nil || *key.First > *key.Last {
			return nil, errors.New("receipt history key or range refused")
		}
		seen[id] = true
		keys = append(keys, XwebReceiptKey{id, hex.EncodeToString(public), *key.First, *key.Last})
	}
	return keys, nil
}

// SearchFrontPin is the live-check consumer of RenderSearchFront's output.
func SearchFrontPin(selection SearchSelection, rendered []byte) (string, error) {
	text := regexp.MustCompile(`(?m)^\s*#.*$`).ReplaceAllString(string(rendered), "")
	contract := regexp.MustCompile(`^\s*split_clients\s+"\$\{xweb_client\}"\s+\$xweb_node\s*\{\s*\*\s+([a-z0-9.-]+);\s*\}\s*map\s+\$xweb_node\s+\$xweb_eligible\s*\{\s*default\s+0;\s*([a-z0-9.-]+)\s+([01]);\s*\}\s*$`).FindStringSubmatch(text)
	if selection.Primary == "" || selection.Backup == "" || selection.Primary == selection.Backup || len(contract) != 4 || contract[1] != contract[2] || contract[3] != "1" || (contract[1] != selection.Primary && contract[1] != selection.Backup) {
		return "", errors.New("search pin is unknown, ineligible or unready")
	}
	return contract[1], nil
}

func runQualificationInspector(ctx context.Context, args []string, input io.Reader, output io.Writer) (bool, error) {
	invalid := errors.New("invalid qualification arguments")
	switch args[0] {
	case "foundation-provenance":
		if len(args) != 5 {
			return true, invalid
		}
		raw, e := io.ReadAll(io.LimitReader(input, (16<<20)+1))
		if e != nil || len(raw) > 16<<20 {
			return true, invalid
		}
		if e = QualifyFoundation(args[1], args[2], args[3], args[4], raw); e != nil {
			return true, e
		}
		_, e = output.Write(raw)
		return true, e
	case "xweb-receipt-keys":
		if len(args) != 1 {
			return true, invalid
		}
		raw, e := io.ReadAll(io.LimitReader(input, 1<<20))
		if e != nil {
			return true, e
		}
		keys, e := XwebReceiptHistory(raw)
		if e != nil {
			return true, e
		}
		return true, json.NewEncoder(output).Encode(keys)
	case "search-pin":
		if len(args) != 3 {
			return true, invalid
		}
		raw, e := io.ReadAll(io.LimitReader(input, 65537))
		if e != nil || len(raw) > 65536 {
			return true, invalid
		}
		pin, e := SearchFrontPin(SearchSelection{args[1], args[2]}, raw)
		if e == nil {
			_, e = fmt.Fprintln(output, pin)
		}
		return true, e
	case "committed-transaction":
		if len(args) != 4 {
			return true, invalid
		}
		if !isHexWord(args[2]) {
			return true, invalid
		}
		client, e := liveHTTPClient(args[3])
		if e != nil {
			return true, e
		}
		defer client.CloseIdleConnections()
		client.Timeout = 15 * time.Second
		receipt, e := ReadCommittedTransaction(ctx, client, args[1], args[2])
		if e != nil {
			return true, e
		}
		return true, json.NewEncoder(output).Encode(map[string]any{"hash": receipt.Hash, "height": receipt.Height, "block": receipt.Block})
	}
	return false, nil
}

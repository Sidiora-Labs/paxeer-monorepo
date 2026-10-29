package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func copyConformanceRepository(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	paths := []string{
		"sdk/platform/conformance/operations.json",
		"sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json",
		"sdk/platform/conformance/fixtures/vector-v2-receipted-revert.json",
		"sdk/platform/conformance/fixtures/vector-v3-receipt-refusals.json",
		"sdk/platform/conformance/fixtures/vector-v4-committed.json",
		"sdk/platform/conformance/fixtures/vector-v5-portable.json",
	}
	err = filepath.WalkDir(filepath.Join(source, "services/human/schema/human-api/golden"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			relative, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			paths = append(paths, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(source, path))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestConformanceCommandPassesVectorFiles(t *testing.T) {
	repo := copyConformanceRepository(t)
	if err := runVectors(repo); err != nil {
		t.Fatalf("V1-V5 verification must match every expected result: %v", err)
	}
	command := exec.Command("go", "run", ".", "-repo", repo)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("conformance command must exit 0 over V1-V5: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "receipt_vector_files=5") {
		t.Fatalf("missing completion report for the five vector files: %s", output)
	}
}

func TestConformanceCommandFailsOnTamperedVector(t *testing.T) {
	repo := copyConformanceRepository(t)
	path := filepath.Join(repo, "sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Cases []struct {
			ID          string `json:"id"`
			ReceiptJSON struct {
				Receipt string `json:"receipt"`
			} `json:"receiptJson"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	if len(vector.Cases) != 4 {
		t.Fatalf("expected four V1 cases, got %d", len(vector.Cases))
	}
	c := vector.Cases[0]
	if !strings.HasPrefix(c.ReceiptJSON.Receipt, "0x") {
		t.Fatal("V1 receipt is not 0x-prefixed")
	}
	receipt, err := hex.DecodeString(c.ReceiptJSON.Receipt[2:])
	if err != nil || len(receipt) == 0 {
		t.Fatalf("invalid V1 receipt: %v", err)
	}
	receipt[len(receipt)-1] ^= 1
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	var cases []map[string]json.RawMessage
	if err := json.Unmarshal(document["cases"], &cases); err != nil {
		t.Fatal(err)
	}
	var view map[string]json.RawMessage
	if err := json.Unmarshal(cases[0]["receiptJson"], &view); err != nil {
		t.Fatal(err)
	}
	view["receipt"], err = json.Marshal("0x" + hex.EncodeToString(receipt))
	if err != nil {
		t.Fatal(err)
	}
	cases[0]["receiptJson"], err = json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	document["cases"], err = json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "run", ".", "-repo", repo)
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("tampered V1 receipt must exit 1, got %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "layerx-go-conformance: vector-v1-receipted-positive: "+c.ID+":") {
		t.Fatalf("failure did not identify the tampered V1 case: %s", output)
	}
	if strings.Contains(string(output), "receipt_vector_files=5") {
		t.Fatalf("tampered vectors produced a completion report: %s", output)
	}
}

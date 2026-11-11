package runtimedocs

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestRepair_22_39_328(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "sdk", "launchpad", "protocol", "catalog", "reviewed.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := deployment.Current()
	catalog, err := TrimCatalog(raw, manifest)
	if err != nil {
		t.Fatalf("reviewed catalogue must trim for the current deployment: %v", err)
	}
	var full, trimmed reviewedCatalog
	if err := json.Unmarshal(raw, &full); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(catalog.JSON, &trimmed); err != nil {
		t.Fatal(err)
	}
	if full.ManifestSHA256 != manifest.SourceManifestSHA256 || trimmed.ManifestSHA256 != manifest.SourceManifestSHA256 || trimmed.SchemaVersion != 1 || trimmed.ChainID != 125 {
		t.Fatal("reviewed and trimmed catalogues must bind the current manifest exactly")
	}
	wanted := []string{"EventEmitter", "FeeAccumulator", "FeesRouter", "PoolBeacon", "PoolRegistry", "ProtocolConfig", "Quoter", "Router", "SidioraERC20", "SidioraFactory", "SidioraNFT", "SidioraPool", "Treasury"}
	if len(trimmed.Contracts) != len(wanted) || len(catalog.Revisions) != len(wanted) || len(catalog.ERC20Ranges) == 0 {
		t.Fatalf("trimmed contract or immutable provenance missing: contracts=%d revisions=%d ranges=%v", len(trimmed.Contracts), len(catalog.Revisions), catalog.ERC20Ranges)
	}
	reviewed := map[string]reviewedContract{}
	for _, rawContract := range full.Contracts {
		var contract reviewedContract
		if err := json.Unmarshal(rawContract, &contract); err != nil {
			t.Fatal(err)
		}
		reviewed[contract.Name] = contract
	}
	for i, rawContract := range trimmed.Contracts {
		var contract reviewedContract
		if err := json.Unmarshal(rawContract, &contract); err != nil {
			t.Fatal(err)
		}
		original, ok := reviewed[contract.Name]
		if !ok || contract.Name != wanted[i] || contract.ABIRevision == "" || catalog.Revisions[contract.Name] != original.ABIRevision || contract.ABIRevision != original.ABIRevision || contract.SourceDigest != original.SourceDigest || contract.CompilerDigest != original.CompilerDigest || contract.CompilerVersion != original.CompilerVersion {
			t.Fatalf("trimmed provenance differs from reviewed contract %s", contract.Name)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, original.ABI); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(contract.ABI, compact.Bytes()) {
			t.Fatalf("trimmed ABI differs for %s", contract.Name)
		}
	}
	if _, err := decode.NewRegistry(catalog.JSON, nil); err != nil {
		t.Fatalf("real decoder rejected the trimmed catalogue: %v", err)
	}
	var inputs struct {
		CurrentReviewedCatalogSHA256 string `json:"currentReviewedCatalogSHA256"`
	}
	pins, err := os.ReadFile(filepath.Join(root, "sdk", "launchpad", "protocol", "generator-inputs", "phase2-inputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(pins, &inputs); err != nil {
		t.Fatal(err)
	}
	if inputs.CurrentReviewedCatalogSHA256 != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatal("generator input must pin the actual reviewed catalogue bytes")
	}

	// Decision 3177: preserve the original review for historical RPC evidence;
	// the recorded packet is re-recorded after the upgrade (decision 3119).
	historical, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("historical deployment must remain approved")
	}
	prior, err := os.ReadFile(filepath.Join(root, "sdk", "launchpad", "protocol", "catalog", "history", "reviewed-a35ff7c6f532d2f9.json"))
	if err != nil {
		t.Fatal(err)
	}
	var oldReview, newReview map[string]any
	if err := json.Unmarshal(prior, &oldReview); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &newReview); err != nil {
		t.Fatal(err)
	}
	if oldReview["manifestSha256"] != historical.SourceManifestSHA256 || fmt.Sprintf("%x", sha256.Sum256(prior)) != "fee302f74c970e71d9eec27f704341bd007e80e36e99f6a9855dee731b912716" {
		t.Fatal("historical review bytes changed")
	}
	oldReview["manifestSha256"] = manifest.SourceManifestSHA256
	if !reflect.DeepEqual(oldReview, newReview) {
		t.Fatal("catalogue rebind must change only manifestSha256")
	}
	if _, err := TrimCatalog(raw, historical); err == nil {
		t.Fatal("current review accepted for a different source manifest")
	}
	if _, err := TrimCatalog(prior, historical); err != nil {
		t.Fatalf("original review must still trim for approved historical evidence: %v", err)
	}
}

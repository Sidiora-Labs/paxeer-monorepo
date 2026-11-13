package receipt

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func TestHistoricalEmitterEpochsCatalogAdmission(t *testing.T) {
	manifest, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("approved historical deployment required")
	}
	raw, err := os.ReadFile("../../../../../sdk/launchpad/protocol/catalog/history/reviewed-a35ff7c6f532d2f9.json")
	if err != nil {
		t.Fatal(err)
	}
	epochs, err := historicalEmitterEpochs(manifest, raw)
	if err != nil || len(epochs) != 1 {
		t.Fatalf("reviewed Treasury epoch: %v %v", epochs, err)
	}
	treasury := epochs[0]
	if treasury.Deployment.Role != "Treasury" || treasury.Deployment.Address != common.HexToAddress("0xBb82d74E071e03d8E68F1d78Fae9EFe8b6b7A723") || treasury.Identity.Kind != "erc1967" || treasury.Identity.ImplementationHash != "0xc30ea8fb9d01825692b148659cc2a14af5759f962d14d2216498ed618bca5244" {
		t.Fatalf("supplemental epoch differs from approved producer: %+v", treasury)
	}
	var catalog decode.Catalog
	if err = json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	kept := catalog.Contracts[:0]
	for _, contract := range catalog.Contracts {
		if contract.Name != "Treasury" {
			kept = append(kept, contract)
		}
	}
	catalog.Contracts = kept
	trimmed, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if epochs, err = historicalEmitterEpochs(manifest, trimmed); err != nil || len(epochs) != 0 {
		t.Fatalf("non-decoded Treasury must stay opaque: %v %v", epochs, err)
	}
	catalog.ManifestSHA256 = strings.Repeat("0", 64)
	changed, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = historicalEmitterEpochs(manifest, changed); err == nil {
		t.Fatal("unapproved catalog admitted")
	}
	manifest.SourceManifestSHA256 = strings.Repeat("0", 64)
	if _, err = historicalEmitterEpochs(manifest, raw); err == nil {
		t.Fatal("unapproved deployment source admitted")
	}
}

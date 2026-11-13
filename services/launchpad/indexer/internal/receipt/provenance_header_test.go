package receipt

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestProvenanceHeaderDigestRecordedCreation(t *testing.T) {
	raw, err := os.ReadFile("../processor/testdata/repair_m1000_batches.json")
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Responses map[string]struct{ Result json.RawMessage }
	}
	if err = json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	recorded := packet.Responses[`eth_getBlockByNumber ["0x2063ec4",false]`].Result
	var header types.Header
	if err = json.Unmarshal(recorded, &header); err != nil {
		t.Fatal(err)
	}
	if header.Number.Uint64() != 33963716 || header.Hash() != common.HexToHash("0xe83bd786ee2456fe683b1cc6a7c4b9755783d09071d189ec20dc5a565c6f6292") {
		t.Fatal("recorded creation header changed")
	}
	raw, err = os.ReadFile("../../../../../services/launchpad/pnl-tracker/internal/production/l16/testdata/indexer_real_market/creation.json")
	if err != nil {
		t.Fatal(err)
	}
	var producer struct {
		Creations []struct{ Provenance accounting.Provenance } `json:"indexer.indexer_canonical_creations"`
	}
	if err = json.Unmarshal(raw, &producer); err != nil || len(producer.Creations) != 1 {
		t.Fatalf("recorded creation provenance: %v", err)
	}
	want := producer.Creations[0].Provenance.HeaderSHA256
	if want != "1b7f1e594d0e4acb937e4e4d368eff29440fd385c27a49f0bb932c83bccbb6ae" {
		t.Fatal("original producer digest changed")
	}
	for _, accessList := range []string{"absent", "null"} {
		t.Run(accessList, func(t *testing.T) {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(recorded, &fields); err != nil {
				t.Fatal(err)
			}
			if accessList == "null" {
				fields["blockAccessListHash"] = json.RawMessage("null")
			}
			wire, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			var decoded types.Header
			if err = json.Unmarshal(wire, &decoded); err != nil {
				t.Fatal(err)
			}
			got, err := provenanceHeaderDigest(&decoded)
			if err != nil || hex.EncodeToString(got[:]) != want {
				t.Fatalf("recorded producer digest: %x, want %s: %v", got, want, err)
			}
		})
	}
	original, err := provenanceHeaderDigest(&header)
	if err != nil {
		t.Fatal(err)
	}
	accessList := common.HexToHash("0x01")
	header.BlockAccessListHash = &accessList
	changed, err := provenanceHeaderDigest(&header)
	if err != nil || changed == original {
		t.Fatal("access-list hash is not bound by provenance")
	}
	header.BlockAccessListHash = nil
	slot := uint64(1)
	header.SlotNumber = &slot
	changed, err = provenanceHeaderDigest(&header)
	if err != nil || changed == original {
		t.Fatal("slot number is not bound by provenance")
	}
	if _, err = provenanceHeaderDigest(nil); err != ErrQualification {
		t.Fatalf("nil header accepted: %v", err)
	}
}

func TestHistoricalProvenanceRecordedCodecs(t *testing.T) {
	input := repair53InputsFromProducer(t)
	var market DiscoveredMarket
	for _, candidate := range input.Markets {
		if candidate.Height == 33963716 {
			market = candidate
		}
	}
	if market.Creation == nil || market.Creation.Provenance.HeaderSHA256 != "472cabba2d256eff4f23f613e5b9e48164ff1e01edac2e55ec864abf73d4bb2c" {
		t.Fatal("re-recorded producer creation required")
	}
	raw, err := os.ReadFile("../../../../../services/launchpad/pnl-tracker/internal/production/l16/testdata/indexer_real_market/creation.json")
	if err != nil {
		t.Fatal(err)
	}
	var original struct {
		Creations []struct{ Provenance accounting.Provenance } `json:"indexer.indexer_canonical_creations"`
	}
	if err = json.Unmarshal(raw, &original); err != nil || len(original.Creations) != 1 {
		t.Fatalf("original durable producer creation: %v", err)
	}
	legacy := original.Creations[0].Provenance
	if legacy.HeaderSHA256 != "1b7f1e594d0e4acb937e4e4d368eff29440fd385c27a49f0bb932c83bccbb6ae" {
		t.Fatal("original durable header encoding changed")
	}
	for _, recorded := range []struct {
		name       string
		provenance accounting.Provenance
	}{{"geth_1_17_3", legacy}, {"geth_1_17_4", market.Creation.Provenance}} {
		t.Run(recorded.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			persisted, err := clone(market)
			if err != nil {
				t.Fatal(err)
			}
			persisted.Creation.Provenance = recorded.provenance
			for restart := 0; restart < 2; restart++ {
				authority, closeClient := repair53Open(t, ctx, input)
				if err = authority.Rehydrate(ctx, []DiscoveredMarket{persisted}); err != nil {
					closeClient()
					t.Fatalf("rehydrate recorded creation after restart %d: %v", restart, err)
				}
				if restart == 0 {
					for _, field := range []string{"deployment", "abi", "emitter", "pool", "receipt", "header"} {
						corrupt, err := clone(persisted)
						if err != nil {
							t.Fatal(err)
						}
						proof := &corrupt.Creation.Provenance
						switch field {
						case "deployment":
							proof.DeploymentID = input.Options.Manifest.DeploymentVersion
						case "abi":
							proof.ABIRevision = strings.Repeat("0", 64)
						case "emitter":
							proof.EmitterCodeEpoch = strings.Repeat("0", 64)
						case "pool":
							proof.PoolCodeEpoch = strings.Repeat("0", 64)
						case "receipt":
							proof.ReceiptSHA256 = strings.Repeat("0", 64)
						case "header":
							proof.HeaderSHA256 = strings.Repeat("0", 64)
						}
						if err = authority.Rehydrate(ctx, []DiscoveredMarket{corrupt}); !errors.Is(err, ErrQualification) {
							closeClient()
							t.Fatalf("altered %s provenance admitted: %v", field, err)
						}
					}
				}
				closeClient()
			}
		})
	}
}

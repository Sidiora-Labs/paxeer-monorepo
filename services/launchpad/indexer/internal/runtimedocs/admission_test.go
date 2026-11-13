package runtimedocs

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func TestRuntimeCatalogRequiresReviewedContractsAndImmutableProvenance(t *testing.T) {
	manifest, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("approved source absent")
	}
	body, err := os.ReadFile("../../../../../sdk/launchpad/protocol/catalog/history/reviewed-a35ff7c6f532d2f9.json")
	if err != nil {
		t.Fatal(err)
	}
	var original reviewedCatalog
	if err = json.Unmarshal(body, &original); err != nil {
		t.Fatal(err)
	}
	baseline, err := TrimCatalog(body, manifest)
	if err != nil || len(baseline.ERC20Ranges) == 0 {
		t.Fatalf("reviewed catalog: %v", err)
	}
	for _, mode := range []string{"malformed", "contract shape", "missing ABI", "duplicate contract", "missing token provenance"} {
		t.Run(mode, func(t *testing.T) {
			var catalog reviewedCatalog
			if e := json.Unmarshal(body, &catalog); e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "contract shape":
				catalog.Contracts = []json.RawMessage{json.RawMessage(`"contract"`)}
			case "missing ABI":
				catalog.Contracts = []json.RawMessage{json.RawMessage(`{"name":"SidioraERC20"}`)}
			case "duplicate contract":
				for _, raw := range original.Contracts {
					var c reviewedContract
					if e := json.Unmarshal(raw, &c); e != nil {
						t.Fatal(e)
					}
					if c.Name == "SidioraERC20" {
						catalog.Contracts = append(catalog.Contracts, raw)
						break
					}
				}
			case "missing token provenance":
				catalog.Contracts = nil
			}
			changed, e := json.Marshal(catalog)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "malformed" {
				changed = []byte("{")
			}
			if _, e = TrimCatalog(changed, manifest); e == nil {
				t.Fatal("incomplete reviewed provenance admitted")
			}
		})
	}
	for _, ranges := range [][]receipt.CodeRange{{{Start: -1, Length: 1}}, {{Start: 0, Length: 0}}, {{Start: 2, Length: 2}}} {
		if _, err = MaskedRuntimeHash([]byte{1, 2, 3}, ranges); err == nil {
			t.Fatal("invalid immutable span admitted")
		}
	}
	code := make([]byte, 128<<10)
	base := Inputs{Manifest: manifest, ReviewedCatalog: body, ERC20RuntimeCode: code, PoolRuntimeHash: common.HexToHash("0x1"), Anchor: receipt.BlockAnchor{Height: manifest.ActivationHeight, Hash: common.HexToHash("0x2")}}
	for _, mode := range []string{"chain", "catalog", "short code", "inactive", "native identity"} {
		t.Run(mode, func(t *testing.T) {
			in := base
			in.Manifest = deployment.Manifest{}
			encoded, e := json.Marshal(base.Manifest)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(encoded, &in.Manifest); e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "chain":
				in.Manifest.ChainID = 1
			case "catalog":
				in.ReviewedCatalog = []byte("{")
			case "short code":
				in.ERC20RuntimeCode = []byte{1}
			case "inactive":
				in.Manifest.Status = "retired"
			case "native identity":
				in.Manifest.Roles[deployment.LaunchpadPrecompileRole] = deployment.Role{Kind: "direct", Address: deployment.LaunchpadPrecompileAddress}
			}
			if _, _, e = ReceiptOptions(in); e == nil {
				t.Fatal("unbound receipt inputs admitted")
			}
		})
	}
	options, _, err := ReceiptOptions(base)
	if err != nil {
		t.Fatal(err)
	}
	options.Anchor = nil
	if _, err = CommittedOptions(options); err == nil || !strings.Contains(err.Error(), "anchor required") {
		t.Fatalf("unanchored committed source: %v", err)
	}
}

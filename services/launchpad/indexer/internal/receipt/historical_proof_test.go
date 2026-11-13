package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func TestHistoricalProofRequiresOriginalCoordinatesAndProvenance(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	market := input.Markets[0]
	historical, err := a.historicalAuthority(ctx, market.Height)
	if err != nil {
		t.Fatal(err)
	}
	if err = historical.proveHistoricalMarket(ctx, market); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*DiscoveredMarket)
	}{
		{"missing proof", func(m *DiscoveredMarket) { m.Creation = nil }},
		{"unavailable transaction", func(m *DiscoveredMarket) { m.Creation.TransactionHash = common.HexToHash("0xbad") }},
		{"block", func(m *DiscoveredMarket) { m.Creation.BlockHash = common.HexToHash("0xbad") }},
		{"height", func(m *DiscoveredMarket) { m.Height++ }},
		{"transaction index", func(m *DiscoveredMarket) { m.TransactionIndex++ }},
		{"log index", func(m *DiscoveredMarket) { m.Creation.LogIndex = ^uint(0) }},
		{"token", func(m *DiscoveredMarket) { m.Token = common.HexToAddress("0xbad") }},
		{"pool", func(m *DiscoveredMarket) { m.Pool = common.HexToAddress("0xbad") }},
		{"source digest", func(m *DiscoveredMarket) { m.Creation.Provenance.ReceiptSHA256 = strings.Repeat("0", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed, e := clone(market)
			if e != nil {
				t.Fatal(e)
			}
			tc.edit(&changed)
			if e = historical.proveHistoricalMarket(ctx, changed); e == nil {
				t.Fatal("changed historical creation admitted")
			}
		})
	}
	changed, err := clone(input)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for key, response := range changed.Responses {
		if strings.HasPrefix(key, "eth_getBlockByNumber ") && strings.Contains(key, fmt.Sprintf(`"0x%x"`, a.options.Anchor.Height)) {
			response.Result = nil
			response.Error = json.RawMessage(`{"code":-32000,"message":"header pruned"}`)
			changed.Responses[key] = response
			count++
		}
	}
	if count == 0 {
		t.Fatal("recorded anchor response absent")
	}
	changed = repair53Replay(t, changed)
	client, closeChanged, err := NewBoundedRPCClient(ctx, changed.Endpoint, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeChanged()
	a.client = client
	a.identityVerified = false
	if _, err = a.historicalAuthority(ctx, market.Height); !errors.Is(err, ErrAnchorUnavailable) {
		t.Fatalf("unattested historical authority: %v", err)
	}
	a.identityVerified = true
	if _, err = a.historicalAuthority(ctx, market.Height); err != nil {
		t.Fatalf("attested historical authority: %v", err)
	}
}

func TestHistoricalEmitterCatalogRequiresExactApprovedSource(t *testing.T) {
	input := repair53InputsFromProducer(t)
	manifest, err := approvedManifestAt(input.Markets[0].Height)
	if err != nil {
		t.Fatal(err)
	}
	var catalog decode.Catalog
	if err = json.Unmarshal(input.Options.CatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	var treasury decode.CatalogContract
	found := false
	for _, contract := range catalog.Contracts {
		if contract.Name == "Treasury" {
			treasury = contract
			found = true
		}
	}
	if !found {
		t.Fatal("approved treasury ABI absent")
	}
	for _, mode := range []string{"source digest", "malformed catalog", "catalog digest", "duplicate treasury", "opaque treasury", "manifest treasury"} {
		t.Run(mode, func(t *testing.T) {
			m, e := clone(manifest)
			if e != nil {
				t.Fatal(e)
			}
			c, e := clone(catalog)
			if e != nil {
				t.Fatal(e)
			}
			c.ManifestSHA256 = m.SourceManifestSHA256
			switch mode {
			case "source digest":
				m.SourceManifestSHA256 = strings.Repeat("0", 64)
			case "catalog digest":
				c.ManifestSHA256 = strings.Repeat("0", 64)
			case "duplicate treasury":
				c.Contracts = append(c.Contracts, treasury)
			case "opaque treasury":
				c.Contracts = nil
			case "manifest treasury":
				m.Roles["Treasury"] = deployment.Role{}
			}
			body, e := json.Marshal(c)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "malformed catalog" {
				body = []byte("{")
			}
			epochs, e := historicalEmitterEpochs(m, body)
			if mode == "opaque treasury" || mode == "manifest treasury" {
				if e != nil || len(epochs) != 0 {
					t.Fatalf("opaque role epochs %+v: %v", epochs, e)
				}
			} else if !errors.Is(e, ErrQualification) {
				t.Fatalf("unbound historical catalog admitted: %v", e)
			}
		})
	}
}

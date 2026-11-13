package processor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
)

func TestCreationMetadataRequiresMatchingFieldsAndProvenance(t *testing.T) {
	for _, field := range []string{"token", "pool", "creator", "optical", "nftId", "factory-proof", "factory-abi", "name", "symbol", "decimals", "totalSupply", "decimal-bound", "supply-bound", "mirror-proof", "mirror-abi", "timestamp", "duplicate"} {
		t.Run(field, func(t *testing.T) {
			market := &decode.DecodedEnvelope{Raw: decode.RawLog{Identity: decode.RawIdentity{TransactionHash: "0x" + strings.Repeat("1", 64), BlockHash: "0x" + strings.Repeat("2", 64)}, Emitter: "0x" + strings.Repeat("3", 40), Order: decode.Order{Block: 42}}, ABIRevision: "factory-r1", EmitterRole: "SidioraFactory", EventName: "MarketCreated", Event: &decode.Event{Args: map[string]any{"token": "token", "pool": "pool", "creator": "creator", "optical": "0", "nftId": "7"}}}
			mirror := &decode.DecodedEnvelope{Raw: market.Raw, ABIRevision: "emitter-r1", EmitterRole: "EventEmitter", EventName: "TokenDeployed", Event: &decode.Event{Args: map[string]any{"token": "token", "pool": "pool", "creator": "creator", "name": "Kindle", "symbol": "KDL", "decimals": "18", "totalSupply": "1000", "timestamp": "1700000000", "blockNumber": "42"}}}
			mirror.Raw.Emitter = "0x" + strings.Repeat("4", 40)
			prove := func(address common.Address) (accounting.Provenance, error) {
				if strings.EqualFold(address.Hex(), market.Raw.Emitter) {
					if field == "factory-proof" {
						return accounting.Provenance{}, store.ErrIntegrity
					}
					if field == "factory-abi" {
						return accounting.Provenance{ABIRevision: "other"}, nil
					}
					return accounting.Provenance{ABIRevision: "factory-r1"}, nil
				}
				if field == "mirror-proof" {
					return accounting.Provenance{}, store.ErrIntegrity
				}
				if field == "mirror-abi" {
					return accounting.Provenance{ABIRevision: "other"}, nil
				}
				return accounting.Provenance{ABIRevision: "emitter-r1"}, nil
			}
			switch field {
			case "token", "pool", "creator", "optical", "nftId":
				market.Event.Args[field] = nil
			case "name", "symbol", "decimals", "totalSupply":
				mirror.Event.Args[field] = nil
			case "decimal-bound":
				mirror.Event.Args["decimals"] = "256"
			case "supply-bound":
				mirror.Event.Args["totalSupply"] = "-1"
			case "timestamp":
				mirror.Event.Args["timestamp"] = "1700000001"
			}
			events := []*decode.DecodedEnvelope{nil, {}, market, mirror}
			if field == "duplicate" {
				events = append(events, mirror)
			}
			if _, err := creationFacts(events, 1700000000, prove); err == nil {
				t.Fatal("invalid creation metadata accepted")
			}
		})
	}
	if tokenMirror(nil) || tokenMirror(&decode.DecodedEnvelope{}) {
		t.Fatal("absent token mirror accepted")
	}
	for _, refusal := range []error{inventoryIneligible("not-same-owner"), inventoryPending("baseline-unissued")} {
		if !errors.Is(refusal, errInventoryIneligible) || !strings.Contains(refusal.Error(), "inventory enrichment ineligible") {
			t.Fatalf("inventory refusal %v", refusal)
		}
	}
}

func TestQualifiedHelpersRejectUnissuedAndIncompleteInputs(t *testing.T) {
	ctx := context.Background()
	q := new(receipt.QualifiedReceipt)
	if _, err := qualifiedCreationFacts(ctx, []*receipt.QualifiedReceipt{q}); err == nil {
		t.Fatal("unissued creation receipt accepted")
	}
	if _, err := qualifiedEventsWithActors(ctx, q); err == nil {
		t.Fatal("unissued actor receipt accepted")
	}
	if _, err := qualifiedSegment(ctx, "stream", store.CanonicalBatch{}, nil); err == nil {
		t.Fatal("empty source segment accepted")
	}
	if _, err := qualifiedSegment(ctx, "stream", fixtureBatch(t), []*receipt.QualifiedReceipt{q}); err == nil {
		t.Fatal("unissued source segment accepted")
	}
	p := New(Deps{})
	if err := p.projectQualifiedLegacy(ctx, nil, store.CanonicalBatch{}, nil, nil); err == nil {
		t.Fatal("missing legacy store accepted")
	}
	if _, err := p.readSwapReserves(ctx, []accounting.FillV1{{}}); err == nil {
		t.Fatal("missing reserve source accepted")
	}
	if v := p.resolveNftID(ctx, "pool"); v != nil {
		t.Fatalf("absent registry returned nft %v", v)
	}
	if v := p.resolveNftID(ctx, "POOL"); v != nil || len(p.nftCache) != 1 {
		t.Fatalf("registry absence cache %v", p.nftCache)
	}
}

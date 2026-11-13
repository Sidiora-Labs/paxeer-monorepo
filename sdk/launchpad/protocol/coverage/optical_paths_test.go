package coverage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/catalog"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestReviewedOpticalEventsAdvanceCanonicalInvalidation(t *testing.T) {
	base, _, _ := sourceRegistry(t)
	raw, err := os.ReadFile("../catalog/testdata/repair-m924/review.json")
	if err != nil {
		t.Fatal(err)
	}
	var review catalog.OpticalRegistryReview
	if err := json.Unmarshal(raw, &review); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	address := common.HexToAddress("0x6789")
	registry, err := base.WithReviewedOpticalRegistry([]catalog.Deployment{{ChainID: 125, Address: address, Family: review.Bundle.Name, Version: review.Bundle.ABIHash, Attested: true}}, raw, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(review.Bundle.ABI))
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewLifecycle(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := NewProjection(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	optical := common.HexToAddress("0x789a")
	invalidated := eventLog(t, address, parsed.Events["OpticalDeregistered"], map[string]any{"optical": optical, "timestamp": big.NewInt(12)})
	invalidResult := correlateInput(t, registry, invalidated)
	if len(invalidResult.Effects) != 1 {
		t.Fatal("reviewed optical invalidation effect absent")
	}
	invalidResult.Effects[0].Fields = copyFields(invalidResult.Effects[0].Fields)
	invalidResult.Effects[0].Fields["optical"] = common.Address{}
	invalidResult.seal = receiptSeal(invalidResult)
	if err := projection.ApplyReceipt(invalidResult); err == nil || len(projection.Lifecycle.InvalidatedOpticals) != 0 {
		t.Fatal("optical invalidation without an identity changed lifecycle")
	}
	apply := func(block uint64, event string) error {
		t.Helper()
		log := eventLog(t, address, parsed.Events[event], map[string]any{"optical": optical, "timestamp": big.NewInt(int64(block))})
		header, tx, receipt := receiptInputAt(t, block, log)
		result, err := (&Correlator{Registry: registry}).Correlate(context.Background(), header, tx, receipt)
		if err != nil {
			return err
		}
		return projection.ApplyReceipt(result)
	}
	if err := apply(12, "OpticalDeregistered"); err != nil {
		t.Fatal(err)
	}
	if projection.Lifecycle.InvalidatedOpticals[optical].Block != 12 {
		t.Fatal("reviewed optical invalidation absent")
	}
	if err := apply(12, "OpticalMetadataUpdated"); err == nil {
		t.Fatal("out of order optical change accepted")
	}
	if err := apply(13, "OpticalMetadataUpdated"); err != nil || projection.Lifecycle.InvalidatedOpticals[optical].Block != 13 {
		t.Fatalf("later reviewed optical change absent: %v", err)
	}
}

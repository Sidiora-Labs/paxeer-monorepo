package control

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestChainVerifierChecksReportedBlockAndEvidenceShape(t *testing.T) {
	ctx := context.Background()
	if verifier, err := NewChainVerifier(nil, custody, fixtureManifest, nil, nil); !errors.Is(err, Invalid) || verifier != nil {
		t.Fatalf("chain verifier without client: %+v, %v", verifier, err)
	}
	verifier := chain(t, nil)
	if _, err := verifier.Inventory(ctx, Asset{}); !errors.Is(err, Invalid) {
		t.Fatalf("invalid inventory asset: %v", err)
	}
	if _, _, err := verifier.reportedBlock(ctx, big.NewInt(999999999)); !errors.Is(err, Unavailable) {
		t.Fatalf("absent block: %v", err)
	}
	header, reported, err := verifier.reportedBlock(ctx, nil)
	if err != nil || header == nil || reported == (common.Hash{}) {
		t.Fatalf("reported head: %v, %v", reported, err)
	}
	if err := verifier.atReported(ctx, header, common.HexToHash("0x"+strings.Repeat("f", 64))); !errors.Is(err, Denied) {
		t.Fatalf("different reported hash: %v", err)
	}
	obligation := Obligation{Asset: pax, Destination: beneficiary, Amount: "100"}
	if _, err := verifier.Settlement(ctx, obligation, "invalid"); !errors.Is(err, Invalid) {
		t.Fatalf("short receipt reference: %v", err)
	}
	if _, err := verifier.Settlement(ctx, obligation, "0x"+strings.Repeat("z", 64)); !errors.Is(err, Invalid) {
		t.Fatalf("nonhex receipt reference: %v", err)
	}
	if err := verifier.Award(ctx, Bounty{FundingSource: custody}, "evidence", beneficiary); !errors.Is(err, Unavailable) {
		t.Fatalf("award without reviewer authority: %v", err)
	}
}

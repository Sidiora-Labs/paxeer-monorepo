package accounting_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestSourcePositionOrdersCanonicalCoordinates(t *testing.T) {
	base := wireInitialPublication(t).Fill.Position
	for _, tc := range []struct {
		name string
		edit func(*accounting.SourcePosition)
		want int
	}{
		{"height", func(p *accounting.SourcePosition) { p.Height = "101" }, -1},
		{"transaction", func(p *accounting.SourcePosition) { p.TransactionIndex = "1" }, -1},
		{"log", func(p *accounting.SourcePosition) { p.LogIndex = "8" }, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			after := base
			tc.edit(&after)
			if got, err := accounting.CompareSourcePosition(base, after); err != nil || got != tc.want {
				t.Fatalf("forward order: %d, %v", got, err)
			}
			if got, err := accounting.CompareSourcePosition(after, base); err != nil || got != -tc.want {
				t.Fatalf("reverse order: %d, %v", got, err)
			}
		})
	}
	if got, err := accounting.CompareSourcePosition(base, base); err != nil || got != 0 {
		t.Fatalf("equal position: %d, %v", got, err)
	}
	changedHash := base
	changedHash.BlockHash = "0x" + strings.Repeat("c", 64)
	if _, err := accounting.CompareSourcePosition(base, changedHash); !errors.Is(err, accounting.ErrIdentity) {
		t.Fatalf("same coordinate changed block identity: %v", err)
	}
	invalid := base
	invalid.Height = "-1"
	if _, err := accounting.CompareSourcePosition(base, invalid); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("invalid coordinate: %v", err)
	}
}

func TestFillPricingAndEvidenceVariants(t *testing.T) {
	baseline := *wireInitialPublication(t).Fill
	priced := "5000000000"
	net := "4985000000"
	digest := accounting.Digest(strings.Repeat("a", 64))
	feeBPS := uint16(30)
	evidence := []accounting.EvidenceRef{{Kind: "receipt", ArtifactID: "receipt-1", ArtifactSHA256: string(digest)}}
	baseline.PricedInputRaw = &priced
	baseline.AMMInputAfterFeeRaw = &net
	baseline.InputDerivationQuality = "exact"
	baseline.Execution = accounting.ExecutionPolicy{Quality: "exact", Evidence: evidence, FeeBPS: &feeBPS, ConfigDigest: &digest, PreTransactionStateDigest: &digest, ReplayEvidenceDigest: &digest}
	if err := accounting.ValidateFillV1(baseline); err != nil {
		t.Fatalf("priced buy: %v", err)
	}
	changed := baseline
	changed.AMMInputAfterFeeRaw = &priced
	if err := accounting.ValidateFillV1(changed); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("net input arithmetic: %v", err)
	}
	changed = baseline
	badFeeBPS := uint16(31)
	changed.Execution.FeeBPS = &badFeeBPS
	if err := accounting.ValidateFillV1(changed); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("fee rate arithmetic: %v", err)
	}
	sell := *wireInitialPublication(t).Fill
	sell.Side = "sell"
	sell.PaidInputRaw = sell.TokenAmountRaw
	sell.FeeDisposition = "retained_token_reserve"
	if err := accounting.ValidateFillV1(sell); err != nil {
		t.Fatalf("unpriced sell: %v", err)
	}
	badEvidence := append([]accounting.EvidenceRef{}, evidence...)
	badEvidence = append(badEvidence, evidence[0])
	if err := accounting.ValidateEvidence(badEvidence); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("duplicate evidence: %v", err)
	}
	actor := baseline.Attribution
	actor.Quality = "exact_receipt"
	actor.Trader = &actor.TransactionSender
	actor.Evidence = evidence
	if err := accounting.ValidateAttribution(actor); err != nil {
		t.Fatalf("receipt actor: %v", err)
	}
	actor.Evidence = nil
	if err := accounting.ValidateAttribution(actor); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("unproven actor: %v", err)
	}
}

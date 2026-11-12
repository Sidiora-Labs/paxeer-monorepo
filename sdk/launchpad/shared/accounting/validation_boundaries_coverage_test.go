package accounting_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestFillRejectsChangedEconomicFactsAndProvenance(t *testing.T) {
	base := *wireInitialPublication(t).Fill
	for _, tc := range []struct {
		name string
		edit func(*accounting.FillV1)
		want string
	}{
		{"provenance", func(f *accounting.FillV1) { f.Provenance.HeaderSHA256 = "changed" }, "provenance"},
		{"source identity", func(f *accounting.FillV1) { f.SourceFillID = strings.Repeat("0", 64) }, "identity"},
		{"timestamp", func(f *accounting.FillV1) { f.BlockTimestampSeconds = "-1" }, "timestamp"},
		{"settlement", func(f *accounting.FillV1) { f.USDLAmountRaw = "0" }, "settlement amount"},
		{"paid input", func(f *accounting.FillV1) { f.PaidInputRaw = "0" }, "paid input"},
		{"fee", func(f *accounting.FillV1) { f.FeeRaw = "5000000001" }, "fee"},
		{"sell units", func(f *accounting.FillV1) { f.Side = "sell" }, "sell units"},
		{"side", func(f *accounting.FillV1) { f.Side = "other" }, "side"},
		{"net without priced input", func(f *accounting.FillV1) { value := "1"; f.AMMInputAfterFeeRaw = &value }, "net without priced input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := base
			tc.edit(&changed)
			err := accounting.ValidateFillV1(changed)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("changed fill = %v, want %s", err, tc.want)
			}
		})
	}
	priced := base
	price := "5000000000"
	net := "4985000000"
	digest := accounting.Digest(strings.Repeat("a", 64))
	evidence := []accounting.EvidenceRef{{Kind: "receipt", ArtifactID: "receipt-1", ArtifactSHA256: string(digest)}}
	priced.PricedInputRaw, priced.AMMInputAfterFeeRaw, priced.InputDerivationQuality = &price, &net, "exact"
	priced.Execution = accounting.ExecutionPolicy{Quality: "exact", Evidence: evidence, ConfigDigest: &digest, PreTransactionStateDigest: &digest, ReplayEvidenceDigest: &digest}
	for _, tc := range []struct {
		name string
		edit func(*accounting.FillV1)
		want string
	}{
		{"priced quality", func(f *accounting.FillV1) { f.InputDerivationQuality = "estimated" }, "priced input quality"},
		{"priced number", func(f *accounting.FillV1) { value := "01"; f.PricedInputRaw = &value }, "priced input"},
		{"net number", func(f *accounting.FillV1) { value := "01"; f.AMMInputAfterFeeRaw = &value }, "net input"},
		{"priced authority", func(f *accounting.FillV1) { f.Execution = base.Execution }, "priced input authority"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := priced
			tc.edit(&changed)
			err := accounting.ValidateFillV1(changed)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("changed priced fill = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestEvidenceAndExecutionRejectUnboundProofs(t *testing.T) {
	base := *wireInitialPublication(t).Fill
	digest := accounting.Digest(strings.Repeat("a", 64))
	evidence := accounting.EvidenceRef{Kind: "receipt", ArtifactID: "receipt-1", ArtifactSHA256: string(digest)}
	index := "7"
	evidence.LogIndex = &index
	if err := accounting.ValidateEvidence([]accounting.EvidenceRef{evidence}); err != nil {
		t.Fatalf("log-scoped receipt evidence = %v", err)
	}
	for _, refs := range [][]accounting.EvidenceRef{nil, {{Kind: "", ArtifactID: "receipt-1", ArtifactSHA256: string(digest)}}, {evidence, evidence}} {
		if err := accounting.ValidateEvidence(refs); !errors.Is(err, accounting.ErrInvalid) {
			t.Fatalf("invalid evidence = %v", err)
		}
	}
	actor := base.Attribution
	actor.Trader = &base.PoolAddress
	if err := accounting.ValidateAttribution(actor); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("unavailable attributed actor = %v", err)
	}
	actor = base.Attribution
	actor.Quality = "exact_receipt"
	actor.Evidence = []accounting.EvidenceRef{evidence}
	if err := accounting.ValidateAttribution(actor); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("exact actor without attributed party = %v", err)
	}
	policy := base.Execution
	policy.Quality = "exact"
	if err := accounting.ValidateExecutionPolicy(policy); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("exact execution without evidence = %v", err)
	}
	policy = base.Execution
	policy.Evidence = []accounting.EvidenceRef{evidence}
	if err := accounting.ValidateExecutionPolicy(policy); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("unavailable execution with evidence = %v", err)
	}
	policy = base.Execution
	policy.Quality = "unknown"
	if err := accounting.ValidateExecutionPolicy(policy); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("unknown execution quality = %v", err)
	}
}

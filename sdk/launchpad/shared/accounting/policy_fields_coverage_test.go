package accounting_test

import (
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestAccountingAttributionAndExecutionRequireBoundFieldIdentities(t *testing.T) {
	fill := *wireInitialPublication(t).Fill
	invalid := "invalid"
	actor := fill.Attribution
	actor.RouterCaller = &invalid
	if err := accounting.ValidateAttribution(actor); err == nil {
		t.Fatal("invalid attributed address admitted")
	}
	actor = fill.Attribution
	actor.Quality = "unknown"
	if err := accounting.ValidateAttribution(actor); err == nil {
		t.Fatal("unknown attribution quality admitted")
	}
	actor = fill.Attribution
	actor.Quality = "exact_receipt"
	if err := accounting.ValidateAttribution(actor); err == nil {
		t.Fatal("exact attribution without evidence admitted")
	}
	for _, mode := range []string{"evidence", "digest", "optical", "fee"} {
		policy := fill.Execution
		switch mode {
		case "evidence":
			policy.Evidence = []accounting.EvidenceRef{{Kind: "receipt", ArtifactID: "id", ArtifactSHA256: "invalid"}}
		case "digest":
			value := accounting.Digest("invalid")
			policy.ConfigDigest = &value
		case "optical":
			policy.OpticalImplementation = &invalid
		case "fee":
			value := uint16(10001)
			policy.FeeBPS = &value
		}
		if err := accounting.ValidateExecutionPolicy(policy); err == nil {
			t.Fatalf("invalid %s policy admitted", mode)
		}
	}
	publication := wireInitialPublication(t)
	for _, mode := range []string{"initial predecessor", "invalid successor", "market chain", "inventory chain", "kind"} {
		position := publication.Positions[0]
		switch mode {
		case "initial predecessor":
			position.Sequence = "1"
			position.PreviousEffectIdentity = strings.Repeat("a", 64)
		case "invalid successor":
			position.Sequence = "2"
			position.PreviousEffectIdentity = "invalid"
		case "market chain":
			position.PartitionKey = "1:" + fill.PoolAddress
		case "inventory chain":
			position.PartitionKind = "inventory"
			position.PartitionKey = "125:" + fill.PoolAddress
		case "kind":
			position.PartitionKind = "unknown"
		}
		if err := accounting.ValidatePartitionPosition(position); err == nil {
			t.Fatalf("invalid %s partition admitted", mode)
		}
	}
	for _, mode := range []string{"attribution", "execution", "inventory"} {
		attribution, execution := fill.Attribution, fill.Execution
		enrichment := accounting.EnrichmentV1{SourceFillID: fill.SourceFillID, SourceFactDigest: publication.SourceFactDigest, EvidenceDigest: accounting.Digest(strings.Repeat("a", 64)), EnrichmentVersion: "enriched", ProjectionVersion: "v1", Facet: "market", Attribution: &attribution, Execution: &execution}
		switch mode {
		case "attribution":
			enrichment.Attribution.TransactionSender = "invalid"
		case "execution":
			enrichment.Execution.Quality = "unknown"
		case "inventory":
			enrichment.Facet = "inventory"
		}
		if err := accounting.ValidateEnrichmentV1(enrichment); err == nil {
			t.Fatalf("invalid %s enrichment admitted", mode)
		}
	}
}

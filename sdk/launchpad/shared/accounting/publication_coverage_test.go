package accounting_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestPublicationConstructorsPreserveCommittedFacts(t *testing.T) {
	fixture := wireInitialPublication(t)
	market, err := accounting.NewMarketPublication(*fixture.Fill, fixture.Positions)
	if err != nil || !reflect.DeepEqual(market, fixture) {
		t.Fatalf("market publication changed source facts: %+v, %v", market, err)
	}
	if _, err := accounting.NewMarketPublication(*fixture.Fill, nil); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("unpositioned market publication: %v", err)
	}
	canonical, err := accounting.CanonicalPublicationV1(market)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := accounting.DecodePublicationV1(canonical)
	if err != nil || !reflect.DeepEqual(decoded, market) {
		t.Fatalf("canonical publication: %+v, %v", decoded, err)
	}
	fillBytes, err := json.Marshal(fixture.Fill)
	if err != nil {
		t.Fatal(err)
	}
	fill, err := accounting.DecodeFillV1(fillBytes)
	if err != nil || !reflect.DeepEqual(fill, *fixture.Fill) {
		t.Fatalf("decoded fill: %+v, %v", fill, err)
	}
	if _, err := accounting.DecodeFillV1(bytes.Replace(fillBytes, []byte(`"chainId":"125"`), []byte(`"chainId":"1"`), 1)); err == nil {
		t.Fatal("changed source chain accepted")
	}
	for _, tc := range []struct{ facet, subject string }{{"market", accounting.MarketSubject}, {"inventory", accounting.InventorySubject}} {
		got, err := accounting.SubjectForFacet(tc.facet)
		if err != nil || got != tc.subject {
			t.Fatalf("subject for %s: %s, %v", tc.facet, got, err)
		}
	}
	if subject, err := accounting.SubjectForFacet("other"); !errors.Is(err, accounting.ErrInvalid) || subject != "" {
		t.Fatalf("unsupported facet subject: %s, %v", subject, err)
	}
	bad := fixture
	bad.Facet = "inventory"
	if wire, err := accounting.CanonicalPublicationV1(bad); err == nil || wire != nil {
		t.Fatal("mismatched publication serialized")
	}
	enrichment := accounting.EnrichmentV1{
		SourceFillID: fixture.SourceFillID, SourceFactDigest: fixture.SourceFactDigest,
		Facet: "market", EnrichmentVersion: "attribution-v1", ProjectionVersion: "market-v1",
		EvidenceDigest: accounting.Digest(strings.Repeat("e", 64)), Attribution: &fixture.Fill.Attribution,
	}
	position := fixture.Positions[0]
	position.PayloadDigest, err = accounting.FacetDigest(accounting.PublicationV1{Enrichment: &enrichment})
	if err != nil {
		t.Fatal(err)
	}
	position.EffectIdentity = accounting.EffectIdentity(fixture.SourceFillID, "market", position.ProjectionVersion, enrichment.EvidenceDigest, position.PartitionKind, position.PartitionKey)
	updated, err := accounting.NewEnrichmentPublication(enrichment, []accounting.PartitionPosition{position})
	if err != nil || updated.Enrichment == nil || updated.PublicationID == fixture.PublicationID {
		t.Fatalf("enrichment publication: %+v, %v", updated, err)
	}
	if _, err := accounting.CanonicalPublicationV1(updated); err != nil {
		t.Fatalf("enrichment canonicalization: %v", err)
	}
}

package accounting_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestFillWireRejectsMalformedAndIncompleteSourceFacts(t *testing.T) {
	for _, wire := range []string{"", `{"version":`, `{"version":1,}`, `[1,]`, `{"values":[1,]}`, `{"values":[`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66), `null`, `{"version":"1"}`} {
		if _, err := accounting.DecodeFillV1([]byte(wire)); err == nil {
			t.Fatalf("incomplete fill wire admitted: %q", wire)
		}
	}
	if _, err := accounting.FillDigest(accounting.FillV1{}); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("invalid fill digest: %v", err)
	}
	if _, err := accounting.FacetDigest(accounting.PublicationV1{}); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("invalid facet digest: %v", err)
	}
	if _, err := accounting.NewMarketPublication(accounting.FillV1{}, nil); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("invalid initial publication: %v", err)
	}
	if err := accounting.ValidateEnrichmentV1(accounting.EnrichmentV1{}); !errors.Is(err, accounting.ErrInvalid) {
		t.Fatalf("invalid enrichment: %v", err)
	}
}

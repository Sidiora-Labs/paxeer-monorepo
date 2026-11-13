package social

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestSocialNullableValuesAndPageLimits(t *testing.T) {
	if got := strToTextPtr(nil); got != nil {
		t.Fatalf("missing address = %v", got)
	}
	empty := ""
	if got := strToTextPtr(&empty); got != nil {
		t.Fatalf("empty address = %v", got)
	}
	address := "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if got := strToTextPtr(&address); got == nil || *got != "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("normalized address = %v", got)
	}
	if got := reasonPtr(pgtype.Text{}); got != nil {
		t.Fatalf("missing reason = %v", got)
	}
	if got := reasonPtr(pgtype.Text{String: "reviewed", Valid: true}); got == nil || *got != "reviewed" {
		t.Fatalf("review reason = %v", got)
	}
	for _, tc := range []struct {
		raw  string
		want int32
	}{{"", 20}, {"invalid", 20}, {"0", 20}, {"-3", 20}, {"4", 4}, {"999", 100}} {
		if got := parseLimit(tc.raw, 20, 100); got != tc.want {
			t.Fatalf("limit %q = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

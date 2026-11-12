package search

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestIndexedDocumentRejectsChangedAuthorityFields(t *testing.T) {
	base := indexedFixture(731)
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	liquidity := "100"
	bad := []struct {
		name   string
		change func(*IndexedDocument)
	}{
		{"creator", func(d *IndexedDocument) { d.Creator = fixtureAddress(0) }},
		{"height", func(d *IndexedDocument) { d.SourceHeight = "2" }},
		{"block hash", func(d *IndexedDocument) { d.SourceBlockHash = "0x" + strings.Repeat("z", 64) }},
		{"profile wallet", func(d *IndexedDocument) { d.ProfileWallet = "other" }},
		{"profile label", func(d *IndexedDocument) { d.ProfileLabel = strings.Repeat("x", 101) }},
		{"explanation", func(d *IndexedDocument) { d.PublicExplanation = "/other/path" }},
		{"metrics timestamp", func(d *IndexedDocument) { d.MetricsAt = &now; d.MetricRevision = 0; d.LiquidityRaw = &liquidity }},
		{"liquidity", func(d *IndexedDocument) { d.LiquidityRaw = &liquidity }},
		{"volume window", func(d *IndexedDocument) { d.Volumes = map[string]string{"30d": "100"} }},
		{"volume value", func(d *IndexedDocument) { d.Volumes = map[string]string{"1h": "negative"} }},
		{"risk flag", func(d *IndexedDocument) { d.RiskFlags = []string{"unknown"} }},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			d := base
			tc.change(&d)
			if err := d.Validate(); err == nil {
				t.Fatal("changed indexed document accepted")
			}
		})
	}
	valid := base
	valid.MetricRevision, valid.MetricsAt, valid.LiquidityRaw = 1, &now, &liquidity
	valid.Volumes = map[string]string{"1h": "50"}
	valid.VerifiedQuoteAsset = new(bool)
	if err := valid.Validate(); err != nil {
		t.Fatalf("current measured document: %v", err)
	}
}

func TestIntegratedRequestParsingAndSQLPredicates(t *testing.T) {
	minimum, maximum := "10", "20"
	ageMinimum, ageMaximum := uint64(10), uint64(100)
	verified := true
	r := IntegratedRequest{Request: Request{Query: "name", Kind: "pool", Limit: 5, Filter: Filter{
		PoolStatus: "active", Creator: fixtureAddress(5), VerifiedQuoteAsset: &verified,
		CreationAge:         &AgeRange{MinInclusiveSeconds: &ageMinimum, MaxExclusiveSeconds: &ageMaximum},
		LiquidityRange:      &LiquidityFilter{Asset: "USDL", Decimals: 6, Depth: "total", Range: RawRange{MinInclusive: &minimum, MaxExclusive: &maximum}},
		VolumeWindow:        &VolumeFilter{Window: "7d", Range: RawRange{MinInclusive: &minimum, MaxExclusive: &maximum}},
		RiskDisclosureFlags: []string{"unverified"},
	}}, Sort: "address"}
	q, err := r.Validate()
	if err != nil {
		t.Fatal(err)
	}
	page := IntegratedPage{Generation: "g1", Revision: 1, ObservedAt: time.Now().UTC()}
	query, args, err := integratedSQL(q, r, page, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, predicate := range []string{"d.pool_status=", "d.creator=", "d.verified_quote=", "d.metrics_at >=", "d.created_at <=", "d.created_at >", "d.liquidity_raw>=", "d.liquidity_raw<", "d.volume_7d>=", "d.volume_7d<", "d.risk_flags @>"} {
		if !strings.Contains(query, predicate) {
			t.Fatalf("missing predicate %q in %s", predicate, query)
		}
	}
	if len(args) != 18 {
		t.Fatalf("bound arguments: %d", len(args))
	}
	parsed, err := ParseIntegratedRequest(url.Values{"q": {"name"}, "type": {"pool"}, "limit": {"5"}, "sort": {"address"}})
	if err != nil || parsed.Query != "name" || parsed.Limit != 5 {
		t.Fatalf("parsed request: %+v %v", parsed, err)
	}
	for _, values := range []url.Values{{"q": {"name", "other"}}, {"other": {"value"}}, {"limit": {"05"}}, {"filter": {`{"unknown":1}`}}, {"cursor": {`{"version":2,"version":1}`}}} {
		if _, err := ParseIntegratedRequest(values); err == nil {
			t.Fatalf("invalid request accepted: %v", values)
		}
	}
}

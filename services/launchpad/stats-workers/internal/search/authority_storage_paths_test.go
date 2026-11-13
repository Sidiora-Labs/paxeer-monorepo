package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func TestSearchCreationAuthorizationBindsCanonicalMetadata(t *testing.T) {
	pool := integratedDatabase(t)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join("../../../../../migrations/190001.sql"))
	if err != nil {
		t.Fatal(err)
	}
	ddl := strings.Split(string(raw), "CREATE TABLE indexer.indexer_token_transfers")[0]
	if _, err = pool.Exec(ctx, "CREATE SCHEMA indexer;"+ddl); err != nil {
		t.Fatal(err)
	}
	d := indexedFixture(701)
	d.CreatedAt = d.CreatedAt.Truncate(time.Second)
	d.MetricRevision = 0
	d.MetricsAt = nil
	d.LiquidityRaw = nil
	d.Volumes = map[string]string{}
	d.VerifiedQuoteAsset = nil
	d.RiskFlags = []string{}
	d.PublicExplanation = ""
	d.ProfileWallet = ""
	d.ProfileLabel = ""
	payload := map[string]any{"pool": d.PoolAddress, "token": d.TokenAddress, "creator": d.Creator, "blockTimestampSeconds": fmt.Sprint(d.CreatedAt.Unix()), "metadata": map[string]string{"name": d.Name, "symbol": d.Ticker}}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,1,$6,$7,$8,0,0,$9,'{}',$10)`, d.TokenAddress, d.PoolAddress, d.Creator, fixtureAddress(801), fixtureAddress(802), d.SourceHeight, d.SourceBlockHash, "0x"+strings.Repeat("a", 64), d.CreatedAt.Unix(), body); err != nil {
		t.Fatal(err)
	}
	p := Projection{}
	c := Change{Version: 2, Kind: "creation", Revision: 1, PoolAddress: d.PoolAddress, Document: &d}
	e := delivery.Envelope{SourceOrder: delivery.SourceOrder{Height: d.SourceHeight}}
	for _, tc := range []struct {
		name, sql string
		edit      func(*Change, *delivery.Envelope)
		want      error
	}{
		{"canonical metadata", "", nil, nil},
		{"metric enrichment", "", func(c *Change, _ *delivery.Envelope) { c.Document.MetricRevision = 1 }, delivery.ErrIntegrity},
		{"metadata name", "UPDATE indexer.indexer_canonical_creations SET payload=jsonb_set(payload,'{metadata,name}','\"Other\"')", nil, delivery.ErrIntegrity},
		{"metadata shape", "UPDATE indexer.indexer_canonical_creations SET payload='[]'", nil, delivery.ErrIntegrity},
		{"creation timestamp", "UPDATE indexer.indexer_canonical_creations SET payload=jsonb_set(payload,'{blockTimestampSeconds}','\"1\"')", nil, delivery.ErrIntegrity},
		{"source position", "", func(_ *Change, e *delivery.Envelope) { e.SourceOrder.Height = "2" }, delivery.ErrIntegrity},
		{"missing creation", "DELETE FROM indexer.indexer_canonical_creations", nil, pgx.ErrNoRows},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			changed := c
			doc := *c.Document
			changed.Document = &doc
			event := e
			if tc.edit != nil {
				tc.edit(&changed, &event)
			}
			if tc.sql != "" {
				if _, err = tx.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			err = p.authorizeTx(ctx, tx, event, changed)
			if tc.want == nil && err != nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("creation authority: %v; want %v", err, tc.want)
			}
		})
	}
}

func TestSearchProjectionStorageAndSourceBindings(t *testing.T) {
	pool := integratedDatabase(t)
	ctx := context.Background()
	d := seedIndexed(t, pool, 702)
	if _, err := pool.Exec(ctx, "CREATE SCHEMA search_source;CREATE TABLE search_source.events_v2(event_id text PRIMARY KEY,payload jsonb NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	change := Change{Version: 2, Kind: "visibility", Revision: 2, PoolAddress: d.PoolAddress, Public: false}
	payload, err := json.Marshal(change)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO search_source.events_v2 VALUES('source',$1)", payload); err != nil {
		t.Fatal(err)
	}
	p := Projection{Sources: map[string]SourceRelation{"visibility": {Schema: "search_source", View: "events_v2"}}}
	for _, tc := range []struct {
		name, sql string
		source    SourceRelation
		want      error
	}{
		{"source payload", "", SourceRelation{"search_source", "events_v2"}, nil},
		{"source schema", "", SourceRelation{"Invalid", "events_v2"}, ErrUnsupported},
		{"source version", "", SourceRelation{"search_source", "events"}, ErrUnsupported},
		{"source absent", "DELETE FROM search_source.events_v2", SourceRelation{"search_source", "events_v2"}, pgx.ErrNoRows},
		{"source JSON", "UPDATE search_source.events_v2 SET payload='[]'", SourceRelation{"search_source", "events_v2"}, delivery.ErrIntegrity},
		{"source content", "UPDATE search_source.events_v2 SET payload=jsonb_set(payload,'{public}','true')", SourceRelation{"search_source", "events_v2"}, delivery.ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.sql != "" {
				if _, err = tx.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			p.Sources["visibility"] = tc.source
			err = p.authorizeTx(ctx, tx, delivery.Envelope{EventID: "source"}, change)
			if tc.want == nil && err != nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("source binding: %v; want %v", err, tc.want)
			}
		})
	}
	for _, table := range []string{"source_documents_v2", "authority_v2", "generations_v2", "profile_tombstones_v2", "documents_v2"} {
		t.Run(table, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, "ALTER TABLE stats_search."+table+" RENAME TO unavailable_projection"); err != nil {
				t.Fatal(err)
			}
			doc := d
			doc.MetadataRevision = 2
			doc.Name = "Changed"
			doc.NameKey = "changed"
			c := Change{Version: 2, Kind: "metadata", Revision: 2, PoolAddress: d.PoolAddress, Document: &doc}
			if table == "authority_v2" {
				c = change
			}
			if table == "profile_tombstones_v2" {
				doc.ProfileWallet = fixtureAddress(900)
				doc.ProfileLabel = "Public"
			}
			if _, err = applyChangeTx(ctx, tx, c); err == nil {
				t.Fatal("unavailable projection accepted")
			}
		})
	}
	for _, kind := range []string{"creation", "lifecycle", "metrics"} {
		t.Run("unchanged "+kind, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			revision := int64(1)
			if kind == "metrics" {
				revision = 0
			}
			changed, err := applyChangeTx(ctx, tx, Change{Version: 2, Kind: kind, Revision: revision, PoolAddress: d.PoolAddress, Document: &d})
			if err != nil || changed {
				t.Fatalf("unchanged source: %v %v", changed, err)
			}
		})
	}
}

func TestPublicSearchRequestFieldBounds(t *testing.T) {
	for _, tc := range []struct {
		values url.Values
		status int
		code   string
	}{
		{url.Values{"q": {"alpha", "beta"}, "type": {"pool"}}, 400, "QUERY_INVALID"},
		{url.Values{"q": {strings.Repeat("a", 1025)}, "type": {"pool"}}, 413, "PAYLOAD_TOO_LARGE"},
		{url.Values{"q": {"alpha"}, "type": {"pool"}, "limit": {"01"}}, 422, "FIELD_INVALID"},
		{url.Values{"q": {"alpha"}, "type": {"pool"}, "filter": {`{"poolStatus":"other"}`}}, 422, "FIELD_INVALID"},
		{url.Values{"q": {"alpha"}, "type": {"pool"}, "cursor": {`{"version":2}`}}, 400, "CURSOR_INVALID"},
		{url.Values{"q": {"alpha"}, "type": {"profile"}}, 412, "PRECONDITION_FAILED"},
	} {
		_, err := ParsePublicRequest(tc.values)
		var invalid PublicRequestError
		if !errors.As(err, &invalid) || invalid.Status != tc.status || invalid.Code != tc.code || invalid.Error() != tc.code {
			t.Fatalf("public request %v: %v", tc.values, err)
		}
	}
	base := indexedFixture(703).Document
	for _, edit := range []func(*Document){
		func(d *Document) { d.LanguageHints = []string{strings.Repeat("a", 36)} }, func(d *Document) { d.LanguageHints = []string{"invalid_language!"} },
		func(d *Document) { d.ConfusableWarnings = []string{"mixed_script", "mixed_script"} }, func(d *Document) { d.CreationEventID = "invalid!" },
		func(d *Document) { d.CreationEventID = strings.Repeat("a", 257) }, func(d *Document) { d.IndexedAt = "0000-01-01T00:00:00.000Z" },
	} {
		d := base
		edit(&d)
		if !errors.Is(d.Validate(), ErrInvalid) {
			t.Fatalf("invalid document accepted: %+v", d)
		}
	}
	if base.Key() != "125:"+base.PoolAddress {
		t.Fatal(base.Key())
	}
	for _, raw := range []string{`{"a":[1,}`, `{"a":`, strings.Repeat(`{"a":`, 18) + `1` + strings.Repeat(`}`, 18), `{"a":[1}`} {
		var c Change
		if strictJSON([]byte(raw), &c) == nil {
			t.Fatalf("malformed JSON accepted: %s", raw)
		}
	}
}

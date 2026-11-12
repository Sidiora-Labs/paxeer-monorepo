package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/risk"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestOwnershipReadValidatesCursorAndLedgerState(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	st := store.New(db)
	address := func(n int) string { return fmt.Sprintf("0x%040x", n) }
	market, token, wallet := address(1), address(2), address(3)
	if _, err := st.InsertInitialPoolStats(ctx, store.InitialPoolStats{PoolAddress: market, TokenAddress: token, Price: "1", MarketCap: "100", High24h: "1", Low24h: "1", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	ddl, err := holdings.SchemaSQL("ownership_bounds")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := holdings.NewRepository(db, "ownership_bounds")
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.Discover(ctx, token, 6, "10", true); err != nil {
		t.Fatal(err)
	}
	mint := holdings.Transfer{ChainID: "125", Token: token, From: holdings.Zero, To: wallet, Amount: "100", Height: "10", TransactionIndex: "0", LogIndex: "0", TransactionHash: "0x" + strings.Repeat("a", 64), BlockHash: "0x" + strings.Repeat("b", 64)}
	if _, err = repo.Apply(ctx, mint); err != nil {
		t.Fatal(err)
	}
	encode := func(c ownershipCursor) string {
		raw, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	for _, tc := range []struct {
		name   string
		cursor ownershipCursor
	}{
		{"amount", ownershipCursor{Scope: "distribution:" + market, Balance: "01", Address: wallet, Revision: 1, Rank: 1}},
		{"address", ownershipCursor{Scope: "distribution:" + market, Balance: "100", Address: "invalid", Revision: 1, Rank: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, status, err := readOwnership(ctx, st, "ownership_bounds", market, httptest.NewRequest("GET", "/?cursor="+encode(tc.cursor), nil), false)
			if err == nil || status != 400 {
				t.Fatalf("cursor bounds: %d %v", status, err)
			}
		})
	}
	for _, tc := range []struct {
		name, setup, restore string
		whales               bool
	}{
		{"quarantined coverage", "UPDATE ownership_bounds.tokens SET coverage='quarantined'", "UPDATE ownership_bounds.tokens SET coverage='complete'", false},
		{"aggregate conservation", "UPDATE ownership_bounds.aggregates SET raw=99", "UPDATE ownership_bounds.aggregates SET raw=100", false},
		{"aggregate storage", "ALTER TABLE ownership_bounds.aggregates RENAME TO saved_aggregates", "ALTER TABLE ownership_bounds.saved_aggregates RENAME TO aggregates", false},
		{"leader storage", "ALTER TABLE ownership_bounds.balances RENAME TO saved_balances", "ALTER TABLE ownership_bounds.saved_balances RENAME TO balances", false},
		{"whale storage", "ALTER TABLE ownership_bounds.balances RENAME TO saved_balances", "ALTER TABLE ownership_bounds.saved_balances RENAME TO balances", true},
		{"token storage", "ALTER TABLE ownership_bounds.tokens RENAME TO saved_tokens", "ALTER TABLE ownership_bounds.saved_tokens RENAME TO tokens", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.Exec(ctx, tc.setup); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := db.Exec(ctx, tc.restore); err != nil {
					t.Fatal(err)
				}
			}()
			_, status, err := readOwnership(ctx, st, "ownership_bounds", market, httptest.NewRequest("GET", "/", nil), tc.whales)
			if err == nil || status != 503 {
				t.Fatalf("ledger admission: %d %v", status, err)
			}
		})
	}
	if _, err = db.Exec(ctx, "UPDATE ownership_bounds.tokens SET coverage='incomplete'"); err != nil {
		t.Fatal(err)
	}
	page, status, err := readOwnership(ctx, st, "ownership_bounds", market, httptest.NewRequest("GET", "/", nil), false)
	if err != nil || status != 200 || len(page.Holders) != 1 || page.Holders[0].Balance != "100" || page.Distribution.Top10.BasisPoints != nil || page.Distribution.Top10.Reason != "incomplete" || page.Distribution.CirculatingTop10.Reason != "incomplete" || page.Distribution.Creator.Reason != "incomplete" || page.Distribution.CirculatingCreator.Reason != "incomplete" {
		t.Fatalf("incomplete ownership: %+v %d %v", page, status, err)
	}
	if _, status, err = readOwnership(ctx, st, "ownership_bounds", address(99), httptest.NewRequest("GET", "/", nil), false); err == nil || status != 404 {
		t.Fatalf("absent pool: %d %v", status, err)
	}
	if ownershipSchema([]string{"one", "two"}) != "" {
		t.Fatal("ambiguous schema selected")
	}
	if _, status, err = readOwnership(ctx, nil, "ownership_bounds", market, httptest.NewRequest("GET", "/", nil), false); err == nil || status != 503 {
		t.Fatalf("missing store: %d %v", status, err)
	}
	db.Close()
	if _, status, err = readOwnership(ctx, st, "ownership_bounds", market, httptest.NewRequest("GET", "/", nil), false); err == nil || status != 503 {
		t.Fatalf("closed store: %d %v", status, err)
	}
}

func TestDurableAnalyticsHTTPReadsCommittedSnapshotsAndReplay(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	if _, err := db.Exec(ctx, "CREATE SCHEMA http_analytics"); err != nil {
		t.Fatal(err)
	}
	a := analytics.ReactionAuthority{Pool: db, Schema: "http_analytics"}
	for _, schema := range []func() (string, error){a.SchemaSQL, a.SnapshotSQL, a.ProjectionSQL} {
		ddl, err := schema()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	market := "0x" + strings.Repeat("1", 40)
	if _, err := db.Exec(ctx, "INSERT INTO http_analytics.analytics_progress(pool,height,transaction_index,log_index,block_hash,revision,observed_at) VALUES($1,1,0,0,$2,1,$3)", market, "0x"+strings.Repeat("a", 64), time.Now().UTC().Format("2006-01-02T15:04:05.000Z")); err != nil {
		t.Fatal(err)
	}
	projector, err := analytics.NewProjector(a, risk.ProjectionModelV1{}, analytics.PostgresRiskInputs{Authority: a})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = projector.RecomputeExpiryTx(ctx, tx, market); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	RegisterDurableAnalytics(router, a)
	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{"/v2/stats/" + market + "/pressure", 200, "pressure-v1"},
		{"/v2/stats/" + market + "/risk", 200, "model_metadata"},
		{"/v2/stats/" + market + "/analytics/revisions?limit=1", 200, "nextCursor"},
		{"/v2/stats/" + market + "/analytics/revisions?after=0", 200, "pressure-v1"},
		{"/v2/stats/" + market + "/analytics/revisions?limit=101", 400, "invalid analytics page limit"},
		{"/v2/stats/" + market + "/analytics/revisions?after=01", 422, "Invalid analytics field"},
		{"/v2/stats/0xinvalid/risk", 422, "Invalid analytics field"},
		{"/v2/stats/0x" + strings.Repeat("2", 40) + "/risk", 404, "Analytics snapshot not found"},
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.body) {
			t.Fatalf("%s: %d %s", tc.path, rec.Code, rec.Body.String())
		}
		if tc.status == 200 && rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(rec.Header())
		}
	}
	db.Close()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/v2/stats/"+market+"/risk", nil))
	if rec.Code != 503 {
		t.Fatalf("unavailable authority: %d %s", rec.Code, rec.Body.String())
	}
}

func TestReactionCountsPropagateRedisTypeErrors(t *testing.T) {
	rdb := internaltest.NewRedis(t)
	ctx := context.Background()
	market := uuid.NewString()
	for _, reaction := range []string{"hot", "diamond", "bearish", "trash", "warning"} {
		key := "reactions:" + market + ":" + reaction
		if err := rdb.Set(ctx, key, "invalid set", time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		if _, err := countReactions(ctx, rdb, market); err == nil || !strings.Contains(err.Error(), "WRONGTYPE") {
			t.Fatalf("reaction %s type: %v", reaction, err)
		}
		if err := rdb.Del(ctx, key).Err(); err != nil {
			t.Fatal(err)
		}
	}
}

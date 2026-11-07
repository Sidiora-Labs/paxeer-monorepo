package l18

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func (f *repair62Fixture) handler(who string) http.Handler {
	h := HTTP{Service: f.journey, Authenticate: func(*http.Request) (Actor, error) { return Actor{Wallet: who}, nil }}
	router := chi.NewRouter()
	router.Get("/v2/me/rewards", h.Rewards)
	router.Post("/v2/me/rewards:prepare-claim", h.PrepareClaim)
	return router
}
func repair62Rewards(t *testing.T, h http.Handler, cursor string, limit int) iface.RewardsResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v2/me/rewards?limit="+strconv.Itoa(limit)+"&cursor="+url.QueryEscape(cursor), nil)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("rewards HTTP %d: %s", response.Code, response.Body.String())
	}
	if e := iface.ValidateJSON("RewardsResponse", response.Body.Bytes()); e != nil {
		t.Fatal(e)
	}
	var out iface.RewardsResponse
	if e := json.Unmarshal(response.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	return out
}

func TestRepair_62_SchemaSequenceGrants(t *testing.T) {
	d := repair62Bootstrap(t)
	for _, id := range []int64{180000, 180001, 190001} {
		if d.catalog.Applied[id] == "" {
			t.Fatalf("reviewed migration %d not applied", id)
		}
	}
	if len(d.catalog.Applied) != 37 {
		t.Fatalf("complete coordinator dependency plan=%d, want 37", len(d.catalog.Applied))
	}
	if d.manifest.RuntimeRole != "pnl_runtime" || d.manifest.OwnerRole != "pnl_owner" || d.manifest.Superuser || d.manifest.CreateDB || d.manifest.CreateRole || d.manifest.BypassRLS || d.manifest.RuntimeOwnsObjects {
		t.Fatal("runtime gained administrative authority")
	}
	for _, name := range []string{"entries_sequence", "outbox_sequence", "community_history_revision", "referral_consents_revision"} {
		object := "pnl.l18_" + name + "_seq"
		if d.catalog.Objects[object].Kind != "sequence" {
			t.Fatalf("real sequence absent: %s", object)
		}
		if err := d.permit(object, "USAGE"); err != nil {
			t.Fatalf("real sequence grant: %v", err)
		}
	}
	for _, name := range []string{"funding", "inbox", "awards", "claims", "epochs", "epoch_claims"} {
		object := "pnl.l18_" + name + "_sequence_seq"
		if _, present := d.catalog.Objects[object]; present {
			t.Fatalf("natural key table unexpectedly has serial sequence: %s", name)
		}
		err := d.applyGrant(`GRANT USAGE ON SEQUENCE "pnl"."l18_` + name + `_sequence_seq" TO "pnl_runtime"`)
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "42P01" {
			t.Fatalf("missing-object GRANT was suppressed: %s: %v", object, err)
		}
	}
	pool := d.pool(t)
	for _, name := range []string{"canonical_creations", "ingest_watermarks", "block_completeness", "raw_logs"} {
		rows, err := pool.Query(context.Background(), "SELECT * FROM indexer.indexer_"+name+" LIMIT 0")
		if rows != nil {
			rows.Close()
			if err == nil {
				err = rows.Err()
			}
		}
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "42501" {
			t.Fatalf("base table %s must remain private: %v", name, err)
		}
	}
}

func TestRepair_62_JourneyIndexerGrants(t *testing.T) {
	f := repair62Chain(t)
	b := Checkpoint{Number: f.db.peers.Initial, Hash: repair62Hash(f.db.peers.Initial)}
	h := f.handler(f.chain.Alice)
	response := repair62Rewards(t, h, "", 1)
	if len(response.Data.Items) != 1 || response.Data.Items[0].RewardId != "creator:1" || response.Data.Items[0].ClaimableRaw != "10000000" || response.Watermark == nil || string(response.Watermark.Height) != strconv.FormatUint(b.Number, 10) {
		t.Fatalf("runtime reward=%+v", response)
	}
	empty := repair62Rewards(t, f.handler(strings.ToLower(f.chain.Admin)), "", 1)
	if len(empty.Data.Items) != 0 || empty.NextCursor != nil {
		t.Fatalf("unrelated wallet rewards=%+v", empty)
	}
	p, e := f.journey.Preview(context.Background(), Actor{Wallet: f.chain.Alice}, "1")
	if e != nil || p.Pool != f.chain.Pool {
		t.Fatalf("market view=%+v: %v", p, e)
	}
	key := uuid.NewString()
	body := fmt.Sprintf(`{"rewardIds":["creator:1"],"recipient":%q}`, f.chain.Alice)
	var first string
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v2/me/rewards:prepare-claim", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		out := httptest.NewRecorder()
		h.ServeHTTP(out, req)
		if out.Code != http.StatusOK {
			t.Fatalf("runtime preparation %d: %s", out.Code, out.Body.String())
		}
		if e := iface.ValidateJSON("RewardClaimResponse", out.Body.Bytes()); e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			first = out.Body.String()
		} else if first != out.Body.String() {
			t.Fatal("preparation replay changed")
		}
	}
	watcher, e := NewCommunityWatcher(f.journey, []string{f.chain.Alice})
	if e != nil {
		t.Fatal(e)
	}
	if e = watcher.RunOnce(context.Background()); e != nil {
		t.Fatal(e)
	}

	f.db.mu.Lock()
	defer f.db.mu.Unlock()
	if got := f.db.community["1"]; len(got) < 3 || got[2] != "active" {
		t.Fatalf("community history=%v", got)
	}
	if len(f.db.scans) != 1 || !f.db.scans["1"] {
		t.Fatalf("scan checkpoint=%v", f.db.scans)
	}
	prepared := 0
	for _, topic := range f.db.outbox {
		if topic == "reward.claim_prepared" {
			prepared++
		}
	}
	if prepared != 1 || len(f.db.requests) != 1 || len(f.db.audit) != 1 {
		t.Fatalf("replay duplicated preparation: outbox=%d requests=%d audit=%d", prepared, len(f.db.requests), len(f.db.audit))
	}
}

func TestRepair_62_RewardHistoryAgeOutage(t *testing.T) {
	f := repair62Chain(t)
	ctx := context.Background()
	b := f.advance(f.db.peers.Aged)
	oldest, err := f.journey.market(ctx, "1")
	if err != nil || b.Number-oldest.Height <= 100000 {
		t.Fatalf("oldest creation=%+v checkpoint=%+v: %v", oldest, b, err)
	}
	page := repair62Rewards(t, f.handler(f.chain.Alice), "", 1)
	if len(page.Data.Items) != 1 || page.Data.Items[0].RewardId != "creator:3" || page.NextCursor == nil {
		t.Fatalf("unrelated old market blocked later first page: %+v", page)
	}
	reward := page.Data.Items[0]
	if reward.ClaimedRaw != "11000000" || reward.ClaimableRaw != "2000000" || reward.EarnedRaw != "13000000" {
		t.Fatalf("exact indexed totals=%+v", reward)
	}
	// The peer's next committed response includes a further claim and transfers.
	// The original cursor must still read canonical history and ownership at b.
	f.advance(f.db.peers.Advanced)
	next := repair62Rewards(t, f.handler(f.chain.Alice), *page.NextCursor, 1)
	if len(next.Data.Items) != 1 || next.Data.Items[0].RewardId != "creator:4" || next.NextCursor != nil || next.Watermark == nil || next.Watermark.Height != page.Watermark.Height || next.Watermark.BlockHash != page.Watermark.BlockHash {
		t.Fatalf("pinned continuation=%+v first=%+v", next, page)
	}
	fresh := repair62Rewards(t, f.handler(f.chain.Alice), "", 1)
	if len(fresh.Data.Items) != 1 || fresh.Data.Items[0].RewardId != "creator:3" || fresh.Data.Items[0].ClaimedRaw != "13000000" || fresh.Data.Items[0].ClaimableRaw != "0" {
		t.Fatalf("new indexed claims=%+v", fresh)
	}
	bob := repair62Rewards(t, f.handler(f.chain.Bob), "", 1)
	if len(bob.Data.Items) != 1 || bob.Data.Items[0].RewardId != "creator:1" || bob.Data.Items[0].ClaimableRaw != "10000000" {
		t.Fatalf("old relevant market=%+v", bob)
	}
}

func TestRepair_62_IndexedPagingIntegrity(t *testing.T) {
	f := repair62Chain(t)
	b := f.advance(f.db.peers.Aged)
	// A whole 500-row boundary plus a trailing row must be summed exactly once,
	// including transaction/log ordering within a single block.
	f.db.mu.Lock()
	base := f.db.peers.Claims[0]
	f.db.peers.Claims = nil
	for i := 0; i < 501; i++ {
		x := base
		x.TxIndex = int64(i / 100)
		x.LogIndex = int64(i % 100)
		x.Data = "0x" + fmt.Sprintf("%064x", i+1)
		f.db.peers.Claims = append(f.db.peers.Claims, x)
	}
	f.db.mu.Unlock()
	m, err := f.journey.market(context.Background(), "3")
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.journey.claimedHistory(context.Background(), m, f.chain.Alice, b)
	if err != nil || got != "125751" {
		t.Fatalf("paged exact claim total=%s: %v", got, err)
	}
	f.db.mu.Lock()
	if len(f.db.claimPages) != 2 || f.db.claimPages[0] != 500 || f.db.claimPages[1] != 1 {
		t.Fatalf("history query bounds=%v", f.db.claimPages)
	}
	f.db.peers.Claims[0].Data = "0x01"
	f.db.mu.Unlock()
	if _, err = f.journey.claimedHistory(context.Background(), m, f.chain.Alice, b); !errors.Is(err, ErrConflict) {
		t.Fatalf("malformed admitted claim was accepted: %v", err)
	}
}

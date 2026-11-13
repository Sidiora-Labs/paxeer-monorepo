package projectionstats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func localReadFixture(t *testing.T) (context.Context, *pgxpool.Pool, *ReadService, string) {
	t.Helper()
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	if _, err := pool.Exec(ctx, "CREATE SCHEMA indexer"); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"60000", "190001"} {
		if name == "190001" {
			ddl, err := delivery.PartitionSchemaDDL("indexer")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := os.ReadFile(filepath.Join(root, "launchpad/spec/production-system/integration/migrations", name+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA stats_local"); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, "stats_local")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, repo.MigrationSQL()+repo.Phase2MigrationSQL()); err != nil {
		t.Fatal(err)
	}
	const stream = "canonical"
	blockHash := "0x" + strings.Repeat("a", 64)
	poolAddress := fmt.Sprintf("0x%040x", 101)
	token := fmt.Sprintf("0x%040x", 102)
	creator := fmt.Sprintf("0x%040x", 103)
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,1,10,$6,$7,0,0,100,'{}','{}')`, token, poolAddress, creator, creator, token, blockHash, blockHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'indexer',1,clock_timestamp()+interval '1 hour')`, stream); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,10,10)`, stream); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,10,$2,0,true,true)`, stream, blockHash); err != nil {
		t.Fatal(err)
	}
	service, err := NewReadService(repo, ReadConfig{SourceSchema: "indexer", SourceStream: stream, ProjectionVersion: "projection-v1", CursorKey: []byte(strings.Repeat("k", 32)), CursorTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return ctx, pool, service, poolAddress
}

func TestLocalCanonicalReadAndWindow(t *testing.T) {
	ctx, pool, service, address := localReadFixture(t)
	page, err := service.Trades(ctx, address, "", 10, "", 200)
	if err != nil || len(page.Trades) != 0 || page.NextCursor != nil || page.Watermark.Height != "10" || page.Watermark.SourceStream != "canonical" {
		t.Fatalf("canonical trades: %+v %v", page, err)
	}
	window, err := service.Window(ctx, address, 60, 200)
	if err != nil || window.VolumeUSDLRaw != "0" || window.PoolFillCount != "0" || window.UniqueActors != "0" || window.WindowStart != 140 || window.WindowEnd != 200 {
		t.Fatalf("canonical window: %+v %v", window, err)
	}
	saved, err := service.LastWindow(ctx, address, 60)
	if err != nil || saved.Watermark != window.Watermark || saved.VolumeUSDLRaw != "0" {
		t.Fatalf("saved window: %+v %v", saved, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM stats_local.phase2_read_snapshots WHERE pool=$1`, address).Scan(&count); err != nil || count != 1 {
		t.Fatalf("snapshot rows=%d err=%v", count, err)
	}
	router := chi.NewRouter()
	PublicHandlers{Service: service, Clock: func() time.Time { return time.Unix(200, 0) }}.Register(router)
	for _, path := range []string{"/v2/stats/" + address + "/transactions", "/v2/stats/" + address + "?windowSeconds=60"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || len(body) == 0 {
			t.Fatalf("%s body: %s %v", path, res.Body.String(), err)
		}
	}
	if _, err := service.Trades(ctx, address, "", 10, "changed", 200); err == nil {
		t.Fatal("changed cursor accepted")
	}
	if _, err := service.Window(ctx, address, 0, 200); err == nil {
		t.Fatal("zero window accepted")
	}
}

func TestRealReceiptReadPagesAndExpiry(t *testing.T) {
	f := newPublicFixture(t)
	fills := f.ingestReal()
	f.advance(33963761, f.real.Blocks["33963761"].Hash)
	service := f.readService()
	now := f.real.Blocks["33963749"].Timestamp + 60
	first, err := service.Trades(f.ctx, f.real.Pool, "", 1, "", now)
	if err != nil || len(first.Trades) != 1 || first.NextCursor == nil || first.Trades[0].Position != Position(fills["sell"].Position) {
		t.Fatalf("first receipt page: %+v %v", first, err)
	}
	second, err := service.Trades(f.ctx, f.real.Pool, "", 1, *first.NextCursor, now)
	if err != nil || len(second.Trades) != 1 || second.NextCursor != nil || second.Trades[0].Position != Position(fills["buy"].Position) || second.Watermark != first.Watermark {
		t.Fatalf("second receipt page: %+v %v", second, err)
	}
	window, err := service.Window(f.ctx, f.real.Pool, 3600, now)
	if err != nil || window.PoolFillCount != "2" || window.VolumeUSDLRaw == "0" || len(window.Fees) == 0 || window.Watermark.Height != "33963761" {
		t.Fatalf("receipt window: %+v %v", window, err)
	}
	scheduler := ExpiryScheduler{Service: service, Windows: []int64{3600}, Interval: time.Second, Clock: func() time.Time { return time.Unix(now, 0) }}
	if err := scheduler.Refresh(f.ctx, now); err != nil {
		t.Fatal(err)
	}
	saved, err := service.LastWindow(f.ctx, f.real.Pool, 3600)
	if err != nil || saved.PoolFillCount != "2" || saved.Watermark.Height != "33963761" {
		t.Fatalf("refreshed window: %+v %v", saved, err)
	}
	cancelled, cancel := context.WithCancel(f.ctx)
	cancel()
	if err := scheduler.Run(cancelled); err == nil {
		t.Fatal("cancelled expiry run succeeded")
	}
}

func TestLocalPublicHandlersValidateRequestsAndServeDurableWindow(t *testing.T) {
	ctx, pool, service, address := localReadFixture(t)
	router := chi.NewRouter()
	PublicHandlers{Service: service, Clock: func() time.Time { return time.Unix(200, 0) }}.Register(router)
	request := func(path string, want int) []byte {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != want || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
		return response.Body.Bytes()
	}
	for _, path := range []string{
		"/v2/stats/" + address + "/transactions?other=1", "/v2/stats/" + address + "/transactions?direction=asc",
		"/v2/stats/" + address + "/transactions?limit=01", "/v2/stats/" + address + "/transactions?side=other",
		"/v2/stats/" + address + "/transactions?limit=101", "/v2/stats/" + address + "?other=1",
		"/v2/stats/" + address + "?windowSeconds=01", "/v2/stats/" + address + "?windowSeconds=86401",
		"/v2/stats/not-an-address?windowSeconds=60",
	} {
		request(path, 400)
	}
	request("/v2/stats/0x"+strings.Repeat("f", 40)+"?windowSeconds=60", 404)
	var fresh WindowSnapshot
	if err := json.Unmarshal(request("/v2/stats/"+address+"?windowSeconds=60", 200), &fresh); err != nil || fresh.Stale {
		t.Fatalf("fresh window: %+v %v", fresh, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()-interval '1 second' WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	var stale WindowSnapshot
	if err := json.Unmarshal(request("/v2/stats/"+address+"?windowSeconds=60", 200), &stale); err != nil || !stale.Stale || stale.Watermark != fresh.Watermark {
		t.Fatalf("durable window: %+v %v", stale, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM stats_local.phase2_windows WHERE pool=$1`, address); err != nil {
		t.Fatal(err)
	}
	request("/v2/stats/"+address+"?windowSeconds=60", 503)
}

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func persistenceDatabase(t *testing.T) (*pgxpool.Pool, *store.PublicationStore) {
	t.Helper()
	database := internaltest.NewRankingRuntimeDatabase(t)
	pool, err := pgxpool.New(context.Background(), database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	guard, err := queue.NewEpochGuard("recovery", database.Epoch, database.Region)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := store.NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	return pool, publication
}

func TestPublicationPersistenceFailuresRollBackEveryTable(t *testing.T) {
	pool, publication := persistenceDatabase(t)
	ctx := context.Background()
	snapshots, err := ranker.ComputeRun(publicationPopulatedInput(t), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"snapshots", "snapshot_entries", "category_heads", "publication_outbox"} {
		t.Run(table, func(t *testing.T) {
			name := pgx.Identifier{"ranking", table}.Sanitize()
			if _, err := pool.Exec(ctx, "ALTER TABLE "+name+" ADD CONSTRAINT storage_available CHECK(false) NOT VALID"); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := pool.Exec(ctx, "ALTER TABLE "+name+" DROP CONSTRAINT storage_available"); err != nil {
					t.Error(err)
				}
			})
			err := publication.Complete(ctx, snapshots)
			var databaseError *pgconn.PgError
			if !errors.As(err, &databaseError) || databaseError.Code != "23514" || databaseError.ConstraintName != "storage_available" {
				t.Fatalf("persistence error: %v", err)
			}
			for _, affected := range []string{"snapshots", "snapshot_entries", "category_heads", "publication_outbox"} {
				var count int
				if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{"ranking", affected}.Sanitize()).Scan(&count); err != nil || count != 0 {
					t.Fatalf("rolled back %s rows = %d: %v", affected, count, err)
				}
			}
		})
	}
	if err := publication.Complete(ctx, snapshots); err != nil {
		t.Fatalf("completion after storage recovery: %v", err)
	}
}

func TestPublicationAndInputsRefuseClosedDatabase(t *testing.T) {
	pool, publication := persistenceDatabase(t)
	ctx := context.Background()
	snapshots, err := ranker.ComputeRun(publicationPopulatedInput(t), false)
	if err != nil {
		t.Fatal(err)
	}
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer peer.Close()
	visibility, err := store.NewHTTPRankingVisibility(peer.URL, peer.Client())
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.NewCanonicalInputReader("indexer", "stats", "canonical", "projection", visibility)
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := store.NewFrozenInputs(pool, reader)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	for _, operation := range []struct {
		name string
		run  func() error
	}{
		{"complete", func() error { return publication.Complete(ctx, snapshots) }},
		{"save", func() error { return publication.Save(ctx, snapshots[0]) }},
		{"freeze", func() error { _, err := inputs.FreezeAtDatabaseClock(ctx, uuid.NewString()); return err }},
		{"marshal", func() error { _, err := reader.MarshalUniverse(ctx, pool, uuid.NewString()); return err }},
		{"explain", func() error { _, err := reader.ExplainUniverse(ctx, pool, 200, time.Now()); return err }},
		{"publish", func() error {
			_, err := publication.CommitStreamOnce(ctx, delivery.StreamWriter{Schema: "api_stream"}, 10)
			return err
		}},
		{"prune", func() error { _, err := publication.Prune(ctx, time.Now(), 100); return err }},
		{"health", func() error { _, err := publication.PublicationHealth(ctx, time.Now(), 10); return err }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.run(); err == nil || err.Error() != "closed pool" {
				t.Fatalf("closed database operation: %v", err)
			}
		})
	}
}

func TestStoredPagesAndCacheRejectIncompletePersistence(t *testing.T) {
	pool, publication := persistenceDatabase(t)
	ctx := context.Background()
	snapshots, err := ranker.ComputeRun(publicationPopulatedInput(t), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = publication.Complete(ctx, snapshots); err != nil {
		t.Fatal(err)
	}
	header, err := publication.Current(ctx, "trending")
	if err != nil {
		t.Fatal(err)
	}
	page, err := publication.Page(ctx, header, 0, 10)
	if err != nil || len(page.Items) != 10 || page.Items[0].Rank != 1 || page.Items[9].Rank != 10 {
		t.Fatalf("bounded persisted page: %+v, %v", page, err)
	}
	client := internaltest.NewRedis(t)
	cache, err := store.NewSnapshotCache(client, "persisted-pages")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = cache.Get(ctx, header, 0, 10); !errors.Is(err, redis.Nil) {
		t.Fatalf("missing cached page: %v", err)
	}
	if err = cache.Put(ctx, page, 10); err != nil {
		t.Fatal(err)
	}
	cached, err := cache.Get(ctx, header, 0, 10)
	if err != nil || cached.Header.RunID != header.RunID || len(cached.Items) != 10 || cached.Items[9] != page.Items[9] {
		t.Fatalf("bounded cached page: %+v, %v", cached, err)
	}
	if err = cache.Rebuild(ctx, publication.SnapshotStore, "new"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cache rebuilt absent category: %v", err)
	}
	for _, offset := range []int64{-1, header.Total + 1} {
		if _, err = publication.Page(ctx, header, offset, 10); !errors.Is(err, ranker.ErrUnavailable) {
			t.Fatalf("page offset %d: %v", offset, err)
		}
	}
	var publicHeader []byte
	if err = pool.QueryRow(ctx, "SELECT public_header FROM ranking.snapshots WHERE run_id=$1 AND category='trending'", header.RunID).Scan(&publicHeader); err != nil {
		t.Fatal(err)
	}
	for _, provenance := range []string{`"invalid"`, `{}`} {
		if _, err = pool.Exec(ctx, "UPDATE ranking.snapshots SET public_header=jsonb_set(public_header,'{provenance}',$2::jsonb) WHERE run_id=$1 AND category='trending'", header.RunID, provenance); err != nil {
			t.Fatal(err)
		}
		if _, err = publication.Header(ctx, header.RunID, header.Category); err == nil {
			t.Fatal("invalid stored provenance accepted")
		}
	}
	if _, err = pool.Exec(ctx, "UPDATE ranking.snapshots SET public_header=$2 WHERE run_id=$1 AND category='trending'", header.RunID, publicHeader); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "DELETE FROM ranking.snapshot_entries WHERE run_id=$1 AND category='trending' AND rank=1", header.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err = publication.Page(ctx, header, 0, 100); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("incomplete durable page: %v", err)
	}
	if err = cache.Rebuild(ctx, publication.SnapshotStore, header.Category); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("cache rebuilt incomplete page: %v", err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO ranking.snapshot_entries(run_id,category,rank,pool_address,score) VALUES($1,'trending',1,$2,$3)", header.RunID, page.Items[0].Address, page.Items[0].Score); err != nil {
		t.Fatal(err)
	}
	if err = client.Close(); err != nil {
		t.Fatal(err)
	}
	if err = cache.Rebuild(ctx, publication.SnapshotStore, header.Category); !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("cache rebuild without Redis: %v", err)
	}
}

func TestVisibilityReaderRefusesBrokenTransport(t *testing.T) {
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("{"))
	}))
	defer peer.Close()
	reader, err := store.NewHTTPRankingVisibility(peer.URL, peer.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadRankingVisibility(context.Background()); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("incomplete visibility body: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.ReadRankingVisibility(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled visibility request: %v", err)
	}
	if _, err := store.WireSnapshot(ranker.Header{}, nil); !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("invalid stream snapshot: %v", err)
	}
}

func TestFrozenInputCutoffUsesDatabaseClock(t *testing.T) {
	pool, _ := persistenceDatabase(t)
	ctx := context.Background()
	for _, sql := range []string{
		`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'canonical','indexer-1',1,clock_timestamp()+interval '10 minutes')`,
		`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'canonical',200,200)`,
		`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at) VALUES(125,'canonical',200,'0x` + strings.Repeat("c8", 32) + `',0,true,true,date_trunc('second',clock_timestamp()))`,
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	var before, after time.Time
	if err := pool.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(store.RankingVisibility{Complete: true, Revision: strings.Repeat("a", 64), ObservedAt: before.UTC().Truncate(time.Second), Pools: map[string]bool{}})
	}))
	defer peer.Close()
	visibility, err := store.NewHTTPRankingVisibility(peer.URL, peer.Client())
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.NewCanonicalInputReader("indexer", "stats", "canonical", "projection", visibility)
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := store.NewFrozenInputs(pool, reader)
	if err != nil {
		t.Fatal(err)
	}
	runID := uuid.NewString()
	input, err := inputs.FreezeAtDatabaseClock(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if input.RunID != runID || !input.Complete || input.Candidates == nil || len(input.Candidates) != 0 || input.CutoffTimeUTC < before.Unix() || input.CutoffTimeUTC > after.Unix() || input.CutoffHeight != 200 || input.Provenance == nil || input.Provenance.Validate(input) != nil {
		t.Fatalf("database clock input: %+v", input)
	}
	if _, err = inputs.Freeze(ctx, uuid.NewString(), before, after); err == nil {
		t.Fatal("multiple cutoff times accepted")
	}
	if _, err = inputs.Freeze(ctx, uuid.NewString(), time.Unix(1, 0)); err == nil {
		t.Fatal("cutoff without a full preceding window accepted")
	}
}

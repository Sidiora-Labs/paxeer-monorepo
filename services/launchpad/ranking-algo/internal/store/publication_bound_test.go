package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestDirectPublicationExhaustionSurvivesReopen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := internaltest.NewRankingRuntimeDatabase(t)
	natsURL := os.Getenv("RANKING_TEST_NATS_URL")
	if natsURL == "" {
		t.Fatal("RANKING_TEST_NATS_URL is required")
	}
	conn, err := nats.Connect(natsURL, nats.NoReconnect(), nats.Timeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatal(err)
	}
	streamName := "RANKING_DIRECT_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: streamName, Subjects: []string{"market.rankings.*"}, Storage: jetstream.FileStorage, Duplicates: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = js.DeleteStream(context.Background(), streamName) }()
	publisher, err := natsdriver.NewPublisher(conn, streamName)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", db.Epoch, db.Region)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := ranker.ComputeRun(publicationPopulatedInput(t), false)
	if err != nil {
		t.Fatal(err)
	}
	open := func() (*pgxpool.Pool, *store.PublicationStore) {
		t.Helper()
		pool, err := pgxpool.New(ctx, db.AdminDSN)
		if err != nil {
			t.Fatal(err)
		}
		publication, err := store.NewPublicationStore(pool, "ranking", guard)
		if err != nil {
			pool.Close()
			t.Fatal(err)
		}
		return pool, publication
	}
	pool, publication := open()
	if err = publication.Complete(ctx, snapshots); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	for range 2 * len(snapshots) {
		worked, err := publication.DispatchOnce(ctx, publisher, streamName+"_OTHER", 2)
		if err == nil || worked {
			pool.Close()
			t.Fatalf("foreign stream acknowledgement admitted: worked=%v, %v", worked, err)
		}
	}
	pool.Close()
	pool, publication = open()
	defer pool.Close()
	if worked, err := publication.DispatchOnce(ctx, publisher, streamName, 2); err != nil || worked {
		t.Fatalf("exhausted intent dispatched after reopen: worked=%v, %v", worked, err)
	}
	health, err := publication.PublicationHealth(ctx, time.Now(), 2)
	if err != nil || health.Exhausted != int64(len(snapshots)) || health.Pending != int64(len(snapshots)) {
		t.Fatalf("durable exhaustion: %+v, %v", health, err)
	}
	var exhausted int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM ranking.publication_outbox WHERE attempts=2 AND published_at IS NULL AND broker_stream IS NULL AND broker_sequence IS NULL`).Scan(&exhausted); err != nil || exhausted != len(snapshots) {
		t.Fatalf("outbox exhaustion rows=%d, %v", exhausted, err)
	}
	info, err := stream.Info(ctx)
	if err != nil || info.State.Msgs != uint64(len(snapshots)) {
		t.Fatalf("deduplicated broker messages: %+v, %v", info, err)
	}
}

func TestPublicationBoundRetriesDrainInCategoryOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	db := internaltest.NewRankingRuntimeDatabase(t)
	pool, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	guard, err := queue.NewEpochGuard("recovery", db.Epoch, db.Region)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := store.NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	pace := 100 * time.Millisecond
	if err = publication.SetRetryInterval(pace); err != nil {
		t.Fatal(err)
	}
	want := map[string]map[int64]ranker.Snapshot{}
	input := publicationPopulatedInput(t)
	input.CutoffTimeUTC -= 2
	for revision := int64(1); revision <= 2; revision++ {
		input.RunID = uuid.NewString()
		input.CutoffTimeUTC++
		input.Provenance.WindowEnd = input.CutoffTimeUTC
		input.Provenance.Watermark.ObservedAt = time.Unix(input.CutoffTimeUTC, 0).UTC().Format("2006-01-02T15:04:05.000Z")
		for i := range input.Candidates {
			input.Candidates[i].SourceObservedAt = input.CutoffTimeUTC
			input.Candidates[i].ModerationObservedAt = input.CutoffTimeUTC
			input.Candidates[i].Activity.WindowEnd = input.CutoffTimeUTC
		}
		input.Provenance.UniverseDigest = ranker.CanonicalUniverse(input.Candidates)
		for _, newOnly := range []bool{false, true} {
			snapshots, e := ranker.ComputeRun(input, newOnly)
			if e != nil {
				t.Fatal(e)
			}
			if e = publication.Complete(ctx, snapshots); e != nil {
				t.Fatal(e)
			}
			for _, snapshot := range snapshots {
				if want[snapshot.Category()] == nil {
					want[snapshot.Category()] = map[int64]ranker.Snapshot{}
				}
				want[snapshot.Category()][revision] = snapshot
			}
		}
	}
	_, err = pool.Exec(ctx, `CREATE SEQUENCE ranking.publication_append_calls;
 CREATE FUNCTION ranking.publication_append_available() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$ BEGIN
 IF convert_from(NEW.frame,'UTF8')::jsonb->>'topic'='market.rankings.breakout' AND NEW.sequence=1 THEN
  IF nextval('ranking.publication_append_calls')<=3 THEN RAISE EXCEPTION 'publication unavailable'; END IF;
 END IF;
 RETURN NEW; END $$;
 CREATE TRIGGER publication_append_available BEFORE INSERT ON api_stream.stream_delivery_outbox FOR EACH ROW EXECUTE FUNCTION ranking.publication_append_available()`)
	if err != nil {
		t.Fatal(err)
	}
	writer := delivery.StreamWriter{Schema: "api_stream"}
	for i := 0; i < 2; i++ {
		result, e := publication.CommitStreamOnceWithStatus(ctx, writer, 2)
		if e == nil || !strings.Contains(e.Error(), "publication unavailable") || result.Published || result.Category != "breakout" || result.ReachedBound != (i == 1) {
			t.Fatalf("failure %d: %+v %v", i, result, e)
		}
	}
	lastFailure := time.Now()
	failures := 2
	for {
		result, e := publication.CommitStreamOnceWithStatus(ctx, writer, 2)
		if e != nil {
			if !strings.Contains(e.Error(), "publication unavailable") || !result.AtBound || result.ReachedBound || result.Category != "breakout" {
				t.Fatalf("bounded failure: %+v %v", result, e)
			}
			if time.Since(lastFailure) < pace {
				t.Fatal("bounded append retried before the configured pace")
			}
			failures++
			if failures > 3 {
				t.Fatalf("append still failing after availability returned: %v", e)
			}
			lastFailure = time.Now()
		}
		if result.Published && result.AtBound {
			if failures != 3 || time.Since(lastFailure) < pace {
				t.Fatalf("bounded recovery: failures=%d elapsed=%s", failures, time.Since(lastFailure))
			}
		}
		var attempts, publishedBehind int
		if err = pool.QueryRow(ctx, `SELECT attempts FROM ranking.publication_outbox WHERE category='breakout' AND category_sequence=1`).Scan(&attempts); err != nil || attempts != 2 {
			t.Fatalf("attempt count=%d: %v", attempts, err)
		}
		if !result.Published || result.Category != "breakout" {
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM ranking.publication_outbox WHERE category='breakout' AND category_sequence=2 AND published_at IS NOT NULL AND EXISTS(SELECT 1 FROM ranking.publication_outbox WHERE category='breakout' AND category_sequence=1 AND published_at IS NULL)`).Scan(&publishedBehind); err != nil || publishedBehind != 0 {
				t.Fatalf("category order: %d %v", publishedBehind, err)
			}
		}
		health, e := publication.PublicationHealth(ctx, time.Now().UTC(), 2)
		if e != nil {
			t.Fatal(e)
		}
		if health.Pending == 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	if failures != 3 {
		t.Fatalf("failed rounds=%d, want 3", failures)
	}
	var calls int
	if err = pool.QueryRow(ctx, `SELECT last_value FROM ranking.publication_append_calls`).Scan(&calls); err != nil || calls != 4 {
		t.Fatalf("append calls=%d: %v", calls, err)
	}
	rows, err := pool.Query(ctx, `SELECT p.topic,o.sequence,o.event_id,o.frame FROM api_stream.stream_delivery_outbox o JOIN api_stream.stream_projection p USING(partition_id) ORDER BY p.topic,o.sequence`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string]int64{}
	for rows.Next() {
		var topic, id string
		var sequence int64
		var raw []byte
		if err = rows.Scan(&topic, &sequence, &id, &raw); err != nil {
			t.Fatal(err)
		}
		if _, err = delivery.ParseCommittedStreamFrame(raw); err != nil {
			t.Fatal(err)
		}
		var event v1.RankingEvent
		if err = json.Unmarshal(raw, &event); err != nil {
			t.Fatal(err)
		}
		category := strings.TrimPrefix(topic, "market.rankings.")
		if sequence != seen[category]+1 {
			t.Fatalf("%s sequence=%d after %d", category, sequence, seen[category])
		}
		snapshot, ok := want[category][sequence]
		if !ok {
			t.Fatalf("unexpected frame %s:%d", category, sequence)
		}
		expectedID := uuid.NewSHA1(uuid.MustParse(snapshot.RunID()), []byte(category+":"+snapshot.OutputDigest()+":"+strconv.FormatInt(sequence, 10))).String()
		if id != expectedID || event.EventId != expectedID || event.Payload.SnapshotId != snapshot.RunID() || string(event.Sequence) != fmt.Sprint(sequence) {
			t.Fatalf("stream identity differs for %s:%d", category, sequence)
		}
		seen[category] = sequence
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seen) != len(ranker.Categories) {
		t.Fatalf("published categories=%d", len(seen))
	}
	for category, count := range seen {
		if count != 2 {
			t.Fatalf("%s events=%d, want 2", category, count)
		}
	}
	var invalid int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM ranking.publication_outbox WHERE published_at IS NULL OR attempts>2`).Scan(&invalid); err != nil || invalid != 0 {
		t.Fatalf("undrained or over-bound rows=%d: %v", invalid, err)
	}
}

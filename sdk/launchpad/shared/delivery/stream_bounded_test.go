package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/jackc/pgx/v5/pgxpool"
)

func boundedOutbox(t *testing.T) (context.Context, *pgxpool.Pool, *StreamOutboxRelay) {
	t.Helper()
	ctx, pool, _, _ := orderedPostgres(t)
	if _, err := pool.Exec(ctx, "CREATE SCHEMA recovery"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"150002.sql", "440000.sql"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, StreamDeliveryDDL); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "150010.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO recovery.recovery_control VALUES(true,1,'fixture-loopback',1,$1)", Hash([]byte("bounded-outbox"))); err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", 1, "fixture-loopback")
	if err != nil {
		t.Fatal(err)
	}
	budget, err := NewWorkBudget(1, 1000, 100)
	if err != nil {
		t.Fatal(err)
	}
	relay, err := NewStreamOutboxRelay(pool, orderedBroker(t), StreamOutboxConfig{Stream: "BOUNDED", Owner: "relay", Batch: 10, MaxAttempts: 3, LeaseDuration: time.Minute, PublishTimeout: time.Second, RetryDelay: time.Second}, guard.Assert, budget)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, pool, relay
}

func boundedHistory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, topic string, published, total, recent int64, pendingAt time.Time) int64 {
	t.Helper()
	frame := committedPriceFrame(t)
	frame["topic"] = topic
	wire, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var partition int64
	if err = pool.QueryRow(ctx, "INSERT INTO api_stream.stream_projection(topic,sequence) VALUES($1,0) RETURNING partition_id", topic).Scan(&partition); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame,created_at)
SELECT $1,n,'history_'||$1::bigint||'_'||n,
 convert_to(($2::jsonb||jsonb_build_object('sequence',n::text,'revision',n::text,'eventId','history_'||$1::bigint||'_'||n))::text,'UTF8'),
 CASE WHEN n=$5 THEN clock_timestamp() WHEN n>$3 THEN $6::timestamptz ELSE clock_timestamp()-interval '96 hours' END
FROM generate_series(1,$4::bigint) n`, partition, wire, published, total, recent, pendingAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE api_stream.stream_projection SET sequence=$2,frame=(SELECT frame FROM api_stream.stream_delivery_outbox WHERE partition_id=$1 AND sequence=$2) WHERE partition_id=$1`, partition, total); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO api_stream.stream_delivery_publications(partition_id,sequence,event_id,frame_sha256,state,published_at,lease_expiry_count,acknowledgement_latency_ms)
SELECT partition_id,sequence,event_id,encode(sha256(frame),'hex'),'published',created_at,CASE WHEN sequence=2 THEN 3 ELSE 0 END,9 FROM api_stream.stream_delivery_outbox WHERE partition_id=$1 AND sequence<=$2`, partition, published); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_offsets(partition_id,next_sequence) VALUES($1,$2)", partition, published+1); err != nil {
		t.Fatal(err)
	}
	return partition
}

func boundedStatsSQL(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(gotoken.NewFileSet(), "stream.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var queries []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Stats" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			literal, ok := n.(*ast.BasicLit)
			if ok && literal.Kind == gotoken.STRING {
				value, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(value, "SELECT ") {
					queries = append(queries, value)
				}
			}
			return true
		})
	}
	if len(queries) != 2 {
		t.Fatalf("statistics statements: %d", len(queries))
	}
	return queries
}

func TestOutboxBoundedStatistics(t *testing.T) {
	ctx, pool, relay := boundedOutbox(t)
	pendingAt := time.Now().Add(-2 * time.Minute)
	var partition int64
	for _, digit := range []string{"a", "b", "c", "d", "e"} {
		partition = boundedHistory(t, ctx, pool, "market.price.0x"+strings.Repeat(digit, 40), 60000, 60001, 0, pendingAt)
	}
	for _, table := range []string{"stream_projection", "stream_delivery_offsets", "stream_delivery_outbox", "stream_delivery_publications"} {
		if _, err := pool.Exec(ctx, "ANALYZE api_stream."+table); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := relay.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var frame []byte
	if err = pool.QueryRow(ctx, "SELECT frame FROM api_stream.stream_delivery_outbox WHERE partition_id=$1 AND sequence=60001", partition).Scan(&frame); err != nil {
		t.Fatal(err)
	}
	if stats.PendingRows != 5 || stats.PendingBytes != int64(len(frame))*5 || stats.QuarantinedRows != 0 || stats.LeaseExpiryCount != 15 || stats.MaxAcknowledgementLatencyMS != 9 || stats.GapPartitions != 0 || math.Abs(stats.OldestUnpublishedAgeSeconds-time.Since(pendingAt).Seconds()) > 2 {
		t.Fatalf("pending statistics: %+v", stats)
	}
	for _, sql := range boundedStatsSQL(t) {
		var raw []byte
		if err = pool.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+sql).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var plan any
		if err = json.Unmarshal(raw, &plan); err != nil {
			t.Fatal(err)
		}
		var inspect func(any)
		inspect = func(node any) {
			switch n := node.(type) {
			case map[string]any:
				relation, _ := n["Relation Name"].(string)
				kind, _ := n["Node Type"].(string)
				if (relation == "stream_delivery_outbox" || relation == "stream_delivery_publications") && strings.Contains(kind, "Seq Scan") {
					t.Fatalf("history scan in statistics plan: %s", raw)
				}
				for _, child := range n {
					inspect(child)
				}
			case []any:
				for _, child := range n {
					inspect(child)
				}
			}
		}
		inspect(plan)
	}
	if _, err = pool.Exec(ctx, "UPDATE api_stream.stream_delivery_publications SET state='quarantined',published_at=NULL WHERE partition_id=$1 AND sequence=60000", partition); err != nil {
		t.Fatal(err)
	}
	if err = relay.Ready(ctx, time.Hour); err == nil || err.Error() != "stream: committed publication backlog or source gap" {
		t.Fatalf("quarantine readiness: %v", err)
	}
}

func TestOutboxBoundedRetention(t *testing.T) {
	ctx, pool, relay := boundedOutbox(t)
	partition := boundedHistory(t, ctx, pool, "market.price.0x"+strings.Repeat("a", 40), 25007, 25009, 25006, time.Now().Add(-96*time.Hour))
	other := boundedHistory(t, ctx, pool, "market.price.0x"+strings.Repeat("b", 40), 3, 3, 0, time.Now())
	_, err := pool.Exec(ctx, "UPDATE api_stream.stream_delivery_publications SET state='quarantined',published_at=NULL WHERE partition_id=$1 AND sequence=2", partition)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE api_stream.stream_delivery_publications SET state='pending',published_at=NULL WHERE partition_id=$1 AND sequence=3", partition); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "DELETE FROM api_stream.stream_delivery_publications WHERE partition_id=$1 AND sequence=4", partition); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `CREATE TABLE api_stream.retention_batches(rows bigint NOT NULL);
CREATE FUNCTION api_stream.count_retention_batch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO api_stream.retention_batches SELECT count(*) FROM removed; RETURN NULL; END $$;
CREATE TRIGGER count_retention_batch AFTER DELETE ON api_stream.stream_delivery_outbox REFERENCING OLD TABLE AS removed FOR EACH STATEMENT EXECUTE FUNCTION api_stream.count_retention_batch();`)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := relay.Prune(ctx)
	if err != nil || removed != 20000 {
		t.Fatalf("bounded retention pass: %d %v", removed, err)
	}
	// A new relay shares the durable cadence even when its owner changes.
	restarted := *relay
	restarted.config.Owner = "relay_restart"
	if n, err := restarted.Prune(ctx); err != nil || n != 0 {
		t.Fatalf("retention cadence after restart: %d %v", n, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE api_stream.stream_delivery_offsets SET retention_next_at=clock_timestamp()-interval '1 second' WHERE partition_id=$1", partition); err != nil {
		t.Fatal(err)
	}
	if n, err := restarted.Prune(ctx); err != nil || n != 5004 {
		t.Fatalf("remaining retention batches: %d %v", n, err)
	}
	var batches, total, largest int64
	if err = pool.QueryRow(ctx, "SELECT count(*),sum(rows)::bigint,max(rows) FROM api_stream.retention_batches").Scan(&batches, &total, &largest); err != nil || batches != 6 || total != 25004 || largest != 5000 {
		t.Fatalf("bounded statements: %d %d %d %v", batches, total, largest, err)
	}
	rows, err := pool.Query(ctx, "SELECT partition_id,sequence FROM api_stream.stream_delivery_outbox ORDER BY partition_id,sequence")
	if err != nil {
		t.Fatal(err)
	}
	var kept [][2]int64
	for rows.Next() {
		var row [2]int64
		if err = rows.Scan(&row[0], &row[1]); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		kept = append(kept, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]int64{{partition, 2}, {partition, 3}, {partition, 4}, {partition, 25006}, {partition, 25007}, {partition, 25008}, {partition, 25009}, {other, 3}}
	if len(kept) != len(want) {
		t.Fatalf("retained frames: %v", kept)
	}
	for i := range want {
		if kept[i] != want[i] {
			t.Fatalf("retained frames: %v", kept)
		}
	}
	var orphans int64
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM api_stream.stream_delivery_publications p LEFT JOIN api_stream.stream_delivery_outbox o USING(partition_id,sequence) WHERE o.sequence IS NULL").Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("publication cascade: %d %v", orphans, err)
	}
	stats, err := relay.Stats(ctx)
	if err != nil || stats.LeaseExpiryCount != 3 || stats.MaxAcknowledgementLatencyMS != 9 {
		t.Fatalf("retained publication statistics: %+v %v", stats, err)
	}
}

func TestOutboxBoundedLeaseAndFence(t *testing.T) {
	ctx, pool, relay := boundedOutbox(t)
	partition := boundedHistory(t, ctx, pool, "market.price.0x"+strings.Repeat("a", 40), 6, 6, 0, time.Now())
	if n, err := relay.pruneRetentionBatch(ctx, streamRetentionLease{partition: partition, epoch: 1}); !errors.Is(err, ErrLease) || n != 0 {
		t.Fatalf("cleanup without lease: %d %v", n, err)
	}
	lease, err := relay.acquireRetention(ctx)
	if err != nil || lease == nil {
		t.Fatalf("acquire cleanup lease: %v %v", lease, err)
	}
	competitor := *relay
	competitor.config.Owner = "competitor"
	if n, err := competitor.pruneRetentionBatch(ctx, *lease); !errors.Is(err, ErrLease) || n != 0 {
		t.Fatalf("cleanup with another owner's lease: %d %v", n, err)
	}
	stale := *lease
	stale.epoch++
	if n, err := relay.pruneRetentionBatch(ctx, stale); !errors.Is(err, ErrLease) || n != 0 {
		t.Fatalf("cleanup with stale lease: %d %v", n, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE recovery.recovery_control SET epoch=2 WHERE singleton"); err != nil {
		t.Fatal(err)
	}
	if n, err := relay.pruneRetentionBatch(ctx, *lease); err == nil || n != 0 || !strings.Contains(err.Error(), "restoration epoch fence rejected") {
		t.Fatalf("cleanup behind restoration fence: %d %v", n, err)
	}
	if n, err := relay.Prune(ctx); err == nil || n != 0 {
		t.Fatalf("cleanup pass behind restoration fence: %d %v", n, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE recovery.recovery_control SET epoch=1 WHERE singleton; UPDATE api_stream.stream_delivery_offsets SET retention_expires_at=clock_timestamp()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if n, err := relay.pruneRetentionBatch(ctx, *lease); !errors.Is(err, ErrLease) || n != 0 {
		t.Fatalf("cleanup with expired lease: %d %v", n, err)
	}
	var frames int64
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM api_stream.stream_delivery_outbox").Scan(&frames); err != nil || frames != 6 {
		t.Fatalf("lease and fence preserve frames: %d %v", frames, err)
	}
}

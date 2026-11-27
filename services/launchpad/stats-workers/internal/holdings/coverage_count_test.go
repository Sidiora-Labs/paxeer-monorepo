package holdings

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type coverageCountQueries struct {
	counts  int
	genesis int
	sql     string
	args    []any
}

func (q *coverageCountQueries) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "SELECT count(DISTINCT height)::text FROM indexer.indexer_block_completeness_v1 ") {
		q.counts++
		q.sql = data.SQL
		q.args = append([]any(nil), data.Args...)
	}
	if strings.HasPrefix(data.SQL, "SELECT t.expected_initial_supply IS NOT NULL ") {
		q.genesis++
	}
	return ctx
}

func (*coverageCountQueries) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func coverageCountFixture(t *testing.T) (context.Context, *pgxpool.Pool, *Repository, Transfer, *coverageCountQueries) {
	t.Helper()
	ctx, pool, mint := canonicalLocalFixture(t)
	queries := &coverageCountQueries{}
	cfg := pool.Config()
	cfg.MaxConns = 1
	cfg.ConnConfig.Tracer = queries
	observed, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(observed.Close)
	repo := holderIdleRepository(t, observed)
	if err = repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	return ctx, pool, repo, mint, queries
}

func coverageCountRange(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES
(125,'canonical',11,$1,0,true,true),(125,'canonical',12,$2,0,true,true)`, "0x"+strings.Repeat("c", 64), "0x"+strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=12,typed_complete_height=12"); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageCountNotRunWhenComplete(t *testing.T) {
	ctx, pool, repo, mint, queries := coverageCountFixture(t)
	batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
	want := CanonicalBatch{Applied: 1, Revision: 2, Coverage: "complete"}
	if err != nil || batch != want || queries.counts != 1 || queries.genesis != 1 {
		t.Fatalf("complete: %+v counts=%d genesis=%d: %v", batch, queries.counts, queries.genesis, err)
	}
	want.Applied = 0
	before, row := holderIdleVersion(t, ctx, repo, mint.Token)
	for _, watermark := range []int64{10, 12} {
		if _, err = pool.Exec(ctx, "UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=$1,typed_complete_height=$1", watermark); err != nil {
			t.Fatal(err)
		}
		*queries = coverageCountQueries{}
		batch, err = repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
		if err != nil || batch != want || queries.counts != 0 || queries.genesis != 0 {
			t.Fatalf("complete idle round at %d: %+v counts=%d genesis=%d: %v", watermark, batch, queries.counts, queries.genesis, err)
		}
		after, current := holderIdleVersion(t, ctx, repo, mint.Token)
		if after != before || current != row {
			t.Fatalf("complete idle round changed token: %s %s -> %s %s", before, row, after, current)
		}
	}
}

func TestCoverageCountCompletesACoin(t *testing.T) {
	ctx, pool, repo, mint, queries := coverageCountFixture(t)
	coverageCountRange(t, ctx, pool)
	before, err := repo.Snapshot(ctx, mint.Token, 10)
	if err != nil || before.Coverage != "incomplete" || before.Revision != 0 {
		t.Fatalf("discovered token: %+v: %v", before, err)
	}
	batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
	want := CanonicalBatch{Applied: 1, Revision: 2, Coverage: "complete"}
	if err != nil || batch != want || queries.counts != 1 || queries.genesis != 1 {
		t.Fatalf("complete range: %+v counts=%d genesis=%d: %v", batch, queries.counts, queries.genesis, err)
	}
	holderIdleReference(t, ctx, repo, mint, mint, want, []Balance{{Address: mint.To, Raw: "100", Category: "unknown"}})
}

func TestCoverageCountKeepsAGap(t *testing.T) {
	for _, gap := range []string{"fetch_complete", "required_decode_complete", "missing"} {
		t.Run(gap, func(t *testing.T) {
			ctx, pool, repo, mint, queries := coverageCountFixture(t)
			coverageCountRange(t, ctx, pool)
			statement := "DELETE FROM indexer.indexer_block_completeness WHERE height=11"
			if gap != "missing" {
				statement = "UPDATE indexer.indexer_block_completeness SET " + gap + "=false WHERE height=11"
			}
			if _, err := pool.Exec(ctx, statement); err != nil {
				t.Fatal(err)
			}
			batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
			want := CanonicalBatch{Applied: 1, Revision: 1, Coverage: "incomplete"}
			if err != nil || batch != want || queries.counts != 1 || queries.genesis != 0 {
				t.Fatalf("range with gap: %+v counts=%d genesis=%d: %v", batch, queries.counts, queries.genesis, err)
			}
			holderIdleReference(t, ctx, repo, mint, mint, want, []Balance{{Address: mint.To, Raw: "100", Category: "unknown"}})
			before, row := holderIdleVersion(t, ctx, repo, mint.Token)
			*queries = coverageCountQueries{}
			want.Applied = 0
			batch, err = repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
			if err != nil || batch != want || queries.counts != 1 || queries.genesis != 0 {
				t.Fatalf("idle range with gap: %+v counts=%d genesis=%d: %v", batch, queries.counts, queries.genesis, err)
			}
			after, current := holderIdleVersion(t, ctx, repo, mint.Token)
			if after != before || current != row {
				t.Fatal("unchanged incomplete coverage changed the token")
			}
		})
	}
}

type coverageCountPlan struct {
	Relation string              `json:"Relation Name"`
	Index    string              `json:"Index Cond"`
	Filter   string              `json:"Filter"`
	Plans    []coverageCountPlan `json:"Plans"`
}

func TestCoverageCountUsesTheRange(t *testing.T) {
	ctx, pool, repo, mint, queries := coverageCountFixture(t)
	coverageCountRange(t, ctx, pool)
	batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
	if err != nil || batch != (CanonicalBatch{Applied: 1, Revision: 2, Coverage: "complete"}) || queries.counts != 1 {
		t.Fatalf("range count: %+v counts=%d: %v", batch, queries.counts, err)
	}
	const statement = "SELECT count(DISTINCT height)::text FROM indexer.indexer_block_completeness_v1 WHERE chain_id=125 AND stream_id=$1 AND height BETWEEN $2 AND $3 AND fetch_complete AND required_decode_complete"
	if queries.sql != statement || len(queries.args) != 3 || queries.args[0] != "canonical" || queries.args[1] != int64(10) || queries.args[2] != int64(12) {
		t.Fatalf("count statement: %s args=%#v", queries.sql, queries.args)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackTransaction(ctx, tx)
	if _, err = tx.Exec(ctx, "SET LOCAL enable_seqscan=off"); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err = tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+queries.sql, queries.args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plans []struct {
		Plan coverageCountPlan `json:"Plan"`
	}
	if err = json.Unmarshal(raw, &plans); err != nil || len(plans) != 1 {
		t.Fatalf("count plan: %s: %v", raw, err)
	}
	lower := regexp.MustCompile(`height >= '?10'?`)
	upper := regexp.MustCompile(`height <= '?12'?`)
	reads := 0
	var inspect func(coverageCountPlan)
	inspect = func(plan coverageCountPlan) {
		if strings.Contains(plan.Filter, "height") {
			t.Fatalf("height bound in a filter: %s", raw)
		}
		if plan.Relation == "indexer_block_completeness" {
			reads++
			if !lower.MatchString(plan.Index) || !upper.MatchString(plan.Index) {
				t.Fatalf("count must use both height bounds in its index condition: %s", raw)
			}
		}
		for _, child := range plan.Plans {
			inspect(child)
		}
	}
	inspect(plans[0].Plan)
	if reads == 0 {
		t.Fatalf("count plan does not read block completeness: %s", raw)
	}
}

func TestCoverageCountCreationOutsideBigint(t *testing.T) {
	for _, creation := range []string{"9223372036854775808", "115792089237316195423570985008687907853269984665640564039457584007913129639935"} {
		t.Run(creation, func(t *testing.T) {
			ctx, _, repo, _, queries := coverageCountFixture(t)
			token := addr(87)
			if err := repo.Discover(ctx, token, 6, creation, false); err != nil {
				t.Fatal(err)
			}
			batch, err := repo.CatchUpCanonical(ctx, token, "canonical", 10)
			if err != nil || batch != (CanonicalBatch{Coverage: "incomplete"}) || queries.counts != 0 || queries.genesis != 0 {
				t.Fatalf("creation outside bigint: %+v counts=%d genesis=%d: %v", batch, queries.counts, queries.genesis, err)
			}
			snapshot, err := repo.Snapshot(ctx, token, 10)
			if err != nil || snapshot.Coverage != "incomplete" || snapshot.Revision != 0 || snapshot.Supply != "0" {
				t.Fatalf("creation outside bigint snapshot: %+v: %v", snapshot, err)
			}
		})
	}
}

func TestCoverageCountRequiresInitialSupply(t *testing.T) {
	ctx, pool, repo, mint, queries := coverageCountFixture(t)
	coverageCountRange(t, ctx, pool)
	if _, err := pool.Exec(ctx, "UPDATE idle_holders.tokens SET expected_initial_supply=99 WHERE token=$1", mint.Token); err != nil {
		t.Fatal(err)
	}
	batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
	if err != nil || batch != (CanonicalBatch{Applied: 1, Revision: 1, Coverage: "incomplete"}) || queries.counts != 1 || queries.genesis != 1 {
		t.Fatalf("initial supply mismatch: %+v counts=%d genesis=%d: %v", batch, queries.counts, queries.genesis, err)
	}
}

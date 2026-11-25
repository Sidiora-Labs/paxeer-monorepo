package holdings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func holderIdleRepository(t *testing.T, pool *pgxpool.Pool) *Repository {
	t.Helper()
	ddl, err := SchemaSQL("idle_holders")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, "idle_holders")
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func holderIdleTransfer(t *testing.T, ctx context.Context, pool *pgxpool.Pool, transfer Transfer) {
	t.Helper()
	topics, err := json.Marshal([]string{crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")).Hex(), "0x" + strings.Repeat("0", 24) + transfer.From[2:], "0x" + strings.Repeat("0", 24) + transfer.To[2:]})
	if err != nil {
		t.Fatal(err)
	}
	amount, err := UInt(transfer.Amount)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(transfer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2::bigint,$3::bigint,$4::bigint,$5,$6,$7,$8)`, transfer.TransactionHash, transfer.LogIndex, transfer.Height, transfer.TransactionIndex, transfer.BlockHash, transfer.Token, topics, amount.FillBytes(make([]byte, 32))); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_token_transfers(chain_id,transaction_hash,log_index,token,from_address,to_address,amount_raw,height,transaction_index,block_hash,provenance,payload) VALUES(125,$1,$2::bigint,$3,$4,$5,$6::numeric,$7::bigint,$8::bigint,$9,'{}',$10)`, transfer.TransactionHash, transfer.LogIndex, transfer.Token, transfer.From, transfer.To, transfer.Amount, transfer.Height, transfer.TransactionIndex, transfer.BlockHash, payload); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE indexer.indexer_block_completeness SET raw_count=raw_count+1 WHERE height=$1::bigint`,
		`UPDATE indexer.indexer_ingest_commits SET record_count=record_count+1 WHERE last_complete_height=$1::bigint`,
	} {
		if _, err = pool.Exec(ctx, statement, transfer.Height); err != nil {
			t.Fatal(err)
		}
	}
}

func holderIdleVersion(t *testing.T, ctx context.Context, repo *Repository, token string) (string, string) {
	t.Helper()
	var version, row string
	if err := repo.pool.QueryRow(ctx, "SELECT xmin::text,row_to_json(t)::text FROM "+repo.schema+".tokens t WHERE token=$1", token).Scan(&version, &row); err != nil {
		t.Fatal(err)
	}
	return version, row
}

func holderIdleReference(t *testing.T, ctx context.Context, repo *Repository, mint, last Transfer, want CanonicalBatch, balances []Balance) {
	t.Helper()
	snapshot, err := repo.Snapshot(ctx, mint.Token, 10)
	expected := Snapshot{Token: mint.Token, Decimals: 6, Supply: "100", Revision: want.Revision, Coverage: want.Coverage, Position: &last, Balances: balances}
	if err != nil || !reflect.DeepEqual(snapshot, expected) {
		t.Fatalf("holders: %+v, want %+v: %v", snapshot, expected, err)
	}
	var decimals int
	var creation, supply, expectedSupply, creationHash, coverage string
	var revision int64
	var firstBody, lastBody []byte
	if err = repo.pool.QueryRow(ctx, "SELECT decimals,creation_height::text,supply::text,expected_initial_supply::text,creation_transaction_hash,revision,coverage,first_event,last_event FROM "+repo.schema+".tokens WHERE token=$1", mint.Token).Scan(&decimals, &creation, &supply, &expectedSupply, &creationHash, &revision, &coverage, &firstBody, &lastBody); err != nil {
		t.Fatal(err)
	}
	var firstEvent, lastEvent Transfer
	if err = json.Unmarshal(firstBody, &firstEvent); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(lastBody, &lastEvent); err != nil {
		t.Fatal(err)
	}
	if decimals != 6 || creation != "10" || supply != "100" || expectedSupply != "100" || creationHash != mint.TransactionHash || revision != want.Revision || coverage != want.Coverage || firstEvent != mint || lastEvent != last {
		t.Fatalf("token: decimals=%d creation=%s supply=%s expected=%s transaction=%s revision=%d coverage=%s first=%+v last=%+v", decimals, creation, supply, expectedSupply, creationHash, revision, coverage, firstEvent, lastEvent)
	}
}

func TestHolderIdlePassReferenceTransitions(t *testing.T) {
	for _, state := range []string{"complete", "incomplete", "quarantined"} {
		t.Run(state, func(t *testing.T) {
			ctx, pool, mint := canonicalLocalFixture(t)
			repo := holderIdleRepository(t, pool)
			if state == "incomplete" {
				if _, err := pool.Exec(ctx, "UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=11,typed_complete_height=11"); err != nil {
					t.Fatal(err)
				}
			}
			if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
				t.Fatal(err)
			}
			want := CanonicalBatch{Applied: 1, Revision: 2, Coverage: "complete"}
			if state == "incomplete" {
				want.Revision, want.Coverage = 1, "incomplete"
			}
			batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
			if err != nil || batch != want {
				t.Fatalf("mint: %+v, want %+v: %v", batch, want, err)
			}
			balances := []Balance{{Address: mint.To, Raw: "100", Category: "unknown"}}
			holderIdleReference(t, ctx, repo, mint, mint, want, balances)
			before, _ := holderIdleVersion(t, ctx, repo, mint.Token)
			amount := "40"
			if state == "quarantined" {
				amount = "101"
			}
			move := event(mint.Token, mint.To, addr(86), amount, 2)
			holderIdleTransfer(t, ctx, pool, move)
			want.Revision++
			last := move
			if state == "quarantined" {
				want.Applied, want.Incident, want.Coverage = 0, true, "quarantined"
				last = mint
			} else {
				balances = []Balance{{Address: mint.To, Raw: "60", Category: "unknown"}, {Address: move.To, Raw: "40", Category: "unknown"}}
			}
			batch, err = repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
			if err != nil || batch != want {
				t.Fatalf("transfer: %+v, want %+v: %v", batch, want, err)
			}
			after, _ := holderIdleVersion(t, ctx, repo, mint.Token)
			if before == after {
				t.Fatal("transfer retained the token row version")
			}
			holderIdleReference(t, ctx, repo, mint, last, want, balances)
			want.Applied = 0
			batch, err = repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
			if err != nil || batch != want {
				t.Fatalf("settled: %+v, want %+v: %v", batch, want, err)
			}
			holderIdleReference(t, ctx, repo, mint, last, want, balances)
		})
	}
}

type holderIdleQueries struct {
	statements int
	writes     int64
}

func (q *holderIdleQueries) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if data.SQL != "begin isolation level repeatable read" && data.SQL != "commit" && data.SQL != "rollback" {
		q.statements++
	}
	return ctx
}

func (q *holderIdleQueries) TraceQueryEnd(_ context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.CommandTag.Insert() || data.CommandTag.Update() || data.CommandTag.Delete() {
		q.writes += data.CommandTag.RowsAffected()
	}
}

func TestHolderIdlePassPreservesRows(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	queries := &holderIdleQueries{}
	cfg := pool.Config()
	cfg.ConnConfig.Tracer = queries
	observed, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(observed.Close)
	repo := holderIdleRepository(t, observed)
	if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	batch, err := repo.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "10", 10)
	if err != nil || batch != (CanonicalBatch{Applied: 1, Revision: 2, Coverage: "complete"}) {
		t.Fatalf("complete: %+v %v", batch, err)
	}
	want := batch
	want.Applied = 0
	before, row := holderIdleVersion(t, ctx, repo, mint.Token)
	if _, err = pool.Exec(ctx, `CREATE FUNCTION idle_holders.preserve_rows() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'settled holdings row changed'; END $$`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"tokens", "balances", "transfers", "provenance", "aggregates", "classifications", "incidents", "legacy_rows", "snapshots", "snapshot_balances", "checkpoints"} {
		if _, err = pool.Exec(ctx, "CREATE TRIGGER preserve_rows AFTER INSERT OR UPDATE OR DELETE ON idle_holders."+table+" FOR EACH ROW EXECUTE FUNCTION idle_holders.preserve_rows()"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 10; i++ {
		*queries = holderIdleQueries{}
		if err = repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
			t.Fatal(err)
		}
		batch, err = repo.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "10", 10)
		if err != nil || batch != want {
			t.Fatalf("idle pass %d: %+v, want %+v: %v", i, batch, want, err)
		}
		if queries.statements != 9 || queries.writes != 0 {
			t.Fatalf("idle statements=%d writes=%d; want 9 and 0", queries.statements, queries.writes)
		}
		after, current := holderIdleVersion(t, ctx, repo, mint.Token)
		if after != before || current != row {
			t.Fatalf("idle pass %d changed token: %s %s -> %s %s", i, before, row, after, current)
		}
	}
}

func TestHolderIdlePassRetriesConcurrentUpdate(t *testing.T) {
	base, pool, mint := canonicalLocalFixture(t)
	ctx, cancel := context.WithTimeout(base, 15*time.Second)
	defer cancel()
	repo := holderIdleRepository(t, pool)
	if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	if batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10); err != nil || batch.Coverage != "complete" {
		t.Fatalf("complete: %+v %v", batch, err)
	}
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackTransaction(context.Background(), blocker)
	var revision int64
	if err = blocker.QueryRow(ctx, `SELECT revision FROM idle_holders.tokens WHERE token=$1 FOR UPDATE`, mint.Token).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	pid := blocker.Conn().PgConn().PID()
	type result struct {
		batch CanonicalBatch
		err   error
	}
	finished := make(chan result, 1)
	go func() {
		batch, err := repo.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "10", 10)
		finished <- result{batch, err}
	}()
	poll := time.NewTicker(5 * time.Millisecond)
	defer poll.Stop()
	for {
		var waiting bool
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND $1::integer=ANY(pg_blocking_pids(pid)) AND query LIKE 'SELECT creation_height::text,last_event,revision,coverage FROM %')`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case got := <-finished:
			t.Fatalf("catchup ended before waiting for the token lock: %+v %v", got.batch, got.err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-poll.C:
		}
	}
	if _, err = blocker.Exec(ctx, `UPDATE idle_holders.tokens SET revision=revision+1 WHERE token=$1`, mint.Token); err != nil {
		t.Fatal(err)
	}
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var concurrent result
	select {
	case concurrent = <-finished:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	alone, err := repo.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "10", 10)
	want := CanonicalBatch{Revision: revision + 1, Coverage: "complete"}
	if concurrent.err != nil || err != nil || concurrent.batch != want || alone != want {
		t.Fatalf("concurrent=%+v %v alone=%+v %v want=%+v", concurrent.batch, concurrent.err, alone, err, want)
	}
}

func TestHolderIdlePassRetryBoundsAndRollback(t *testing.T) {
	for _, tc := range []struct {
		code     string
		failures int
		attempts int
		wantCode string
	}{
		{"40001", 4, 4, "40001"},
		{"40P01", 4, 4, "40P01"},
		{"40001", 1, 2, ""},
		{"40P01", 1, 2, ""},
		{"23514", 1, 1, "23514"},
	} {
		t.Run(tc.code+"_"+strconv.Itoa(tc.failures), func(t *testing.T) {
			ctx, pool, mint := canonicalLocalFixture(t)
			repo := holderIdleRepository(t, pool)
			if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
				t.Fatal(err)
			}
			before, row := holderIdleVersion(t, ctx, repo, mint.Token)
			var logs bytes.Buffer
			repo.observer.logger = slog.New(slog.NewTextHandler(&logs, nil))
			attempts := 0
			transactions := map[string]bool{}
			repo.canonicalBeforeCommit = func(ctx context.Context, tx pgx.Tx) error {
				attempts++
				var xid string
				if err := tx.QueryRow(ctx, "SELECT pg_current_xact_id()::text").Scan(&xid); err != nil {
					return err
				}
				if transactions[xid] {
					t.Error("retry reused a transaction")
				}
				transactions[xid] = true
				if attempts > tc.failures {
					return nil
				}
				_, err := tx.Exec(ctx, "DO $$ BEGIN RAISE EXCEPTION USING ERRCODE='"+tc.code+"', MESSAGE='holder transaction rejected'; END $$")
				return err
			}
			batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
			if attempts != tc.attempts || len(transactions) != tc.attempts {
				t.Fatalf("attempts=%d transactions=%v want=%d", attempts, transactions, tc.attempts)
			}
			if strings.Contains(logs.String(), "level=ERROR") || strings.Contains(logs.String(), "level=WARN") {
				t.Fatalf("retry logged an error or warning: %s", logs.String())
			}
			committed := strings.Count(logs.String(), "holdings transfer committed")
			if tc.wantCode == "" {
				if committed != 1 {
					t.Fatalf("committed observations=%d, want 1: %s", committed, logs.String())
				}
				if err != nil || batch != (CanonicalBatch{Applied: 1, Revision: 2, Coverage: "complete"}) {
					t.Fatalf("retried mint: %+v %v", batch, err)
				}
				holderIdleReference(t, ctx, repo, mint, mint, batch, []Balance{{Address: mint.To, Raw: "100", Category: "unknown"}})
				return
			}
			if committed != 0 {
				t.Fatalf("rejected transaction emitted committed observations: %s", logs.String())
			}
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != tc.wantCode || pgerr.Message != "holder transaction rejected" {
				t.Fatalf("catchup: %+v %v; want SQLSTATE %s", batch, err, tc.wantCode)
			}
			after, current := holderIdleVersion(t, ctx, repo, mint.Token)
			if before != after || row != current {
				t.Fatalf("rejected transactions changed token: %s %s -> %s %s", before, row, after, current)
			}
			for _, table := range []string{"balances", "aggregates", "transfers", "provenance"} {
				var count int
				if err = pool.QueryRow(ctx, "SELECT count(*) FROM idle_holders."+table).Scan(&count); err != nil || count != 0 {
					t.Fatalf("rejected %s rows=%d: %v", table, count, err)
				}
			}
		})
	}
}

func TestHolderIdlePassDiscoveryRetries(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	repo := holderIdleRepository(t, pool)
	if err := repo.Discover(ctx, mint.Token, 6, "10", false); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE SEQUENCE idle_holders.discovery_attempt;
CREATE FUNCTION idle_holders.serialize_discovery() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
IF nextval('idle_holders.discovery_attempt') <= 2 THEN RAISE EXCEPTION USING ERRCODE='40001', MESSAGE='holder discovery serialization'; END IF;
RETURN NEW; END $$;
CREATE TRIGGER serialize_discovery BEFORE UPDATE ON idle_holders.tokens FOR EACH ROW EXECUTE FUNCTION idle_holders.serialize_discovery()`); err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	var attempts int
	if err := pool.QueryRow(ctx, "SELECT last_value FROM idle_holders.discovery_attempt").Scan(&attempts); err != nil || attempts != 3 {
		t.Fatalf("discovery attempts=%d: %v", attempts, err)
	}
	before, row := holderIdleVersion(t, ctx, repo, mint.Token)
	if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	after, current := holderIdleVersion(t, ctx, repo, mint.Token)
	if before != after || row != current {
		t.Fatal("settled discovery changed the token")
	}
}

func TestHolderIdlePassDiscoveryPreservesMetadata(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	repo := holderIdleRepository(t, pool)
	if err := repo.Discover(ctx, mint.Token, 6, "10", false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		supply, hash any
		conflict     bool
	}{
		{nil, nil, false},
		{"100", nil, false},
		{nil, mint.TransactionHash, false},
		{"99", mint.TransactionHash, true},
		{"99", nil, true},
		{nil, strings.Repeat("f", 64), true},
		{"100", strings.Repeat("f", 64), true},
	} {
		if _, err := pool.Exec(ctx, `UPDATE idle_holders.tokens SET expected_initial_supply=$1::numeric,creation_transaction_hash=$2`, tc.supply, tc.hash); err != nil {
			t.Fatal(err)
		}
		before, row := holderIdleVersion(t, ctx, repo, mint.Token)
		err := repo.DiscoverCanonical(ctx, mint.Token, "canonical")
		if tc.conflict {
			if !errors.Is(err, ErrConflict) {
				t.Fatalf("conflicting metadata: %v", err)
			}
			after, current := holderIdleVersion(t, ctx, repo, mint.Token)
			if before != after || row != current {
				t.Fatal("conflicting metadata changed the token")
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHolderIdlePassCoverageTransitions(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	repo := holderIdleRepository(t, pool)
	if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	if batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10); err != nil || batch != (CanonicalBatch{Applied: 1, Revision: 2, Coverage: "complete"}) {
		t.Fatalf("complete: %+v %v", batch, err)
	}
	before, _ := holderIdleVersion(t, ctx, repo, mint.Token)
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=11,typed_complete_height=11`); err != nil {
		t.Fatal(err)
	}
	batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
	want := CanonicalBatch{Revision: 3, Coverage: "incomplete"}
	if err != nil || batch != want {
		t.Fatalf("unproven range: %+v, want %+v: %v", batch, want, err)
	}
	after, row := holderIdleVersion(t, ctx, repo, mint.Token)
	if before == after {
		t.Fatal("coverage change retained the token row version")
	}
	batch, err = repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
	if err != nil || batch != want {
		t.Fatalf("unproven idle range: %+v, want %+v: %v", batch, want, err)
	}
	settled, current := holderIdleVersion(t, ctx, repo, mint.Token)
	if after != settled || row != current {
		t.Fatal("unchanged incomplete coverage changed the token")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'canonical',11,$1,0,true,true)`, "0x"+strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	batch, err = repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
	want = CanonicalBatch{Revision: 4, Coverage: "complete"}
	if err != nil || batch != want {
		t.Fatalf("proven range: %+v, want %+v: %v", batch, want, err)
	}
	holderIdleReference(t, ctx, repo, mint, mint, want, []Balance{{Address: mint.To, Raw: "100", Category: "unknown"}})
}

func TestHolderIdlePassRetryCancellation(t *testing.T) {
	base, pool, mint := canonicalLocalFixture(t)
	repo := holderIdleRepository(t, pool)
	if err := repo.DiscoverCanonical(base, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	attempts := 0
	repo.canonicalBeforeCommit = func(ctx context.Context, tx pgx.Tx) error {
		attempts++
		_, err := tx.Exec(ctx, `DO $$ BEGIN RAISE EXCEPTION USING ERRCODE='40001', MESSAGE='holder transaction serialization'; END $$`)
		cancel()
		return err
	}
	if _, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10); !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("canceled retries=%d: %v", attempts, err)
	}
}

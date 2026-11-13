package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This SQL projection persists the archive's actual raw records and compares
// their ordered bytes with the canonical source, without replacing the engine,
// archive, object transport, store or promotion checks under test.
type repair41Projection struct {
	archive *Archive
}

func (p repair41Projection) ApplyRawTx(ctx context.Context, tx pgx.Tx, generation string, record ArchiveRaw) error {
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO "+p.archive.store.table("repair_41_projection")+"(generation,height,transaction_index,log_index,body) VALUES($1,$2,$3,$4,$5)", generation, record.Height, int64(record.TransactionIndex), int64(record.LogIndex), body)
	return err
}

func (p repair41Projection) ProjectionDigestTx(ctx context.Context, tx pgx.Tx, generation string) (string, error) {
	rows, err := tx.Query(ctx, "SELECT body FROM "+p.archive.store.table("repair_41_projection")+" WHERE generation=$1 ORDER BY height,transaction_index,log_index", generation)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var raw bytes.Buffer
	for rows.Next() {
		var body []byte
		if err = rows.Scan(&body); err != nil {
			return "", err
		}
		raw.Write(body)
		raw.WriteByte('\n')
	}
	return Hash(raw.Bytes()), rows.Err()
}

func (p repair41Projection) CanonicalDigestTx(ctx context.Context, tx pgx.Tx, manifest ArchiveManifest) (string, error) {
	if err := p.archive.ValidateSourceTx(ctx, tx, manifest); err != nil {
		return "", err
	}
	var raw bytes.Buffer
	for _, block := range manifest.Blocks {
		_, body, err := p.archive.sourceBlock(ctx, tx, block.Height)
		if err != nil {
			return "", err
		}
		raw.Write(body)
	}
	return Hash(raw.Bytes()), nil
}

func repair41Open(t *testing.T, schema string) (context.Context, *pgxpool.Pool, *Archive, *ReplayEngine) {
	t.Helper()
	requireArchiveFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	cfg, err := pgxpool.ParseConfig(os.Getenv("DELIVERY_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 25432 || cfg.ConnConfig.Database != "kindle_fixture" || !strings.HasPrefix(schema, "kd_") || !token.MatchString(schema) || len(schema) > 63 {
		t.Fatal("explicit private PostgreSQL fixture and bounded kd_ schema required")
	}
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store, err := NewStore(pool, schema, fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(os.Getenv("DELIVERY_ARCHIVE_BUCKET"), "kd-") {
		t.Fatal("explicit private kd- object-storage bucket required")
	}
	objects, err := storage.New(ctx, storage.Config{Endpoint: os.Getenv("DELIVERY_ARCHIVE_ENDPOINT"), AccessKeyID: os.Getenv("DELIVERY_ARCHIVE_ACCESS_KEY"), SecretAccessKey: os.Getenv("DELIVERY_ARCHIVE_SECRET_KEY"), Bucket: os.Getenv("DELIVERY_ARCHIVE_BUCKET")})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := NewArchive(store, objects, schema, os.Getenv("DELIVERY_ARCHIVE_STREAM"), os.Getenv("DELIVERY_ARCHIVE_DEPLOYMENT"), os.Getenv("DELIVERY_ARCHIVE_DECODER_SHA"), os.Getenv("DELIVERY_ARCHIVE_ABI_SHA"), os.Getenv("DELIVERY_ARCHIVE_CODE_SHA"))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewReplayEngine(archive, "repair_41", "generation_41", "owner_41", repair41Projection{archive}, verifyWebhookFixtureAdmin, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, pool, archive, engine
}

func repair41Fixture(t *testing.T) (context.Context, *pgxpool.Pool, *Archive, *ReplayEngine, ArchiveManifest, string) {
	t.Helper()
	schema := "kd_repair_41_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, pool, archive, engine := repair41Open(t, schema)
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	for _, ddl := range []func(string) (string, error){SchemaDDL, ReplayDDL} {
		sql, err := ddl(schema)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, "CREATE TABLE "+archive.store.table("repair_41_projection")+"(generation text NOT NULL,height bigint NOT NULL,transaction_index bigint NOT NULL,log_index bigint NOT NULL,body bytea NOT NULL,PRIMARY KEY(generation,height,transaction_index,log_index))"); err != nil {
		t.Fatal(err)
	}
	from, err := strconv.ParseInt(os.Getenv("DELIVERY_ARCHIVE_FROM"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	to, err := strconv.ParseInt(os.Getenv("DELIVERY_ARCHIVE_TO"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	manifest, digest, err := archive.Capture(ctx, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.RawCount == 0 {
		t.Fatal("restart proof requires actual archived source records")
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := archive.objects.Delete(cleanup, archive.key("manifests", digest)); err != nil {
			t.Error(err)
		}
		for _, block := range manifest.Blocks {
			if err := archive.objects.Delete(cleanup, archive.key("segments", block.SegmentHash)); err != nil {
				t.Error(err)
			}
		}
	})
	for _, block := range manifest.Blocks {
		_, records, err := archive.ReadBlock(ctx, digest, block.Height)
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range records {
			body, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			e := Envelope{EventID: Hash(body), EventType: "Swap", SchemaVersion: 1, ChainID: 125, SourceOrder: SourceOrder{strconv.FormatInt(record.Height, 10), strconv.FormatUint(uint64(record.TransactionIndex), 10), strconv.FormatUint(uint64(record.LogIndex), 10)}, EmittedAt: time.Unix(1, 0).UTC(), TraceID: Hash(body), PayloadHash: Hash(body), Payload: body}
			if err = archive.store.Produce(ctx, []Pending{{fixtureSubject, e}}, func(ctx context.Context, tx pgx.Tx) error {
				for _, path := range []string{"legacy", "durable"} {
					if err := archive.store.RecordShadowTx(ctx, tx, digest, path, fixtureSubject, e); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = archive.store.CompareShadowGeneration(ctx, digest, []string{fixtureSubject}, engine.consumer, engine.generation); err != nil {
		t.Fatal(err)
	}
	return ctx, pool, archive, engine, manifest, digest
}

func repair41Rebuild(t *testing.T, ctx context.Context, engine *ReplayEngine, digest string) {
	t.Helper()
	budget, err := NewWorkBudget(1, 10000, 1000)
	if err != nil {
		t.Fatal(err)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	credential := webhookAdminCredential{timestamp, signWebhookBody(strings.Repeat("admin-secret", 4), timestamp, "replay.promote")}
	ctx = context.WithValue(ctx, webhookAdminContextKey{}, credential)
	if err = engine.Rebuild(ctx, digest, budget, true, 0); err != nil {
		t.Fatal("configured startup rebuild:", err)
	}
	status, err := engine.Status(ctx, 0)
	if err != nil || status.Status != "active" || !status.Ready || status.Gap {
		t.Fatalf("rebuild readiness after startup: %+v, %v", status, err)
	}
}

func repair41Snapshot(t *testing.T, ctx context.Context, archive *Archive) string {
	t.Helper()
	var snapshot strings.Builder
	for _, table := range []string{"delivery_rebuilds", "delivery_read_aliases", "delivery_cutover_history", "delivery_generation_paths", "repair_41_projection"} {
		var body string
		if err := archive.store.pool.QueryRow(ctx, "SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]'::jsonb)::text FROM "+archive.store.table(table)+" t").Scan(&body); err != nil {
			t.Fatal(err)
		}
		snapshot.WriteString(body)
	}
	return snapshot.String()
}

func TestRepair_41_ActiveRebuildRestart(t *testing.T) {
	if os.Getenv("DELIVERY_REPAIR_41_CHILD") == "1" {
		ctx, _, _, engine := repair41Open(t, os.Getenv("DELIVERY_REPAIR_41_SCHEMA"))
		repair41Rebuild(t, ctx, engine, os.Getenv("DELIVERY_REPAIR_41_MANIFEST"))
		return
	}
	ctx, pool, archive, engine, manifest, digest := repair41Fixture(t)
	runProcess := func() {
		t.Helper()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRepair_41_ActiveRebuildRestart$", "-test.v")
		cmd.Env = append(os.Environ(), "DELIVERY_REPAIR_41_CHILD=1", "DELIVERY_REPAIR_41_SCHEMA="+strings.Trim(archive.store.schema, `"`), "DELIVERY_REPAIR_41_MANIFEST="+digest)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("rebuild process: %v\n%s", err, output)
		}
	}
	runProcess()
	// A completed rebuild must not need its previous process's live lease.
	if _, err := pool.Exec(ctx, "UPDATE "+archive.store.table("delivery_rebuilds")+" SET lease_until=clock_timestamp()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	before := repair41Snapshot(t, ctx, archive)
	runProcess()
	if after := repair41Snapshot(t, ctx, archive); after != before {
		t.Fatal("restart changed persisted rebuild, projection, lease, alias or cutover history")
	}
	var revision, cutovers, visible int64
	if err := pool.QueryRow(ctx, "SELECT a.revision,(SELECT count(*) FROM "+archive.store.table("delivery_cutover_history")+"),(SELECT count(*) FROM "+archive.store.table("repair_41_projection")+" p WHERE p.generation=a.generation) FROM "+archive.store.table("delivery_read_aliases")+" a WHERE a.consumer_id=$1 AND a.generation=$2", engine.consumer, engine.generation).Scan(&revision, &cutovers, &visible); err != nil {
		t.Fatal(err)
	}
	if revision != 1 || cutovers != 1 || visible != manifest.RawCount {
		t.Fatalf("recovered consumer view: revision=%d cutovers=%d records=%d want=%d", revision, cutovers, visible, manifest.RawCount)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackBounded(tx)
	projection := repair41Projection{archive}
	actual, err := projection.ProjectionDigestTx(ctx, tx, engine.generation)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := projection.CanonicalDigestTx(ctx, tx, manifest)
	if err != nil || actual != expected {
		t.Fatal("recovered projection differs from the canonical producer", actual, expected, err)
	}
}

func TestRepair_41_RejectInconsistentActiveRecovery(t *testing.T) {
	ctx, pool, archive, engine, manifest, digest := repair41Fixture(t)
	repair41Rebuild(t, ctx, engine, digest)
	other := *archive
	other.deployment += "-other"
	_, otherDigest, err := other.Capture(ctx, manifest.FromHeight, manifest.ToHeight)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := other.objects.Delete(context.Background(), other.key("manifests", otherDigest)); err != nil {
			t.Error(err)
		}
	})
	alternate, err := NewReplayEngine(&other, engine.consumer, engine.generation, engine.owner, repair41Projection{&other}, verifyWebhookFixtureAdmin, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	before := repair41Snapshot(t, ctx, archive)
	if err = alternate.Start(ctx, otherDigest); !errors.Is(err, ErrIntegrity) {
		t.Fatal("active generation accepted a different valid manifest", err)
	}
	if after := repair41Snapshot(t, ctx, archive); after != before {
		t.Fatal("refused manifest recovery changed persisted state")
	}
	for _, test := range []struct {
		name, corrupt, restore string
	}{
		{"status", "UPDATE %s.delivery_rebuilds SET status='verified'", "UPDATE %s.delivery_rebuilds SET status='active'"},
		{"next_height", "UPDATE %s.delivery_rebuilds SET next_height=next_height-1", "UPDATE %s.delivery_rebuilds SET next_height=next_height+1"},
		{"raw_count", "UPDATE %s.delivery_rebuilds SET raw_count=raw_count+1", "UPDATE %s.delivery_rebuilds SET raw_count=raw_count-1"},
		{"missing_alias", "DELETE FROM %s.delivery_read_aliases", "INSERT INTO %s.delivery_read_aliases(consumer_id,generation,revision) VALUES('repair_41','generation_41',1)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, strings.ReplaceAll(test.corrupt, "%s", archive.store.schema)); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, strings.ReplaceAll(test.restore, "%s", archive.store.schema)); err != nil {
					t.Error(err)
				}
			}()
			before := repair41Snapshot(t, ctx, archive)
			if err := engine.Start(ctx, digest); !errors.Is(err, ErrIntegrity) {
				t.Fatal("inconsistent active recovery accepted", err)
			}
			if after := repair41Snapshot(t, ctx, archive); after != before {
				t.Fatal("refused active recovery changed persisted state")
			}
		})
	}
	if err := engine.Start(ctx, digest); err != nil {
		t.Fatal("matching active recovery refused after restoring identity", err)
	}
}

package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func orderedPostgres(t *testing.T) (context.Context, *pgxpool.Pool, *PartitionStore, string) {
	t.Helper()
	requireDeliveryEnv(t, "DELIVERY_TEST_DSN", "DELIVERY_TEST_SCHEMA")
	dsn, base := os.Getenv("DELIVERY_TEST_DSN"), os.Getenv("DELIVERY_TEST_SCHEMA")
	schema := base + "_ordered"
	if dsn == "" || !strings.HasPrefix(base, "kd_") || !token.MatchString(schema) || len(schema) > 63 {
		t.Fatal("explicit private PostgreSQL DSN and unique bounded kd_ namespace required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 25432 || cfg.ConnConfig.Database != "kindle_fixture" {
		t.Fatal("refusing non-fixture database")
	}
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := pgx.Identifier{schema}.Sanitize()
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+q); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := pool.Exec(cleanup, "DROP SCHEMA "+q+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	for _, ddl := range []func(string) (string, error){SchemaDDL, PartitionSchemaDDL} {
		sql, err := ddl(schema)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, "CREATE TABLE "+q+".source_checkpoints(digest text PRIMARY KEY); CREATE TABLE "+q+".ordered_effects(id text PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO "+q+".source_checkpoints(digest) VALUES($1)", Hash([]byte("genuine-private-checkpoint-row"))); err != nil {
		t.Fatal(err)
	}
	partitions, err := NewPartitionStore(pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, pool, partitions, q
}

func TestQuarantinePoisonWithoutAffectedPoolsPostgres(t *testing.T) {
	ctx, pool, _, q := orderedPostgres(t)
	schema := os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"
	store, err := NewStore(pool, schema, fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	envelope := fixtureEnvelope("empty-pools-poison")
	if err = store.Produce(ctx, []Pending{{fixtureSubject, envelope}}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO "+q+".ordered_effects(id) VALUES($1)", envelope.EventID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var produced []byte
	if err = pool.QueryRow(ctx, "SELECT envelope FROM "+q+".delivery_outbox WHERE event_id=$1", envelope.EventID).Scan(&produced); err != nil {
		t.Fatal(err)
	}
	envelope, err = Decode(produced, fixturePolicy(), fixtureSubject)
	if err != nil {
		t.Fatal(err)
	}
	envelope.SchemaVersion = 2
	poison, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(poison, fixturePolicy(), fixtureSubject)
	if !errors.Is(err, ErrSchema) {
		t.Fatal("unknown-version poison was not refused", err)
	}
	in := QuarantineInput{ConsumerID: "empty-pools", Generation: "g1", Subject: fixtureSubject, PublicationID: decoded.EventID, Wire: poison, WireSHA256: accounting.Digest(Hash(poison)), Class: InnerSchema, EvidenceDigest: accounting.Digest(Hash(poison))}
	for _, pools := range [][]string{nil, {}} {
		in.AffectedPools = pools
		if err = store.QuarantineAfterRollback(ctx, in, nil); err != nil {
			t.Fatal("poison without affected pools did not commit", err)
		}
		var subject, publication, evidence string
		var recorded []byte
		var recordedPools []string
		if err = pool.QueryRow(ctx, "SELECT subject,publication_id,envelope,affected_pools,evidence_digest FROM "+q+".delivery_ordered_quarantine WHERE consumer_id=$1 AND projection_generation=$2 AND envelope_hash=$3 AND failure_class=$4", in.ConsumerID, in.Generation, string(in.WireSHA256), string(in.Class)).Scan(&subject, &publication, &recorded, &recordedPools, &evidence); err != nil {
			t.Fatal("committed poison quarantine missing", err)
		}
		if subject != in.Subject || publication != in.PublicationID || !bytes.Equal(recorded, poison) || recordedPools == nil || len(recordedPools) != 0 || evidence != string(in.EvidenceDigest) {
			t.Fatal("committed poison quarantine changed its evidence")
		}
		store, err = NewStore(pool, schema, fixturePolicy())
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+q+".delivery_ordered_quarantine WHERE consumer_id=$1 AND projection_generation=$2 AND envelope_hash=$3 AND failure_class=$4", in.ConsumerID, in.Generation, string(in.WireSHA256), string(in.Class)).Scan(&count); err != nil || count != 1 {
		t.Fatal("restart duplicated the poison quarantine", count, err)
	}
}

func orderedAppendInput(id, height string) AppendInput {
	return AppendInput{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: "125:0x0000000000000000000000000000000000000001", SourceEventID: Hash([]byte("source-" + id)), EffectIdentity: Hash([]byte("effect-" + id)), PayloadDigest: accounting.Digest(Hash([]byte("body-" + id))), SourceOrder: accounting.SourcePosition{ChainID: "125", Height: height, TransactionIndex: "0", LogIndex: "1", TransactionHash: "0x" + strings.Repeat("1", 64), BlockHash: "0x" + strings.Repeat("2", 64)}}
}

func orderedAppend(t *testing.T, ctx context.Context, pool *pgxpool.Pool, partitions *PartitionStore, in AppendInput) accounting.PartitionPosition {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackBounded(tx)
	position, err := partitions.AppendTx(ctx, tx, in)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return position
}

func orderedBootstrap(t *testing.T, ctx context.Context, pool *pgxpool.Pool, partitions *PartitionStore, q string, key ConsumerKey, ttl time.Duration) PartitionLease {
	t.Helper()
	lease, err := partitions.Acquire(ctx, key, "ordered-owner", ttl)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackBounded(tx)
	input := BootstrapInput{Lease: lease, CheckpointDigest: accounting.Digest(Hash([]byte("genuine-private-checkpoint-row"))), NextSequence: "1", PreviousEffectIdentity: ""}
	err = partitions.BootstrapTx(ctx, tx, input, func(ctx context.Context, tx pgx.Tx, in BootstrapInput) error {
		var stored string
		if err := tx.QueryRow(ctx, "SELECT digest FROM "+q+".source_checkpoints WHERE digest=$1", string(in.CheckpointDigest)).Scan(&stored); err != nil {
			return err
		}
		if stored != string(in.CheckpointDigest) {
			return ErrIntegrity
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return lease
}

func TestOrderedPostgres(t *testing.T) {
	ctx, pool, partitions, q := orderedPostgres(t)
	firstInput := orderedAppendInput("one", "1")
	first := orderedAppend(t, ctx, pool, partitions, firstInput)
	again := orderedAppend(t, ctx, pool, partitions, firstInput)
	if first.Sequence != "1" || again != first || first.PreviousEffectIdentity != "" {
		t.Fatal("producer identity not immutable or sequence not contiguous", first, again)
	}
	second := orderedAppend(t, ctx, pool, partitions, orderedAppendInput("two", "2"))
	if second.Sequence != "2" || second.PreviousEffectIdentity != first.EffectIdentity {
		t.Fatal("producer predecessor mismatch", second)
	}
	t.Run("ProducerDigestConflictAndSourceRegression", func(t *testing.T) {
		for _, in := range []AppendInput{func() AppendInput {
			v := firstInput
			v.PayloadDigest = accounting.Digest(Hash([]byte("changed-body")))
			return v
		}(), orderedAppendInput("late", "0")} {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = partitions.AppendTx(ctx, tx, in); err == nil {
				rollbackBounded(tx)
				t.Fatal("conflicting or regressing producer effect accepted")
			}
			rollbackBounded(tx)
		}
	})
	t.Run("IndependentConsumerAndGenerationOffsets", func(t *testing.T) {
		for _, identity := range [][2]string{{"charts", "g1"}, {"stats", "g1"}, {"charts", "g2"}} {
			key := ConsumerKey{ConsumerID: identity[0], ProjectionGeneration: identity[1], PartitionKind: first.PartitionKind, PartitionKey: first.PartitionKey}
			lease := orderedBootstrap(t, ctx, pool, partitions, q, key, time.Minute)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			g, err := partitions.CheckAndLockTx(ctx, tx, lease, second)
			if !errors.Is(err, ErrGap) {
				rollbackBounded(tx)
				t.Fatal("missing predecessor accepted", g, err)
			}
			rollbackBounded(tx)
			tx, err = pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			g, err = partitions.CheckAndLockTx(ctx, tx, lease, first)
			if err != nil || g.DuplicateCommitted {
				rollbackBounded(tx)
				t.Fatal("independent consumer was advanced", g, err)
			}
			if err = partitions.AdvanceTx(ctx, tx, g); err != nil {
				rollbackBounded(tx)
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			tx, err = pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			duplicate, err := partitions.CheckAndLockTx(ctx, tx, lease, first)
			if err != nil || !duplicate.DuplicateCommitted {
				rollbackBounded(tx)
				t.Fatal("committed position not recognized", duplicate, err)
			}
			if err = partitions.AdvanceTx(ctx, tx, duplicate); err != nil {
				rollbackBounded(tx)
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("MissingBootstrapNeverGuessesFrontier", func(t *testing.T) {
		key := ConsumerKey{ConsumerID: "unbootstrapped", ProjectionGeneration: "g1", PartitionKind: first.PartitionKind, PartitionKey: first.PartitionKey}
		lease, err := partitions.Acquire(ctx, key, "ordered-owner", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer rollbackBounded(tx)
		if _, err = partitions.CheckAndLockTx(ctx, tx, lease, first); err == nil {
			t.Fatal("missing authoritative bootstrap accepted")
		}
	})
	t.Run("ExpiryAtFinalFenceRollsBackAtomicWrites", func(t *testing.T) {
		key := ConsumerKey{ConsumerID: "expires", ProjectionGeneration: "g1", PartitionKind: first.PartitionKind, PartitionKey: first.PartitionKey}
		lease := orderedBootstrap(t, ctx, pool, partitions, q, key, time.Second)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		g, err := partitions.CheckAndLockTx(ctx, tx, lease, first)
		if err != nil {
			rollbackBounded(tx)
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO "+q+".ordered_effects VALUES('must-rollback')"); err != nil {
			rollbackBounded(tx)
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO "+q+".delivery_inbox(consumer_id,projection_generation,event_id,envelope_hash) VALUES('expires','g1','must-rollback',$1)", Hash([]byte("wire"))); err != nil {
			rollbackBounded(tx)
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "SELECT pg_sleep(1.1)"); err != nil {
			rollbackBounded(tx)
			t.Fatal(err)
		}
		if err = partitions.AdvanceTx(ctx, tx, g); !errors.Is(err, ErrLease) {
			rollbackBounded(tx)
			t.Fatal("expired final fence accepted", err)
		}
		rollbackBounded(tx)
		var effects, inbox int
		if err = pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM "+q+".ordered_effects),(SELECT count(*) FROM "+q+".delivery_inbox WHERE consumer_id='expires')").Scan(&effects, &inbox); err != nil {
			t.Fatal(err)
		}
		if effects != 0 || inbox != 0 {
			t.Fatal("expired effect partially committed", effects, inbox)
		}
		replacement, err := partitions.Acquire(ctx, key, "replacement-owner", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if replacement.Epoch <= lease.Epoch {
			t.Fatal("owner replacement did not fence old epoch")
		}
		tx, err = pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer rollbackBounded(tx)
		if _, err = partitions.CheckAndLockTx(ctx, tx, lease, first); !errors.Is(err, ErrLease) {
			t.Fatal("old epoch accepted", err)
		}
		current, err := partitions.CheckAndLockTx(ctx, tx, replacement, first)
		if err != nil || current.DuplicateCommitted {
			t.Fatal("failed effect advanced durable frontier", current, err)
		}
	})
}

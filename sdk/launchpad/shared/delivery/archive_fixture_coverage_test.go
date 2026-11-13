package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestArchiveCapturesCommittedSourceWithActualObjectStore(t *testing.T) {
	endpoint := os.Getenv("SHARED_TEST_MINIO_ENDPOINT")
	access := os.Getenv("SHARED_TEST_MINIO_ACCESS_KEY_ID")
	secret := os.Getenv("SHARED_TEST_MINIO_SECRET_ACCESS_KEY")
	if endpoint == "" || access == "" || secret == "" {
		t.Fatal("private object storage fixture is required")
	}
	ctx, pool, _, _ := orderedPostgres(t)
	schema := os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"
	ddl, err := ReplayDDL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	config, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(access, secret, "")))
	if err != nil {
		t.Fatal(err)
	}
	raw := s3.NewFromConfig(config, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	bucket := "kd-archive-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := raw.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	objects, err := storage.New(ctx, storage.Config{Endpoint: endpoint, AccessKeyID: access, SecretAccessKey: secret, Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	const height = int64(140)
	stream := "archive_fixture"
	blockHash := "0x" + strings.Repeat("a", 64)
	transactionHash := "0x" + strings.Repeat("b", 64)
	if _, err := pool.Exec(ctx, `CREATE SCHEMA indexer;
CREATE TABLE indexer.indexer_block_completeness_v1(chain_id integer,stream_id text,height bigint,block_hash text,raw_count bigint,fetch_complete boolean,required_decode_complete boolean);
CREATE TABLE indexer.indexer_raw_logs_v1(chain_id integer,height bigint,transaction_hash text,block_hash text,emitter text,transaction_index integer,log_index integer,topics jsonb,data bytea);
CREATE TABLE indexer.indexer_ingest_watermarks_v1(chain_id integer,stream_id text,typed_complete_height bigint,PRIMARY KEY(chain_id,stream_id))`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, "DROP SCHEMA indexer CASCADE"); err != nil {
			t.Error(err)
		}
	})
	if _, err := pool.Exec(ctx, "INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,$1,$2,$3,1,true,true)", stream, height, blockHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO indexer.indexer_ingest_watermarks_v1 VALUES(125,$1,$2)", stream, height); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO indexer.indexer_raw_logs_v1 VALUES(125,$1,$2,$3,$4,0,0,'[]',$5)", height, transactionHash, blockHash, "0x0000000000000000000000000000000000000001", []byte{1}); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(pool, schema, fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	archive, err := NewArchive(store, objects, schema, stream, "deployment", strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	manifest, digest, err := archive.Capture(ctx, height, height)
	if err != nil || manifest.RawCount != 1 || len(manifest.Blocks) != 1 || manifest.Blocks[0].BlockHash != blockHash || digest == "" {
		t.Fatalf("captured committed source = %+v, %s, %v", manifest, digest, err)
	}
	loaded, err := archive.Load(ctx, digest)
	if err != nil || loaded.RawCount != 1 || loaded.Blocks[0].SegmentHash != manifest.Blocks[0].SegmentHash {
		t.Fatalf("loaded source manifest = %+v, %v", loaded, err)
	}
	block, records, err := archive.ReadBlock(ctx, digest, height)
	if err != nil || block.RawCount != 1 || len(records) != 1 || records[0].TransactionHash != transactionHash || len(records[0].Data) != 1 || records[0].Data[0] != 1 {
		t.Fatalf("read archived block = %+v, %+v, %v", block, records, err)
	}
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := archive.ValidateSourceTx(ctx, transaction, manifest); err != nil {
		t.Fatalf("committed source identity = %v", err)
	}
	if err := transaction.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "CREATE TABLE "+archive.store.table("repair_41_projection")+"(generation text NOT NULL,height bigint NOT NULL,transaction_index bigint NOT NULL,log_index bigint NOT NULL,body bytea NOT NULL,PRIMARY KEY(generation,height,transaction_index,log_index))"); err != nil {
		t.Fatal(err)
	}
	replay, err := NewReplayEngine(archive, "archive_reader", "generation_1", "fixture_worker", repair41Projection{archive}, verifyWebhookFixtureAdmin, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := replay.Start(ctx, digest); err != nil {
		t.Fatalf("start committed archive replay: %v", err)
	}
	assertReplayCatalogBoundaries(t, ctx, replay, manifest, digest)
	if err := replay.RunBlockWithBudget(ctx, nil); err == nil {
		t.Fatal("archive replay admitted an absent work budget")
	}
	budget, err := NewWorkBudget(1, 1000, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := replay.RunBlockWithBudget(ctx, budget); err != nil {
		t.Fatalf("apply committed archive block: %v", err)
	}
	if err := replay.RunBlockWithBudget(ctx, budget); err != nil {
		t.Fatalf("verify committed archive projection: %v", err)
	}
	var status string
	var next, rawCount, projected int64
	if err := pool.QueryRow(ctx, "SELECT status,next_height,raw_count,(SELECT count(*) FROM "+archive.store.table("repair_41_projection")+") FROM "+archive.store.table("delivery_rebuilds")+" WHERE consumer_id='archive_reader' AND generation='generation_1'").Scan(&status, &next, &rawCount, &projected); err != nil || status != "verified" || next != height+1 || rawCount != 1 || projected != 1 {
		t.Fatalf("replayed archive state: status=%s next=%d raw=%d projected=%d err=%v", status, next, rawCount, projected, err)
	}
	state, err := replay.Status(ctx, 0)
	if err != nil || !state.Ready || state.Gap || state.Status != "verified" || state.SourceHeight != height {
		t.Fatalf("verified archive readiness: %+v, %v", state, err)
	}
	for _, bounds := range [][2]int64{{-1, height}, {height, height - 1}, {0, 10000}} {
		if _, _, err := archive.Capture(ctx, bounds[0], bounds[1]); !errors.Is(err, ErrSchema) {
			t.Fatalf("archive range: %v", err)
		}
	}
	if _, _, err := archive.Capture(ctx, height+1, height+1); !errors.Is(err, ErrGap) {
		t.Fatalf("incomplete archive range: %v", err)
	}
	if _, sameDigest, err := archive.Capture(ctx, height, height); err != nil || sameDigest != digest {
		t.Fatalf("archive identity on recapture: %s %v", sameDigest, err)
	}
	if _, err := archive.read(ctx, "segments", "invalid", 100); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("archive digest admission: %v", err)
	}
	if _, err := archive.read(ctx, "segments", manifest.Blocks[0].SegmentHash, 1); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("archive byte budget: %v", err)
	}
	for _, edit := range []struct {
		sql  string
		want error
	}{
		{"UPDATE indexer.indexer_raw_logs_v1 SET topics='{}'", ErrIntegrity},
		{"UPDATE indexer.indexer_raw_logs_v1 SET block_hash='invalid'", ErrIntegrity},
		{"UPDATE indexer.indexer_raw_logs_v1 SET transaction_hash='invalid'", ErrIntegrity},
		{"INSERT INTO indexer.indexer_raw_logs_v1 SELECT * FROM indexer.indexer_raw_logs_v1", ErrIntegrity},
		{"DELETE FROM indexer.indexer_raw_logs_v1", ErrGap},
	} {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, edit.sql); err != nil {
			t.Fatal(err)
		}
		if _, _, err := archive.sourceBlock(ctx, tx, height); !errors.Is(err, edit.want) {
			t.Fatalf("archive source admission: %v", err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		wire      []byte
		count     int64
		blockHash string
		want      error
	}{
		{[]byte("{\n"), 1, blockHash, ErrSchema},
		{[]byte("{}\n"), 1, blockHash, ErrIntegrity},
		{nil, 1, blockHash, ErrGap},
	} {
		segment, err := archive.put(ctx, "segments", test.wire)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := archive.records(ctx, ArchiveBlock{Height: height, BlockHash: test.blockHash, RawCount: test.count, SegmentHash: segment, SegmentBytes: int64(len(test.wire))}); !errors.Is(err, test.want) {
			t.Fatalf("archive segment admission: %v", err)
		}
		if _, err := archive.records(ctx, ArchiveBlock{SegmentHash: segment, SegmentBytes: int64(len(test.wire)) + 1}); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("archive segment size: %v", err)
		}
		if err := objects.Delete(ctx, archive.key("segments", segment)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.ValidateSourceTx(ctx, nil, ArchiveManifest{}); err == nil {
		t.Fatal("unbound source manifest admitted")
	}
	if _, err := replay.Status(ctx, -1); !errors.Is(err, ErrSchema) {
		t.Fatalf("negative replay lag: %v", err)
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	admin := context.WithValue(ctx, webhookAdminContextKey{}, webhookAdminCredential{stamp, signWebhookBody(strings.Repeat("admin-secret", 4), stamp, "replay.promote")})
	if err := replay.Promote(admin, 0); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("uncompared archive promoted: %v", err)
	}

	for _, record := range records {
		body, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		envelope := Envelope{EventID: Hash(body), EventType: "Swap", SchemaVersion: 1, ChainID: 125, SourceOrder: SourceOrder{strconv.FormatInt(record.Height, 10), strconv.FormatUint(uint64(record.TransactionIndex), 10), strconv.FormatUint(uint64(record.LogIndex), 10)}, EmittedAt: time.Unix(1, 0).UTC(), TraceID: Hash(body), PayloadHash: Hash(body), Payload: body}
		if err := store.Produce(ctx, []Pending{{fixtureSubject, envelope}}, func(ctx context.Context, tx pgx.Tx) error {
			for _, path := range []string{valueLegacy, valueDurable} {
				if err := store.RecordShadowTx(ctx, tx, digest, path, fixtureSubject, envelope); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if diff, err := store.CompareShadowGeneration(ctx, digest, []string{fixtureSubject}, replay.consumer, replay.generation); err != nil || diff.Digest == "" {
		t.Fatalf("canonical replay comparison: %+v %v", diff, err)
	}

	for _, mode := range []string{"manifest", "observation catalog", "observation fields", "source order", "missing durable", "outbox catalog", "outbox fields", "outbox identity"} {
		t.Run("shadow/"+mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackBounded(tx)
			var sql string
			switch mode {
			case "manifest":
				sql = "UPDATE " + store.table("delivery_archive_manifests") + " SET manifest='{}'"
			case "observation catalog":
				sql = "ALTER TABLE " + store.table("delivery_shadow_observations") + " RENAME COLUMN event_id TO unavailable_identity"
			case "observation fields":
				sql = "ALTER TABLE " + store.table("delivery_shadow_observations") + " ALTER COLUMN payload_hash DROP NOT NULL; UPDATE " + store.table("delivery_shadow_observations") + " SET payload_hash=NULL"
			case "source order":
				sql = "UPDATE " + store.table("delivery_shadow_observations") + " SET source_order='{}'"
			case "missing durable":
				sql = "DELETE FROM " + store.table("delivery_shadow_observations") + " WHERE path='durable'"
			case "outbox catalog":
				sql = "ALTER TABLE " + store.table("delivery_outbox") + " RENAME COLUMN payload_hash TO unavailable_digest"
			case "outbox fields":
				sql = "ALTER TABLE " + store.table("delivery_outbox") + " ALTER COLUMN payload_hash DROP NOT NULL; INSERT INTO " + store.table("delivery_outbox") + "(event_id,schema_version,subject,payload_hash,envelope) SELECT event_id||'_other',schema_version,subject,NULL,envelope FROM " + store.table("delivery_outbox")
			case "outbox identity":
				sql = "INSERT INTO " + store.table("delivery_outbox") + "(event_id,schema_version,subject,payload_hash,envelope) SELECT event_id||'_other',schema_version,subject,'" + strings.Repeat("f", 64) + "',envelope FROM " + store.table("delivery_outbox")
			}
			if _, err := tx.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.compareShadowTx(ctx, tx, digest, []string{fixtureSubject}); err == nil {
				t.Fatal("inconsistent shadow catalog admitted")
			}
		})
	}
	if _, err := store.CompareShadowRange(ctx, digest, nil); !errors.Is(err, ErrSchema) {
		t.Fatalf("missing shadow subjects: %v", err)
	}
	if _, err := store.CompareShadowRange(ctx, digest, []string{"unknown.subject"}); !errors.Is(err, ErrSchema) {
		t.Fatalf("unapproved shadow subject: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordShadowTx(ctx, tx, digest, valueDurable, fixtureSubject, Envelope{}); err == nil {
		t.Fatal("unbound shadow envelope admitted")
	}
	if err := store.BindGenerationPathTx(ctx, tx, "consumer", "g1", "unbound"); !errors.Is(err, ErrSchema) {
		t.Fatalf("unbound generation path: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := replay.Promote(admin, 0); err != nil {
		t.Fatal(err)
	}
	if err := replay.Rebuild(ctx, digest, budget, true, 1); err != nil {
		t.Fatalf("active generation recovery: %v", err)
	}
	state, err = replay.Status(ctx, 0)
	if err != nil || !state.Ready || state.Status != valueActive {
		t.Fatalf("active replay readiness: %+v %v", state, err)
	}
	nextReplay := *replay
	nextReplay.generation = "generation_2"
	if err := nextReplay.Rebuild(ctx, digest, budget, false, 1); err != nil {
		t.Fatalf("next generation rebuild: %v", err)
	}
	if diff, err := store.CompareShadowGeneration(ctx, digest, []string{fixtureSubject}, nextReplay.consumer, nextReplay.generation); err != nil || diff.Digest == "" {
		t.Fatalf("next generation comparison: %+v %v", diff, err)
	}
	if err := nextReplay.Promote(admin, 1); err != nil {
		t.Fatal(err)
	}
	var generation, prior string
	var revision int64
	if err := pool.QueryRow(ctx, "SELECT generation,prior_generation,revision FROM "+store.table("delivery_read_aliases")+" WHERE consumer_id=$1", replay.consumer).Scan(&generation, &prior, &revision); err != nil || generation != nextReplay.generation || prior != replay.generation || revision != 2 {
		t.Fatalf("generation cutover: %s %s %d %v", generation, prior, revision, err)
	}
	if err := nextReplay.Promote(admin, 2); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("duplicate cutover: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := nextReplay.RunBlockWithBudget(cancelled, budget); err == nil {
		t.Fatal("cancelled rebuild admitted")
	}
	if _, _, err := archive.ReadBlock(ctx, digest, height+1); !errors.Is(err, ErrGap) {
		t.Fatalf("uncommitted archive height = %v", err)
	}
	if _, err := archive.Load(ctx, strings.Repeat("0", 64)); err == nil {
		t.Fatal("missing object accepted")
	}

	for _, test := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"syntax", []byte("{"), ErrSchema},
		{"unknown field", []byte(`{"unknown":true}`), ErrSchema},
		{"shape", []byte(`{}`), ErrIntegrity},
	} {
		objectDigest, err := archive.put(ctx, "manifests", test.raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Load(ctx, objectDigest); !errors.Is(err, test.want) {
			t.Fatalf("manifest %s admission: %v", test.name, err)
		}
		if err := objects.Delete(ctx, archive.key("manifests", objectDigest)); err != nil {
			t.Fatal(err)
		}
	}
	foreign := *archive
	foreign.deployment = "another_deployment"
	if _, err := foreign.Load(ctx, digest); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("foreign manifest provenance: %v", err)
	}
	if _, err := archive.put(cancelled, "segments", []byte("cancelled")); err == nil {
		t.Fatal("cancelled archive write admitted")
	}
	if _, _, err := archive.Capture(cancelled, height, height); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled archive capture: %v", err)
	}
	if _, _, err := archive.ReadBlock(cancelled, digest, height); err == nil {
		t.Fatal("cancelled archive read admitted")
	}
	for _, phase := range []int{1, 2} {
		calls := 0
		guardedStore, err := NewStoreWithEpochGuard(pool, schema, fixturePolicy(), func(context.Context, pgx.Tx) error {
			calls++
			if calls == phase {
				return ErrAuthorization
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		guarded := *archive
		guarded.store = guardedStore
		if _, _, err := guarded.Capture(ctx, height, height); !errors.Is(err, ErrAuthorization) || calls != phase {
			t.Fatalf("archive epoch fence: %d %v", calls, err)
		}
	}
	for _, tc := range []struct {
		name, sql, restore string
		want               error
	}{
		{"manifest identity", "UPDATE " + store.table("delivery_archive_manifests") + " SET manifest='{}'", "", ErrIntegrity},
		{"catalog column", "ALTER TABLE " + store.table("delivery_archive_manifests") + " RENAME COLUMN manifest TO unbound_manifest", "ALTER TABLE " + store.table("delivery_archive_manifests") + " RENAME COLUMN unbound_manifest TO manifest", nil},
	} {
		if _, err := pool.Exec(ctx, tc.sql); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Load(ctx, digest); err == nil || tc.want != nil && !errors.Is(err, tc.want) {
			t.Fatalf("archive %s admission: %v", tc.name, err)
		}
		if tc.restore != "" {
			if _, err := pool.Exec(ctx, tc.restore); err != nil {
				t.Fatal(err)
			}
		} else {
			body, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_archive_manifests")+" SET manifest=$1", body); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, mode := range []string{"frontier", "count", "projection", "source", "manifest", "catalog", "projection table"} {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		next, count, projection := height+1, manifest.RawCount, ""
		if err := tx.QueryRow(ctx, "SELECT projection_digest FROM "+store.table("delivery_rebuilds")+" WHERE consumer_id=$1 AND generation=$2", nextReplay.consumer, nextReplay.generation).Scan(&projection); err != nil {
			t.Fatal(err)
		}
		switch mode {
		case "frontier":
			next++
		case "count":
			count++
		case "projection":
			projection = strings.Repeat("0", 64)
		case "source":
			_, err = tx.Exec(ctx, "UPDATE indexer.indexer_raw_logs_v1 SET data='changed'")
		case "manifest":
			_, err = tx.Exec(ctx, "UPDATE "+store.table("delivery_archive_manifests")+" SET manifest='{}'")
		case "catalog":
			_, err = tx.Exec(ctx, "ALTER TABLE indexer.indexer_ingest_watermarks_v1 RENAME COLUMN typed_complete_height TO other_height")
		case "projection table":
			_, err = tx.Exec(ctx, "DROP TABLE "+store.table("repair_41_projection"))
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := nextReplay.validateRecoveryTx(ctx, tx, nextReplay.generation, digest, next, count, projection); err == nil {
			t.Fatalf("unbound recovery %s admitted", mode)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	third := *replay
	third.generation = "generation_3"
	if err := third.Start(ctx, digest); err != nil {
		t.Fatal(err)
	}
	if err := third.Start(ctx, digest); err != nil {
		t.Fatalf("matching replay restart: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_rebuilds")+" SET next_height=$1 WHERE generation='generation_3'", height-1); err != nil {
		t.Fatal(err)
	}
	if err := third.RunBlock(ctx); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("replay height binding: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_rebuilds")+" SET next_height=$1,owner='another_worker',lease_until=clock_timestamp()+interval '1 minute' WHERE generation='generation_3'", height); err != nil {
		t.Fatal(err)
	}
	if err := third.Start(ctx, digest); !errors.Is(err, ErrLease) {
		t.Fatalf("replay worker fence: %v", err)
	}
	if err := third.RunBlock(ctx); !errors.Is(err, ErrLease) {
		t.Fatalf("replay apply fence: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_rebuilds")+" SET owner=$1,next_height=$2,raw_count=0 WHERE generation='generation_3'", third.owner, height+1); err != nil {
		t.Fatal(err)
	}
	if err := third.RunBlock(ctx); !errors.Is(err, ErrGap) {
		t.Fatalf("replay count binding: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_rebuilds")+" SET raw_count=1 WHERE generation='generation_3'"); err != nil {
		t.Fatal(err)
	}
	if err := third.RunBlock(ctx); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("replay projection binding: %v", err)
	}
	if err := third.Promote(ctx, 2); err == nil {
		t.Fatal("unauthorized replay promotion admitted")
	}
	if err := third.Promote(admin, 2); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unverified replay promotion: %v", err)
	}
	if err := third.RollbackAlias(ctx, 2); err == nil {
		t.Fatal("unauthorized alias rollback admitted")
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_rebuilds")+" SET raw_count=2 WHERE generation='generation_2'"); err != nil {
		t.Fatal(err)
	}
	if err := nextReplay.Start(ctx, digest); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("active replay source binding: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_rebuilds")+" SET raw_count=1 WHERE generation='generation_2'"); err != nil {
		t.Fatal(err)
	}

	for _, mode := range []string{"completeness column", "raw column", "null emitter", "invalid topics", "changed block", "duplicate position", "raw count"} {
		t.Run("source/"+mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackBounded(tx)
			var sql string
			switch mode {
			case "completeness column":
				sql = "ALTER TABLE indexer.indexer_block_completeness_v1 RENAME COLUMN raw_count TO unbound_count"
			case "raw column":
				sql = "ALTER TABLE indexer.indexer_raw_logs_v1 RENAME COLUMN emitter TO unbound_emitter"
			case "null emitter":
				sql = "UPDATE indexer.indexer_raw_logs_v1 SET emitter=NULL"
			case "invalid topics":
				sql = "UPDATE indexer.indexer_raw_logs_v1 SET topics='{}'"
			case "changed block":
				sql = "UPDATE indexer.indexer_raw_logs_v1 SET block_hash='changed'"
			case "duplicate position":
				sql = "INSERT INTO indexer.indexer_raw_logs_v1 SELECT * FROM indexer.indexer_raw_logs_v1"
			case "raw count":
				sql = "UPDATE indexer.indexer_block_completeness_v1 SET raw_count=2"
			}
			if _, err := tx.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
			if _, _, err := archive.sourceBlock(ctx, tx, height); err == nil {
				t.Fatal("inconsistent canonical source admitted")
			}
		})
	}
	if err := objects.Delete(ctx, archive.key("segments", manifest.Blocks[0].SegmentHash)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := archive.ReadBlock(ctx, digest, height); err == nil {
		t.Fatal("missing source segment accepted")
	}
	if err := objects.Delete(ctx, archive.key("manifests", digest)); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
}

func assertReplayCatalogBoundaries(t *testing.T, ctx context.Context, replay *ReplayEngine, manifest ArchiveManifest, digest string) {
	t.Helper()
	store := replay.archive.store
	rebuilds := store.table("delivery_rebuilds")
	aliases := store.table("delivery_read_aliases")
	where := " WHERE consumer_id='archive_reader' AND generation='generation_1'"
	if _, err := store.pool.Exec(ctx, "INSERT INTO "+store.table("delivery_archive_manifests")+"(manifest_digest,source_stream,from_height,to_height,raw_count,final_block_hash,manifest) SELECT $1,source_stream,from_height,to_height,raw_count,final_block_hash,manifest FROM "+store.table("delivery_archive_manifests")+" WHERE manifest_digest=$2", strings.Repeat("0", 64), digest); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, change, restore string
		run                   func() error
	}{
		{"alias catalog", "ALTER TABLE " + aliases + " RENAME COLUMN generation TO unbound_generation", "ALTER TABLE " + aliases + " RENAME COLUMN unbound_generation TO generation", func() error { return replay.Start(ctx, digest) }},
		{"rebuild catalog", "ALTER TABLE " + rebuilds + " RENAME COLUMN next_height TO unbound_height", "ALTER TABLE " + rebuilds + " RENAME COLUMN unbound_height TO next_height", func() error { return replay.Start(ctx, digest) }},
		{"changed manifest", "UPDATE " + rebuilds + " SET manifest_digest='" + strings.Repeat("0", 64) + "'" + where, "UPDATE " + rebuilds + " SET manifest_digest='" + digest + "'" + where, func() error { return replay.Start(ctx, digest) }},
		{"leased owner", "UPDATE " + rebuilds + " SET owner='other_worker'" + where, "UPDATE " + rebuilds + " SET owner='fixture_worker'" + where, func() error { return replay.Start(ctx, digest) }},
		{"inactive work", "UPDATE " + rebuilds + " SET status='verified'" + where, "UPDATE " + rebuilds + " SET status='building'" + where, func() error { return replay.RunBlock(ctx) }},
		{"height before source", "UPDATE " + rebuilds + " SET next_height=139" + where, "UPDATE " + rebuilds + " SET next_height=140" + where, func() error { return replay.RunBlock(ctx) }},
		{"missing manifest", "UPDATE " + rebuilds + " SET manifest_digest='" + strings.Repeat("0", 64) + "'" + where, "UPDATE " + rebuilds + " SET manifest_digest='" + digest + "'" + where, func() error { return replay.RunBlock(ctx) }},
		{"source catalog", "ALTER TABLE indexer.indexer_raw_logs_v1 RENAME COLUMN emitter TO unavailable_emitter", "ALTER TABLE indexer.indexer_raw_logs_v1 RENAME COLUMN unavailable_emitter TO emitter", func() error { return replay.RunBlock(ctx) }},
		{"changed source", "UPDATE indexer.indexer_raw_logs_v1 SET data=decode('02','hex')", "UPDATE indexer.indexer_raw_logs_v1 SET data=decode('01','hex')", func() error { return replay.RunBlock(ctx) }},
		{"projection catalog", "ALTER TABLE " + store.table("repair_41_projection") + " RENAME COLUMN body TO unavailable_body", "ALTER TABLE " + store.table("repair_41_projection") + " RENAME COLUMN unavailable_body TO body", func() error { return replay.RunBlock(ctx) }},
		{"verification gap", "UPDATE " + rebuilds + " SET raw_count=2" + where, "UPDATE " + rebuilds + " SET raw_count=0" + where, func() error { return replay.verify(ctx, manifest, digest) }},
	} {
		t.Run("replay/"+tc.name, func(t *testing.T) {
			if _, err := store.pool.Exec(ctx, tc.change); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := store.pool.Exec(ctx, tc.restore); err != nil {
					t.Error(err)
				}
			}()
			if err := tc.run(); err == nil {
				t.Fatal("replay without complete source and catalog authority admitted")
			}
		})
	}
	var height, count, projected int64
	if err := store.pool.QueryRow(ctx, "SELECT next_height,raw_count,(SELECT count(*) FROM "+store.table("repair_41_projection")+") FROM "+rebuilds+where).Scan(&height, &count, &projected); err != nil || height != manifest.FromHeight || count != 0 || projected != 0 {
		t.Fatalf("rejected replay preserved state: height=%d count=%d projected=%d error=%v", height, count, projected, err)
	}
}

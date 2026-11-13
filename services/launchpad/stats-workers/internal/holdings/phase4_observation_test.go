package holdings

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

func TestPhase4ObservationReadsCanonicalSourceAndBroker(t *testing.T) {
	ctx, pool, transfer := canonicalLocalFixture(t)
	ddl, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "migrations", "60001.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,'canonical',10,$1,'healthy','indexer')`, transfer.BlockHash); err != nil {
		t.Fatal(err)
	}
	const schema = "holdings_observation"
	create, err := SchemaSQL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, create); err != nil {
		t.Fatal(err)
	}
	repository, err := NewRepository(pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.DiscoverCanonical(ctx, transfer.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	if batch, err := repository.CatchUpCanonical(ctx, transfer.Token, "canonical", 10); err != nil || batch.Applied != 1 {
		t.Fatalf("projected source: %+v %v", batch, err)
	}
	brokerURL := os.Getenv("L20_NATS_URL")
	if brokerURL == "" {
		t.Fatal("L20_NATS_URL required")
	}
	nc := internaltest.ConnectNATS(t, brokerURL)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	stream := "PHASE4_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := js.AddStream(&nats.StreamConfig{Name: stream, Subjects: []string{stream + ".events"}, Storage: nats.FileStorage, MaxBytes: 1048576}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := js.DeleteStream(stream); err != nil {
			t.Error(err)
		}
	})
	if _, err := js.AddConsumer(stream, &nats.ConsumerConfig{Durable: "observation", AckPolicy: nats.AckExplicitPolicy, FilterSubject: stream + ".events"}); err != nil {
		t.Fatal(err)
	}
	plan := Phase4Plan{SourceStream: "canonical", HolderSchema: schema, BrokerStream: stream, BrokerConsumer: "observation", Pools: []Phase4Pool{{Pool: addr(83), Token: transfer.Token}}, MaxProjectionLagMS: 1000}
	artifacts := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	var commitTimestamps string
	if err := pool.QueryRow(ctx, "SHOW track_commit_timestamp").Scan(&commitTimestamps); err != nil {
		t.Fatal(err)
	}
	backlog, err := phase4Observe(ctx, pool, js, plan, artifacts, "normal", time.Now().Add(time.Hour))
	if commitTimestamps == "off" {
		if err == nil || !strings.Contains(err.Error(), "could not get commit timestamp data") {
			t.Fatalf("missing commit timestamps: %v", err)
		}
	} else if err != nil || backlog != 0 {
		t.Fatalf("source observation: backlog=%d err=%v", backlog, err)
	}
	if err := artifacts.close(); err != nil {
		t.Fatal(err)
	}
	names := []string{"database.ndjson"}
	if commitTimestamps == "on" {
		names = append(names, "broker.ndjson")
	}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(artifacts.directory, name))
		if commitTimestamps == "off" {
			if err == nil {
				t.Fatalf("%s written without commit timestamps", name)
			}
		} else if err != nil || len(raw) == 0 {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := pool.Exec(context.Background(), `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()-interval '1 second' WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	failed := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	if _, err := phase4Observe(ctx, pool, js, plan, failed, "normal", time.Now().Add(time.Hour)); err == nil {
		t.Fatal("expired canonical source accepted")
	}
	if err := failed.close(); err != nil {
		t.Fatal(err)
	}
}

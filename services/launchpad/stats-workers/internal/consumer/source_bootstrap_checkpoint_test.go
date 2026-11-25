package consumer

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func TestStatsCreationBootstrapRejectsLegacyCheckpoint(t *testing.T) {
	chain, log, timestamp := localAccountingChain(t)
	pool, deliveries, partitions, repo, authority := localAccountingDatabase(t, log)
	publication := localAccountingPublication(t, pool, log, timestamp)
	evidence, err := sourcebootstrap.LoadStats(t.Context(), pool, "indexer", "canonical", "projection-v1", publication.Fill.PoolAddress)
	if err != nil {
		t.Fatal(err)
	}
	legacy := accounting.Digest(accounting.HashTuple("stats-source-bootstrap-v1", evidence.SourceStream, evidence.Pool, evidence.ProjectionVersion, evidence.PublicationID, string(evidence.PayloadDigest), "0", evidence.FirstCommitID, strconv.FormatInt(evidence.FirstCommitLastHeight, 10), evidence.FirstCommitBlockHash))
	key := delivery.ConsumerKey{ConsumerID: "stats", ProjectionGeneration: "g1", PartitionKind: "market", PartitionKey: publication.Positions[0].PartitionKey}
	lease, err := partitions.Acquire(t.Context(), key, "stats-worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(t.Context()) }()
	if err = partitions.BootstrapTx(t.Context(), tx, delivery.BootstrapInput{Lease: lease, CheckpointDigest: legacy, NextSequence: "1"}, func(context.Context, pgx.Tx, delivery.BootstrapInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	consumer, err := NewSourceVerifiedAccountingConsumer(SourceVerifiedAccountingConfig{Pool: pool, Deliveries: deliveries, Partitions: partitions, Repository: repo, Authority: authority, ConsumerID: "stats", Generation: "g1", ProjectionVersion: "projection-v1", Owner: "stats-worker", SourceStream: "canonical", SourcePartitionSchema: "indexer", RPC: chain, MaxReplayBlocks: 16, MaxReplayFills: 16})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = consumer.lease(t.Context(), *publication.Fill); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatalf("legacy checkpoint admission: %v", err)
	}
	var checkpoint string
	if err = pool.QueryRow(t.Context(), `SELECT checkpoint_digest FROM stats_delivery.delivery_partition_offsets WHERE partition_key=$1`, key.PartitionKey).Scan(&checkpoint); err != nil || checkpoint != string(legacy) {
		t.Fatalf("legacy checkpoint conversion: %q %v", checkpoint, err)
	}
}

package delivery

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

func TestPartitionPublicationRejectsInconsistentStoredState(t *testing.T) {
	ctx, pool, partitions, _ := orderedPostgres(t)
	input := orderedAppendInput("partition_state", "2")
	position := orderedAppend(t, ctx, pool, partitions, input)
	for _, tc := range []struct {
		name, sql string
		want      error
	}{
		{"position encoding", "UPDATE " + partitions.table("delivery_partition_positions") + " SET position='invalid'", ErrIntegrity},
		{"position binding", "UPDATE " + partitions.table("delivery_partition_positions") + " SET position='{}'", ErrIntegrity},
		{"position unavailable", "DROP TABLE " + partitions.table("delivery_partition_positions"), nil},
		{"producer unavailable", "DROP TABLE " + partitions.table("delivery_partition_producers"), nil},
		{"sequence exhausted", "DELETE FROM " + partitions.table("delivery_partition_positions") + "; UPDATE " + partitions.table("delivery_partition_producers") + " SET last_sequence=9223372036854775807", nil},
		{"source encoding", "DELETE FROM " + partitions.table("delivery_partition_positions") + "; UPDATE " + partitions.table("delivery_partition_producers") + " SET last_source_order='invalid'", ErrIntegrity},
		{"source chain", "DELETE FROM " + partitions.table("delivery_partition_positions") + "; UPDATE " + partitions.table("delivery_partition_producers") + ` SET last_source_order='{"chainId":"1"}'`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackBounded(tx)
			if _, err := tx.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			if _, err := partitions.AppendTx(ctx, tx, input); err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("inconsistent publication state admitted: %v", err)
			}
		})
	}
	key := ConsumerKey{ConsumerID: "state_consumer", ProjectionGeneration: "g1", PartitionKind: position.PartitionKind, PartitionKey: position.PartitionKey}
	lease := orderedBootstrap(t, ctx, pool, partitions, partitions.schema, key, time.Minute)
	for _, mode := range []string{"invalid position", "foreign partition", "missing offset", "missing lease", "invalid lease", "sequence gap", "predecessor gap", "missing commit", "changed commit", "commit table", "offset table", "sequence exhausted", "duplicate flag", "bootstrap predecessor", "bootstrap verification", "bootstrap identity", "commit encoding"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackBounded(tx)
			p, l := position, lease
			var got error
			switch mode {
			case "invalid position":
				p.PayloadDigest = "invalid"
			case "foreign partition":
				l.Key.PartitionKey = "125:0x" + strings.Repeat("a", 40)
			case "missing offset":
				_, err = tx.Exec(ctx, "DELETE FROM "+partitions.table("delivery_partition_offsets"))
			case "missing lease":
				_, err = tx.Exec(ctx, "DELETE FROM "+partitions.table("delivery_partition_offsets")+";DELETE FROM "+partitions.table("delivery_partition_leases"))
			case "invalid lease":
				l.Epoch = 0
			case "sequence gap":
				p.Sequence = "2"
				p.PreviousEffectIdentity = strings.Repeat("a", 64)
			case "predecessor gap":
				_, err = tx.Exec(ctx, "UPDATE "+partitions.table("delivery_partition_offsets")+" SET previous_effect_identity=$1", strings.Repeat("a", 64))
			case "missing commit", "changed commit", "commit table":
				_, err = tx.Exec(ctx, "UPDATE "+partitions.table("delivery_partition_offsets")+" SET next_sequence=2")
				if err != nil {
					t.Fatal(err)
				}
				if mode == "changed commit" {
					_, err = tx.Exec(ctx, "INSERT INTO "+partitions.table("delivery_partition_commits")+"(consumer_id,projection_generation,partition_kind,partition_key,sequence,effect_identity,position) VALUES($1,$2,$3,$4,1,$5,'{}')", key.ConsumerID, key.ProjectionGeneration, key.PartitionKind, key.PartitionKey, p.EffectIdentity)
				}
				if mode == "commit table" {
					_, err = tx.Exec(ctx, "DROP TABLE "+partitions.table("delivery_partition_commits"))
				}
			case "offset table":
				_, err = tx.Exec(ctx, "ALTER TABLE "+partitions.table("delivery_partition_offsets")+" RENAME COLUMN next_sequence TO other_sequence")
			case "sequence exhausted":
				p.Sequence = "9223372036854775807"
				p.PreviousEffectIdentity = strings.Repeat("a", 64)
				_, err = tx.Exec(ctx, "UPDATE "+partitions.table("delivery_partition_offsets")+" SET next_sequence=$1,previous_effect_identity=$2", int64(math.MaxInt64), p.PreviousEffectIdentity)
			case "duplicate flag":
			case "bootstrap predecessor", "bootstrap verification", "bootstrap identity":
				in := BootstrapInput{Lease: l, CheckpointDigest: accounting.Digest(Hash([]byte("genuine-private-checkpoint-row"))), NextSequence: "1"}
				verify := func(context.Context, pgx.Tx, BootstrapInput) error { return nil }
				if mode == "bootstrap identity" {
					in.CheckpointDigest = accounting.Digest(strings.Repeat("b", 64))
				}
				if mode == "bootstrap predecessor" {
					in.PreviousEffectIdentity = strings.Repeat("a", 64)
				}
				if mode == "bootstrap verification" {
					verify = func(context.Context, pgx.Tx, BootstrapInput) error { return ErrGap }
				}
				got = partitions.BootstrapTx(ctx, tx, in, verify)
			case "commit encoding":
				got = partitions.verifyCommitTx(ctx, tx, GuardedPosition{Lease: l, Position: accounting.PartitionPosition{Sequence: "invalid"}})
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(mode, "bootstrap") && mode != "commit encoding" {
				if mode == "sequence exhausted" || mode == "duplicate flag" {
					got = partitions.AdvanceTx(ctx, tx, GuardedPosition{Lease: l, Position: p, DuplicateCommitted: mode == "duplicate flag"})
				} else {
					_, got = partitions.CheckAndLockTx(ctx, tx, l, p)
				}
			}
			if got == nil {
				t.Fatal("inconsistent consumer position admitted")
			}
		})
	}
	if _, err := partitions.AppendTx(ctx, nil, input); err == nil {
		t.Fatal("publication without transaction admitted")
	}
	if _, err := partitions.AppendTx(ctx, nil, AppendInput{}); err == nil {
		t.Fatal("empty publication admitted")
	}
}

package delivery

import (
	"errors"
	"testing"
	"time"
)

func TestPartitionLeaseRenewalPreservesFenceBeforeCommit(t *testing.T) {
	ctx, pool, partitions, q := orderedPostgres(t)
	position := orderedAppend(t, ctx, pool, partitions, orderedAppendInput("renewal", "1"))
	key := ConsumerKey{ConsumerID: "renewal_consumer", ProjectionGeneration: "g1", PartitionKind: position.PartitionKind, PartitionKey: position.PartitionKey}
	lease := orderedBootstrap(t, ctx, pool, partitions, q, key, time.Minute)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackBounded(tx)
	renewed, err := partitions.Acquire(ctx, key, lease.Owner, time.Minute)
	if err != nil || renewed.Epoch != lease.Epoch || !renewed.ExpiresAt.After(lease.ExpiresAt) {
		t.Fatal(renewed, lease, err)
	}
	g, err := partitions.CheckAndLockTx(ctx, tx, lease, position)
	if err != nil || g.DuplicateCommitted {
		t.Fatal(g, err)
	}
	if err = partitions.AdvanceTx(ctx, tx, g); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE "+partitions.table("delivery_partition_leases")+" SET expires_at=clock_timestamp()-interval '1 second' WHERE "+consumerPredicate, keyArgs(key)...); err != nil {
		t.Fatal(err)
	}
	replacement, err := partitions.Acquire(ctx, key, lease.Owner, time.Minute)
	if err != nil || replacement.Epoch != lease.Epoch+1 {
		t.Fatal(replacement, err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackBounded(tx)
	if _, err = partitions.CheckAndLockTx(ctx, tx, lease, position); !errors.Is(err, ErrLease) {
		t.Fatal(err)
	}
	duplicate, err := partitions.CheckAndLockTx(ctx, tx, replacement, position)
	if err != nil || !duplicate.DuplicateCommitted {
		t.Fatal(duplicate, err)
	}
}

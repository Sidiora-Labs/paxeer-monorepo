package projectionstats

import (
	"context"
	"errors"
	"testing"
)

func TestAccountingSnapshotReadsCommittedMarketPublication(t *testing.T) {
	fixture := newPublicFixture(t)
	publication, _ := repairM1043Receipt(t)
	input, err := qualifiedInput(*publication.Fill, "snapshot-consumer", publication.Positions[0].ProjectionVersion)
	if err != nil {
		t.Fatal(err)
	}
	start, err := Minute(input.BlockTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := fixture.pg.Begin(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	result, err := fixture.repo.ApplyQualifiedTx(fixture.ctx, tx, input)
	if err != nil || !result.Accepted {
		t.Fatalf("qualified trade: %+v %v", result, err)
	}
	if err := fixture.repo.RecordMarketPublicationTx(fixture.ctx, tx, publication, result); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(fixture.ctx); err != nil {
		t.Fatal(err)
	}
	snapshot, found, err := fixture.repo.AccountingSnapshot(fixture.ctx, publication.Fill.PoolAddress, publication.Positions[0].ProjectionVersion, start)
	if err != nil || !found || snapshot.VolumeUSDLRaw != publication.Fill.USDLAmountRaw || snapshot.PoolFillCount != 1 || snapshot.Close.SourceFillID != publication.SourceFillID || snapshot.PartitionPosition != publication.Positions[0] || len(snapshot.Fees) != 1 {
		t.Fatalf("market snapshot: found=%v %+v %v", found, snapshot, err)
	}
	missing, found, err := fixture.repo.AccountingSnapshot(fixture.ctx, publication.Fill.PoolAddress, publication.Positions[0].ProjectionVersion, start+60)
	if err != nil || found || missing.PoolAddress != "" {
		t.Fatalf("empty minute: found=%v %+v %v", found, missing, err)
	}
	if _, _, err := fixture.repo.AccountingSnapshot(fixture.ctx, "bad", publication.Positions[0].ProjectionVersion, start); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid pool: %v", err)
	}
	if _, _, err := fixture.repo.AccountingSnapshotTx(fixture.ctx, nil, publication.Fill.PoolAddress, publication.Positions[0].ProjectionVersion, start); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing transaction: %v", err)
	}
}

package projectionstats

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestQualifiedBatchUsesRecordedSourceReceipt(t *testing.T) {
	ctx, repo := repository(t)
	publication, _ := repairM1043Receipt(t)
	authority, err := NewAccountingSourceAuthority(strings.Trim(repo.schema, `"`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, "CREATE TABLE "+repo.schema+".indexer_accounting_fills_v1(source_fill_id text PRIMARY KEY,source_fact_digest text NOT NULL,payload jsonb NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	fillRaw, err := json.Marshal(publication.Fill)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, "INSERT INTO "+repo.schema+".indexer_accounting_fills_v1 VALUES($1,$2,$3)", publication.SourceFillID, string(publication.SourceFactDigest), fillRaw); err != nil {
		t.Fatal(err)
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	original, err := authority.OriginalTx(ctx, tx, publication)
	if err != nil || original.SourceFillID != publication.SourceFillID {
		t.Fatalf("original: %+v %v", original, err)
	}
	input, err := authority.ResolveTx(ctx, tx, publication, "qualified-batch-v1")
	if err != nil || input.Trade.PoolAddress != publication.Fill.PoolAddress || input.Policy.AuthorityEvidenceDigest != string(publication.SourceFactDigest) {
		t.Fatalf("qualified input: %+v %v", input, err)
	}
	results, err := repo.ApplyQualifiedBatchTx(ctx, tx, []QualifiedApplyInput{input})
	if err != nil || len(results) != 1 || !results[0].Accepted || results[0].Revision != 1 {
		t.Fatalf("batch: %+v %v", results, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, ctx, repo, "trade_facts") != 1 || count(t, ctx, repo, "minute_buckets") != 1 || count(t, ctx, repo, "fee_groups") != 1 {
		t.Fatal("qualified batch did not persist one trade")
	}
	replayTx, err := repo.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer replayTx.Rollback(context.Background())
	replay, err := repo.ApplyQualifiedBatchTx(ctx, replayTx, []QualifiedApplyInput{input})
	if err != nil || len(replay) != 1 || replay[0].Accepted || replay[0].Revision != 1 {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	if err := replayTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, "UPDATE "+repo.schema+".indexer_accounting_fills_v1 SET source_fact_digest=$1 WHERE source_fill_id=$2", strings.Repeat("0", 64), publication.SourceFillID); err != nil {
		t.Fatal(err)
	}
	badTx, err := repo.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer badTx.Rollback(context.Background())
	if _, err := authority.OriginalTx(ctx, badTx, publication); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed source: %v", err)
	}
	if _, err := repo.ApplyQualifiedBatchTx(ctx, badTx, []QualifiedApplyInput{{}}); err == nil {
		t.Fatal("empty qualified input accepted")
	}
	if _, err := authority.ResolveTx(ctx, badTx, accounting.PublicationV1{}, "qualified-batch-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty publication: %v", err)
	}
	if _, err := NewAccountingSourceAuthority("bad-schema!"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid schema: %v", err)
	}
	if _, err := authority.OriginalTx(ctx, nil, publication); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing transaction: %v", err)
	}
}

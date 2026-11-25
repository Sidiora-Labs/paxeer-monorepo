package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store/accountingtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestUntradedPoolPageClassification(t *testing.T) {
	f := accountingtest.New(t)
	request := store.AccountingPageRequest{Pool: accountingtest.Pool, Limit: 5, Interval: "1m", From: 1699999980000, To: 1700000040000}
	for _, read := range []struct {
		name string
		run  func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error)
	}{{"trades", f.Model.Trades}, {"candles", f.Model.Candles}} {
		t.Run(read.name, func(t *testing.T) {
			page, err := read.run(t.Context(), request)
			if !errors.Is(err, store.ErrAccountingUntraded) || page.Items == nil || len(page.Items) != 0 || page.Revision != "" || string(page.Watermark) != "null" || page.NextSequence != nil || page.NextStart != nil || page.SeriesModelVersion != accountingtest.Version {
				t.Fatalf("untraded page: %+v %v", page, err)
			}
			if _, err := uuid.Parse(page.SnapshotID); err != nil {
				t.Fatal(err)
			}
			if _, err := time.Parse("2006-01-02T15:04:05.000Z", page.GeneratedAt); err != nil {
				t.Fatal(err)
			}
			for _, change := range []func(*store.AccountingPageRequest){
				func(r *store.AccountingPageRequest) { r.Pool = accountingtest.UnknownPool },
				func(r *store.AccountingPageRequest) { r.ExpectedSnapshot = page.SnapshotID },
				func(r *store.AccountingPageRequest) { v := int64(1); r.AfterSequence = &v },
				func(r *store.AccountingPageRequest) { v := int64(1699999980); r.AfterStart = &v },
			} {
				r := request
				change(&r)
				if _, err := read.run(t.Context(), r); !errors.Is(err, store.ErrAccountingNotReady) {
					t.Fatalf("noninitial or unknown pool: %v", err)
				}
			}
			c := f.Config
			c.ProjectionGeneration = "generation2"
			other, err := store.NewAccountingReadModel(f.DB, c)
			if err != nil {
				t.Fatal(err)
			}
			otherPage, err := other.Trades(t.Context(), request)
			if !errors.Is(err, store.ErrAccountingUntraded) || otherPage.SnapshotID == page.SnapshotID {
				t.Fatalf("generation identity: %+v %v", otherPage, err)
			}
		})
	}
	for _, statement := range []string{
		`UPDATE indexer.indexer_block_completeness SET fetch_complete=false WHERE height=100`,
		`UPDATE indexer.indexer_block_completeness SET fetch_complete=true,required_decode_complete=false WHERE height=100`,
		`UPDATE indexer.indexer_block_completeness SET required_decode_complete=true,stream_id='other' WHERE height=100`,
	} {
		f.Exec(t, statement)
		for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
			if _, err := read(t.Context(), request); !errors.Is(err, store.ErrAccountingNotReady) {
				t.Fatalf("incomplete creation: %v", err)
			}
		}
	}
	f.Exec(t, `UPDATE indexer.indexer_block_completeness SET stream_id=$1 WHERE height=100`, accountingtest.Stream)
	fill, position := f.Publish(t)
	for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
		if page, err := read(t.Context(), request); !errors.Is(err, store.ErrAccountingUntraded) || page.Items == nil || len(page.Items) != 0 || page.Revision != "" || string(page.Watermark) != "null" {
			t.Fatalf("position awaiting consumer offsets: %+v %v", page, err)
		}
	}
	f.Commit(t, fill, position)
	for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
		page, err := read(t.Context(), request)
		if err != nil || len(page.Items) != 1 || page.Revision != "1" || string(page.Watermark) == "null" {
			t.Fatalf("committed market page: %+v %v", page, err)
		}
	}
	f.Exec(t, `UPDATE candles.delivery_partition_commits SET position='{}'`)
	for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
		if _, err := read(t.Context(), request); !errors.Is(err, store.ErrDataIntegrity) {
			t.Fatalf("invalid commit: %v", err)
		}
	}
}

func TestUntradedPoolPageRequiresAbsentConsumerOffsets(t *testing.T) {
	for _, schema := range []string{"candles", "stats"} {
		t.Run(schema, func(t *testing.T) {
			f := accountingtest.New(t)
			consumer := "charts"
			if schema == "stats" {
				consumer = "stats"
			}
			f.Exec(t, "INSERT INTO "+pgx.Identifier{schema, "delivery_partition_leases"}.Sanitize()+" VALUES($1,$2,'market',$3,'fixture',1,now()+interval '1 hour')", consumer, accountingtest.Generation, "125:"+accountingtest.Pool)
			f.Exec(t, "INSERT INTO "+pgx.Identifier{schema, "delivery_partition_offsets"}.Sanitize()+" VALUES($1,$2,'market',$3,$4,1,'',1,'')", consumer, accountingtest.Generation, "125:"+accountingtest.Pool, strings.Repeat("a", 64))
			r := store.AccountingPageRequest{Pool: accountingtest.Pool, Limit: 5, Interval: "1m", From: 1699999980000, To: 1700000040000}
			for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
				if _, err := read(t.Context(), r); !errors.Is(err, store.ErrDataIntegrity) {
					t.Fatalf("unverified consumer offset: %v", err)
				}
			}
		})
	}
}

package accountingtest_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store/accountingtest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestMarketPublicationBecomesReadableAfterBothConsumersCommit(t *testing.T) {
	f := accountingtest.New(t)
	request := store.AccountingPageRequest{Pool: accountingtest.Pool, Limit: 5, Interval: "1m", From: 1699999980000, To: 1700000040000}
	if page, err := f.Model.Trades(t.Context(), request); !errors.Is(err, store.ErrAccountingUntraded) || len(page.Items) != 0 {
		t.Fatalf("untraded pool: %+v, %v", page, err)
	}
	fill, position := f.Publish(t)
	if err := accounting.ValidatePartitionPosition(position); err != nil || position.Sequence != "1" || position.PartitionKey != "125:"+accountingtest.Pool {
		t.Fatalf("published partition: %+v, %v", position, err)
	}
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := f.DB.QueryRow(t.Context(), `SELECT payload FROM indexer.indexer_ingest_outbox WHERE transaction_hash=$1 AND log_index=0`, fill.Position.TransactionHash).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	want, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	var persisted accounting.PublicationV1
	if err := json.Unmarshal(raw, &persisted); err != nil || persisted.PublicationID != publication.PublicationID {
		t.Fatalf("persisted publication: %+v, %v", persisted, err)
	}
	canonical, err := accounting.CanonicalPublicationV1(persisted)
	if err != nil || string(canonical) != string(want) {
		t.Fatalf("publication content differs: %s, %v", canonical, err)
	}
	if _, err := f.Model.Trades(t.Context(), request); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("publication without consumer commits: %v", err)
	}
	f.Commit(t, fill, position)
	for _, tc := range []struct {
		name, document string
		read           func() (store.AccountingPage, error)
	}{
		{"trades", "Trade", func() (store.AccountingPage, error) { return f.Model.Trades(t.Context(), request) }},
		{"candles", "Candle", func() (store.AccountingPage, error) { return f.Model.Candles(t.Context(), request) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, err := tc.read()
			if err != nil || len(page.Items) != 1 || page.Revision != "1" || page.SnapshotID == "" || string(page.Watermark) == "null" {
				t.Fatalf("committed page: %+v, %v", page, err)
			}
			if err := interfaces.ValidateJSON(tc.document, page.Items[0]); err != nil {
				t.Fatalf("public %s: %v", tc.document, err)
			}
			var item struct {
				Pool    string `json:"poolAddress"`
				Volume  string `json:"volumeUsdlRaw"`
				EventID string `json:"eventId"`
			}
			if err := json.Unmarshal(page.Items[0], &item); err != nil || item.Pool != accountingtest.Pool {
				t.Fatalf("public pool: %+v, %v", item, err)
			}
			if tc.name == "trades" && item.EventID != fill.SourceFillID || tc.name == "candles" && item.Volume != fill.USDLAmountRaw {
				t.Fatalf("public market values: %+v", item)
			}
		})
	}
	for _, schema := range []string{"candles", "stats"} {
		var next int64
		var digest string
		if err := f.DB.QueryRow(t.Context(), `SELECT next_sequence,checkpoint_digest FROM `+schema+`.delivery_partition_offsets WHERE partition_key=$1`, position.PartitionKey).Scan(&next, &digest); err != nil || next != 2 || len(digest) != 64 {
			t.Fatalf("%s consumer position: next=%d checkpoint=%q err=%v", schema, next, digest, err)
		}
	}
}

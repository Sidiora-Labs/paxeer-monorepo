package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store/accountingtest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func lowerHeadRequest() store.AccountingPageRequest {
	return store.AccountingPageRequest{Pool: accountingtest.Pool, Limit: 1, Interval: "1m", From: 1699999980000, To: 1700000100000}
}

func lowerHeadMarkets(t *testing.T, f accountingtest.Fixture) ([]accounting.FillV1, []accounting.PartitionPosition) {
	t.Helper()
	fills := make([]accounting.FillV1, 3)
	positions := make([]accounting.PartitionPosition, 3)
	for i := range fills {
		fills[i], positions[i] = f.PublishSequence(t, int64(i+1))
		f.CommitConsumer(t, f.Config.ChartsConsumerID, fills[i], positions[i])
		if i < 2 {
			f.CommitConsumer(t, f.Config.StatsConsumerID, fills[i], positions[i])
		}
	}
	return fills, positions
}

func lowerHeadContinuation(t *testing.T, r store.AccountingPageRequest, candles bool, page store.AccountingPage) store.AccountingPageRequest {
	t.Helper()
	if !candles {
		r.Interval = ""
		r.From = 0
		r.To = 0
	}
	codec := rest.AccountingV2Handler{SigningKey: []byte(strings.Repeat("k", 32))}
	body, err := codec.EncodeCachePage(r, candles, page, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		NextCursor string `json:"nextCursor"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.NextCursor == "" {
		t.Fatalf("continuation envelope: %s %v", body, err)
	}
	query := url.Values{"limit": {strconv.Itoa(r.Limit)}, "cursor": {envelope.NextCursor}}
	if candles {
		query.Set("interval", "1m")
		query.Set("from", "2023-11-14T22:13:00.000Z")
		query.Set("to", "2023-11-14T22:15:00.000Z")
	}
	req := httptest.NewRequest("GET", "/?"+query.Encode(), nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("poolAddress", accountingtest.Pool)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	next, err := codec.ParseCacheRequest(req, candles)
	if err != nil || next.SnapshotSequence == nil || *next.SnapshotSequence != 2 || next.ExpectedSnapshot != page.SnapshotID {
		t.Fatalf("pinned cursor: %+v %v", next, err)
	}
	return next
}

func TestAccountingLowerHeadReadsAdvanceAndKeepCursors(t *testing.T) {
	f := accountingtest.New(t)
	fills, positions := lowerHeadMarkets(t, f)
	r := lowerHeadRequest()
	trades, err := f.Model.Trades(t.Context(), r)
	if err != nil || trades.Revision != "2" || len(trades.Items) != 1 || trades.NextSequence == nil {
		t.Fatalf("common trade page: %+v %v", trades, err)
	}
	candles, err := f.Model.Candles(t.Context(), r)
	if err != nil || candles.Revision != "2" || len(candles.Items) != 1 || candles.NextStart == nil || candles.SnapshotID != trades.SnapshotID {
		t.Fatalf("common candle page: %+v %v", candles, err)
	}
	var trade struct {
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(trades.Items[0], &trade); err != nil || trade.EventID != fills[1].SourceFillID {
		t.Fatalf("trade at common sequence: %s %v", trades.Items[0], err)
	}
	var watermark struct {
		ProjectionRevision string `json:"projectionRevision"`
		Height             string `json:"height"`
		BlockHash          string `json:"blockHash"`
	}
	if err := json.Unmarshal(trades.Watermark, &watermark); err != nil || watermark.ProjectionRevision != "2" || watermark.Height != positions[1].SourceOrder.Height || watermark.BlockHash != positions[1].SourceOrder.BlockHash {
		t.Fatalf("common watermark: %s %v", trades.Watermark, err)
	}
	tradeNext := lowerHeadContinuation(t, r, false, trades)
	candleNext := lowerHeadContinuation(t, r, true, candles)
	f.CommitConsumer(t, f.Config.StatsConsumerID, fills[2], positions[2])
	for _, tc := range []struct {
		name, document string
		read           func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error)
		first          store.AccountingPage
		next           store.AccountingPageRequest
	}{{"trades", "Trade", f.Model.Trades, trades, tradeNext}, {"candles", "Candle", f.Model.Candles, candles, candleNext}} {
		t.Run(tc.name, func(t *testing.T) {
			advanced, err := tc.read(t.Context(), r)
			if err != nil || advanced.Revision != "3" || advanced.SnapshotID == tc.first.SnapshotID || len(advanced.Items) != 1 {
				t.Fatalf("advanced common page: %+v %v", advanced, err)
			}
			continued, err := tc.read(t.Context(), tc.next)
			if err != nil || continued.Revision != "2" || continued.SnapshotID != tc.first.SnapshotID || string(continued.Watermark) != string(tc.first.Watermark) || len(continued.Items) != 1 || continued.NextSequence != nil || continued.NextStart != nil {
				t.Fatalf("continued common page: %+v %v", continued, err)
			}
			if err := interfaces.ValidateJSON(tc.document, continued.Items[0]); err != nil {
				t.Fatal(err)
			}
			if tc.name == "trades" {
				if err := json.Unmarshal(continued.Items[0], &trade); err != nil || trade.EventID != fills[0].SourceFillID {
					t.Fatalf("continued trade: %s %v", continued.Items[0], err)
				}
			} else {
				var candle struct {
					Start string `json:"start"`
				}
				if err := json.Unmarshal(continued.Items[0], &candle); err != nil || candle.Start != "2023-11-14T22:14:00.000Z" {
					t.Fatalf("continued candle: %s %v", continued.Items[0], err)
				}
			}
		})
	}
}

func TestAccountingLowerHeadUntradedUntilBothCommit(t *testing.T) {
	for _, consumer := range []string{"charts", "stats"} {
		t.Run(consumer, func(t *testing.T) {
			f := accountingtest.New(t)
			r := lowerHeadRequest()
			before, err := f.Model.Trades(t.Context(), r)
			if !errors.Is(err, store.ErrAccountingUntraded) {
				t.Fatal(err)
			}
			fill, position := f.Publish(t)
			for stage := 0; stage < 2; stage++ {
				if stage == 1 {
					f.CommitConsumer(t, consumer, fill, position)
				}
				for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
					page, err := read(t.Context(), r)
					if !errors.Is(err, store.ErrAccountingUntraded) || page.Items == nil || len(page.Items) != 0 || page.SnapshotID != before.SnapshotID || page.Revision != "" || string(page.Watermark) != "null" || page.NextStart != nil || page.NextSequence != nil {
						t.Fatalf("pending consumer page: %+v %v", page, err)
					}
					pinned := r
					pinned.ExpectedSnapshot = before.SnapshotID
					if _, err := read(t.Context(), pinned); !errors.Is(err, store.ErrAccountingNotReady) {
						t.Fatalf("uncommitted continuation: %v", err)
					}
				}
			}
		})
	}
}

func TestAccountingLowerHeadIntegrityStops(t *testing.T) {
	for _, tc := range []struct{ name, statement string }{
		{"charts_fence", `UPDATE candles.chart_pool_fences SET frozen=true`},
		{"stats_fence", `UPDATE stats.accounting_pool_fences SET frozen=true`},
		{"same_sequence_position", `UPDATE stats.delivery_partition_commits SET position=convert_to(jsonb_set(convert_from(position,'UTF8')::jsonb,'{sourceOrder,logIndex}','"1"')::text,'UTF8') WHERE sequence=2`},
		{"head_position", `UPDATE candles.delivery_partition_commits SET position='{}' WHERE sequence=3`},
		{"checkpoint", `UPDATE stats.delivery_partition_offsets SET checkpoint_digest=repeat('a',64)`},
		{"digest", `UPDATE stats.accounting_market_commits SET source_fact_digest=repeat('a',64) WHERE sequence=2`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := accountingtest.New(t)
			lowerHeadMarkets(t, f)
			f.Exec(t, tc.statement)
			for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
				if _, err := read(t.Context(), lowerHeadRequest()); !errors.Is(err, store.ErrDataIntegrity) {
					t.Fatalf("integrity stop: %v", err)
				}
			}
		})
	}
	for _, schema := range []string{"candles", "stats"} {
		t.Run(schema+"_pending_fence", func(t *testing.T) {
			f := accountingtest.New(t)
			f.Publish(t)
			if schema == "candles" {
				f.Exec(t, `INSERT INTO candles.chart_pool_fences VALUES('125',$1,1,true)`, accountingtest.Pool)
			} else {
				f.Exec(t, `INSERT INTO stats.accounting_pool_fences(pool,frozen) VALUES($1,true)`, accountingtest.Pool)
			}
			for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
				if _, err := read(t.Context(), lowerHeadRequest()); !errors.Is(err, store.ErrDataIntegrity) {
					t.Fatalf("pending integrity stop: %v", err)
				}
			}
		})
	}
}

func TestAccountingLowerHeadBootstrapWithoutCommits(t *testing.T) {
	f := accountingtest.New(t)
	f.Publish(t)
	f.BootstrapConsumer(t, f.Config.ChartsConsumerID)
	f.BootstrapConsumer(t, f.Config.StatsConsumerID)
	r := lowerHeadRequest()
	for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
		page, err := read(t.Context(), r)
		if !errors.Is(err, store.ErrAccountingUntraded) || page.Items == nil || len(page.Items) != 0 || page.Revision != "" || string(page.Watermark) != "null" {
			t.Fatalf("bootstrapped pending page: %+v %v", page, err)
		}
	}
}

func TestAccountingLowerHeadStatsAhead(t *testing.T) {
	f := accountingtest.New(t)
	for sequence := int64(1); sequence <= 3; sequence++ {
		fill, position := f.PublishSequence(t, sequence)
		f.CommitConsumer(t, f.Config.StatsConsumerID, fill, position)
		if sequence < 3 {
			f.CommitConsumer(t, f.Config.ChartsConsumerID, fill, position)
		}
	}
	r := lowerHeadRequest()
	r.Limit = 5
	for _, read := range []func(context.Context, store.AccountingPageRequest) (store.AccountingPage, error){f.Model.Trades, f.Model.Candles} {
		page, err := read(t.Context(), r)
		if err != nil || page.Revision != "2" || len(page.Items) != 2 {
			t.Fatalf("stats ahead common page: %+v %v", page, err)
		}
	}
}

func TestAccountingLowerHeadFrozenPinnedPages(t *testing.T) {
	for _, schema := range []string{"candles", "stats"} {
		t.Run(schema, func(t *testing.T) {
			f := accountingtest.New(t)
			lowerHeadMarkets(t, f)
			r := lowerHeadRequest()
			trades, err := f.Model.Trades(t.Context(), r)
			if err != nil {
				t.Fatal(err)
			}
			candles, err := f.Model.Candles(t.Context(), r)
			if err != nil {
				t.Fatal(err)
			}
			tradeNext := lowerHeadContinuation(t, r, false, trades)
			candleNext := lowerHeadContinuation(t, r, true, candles)
			if schema == "candles" {
				f.Exec(t, `UPDATE candles.chart_pool_fences SET frozen=true`)
			} else {
				f.Exec(t, `UPDATE stats.accounting_pool_fences SET frozen=true`)
			}
			if _, err := f.Model.Trades(t.Context(), tradeNext); !errors.Is(err, store.ErrDataIntegrity) {
				t.Fatalf("frozen trade continuation: %v", err)
			}
			if _, err := f.Model.Candles(t.Context(), candleNext); !errors.Is(err, store.ErrDataIntegrity) {
				t.Fatalf("frozen candle continuation: %v", err)
			}
		})
	}
}

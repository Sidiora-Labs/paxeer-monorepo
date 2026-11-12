package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestCanonicalPagesReadCommittedFillAndCandle(t *testing.T) {
	model, db := productionAccountingFixture(t)
	ctx := context.Background()
	insertProductionFill(t, db, 1, "")
	fill, _, _ := productionFill(t, productionAddress, productionBase, 1, "")
	for _, tc := range []struct {
		category, wallet, metric string
	}{
		{"volume", fill.Attribution.TransactionSender, "1000000"},
		{"creator", "0x" + "5555555555555555555555555555555555555555", "3000"},
	} {
		board, err := model.Leaderboard(ctx, tc.category, "24h", 1700000001)
		if err != nil || len(board.Entries) != 1 || board.Entries[0].Wallet != tc.wallet || board.Entries[0].MetricRaw != tc.metric || board.Revision != "1" {
			t.Fatalf("%s committed leaderboard: %+v, %v", tc.category, board, err)
		}
	}
	trades, err := model.Trades(ctx, store.AccountingPageRequest{Pool: productionAddress, Limit: 10})
	if err != nil || len(trades.Items) != 1 || trades.Revision != "1" || trades.SnapshotID == "" {
		t.Fatalf("committed trade: %+v, %v", trades, err)
	}
	var trade struct {
		EventID string `json:"eventId"`
		Side    string `json:"side"`
	}
	if err := json.Unmarshal(trades.Items[0], &trade); err != nil || trade.EventID != fill.SourceFillID || trade.Side != "buy" {
		t.Fatalf("trade document: %+v, %v", trade, err)
	}
	source := map[string]any{"sourceFillId": fill.SourceFillID, "position": fill.Position, "actualEmitter": fill.ActualEmitter, "provenance": fill.Provenance}
	price := map[string]string{"numerator": "1", "denominator": "1"}
	const start = int64(1700000000)
	candle, err := json.Marshal(map[string]any{
		"poolAddress": productionAddress, "interval": "1m", "start": start,
		"open": price, "high": price, "low": price, "close": price,
		"openSource": source, "highSource": source, "lowSource": source, "closeSource": source,
		"volumeUsdlRaw": "1000000", "volumeTokenRaw": fill.TokenAmountRaw,
		"buyVolumeUsdlRaw": "1000000", "sellVolumeUsdlRaw": "0", "trades": 1, "revision": 1,
		"token": fill.Token, "settlement": fill.Settlement,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',$2,$3)`, productionAddress, start, candle); err != nil {
		t.Fatal(err)
	}
	request := store.AccountingPageRequest{Pool: productionAddress, Interval: "1m", From: start * 1000, To: (start + 60) * 1000, Limit: 10}
	page, err := model.Candles(ctx, request)
	if err != nil || len(page.Items) != 1 || page.Revision != "1" || page.SnapshotID != trades.SnapshotID {
		t.Fatalf("committed candle: %+v, %v", page, err)
	}
	var document struct {
		Trades string `json:"trades"`
		Open   struct {
			Numerator string `json:"numerator"`
		} `json:"open"`
	}
	if err := json.Unmarshal(page.Items[0], &document); err != nil || document.Trades != "1" || document.Open.Numerator != "1" {
		t.Fatalf("candle document: %+v, %v", document, err)
	}
	for _, statement := range []string{
		`CREATE SCHEMA api_stream_dev`,
		`CREATE TABLE api_stream_dev.candle_projection(pool_address text,timeframe text,candle_start bigint,source_revision bigint,topic_sequence bigint,payload bytea,watermark bytea)`,
	} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO candles.candles(pool_address,timeframe,candle_start,open,high,low,close,trade_count,last_trade_ts,sequence_num) VALUES($1,'1m',$2,'1','1','1','1',1,$2,1)`, productionAddress, start); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO api_stream_dev.candle_projection VALUES($1,'1m',$2,1,1,$3,$4)`, productionAddress, start, page.Items[0], page.Watermark); err != nil {
		t.Fatal(err)
	}
	development, err := store.New(db, nil).DevelopmentCandlePage(ctx, request)
	if err != nil || development == nil || len(development.Items) != 1 || development.Revision != "1" || development.SnapshotID == "" || development.SeriesModelVersion != productionProjection {
		t.Fatalf("development candle page: %+v, %v", development, err)
	}
	request.ExpectedSnapshot = "another-snapshot"
	if _, err := store.New(db, nil).DevelopmentCandlePage(ctx, request); !errors.Is(err, store.ErrDevelopmentCandleSnapshotChanged) {
		t.Fatalf("changed development snapshot accepted: %v", err)
	}
	var second map[string]any
	if err := json.Unmarshal(page.Items[0], &second); err != nil {
		t.Fatal(err)
	}
	secondStart := time.Unix(start+60, 0).UTC().Format("2006-01-02T15:04:05.000Z")
	second["start"] = secondStart
	secondPayload, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if err := interfaces.ValidateJSON("Candle", secondPayload); err != nil {
		t.Fatalf("next candle document: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO candles.candles(pool_address,timeframe,candle_start,open,high,low,close,trade_count,last_trade_ts,sequence_num) VALUES($1,'1m',$2,'1','1','1','1',1,$2,2)`, productionAddress, start+60); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO api_stream_dev.candle_projection VALUES($1,'1m',$2,1,2,$3,$4)`, productionAddress, start+60, secondPayload, page.Watermark); err != nil {
		t.Fatal(err)
	}
	request.ExpectedSnapshot = ""
	request.To = (start + 120) * 1000
	request.Limit = 1
	firstDevelopment, err := store.New(db, nil).DevelopmentCandlePage(ctx, request)
	if err != nil || firstDevelopment == nil || len(firstDevelopment.Items) != 1 || firstDevelopment.NextStart == nil || *firstDevelopment.NextStart != start || firstDevelopment.Revision != "2" {
		t.Fatalf("first development candle page: %+v, %v", firstDevelopment, err)
	}
	request.AfterStart = firstDevelopment.NextStart
	request.ExpectedSnapshot = firstDevelopment.SnapshotID
	secondDevelopment, err := store.New(db, nil).DevelopmentCandlePage(ctx, request)
	if err != nil || secondDevelopment == nil || len(secondDevelopment.Items) != 1 || secondDevelopment.NextStart != nil || secondDevelopment.SnapshotID != firstDevelopment.SnapshotID {
		t.Fatalf("next development candle page: %+v, %v, payload=%s", secondDevelopment, err, secondPayload)
	}
	var nextDocument struct {
		Start string `json:"start"`
	}
	if err := json.Unmarshal(secondDevelopment.Items[0], &nextDocument); err != nil || nextDocument.Start != secondStart {
		t.Fatalf("next candle: %+v, %v", nextDocument, err)
	}
	request.AfterStart = nil
	request.ExpectedSnapshot = ""
	request.Limit = 2
	if _, err := db.Exec(ctx, `UPDATE api_stream_dev.candle_projection SET watermark='{' WHERE pool_address=$1 AND candle_start=$2`, productionAddress, start+60); err != nil {
		t.Fatal(err)
	}
	if _, err := store.New(db, nil).DevelopmentCandlePage(ctx, request); err == nil {
		t.Fatal("malformed projection watermark accepted")
	}
	if _, err := db.Exec(ctx, `UPDATE api_stream_dev.candle_projection SET watermark=$3 WHERE pool_address=$1 AND candle_start=$2`, productionAddress, start+60, page.Watermark); err != nil {
		t.Fatal(err)
	}
	var differentVersion map[string]any
	if err := json.Unmarshal(secondPayload, &differentVersion); err != nil {
		t.Fatal(err)
	}
	differentVersion["seriesModelVersion"] = "another-series"
	differentPayload, err := json.Marshal(differentVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := interfaces.ValidateJSON("Candle", differentPayload); err != nil {
		t.Fatalf("alternate series document: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE api_stream_dev.candle_projection SET payload=$3 WHERE pool_address=$1 AND candle_start=$2`, productionAddress, start+60, differentPayload); err != nil {
		t.Fatal(err)
	}
	if _, err := store.New(db, nil).DevelopmentCandlePage(ctx, request); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("mixed series models accepted: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE api_stream_dev.candle_projection SET payload='{}' WHERE pool_address=$1 AND candle_start=$2`, productionAddress, start+60); err != nil {
		t.Fatal(err)
	}
	if _, err := store.New(db, nil).DevelopmentCandlePage(ctx, request); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid candle projection accepted: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO candles.candles(pool_address,timeframe,candle_start,open,high,low,close,trade_count,last_trade_ts,sequence_num) VALUES($1,'1m',$2,'1','1','1','1',1,$2,3)`, productionAddress, start+120); err != nil {
		t.Fatal(err)
	}
	request.To = (start + 180) * 1000
	if _, err := store.New(db, nil).DevelopmentCandlePage(ctx, request); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("candle without committed projection accepted: %v", err)
	}
	request.ExpectedSnapshot = "another-snapshot"
	if _, err := model.Candles(ctx, request); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("changed snapshot accepted: %v", err)
	}
	request.ExpectedSnapshot = ""
	request.Interval = "2m"
	if _, err := model.Candles(ctx, request); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid interval accepted: %v", err)
	}
	canonical := store.AccountingPageRequest{Pool: productionAddress, Interval: "1m", From: start * 1000, To: (start + 60) * 1000, Limit: 10}
	outside := start - 1
	canonical.AfterStart = &outside
	if _, err := model.Candles(ctx, canonical); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("candle boundary outside range accepted: %v", err)
	}
	canonical.AfterStart = nil
	if _, err := db.Exec(ctx, `UPDATE candles.chart_exact SET snapshot='{}' WHERE pool=$1 AND start=$2`, productionAddress, start); err != nil {
		t.Fatal(err)
	}
	if _, err := model.Candles(ctx, canonical); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("candle without source fields accepted: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE candles.chart_exact SET snapshot=$3 WHERE pool=$1 AND start=$2`, productionAddress, start, candle); err != nil {
		t.Fatal(err)
	}
	tradeRequest := store.AccountingPageRequest{Pool: productionAddress, Limit: 10}
	sequence := int64(2)
	tradeRequest.AfterSequence = &sequence
	if _, err := model.Trades(ctx, tradeRequest); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("trade sequence beyond committed frontier accepted: %v", err)
	}
	tradeRequest.AfterSequence = nil
	position := fill.Position
	tradeRequest.AfterOrder = &position
	tradeRequest.AfterEventID = "unknown-fill"
	if _, err := model.Trades(ctx, tradeRequest); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("unknown trade boundary accepted: %v", err)
	}
	tradeRequest.AfterOrder = nil
	tradeRequest.AfterEventID = ""
	if _, err := db.Exec(ctx, `UPDATE stats.accounting_market_commits SET source_fact_digest=$2 WHERE pool=$1`, productionAddress, "0"+strings.Repeat("a", 63)); err != nil {
		t.Fatal(err)
	}
	if _, err := model.Trades(ctx, tradeRequest); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("trade digest mismatch accepted: %v", err)
	}
}

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/candlewire"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/candlewire/testfixture"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestCandleIdentityCanonicalHistory(t *testing.T) {
	document, err := os.ReadFile("../../../../../deploy/launchpad/documents/values/charts.example.json")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Features struct {
			SeriesModel string `json:"canonicalSeriesModel"`
		} `json:"features"`
	}
	if err = json.Unmarshal(document, &config); err != nil {
		t.Fatal(err)
	}
	if config.Features.SeriesModel != candlewire.SeriesModelVersion {
		t.Fatal("published series model differs from configured series model")
	}
	raw, err := os.ReadFile("../../candlewire/testdata/candle_identity.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []testfixture.Record
	if err = json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	db := historyPostgres(t)
	if _, err = db.Exec(t.Context(), `CREATE SCHEMA candle_identity; CREATE TABLE candle_identity.chart_exact(chain_id text,pool text,interval text,start bigint,snapshot jsonb); CREATE VIEW candle_identity.chart_exact_v1 AS SELECT * FROM candle_identity.chart_exact`); err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		var bar struct {
			Pool     string `json:"poolAddress"`
			Interval string `json:"interval"`
			Start    int64  `json:"start"`
		}
		if err = json.Unmarshal(record.StoredBar, &bar); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(t.Context(), `INSERT INTO candle_identity.chart_exact VALUES('125',$1,$2,$3,$4)`, bar.Pool, bar.Interval, bar.Start, record.StoredBar); err != nil {
			t.Fatal(err)
		}
		var candle map[string]any
		if err = json.Unmarshal(record.PublicCandle, &candle); err != nil {
			t.Fatal(err)
		}
		const snapshot = "11111111-1111-4111-8111-111111111111"
		candle["snapshotId"] = snapshot
		data := map[string]any{"items": []any{candle}, "snapshotId": snapshot, "seriesModelVersion": candlewire.SeriesModelVersion}
		response := map[string]any{"data": data, "requestId": "22222222-2222-4222-8222-222222222222", "revision": "1", "watermark": map[string]any{"chainId": "125", "height": "110", "blockHash": candle["lastPosition"].(map[string]any)["blockHash"], "projectionRevision": "1", "observedAt": "2023-11-14T22:13:20.000Z", "integrity": "healthy"}, "quality": "partial", "nextCursor": nil, "generatedAt": "2023-11-14T22:13:20.000Z"}
		wire, err := json.Marshal(response)
		if err != nil || interfaces.ValidateJSON("CandlesResponse", wire) != nil {
			t.Fatalf("canonical page contract: %v %s", err, wire)
		}
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v2/pools/"+bar.Pool+"/candles" || r.URL.Query().Get("interval") != bar.Interval {
				http.Error(w, "candle scope", 422)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(wire)
		}))
		t.Cleanup(origin.Close)
		request := HistoryRequest{Symbol: bar.Pool, Timeframe: bar.Interval, From: bar.Start, To: bar.Start + 300}
		source := CanonicalCandleClient{Endpoint: origin.URL, Client: origin.Client()}
		page, err := source.Read(t.Context(), request)
		if err != nil || len(page.Items) != 1 || page.Algorithm != config.Features.SeriesModel || page.SnapshotID != snapshot {
			t.Fatalf("canonical page read: %+v %v", page, err)
		}
		for i := 0; i < 2; i++ {
			cache, err := NewHistoryCache(nil, PostgresHistoryRevision{Pool: db, Schema: "candle_identity"}, 1)
			if err != nil {
				t.Fatal(err)
			}
			value, err := cache.read(t.Context(), request, modePrice, config.Features.SeriesModel, nil, source, nil)
			if err != nil || !validChartCache(value, value.Scope, request, modePrice, config.Features.SeriesModel) {
				t.Fatalf("canonical history cache: %+v %v", value, err)
			}
			var history struct {
				Status    string  `json:"s"`
				Times     []int64 `json:"t"`
				Algorithm string  `json:"algorithmVersion"`
			}
			if err = json.Unmarshal(value.Body, &history); err != nil || history.Status != "ok" || len(history.Times) != 1 || history.Times[0] != bar.Start || history.Algorithm != config.Features.SeriesModel {
				t.Fatalf("canonical history: %s %v", value.Body, err)
			}
		}
		data["seriesModelVersion"] = "accounting-market-v1"
		wire, err = json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = source.Read(t.Context(), request); err == nil || !strings.Contains(err.Error(), "candle scope mismatch") {
			t.Fatalf("projection label accepted: %v", err)
		}
		candle["seriesModelVersion"] = "accounting-market-v1"
		wire, err = json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		cache, err := NewHistoryCache(nil, PostgresHistoryRevision{Pool: db, Schema: "candle_identity"}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = cache.read(t.Context(), request, modePrice, config.Features.SeriesModel, nil, source, nil); err == nil || !strings.Contains(err.Error(), "history algorithm scope mismatch") {
			t.Fatalf("projection model accepted by history cache: %v", err)
		}
	}
}

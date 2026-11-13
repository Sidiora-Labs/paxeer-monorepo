package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestCanonicalCandleClientChecksRealProducerRowsAcrossPages(t *testing.T) {
	frame, err := interfaces.DecodeCandleEvent(repairM1050CandleFrames(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	first := frame.Payload
	stamp, err := time.Parse("2006-01-02T15:04:05.000Z", first.Start)
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.Start = stamp.Add(time.Minute).Format("2006-01-02T15:04:05.000Z")
	revision := interfaces.PositiveUInt256("1")
	watermark := &interfaces.Watermark{ChainId: "125", Height: "103", BlockHash: "0x" + strings.Repeat("a", 64), Integrity: "healthy", ObservedAt: stamp.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"), ProjectionRevision: "1"}
	base := interfaces.CandlesResponse{Data: interfaces.CandlesData{Items: []interfaces.Candle{first, second}, SeriesModelVersion: first.SeriesModelVersion, SnapshotId: first.SnapshotId}, GeneratedAt: stamp.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"), Quality: "complete", RequestId: "22222222-2222-4222-8222-222222222222", Revision: &revision, Watermark: watermark}
	request := HistoryRequest{Symbol: first.PoolAddress, Timeframe: first.Interval, From: stamp.Unix(), To: stamp.Add(2 * time.Minute).Unix(), Countback: 1}
	for _, tc := range []struct {
		name, want string
		change     func(*interfaces.CandlesResponse)
	}{
		{"valid countback", "", func(*interfaces.CandlesResponse) {}},
		{"unhealthy source", "invalid frozen chart upstream response", func(p *interfaces.CandlesResponse) { p.Watermark.Integrity = "degraded" }},
		{"stale quality", "not healthy", func(p *interfaces.CandlesResponse) { p.Quality = "stale" }},
		{"changed pool", "scope mismatch", func(p *interfaces.CandlesResponse) { p.Data.Items[0].PoolAddress = "0x" + strings.Repeat("b", 40) }},
		{"unordered candles", "order mismatch", func(p *interfaces.CandlesResponse) { p.Data.Items[1].Start = p.Data.Items[0].Start }},
		{"outside request", "scope mismatch", func(p *interfaces.CandlesResponse) {
			p.Data.Items[0].Start = stamp.Add(-time.Minute).Format("2006-01-02T15:04:05.000Z")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := base
			page.Data.Items = append([]interfaces.Candle(nil), base.Data.Items...)
			copyWatermark := *watermark
			page.Watermark = &copyWatermark
			tc.change(&page)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(page); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			got, err := (CanonicalCandleClient{Endpoint: server.URL}).Read(context.Background(), request)
			if tc.want == "" {
				if err != nil || len(got.Items) != 1 || got.Items[0].Start != second.Start || got.SnapshotID != first.SnapshotId {
					t.Fatalf("bounded canonical countback: %+v, %v", got, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("canonical page admission: %+v, %v", got, err)
			}
		})
	}
}

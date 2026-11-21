package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestCandlePresentationRejectsInvalidMetadataAndSubscriptions(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		edit       func(*CandleSnapshot)
	}{
		{"missing watermark", "invalid snapshot metadata", func(c *CandleSnapshot) { c.SourceWatermark = "" }},
		{"unsupported interval", "unknown timeframe", func(c *CandleSnapshot) { c.Timeframe = "2m" }},
		{"revision overflow", "revision exceeds uint256", func(c *CandleSnapshot) { c.Revision = new(big.Int).Lsh(big.NewInt(1), 256).String() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fixture()
			tc.edit(&c)
			if raw, err := EncodeCandle(c); raw != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid presentation: %s, %v", raw, err)
			}
			coalescer := NewCandleCoalescer(0)
			if err := coalescer.Add(c); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid pending snapshot: %v", err)
			}
			if got := coalescer.Drain(time.Now()); len(got) != 0 {
				t.Fatalf("invalid snapshot retained: %+v", got)
			}
		})
	}
	pools := []string{}
	for i := 1; i <= 11; i++ {
		pools = append(pools, fmt.Sprintf("0x%040x", i))
	}
	if _, _, err := validateSubscriptions(pools, []string{"1m", "5m"}); err == nil || !strings.Contains(err.Error(), "subscription quota exceeded") {
		t.Fatalf("series cross product quota: %v", err)
	}
	c := NewCandleCoalescer(-1)
	first := fixture()
	if err := c.Add(first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Start += 60
	if err := c.Add(second); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("minimum retention capacity: %v", err)
	}
}

func TestHistoryPresentationRejectsUnboundedRequests(t *testing.T) {
	request := HistoryRequest{Symbol: fixture().PoolAddress, Timeframe: "1m", From: 60, To: 120}
	query := url.Values{"symbol": {request.Symbol}, "resolution": {"1"}, "to": {"120"}}
	if _, err := parseHistory(query); err == nil || !strings.Contains(err.Error(), "from required") {
		t.Fatalf("missing history lower bound: %v", err)
	}
	invalid := request
	invalid.Timeframe = "2m"
	if err := validateHistoryRequest(invalid); err == nil {
		t.Fatal("unsupported history interval accepted")
	}
	handler := HistoryHandler{SigningKey: bytes.Repeat([]byte{1}, 32)}
	if _, err := handler.applyCursor(request, "undelimited"); err == nil {
		t.Fatal("undelimited history cursor accepted")
	}
	if _, err := developmentValuationHistory(request, nil, "unsupported"); err == nil || !strings.Contains(err.Error(), "invalid valuation mode") {
		t.Fatalf("invalid history valuation mode: %v", err)
	}
	row := store.CandleRow{CandleStart: 120, Open: "1", High: "1", Low: "1", Close: "1", VolumeUsdl: "0"}
	if _, err := developmentValuationHistory(request, []store.CandleRow{row}, modePrice); err == nil || !strings.Contains(err.Error(), "outside requested scope") {
		t.Fatalf("out-of-range candle: %v", err)
	}
	row.CandleStart = 60
	row.High = "1" + strings.Repeat("0", 400)
	if _, err := developmentValuationHistory(request, []store.CandleRow{row}, modePrice); err == nil {
		t.Fatal("overflowing candle display accepted")
	}
}

func TestMetadataPresentationRejectsMalformedDocuments(t *testing.T) {
	if got := boundedMetadataText("\n<>\u202e", 16, "invalid"); got != "" {
		t.Fatalf("empty sanitized metadata: %q", got)
	}
	if raw, err := normalizePoolMetadataJSON([]byte("{")); raw != nil || err == nil {
		t.Fatalf("malformed metadata document: %s, %v", raw, err)
	}
	raw := []byte(`{"data":{"token":{"address":125,"name":"Coin"}}}`)
	got, err := normalizePoolMetadataJSON(raw)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("non-string metadata address: %s, %v", got, err)
	}
	if text, err := decodeMetadataString(make([]byte, 63)); text != "" || err == nil {
		t.Fatalf("truncated token ABI text: %q, %v", text, err)
	}
	position := interfaces.ChainPosition{ChainId: "125", Height: "invalid"}
	if _, err := (RPCSupplySource{}).At(context.Background(), fixture().PoolAddress, position); err == nil {
		t.Fatal("missing supply authority accepted")
	}
	if _, err := (RPCSupplySource{Chain: &chainread.Reader{}}).At(context.Background(), fixture().PoolAddress, position); err == nil {
		t.Fatal("invalid supply height accepted")
	}
}

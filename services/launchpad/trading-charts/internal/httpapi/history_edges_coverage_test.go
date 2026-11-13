package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestHistoryAuthoritiesRejectUnboundRequestsAndMissingRelations(t *testing.T) {
	ctx := context.Background()
	request := HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 120}
	invalid := request
	invalid.Symbol = "invalid"
	if _, err := (PostgresHistoryRevision{}).Read(ctx, invalid); err == nil || !strings.Contains(err.Error(), "invalid history pool") {
		t.Fatalf("invalid revision scope: %v", err)
	}
	if _, err := (PostgresHistoryRevision{}).Read(ctx, request); err == nil || !strings.Contains(err.Error(), "revision authority unavailable") {
		t.Fatalf("unbound revision authority: %v", err)
	}
	if _, err := (PostgresHistorySource{}).QueryHistory(ctx, invalid, 1); err == nil || !strings.Contains(err.Error(), "invalid history pool") {
		t.Fatalf("invalid native history scope: %v", err)
	}
	if _, err := (PostgresHistorySource{}).QueryHistory(ctx, request, 1); err == nil || !strings.Contains(err.Error(), "history pool unavailable") {
		t.Fatalf("unbound native history source: %v", err)
	}
	pool := historyPostgres(t)
	if _, err := (PostgresHistoryRevision{Pool: pool, Schema: "missing_revision"}).Read(ctx, request); err == nil || !strings.Contains(err.Error(), "chart_exact_v1") {
		t.Fatalf("unavailable revision relation: %v", err)
	}
	if _, err := (PostgresHistorySource{Pool: pool}).QueryHistory(ctx, request, 0); err == nil || !strings.Contains(err.Error(), "history limit invalid") {
		t.Fatalf("unbounded native history read: %v", err)
	}
	response := httptest.NewRecorder()
	(HistoryHandler{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, historyURL("from=0&to=120"), nil))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("absent history source: %d %s", response.Code, response.Body.String())
	}
}

func TestHistoryCursorAndDisplayRejectUnrepresentableValues(t *testing.T) {
	request := HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 240, Countback: 2}
	handler := HistoryHandler{SigningKey: []byte("bounded-history-cursor-signing-key-32-bytes")}
	token, err := handler.encodeCursor(request, 120)
	if err != nil {
		t.Fatal(err)
	}
	read, err := handler.applyCursor(request, token)
	if err != nil || read.To != 120 || read.From != request.From {
		t.Fatalf("countback cursor boundary: %+v, %v", read, err)
	}
	for _, value := range []string{"@@.AA", "e30.@@"} {
		if _, err := handler.applyCursor(request, value); err == nil {
			t.Fatalf("malformed signed cursor admitted: %q", value)
		}
	}
	body, err := json.Marshal(historyCursor{Fingerprint: fingerprint(request), Boundary: 120, Expires: time.Now().Add(16 * time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, handler.SigningKey)
	if _, err := mac.Write(body); err != nil {
		t.Fatal(err)
	}
	future := base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if _, err := handler.applyCursor(request, future); err == nil || !strings.Contains(err.Error(), "invalid cursor expiry") {
		t.Fatalf("future cursor expiry: %v", err)
	}
	if _, err := CanonicalHistory([]store.CandleRow{{CandleStart: -1}}); err == nil || !strings.Contains(err.Error(), "invalid stored candle time") {
		t.Fatalf("negative canonical candle time: %v", err)
	}
	row := store.CandleRow{CandleStart: 60, Open: strings.Repeat("9", 1024), High: "1000000000000000000", Low: "1000000000000000000", Close: "1000000000000000000", VolumeUsdl: "1000000"}
	response := httptest.NewRecorder()
	writeHistoryPage(response, []store.CandleRow{row}, "")
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "unrepresentable display candle") {
		t.Fatalf("unrepresentable display price: %d %s", response.Code, response.Body.String())
	}
	if cache, err := NewHistoryCache(nil, PostgresHistoryRevision{}, 0); cache != nil || err == nil {
		t.Fatalf("unbounded history concurrency: %+v, %v", cache, err)
	}
}

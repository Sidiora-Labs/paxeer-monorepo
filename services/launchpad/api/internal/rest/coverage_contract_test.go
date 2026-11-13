package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestPublicErrorAndQueryContracts(t *testing.T) {
	const address = "0x1111111111111111111111111111111111111111"
	router := chi.NewRouter()
	router.Get("/v1/pools/{poolAddress}", func(w http.ResponseWriter, r *http.Request) {
		pool, ok := poolParameter(w, r)
		if ok {
			_, _ = w.Write([]byte(pool))
		}
	})
	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{"/v1/pools/" + address, 200, address},
		{"/v1/pools/invalid", 400, "ADDRESS_INVALID"},
		{"/v1/pools/" + address + "?chainId=1", 400, "CHAIN_UNSUPPORTED"},
	} {
		record := httptest.NewRecorder()
		router.ServeHTTP(record, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if record.Code != tc.status || !strings.Contains(record.Body.String(), tc.body) {
			t.Errorf("%s: status=%d body=%s", tc.path, record.Code, record.Body.String())
		}
	}

	for _, tc := range []struct {
		query url.Values
		want  string
	}{
		{url.Values{"chainId": {"125"}, "limit": {"5", "5"}}, ""},
		{url.Values{"chainId": {"126"}}, "CHAIN_UNSUPPORTED"},
		{url.Values{"limit": {"5", "6"}}, literalQUERYINVALID},
		{url.Values{strings.Repeat("x", 65): {"a"}}, literalQUERYINVALID},
		{url.Values{"x": {strings.Repeat("x", 2049)}}, literalQUERYINVALID},
	} {
		err := QueryParameters(tc.query)
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || err.Error() != tc.want) {
			t.Errorf("query %v: got %v, want %q", tc.query, err, tc.want)
		}
	}

	addresses, err := PoolAddresses(address + "," + address[:2] + strings.ToUpper(address[2:]) + ",")
	if err != nil || len(addresses) != 1 || addresses[0] != address {
		t.Fatalf("canonical addresses: %v, %v", addresses, err)
	}
	for _, raw := range []string{"invalid", strings.Repeat(address+",", 101), strings.Repeat("x", 4301)} {
		if _, err := PoolAddresses(raw); err == nil {
			t.Errorf("accepted invalid pool list of length %d", len(raw))
		}
	}
}

func TestPublicErrorEnvelopeContracts(t *testing.T) {
	for _, tc := range []struct {
		path   string
		cause  error
		status int
		code   string
		retry  bool
	}{
		{"/v1/pools", ErrCacheFillSaturated, 503, "CACHE_FILL_SATURATED", true},
		{"/v1/pools", store.ErrDataIntegrity, 500, "DATA_INTEGRITY", false},
		{"/v1/pools", ErrCacheBodyTooLarge, 500, "DATA_INTEGRITY", false},
		{"/v1/pools", store.ErrPoolNotFound, 404, "POOL_NOT_FOUND", false},
		{"/v1/pools", errors.New("origin offline"), 503, literalDATAUNAVAILABLE, true},
		{"/v2/pools", store.ErrPoolNotFound, 404, literalNOTFOUND, false},
		{"/v2/pools", store.ErrDataIntegrity, 503, literalINTEGRITYQUARANTINED, true},
	} {
		record := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		WriteReadError(record, req, tc.cause)
		var body APIError
		if err := json.Unmarshal(record.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if record.Code != tc.status || body.Code != tc.code || body.Retryable != tc.retry || body.RequestID == "" {
			t.Errorf("%s %v: status=%d body=%+v", tc.path, tc.cause, record.Code, body)
		}
	}

	for _, tc := range []struct {
		status     int
		code       string
		wantStatus int
		wantCode   string
		wantRetry  bool
	}{
		{429, "RATE_LIMITED", 429, "RATE_LIMITED", true},
		{422, literalFIELDINVALID, 422, literalFIELDINVALID, false},
		{418, "UNKNOWN", 500, "INTERNAL_ERROR", false},
	} {
		record := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/pools", nil)
		req.Header.Set("Authorization", "Bearer sample")
		WriteV2APIError(record, req, tc.status, tc.code)
		var body APIError
		if err := json.Unmarshal(record.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if record.Code != tc.wantStatus || body.Code != tc.wantCode || body.Retryable != tc.wantRetry || record.Header().Get("Cache-Control") != "private, no-store" {
			t.Errorf("v2 %s: status=%d body=%+v headers=%v", tc.code, record.Code, body, record.Header())
		}
	}

	record := httptest.NewRecorder()
	WriteV2NotConfiguredError(record, httptest.NewRequest(http.MethodGet, "/v2/pools", nil))
	var body APIError
	if err := json.Unmarshal(record.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if record.Code != 503 || body.Code != literalDEPENDENCYUNAVAILABLE || body.Retryable || body.RetryAfterMS != nil || record.Header().Get("Retry-After") != "" {
		t.Errorf("not configured: status=%d body=%+v", record.Code, body)
	}
}

func TestBoundedIntegerContract(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		want  int
		valid bool
	}{
		{"", 5, true}, {"1", 1, true}, {"10", 10, true},
		{"0", 0, false}, {"11", 0, false}, {"01", 0, false}, {"x", 0, false},
	} {
		got, err := boundedInteger(tc.raw, 5, 1, 10)
		if got != tc.want || (err == nil) != tc.valid {
			t.Errorf("%q: got %d, %v", tc.raw, got, err)
		}
	}
}

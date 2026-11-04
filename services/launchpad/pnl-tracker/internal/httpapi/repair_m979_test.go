package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/card"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/go-chi/chi/v5"
)

const repair979Secret = "0123456789abcdef0123456789abcdef"

func repair979Webhook() http.Handler {
	r := chi.NewRouter()
	RegisterWebhook(r, WebhookDeps{Secret: repair979Secret, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return r
}

func repair979Sign(req *http.Request, body []byte) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("X-Sidiora-Timestamp", ts)
	req.Header.Set("X-Sidiora-Signature", auth.SignWebhook(repair979Secret, ts, string(body)))
}

func repair979Result(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, status, rec.Body.String())
	}
	var failure sharedhttp.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &failure); err != nil {
		t.Fatal(err)
	}
	if failure.StatusCode != status || failure.Message != message {
		t.Fatalf("error=%+v want status=%d message=%q", failure, status, message)
	}
}

func TestRepairM_979_LegacyCardPublicationRetired(t *testing.T) {
	r := chi.NewRouter()
	// These are real archive service/store instances without a database: a
	// publication attempt cannot access storage or silently create a disclosure.
	st := store.New(nil)
	RegisterCards(r, CardDeps{Cards: card.New(st, "https://share.example", "https://og.example", nil)})
	for _, body := range []string{
		`{"ownerAddress":"0xVictim","poolAddress":"0xPool"}`,
		`{"ownerAddress":"0xOwner","poolAddress":"0xPool","previewId":"arbitrary","includeWallet":true}`,
		`{"ownerAddress":""}`, `not JSON`, "",
	} {
		t.Run(body, func(t *testing.T) {
			var consumed bytes.Buffer
			req := httptest.NewRequest(http.MethodPost, "/pnl/cards", io.TeeReader(strings.NewReader(body), &consumed))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			repair979Result(t, rec, http.StatusGone, "legacy card minting retired; use owner-approved /v3/me/portfolio/shares/preview and /v3/me/portfolio/shares")
			if consumed.Len() != 0 || rec.Header().Get("Location") != "" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("retired endpoint read body or advertised a publication: read=%d headers=%v", consumed.Len(), rec.Header())
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"cardId", "shortCode", "shareUrl", "ogUrl", "snapshot"} {
				if _, exists := payload[field]; exists {
					t.Fatalf("retired mint disclosed %s", field)
				}
			}
		})
	}
	if st.Pool() != nil {
		t.Fatal("retired route initialized persistence")
	}
}

func TestRepairM_979_WebhookBodyBound(t *testing.T) {
	const limit = 1 << 20
	for _, signed := range []bool{false, true} {
		for _, knownLength := range []bool{false, true} {
			t.Run(fmtRepair979Case(signed, knownLength), func(t *testing.T) {
				payload := []byte(strings.Repeat("x", 3*limit))
				var consumed bytes.Buffer
				req := httptest.NewRequest(http.MethodPost, "/webhooks/events", io.TeeReader(bytes.NewReader(payload), &consumed))
				req.ContentLength = -1
				req.TransferEncoding = []string{"chunked"}
				if knownLength {
					req.ContentLength = int64(len(payload))
					req.TransferEncoding = nil
				}
				if signed {
					repair979Sign(req, payload)
				}
				rec := httptest.NewRecorder()
				repair979Webhook().ServeHTTP(rec, req)
				repair979Result(t, rec, http.StatusRequestEntityTooLarge, "webhook body exceeds limit")
				if consumed.Len() > limit+1 || (knownLength && consumed.Len() != 0) {
					t.Fatalf("oversized request consumed %d bytes", consumed.Len())
				}
			})
		}
	}
	// Use a real HTTP connection as well: an unknown-length request is encoded
	// as chunked by net/http, with no signature and no Content-Length shortcut.
	server := httptest.NewServer(repair979Webhook())
	defer server.Close()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/webhooks/events", io.NopCloser(strings.NewReader(strings.Repeat("x", 3*limit))))
	if err != nil {
		t.Fatal(err)
	}
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var failure sharedhttp.ErrorResponse
	if err := json.Unmarshal(payload, &failure); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusRequestEntityTooLarge || failure.Message != "webhook body exceeds limit" {
		t.Fatalf("chunked unauthenticated HTTP result: %d %s", res.StatusCode, payload)
	}
}

func fmtRepair979Case(signed, knownLength bool) string {
	return "signed=" + strconv.FormatBool(signed) + "/known-length=" + strconv.FormatBool(knownLength)
}

func TestRepairM_979_WebhookRawHMACAndBoundaries(t *testing.T) {
	const limit = 1 << 20
	// Whitespace, escaped text and large JSON integers must reach authentication
	// as sent; decoding and re-encoding this payload changes the signature.
	body := []byte("{\n \"events\" : [{\"eventName\":\"PoolStateUpdated\",\"args\":{\"price\":9007199254740993}}, {\"eventName\":\"Other\\u0045vent\"}]\n}")
	var downstream bytes.Buffer
	handler := webhookHandler(WebhookDeps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	observed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		original := r.Body
		r.Body = struct {
			io.Reader
			io.Closer
		}{io.TeeReader(original, &downstream), original}
		handler.ServeHTTP(w, r)
	})
	originalReq := httptest.NewRequest(http.MethodPost, "/webhooks/events", bytes.NewReader(body))
	repair979Sign(originalReq, body)
	originalRec := httptest.NewRecorder()
	webhookAuth(repair979Secret)(observed).ServeHTTP(originalRec, originalReq)
	if originalRec.Code != http.StatusOK || !bytes.Equal(downstream.Bytes(), body) {
		t.Fatalf("authenticated raw bytes changed before production handler: status=%d downstream=%q", originalRec.Code, downstream.Bytes())
	}
	for _, size := range []int{len(body), limit - 1, limit, limit + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			payload := append(bytes.Clone(body), bytes.Repeat([]byte(" "), size-len(body))...)
			req := httptest.NewRequest(http.MethodPost, "/webhooks/events", bytes.NewReader(payload))
			repair979Sign(req, payload)
			rec := httptest.NewRecorder()
			repair979Webhook().ServeHTTP(rec, req)
			if size > limit {
				repair979Result(t, rec, http.StatusRequestEntityTooLarge, "webhook body exceeds limit")
				return
			}
			var result struct {
				OK        bool `json:"ok"`
				Processed int  `json:"processed"`
				Errors    int  `json:"errors"`
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if !result.OK || result.Processed != 2 || result.Errors != 0 {
				t.Fatalf("downstream result=%+v", result)
			}
		})
	}
	for _, alias := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/events", bytes.NewReader(body))
		repair979Sign(req, body)
		if alias {
			req.Header.Set("X-Hub-Signature-256", req.Header.Get("X-Sidiora-Signature"))
			req.Header.Del("X-Sidiora-Signature")
		}
		rec := httptest.NewRecorder()
		repair979Webhook().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("HMAC alias=%v status=%d body=%s", alias, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/events", bytes.NewReader(append(bytes.Clone(body), ' ')))
	repair979Sign(req, body)
	rec := httptest.NewRecorder()
	repair979Webhook().ServeHTTP(rec, req)
	repair979Result(t, rec, http.StatusUnauthorized, "invalid webhook signature")
}

func TestRepairM_979_WebhookEventBatchBound(t *testing.T) {
	for _, count := range []int{0, 999, 1000, 1001} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			events := make([]WebhookEvent, count)
			for i := range events {
				events[i].EventName = "PoolStateUpdated"
			}
			// If an oversized batch ever starts dispatch, the real Swap path
			// requires persistence and will fail; rejection must be atomic.
			if count > 1000 {
				events[0].EventName = "Swap"
			}
			body, err := json.Marshal(WebhookBody{Events: events})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/webhooks/events", bytes.NewReader(body))
			repair979Sign(req, body)
			rec := httptest.NewRecorder()
			repair979Webhook().ServeHTTP(rec, req)
			if count > 1000 {
				repair979Result(t, rec, http.StatusRequestEntityTooLarge, "webhook event batch exceeds limit")
				return
			}
			var result struct {
				OK        bool `json:"ok"`
				Processed int  `json:"processed"`
				Errors    int  `json:"errors"`
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if !result.OK || result.Processed != count || result.Errors != 0 {
				t.Fatalf("result=%+v want processed=%d", result, count)
			}
		})
	}
	// Reject before decoding the first out-of-budget typed event as well.
	body := []byte(`{"events":[` + strings.Repeat(`{},`, 1000) + `{"args":"invalid typed event"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/events", bytes.NewReader(body))
	repair979Sign(req, body)
	rec := httptest.NewRecorder()
	repair979Webhook().ServeHTTP(rec, req)
	repair979Result(t, rec, http.StatusRequestEntityTooLarge, "webhook event batch exceeds limit")
}

func TestRepairM_979_WebhookMalformedAndUnauthorized(t *testing.T) {
	for _, tc := range []struct{ body, message string }{
		{`{}`, "events must be an array"}, {`{"events":null}`, "events must be an array"},
		{`{"events":"nope"}`, "events must be an array"}, {`{"events":{}}`, "events must be an array"},
		{`{"events":[`, "invalid JSON body"}, {`{"events":[]} {}`, "invalid JSON body"},
		{`{"events":[{"args":"bad"}]}`, "invalid JSON body"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhooks/events", strings.NewReader(tc.body))
			repair979Sign(req, []byte(tc.body))
			rec := httptest.NewRecorder()
			repair979Webhook().ServeHTTP(rec, req)
			repair979Result(t, rec, http.StatusBadRequest, tc.message)
		})
	}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/events", strings.NewReader(`{"events":[]}`))
	rec := httptest.NewRecorder()
	repair979Webhook().ServeHTTP(rec, req)
	repair979Result(t, rec, http.StatusUnauthorized, "invalid webhook signature")
}

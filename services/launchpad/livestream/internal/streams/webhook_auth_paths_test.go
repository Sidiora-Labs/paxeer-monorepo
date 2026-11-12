package streams

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func TestNormalizedWebhookRejectsUnboundTransportAndPayload(t *testing.T) {
	secret := strings.Repeat("s", 32)
	now := time.Now().UTC().Truncate(time.Second)
	check := func(body []byte, media string, status int) {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/v2/provider-webhooks/livepeer", bytes.NewReader(body))
		stamp := fmt.Sprint(now.Unix())
		request.Header.Set(NormalizedWebhookTimestampHeader, stamp)
		request.Header.Set(NormalizedWebhookSignatureHeader, sharedauth.SignWebhook(normalizedWebhookKey(secret), stamp, string(body)))
		request.Header.Set("Content-Type", media)
		response := httptest.NewRecorder()
		if _, ok := VerifyNormalizedProviderWebhook(response, request, secret, now); ok || response.Code != status {
			t.Fatalf("normalized callback = %d %s", response.Code, response.Body.String())
		}
	}
	check([]byte(`{}`), "text/plain", http.StatusUnsupportedMediaType)
	check([]byte(`{}`), "application/json", http.StatusBadRequest)
	check([]byte(strings.Repeat("x", 16385)), "application/json", http.StatusRequestEntityTooLarge)
	request := httptest.NewRequest(http.MethodPost, "/v2/provider-webhooks/livepeer", nil)
	request.Body = io.NopCloser(unreadableBody{})
	response := httptest.NewRecorder()
	if _, ok := VerifyNormalizedProviderWebhook(response, request, secret, now); ok || response.Code != http.StatusBadRequest {
		t.Fatalf("unreadable normalized body = %d", response.Code)
	}
	response = httptest.NewRecorder()
	if _, ok := VerifyNormalizedProviderWebhook(response, httptest.NewRequest(http.MethodPost, "/v2/provider-webhooks/livepeer", nil), "short", now); ok || response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing normalized authority = %d", response.Code)
	}
	if _, err := AdaptV2LiveWebhook("request_1", time.Time{}); err != ErrInvalid {
		t.Fatalf("zero webhook time = %v", err)
	}
	if _, err := marshalNormalizedWebhook(ProviderEvent{EventID: "event_1", ProviderID: "provider_1", State: stateReady, ObservedAt: now}); err != ErrInvalid {
		t.Fatalf("unsupported normalized state = %v", err)
	}
	if _, err := marshalNormalizedWebhook(ProviderEvent{EventID: "event_1", ProviderID: "provider_1", State: stateLive, ObservedAt: now.Add(time.Nanosecond)}); err != ErrInvalid {
		t.Fatalf("submillisecond normalized time = %v", err)
	}
}

func TestProviderWebhookRejectsAmbiguousSignedTimestamp(t *testing.T) {
	secret := strings.Repeat("s", 32)
	now := time.Now().UTC().Truncate(time.Millisecond)
	stamp := now.UnixMilli()
	check := func(body []byte, header string, signingKey string, want int) {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/webhooks/livepeer", bytes.NewReader(body))
		request.Header.Set("Livepeer-Signature", header)
		response := httptest.NewRecorder()
		if _, ok := VerifyProviderWebhook(response, request, signingKey, now, true); ok || response.Code != want {
			t.Fatalf("provider signature decision = accepted:%t status:%d body:%s", ok, response.Code, response.Body.String())
		}
	}
	sign := func(body []byte, timestamp int64) string {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
	}
	valid := []byte(fmt.Sprintf(`{"timestamp":%d}`, stamp))
	check(valid, sign(valid, stamp), "short", http.StatusServiceUnavailable)
	check(valid, strings.Repeat("x", 1025), secret, http.StatusUnauthorized)
	check(valid, "malformed", secret, http.StatusUnauthorized)
	check([]byte(strings.Repeat("x", (1<<20)+1)), "t=1,v1=invalid", secret, http.StatusRequestEntityTooLarge)
	duplicate := []byte(fmt.Sprintf(`{"timestamp":%d,"timestamp":%d}`, stamp, stamp))
	check(duplicate, sign(duplicate, stamp), secret, http.StatusBadRequest)
	mismatch := []byte(fmt.Sprintf(`{"timestamp":%d}`, stamp+1))
	check(mismatch, sign(mismatch, stamp), secret, http.StatusUnauthorized)
	check(valid, sign(valid, stamp-301000), secret, http.StatusUnauthorized)
	invalidJSON := []byte(`[]`)
	check(invalidJSON, sign(invalidJSON, stamp), secret, http.StatusBadRequest)
}

func TestProviderWebhookAppliesSignedStateThroughLifecycle(t *testing.T) {
	f := repair57New(t)
	f.handlers.studioWebhook = false
	ctx := context.Background()
	o, _ := f.create("provider-webhook-application")
	o, err := f.store.BindProvider(ctx, o.ID, "webhook_provider")
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UTC().Truncate(time.Millisecond).UnixMilli()
	call := func(body []byte, want int) []byte {
		t.Helper()
		mac := hmac.New(sha256.New, []byte(f.secret))
		_, _ = mac.Write(body)
		return f.exchange(http.MethodPost, "/webhooks/livepeer", body, map[string]string{"Content-Type": "application/json", "Livepeer-Signature": fmt.Sprintf("t=%d,v1=%s", stamp, hex.EncodeToString(mac.Sum(nil)))}, want)
	}
	invalid := []byte(fmt.Sprintf(`{"id":"webhook_event","webhookId":"callback_1","createdAt":%d,"timestamp":%d,"event":"unknown","stream":{"id":%q}}`, stamp, stamp, o.ProviderID))
	call(invalid, http.StatusUnprocessableEntity)
	valid := []byte(fmt.Sprintf(`{"id":"webhook_event","webhookId":"callback_1","createdAt":%d,"timestamp":%d,"event":"stream.started","stream":{"id":%q}}`, stamp, stamp, o.ProviderID))
	if raw := call(valid, http.StatusOK); !bytes.Contains(raw, []byte(`"outcome":"applied"`)) {
		t.Fatalf("provider state application = %s", raw)
	}
	if raw := call(valid, http.StatusOK); !bytes.Contains(raw, []byte(`"outcome":"duplicate"`)) {
		t.Fatalf("provider event deduplication = %s", raw)
	}
	current, err := f.store.Get(ctx, o.ID)
	if err != nil || current.State != stateLive {
		t.Fatalf("provider state = %+v, %v", current, err)
	}
	f.handlers.decodeWebhook = nil
	call(valid, http.StatusServiceUnavailable)
}

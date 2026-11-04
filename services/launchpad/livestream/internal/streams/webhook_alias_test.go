package streams

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func TestPostgresProviderWebhookAliasVerifiesSignature(t *testing.T) {
	for _, milliseconds := range []bool{false, true} {
		t.Run(fmt.Sprintf("milliseconds=%t", milliseconds), func(t *testing.T) {
			s := fixture(t)
			ctx := context.Background()
			request, decision := intent("webhook_alias")
			o, err := s.Create(ctx, request, decision, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.BeginProvision(ctx, o.ID); err != nil {
				t.Fatal(err)
			}
			o, err = s.BindProvider(ctx, o.ID, "provider_alias")
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Truncate(time.Second)
			secret := strings.Repeat("k", 32)
			var lastOutcome string
			h := New(Deps{
				ObserveOutcome:      func(outcome string) { lastOutcome = outcome },
				Lifecycle:           s,
				WebhookSecret:       secret,
				WebhookMilliseconds: milliseconds,
				Clock:               func() time.Time { return now },
				DecodeWebhook: func(raw []byte) (ProviderEvent, error) {
					var event ProviderEvent
					err := json.Unmarshal(raw, &event)
					return event, err
				},
			})
			router := chi.NewRouter()
			h.RegisterRoutes(router)
			timestamp := func(at time.Time) int64 {
				if milliseconds {
					return at.UnixMilli()
				}
				return at.Unix()
			}
			body := func(at time.Time) string {
				return fmt.Sprintf(`{"EventID":"alias_event","ProviderID":"provider_alias","State":"LIVE","ObservedAt":%q,"timestamp":%d}`, now.Format(time.RFC3339Nano), timestamp(at))
			}
			signature := func(raw string, at time.Time) string {
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write([]byte(raw))
				return fmt.Sprintf("t=%d,v1=%s", timestamp(at), hex.EncodeToString(mac.Sum(nil)))
			}
			post := func(path, raw, header string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(raw))
				req.Header.Set("Content-Type", "application/json")
				if header != "" {
					if path == "/webhooks/livepeer" {
						req.Header.Set("Livepeer-Signature", header)
					} else {
						parts := strings.SplitN(header, ",v1=", 2)
						req.Header.Set(NormalizedWebhookTimestampHeader, strings.TrimPrefix(parts[0], "t="))
						req.Header.Set(NormalizedWebhookSignatureHeader, "sha256="+parts[1])
					}
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				return rec
			}
			paths := []string{"/webhooks/livepeer", "/v2/provider-webhooks/livepeer"}
			wireBody := func(path string, at time.Time) string {
				if path == "/webhooks/livepeer" {
					return body(at)
				}
				raw, err := marshalNormalizedWebhook(ProviderEvent{EventID: "alias_event", ProviderID: "provider_alias", State: "LIVE", ObservedAt: now})
				if err != nil {
					t.Fatal(err)
				}
				return string(raw)
			}
			wireTimestamp := func(path string, at time.Time) int64 {
				if path == "/webhooks/livepeer" {
					return timestamp(at)
				}
				return at.Unix()
			}
			wireSignature := func(path, raw string, at time.Time) string {
				if path == "/webhooks/livepeer" {
					return signature(raw, at)
				}
				ts := strconv.FormatInt(at.Unix(), 10)
				return "t=" + ts + ",v1=" + strings.TrimPrefix(sharedauth.SignWebhook(normalizedWebhookKey(secret), ts, raw), "sha256=")
			}
			for _, path := range paths {
				t.Run(path, func(t *testing.T) {
					raw := wireBody(path, now)
					tampered := strings.Replace(raw, `"LIVE"`, `"ENDED"`, 1)
					if path != "/webhooks/livepeer" {
						tampered = strings.Replace(raw, `"active"`, `"inactive"`, 1)
					}
					header := wireSignature(path, raw, now)
					_, mac, _ := strings.Cut(header, ",v1=")
					for _, tc := range []struct{ name, raw, header string }{
						{"unsigned", raw, ""},
						{"wrong_signature", raw, fmt.Sprintf("t=%d,v1=%s", wireTimestamp(path, now), strings.Repeat("0", 64))},
						{"tampered_body", tampered, header},
						{"substituted_timestamp", raw, fmt.Sprintf("t=%d,v1=%s", wireTimestamp(path, now.Add(time.Second)), mac)},
						{"expired", wireBody(path, now.Add(-301*time.Second)), wireSignature(path, wireBody(path, now.Add(-301*time.Second)), now.Add(-301*time.Second))},
						{"future", wireBody(path, now.Add(301*time.Second)), wireSignature(path, wireBody(path, now.Add(301*time.Second)), now.Add(301*time.Second))},
					} {
						t.Run(tc.name, func(t *testing.T) {
							requireEnvelope(t, tc.name, post(path, tc.raw, tc.header), http.StatusUnauthorized, "AUTH_EXPIRED")
						})
					}
				})
			}
			var count int
			if err = s.pool.QueryRow(ctx, "SELECT count(*) FROM "+s.schema+".stream_inbox").Scan(&count); err != nil || count != 0 {
				t.Fatalf("rejected events reached inbox: count=%d err=%v", count, err)
			}
			current, err := s.Get(ctx, o.ID)
			if err != nil || current.State != o.State || current.Revision != o.Revision {
				t.Fatalf("rejected events changed operation: %+v err=%v", current, err)
			}
			for i, path := range paths {
				raw := wireBody(path, now)
				rec := post(path, raw, wireSignature(path, raw, now))
				want := "applied"
				if i > 0 {
					want = "duplicate"
				}
				if lastOutcome != want {
					t.Fatalf("%s outcome = %q, want %q", path, lastOutcome, want)
				}
				var response struct {
					Outcome string `json:"outcome"`
				}
				if path == "/v2/provider-webhooks/livepeer" {
					if err = v2.ValidateJSON("LiveWebhookResponse", rec.Body.Bytes()); err != nil {
						t.Fatal(err, rec.Body.String())
					}
					var frozen v2.LiveWebhookResponse
					if err = json.Unmarshal(rec.Body.Bytes(), &frozen); err != nil || rec.Code != http.StatusOK || !frozen.Data.Accepted {
						t.Fatal(rec.Code, rec.Body.String(), err)
					}
					continue
				}
				if err = json.Unmarshal(rec.Body.Bytes(), &response); err != nil || rec.Code != http.StatusOK || response.Outcome != want {
					t.Fatalf("%s signed event: status=%d body=%s err=%v", path, rec.Code, rec.Body.String(), err)
				}
			}
			if err = s.pool.QueryRow(ctx, "SELECT count(*) FROM "+s.schema+".stream_inbox").Scan(&count); err != nil || count != 1 {
				t.Fatalf("signed event must be persisted once: count=%d err=%v", count, err)
			}
			current, err = s.Get(ctx, o.ID)
			if err != nil || current.State != "LIVE" || current.Revision != o.Revision+1 || current.ProviderTime == nil || !current.ProviderTime.Equal(now) {
				t.Fatalf("signed event must be applied once: %+v err=%v", current, err)
			}
		})
	}
}

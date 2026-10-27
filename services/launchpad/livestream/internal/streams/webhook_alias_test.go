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
	"strings"
	"testing"
	"time"

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
			h := New(Deps{
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
					req.Header.Set("Livepeer-Signature", header)
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				return rec
			}
			raw := body(now)
			paths := []string{"/webhooks/livepeer", "/v2/provider-webhooks/livepeer"}
			for _, path := range paths {
				t.Run(path, func(t *testing.T) {
					for _, tc := range []struct {
						name, raw, header string
					}{
						{"unsigned", raw, ""},
						{"wrong_signature", raw, fmt.Sprintf("t=%d,v1=%s", timestamp(now), strings.Repeat("0", 64))},
						{"tampered_body", strings.Replace(raw, `"LIVE"`, `"ENDED"`, 1), signature(raw, now)},
						{"substituted_timestamp", raw, signature(raw, now.Add(time.Second))},
						{"expired", body(now.Add(-301 * time.Second)), signature(body(now.Add(-301*time.Second)), now.Add(-301*time.Second))},
						{"future", body(now.Add(301 * time.Second)), signature(body(now.Add(301*time.Second)), now.Add(301*time.Second))},
					} {
						t.Run(tc.name, func(t *testing.T) {
							rec := post(path, tc.raw, tc.header)
							requireEnvelope(t, tc.name, rec, http.StatusUnauthorized, "AUTH_EXPIRED")
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
				rec := post(path, raw, signature(raw, now))
				want := "applied"
				if i > 0 {
					want = "duplicate"
				}
				var response struct {
					Outcome string `json:"outcome"`
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

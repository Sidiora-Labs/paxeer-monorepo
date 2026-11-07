package livepeer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRepairM_1003_ProviderFailureClassifications(t *testing.T) {
	TestProviderFailureRedactsSecretsAndBoundsBody(t)
}

func TestRepairM_1003_ResponseMediaType(t *testing.T) {
	TestProviderRejectsInvalidResponseMediaType(t)
}

func TestRepairM_1003_RedirectAuthorization(t *testing.T) {
	TestProviderRedirectDoesNotForwardAuthorization(t)
}

func TestRepairM_1003_Cancellation(t *testing.T) {
	TestProviderUnavailableAndCancellation(t)
}

func TestRepairM_1003_JSONBodyLimit(t *testing.T) {
	const limit = 1 << 20
	const prefix = `{"id":"id","playbackId":"playback","streamKey":"SECRET-stream-key","padding":"`
	const suffix = `"}`
	for _, size := range []int{limit, limit + 1} {
		name := "at_limit"
		if size > limit {
			name = "above_limit"
		}
		t.Run(name, func(t *testing.T) {
			body := prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix
			if len(body) != size || !json.Valid([]byte(body)) {
				t.Fatal("fixture must be valid JSON of the exact boundary size")
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/stream" || r.Header.Get("Authorization") != "Bearer fixture-key" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("unexpected stream provisioning request")
				}
				var request struct {
					Name     string          `json:"name"`
					Profiles []streamProfile `json:"profiles"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Name != "fixture-correlation" || len(request.Profiles) != 3 {
					t.Errorf("invalid production provisioning payload: %+v, %v", request, err)
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			c := New("fixture-key", server.URL, nil)
			stream, err := c.CreateStream(context.Background(), "fixture-correlation")
			if size > limit {
				if stream != nil || err == nil || err.Error() != "livepeer: response too large" {
					t.Fatalf("oversize response = (%v, %v), want sanitized size refusal", stream, err)
				}
				return
			}
			if err != nil || stream == nil {
				t.Fatalf("valid response at size limit rejected: %v", err)
			}
			if stream.ID != "id" || stream.PlaybackID != "playback" || stream.StreamKey != "SECRET-stream-key" {
				t.Fatalf("provider response was not decoded correctly: %+v", stream)
			}
		})
	}
}

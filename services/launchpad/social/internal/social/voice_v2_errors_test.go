package social

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func TestVoiceV2RoutesRejectBodiesOutsideFrozenSchema(t *testing.T) {
	const requestID = "8a06b58b-04bd-4877-991d-041886116ecb"
	const roomPath = "/v2/voice-rooms/231146cd-dcd9-46f3-968e-6a811e762f09"
	routes := []struct {
		schema string
		path   string
		body   string
	}{
		{"VoiceCreateBody", "/v2/voice-rooms", `{"scopeType":"pool","scopeId":"0x3333333333333333333333333333333333333333","title":"Room","visibility":"public"}`},
		{"VoiceEndBody", roomPath + ":end", `{"expectedRevision":"1"}`},
		{"VoiceLeaveBody", roomPath + ":leave", `{"clientSessionId":"671d33e1-d87c-4bed-b76d-bb09208d954c"}`},
		{"VoiceJoinTicketBody", roomPath + "/tickets", `{"role":"listener","clientSessionId":"671d33e1-d87c-4bed-b76d-bb09208d954c"}`},
		{"VoiceModerateBody", roomPath + "/moderation-actions", `{"participantAccountId":"671d33e1-d87c-4bed-b76d-bb09208d954c","action":"mute","reason":"Room policy","expectedRevision":"1"}`},
	}
	for _, dependency := range []struct {
		name    string
		control *voice.Control
	}{
		{"nil-control", nil},
		{"control-without-dependencies", &voice.Control{}},
	} {
		t.Run(dependency.name, func(t *testing.T) {
			router := chi.NewRouter()
			RegisterSafetyRoutes(router, nil, dependency.control)
			for _, route := range routes {
				t.Run(route.schema, func(t *testing.T) {
					valid := []byte(route.body)
					if err := v2.ValidateJSON(route.schema, valid); err != nil {
						t.Fatalf("valid request fixture: %v", err)
					}
					request := func(body []byte) *httptest.ResponseRecorder {
						t.Helper()
						req := httptest.NewRequest(http.MethodPost, route.path, bytes.NewReader(body))
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("X-Request-ID", requestID)
						rec := httptest.NewRecorder()
						router.ServeHTTP(rec, req)
						return rec
					}
					assertError := func(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
						t.Helper()
						if rec.Code != status {
							t.Fatalf("status = %d; want %d; body %s", rec.Code, status, rec.Body.String())
						}
						if err := v2.ValidateJSON("Error", rec.Body.Bytes()); err != nil {
							t.Fatalf("frozen Error: %v; body %s", err, rec.Body.String())
						}
						var got v2.Error
						if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
							t.Fatal(err)
						}
						if got.Code != code || got.Retryable != (status == http.StatusServiceUnavailable) {
							t.Fatalf("error = %+v; want code %s, retryable %t", got, code, status == http.StatusServiceUnavailable)
						}
						if got.RequestId != requestID || rec.Header().Get("X-Request-ID") != requestID {
							t.Fatalf("request ID = %q; headers %v", got.RequestId, rec.Header())
						}
						if rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("Cache-Control") != "private, no-store" {
							t.Fatalf("error headers = %v", rec.Header())
						}
					}
					t.Run("valid-body-reaches-control-availability-check", func(t *testing.T) {
						assertError(t, request(valid), http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
					})
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(valid, &fields); err != nil {
						t.Fatal(err)
					}
					var required []string
					for field := range fields {
						required = append(required, field)
					}
					sort.Strings(required)
					cases := []struct {
						name string
						body []byte
					}{}
					fields["extra"] = json.RawMessage(`true`)
					withExtra, err := json.Marshal(fields)
					if err != nil {
						t.Fatal(err)
					}
					cases = append(cases, struct {
						name string
						body []byte
					}{"extra-field", withExtra})
					delete(fields, "extra")
					for _, field := range required {
						value := fields[field]
						delete(fields, field)
						missing, err := json.Marshal(fields)
						if err != nil {
							t.Fatal(err)
						}
						cases = append(cases, struct {
							name string
							body []byte
						}{"missing-" + field, missing})
						fields[field] = value
					}
					for _, tc := range cases {
						t.Run(tc.name, func(t *testing.T) {
							if err := v2.ValidateJSON(route.schema, tc.body); err == nil {
								t.Fatalf("invalid request fixture accepted by %s: %s", route.schema, tc.body)
							}
							assertError(t, request(tc.body), http.StatusBadRequest, "INVALID_REQUEST")
						})
					}
				})
			}
		})
	}
}

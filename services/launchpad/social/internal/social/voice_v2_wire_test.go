package social

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestAdaptV2VoiceEnvelopesMatchFrozenSchema(t *testing.T) {
	requestID := "8a06b58b-04bd-4877-991d-041886116ecb"
	hostID := "671d33e1-d87c-4bed-b76d-bb09208d954c"
	at := time.Date(2026, 10, 26, 13, 0, 0, 123000000, time.UTC)
	room := voice.Room{
		ID: "231146cd-dcd9-46f3-968e-6a811e762f09", PoolID: "pool-1",
		HostWallet: "0x1111111111111111111111111111111111111111", State: "draft", Revision: 7,
		ProviderSession: "private-provider-session", StartKey: "private-start-key",
	}
	capabilities := v2.VoiceCapabilities{Listen: true, ProviderQualified: true}
	wireRoom := v2.VoiceRoom{RoomId: room.ID, HostAccountId: hostID, ScopeType: "pool", ScopeId: room.PoolID, Visibility: "private", State: "scheduled", Revision: "7"}
	ended := room
	ended.State = "ended"
	endedRoom := wireRoom
	endedRoom.State = "ended"
	cases := []struct {
		name     string
		adapt    func() ([]byte, error)
		data     any
		revision any
	}{
		{"VoiceCreateResponse", func() ([]byte, error) {
			return AdaptV2VoiceCreate(requestID, room, hostID, "private", capabilities, at)
		}, v2.VoiceCreateData{Room: wireRoom, Capabilities: capabilities}, "7"},
		{"VoiceReadResponse", func() ([]byte, error) {
			return AdaptV2VoiceRead(requestID, room, hostID, "private", []string{"listen"}, at)
		}, v2.VoiceReadData{Room: wireRoom, AllowedActions: []string{"listen"}}, "7"},
		{"VoiceReadResponse/empty-actions", func() ([]byte, error) { return AdaptV2VoiceRead(requestID, room, hostID, "private", nil, at) }, v2.VoiceReadData{Room: wireRoom, AllowedActions: []string{}}, "7"},
		{"VoiceEndResponse", func() ([]byte, error) { return AdaptV2VoiceEnd(requestID, ended, hostID, "private", at) }, v2.VoiceEndData{Room: endedRoom}, "7"},
		{"VoiceLeaveResponse", func() ([]byte, error) { return AdaptV2VoiceLeave(requestID, "revoked", 9, at) }, v2.VoiceLeaveData{GrantState: "revoked", Revision: "9"}, "9"},
		{"VoiceLeaveResponse/revoking", func() ([]byte, error) { return AdaptV2VoiceLeave(requestID, "revoking", 0, at) }, v2.VoiceLeaveData{GrantState: "revoking", Revision: "0"}, nil},
		{"VoiceJoinTicketResponse", func() ([]byte, error) {
			return AdaptV2VoiceJoinTicket(requestID, "provider-issued-ticket", "https://voice.example.com", at.Add(time.Minute), at)
		}, v2.VoiceJoinTicketData{Ticket: "provider-issued-ticket", ProviderEndpoint: "https://voice.example.com", ExpiresAt: "2026-10-26T13:01:00.123Z"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := tc.adapt()
			if err != nil {
				t.Fatalf("adapt: %v", err)
			}
			if err := v2.ValidateJSON(strings.Split(tc.name, "/")[0], raw); err != nil {
				t.Fatalf("frozen schema: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			wantRaw, err := json.Marshal(map[string]any{
				"requestId": requestID, "data": tc.data, "revision": tc.revision,
				"watermark": nil, "nextCursor": nil, "quality": "complete", "generatedAt": "2026-10-26T13:00:00.123Z",
			})
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal(wantRaw, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("envelope = %s; want %s", raw, wantRaw)
			}
		})
	}
	for _, state := range []string{"draft", "starting", "live", "ending", "ended", "failed"} {
		t.Run("state/"+state, func(t *testing.T) {
			value := room
			value.State = state
			raw, err := AdaptV2VoiceRead(requestID, value, hostID, "public", nil, at)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := v2.DecodeVoiceReadResponse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Data.Room.State != value.APIState() {
				t.Fatalf("state = %q", decoded.Data.Room.State)
			}
		})
	}
	t.Run("invalid-inputs", func(t *testing.T) {
		invalid := room
		invalid.Revision = -1
		checks := []func() ([]byte, error){
			func() ([]byte, error) {
				return AdaptV2VoiceCreate(requestID, room, room.HostWallet, "private", capabilities, at)
			},
			func() ([]byte, error) { return AdaptV2VoiceRead("bad-request-id", room, hostID, "private", nil, at) },
			func() ([]byte, error) { return AdaptV2VoiceEnd(requestID, invalid, hostID, "private", at) },
			func() ([]byte, error) { return AdaptV2VoiceLeave(requestID, "active", 1, at) },
			func() ([]byte, error) {
				return AdaptV2VoiceJoinTicket(requestID, "ticket", "wss://voice.example.com", at.Add(time.Minute), at)
			},
			func() ([]byte, error) {
				return AdaptV2VoiceJoinTicket(requestID, strings.Repeat("t", 4097), "https://voice.example.com", at.Add(time.Minute), at)
			},
		}
		for i, check := range checks {
			if raw, err := check(); err == nil || raw != nil {
				t.Errorf("invalid input %d returned bytes %s, error %v", i, raw, err)
			}
		}
	})
}

func TestWriteV2VoiceErrorUsesFrozenEnvelope(t *testing.T) {
	requestID := "8a06b58b-04bd-4877-991d-041886116ecb"
	assertError := func(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		if rec.Code != status {
			t.Fatalf("status = %d; want %d", rec.Code, status)
		}
		if rec.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("content type = %q", rec.Header().Get("Content-Type"))
		}
		if err := v2.ValidateJSON("Error", rec.Body.Bytes()); err != nil {
			t.Fatalf("frozen Error: %v; body %s", err, rec.Body.String())
		}
		var got v2.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Code != code || got.Retryable != (status == 503) {
			t.Fatalf("error = %+v", got)
		}
		if got.RetryAfterMs != nil || got.Details != nil || got.FieldErrors == nil || len(got.FieldErrors) != 0 {
			t.Fatalf("nullable fields = %+v", got)
		}
		if _, err := uuid.Parse(got.RequestId); err != nil {
			t.Fatal(err)
		}
		if rec.Header().Get("X-Request-ID") != got.RequestId {
			t.Fatal("response request ID differs from envelope")
		}
		if strings.Contains(got.Message, "private") {
			t.Fatal("error exposed internal detail")
		}
	}
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{safety.ErrInvalid, 400, "INVALID_REQUEST"},
		{safety.ErrUnavailable, 503, "DEPENDENCY_UNAVAILABLE"},
		{safety.ErrConflict, 409, "CONFLICT"},
		{safety.ErrForbidden, 403, "SCOPE_DENIED"},
		{pgx.ErrNoRows, 404, "NOT_FOUND"},
		{voice.ErrUnavailable, 503, "DEPENDENCY_UNAVAILABLE"},
	} {
		t.Run(tc.code+"/"+tc.err.Error(), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v2/voice-rooms", nil)
			req.Header.Set("X-Request-ID", requestID)
			rec := httptest.NewRecorder()
			writeV2VoiceError(rec, req, fmt.Errorf("private detail: %w", tc.err))
			assertError(t, rec, tc.status, tc.code)
			var got v2.Error
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.RequestId != requestID {
				t.Fatalf("request ID = %q", got.RequestId)
			}
		})
	}
	for _, id := range []string{"", "untrusted/non-uuid"} {
		t.Run("request-id/"+id, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v2/voice-rooms", nil)
			req.Header.Set("X-Request-ID", id)
			rec := httptest.NewRecorder()
			writeV2VoiceError(rec, req, safety.ErrInvalid)
			assertError(t, rec, 400, "INVALID_REQUEST")
		})
	}
	t.Run("decode", func(t *testing.T) {
		valid := `{"scopeType":"pool","scopeId":"pool-1","title":"Room","visibility":"public"}`
		for _, body := range []string{valid, "", "null", "{}", valid + " {}", valid + "x", strings.Replace(valid, `"public"`, `"unknown"`, 1), strings.Replace(valid, `"title":"Room"`, `"title":"Room","extra":true`, 1), strings.Repeat(" ", 16384) + valid} {
			req := httptest.NewRequest(http.MethodPost, "/v2/voice-rooms", strings.NewReader(body))
			req.Header.Set("X-Request-ID", requestID)
			rec := httptest.NewRecorder()
			var got v2.VoiceCreateBody
			ok := decodeV2Voice(rec, req, "VoiceCreateBody", &got)
			if body == valid {
				if !ok || rec.Body.Len() != 0 || got.ScopeId != "pool-1" || got.Title != "Room" {
					t.Fatalf("valid decode = %+v, ok %v, response %s", got, ok, rec.Body.String())
				}
			} else {
				if ok {
					t.Fatalf("invalid body accepted: %.100s", body)
				}
				assertError(t, rec, 400, "INVALID_REQUEST")
			}
		}
	})
}

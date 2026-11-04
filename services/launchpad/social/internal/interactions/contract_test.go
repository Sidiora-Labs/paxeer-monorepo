package interactions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestPublicContractReadRecognisesOnlyViewerAwareReads(t *testing.T) {
	wallet := "0x" + strings.Repeat("ab", 20)
	for path, want := range map[string]bool{
		"/v2/profiles/" + wallet + "/followers":                        true,
		"/v2/profiles/" + wallet + "/follow-stats":                     true,
		"/v2/profiles/0x" + strings.ToUpper(wallet[2:]) + "/followers": true,
		"/v2/profiles/" + wallet:                                       false,
		"/v2/profiles/" + wallet + "/following":                        false,
		"/v2/profiles/not-a-wallet/followers":                          false,
		"/v2/social/users/" + wallet + "/followers":                    false,
		"/v2/me/following/" + wallet:                                   false,
	} {
		if got := PublicContractRead(httptest.NewRequest(http.MethodGet, path, nil)); got != want {
			t.Fatalf("%s: public=%v want %v", path, got, want)
		}
	}
	if PublicContractRead(httptest.NewRequest(http.MethodDelete, "/v2/profiles/"+wallet+"/followers", nil)) {
		t.Fatal("non-GET admitted as a public read")
	}
}

func TestContractKeyPrefersClientKeyThenRequestID(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/v2/me/following/0x"+strings.Repeat("ab", 20), nil)
	first := contractKey(r)
	second := contractKey(r)
	if !strings.HasPrefix(first, "request:") || first == second {
		t.Fatal("anonymous retries must not collapse into one mutation", first, second)
	}
	id := uuid.NewString()
	r.Header.Set("X-Request-ID", id)
	if contractKey(r) != "request:"+id || contractKey(r) != "request:"+id {
		t.Fatal("request id not reused as the idempotency key")
	}
	r.Header.Set("Idempotency-Key", "client-key")
	if contractKey(r) != "client-key" {
		t.Fatal("client idempotency key ignored")
	}
	r.Header.Set("Idempotency-Key", "bad\nkey")
	if contractKey(r) != "request:"+id {
		t.Fatal("malformed client key not rejected")
	}
}

func TestContractProfileProjection(t *testing.T) {
	wallet := "0x" + strings.Repeat("cd", 20)
	stored := []byte(`{"wallet":"` + wallet + `","displayName":"Ada","bio":"builder","avatarAssetId":"a7a3f3a0-7c2b-4c8e-9a1d-0c5f7f8d9e10","bannerAssetId":null,"website":"https://ada.example","revision":"4","socials":{"twitter":"https://x.com/ada"},"schemaVersion":1,"updatedAt":"2026-10-10T00:00:00Z"}`)
	profile, e := contractProfile(wallet, stored, false, "4")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(profile)
	if e != nil {
		t.Fatal(e)
	}
	var wire map[string]json.RawMessage
	if e = json.Unmarshal(raw, &wire); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"wallet", "displayName", "bio", "avatarAssetId", "bannerAssetId", "website", "revision"} {
		if _, ok := wire[key]; !ok {
			t.Fatal("frozen profile field missing", key)
		}
	}
	for _, key := range []string{"createdAt", "socials", "schemaVersion", "updatedAt"} {
		if _, ok := wire[key]; ok {
			t.Fatal("non-contract or unknown-value field leaked", key)
		}
	}
	if profile.DisplayName != "Ada" || profile.Website == nil || *profile.Website != "https://ada.example" || profile.Revision != "4" || profile.AvatarAssetId == nil || profile.BannerAssetId != nil {
		t.Fatal(profile)
	}
	deleted, e := contractProfile(wallet, []byte(`{"displayName":"gone"}`), true, "9")
	if e != nil || deleted.DisplayName != "" || deleted.Wallet != wallet || deleted.Revision != "9" {
		t.Fatal("deleted account must expose only wallet and revision", deleted, e)
	}
	provisioned, e := contractProfile(wallet, []byte(`{}`), false, "0")
	if e != nil || provisioned.Wallet != wallet || provisioned.Revision != "0" {
		t.Fatal(provisioned, e)
	}
	if _, e = contractProfile(wallet, []byte(`not json`), false, "1"); e == nil {
		t.Fatal("corrupt stored profile accepted")
	}
}

func TestContractErrorMapping(t *testing.T) {
	testContractErrorMapping(t, repair67SDK(t))
}

func testContractErrorMapping(t *testing.T, sdk string) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid-wallet", ErrInvalid, 422, "FIELD_INVALID"},
		{"self-follow", ErrInvalid, 422, "FIELD_INVALID"},
		{"denied", ErrDenied, 403, "SCOPE_DENIED"},
		{"missing", pgx.ErrNoRows, 404, "NOT_FOUND"},
		{"dependency", ErrDependency, 503, "DEPENDENCY_UNAVAILABLE"},
		{"revision", ErrRevision, 409, "REVISION_CONFLICT"},
		{"depth", ErrDepth, 422, "FIELD_INVALID"},
		{"reaction-kind", ErrReactionKind, 422, "FIELD_INVALID"},
		{"unknown", errors.New("private database connection information"), 503, "DEPENDENCY_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/v2/me/following/invalid", nil)
			id := uuid.NewString()
			request.Header.Set("X-Request-ID", id)
			response := httptest.NewRecorder()
			contractError(response, request, fmt.Errorf("wrapped internal error: %w", tc.err))
			if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
				t.Fatal(err, response.Body.String())
			}
			var value v2.Error
			if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			if response.Code != tc.status || value.Code != tc.code || value.RequestId != id || value.Retryable != (tc.status == 503) || value.FieldErrors == nil || len(value.FieldErrors) != 0 || value.Details != nil || value.RetryAfterMs != nil || value.Message == "" {
				t.Fatalf("status=%d error=%+v", response.Code, value)
			}
			if strings.Contains(response.Body.String(), "private database") || strings.Contains(response.Body.String(), "wrapped internal") {
				t.Fatal("internal error leaked", response.Body.String())
			}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contractHeaders(w)
				contractError(w, r, fmt.Errorf("wrapped internal error: %w", tc.err))
			})
			repair67AssertSDK(t, sdk, handler, http.MethodPut, "/v2/me/following/invalid", tc.status, tc.code, value.Message)
		})
	}
}

func TestContractErrorRequestIdentity(t *testing.T) {
	for _, raw := range []string{"", "invalid", uuid.Nil.String(), "A0B1C2D3-1234-4ABC-8DEF-0123456789AB"} {
		request := httptest.NewRequest(http.MethodGet, "/v2/profiles/invalid/followers", nil)
		request.Header.Set("X-Request-ID", raw)
		response := httptest.NewRecorder()
		contractError(response, request, ErrDependency)
		if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
		var value v2.Error
		if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		id, err := uuid.Parse(value.RequestId)
		if err != nil || id == uuid.Nil || response.Header().Get("X-Request-ID") != value.RequestId {
			t.Fatal("missing response identity", value)
		}
		if expected, err := uuid.Parse(raw); err == nil && expected != uuid.Nil && id != expected {
			t.Fatal("request identity replaced", raw, id)
		}
	}
}

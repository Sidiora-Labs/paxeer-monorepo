package streams

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestConnectedRoutesRejectInvalidRequestsAtTheirBoundaries(t *testing.T) {
	f := repair57New(t)
	if _, err := f.database.Exec(t.Context(), f.store.RotationSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	o, _ := f.create("route-boundaries")
	viewers, err := NewViewerCounter(f.redis, "livestream", 100000)
	if err != nil {
		t.Fatal(err)
	}
	f.handlers.viewers = viewers
	f.handlers.viewerTokens = &ViewerTokenAuthority{Key: []byte(strings.Repeat("v", 32))}
	base := "/v2/streams/" + o.ID
	call := func(t *testing.T, method, path, body string, want int) {
		t.Helper()
		request, err := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != want || !bytes.Contains(raw, []byte(`"code"`)) {
			t.Fatalf("%s %s = %d %s, %v", method, path, response.StatusCode, raw, err)
		}
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/v2/streams", `{"poolAddress":"bad","title":"title"}`, http.StatusBadRequest},
		{http.MethodPost, base + "/end", `{"revision":"0"}`, http.StatusUnprocessableEntity},
		{http.MethodPost, base + "/rotate-key", `{"revision":"invalid"}`, http.StatusUnprocessableEntity},
		{http.MethodPost, base + "/restore", `{"revision":"invalid"}`, http.StatusUnprocessableEntity},
		{http.MethodPost, base + "/go-live", `{"revision":"1"}`, http.StatusForbidden},
		{http.MethodPatch, base, `{"title":"changed","revision":"1"}`, http.StatusForbidden},
		{http.MethodPost, base + "/ingest-key", `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPost, base + "/ingest-key", `{}`, http.StatusForbidden},
		{http.MethodPost, base + "/viewer-session", `{}`, http.StatusConflict},
		{http.MethodPost, base + "/heartbeat", `{"sessionId":"bad","token":"bad"}`, http.StatusUnauthorized},
		{http.MethodPost, base + "/viewers:heartbeat", `{"playbackSessionId":"bad","viewerToken":"bad"}`, http.StatusBadRequest},
		{http.MethodPost, base + ":start", `{"expectedRevision":"invalid"}`, http.StatusBadRequest},
		{http.MethodPost, base + ":end", `{"expectedRevision":"invalid"}`, http.StatusBadRequest},
	} {
		t.Run(tc.method+" "+tc.path+" "+tc.body, func(t *testing.T) {
			call(t, tc.method, tc.path, tc.body, tc.status)
		})
	}
	for _, path := range []string{
		"/v2/streams/live?sort=unsupported",
		"/v2/streams/pool/bad",
	} {
		call(t, http.MethodGet, path, "", http.StatusUnprocessableEntity)
	}
	call(t, http.MethodGet, base+"/key-rotation", "", http.StatusForbidden)
	request, err := http.NewRequest(http.MethodGet, f.server.URL+base+"/key-rotation", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+f.session(f.key))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("rotation before request = %d", response.StatusCode)
	}
}

func TestConnectedRoutesReportMissingStreamIdentity(t *testing.T) {
	f := repair57New(t)
	const missing = "11111111-1111-4111-8111-111111111111"
	base := "/v2/streams/" + missing
	viewers, err := NewViewerCounter(f.redis, "livestream", 100000)
	if err != nil {
		t.Fatal(err)
	}
	f.handlers.viewers = viewers
	f.handlers.viewerTokens = &ViewerTokenAuthority{Key: []byte(strings.Repeat("v", 32))}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodGet, base + "/key-rotation", ""},
		{http.MethodGet, "/v2/streams/operations/" + missing, ""},
		{http.MethodPost, base + "/go-live", `{"revision":"1"}`},
		{http.MethodPost, base + "/end", `{"revision":"1"}`},
		{http.MethodPost, base + "/rotate-key", `{"revision":"1"}`},
		{http.MethodPost, base + "/restore", `{"revision":"1"}`},
		{http.MethodPatch, base, `{"title":"Updated","revision":"1"}`},
		{http.MethodPost, base + "/ingest-key", `{}`},
		{http.MethodPost, base + "/viewer-session", `{}`},
	} {
		request, err := http.NewRequest(tc.method, f.server.URL+tc.path, strings.NewReader(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err != nil || response.StatusCode != http.StatusNotFound || !bytes.Contains(body, []byte(`"code":"NOT_FOUND"`)) {
			t.Fatalf("missing stream response = %d %s, %v", response.StatusCode, body, err)
		}
	}
}

func TestEmptyRuntimeRoutesReportUnavailableDependencies(t *testing.T) {
	router := chi.NewRouter()
	New(Deps{}).RegisterRoutes(router)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/v2/streams", `{"poolAddress":"0x2222222222222222222222222222222222222222","title":"title"}`},
		{http.MethodPatch, "/v2/streams/11111111-1111-4111-8111-111111111111", `{"title":"title","revision":"1"}`},
		{http.MethodPost, "/v2/streams/11111111-1111-4111-8111-111111111111/go-live", `{"revision":"1"}`},
		{http.MethodPost, "/v2/streams/11111111-1111-4111-8111-111111111111/end", `{"revision":"1"}`},
		{http.MethodPost, "/v2/streams/11111111-1111-4111-8111-111111111111/rotate-key", `{"revision":"1"}`},
		{http.MethodPost, "/v2/streams/11111111-1111-4111-8111-111111111111/restore", `{"revision":"1"}`},
		{http.MethodPost, "/v2/streams/11111111-1111-4111-8111-111111111111/ingest-key", `{}`},
		{http.MethodGet, "/v2/streams/11111111-1111-4111-8111-111111111111/key-rotation", ""},
		{http.MethodPost, "/v2/streams/11111111-1111-4111-8111-111111111111/viewer-session", `{}`},
		{http.MethodPost, "/v2/streams/11111111-1111-4111-8111-111111111111/heartbeat", `{}`},
		{http.MethodGet, "/v2/streams/11111111-1111-4111-8111-111111111111/private-playback", ""},
		{http.MethodGet, "/v2/streams/11111111-1111-4111-8111-111111111111", ""},
		{http.MethodGet, "/v2/streams/operations/11111111-1111-4111-8111-111111111111", ""},
		{http.MethodGet, "/v2/streams/live", ""},
		{http.MethodGet, "/v2/streams/pool/0x2222222222222222222222222222222222222222", ""},
		{http.MethodPost, "/webhooks/livepeer", `{}`},
		{http.MethodPost, "/webhooks/livepeer/playback-access", `{}`},
	} {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusServiceUnavailable || !bytes.Contains(response.Body.Bytes(), []byte(`"code"`)) {
			t.Fatalf("%s %s = %d %s", tc.method, tc.path, response.Code, response.Body.String())
		}
	}
}

type unreadableBody struct{}

func (unreadableBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestBoundedJSONBodyRejectsAmbiguousRepresentations(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, encoding, body string
		limit, status                     int
		unreadable                        bool
	}{
		{"media", "text/plain", "", `{}`, 16, http.StatusUnsupportedMediaType, false},
		{"encoding", "application/json", "gzip", `{}`, 16, http.StatusUnsupportedMediaType, false},
		{"size", "application/json", "", `{"value":"too long"}`, 4, http.StatusRequestEntityTooLarge, false},
		{"syntax", "application/json", "", `{`, 16, http.StatusBadRequest, false},
		{"unknown", "application/json", "", `{"other":"value"}`, 32, http.StatusBadRequest, false},
		{"trailing", "application/json", "", `{} {}`, 16, http.StatusBadRequest, false},
		{"read", "application/json", "", ``, 16, http.StatusBadRequest, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v2/streams", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.contentType)
			if tc.encoding != "" {
				request.Header.Set("Content-Encoding", tc.encoding)
			}
			if tc.unreadable {
				request.Body = io.NopCloser(unreadableBody{})
			}
			var document struct {
				Value string `json:"value"`
			}
			response := httptest.NewRecorder()
			if _, ok := boundedBody(response, request, tc.limit, &document); ok || response.Code != tc.status || !bytes.Contains(response.Body.Bytes(), []byte(`"code"`)) {
				t.Fatalf("bounded request = %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestLegacyRequestHelpersPreserveCanonicalValues(t *testing.T) {
	body := []byte("stream body")
	secret := "provider secret"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	if !validHMAC(secret, body, signature) || validHMAC(secret, []byte("changed"), signature) || validHMAC(secret, body, "invalid") {
		t.Fatal("provider signature binding differs from the request body")
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.1:9000"
	if got := clientIP(request); got != "192.0.2.1" {
		t.Fatalf("client address = %s", got)
	}
	request.RemoteAddr = "local-request"
	if got := clientIP(request); got != "local-request" {
		t.Fatalf("unparsed client address = %s", got)
	}
	var logs bytes.Buffer
	h := New(Deps{Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	h.logErr("read", context.Canceled)
	if !strings.Contains(logs.String(), "read") || !strings.Contains(logs.String(), "context canceled") {
		t.Fatalf("handler log = %s", logs.String())
	}
}

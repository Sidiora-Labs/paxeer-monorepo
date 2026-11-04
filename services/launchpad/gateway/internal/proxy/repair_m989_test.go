package proxy

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
)

const repairM989Session = "Bearer repair-m989-session"

type repairM989Request struct {
	method, path string
	header       http.Header
	body         []byte
	principal    sharedauth.Principal
}

type repairM989Gateway struct {
	public        *httptest.Server
	principal     sharedauth.IdentityPrincipal
	received      chan repairM989Request
	identityCalls atomic.Int64
}

// The loopback peers supply identity and media fixtures; Connected and both
// shared mTLS assertion verifiers are production implementations. This fixture
// does not stand in for the user authority's durable lifecycle (see repair_m989.kvx).
func repairM989Mount(t *testing.T, name string, peer http.Handler) *repairM989Gateway {
	t.Helper()
	f := &repairM989Gateway{
		principal: sharedauth.IdentityPrincipal{
			AccountID: uuid.NewString(), SessionID: uuid.NewString(),
			Wallet:    "0x1111111111111111111111111111111111111111",
			AuthEpoch: 1, ProofAt: time.Now(), AccessExpiresAt: time.Now().Add(time.Hour),
		},
		received: make(chan repairM989Request, 64),
	}
	pki := testpki.New(t)
	gateway := pki.Leaf("gateway", "gateway.internal")
	server := pki.Leaf(name, name+".internal")
	key := testpki.Ed25519(t)
	identityTransport := sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}
	identityVerifier := sharedauth.ServiceRequestVerifier{
		Issuer: "gateway", Audience: "identity", PeerCertSHA256: gateway.SHA256,
		Key: key.Private.Public().(ed25519.PublicKey), Transport: identityTransport,
	}
	audience := name
	if name == "user" {
		audience = "account"
	}
	actorVerifier := sharedauth.ServiceAssertionVerifier{
		Issuer: "gateway", Audience: audience, PeerCertSHA256: gateway.SHA256,
		Key: key.Private.Public().(ed25519.PublicKey), Transport: actorAssertionTransport(name),
	}
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read private request: %v", err)
			http.Error(w, "read failed", 400)
			return
		}
		if r.URL.Path == "/internal/v2/identity/request:authenticate" {
			f.identityCalls.Add(1)
			if err := identityVerifier.Authenticate(r, body); err != nil {
				t.Errorf("identity service assertion: %v", err)
				http.Error(w, "denied", 401)
				return
			}
			var request sharedauth.IdentityAuthenticationRequest
			if json.Unmarshal(body, &request) != nil || request.Authorization != repairM989Session {
				http.Error(w, "session denied", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(f.principal); err != nil {
				t.Errorf("identity response: %v", err)
			}
			return
		}
		principal, err := actorVerifier.Authenticate(r, body)
		if err != nil {
			t.Errorf("media actor assertion: %v", err)
			http.Error(w, "denied", 401)
			return
		}
		f.received <- repairM989Request{r.Method, r.URL.RequestURI(), r.Header.Clone(), body, principal}
		r.Body = io.NopCloser(bytes.NewReader(body))
		peer.ServeHTTP(w, r)
	}))
	upstream.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{server.Pair}, ClientCAs: pki.Roots(), ClientAuth: tls.RequireAndVerifyClientCert}
	upstream.StartTLS()
	t.Cleanup(upstream.Close)
	clientTLS := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), ServerName: name + ".internal", Certificates: []tls.Certificate{gateway.Pair}}
	transport := &http.Transport{TLSClientConfig: clientTLS.Clone()}
	t.Cleanup(transport.CloseIdleConnections)
	identity, err := sharedauth.NewMTLSIdentityAuthority(upstream.URL, server.SHA256, &http.Client{Transport: transport}, sharedauth.ServiceRequestIssuer{
		Issuer: "gateway", Audience: "identity", Key: key.Private, Transport: identityTransport,
	})
	if err != nil {
		t.Fatal(err)
	}
	routes := UserConnectedRoutes()
	if name == "social" {
		routes = []ConnectedRoute{{Method: http.MethodGet, Template: "/v2/voice-rooms/{roomId}/recording/object", Authenticate: true}}
	}
	connected, err := NewConnected(ConnectedDeps{Name: name, URL: upstream.URL, Routes: routes, Identity: identity, SigningKey: key.Private, TLS: clientTLS})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connected.client.CloseIdleConnections)
	mux := http.NewServeMux()
	mux.Handle("/", connected)
	f.public = httptest.NewServer(mux)
	t.Cleanup(f.public.Close)
	return f
}

func (f *repairM989Gateway) request(t *testing.T, method, path string, body []byte, header http.Header) (*http.Response, error) {
	t.Helper()
	r, err := http.NewRequest(method, f.public.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header = header.Clone()
	if r.Header == nil {
		r.Header = make(http.Header)
	}
	r.Header.Set("Authorization", repairM989Session)
	return f.public.Client().Do(r)
}

func repairM989Recording(t *testing.T) (string, int64, []byte) {
	t.Helper()
	ogg, err := os.ReadFile("testdata/repair_m989_silence.ogg")
	if err != nil {
		t.Fatal(err)
	}
	if len(ogg) == 0 || !bytes.HasPrefix(ogg, []byte("OggS")) {
		t.Fatal("missing Ogg recording fixture")
	}
	// Chained complete Ogg Opus streams keep the final page intact.
	block := bytes.Repeat(ogg, 1024)
	path := filepath.Join(t.TempDir(), "recording.ogg")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.New()
	var size int64
	for size <= 65<<20 {
		n, err := io.Copy(io.MultiWriter(file, digest), bytes.NewReader(block))
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		size += n
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path, size, digest.Sum(nil)
}

func TestRepairM_989_recording_complete(t *testing.T) {
	path, size, wantHash := repairM989Recording(t)
	for _, knownLength := range []bool{true, false} {
		t.Run("known_length="+strconv.FormatBool(knownLength), func(t *testing.T) {
			f := repairM989Mount(t, "social", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				file, err := os.Open(path)
				if err != nil {
					t.Error(err)
					http.Error(w, "unavailable", 503)
					return
				}
				defer file.Close()
				w.Header().Set("Content-Type", "audio/ogg")
				if knownLength {
					w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
				} else {
					w.(http.Flusher).Flush()
				}
				if n, err := io.Copy(w, file); err != nil || n != size {
					t.Errorf("producer copied %d/%d: %v", n, size, err)
				}
			}))
			resp, err := f.request(t, http.MethodGet, "/v2/voice-rooms/"+uuid.NewString()+"/recording/object", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			gotHash := sha256.New()
			n, err := io.Copy(gotHash, resp.Body)
			if err != nil || n != size || !bytes.Equal(gotHash.Sum(nil), wantHash) {
				t.Fatalf("recording copied %d/%d bytes, hash=%x want=%x, error=%v", n, size, gotHash.Sum(nil), wantHash, err)
			}
			if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "audio/ogg" || resp.Header.Get("Cache-Control") != "private, no-store" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("recording response: %s %v", resp.Status, resp.Header)
			}
			if knownLength && resp.ContentLength != size {
				t.Fatalf("lost producer length: %d, want %d", resp.ContentLength, size)
			}
			if f.identityCalls.Load() != 1 {
				t.Fatalf("identity calls = %d", f.identityCalls.Load())
			}
		})
	}
}

func TestRepairM_989_recording_transfer_failure(t *testing.T) {
	path, size, _ := repairM989Recording(t)
	for _, knownLength := range []bool{true, false} {
		t.Run("known_length="+strconv.FormatBool(knownLength), func(t *testing.T) {
			f := repairM989Mount(t, "social", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				file, err := os.Open(path)
				if err != nil {
					t.Error(err)
					http.Error(w, "unavailable", 503)
					return
				}
				defer file.Close()
				w.Header().Set("Content-Type", "audio/ogg")
				if knownLength {
					w.Header().Set("Content-Length", strconv.FormatInt(size+1024, 10))
				}
				w.(http.Flusher).Flush()
				if n, err := io.Copy(w, file); err != nil || n != size {
					t.Errorf("producer copied %d/%d: %v", n, size, err)
				}
				w.(http.Flusher).Flush()
				panic(http.ErrAbortHandler)
			}))
			resp, err := f.request(t, http.MethodGet, "/v2/voice-rooms/"+uuid.NewString()+"/recording/object", nil, nil)
			if err != nil {
				t.Fatalf("request failed before streaming: %v", err)
			}
			defer resp.Body.Close()
			n, err := io.Copy(io.Discard, resp.Body)
			if err == nil || !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("broken upstream appeared complete: bytes=%d, error=%v", n, err)
			}
			if n <= 64<<20 || n > size {
				t.Fatalf("failure proof did not cross old limit: copied %d, producer %d", n, size)
			}
			if knownLength && resp.ContentLength != size+1024 {
				t.Fatalf("lost incomplete producer length: %d", resp.ContentLength)
			}
		})
	}
}

func TestRepairM_989_recording_invalid_content(t *testing.T) {
	f := repairM989Mount(t, "social", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html>restricted</html>")
	}))
	resp, err := f.request(t, http.MethodGet, "/v2/voice-rooms/"+uuid.NewString()+"/recording/object", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusBadGateway || !bytes.Contains(body, []byte("upstream_response_invalid")) || bytes.Contains(body, []byte("restricted")) {
		t.Fatalf("invalid recording: %s %s, %v", resp.Status, body, err)
	}
}

func TestRepairM_989_owner_mounted_transport(t *testing.T) {
	f := repairM989Mount(t, "user", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	job := uuid.NewString()
	capability := strings.Repeat("ab", 32)
	for _, tc := range []struct{ path, capability, want string }{
		{"/v2/me/exports/" + job + "/download", capability, capability},
		{"/v2/me/exports/" + job + "/download", "Bearer " + capability, "Bearer " + capability},
		{"/v2/me/exports/" + job, capability, ""},
		{"/v2/me/watchlist", capability, ""},
	} {
		t.Run(tc.path+"/"+tc.capability[:6], func(t *testing.T) {
			requestID := uuid.NewString()
			header := http.Header{"X-Export-Authorization": {tc.capability}, "X-Request-Id": {requestID}, "Cookie": {"__Host-kindle_access=repair-cookie"}, "X-Actor-Wallet": {"0x2222222222222222222222222222222222222222"}}
			resp, err := f.request(t, http.MethodGet, tc.path, nil, header)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				t.Fatalf("owner transport: %s", resp.Status)
			}
			got := <-f.received
			if got.path != tc.path || got.method != http.MethodGet || got.header.Get("X-Export-Authorization") != tc.want || got.header.Get("Authorization") != repairM989Session || got.header.Get("Cookie") != "__Host-kindle_access=repair-cookie" || got.header.Get("X-Actor-Wallet") != "" {
				t.Fatalf("owner request changed: %+v", got)
			}
			if got.principal.Wallet != f.principal.Wallet || got.principal.SessionID != f.principal.SessionID || got.principal.TraceID != requestID || len(got.principal.Roles) != 1 || got.principal.Roles[0] != "owner" {
				t.Fatalf("owner assertion: %+v", got.principal)
			}
		})
	}
	for _, values := range [][]string{{"bad"}, {strings.ToUpper(capability)}, {capability, capability}} {
		resp, err := f.request(t, http.MethodGet, "/v2/me/exports/"+job+"/download", nil, http.Header{"X-Export-Authorization": values})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("malformed capability returned %s", resp.Status)
		}
		select {
		case got := <-f.received:
			t.Fatalf("malformed capability crossed private channel: %+v", got)
		default:
		}
	}
}

func TestRepairM_989_owner_recovery_refusals(t *testing.T) {
	f := repairM989Mount(t, "user", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid recovery request reached private authority")
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, version := range []string{"v1", "v2"} {
		for _, values := range [][]string{{"invalid"}, {"invalid", "invalid"}, {""}} {
			resp, err := f.request(t, http.MethodGet, "/"+version+"/me/deletion-requests/"+uuid.NewString(), nil, http.Header{"X-Recovery-Receipt": values})
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("invalid %s receipt returned %s", version, resp.Status)
			}
		}
	}
	if f.identityCalls.Load() != 0 {
		t.Fatalf("malformed recovery contacted session authority %d times", f.identityCalls.Load())
	}
}

func TestRepairM_989_owner_existing_assertions(t *testing.T) {
	t.Run("frozen_owner_routes", TestUserConnectedRoutesForwardFrozenV2Owner)
	t.Run("public_profile_reads", TestUserConnectedRoutesKeepV2ProfileReadPublic)
	t.Run("account_audience", TestUserConnectedOwnerRoutesSignAccountAudience)
}

package app

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestPrivateIdentityReadiness(t *testing.T) {
	pki := testpki.New(t)
	serverCertificate := pki.Leaf("gateway-server", "gateway.railway.internal")
	clientCertificate := pki.Leaf("metadata-client", "metadata.railway.internal")
	tracker := process.NewTracker()

	mux := chi.NewRouter()
	mux.Get("/internal/v2/identity/probe", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewUnstartedServer(privateIdentityHandler(mux, tracker))
	server.TLS = privateListenerTLS(pki.Roots(), serverCertificate.Pair)
	server.StartTLS()
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{clientCertificate.Pair}},
	}}
	t.Cleanup(func() { client.Transport.(*http.Transport).CloseIdleConnections() })

	assertStatus := func(name, method, path string, wantStatus int, wantBody map[string]string) {
		t.Helper()
		request, err := http.NewRequest(method, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("%s request: %v", name, err)
		}
		defer response.Body.Close()
		if response.StatusCode != wantStatus {
			t.Fatalf("%s status = %d, want %d", name, response.StatusCode, wantStatus)
		}
		if wantBody != nil {
			var body map[string]string
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("%s body: %v", name, err)
			}
			if len(body) != len(wantBody) {
				t.Fatalf("%s body fields = %d, want %d: %v", name, len(body), len(wantBody), body)
			}
			for key, want := range wantBody {
				if body[key] != want {
					t.Fatalf("%s body[%q] = %q, want %q", name, key, body[key], want)
				}
			}
		}
	}

	assertStatus("before ready", http.MethodGet, "/health/ready", http.StatusServiceUnavailable, map[string]string{"status": "degraded"})
	if err := tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	assertStatus("ready", http.MethodGet, "/health/ready", http.StatusOK, map[string]string{"status": "ok"})
	tracker.BeginDrain()
	assertStatus("after drain", http.MethodGet, "/health/ready", http.StatusServiceUnavailable, map[string]string{"status": "degraded"})

	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodPost, "/health/ready"},
		{http.MethodGet, "/health/live"},
		{http.MethodGet, "/health"},
		{http.MethodGet, "/metrics"},
	} {
		assertStatus(tc.method+" "+tc.path, tc.method, tc.path, http.StatusNotFound, nil)
	}

	request, err := http.NewRequest(http.MethodGet, server.URL+"/internal/v2/identity/probe", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("identity route status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	withoutCertificate := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots()},
	}}
	t.Cleanup(func() { withoutCertificate.Transport.(*http.Transport).CloseIdleConnections() })
	if _, err := withoutCertificate.Get(server.URL + "/health/ready"); err == nil {
		t.Fatal("private listener completed a request without a client certificate")
	}
}

package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/security/control"
)

func TestOperatorGatewayReportsControlRefusals(t *testing.T) {
	store, err := control.OpenLocalStore(filepath.Join(t.TempDir(), "objects.json"), control.NewLocalCredentialAuthority().Reader(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.CloseLocal(); err != nil {
			t.Error(err)
		}
	})
	service, err := control.NewHTTPService(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()
	g, err := gateway(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer g.client.CloseIdleConnections()
	for _, sample := range []struct{ method, path, want string }{
		{http.MethodPost, "actions", "control route status=401 code=AUTH_REQUIRED"},
		{http.MethodGet, "campaigns/absent", "control route status=401 code=AUTH_REQUIRED"},
		{http.MethodGet, "absent/route", "control route status=404 code=INVALID_RESPONSE"},
		{http.MethodDelete, "caps/quote", "control route status=405 code=INVALID_RESPONSE"},
	} {
		err := g.request(context.Background(), sample.method, sample.path, "", nil, nil)
		if err == nil || err.Error() != sample.want {
			t.Fatalf("control refusal %s: %v", sample.path, err)
		}
	}
	if err := g.request(context.Background(), http.MethodPost, "actions", "", make(chan int), nil); err == nil || err.Error() != "invalid control request" {
		t.Fatalf("unencodable control payload: %v", err)
	}
	if err := g.request(context.Background(), "invalid method", "actions", "", nil, nil); err == nil || err.Error() != "invalid control request" {
		t.Fatalf("invalid control method: %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.request(cancelled, http.MethodGet, "caps/quote", "", nil, nil); err == nil || err.Error() != "control route unavailable" {
		t.Fatalf("cancelled control request: %v", err)
	}
	var incompatible int
	if err := g.request(context.Background(), http.MethodGet, "caps/quote", "", nil, &incompatible); !errors.Is(err, errResponse) {
		t.Fatalf("incompatible control response: %v", err)
	}
	if _, _, err := g.cap(context.Background(), "absent/quote"); err == nil {
		t.Fatal("missing capability route admitted")
	}
	output, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	warning, _, err := g.cap(context.Background(), "quote")
	if err != nil {
		t.Fatal(err)
	}
	if err := printCap(output, warning); err == nil || err.Error() != "operator output unavailable" {
		t.Fatalf("closed operator output: %v", err)
	}
	if status := Run([]string{"show"}, []string{gatewaySetting + "=" + server.URL}, strings.NewReader(""), output); status != 1 {
		t.Fatalf("show status with closed output: %d", status)
	}
	var shown bytes.Buffer
	if status := Run([]string{"show"}, []string{gatewaySetting + "=" + server.URL}, strings.NewReader(""), &shown); status != 0 || strings.Count(shown.String(), "\n") != len(capabilities) {
		t.Fatalf("show available capabilities: %d %s", status, shown.String())
	}
}

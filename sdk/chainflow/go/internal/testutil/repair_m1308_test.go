package testutil

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRepairM_1308_ReachableServer(t *testing.T) {
	requests := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Method + " " + r.URL.RequestURI()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ready")
	}))
	t.Cleanup(server.Close)

	if !CheckTestServer(t, server.URL+"/ready?probe=1") {
		t.Fatal("reachable server was reported unavailable")
	}
	select {
	case got := <-requests:
		if got != "GET /ready?probe=1" {
			t.Fatalf("request = %q, want GET /ready?probe=1", got)
		}
	default:
		t.Fatal("probe did not reach the server")
	}
}

func TestRepairM_1308_ResponseBodyClosed(t *testing.T) {
	closed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(server.CloseClientConnections)

	if !CheckTestServer(t, server.URL) {
		t.Fatal("probe failed after receiving response headers")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("probe retained the open response body instead of closing it on return")
	}
}

func TestRepairM_1308_ConnectionRefused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.Close()

	err := probeTestServer(server.URL)
	var connectionError *net.OpError
	if !errors.As(err, &connectionError) || connectionError.Op != "dial" {
		t.Fatalf("probe error = %v, want a connection failure", err)
	}
	if !strings.Contains(err.Error(), server.URL) {
		t.Fatalf("probe error does not identify the server: %v", err)
	}
}

func TestRepairM_1308_InvalidURL(t *testing.T) {
	for _, target := range []string{"://invalid", "ftp://127.0.0.1/", ""} {
		t.Run(target, func(t *testing.T) {
			err := probeTestServer(target)
			var requestError *url.Error
			if !errors.As(err, &requestError) {
				t.Fatalf("probe error = %v, want a URL error", err)
			}
		})
	}
}

func TestRepairM_1308_DisconnectedServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack connection: %v", err)
			return
		}
		if err := connection.Close(); err != nil {
			t.Errorf("close connection: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	if err := probeTestServer(server.URL); !errors.Is(err, io.EOF) {
		t.Fatalf("probe error = %v, want EOF from disconnected server", err)
	}
}

func TestRepairM_1308_Timeout(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(server.CloseClientConnections)

	result := make(chan error, 1)
	go func() { result <- probeTestServer(server.URL) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("probe did not reach the loopback server")
	}
	select {
	case err := <-result:
		var timeoutError net.Error
		if !errors.As(err, &timeoutError) || !timeoutError.Timeout() {
			t.Fatalf("probe error = %v, want a timeout", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("probe exceeded its two-second client timeout")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("timed out probe did not release the server request")
	}
}

func TestRepairM_1308_RedirectError(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "/again", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	err := probeTestServer(server.URL)
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("probe error = %v, want redirect limit failure", err)
	}
	if got := requests.Load(); got != 10 {
		t.Fatalf("redirect requests = %d, want 10", got)
	}
}

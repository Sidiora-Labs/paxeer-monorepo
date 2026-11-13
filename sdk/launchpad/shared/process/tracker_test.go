package process

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTrackerDrainsActiveRequestsAndConnections(t *testing.T) {
	tracker := NewTracker()
	if err := tracker.Ready(context.Background()); err == nil {
		t.Fatal("new tracker is ready before startup")
	}
	if err := tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}

	served := 0
	handler := tracker.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served++
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusNoContent || served != 1 {
		t.Fatalf("ready request: status %d, served %d", response.Code, served)
	}

	release, err := tracker.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	local, peer := net.Pipe()
	defer peer.Close()
	closeConnection, err := tracker.TrackConnection(local)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.TrackConnection(local); err == nil {
		t.Fatal("duplicate connection was accepted")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	drained := make(chan error, 1)
	go func() { drained <- tracker.Drain(ctx) }()
	select {
	case err := <-drained:
		t.Fatalf("drain returned with an active request: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if err := tracker.Ready(context.Background()); err == nil {
		t.Fatal("draining tracker remained ready")
	}
	if _, err := tracker.Acquire(); err == nil {
		t.Fatal("draining tracker accepted new work")
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusServiceUnavailable || served != 1 {
		t.Fatalf("draining request: status %d, served %d", response.Code, served)
	}
	release()
	release()
	closeConnection()
	select {
	case err := <-drained:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("drain did not finish after active work was released")
	}
	if err := tracker.SetReady(); err == nil {
		t.Fatal("drained tracker returned to ready state")
	}
	if _, err := peer.Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("tracked connection stayed open: %v", err)
	}
}

func TestTrackerConnectionStatesFollowReadiness(t *testing.T) {
	tracker := NewTracker()
	local, peer := net.Pipe()
	defer peer.Close()
	tracker.ConnState(local, http.StateHijacked)
	if _, err := peer.Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("unready hijacked connection remained open: %v", err)
	}
	if err := tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	local, peer = net.Pipe()
	defer local.Close()
	defer peer.Close()
	tracker.ConnState(local, http.StateHijacked)
	if _, err := tracker.TrackConnection(local); err == nil {
		t.Fatal("hijacked connection was tracked twice")
	}
	tracker.ConnState(local, http.StateClosed)
	release, err := tracker.TrackConnection(local)
	if err != nil {
		t.Fatalf("closed connection retained its tracked slot: %v", err)
	}
	release()
	if err := tracker.Drain(context.Background()); err != nil {
		t.Fatalf("drained closed connection: %v", err)
	}
}

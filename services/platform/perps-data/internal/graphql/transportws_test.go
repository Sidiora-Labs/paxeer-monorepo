package graphql

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	graphqlgo "github.com/graph-gophers/graphql-go"
)

func TestTransportWSOrigin(t *testing.T) {
	for _, tc := range []struct {
		name    string
		origins []string
		origin  string
		status  int
	}{
		{name: "listed", origins: []string{"https://app.example"}, origin: "https://app.example", status: http.StatusSwitchingProtocols},
		{name: "outside", origins: []string{"https://app.example"}, origin: "https://other.example", status: http.StatusForbidden},
		{name: "without_header", origins: []string{"https://app.example"}, status: http.StatusSwitchingProtocols},
		{name: "empty_list", origin: "https://other.example", status: http.StatusSwitchingProtocols},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &TransportWS{Schema: &graphqlgo.Schema{}, Feed: &Feed{}, AllowedOrigins: tc.origins}
			ts := httptest.NewServer(transport)
			defer ts.Close()
			headers := http.Header{}
			if tc.origin != "" {
				headers.Set("Origin", tc.origin)
			}
			conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http"), headers)
			if tc.status == http.StatusSwitchingProtocols {
				if err != nil {
					t.Fatalf("dial: %v", err)
				}
				if resp.StatusCode != tc.status {
					t.Fatalf("status %d, want %d", resp.StatusCode, tc.status)
				}
				_ = conn.Close()
				return
			}
			if err == nil || resp == nil || resp.StatusCode != tc.status {
				t.Fatalf("dial error=%v response=%v, want status %d", err, resp, tc.status)
			}
		})
	}
}

func TestTransportWSCompleteReusedID(t *testing.T) {
	conns := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := transportUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		conns <- conn
	}))
	defer srv.Close()
	dialer := websocket.Dialer{Subprotocols: []string{Subprotocol}, HandshakeTimeout: 5 * time.Second}
	client, _, err := dialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := <-conns
	defer server.Close()

	s := &wsSession{conn: server, acked: true, subs: map[string]*wsSub{}}
	ctx := context.Background()
	register := func(id string) (context.Context, *wsSub) {
		subCtx, stop := context.WithCancel(ctx)
		sub := &wsSub{stop: stop}
		s.mu.Lock()
		s.subs[id] = sub
		s.mu.Unlock()
		return subCtx, sub
	}

	// forward picks between the cancelled context and the closed channel at
	// random, so repeat until the closed channel path has been taken.
	for i := 0; i < 64; i++ {
		aCtx, a := register("1")
		aResponses := make(chan any)
		s.wg.Add(1)
		aDone := make(chan struct{})
		go func() {
			s.forward(aCtx, "1", a, aResponses)
			close(aDone)
		}()

		if !s.handle(ctx, wsMessage{ID: "1", Type: "complete"}) {
			t.Fatal("complete of A closed the socket")
		}
		bCtx, b := register("1")
		close(aResponses)
		<-aDone

		s.mu.Lock()
		got := s.subs["1"]
		s.mu.Unlock()
		if got != b {
			t.Fatalf("iteration %d: B is no longer registered under id 1 after A ended", i)
		}
		if bCtx.Err() != nil {
			t.Fatalf("iteration %d: B was cancelled by A ending", i)
		}

		if err := s.write(wsMessage{ID: "marker", Type: "pong"}); err != nil {
			t.Fatal(err)
		}
		if err := client.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var frame wsMessage
		if err := client.ReadJSON(&frame); err != nil {
			t.Fatal(err)
		}
		if frame.ID != "marker" {
			t.Fatalf("iteration %d: stale %s frame for id %q after A ended", i, frame.Type, frame.ID)
		}

		if !s.handle(ctx, wsMessage{ID: "1", Type: "complete"}) {
			t.Fatal("complete of B closed the socket")
		}
		if bCtx.Err() == nil {
			t.Fatalf("iteration %d: complete did not cancel B", i)
		}
		s.mu.Lock()
		_, still := s.subs["1"]
		s.mu.Unlock()
		if still {
			t.Fatalf("iteration %d: B still registered after complete", i)
		}
	}
	s.wg.Wait()
}

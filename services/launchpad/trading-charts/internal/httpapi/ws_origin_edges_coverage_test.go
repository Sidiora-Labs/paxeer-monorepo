package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestSocketExplicitOriginAdmissionAndClientAddress(t *testing.T) {
	hub := &wsHub{clients: map[*clientSub]struct{}{}, subscriptionIndex: map[string]map[*clientSub]struct{}{}, wildcardSubs: map[*clientSub]struct{}{}, ipCounts: map[string]int{}, maxConns: 2, maxPerIP: 2}
	server := httptest.NewServer(http.HandlerFunc(hub.handleWS))
	defer server.Close()
	hub.origins = []string{server.URL}
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	if conn, response, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": []string{server.URL + "/other"}}); err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		if conn != nil {
			_ = conn.Close()
		}
		t.Fatalf("unlisted socket origin: %+v, %v", response, err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": []string{server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	_, welcome, err := conn.ReadMessage()
	if err != nil || !strings.Contains(string(welcome), `"connected"`) {
		t.Fatalf("admitted socket welcome: %s, %v", welcome, err)
	}
	request := httptest.NewRequest(http.MethodGet, "/ws", nil)
	request.RemoteAddr = ""
	if address := clientIPFromRequest(request); address != "" {
		t.Fatalf("client address without port: %q", address)
	}
	if options := parseRedisOpts("://"); options == nil || options.Addr != "" || options.Username != "" {
		t.Fatalf("unparseable Redis URL options: %+v", options)
	}
}

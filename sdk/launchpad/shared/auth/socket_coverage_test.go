package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestPrivateSocketRevocationFollowsLiveSubscription(t *testing.T) {
	revoke := make(chan struct{})
	served := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		for _, frame := range []string{
			`{"op":"subscribed","subscriptionId":"other","topics":[{"topic":"private.wallet","accepted":true}]}`,
			`{"op":"subscribed","subscriptionId":"own","topics":[{"topic":"private.wallet","accepted":true}]}`,
			`{"op":"checkpoint","subscriptionId":"own","code":"REPLAY_COMPLETE","cursor":"1"}`,
		} {
			if err := connection.WriteMessage(websocket.TextMessage, []byte(frame)); err != nil {
				return
			}
		}
		<-revoke
		_ = connection.WriteMessage(websocket.TextMessage, []byte(`{"code":"AUTH_REQUIRED","subscriptionId":"own"}`))
		close(served)
	}))
	defer server.Close()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	probe, err := PreparePrivateSocketRevocation(ctx, connection, "own", "private.wallet")
	if err != nil || probe == nil || probe.subscriptionID != "own" || probe.activeAt.IsZero() {
		t.Fatalf("live private subscription: %+v, %v", probe, err)
	}
	revokedAt := time.Now()
	close(revoke)
	duration, err := MeasurePrivateSocketRevocation(ctx, probe, revokedAt)
	<-served
	if err != nil || duration < 0 || duration > 5*time.Second {
		t.Fatalf("private revocation: %s, %v", duration, err)
	}
	if _, err := MeasurePrivateSocketRevocation(ctx, nil, revokedAt); err == nil {
		t.Fatal("revocation without a live private socket admitted")
	}
}

func TestPrivateSocketPreparationRejectsControlErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kind  int
		frame string
	}{
		{"authority denial", websocket.TextMessage, `{"op":"error","subscriptionId":"own"}`},
		{"malformed", websocket.TextMessage, `{`},
		{"binary", websocket.BinaryMessage, `{"op":"subscribed"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				connection, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer connection.Close()
				_ = connection.WriteMessage(tc.kind, []byte(tc.frame))
			}))
			defer server.Close()
			connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if probe, err := PreparePrivateSocketRevocation(ctx, connection, "own", "private.wallet"); err == nil || probe != nil {
				t.Fatalf("invalid private control accepted: %+v, %v", probe, err)
			}
		})
	}
	if probe, err := PreparePrivateSocketRevocation(context.Background(), nil, "own", "private.wallet"); err == nil || probe != nil {
		t.Fatalf("unbounded private preparation: %+v, %v", probe, err)
	}
}

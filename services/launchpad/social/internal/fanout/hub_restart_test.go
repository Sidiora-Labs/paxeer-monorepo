package fanout_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/devsocial"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/gorilla/websocket"
)

func TestHubRunAgainRestoresSubscriptions(t *testing.T) {
	store := durableFixture(t)
	store.Clock = func() time.Time { return time.Now().Truncate(time.Second) }
	pub, err := sharedredis.NewClient("redis://:kindle_fixture_password@127.0.0.1:26379/0")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := sharedredis.NewClient("redis://:kindle_fixture_password@127.0.0.1:26379/0")
	if err != nil {
		_ = pub.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pub.Close(); _ = sub.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hub := fanout.New(fanout.Deps{Pub: pub, Sub: sub, Durable: store, ChannelNamespace: store.ChannelNamespace, Authenticate: devsocial.Authenticate})
	server := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer server.Close()
	first, stopFirst := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- hub.Run(first) }()
	stopFirst()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("hub shutdown = %v", err)
		}
	case <-ctx.Done():
		t.Fatal("hub did not return after cancellation")
	}

	wallet, pool := addr(0xbb), addr(1)
	headers := http.Header{}
	headers.Set(devsocial.ActorHeader, wallet)
	socket, response, err := websocket.DefaultDialer.DialContext(ctx, strings.Replace(server.URL, "http", "ws", 1), headers)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
		}
		t.Fatal(err)
	}
	defer func() { _ = socket.Close() }()
	readUntil(t, socket, "session_accepted")
	send(t, socket, map[string]any{"type": "join_pool", "pool_address": pool})
	readUntil(t, socket, "joined_pool")
	send(t, socket, map[string]any{"type": "subscribe_dms"})
	readUntil(t, socket, "subscribed_dms")

	go func() { done <- hub.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("hub shutdown = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("hub did not stop")
		}
	}()
	for _, channel := range []string{store.ChannelNamespace + "pool:" + pool, store.ChannelNamespace + "dm:wallet:" + wallet} {
		for {
			counts, err := pub.PubSubNumSub(ctx, channel).Result()
			if err != nil {
				t.Fatal(err)
			}
			if counts[channel] == 1 {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("room subscription did not resume")
			case <-time.After(time.Millisecond):
			}
		}
	}
	for _, input := range []fanout.SendInput{
		{Actor: wallet, Kind: "pool", Target: pool, ClientID: "resumed-pool", Content: "resumed pool delivery"},
		{Actor: addr(0xaa), Kind: "dm", Target: wallet, ClientID: "resumed-dm", Content: "resumed direct delivery"},
	} {
		message, err := store.Send(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		n, err := store.DrainOutbox(ctx, 100, func(ctx context.Context, _ string, channels []string, payload []byte) error {
			for _, channel := range channels {
				if err := pub.Publish(ctx, channel, payload).Err(); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil || n != 1 {
			t.Fatalf("durable publication count = %d: %v", n, err)
		}
		expected, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		var want map[string]any
		if err := json.Unmarshal(expected, &want); err != nil {
			t.Fatal(err)
		}
		if got := readUntil(t, socket, message.Type); !reflect.DeepEqual(got, want) {
			t.Fatalf("resumed delivery = %v, want %v", got, want)
		}
	}
}

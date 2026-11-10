package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
)

// subscribeConfirmed subscribes to channel and blocks until the subscription is
// confirmed so a following publish cannot race ahead of registration.
func subscribeConfirmed(t *testing.T, ctx context.Context, rdb *goredis.Client, channel string) *goredis.PubSub {
	t.Helper()
	sub := rdb.Subscribe(ctx, channel)
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe %s: %v", channel, err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	return sub
}

func awaitPublish(t *testing.T, sub *goredis.PubSub) []byte {
	t.Helper()
	select {
	case msg := <-sub.Channel():
		return []byte(msg.Payload)
	case <-time.After(3 * time.Second):
		t.Fatal("expected a published message, got none")
		return nil
	}
}

// TestReactionVotePublishesReactionsUpdate ensures the retired compatibility
// route cannot publish a Redis-only vote as if it were durably committed.
func TestReactionVotePublishesReactionsUpdate(t *testing.T) {
	ctx := context.Background()
	rdb := internaltest.NewRedis(t)
	router := chi.NewRouter()
	httpapi.RegisterReactions(router, rdb)
	key, wallet := newWallet(t)
	message := "vote on " + repair1039ReactionPool
	sub := subscribeConfirmed(t, ctx, rdb, constants.ChannelReactionsUpdate)
	body := mustJSON(t, map[string]string{"reaction": "bullish", "walletAddress": wallet, "signature": signWallet(t, key, message), "message": message})
	response := serve(t, router, http.MethodPost, "/stats/"+repair1039ReactionPool+"/reactions", body, nil)
	repair1039RequireStatus(t, response, http.StatusGone)
	select {
	case msg := <-sub.Channel():
		t.Fatalf("retired mutation published an uncommitted tally: %s", msg.Payload)
	case <-time.After(50 * time.Millisecond):
	}
	keys, err := rdb.Keys(ctx, "reactions:*").Result()
	if err != nil || len(keys) != 0 {
		t.Fatalf("retired route wrote Redis vote state: %v %v", keys, err)
	}
	repair1039DurableReplacement(t)
}

// TestPrecomputePublishesPlatformUpdate proves the background precompute pushes
// platform:update (a global event) carrying the same metrics it cached.
func TestPrecomputePublishesPlatformUpdate(t *testing.T) {
	ctx := context.Background()
	st := newPlatformStore(t)
	rdb := internaltest.NewRedis(t)

	sub := subscribeConfirmed(t, ctx, rdb, constants.ChannelPlatformUpdate)

	if err := httpapi.PrecomputePlatformMetrics(ctx, st, rdb); err != nil {
		t.Fatalf("precompute platform metrics: %v", err)
	}

	payload := awaitPublish(t, sub)
	// The published payload must equal the cached payload byte-for-byte.
	cached, err := rdb.Get(ctx, "platform:metrics").Result()
	if err != nil {
		t.Fatalf("read platform cache: %v", err)
	}
	if string(payload) != cached {
		t.Errorf("published platform payload != cached payload\npub=%s\ncache=%s", payload, cached)
	}
	// And it must be valid JSON object.
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		t.Fatalf("platform:update payload not a JSON object: %v", err)
	}
}

package l18

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestClaimPreparationValidatesRequestAndReplayBinding(t *testing.T) {
	f := repair62Chain(t)
	ctx := context.Background()
	body := iface.RewardClaimBody{RewardIds: []string{"creator:1"}, Recipient: f.chain.Alice}
	for _, tc := range []struct {
		name                 string
		wallet, key, request string
		body                 iface.RewardClaimBody
		want                 error
	}{
		{"actor", "invalid", uuid.NewString(), uuid.NewString(), body, ErrUnauthorized},
		{"key", f.chain.Alice, "invalid", uuid.NewString(), body, ErrInvalid},
		{"request", f.chain.Alice, uuid.NewString(), "invalid", body, ErrInvalid},
		{"batch", f.chain.Alice, uuid.NewString(), uuid.NewString(), iface.RewardClaimBody{Recipient: f.chain.Alice}, ErrUnavailable},
		{"recipient", f.chain.Alice, uuid.NewString(), uuid.NewString(), iface.RewardClaimBody{RewardIds: body.RewardIds, Recipient: "invalid"}, ErrInvalid},
		{"token", f.chain.Alice, uuid.NewString(), uuid.NewString(), iface.RewardClaimBody{RewardIds: []string{"creator:bad"}, Recipient: f.chain.Alice}, ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.journey.Prepare(ctx, Actor{Wallet: tc.wallet}, tc.key, tc.body, tc.request); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	key, request := uuid.NewString(), uuid.NewString()
	original, err := f.journey.Prepare(ctx, Actor{Wallet: f.chain.Alice}, key, body, request)
	if err != nil || !json.Valid(original) {
		t.Fatal(string(original), err)
	}
	changed := body
	changed.RewardIds = []string{"creator:2"}
	if _, err = f.journey.Prepare(ctx, Actor{Wallet: f.chain.Alice}, key, changed, request); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	for _, change := range []func(*JourneyService){func(s *JourneyService) { s.now = func() time.Time { return time.Now().Add(24 * time.Hour) } }, func(s *JourneyService) { s.releaseDigest = strings.Repeat("c", 64) }, func(s *JourneyService) { s.interfaceDigest = strings.Repeat("c", 64) }} {
		candidate := *f.journey
		change(&candidate)
		if _, err := candidate.Prepare(ctx, Actor{Wallet: f.chain.Alice}, key, body, request); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	}
	origin := httptest.NewServer(http.HandlerFunc(f.db.rpcResponse))
	defer origin.Close()
	variant := rewardReceiptVariant(t, origin.URL, func(method string, _ []json.RawMessage, out map[string]any) {
		if method == "eth_call" {
			delete(out, "result")
			out["error"] = map[string]any{"code": -32000, "message": "owner state unavailable"}
		}
	})
	creator := *f.creator
	creator.rpc = variant.rpc
	candidate := *f.journey
	candidate.creator = &creator
	if _, err = candidate.Prepare(ctx, Actor{Wallet: f.chain.Alice}, key, body, request); err == nil {
		t.Fatal("unavailable current owner accepted on replay")
	}
}

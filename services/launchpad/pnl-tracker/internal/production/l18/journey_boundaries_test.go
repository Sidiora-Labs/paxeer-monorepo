package l18

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCommunityAuthorityTracksOwnerAndApprovedWallet(t *testing.T) {
	f := repair62Chain(t)
	ctx := context.Background()
	for _, tc := range []struct {
		approved []string
		state    string
	}{{nil, "proposed"}, {[]string{f.chain.Bob}, "proposed"}, {[]string{f.chain.Alice}, "active"}} {
		got, err := f.journey.CommunityAuthority(ctx, "1", tc.approved)
		if err != nil || got.State != tc.state || got.Owner != f.chain.Alice || got.HistoricalCreator != f.chain.Alice || got.Sharing.Available {
			t.Fatal(got, err)
		}
	}
	if _, err := f.journey.CommunityAuthority(ctx, "1", []string{"invalid"}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	f.advance(f.db.peers.Advanced)
	got, err := f.journey.CommunityAuthority(ctx, "1", nil)
	if err != nil || got.State != "proof_pending" || got.Owner != f.chain.Bob {
		t.Fatal(got, err)
	}
	if _, err = f.journey.CommunityAuthority(ctx, "invalid", nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	for _, actor := range []Actor{{Wallet: "invalid"}, {Wallet: f.chain.Alice}} {
		_, err = f.journey.HolderEntitlement(ctx, actor, "invalid", "1")
		if err == nil {
			t.Fatal("invalid holder query accepted")
		}
		_, err = f.journey.PrepareHolderClaim(ctx, actor, "invalid", "1")
		if err == nil {
			t.Fatal("invalid holder claim accepted")
		}
	}
	capability, err := f.journey.Strategy(ctx, Actor{Wallet: f.chain.Bob}, "1", 0)
	if err != nil || !capability.Available || capability.Strategy != "CLAIM" {
		t.Fatal(capability, err)
	}
	if _, err = f.journey.Strategy(ctx, Actor{Wallet: "invalid"}, "1", 0); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	for _, approved := range [][]string{{"invalid"}, {f.chain.Alice, f.chain.Alice}, make([]string, 65)} {
		if _, err = NewCommunityWatcher(f.journey, approved); err == nil {
			t.Fatal("invalid community policy accepted")
		}
	}
	if _, err = NewCommunityWatcher(nil, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	watcher, err := NewCommunityWatcher(f.journey, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = watcher.Observe(ctx, "invalid", "1"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = watcher.Observe(ctx, uuid.NewString(), "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestRewardCursorBindsResourceWalletAndCheckpoint(t *testing.T) {
	now := time.Unix(10000, 0)
	s := &JourneyService{key: []byte(strings.Repeat("k", 32)), now: func() time.Time { return now }, interfaceDigest: strings.Repeat("a", 64)}
	original := rewardCursor{Wallet: actorA, Program: "creator_fees", After: "1", Block: Checkpoint{10, "0x" + strings.Repeat("b", 64)}, Expires: now.Add(time.Hour).Unix(), Resource: "rewards", SchemaMajor: 2, Interface: s.interfaceDigest}
	token := s.cursor(original)
	if got, err := s.decodeCursor(token, actorA, "creator_fees"); err != nil || got != original {
		t.Fatal(got, err)
	}
	for _, token := range []string{strings.Repeat("x", 2049), "invalid", "!.YQ", "YQ.!", token + "x"} {
		if _, err := s.decodeCursor(token, actorA, "creator_fees"); !errors.Is(err, ErrInvalid) {
			t.Fatal(token, err)
		}
	}
	for _, mutate := range []func(*rewardCursor){func(c *rewardCursor) { c.Wallet = actorB }, func(c *rewardCursor) { c.Program = "other" }, func(c *rewardCursor) { c.Resource = "positions" }, func(c *rewardCursor) { c.SchemaMajor = 3 }, func(c *rewardCursor) { c.Interface = "other" }, func(c *rewardCursor) { c.Expires = 1 }, func(c *rewardCursor) { c.Block.Hash = "invalid" }, func(c *rewardCursor) { c.After = "01" }} {
		changed := original
		mutate(&changed)
		if _, err := s.decodeCursor(s.cursor(changed), actorA, "creator_fees"); !errors.Is(err, ErrInvalid) {
			t.Fatal(changed, err)
		}
	}
	if s.watermark(Checkpoint{}) != nil {
		t.Fatal("empty checkpoint has watermark")
	}
	if got := s.watermark(original.Block); got == nil || got.Height != "10" || got.BlockHash != original.Block.Hash {
		t.Fatal(got)
	}
	capability := &CapabilityError{Capability: "holder", Reason: "snapshot authority unavailable"}
	if capability.Error() != capability.Reason || !errors.Is(capability, ErrUnavailable) {
		t.Fatal(capability)
	}
	for _, err := range []error{ErrInvalid, ErrUnauthorized, ErrConflict, pgx.ErrNoRows, ErrUnavailable, errMalformed, capability} {
		response := httptest.NewRecorder()
		writeJourneyError(response, fmt.Errorf("request: %w", err))
		var body struct {
			Code, Message, RequestID string
			Retryable                bool
			Details                  map[string]string
		}
		if e := json.Unmarshal(response.Body.Bytes(), &body); e != nil || body.Code == "" || body.Message == "" || uuid.Validate(body.RequestID) != nil || body.Retryable != (response.Code == 503) {
			t.Fatal(response.Code, response.Body.String(), e)
		}
		if errors.Is(err, capability) && (body.Code != "FEATURE_UNAVAILABLE" || body.Details["capability"] != "holder") {
			t.Fatal(body)
		}
	}
}

func TestRewardHTTPRejectsInvalidQueriesAndOversizedClaims(t *testing.T) {
	f := repair62Chain(t)
	handler := f.handler(f.chain.Alice)
	for _, query := range []string{"unknown=1", "limit=1&limit=2", "limit=abc", "limit=0", "programId=" + strings.Repeat("x", 65), "cursor=" + strings.Repeat("x", 2049)} {
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, httptest.NewRequest("GET", "/v2/me/rewards?"+query, nil))
		if out.Code != 422 {
			t.Fatal(query, out.Code, out.Body.String())
		}
		var body struct{ Code string }
		if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil || body.Code != "INVALID_REQUEST" {
			t.Fatal(body, err)
		}
	}
	request := httptest.NewRequest("POST", "/v2/me/rewards:prepare-claim", strings.NewReader(strings.Repeat("x", 16385)))
	request.Header.Set("Idempotency-Key", uuid.NewString())
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, request)
	if out.Code != 422 {
		t.Fatal(out.Code, out.Body.String())
	}
	for _, claim := range []bool{false, true} {
		out = httptest.NewRecorder()
		h := HTTP{}
		if claim {
			h.PrepareClaim(out, httptest.NewRequest("POST", "/", nil))
		} else {
			h.Rewards(out, httptest.NewRequest("GET", "/", nil))
		}
		if out.Code != 503 {
			t.Fatal(out.Code)
		}
		out = httptest.NewRecorder()
		if claim {
			h.PrepareClaim(out, httptest.NewRequest("GET", "/", nil))
		} else {
			h.Rewards(out, httptest.NewRequest("POST", "/", nil))
		}
		if out.Code != 405 {
			t.Fatal(out.Code)
		}
	}
	invalid := f.handler("invalid")
	out = httptest.NewRecorder()
	invalid.ServeHTTP(out, httptest.NewRequest("GET", "/v2/me/rewards", nil))
	if out.Code != http.StatusForbidden {
		t.Fatal(out.Code)
	}
	for _, body := range []iface.RewardClaimBody{{RewardIds: []string{"creator:1"}, Recipient: actorA}, {RewardIds: []string{"invalid"}, Recipient: f.chain.Alice}} {
		_, err := f.journey.Prepare(context.Background(), Actor{Wallet: f.chain.Alice}, uuid.NewString(), body, uuid.NewString())
		if err == nil {
			t.Fatal(body)
		}
	}
}

func TestRewardArithmeticRejectsInconsistentEpochState(t *testing.T) {
	for _, values := range [][5]string{{"bad", "0", "10", "0", "1"}, {"10", "11", "10", "0", "1"}, {"10", "10", "0", "0", "0"}, {"10", "10", "10", "11", "1"}, {"10", "10", "10", "0", "11"}, {"10", "10", "10", "9", "2"}, {"10", "0", "10", "0", "1"}, {new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String(), "10", "10", "0", "2"}} {
		if _, err := EpochShare(values[0], values[1], values[2], values[3], values[4], false); !errors.Is(err, ErrInvalid) {
			t.Fatal(values, err)
		}
	}
	if got, err := EpochShare("10", "10", "10", "0", "0", false); err != nil || got != "0" {
		t.Fatal(got, err)
	}
	for _, tc := range []struct {
		net, cap string
		bps      uint16
	}{{"bad", "1", 1}, {"1", "bad", 1}, {"1", "1", 10001}} {
		if _, err := Cashback(tc.net, tc.bps, tc.cap); !errors.Is(err, ErrInvalid) {
			t.Fatal(tc, err)
		}
	}
	var empty *JourneyService
	if _, err := empty.WithSourceStream("stream"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := (&JourneyService{}).WithSourceStream("invalid stream"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if hexDigest(strings.Repeat("A", 64)) || hexDigest("short") {
		t.Fatal("noncanonical digest accepted")
	}
}

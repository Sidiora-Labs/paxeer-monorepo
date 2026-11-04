package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	jwxjwt "github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/auth/jwt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
)

type repairM170Provider struct {
	server  *httptest.Server
	key     jwk.Key
	public  jwk.Key
	fetches atomic.Int64
	fail    atomic.Bool
	rotated atomic.Bool
	now     atomic.Int64
}

func newRepairM170Provider(t *testing.T) *repairM170Provider {
	t.Helper()
	raw, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := jwk.FromRaw(raw)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{jwk.KeyIDKey: "current", jwk.AlgorithmKey: jwa.ES256} {
		if err := key.Set(name, value); err != nil {
			t.Fatal(err)
		}
	}
	public, err := jwk.PublicKeyOf(key)
	if err != nil {
		t.Fatal(err)
	}
	p := &repairM170Provider{key: key, public: public}
	p.now.Store(time.Now().Truncate(time.Second).UnixNano())
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.fetches.Add(1)
		if p.fail.Load() {
			http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
			return
		}
		set := jwk.NewSet()
		if err := set.AddKey(p.public); err != nil {
			t.Error(err)
			http.Error(w, "key set", 500)
			return
		}
		if p.rotated.Load() {
			rotated, err := p.public.Clone()
			if err != nil {
				t.Error(err)
				http.Error(w, "clone", 500)
				return
			}
			if err := rotated.Set(jwk.KeyIDKey, "rotated"); err != nil {
				t.Error(err)
				http.Error(w, "key id", 500)
				return
			}
			if err := set.AddKey(rotated); err != nil {
				t.Error(err)
				http.Error(w, "key set", 500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(set); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(p.server.Close)
	return p
}

func (p *repairM170Provider) clock() time.Time { return time.Unix(0, p.now.Load()) }

func (p *repairM170Provider) mint(t *testing.T, kid string) string {
	t.Helper()
	key, err := p.key.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if err := key.Set(jwk.KeyIDKey, kid); err != nil {
		t.Fatal(err)
	}
	token, err := jwxjwt.NewBuilder().Issuer(p.server.URL).Audience([]string{"attestor"}).Subject("wallet-owner").IssuedAt(p.clock()).Expiration(p.clock().Add(time.Hour)).Build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwxjwt.Sign(token, jwxjwt.WithKey(jwa.ES256, key))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func (p *repairM170Provider) verifier(t *testing.T) (*jwt.TokenVerifier, *store.Store) {
	t.Helper()
	key := sha256.Sum256([]byte("repair-m170-store"))
	st, err := store.Open(t.TempDir(), key[:])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Error(err)
		}
	})
	cfg := tokenVerifierConfig(&config.Config{JWKSURL: p.server.URL, JWTIssuer: p.server.URL, JWTAudience: "attestor", JWTMaxAge: 5 * time.Minute}, st)
	if cfg.MinRefreshInterval != time.Minute {
		t.Fatalf("refresh interval = %s, want one minute", cfg.MinRefreshInterval)
	}
	if cfg.HTTPClient == nil || cfg.HTTPClient.Timeout != 10*time.Second {
		t.Fatal("production HTTP fetch timeout must remain ten seconds")
	}
	if cfg.Replay != st || cfg.MaxAge != 5*time.Minute {
		t.Fatal("production replay store or maximum token age was lost")
	}
	cfg.Now = p.clock
	v, err := jwt.NewTokenVerifier(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return v, st
}

func repairM170Owns(subject, keyID string) (bool, error) {
	return subject == "wallet-owner" && keyID == "wallet-key", nil
}

func repairM170Request(t *testing.T, session string) [32]byte {
	t.Helper()
	body, err := json.Marshal(map[string]string{"session_id": session, "key_id": "wallet-key"})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := jwt.RequestDigest("/v1/sign", "wallet-key", body)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func TestRepairM_170_UnknownKIDFloodAlongsideValidVerification(t *testing.T) {
	p := newRepairM170Provider(t)
	v, _ := p.verifier(t)
	valid := p.mint(t, "current")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	warm := repairM170Request(t, "warm")
	if subject, err := v.Verify(ctx, valid, "wallet-key", warm, repairM170Owns); err != nil || subject != "wallet-owner" {
		t.Fatalf("warm verification = %q, %v", subject, err)
	}
	if got := p.fetches.Load(); got != 1 {
		t.Fatalf("warm fetches = %d, want 1", got)
	}
	p.fail.Store(true)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 64; i++ {
		unknown := p.mint(t, fmt.Sprintf("attacker-%d", i))
		request := repairM170Request(t, fmt.Sprintf("request-%d", i))
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			if _, err := v.Verify(ctx, unknown, "wallet-key", request, repairM170Owns); !errors.Is(err, jwt.ErrUnknownKey) {
				t.Errorf("unknown key: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			if subject, err := v.Verify(ctx, valid, "wallet-key", request, repairM170Owns); err != nil || subject != "wallet-owner" {
				t.Errorf("valid verification during flood = %q, %v", subject, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if ctx.Err() != nil {
		t.Fatalf("flood exceeded verification deadline: %v", ctx.Err())
	}
	if got := p.fetches.Load(); got != 1 {
		t.Fatalf("unknown-kid flood fetched unavailable provider: total %d, want 1", got)
	}
	if _, err := v.Verify(ctx, valid, "wallet-key", warm, repairM170Owns); !errors.Is(err, jwt.ErrReplayed) {
		t.Fatalf("replay protection after flood: %v", err)
	}
}

func TestRepairM_170_FailedFetchIsThrottledAndRecovers(t *testing.T) {
	p := newRepairM170Provider(t)
	v, _ := p.verifier(t)
	valid := p.mint(t, "current")
	request := repairM170Request(t, "recover")
	p.fail.Store(true)
	if _, err := v.Verify(context.Background(), valid, "wallet-key", request, repairM170Owns); !errors.Is(err, jwt.ErrKeySetFetch) {
		t.Fatalf("initial fetch failure: %v", err)
	}
	for i := 0; i < 32; i++ {
		unknown := p.mint(t, fmt.Sprintf("missing-%d", i))
		if _, err := v.Verify(context.Background(), unknown, "wallet-key", request, repairM170Owns); !errors.Is(err, jwt.ErrKeySetFetch) {
			t.Fatalf("throttled failed fetch %d: %v", i, err)
		}
	}
	if got := p.fetches.Load(); got != 1 {
		t.Fatalf("failed-fetch flood requests = %d, want 1", got)
	}
	p.fail.Store(false)
	p.now.Add(int64(time.Minute - time.Nanosecond))
	if _, err := v.Verify(context.Background(), valid, "wallet-key", request, repairM170Owns); !errors.Is(err, jwt.ErrKeySetFetch) {
		t.Fatalf("fetch before refresh boundary: %v", err)
	}
	if got := p.fetches.Load(); got != 1 {
		t.Fatalf("early retry fetches = %d, want 1", got)
	}
	p.now.Add(1)
	if subject, err := v.Verify(context.Background(), valid, "wallet-key", request, repairM170Owns); err != nil || subject != "wallet-owner" {
		t.Fatalf("provider recovery = %q, %v", subject, err)
	}
	if got := p.fetches.Load(); got != 2 {
		t.Fatalf("recovery fetches = %d, want 2", got)
	}
}

func TestRepairM_170_RotationResumesAtRefreshBoundary(t *testing.T) {
	p := newRepairM170Provider(t)
	v, _ := p.verifier(t)
	valid, rotated := p.mint(t, "current"), p.mint(t, "rotated")
	request := repairM170Request(t, "rotation")
	if _, err := v.Verify(context.Background(), valid, "wallet-key", request, repairM170Owns); err != nil {
		t.Fatal(err)
	}
	p.rotated.Store(true)
	p.now.Add(int64(time.Minute - time.Nanosecond))
	if _, err := v.Verify(context.Background(), rotated, "wallet-key", request, repairM170Owns); !errors.Is(err, jwt.ErrUnknownKey) {
		t.Fatalf("rotation before refresh boundary: %v", err)
	}
	if got := p.fetches.Load(); got != 1 {
		t.Fatalf("early rotation fetches = %d, want 1", got)
	}
	p.now.Add(1)
	if subject, err := v.Verify(context.Background(), rotated, "wallet-key", request, repairM170Owns); err != nil || subject != "wallet-owner" {
		t.Fatalf("rotated verification = %q, %v", subject, err)
	}
	if got := p.fetches.Load(); got != 2 {
		t.Fatalf("rotation fetches = %d, want 2", got)
	}
	unknown := p.mint(t, "attacker-after-rotation")
	if _, err := v.Verify(context.Background(), unknown, "wallet-key", request, repairM170Owns); !errors.Is(err, jwt.ErrUnknownKey) {
		t.Fatalf("unknown key after rotation: %v", err)
	}
	if got := p.fetches.Load(); got != 2 {
		t.Fatalf("rotation did not reset throttle: %d fetches", got)
	}
	if subject, err := v.Verify(context.Background(), valid, "wallet-key", repairM170Request(t, "old-key-still-valid"), repairM170Owns); err != nil || subject != "wallet-owner" {
		t.Fatalf("previous cached key after rotation = %q, %v", subject, err)
	}
}

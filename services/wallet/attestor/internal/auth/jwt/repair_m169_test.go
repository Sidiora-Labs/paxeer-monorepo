package jwt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

func (s *jwksServer) replaceKeys(keys ...jwk.Key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = keys
}

func repairM169Verify(t *testing.T, f *fixture, token string, want error) {
	t.Helper()
	subject, err := f.verifier.Verify(context.Background(), token, testKeyID, testRequest, ownsTestKey)
	if want == nil {
		if err != nil || subject != testSubject {
			t.Fatalf("Verify = (%q, %v), want (%q, nil)", subject, err, testSubject)
		}
	} else if !errors.Is(err, want) || subject != "" {
		t.Fatalf("Verify = (%q, %v), want empty subject and %v", subject, err, want)
	}
}

func repairM169Fetches(t *testing.T, f *fixture, want int64) {
	t.Helper()
	if got := f.server.fetches.Load(); got != want {
		t.Fatalf("JWKS HTTP requests = %d, want %d", got, want)
	}
}

func TestRepairM_169_SameKidReplacement(t *testing.T) {
	for _, algorithm := range []string{"RS256", "ES256"} {
		t.Run(algorithm, func(t *testing.T) {
			f := newFixture(t)
			original := f.rsaKey
			if algorithm == "ES256" {
				original = f.ecKey
			}
			oldToken := mint(t, original, f.validClaims())
			repairM169Verify(t, f, oldToken, nil)
			var rotated signingKey
			if algorithm == "RS256" {
				rotated = newRSAKey(t, original.public.KeyID())
			} else {
				rotated = newP256Key(t, original.public.KeyID())
			}
			f.server.replaceKeys(rotated.public)
			newToken := mint(t, rotated, f.validClaims())
			f.clock.Advance(DefaultKeySetRefreshInterval - time.Nanosecond)
			repairM169Verify(t, f, oldToken, nil)
			repairM169Verify(t, f, newToken, ErrSignature)
			repairM169Fetches(t, f, 1)
			f.clock.Advance(time.Nanosecond)
			// The only kid presented has been known since the initial fetch.
			repairM169Verify(t, f, newToken, nil)
			repairM169Verify(t, f, oldToken, ErrSignature)
			repairM169Fetches(t, f, 2)
		})
	}
}

func TestRepairM_169_Removal(t *testing.T) {
	for _, empty := range []bool{false, true} {
		name := "other-key-retained"
		if empty {
			name = "empty-authoritative-set"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			token := mint(t, f.rsaKey, f.validClaims())
			repairM169Verify(t, f, token, nil)
			if empty {
				f.server.replaceKeys()
			} else {
				f.server.replaceKeys(f.ecKey.public)
			}
			f.clock.Advance(DefaultKeySetRefreshInterval)
			repairM169Verify(t, f, token, ErrUnknownKey)
			repairM169Verify(t, f, token, ErrUnknownKey)
			if !empty {
				repairM169Verify(t, f, mint(t, f.ecKey, f.validClaims()), nil)
			}
			repairM169Fetches(t, f, 2)
			// An outage after an authoritative removal cannot resurrect the old set.
			f.server.setFail(true)
			f.clock.Advance(minRefresh)
			repairM169Verify(t, f, token, ErrKeySetFetch)
			repairM169Fetches(t, f, 3)
		})
	}
}

func TestRepairM_169_MaximumStaleAndRecovery(t *testing.T) {
	f := newFixture(t)
	v, err := NewTokenVerifier(Config{
		JWKSURL: f.server.jwksURL(), Issuer: f.server.issuer(), Audience: testAudience,
		HTTPClient: f.server.srv.Client(), Now: f.clock.Now,
		MinRefreshInterval: minRefresh, KeySetRefreshInterval: time.Minute, MaxKeySetAge: 3 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.verifier = v
	token := mint(t, f.ecKey, f.validClaims())
	repairM169Verify(t, f, token, nil)
	f.server.setFail(true)
	f.clock.Advance(time.Minute)
	repairM169Verify(t, f, token, nil)
	repairM169Verify(t, f, token, nil)
	repairM169Fetches(t, f, 2)
	unknown := newP256Key(t, "unknown")
	repairM169Verify(t, f, mint(t, unknown, f.validClaims()), ErrKeySetFetch)
	repairM169Fetches(t, f, 2)
	f.clock.Advance(time.Minute + 50*time.Second)
	repairM169Verify(t, f, token, nil)
	repairM169Fetches(t, f, 3)
	f.clock.Advance(10 * time.Second)
	// The last failed attempt is within the cooldown, but the stale deadline wins.
	repairM169Verify(t, f, token, ErrKeySetFetch)
	repairM169Fetches(t, f, 3)
	f.clock.Advance(20 * time.Second)
	repairM169Verify(t, f, token, ErrKeySetFetch)
	repairM169Fetches(t, f, 4)
	rotated := newP256Key(t, f.ecKey.public.KeyID())
	f.server.replaceKeys(rotated.public)
	f.server.setFail(false)
	f.clock.Advance(minRefresh)
	repairM169Verify(t, f, mint(t, rotated, f.validClaims()), nil)
	repairM169Verify(t, f, token, ErrSignature)
	repairM169Fetches(t, f, 5)
}

func TestRepairM_169_DefaultMaximumStale(t *testing.T) {
	f := newFixture(t)
	token := mint(t, f.ecKey, f.validClaims())
	repairM169Verify(t, f, token, nil)
	f.server.setFail(true)
	f.clock.Advance(DefaultMaxKeySetAge - time.Nanosecond)
	repairM169Verify(t, f, token, nil)
	f.clock.Advance(time.Nanosecond)
	repairM169Verify(t, f, token, ErrKeySetFetch)
	repairM169Fetches(t, f, 2)
}

func TestRepairM_169_ConcurrentKnownKeyRefresh(t *testing.T) {
	f := newFixture(t)
	token := mint(t, f.ecKey, f.validClaims())
	repairM169Verify(t, f, token, nil)
	f.clock.Advance(DefaultKeySetRefreshInterval)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			subject, err := f.verifier.Verify(context.Background(), token, testKeyID, testRequest, ownsTestKey)
			if err != nil || subject != testSubject {
				t.Errorf("concurrent Verify = (%q, %v)", subject, err)
			}
		}()
	}
	wg.Wait()
	repairM169Fetches(t, f, 2)
}

func TestRepairM_169_FetchDurationCountsAgainstMaximumAge(t *testing.T) {
	f := newFixture(t)
	set := jwk.NewSet()
	if err := set.AddKey(f.ecKey.public); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.clock.Advance(DefaultMaxKeySetAge)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(set); err != nil {
			t.Errorf("encode JWKS: %v", err)
		}
	}))
	defer srv.Close()
	v, err := NewTokenVerifier(Config{
		JWKSURL: srv.URL, Issuer: f.server.issuer(), Audience: testAudience,
		HTTPClient: srv.Client(), Now: f.clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.verifier = v
	repairM169Verify(t, f, mint(t, f.ecKey, f.validClaims()), ErrKeySetFetch)
}

func TestRepairM_169_FreshnessConfiguration(t *testing.T) {
	for name, intervals := range map[string][3]time.Duration{
		"negative-refresh":           {0, -time.Second, 0},
		"negative-maximum":           {0, 0, -time.Second},
		"cooldown-exceeds-freshness": {2 * time.Minute, time.Minute, 3 * time.Minute},
		"freshness-exceeds-maximum":  {0, 4 * time.Minute, 3 * time.Minute},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewTokenVerifier(Config{
				JWKSURL: "http://localhost/jwks", Issuer: "issuer", Audience: "audience", HTTPClient: http.DefaultClient,
				MinRefreshInterval: intervals[0], KeySetRefreshInterval: intervals[1], MaxKeySetAge: intervals[2],
			})
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("invalid freshness configuration: %v", err)
			}
		})
	}
}

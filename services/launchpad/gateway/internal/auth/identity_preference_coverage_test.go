package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestIdentityPreferenceContinuityAcrossVerifiedWallets(t *testing.T) {
	f, cfg := measurementFixtureConfig(t)
	first, principal := ownerCredentials(t, f)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	ip, _, err := net.SplitHostPort(f.server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	linkChallenge, linkID, err := f.s.IssueLink(f.ctx, principal, WalletLinkChallengeRequest{wallet, "125", "private"}, ip)
	if err != nil {
		t.Fatal(err)
	}
	link, err := f.s.VerifyLink(f.ctx, principal, WalletLinkVerifyRequest{linkID, linkChallenge.ChallengeID, proofIdentity(t, key, linkChallenge)})
	if err != nil || link.Wallet != wallet || link.AccountID != principal.AccountID() {
		t.Fatalf("verified second wallet: %+v err=%v", link, err)
	}
	challenge := f.challenge(t, wallet)
	second, err := f.s.Login(f.ctx, LoginRequest{challenge.ChallengeID, proofIdentity(t, key, challenge), "second verified wallet"})
	if err != nil {
		t.Fatal(err)
	}
	var changed, unavailable atomic.Bool
	seen := make(chan string, 6)
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/me/preferences" || r.Header.Get("Origin") != "https://kindle.test" || r.Header.Get("X-Request-Id") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token != first.AccessToken && token != second.AccessToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		seen <- token
		if unavailable.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		value := `{"data":{"preferences":{"theme":"dark","risk":2}}}`
		if changed.Load() && token == second.AccessToken {
			value = `{"data":{"preferences":{"theme":"light","risk":2}}}`
		}
		_, _ = io.WriteString(w, value)
	}))
	defer service.Close()
	client := IdentityMeasurementClient{BaseURL: service.URL, Origin: "https://kindle.test", Client: service.Client()}
	result, err := f.s.MeasurePreferenceContinuity(context.Background(), client, first.AccessToken, second.AccessToken, cfg.Binding)
	if err != nil || !result.Passed || result.VerifiedWallets != 2 || result.AccountSHA256 != measurementHash([]byte(principal.AccountID())) || result.PreferencesSHA256 == "" || len(result.ProofHistorySHA256) != hex.EncodedLen(sha256.Size) || result.CapturedAt.IsZero() {
		t.Fatalf("wallet preference continuity: %+v err=%v", result, err)
	}
	if firstSeen, secondSeen := <-seen, <-seen; firstSeen != first.AccessToken || secondSeen != second.AccessToken {
		t.Fatal("preference service did not observe both verified wallet credentials")
	}
	changed.Store(true)
	result, err = f.s.MeasurePreferenceContinuity(context.Background(), client, first.AccessToken, second.AccessToken, cfg.Binding)
	if err == nil || !strings.Contains(err.Error(), "wallet switch changed preference record") || result.Passed {
		t.Fatalf("changed preference record: %+v err=%v", result, err)
	}
	if firstSeen, secondSeen := <-seen, <-seen; firstSeen != first.AccessToken || secondSeen != second.AccessToken {
		t.Fatal("changed preference comparison omitted a wallet")
	}
	unavailable.Store(true)
	result, err = f.s.MeasurePreferenceContinuity(context.Background(), client, first.AccessToken, second.AccessToken, cfg.Binding)
	if err == nil || !strings.Contains(err.Error(), "preference read failed") || result.Passed || <-seen != first.AccessToken {
		t.Fatalf("unavailable preference service: %+v err=%v", result, err)
	}
}

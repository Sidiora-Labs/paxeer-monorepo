package auth

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
)

func TestLegacyLoginRejectsIncompleteChallenge(t *testing.T) {
	router := newAuthRouter(t)
	wallet := newWallet(t)
	for _, tc := range []struct {
		name string
		body loginRequest
		want string
	}{
		{"wallet", loginRequest{Wallet: "invalid", Message: "message", Signature: "signature"}, "invalid wallet address"},
		{"signature", loginRequest{Wallet: wallet.addr, Message: "message"}, "missing message or signature"},
		{"nonce", loginRequest{Wallet: wallet.addr, Message: "message", Signature: "signature"}, "message missing nonce"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := postJSON(t, router, "/auth/login", tc.body)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), tc.want) {
				t.Fatalf("challenge response=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestLegacySessionOptionalActorAndRedisUnavailable(t *testing.T) {
	rdb, _ := internaltest.NewRedis(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auth := New(Deps{Redis: rdb, JWTSecret: "test-signing-secret-key", JWTTTL: time.Hour, NonceTTL: time.Minute, Logger: logger})
	wallet := newWallet(t)
	token, _ := auth.signer.mint(wallet.addr)
	for _, tc := range []struct {
		name, authorization, actor string
	}{
		{"anonymous", "", ""},
		{"signed", "Bearer " + token, wallet.addr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/social/profile", nil)
			request.Header.Set("Authorization", tc.authorization)
			observed := ""
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				observed = Actor(r.Context())
				w.WriteHeader(http.StatusNoContent)
			})
			response := httptest.NewRecorder()
			auth.OptionalSession(next).ServeHTTP(response, request)
			if response.Code != http.StatusNoContent || observed != tc.actor {
				t.Fatalf("optional session: status=%d actor=%q", response.Code, observed)
			}
		})
	}
	if err := rdb.Close(); err != nil {
		t.Fatal(err)
	}
	response := postJSON(t, http.HandlerFunc(auth.nonce), "/auth/nonce", nonceRequest{Wallet: wallet.addr})
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "Internal Server Error") {
		t.Fatalf("nonce store response=%d body=%s", response.Code, response.Body.String())
	}
}

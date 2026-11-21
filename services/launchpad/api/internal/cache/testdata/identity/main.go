package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	owner "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(os.Getenv("CACHE_IDENTITY_DSN"))
	check(err)
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	check(err)
	defer pool.Close()
	raw, err := os.ReadFile(os.Getenv("CACHE_IDENTITY_MIGRATION"))
	check(err)
	up, _, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok {
		panic("identity migration boundary missing")
	}
	_, err = pool.Exec(ctx, up)
	check(err)
	rpc, err := ethclient.DialContext(ctx, os.Getenv("CACHE_IDENTITY_RPC"))
	check(err)
	defer rpc.Close()
	secret := make([]byte, 48)
	_, err = rand.Read(secret)
	check(err)
	check(os.Setenv("CACHE_IDENTITY_SIGNING_KEY", hex.EncodeToString(secret)))
	now := time.Now()
	tokens, err := owner.NewIdentityTokens("https://cache.example", "cache", "active", []owner.SigningKey{{ID: "active", SecretReference: "CACHE_IDENTITY_SIGNING_KEY", ActiveAt: now.Add(-time.Minute), RetireAt: now.Add(time.Hour)}}, owner.EnvironmentIdentitySecrets{}, time.Now)
	check(err)
	identity, err := owner.NewIdentity(owner.IdentityConfig{Pool: pool, Domain: "cache.example", URI: "https://cache.example/auth", AllowedOrigins: []string{"https://cache.example"}, Verifier: owner.Chain125ProofVerifier{Backend: sharedauth.NewReportedHashBackend(rpc.Client())}, LimiterKey: secret, Tokens: tokens})
	check(err)
	router := chi.NewRouter()
	serviceToken := hex.EncodeToString(secret)
	check(identity.RegisterV2Routes(router, owner.IdentityHTTPConfig{ServiceToken: serviceToken}))
	server := httptest.NewServer(router)
	defer server.Close()
	type session struct{ Wallet, Token string }
	sessions := []session{}
	for range 2 {
		key, err := crypto.GenerateKey()
		check(err)
		wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
		challenge, err := identity.IssueLogin(ctx, owner.ChallengeRequest{Wallet: wallet, Origin: "https://cache.example"}, "127.0.0.1")
		check(err)
		signature, err := crypto.Sign(accounts.TextHash([]byte(challenge.Message)), key)
		check(err)
		signature[64] += 27
		credentials, err := identity.Login(ctx, owner.LoginRequest{ChallengeID: challenge.ChallengeID, Proof: owner.WalletProof{Kind: "eoa_eip191", Wallet: wallet, Signature: "0x" + hex.EncodeToString(signature)}, DeviceLabel: "Cache integration"})
		check(err)
		principal, err := identity.AuthenticateJWT(ctx, tokens, credentials.AccessToken)
		check(err)
		sessions = append(sessions, session{principal.Wallet(), credentials.AccessToken})
	}
	check(json.NewEncoder(os.Stdout).Encode(struct {
		URL, ServiceToken string
		Sessions          []session
	}{server.URL, serviceToken, sessions}))
	_, err = io.Copy(io.Discard, os.Stdin)
	check(err)
}

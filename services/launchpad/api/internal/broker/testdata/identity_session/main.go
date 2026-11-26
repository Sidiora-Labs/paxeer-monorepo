package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func require(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(os.Getenv("SNAPSHOT_IDENTITY_DSN"))
	require(err)
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require(err)
	defer pool.Close()
	source, err := os.ReadFile(filepath.Join(os.Getenv("SNAPSHOT_REPOSITORY_ROOT"), "services/launchpad/gateway/internal/devidentity/migrations/00001_identity.sql"))
	require(err)
	up, _, found := strings.Cut(string(source), "-- +goose Down")
	if !found {
		panic("identity migration missing Down")
	}
	_, err = pool.Exec(ctx, up)
	require(err)
	rpc, err := ethclient.DialContext(ctx, os.Getenv("SNAPSHOT_CHAIN_RPC"))
	require(err)
	defer rpc.Close()
	require(os.Setenv("SNAPSHOT_IDENTITY_SIGNING_KEY", strings.Repeat("s", 48)))
	now := time.Now()
	tokens, err := owner.NewIdentityTokens("https://web.example", "snapshot-session", "snapshot", []owner.SigningKey{{ID: "snapshot", SecretReference: "SNAPSHOT_IDENTITY_SIGNING_KEY", ActiveAt: now.Add(-time.Minute), RetireAt: now.Add(time.Hour)}}, owner.EnvironmentIdentitySecrets{}, time.Now)
	require(err)
	identity, err := owner.NewIdentity(owner.IdentityConfig{Pool: pool, Domain: "web.example", URI: "https://web.example/auth", AllowedOrigins: []string{"https://web.example"}, Verifier: owner.Chain125ProofVerifier{Backend: sharedauth.NewReportedHashBackend(rpc.Client())}, LimiterKey: []byte(strings.Repeat("l", 32)), Tokens: tokens})
	require(err)
	router := chi.NewRouter()
	require(identity.RegisterV2Routes(router, owner.IdentityHTTPConfig{ServiceToken: strings.Repeat("t", 32)}))
	server := httptest.NewServer(router)
	defer server.Close()
	key, err := crypto.GenerateKey()
	require(err)
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	challenge, err := identity.IssueLogin(ctx, owner.ChallengeRequest{Wallet: wallet, Origin: "https://web.example"}, "127.0.0.1")
	require(err)
	signature, err := crypto.Sign(accounts.TextHash([]byte(challenge.Message)), key)
	require(err)
	signature[64] += 27
	credentials, err := identity.Login(ctx, owner.LoginRequest{ChallengeID: challenge.ChallengeID, Proof: owner.WalletProof{Kind: "eoa_eip191", Wallet: wallet, Signature: "0x" + hex.EncodeToString(signature)}, DeviceLabel: "snapshot session"})
	require(err)
	principal, err := identity.AuthenticateJWT(ctx, tokens, credentials.AccessToken)
	require(err)
	require(json.NewEncoder(os.Stdout).Encode(struct {
		URL       string
		Principal sharedauth.StreamPrincipal
	}{server.URL, sharedauth.StreamPrincipal{AccountID: principal.AccountID(), SessionID: principal.SessionID(), Wallet: principal.Wallet(), AuthEpoch: principal.AuthEpoch()}}))
	_, err = io.Copy(io.Discard, os.Stdin)
	require(err)
}

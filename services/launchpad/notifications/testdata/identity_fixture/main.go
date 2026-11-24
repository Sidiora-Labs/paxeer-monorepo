package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func check(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(os.Getenv("REPAIR40_DSN"))
	check(err)
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	check(err)
	defer pool.Close()
	source, err := os.ReadFile(filepath.Join(os.Getenv("REPAIR40_ROOT"), "services", "launchpad", "gateway", "internal", "devidentity", "migrations", "00001_identity.sql"))
	check(err)
	up, _, ok := strings.Cut(string(source), "-- +goose Down")
	if !ok {
		panic("missing identity migration boundary")
	}
	_, err = pool.Exec(ctx, up)
	check(err)
	rpc, err := ethclient.DialContext(ctx, os.Getenv("LOCAL_NODE_RPC"))
	check(err)
	defer rpc.Close()
	check(os.Setenv("REPAIR40_IDENTITY_SIGNING_KEY", strings.Repeat("s", 48)))
	now := time.Now()
	tokens, err := owner.NewIdentityTokens("https://kindle.test", "kindle-owner-v2", "repair40", []owner.SigningKey{{ID: "repair40", SecretReference: "REPAIR40_IDENTITY_SIGNING_KEY", ActiveAt: now.Add(-time.Minute), RetireAt: now.Add(time.Hour)}}, owner.EnvironmentIdentitySecrets{}, time.Now)
	check(err)
	identity, err := owner.NewIdentity(owner.IdentityConfig{Pool: pool, Domain: "kindle.test", URI: "https://kindle.test/auth", AllowedOrigins: []string{"https://kindle.test"}, Verifier: owner.Chain125ProofVerifier{Backend: sharedauth.NewReportedHashBackend(rpc.Client())}, LimiterKey: []byte(strings.Repeat("l", 32)), Tokens: tokens})
	check(err)
	router := chi.NewRouter()
	check(identity.RegisterV2Routes(router, owner.IdentityHTTPConfig{ServiceToken: strings.Repeat("t", 32)}))
	server := httptest.NewServer(router)
	defer server.Close()
	type session struct{ AccountID, Wallet, Token string }
	sessions := []session{}
	for i := 0; i < 2; i++ {
		key, err := crypto.GenerateKey()
		check(err)
		wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
		challenge, err := identity.IssueLogin(ctx, owner.ChallengeRequest{Wallet: wallet, Origin: "https://kindle.test"}, "127.0.0.1")
		check(err)
		signature, err := crypto.Sign(accounts.TextHash([]byte(challenge.Message)), key)
		check(err)
		signature[64] += 27
		credentials, err := identity.Login(ctx, owner.LoginRequest{ChallengeID: challenge.ChallengeID, Proof: owner.WalletProof{Kind: "eoa_eip191", Wallet: wallet, Signature: "0x" + hex.EncodeToString(signature)}, DeviceLabel: "notifications repair"})
		check(err)
		principal, err := identity.AuthenticateJWT(ctx, tokens, credentials.AccessToken)
		check(err)
		sessions = append(sessions, session{principal.AccountID(), principal.Wallet(), credentials.AccessToken})
	}
	check(json.NewEncoder(os.Stdout).Encode(struct {
		URL      string
		Sessions []session
	}{server.URL, sessions}))
	// Keep the actual owner alive until the parent closes its control pipe.
	_, err = io.Copy(io.Discard, os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/streams"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type configuration struct {
	DSN, RPC, Listen, Cert, Key, CA, GatewayPin, GatewayPublicKey string
	Identity                                                      sharedauth.MTLSIdentityClientConfig
	Wallet, Pool, VaultURL, VaultCA, VaultToken, StreamKey        string
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	var cfg configuration
	raw, err := os.ReadFile(os.Args[1])
	must(err)
	must(json.Unmarshal(raw, &cfg))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.DSN)
	must(err)
	defer pool.Close()
	store, err := streams.NewLifecycleStore(pool, "livestream", 3, 100)
	must(err)
	_, err = pool.Exec(ctx, store.ConnectedSchemaSQL())
	must(err)
	identity, err := sharedauth.NewMTLSIdentityClient(cfg.Identity)
	must(err)
	rpc, err := ethclient.DialContext(ctx, cfg.RPC)
	must(err)
	defer rpc.Close()
	verifier := &sharedauth.WalletOperationVerifier{Backend: sharedauth.NewReportedHashBackend(rpc.Client())}
	ca, err := os.ReadFile(cfg.VaultCA)
	must(err)
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		log.Fatal("invalid vault roots")
	}
	vault := &streams.VaultTransit{Endpoint: cfg.VaultURL, Token: cfg.VaultToken, Key: "repair48", Client: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13}}}}
	operation, err := store.Create(ctx, streams.CreateIntent{Wallet: cfg.Wallet, Pool: cfg.Pool, Title: "Repair ingest proof", IdempotencyKey: "repair48"}, streams.CreatorDecision{Wallet: cfg.Wallet, Pool: cfg.Pool, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}, true)
	must(err)
	_, err = store.BeginProvision(ctx, operation.ID)
	must(err)
	cipher, err := vault.Seal(ctx, operation.ID, "repair48_provider", cfg.StreamKey)
	must(err)
	operation, err = store.BindEncryptedProvider(ctx, operation.ID, "repair48_provider", cipher, vault.Key)
	must(err)
	public, err := base64.StdEncoding.DecodeString(cfg.GatewayPublicKey)
	must(err)
	assertions := sharedauth.ServiceAssertionVerifier{Issuer: "gateway", Audience: "livestream", Key: ed25519.PublicKey(public), PeerCertSHA256: cfg.GatewayPin, Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.ActorAssertionHeader, SignatureHeader: sharedauth.ActorSignatureHeader}}
	policy := &streams.RestrictionReader{Pool: pool}
	handlers := streams.New(streams.Deps{Lifecycle: store, OperationLedger: &sharedauth.Ledger{Pool: pool, Environment: "test", AuditKey: []byte(strings.Repeat("a", 32))}, OperationVerifier: verifier, Restrictions: policy, IngestKeys: &streams.EncryptedCredentials{Store: store, Transit: vault, IngestEndpoint: "rtmps://ingest.example.test/live"}, Authorize: func(ctx context.Context, r *http.Request, action string, o streams.Operation) (streams.Authorization, error) {
		principal, err := identity.AuthenticateRequest(ctx, r)
		if err != nil {
			return streams.Authorization{}, err
		}
		var creator string
		if err = pool.QueryRow(ctx, `SELECT creator FROM indexer.indexer_canonical_creations_v1 WHERE pool=$1`, o.Pool).Scan(&creator); err != nil {
			return streams.Authorization{}, err
		}
		if creator != principal.Wallet || o.Wallet != principal.Wallet {
			return streams.Authorization{}, streams.ErrDenied
		}
		allowed, err := policy.Allowed(ctx, principal.Wallet, o.Pool)
		return streams.Authorization{Wallet: principal.Wallet, SessionID: principal.SessionID, Pool: o.Pool, Allowed: allowed, ExpiresAt: principal.AccessExpiresAt}, err
	}})
	router := chi.NewRouter()
	handlers.RegisterRoutes(router)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"streamId": operation.ID})
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Verify the gateway's real mTLS actor assertion before the actual
			// owner handler performs its independent session and wallet checks.
			if _, err := assertions.Authenticate(r, []byte(`{}`)); err != nil {
				http.Error(w, "assertion denied", 403)
				return
			}
		}
		router.ServeHTTP(w, r)
	}))
	leaf, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
	must(err)
	ca, err = os.ReadFile(cfg.CA)
	must(err)
	roots = x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		log.Fatal("invalid roots")
	}
	server := &http.Server{Addr: cfg.Listen, Handler: mux, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{leaf}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}, ReadHeaderTimeout: time.Second}
	go func() {
		<-ctx.Done()
		bounded, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(bounded)
	}()
	err = server.ListenAndServeTLS("", "")
	if !errors.Is(err, http.ErrServerClosed) {
		must(err)
	}
}

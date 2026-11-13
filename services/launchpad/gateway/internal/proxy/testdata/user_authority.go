// This executable links the real user authority from its owning module. The
// gateway proof builds it in a temporary module so Go's internal boundary is
// preserved without changing the user module or copying its implementation.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

type configuration struct {
	DSN, RPC, Listen, Cert, Key, CA, GatewayPin, GatewayPublicKey string
	Identity                                                      sharedauth.MTLSIdentityClientConfig
	Objects                                                       storage.Config
	Version                                                       string
	HeaderAudit                                                   string
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
	identity, err := sharedauth.NewMTLSIdentityClient(cfg.Identity)
	must(err)
	var objects *storage.Client
	if cfg.Objects.Endpoint != "" {
		objects, err = storage.New(ctx, cfg.Objects)
		must(err)
	}
	rpc, err := ethclient.DialContext(ctx, cfg.RPC)
	must(err)
	defer rpc.Close()
	public, err := base64.StdEncoding.DecodeString(cfg.GatewayPublicKey)
	must(err)
	assertions := sharedauth.ServiceAssertionVerifier{Issuer: "gateway", Audience: "account", Key: ed25519.PublicKey(public), PeerCertSHA256: cfg.GatewayPin, Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.ActorAssertionHeader, SignatureHeader: sharedauth.ActorSignatureHeader}}
	recovery := sharedauth.ServiceRequestVerifier{Issuer: "gateway", Audience: "account", Key: ed25519.PublicKey(public), PeerCertSHA256: cfg.GatewayPin, Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}}
	policies, routes := sharedauth.AccountOwnerPoliciesV2()
	if cfg.Version == "v2" {
		for name, route := range routes {
			route.Template = strings.Replace(route.Template, "/v1/", "/v2/", 1)
			routes[name] = route
			policy := policies.Actions[name]
			policy.RouteTemplate = route.Template
			policies.Actions[name] = policy
		}
	}
	operations := sharedauth.OperationAuthorizer{Ledger: sharedauth.Ledger{Pool: pool, Environment: "test", AuditKey: bytes.Repeat([]byte{9}, 32)}, Verifier: &sharedauth.WalletOperationVerifier{Backend: sharedauth.NewReportedHashBackend(rpc.Client())}, Policies: policies, Routes: routes, Transport: sharedauth.OperationTransport{EnvelopeHeader: "X-Kindle-Operation", SignatureHeader: "X-Kindle-Operation-Signature"}}
	subjects, allowed, kinds := map[string]string{}, map[string]bool{}, map[string]bool{}
	for _, kind := range []string{"account.export.requested", "account.deletion.requested", "account.deletion.job"} {
		subjects[kind] = "users." + kind
		allowed[subjects[kind]] = true
		kinds[kind] = true
	}
	publisher, err := delivery.NewStore(pool, "gateway_identity", delivery.Policy{Subjects: allowed, EventTypes: kinds})
	must(err)
	runtime := &account.Runtime{RecoveryGatewayVerifier: &recovery, IdentityAuthority: identity, Authorizer: &operations, Objects: objects, ExportKey: bytes.Repeat([]byte{4}, 32), EndpointKey: bytes.Repeat([]byte{7}, 32), DownloadBaseURL: "https://kindle.test", Delivery: publisher, Subjects: subjects}
	store := account.Store{DB: pool}
	core := (account.HTTP{Store: store, Runtime: runtime}).GatewayHandler(assertions, sharedauth.PostgresIdentityResolver{Pool: pool}, operations)
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, err := os.OpenFile(cfg.HeaderAudit, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		must(err)
		must(json.NewEncoder(file).Encode(map[string]string{"path": r.URL.Path, "export": r.Header.Get("X-Export-Authorization"), "authorization": r.Header.Get("Authorization"), "cookie": r.Header.Get("Cookie")}))
		must(file.Close())
		core.ServeHTTP(w, r)
	}))
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if pool.Ping(r.Context()) != nil {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	})
	leaf, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
	must(err)
	roots, err := os.ReadFile(cfg.CA)
	must(err)
	tlsConfig := identity.Client.Transport.(*http.Transport).TLSClientConfig.Clone()
	if !tlsConfig.RootCAs.AppendCertsFromPEM(roots) {
		log.Fatal("invalid root")
	}
	tlsConfig.Certificates = []tls.Certificate{leaf}
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.ClientCAs = tlsConfig.RootCAs
	server := &http.Server{Addr: cfg.Listen, Handler: mux, TLSConfig: tlsConfig, ReadHeaderTimeout: time.Second}
	go func() {
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		worker := account.Worker{Store: store, Runtime: runtime, Owner: "repair48", LeaseDuration: 10 * time.Second}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var pending bool
				if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_job WHERE kind='export' AND state='requested')`).Scan(&pending); err != nil {
					log.Print(err)
					continue
				}
				if pending {
					if err := worker.RunOnce(ctx); err != nil {
						log.Print(fmt.Errorf("export worker: %w", err))
					}
				}
			}
		}
	}()
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

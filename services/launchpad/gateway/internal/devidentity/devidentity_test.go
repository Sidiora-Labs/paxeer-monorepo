package devidentity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/jackc/pgx/v5"
)

func validOptions() Options {
	return Options{
		DatabaseURL:        "postgres://gateway@localhost/gateway?sslmode=disable",
		RPCURL:             "http://localhost:8545",
		PublicURL:          "https://web.example",
		CORSAllowedOrigins: "https://web.example,https://app.example",
		SigningSecret:      strings.Repeat("s", 32),
		LimiterKey:         strings.Repeat("l", 32),
		ServiceToken:       strings.Repeat("t", 32),
	}
}

func TestOptionsValidate(t *testing.T) {
	valid := validOptions()
	origins, domain, err := valid.Validate()
	if err != nil || domain != "web.example" || len(origins) != 2 || origins[0] != valid.PublicURL || origins[1] != "https://app.example" {
		t.Fatalf("valid options: origins=%v domain=%q err=%v", origins, domain, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Options)
	}{
		{"database", func(o *Options) { o.DatabaseURL = " " }},
		{"rpc", func(o *Options) { o.RPCURL = " " }},
		{"signing secret", func(o *Options) { o.SigningSecret = "short" }},
		{"limiter key", func(o *Options) { o.LimiterKey = "short" }},
		{"service token", func(o *Options) { o.ServiceToken = "short" }},
		{"plain HTTP public URL", func(o *Options) { o.PublicURL = "http://web.example" }},
		{"public URL path", func(o *Options) { o.PublicURL += "/app" }},
		{"public URL query", func(o *Options) { o.PublicURL += "?a=1" }},
		{"public URL user", func(o *Options) { o.PublicURL = "https://user@web.example" }},
		{"missing public origin", func(o *Options) { o.CORSAllowedOrigins = "https://app.example" }},
		{"duplicate origin", func(o *Options) { o.CORSAllowedOrigins += ",https://web.example" }},
		{"origin path", func(o *Options) { o.CORSAllowedOrigins += ",https://other.example/app" }},
		{"plain HTTP origin", func(o *Options) { o.CORSAllowedOrigins += ",http://other.example" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := valid
			tc.change(&o)
			if _, _, err := o.Validate(); err == nil {
				t.Fatal("invalid options accepted")
			}
		})
	}
}

func TestKeyIDAndPoolConfiguration(t *testing.T) {
	secret := strings.Repeat("x", 32)
	digest := sha256.Sum256([]byte(secret))
	if got, want := KeyID(secret), "dev-"+hex.EncodeToString(digest[:8]); got != want {
		t.Fatalf("KeyID = %q, want %q", got, want)
	}
	if KeyID(secret) == KeyID(secret+"y") {
		t.Fatal("rotating the signing secret retained its key id")
	}
	if _, err := poolConfig("not a database URL"); err == nil {
		t.Fatal("malformed database URL accepted")
	}
	cfg, err := poolConfig(validOptions().DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.RuntimeParams["search_path"] != Schema || cfg.MaxConns != 8 {
		t.Fatalf("identity pool configuration: search_path=%q max=%d", cfg.ConnConfig.RuntimeParams["search_path"], cfg.MaxConns)
	}
}

func TestOpenRejectsInvalidOptionsBeforeConnecting(t *testing.T) {
	if runtime, err := Open(context.Background(), Options{}); err == nil || runtime != nil {
		t.Fatalf("Open invalid options: runtime=%v err=%v", runtime, err)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if runtime, err := Open(canceled, validOptions()); err == nil || runtime != nil {
		t.Fatalf("Open canceled database: runtime=%v err=%v", runtime, err)
	}
	if pool, err := NewPool(context.Background(), "not a database URL"); err == nil || pool != nil {
		t.Fatalf("pool with invalid database URL: pool=%v err=%v", pool, err)
	}
	if pool, err := NewPool(canceled, validOptions().DatabaseURL); err == nil || pool != nil {
		t.Fatalf("pool with canceled database request: pool=%v err=%v", pool, err)
	}
	if err := Migrate(context.Background(), "not a database URL"); err == nil {
		t.Fatal("migration accepted an invalid database URL")
	}
	if err := Migrate(canceled, validOptions().DatabaseURL); err == nil {
		t.Fatal("migration connected after cancellation")
	}
}

func TestOpenBuildsIdentityOnOwnedDatabaseAndChain(t *testing.T) {
	adminDSN := os.Getenv("L33_POSTGRES_DSN")
	if adminDSN == "" {
		t.Fatal("L33_POSTGRES_DSN is required for the gateway identity fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	name := fmt.Sprintf("gateway_identity_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanupCtx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	address, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	address.Path = "/" + name
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(125)
	chain := simulated.NewBackend(types.GenesisAlloc{}, func(nodeConfig *node.Config, ethConfig *ethconfig.Config) {
		nodeConfig.HTTPHost = "127.0.0.1"
		nodeConfig.HTTPPort = port
		nodeConfig.HTTPModules = []string{"eth", "net", "web3"}
		ethConfig.Genesis.Config = &chainConfig
		ethConfig.NetworkId = 125
	})
	defer func() { _ = chain.Close() }()
	chain.Commit()
	secret := strings.Repeat("s", 32)
	t.Setenv(SigningSecretReference, secret)
	options := validOptions()
	options.DatabaseURL = address.String()
	options.RPCURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	options.CORSAllowedOrigins = options.PublicURL
	t.Setenv(SigningSecretReference, "")
	if runtime, err := Open(ctx, options); err == nil || runtime != nil || !strings.Contains(err.Error(), SigningSecretReference) {
		t.Fatalf("identity signing source: runtime=%v err=%v", runtime, err)
	}
	t.Setenv(SigningSecretReference, secret)
	invalidRPC := options
	invalidRPC.RPCURL = "://invalid"
	if runtime, err := Open(ctx, invalidRPC); err == nil || runtime != nil || !strings.Contains(err.Error(), "development identity rpc") {
		t.Fatalf("identity chain connection: runtime=%v err=%v", runtime, err)
	}
	options.Observe = func(string) {}
	runtime, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if runtime.Identity == nil || runtime.Pool == nil || runtime.RPC == nil {
		t.Fatal("identity runtime resources are incomplete")
	}
	var schema string
	if err := runtime.Pool.QueryRow(ctx, "SELECT current_schema()").Scan(&schema); err != nil || schema != Schema {
		t.Fatalf("identity schema = %q, error = %v", schema, err)
	}
	var versions int
	if err := runtime.Pool.QueryRow(ctx, "SELECT count(*) FROM goose_db_version").Scan(&versions); err != nil || versions == 0 {
		t.Fatalf("identity migrations = %d, error = %v", versions, err)
	}
	(&Runtime{}).Close()
}

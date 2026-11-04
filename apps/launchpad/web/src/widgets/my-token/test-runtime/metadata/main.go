package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/migrations"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type coin struct{ Pool, Token, Creator, Name, Symbol string }
type config struct {
	DatabaseURL, RedisURL, Directory string
	Coins                            []coin
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	var cfg config
	raw, err := os.ReadFile(os.Args[1])
	must(err)
	must(json.Unmarshal(raw, &cfg))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	must(err)
	defer db.Close()
	must(db.Ping(ctx))
	for _, name := range []string{"00001_init.sql", "00002_add_decimals.sql"} {
		source, err := migrations.FS.ReadFile(name)
		must(err)
		_, err = db.Exec(ctx, strings.Split(string(source), "-- +goose Down")[0])
		must(err)
	}
	_, err = db.Exec(ctx, "CREATE SCHEMA indexer; CREATE TABLE indexer.pools (pool_address text PRIMARY KEY, token_address text NOT NULL, creator text NOT NULL)")
	must(err)
	queries := sqlcdb.New(db)
	for _, entry := range cfg.Coins {
		pool, token, creator := strings.ToLower(entry.Pool), strings.ToLower(entry.Token), strings.ToLower(entry.Creator)
		_, err = db.Exec(ctx, "INSERT INTO indexer.pools VALUES ($1,$2,$3)", pool, token, creator)
		must(err)
		must(queries.UpsertTokenMetadata(ctx, sqlcdb.UpsertTokenMetadataParams{TokenAddress: token, PoolAddress: pool, Name: pgtype.Text{String: entry.Name, Valid: true}, Symbol: pgtype.Text{String: entry.Symbol, Valid: true}, CustomTags: pgtype.Text{String: "[]", Valid: true}, CreatedBy: creator, CreatedAt: time.Now().UnixMilli(), UpdatedAt: time.Now().UnixMilli(), Decimals: 6}))
	}
	options, err := redis.ParseURL(cfg.RedisURL)
	must(err)
	cache := redis.NewClient(options)
	defer cache.Close()
	must(cache.Ping(ctx).Err())
	router := chi.NewRouter()
	metadata.New(metadata.Deps{Queries: queries, Redis: cache, PublicURL: "https://metadata.test", Logger: slog.Default()}).RegisterRoutes(router)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "my-token-private-metadata"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	must(err)
	private, err := x509.MarshalECPrivateKey(key)
	must(err)
	certPEM, keyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: private})
	must(os.WriteFile(filepath.Join(cfg.Directory, "cert.pem"), certPEM, 0600))
	must(os.WriteFile(filepath.Join(cfg.Directory, "key.pem"), keyPEM, 0600))
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	must(err)
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		panic("certificate unavailable")
	}
	server := httptest.NewUnstartedServer(router)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	server.StartTLS()
	defer server.Close()
	fmt.Println(server.URL)
	<-ctx.Done()
}

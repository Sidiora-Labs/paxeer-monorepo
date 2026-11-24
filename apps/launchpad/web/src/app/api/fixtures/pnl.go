//go:build ignore

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/devreads"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	var input struct {
		Database string `json:"database"`
		RPC      string `json:"rpc"`
		Identity string `json:"identity"`
		Token    string `json:"token"`
		FontDir  string `json:"fontDir"`
		Output   string `json:"output"`
	}
	must(json.NewDecoder(os.Stdin).Decode(&input))
	config, err := pgxpool.ParseConfig(input.Database)
	must(err)
	admin, err := pgxpool.NewWithConfig(ctx, config)
	must(err)
	defer admin.Close()
	schema := "web_shares_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	must(err)
	defer func() { _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); must(err) }()
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	must(err)
	defer pool.Close()
	for _, ddl := range []string{l17.DDL, l17.MetadataDDL, l17.SharesV3DDL} {
		_, err = pool.Exec(ctx, ddl)
		must(err)
	}
	chain, err := devreads.DialChain(ctx, input.RPC)
	must(err)
	defer chain.Close()
	block, err := chain.Head(ctx)
	must(err)
	key, err := crypto.GenerateKey()
	must(err)
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	rpc, err := ethclient.DialContext(ctx, input.RPC)
	must(err)
	defer rpc.Close()
	balance, err := rpc.BalanceAt(ctx, crypto.PubkeyToAddress(key.PublicKey), nil)
	must(err)
	if balance.Sign() != 0 {
		panic("Empty wallet required")
	}
	repository := l17.Repository{Pool: pool}
	summary, err := repository.CommitSnapshot(ctx, uuid.NewString(), l17.Summary{
		Wallet: wallet,
		Watermark: l17.Watermark{ChainID: "125", Height: fmt.Sprint(block.Number), BlockHash: block.Hash,
			ProjectionRevision: "1", ObservedAt: block.Timestamp.Format("2006-01-02T15:04:05.000Z"), Integrity: "healthy"},
	}, nil, time.Now().Add(time.Hour))
	must(err)
	chainID, currency, quote, decimals := "125", "USDL", "0x85FcD13735F4309833A503EE804ea32395851479", 6
	tx, err := pool.Begin(ctx)
	must(err)
	must(repository.CommitHistoryMetadataTx(ctx, tx, wallet, summary.SnapshotID, l17.SnapshotHistoryMetadata{
		SourceEvent: uuid.NewString(), Start: block.Timestamp.Add(-time.Second), End: block.Timestamp,
		Quote: l17.AssetRef{Kind: "erc20", ChainID: &chainID, Address: &quote, CurrencyCode: &currency, Decimals: &decimals},
	}))
	must(tx.Commit(ctx))
	authority := auth.HTTPIdentityAuthority{Endpoint: input.Identity, ServiceToken: input.Token, AllowPlainHTTP: true}
	secret := make([]byte, 32)
	_, err = rand.Read(secret)
	must(err)
	release := sha256.Sum256([]byte(summary.SnapshotID))
	service, err := l17.NewSharesV3(repository, &authority, secret, "private_fixture", hex.EncodeToString(release[:]), hex.EncodeToString(release[:]), time.Now)
	must(err)
	now := time.Now()
	principal := auth.IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: wallet,
		AuthEpoch: 1, ProofAt: now, AccessExpiresAt: now.Add(time.Hour)}
	worker := l17.ShareWorkerV3{Repository: repository, Owner: "web-shares", FontDir: input.FontDir}
	ids := make([]string, 2)
	for i := range ids {
		preview, err := service.Preview(ctx, principal, l17.ShareSelectionV3{
			SnapshotID: summary.SnapshotID, Fields: []string{"knownRealizedSubtotalUsdlRaw"}, ExpiresAt: now.Add(time.Hour),
		})
		must(err)
		_, err = service.Publish(ctx, principal, l17.SharePublishV3{ID: preview.ID, Intent: preview.Intent}, uuid.NewString())
		must(err)
		must(worker.RunOnce(ctx))
		ids[i] = preview.ID
	}
	_, image, err := service.Public(ctx, ids[0])
	must(err)
	digest := sha256.Sum256(image)
	_, err = service.Revoke(ctx, principal, ids[1])
	must(err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)
	router := chi.NewRouter()
	service.RegisterHTTP(router)
	server := &http.Server{Handler: router, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	ready, err := json.Marshal(map[string]any{"origin": "http://" + listener.Addr().String(), "id": ids[0],
		"revokedId": ids[1], "imageSha256": hex.EncodeToString(digest[:]), "imageBytes": len(image)})
	must(err)
	must(os.WriteFile(filepath.Join(input.Output, "ready.json"), ready, 0600))
	<-ctx.Done()
	must(server.Close())
}

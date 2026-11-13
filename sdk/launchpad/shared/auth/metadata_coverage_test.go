package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestMetadataNonceBindsSignedOperation(t *testing.T) {
	pool, schema := phase2AuthDatabase(t)
	ctx := context.Background()
	endpoint := os.Getenv("L34_ANVIL_URL")
	if endpoint == "" {
		t.Fatal("owned chain 125 is required")
	}
	backend, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	table := pgx.Identifier{schema, "metadata_nonces"}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE TABLE "+table+"(wallet text NOT NULL,nonce text NOT NULL,operation text NOT NULL,digest text NOT NULL,PRIMARY KEY(wallet,nonce))"); err != nil {
		t.Fatal(err)
	}
	verifier := MetadataV2Verifier{Backend: NewReportedHashBackend(backend.Client()), Audience: "metadata", Clock: time.Now, NonceTable: schema + ".metadata_nonces"}
	action := MetadataV2Action{Wallet: wallet, OperationID: uuid.NewString(), Method: "PATCH", Path: "/v2/tokens/" + wallet + "/metadata", BodyHash: strings.Repeat("a", 64), Nonce: strings.Repeat("b", 64), ExpiresAt: strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10), Audience: "metadata", ChainID: 125}
	sign := func(a *MetadataV2Action) {
		t.Helper()
		signature, err := crypto.Sign(accounts.TextHash([]byte(MetadataV2SigningMessage(*a))), key)
		if err != nil {
			t.Fatal(err)
		}
		signature[64] += 27
		a.Signature = hexutil.Encode(signature)
	}
	sign(&action)
	invalidTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer invalidTx.Rollback(ctx)
	for _, tc := range []struct {
		name string
		edit func(*MetadataV2Action)
	}{
		{"chain", func(a *MetadataV2Action) { a.ChainID = 1 }},
		{"wallet", func(a *MetadataV2Action) { a.Wallet = "invalid" }},
		{"target", func(a *MetadataV2Action) { a.Path = "/v2/tokens/invalid/metadata" }},
		{"method", func(a *MetadataV2Action) { a.Method = "POST" }},

		{"body digest", func(a *MetadataV2Action) { a.BodyHash = "not-a-digest" }},
		{"expired", func(a *MetadataV2Action) { a.ExpiresAt = "0" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := action
			tc.edit(&changed)
			if err := verifier.AuthorizeInTx(ctx, invalidTx, changed); !errors.Is(err, ErrScopeDenied) {
				t.Fatalf("invalid metadata action: %v", err)
			}
		})
	}

	for _, table := range []string{"one.two.three", "invalid table"} {
		invalid := verifier
		invalid.NonceTable = table
		if err := invalid.AuthorizeInTx(ctx, invalidTx, action); !errors.Is(err, ErrScopeDenied) {
			t.Fatalf("nonce table admission: %v", err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := verifier.AuthorizeInTx(cancelled, invalidTx, action); err == nil {
		t.Fatal("cancelled metadata authorization admitted")
	}
	invalidAction := action
	invalidAction.Signature = "invalid"
	if err := verifier.AuthorizeInTx(ctx, invalidTx, invalidAction); err == nil {
		t.Fatal("invalid metadata signature admitted")
	}
	for i := 0; i < 2; i++ {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := verifier.AuthorizeInTx(ctx, tx, action); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("signed metadata operation %d: %v", i, err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	statement := MetadataV2SigningMessage(action)
	digest := sha256.Sum256([]byte(statement))
	var operation, recorded string
	if err := pool.QueryRow(ctx, "SELECT operation,digest FROM "+table+" WHERE wallet=$1 AND nonce=$2", wallet, action.Nonce).Scan(&operation, &recorded); err != nil || operation != action.OperationID || recorded != hex.EncodeToString(digest[:]) {
		t.Fatalf("durable metadata nonce binding: operation=%s digest=%s err=%v", operation, recorded, err)
	}
	changed := action
	changed.OperationID = uuid.NewString()
	sign(&changed)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := verifier.AuthorizeInTx(ctx, tx, changed); !errors.Is(err, ErrReplayMismatch) {
		t.Fatalf("changed operation reused nonce: %v", err)
	}
}

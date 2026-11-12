package analytics_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestReactionOperationBindsSignedIntentAndTransaction(t *testing.T) {
	authority := phase2Authority(t)
	ctx := context.Background()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	intent := analytics.Intent{ChainID: "125", Pool: "0x" + strings.Repeat("7", 40), Wallet: wallet, Reaction: "hot", Nonce: strings.Repeat("a", 64), Expiry: time.Now().Add(2 * time.Minute).Unix()}
	body, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := auth.BodyDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	envelope := auth.OperationEnvelope{Version: 1, ChainID: "125", Environment: "qualification", Actor: wallet, Action: "reaction.replace", Method: "POST", Path: "/v2/stats/" + intent.Pool + "/reactions", BodyHash: digest, Nonce: intent.Nonce, IssuedAt: time.Now().Unix(), ExpiresAt: intent.Expiry}
	statement, err := envelope.Statement()
	if err != nil {
		t.Fatal(err)
	}
	signature, err := crypto.Sign(accounts.TextHash([]byte(statement)), key)
	if err != nil {
		t.Fatal(err)
	}
	req := auth.OperationRequest{Envelope: envelope, Signature: "0x" + hex.EncodeToString(signature), Body: body, Actor: wallet, Action: envelope.Action, Method: envelope.Method, Path: envelope.Path}
	verify := func(_ context.Context, actor, signed, provided string) error {
		if !auth.VerifyWalletSignature(actor, signed, provided) {
			return auth.ErrScopeDenied
		}
		return nil
	}
	tx, err := authority.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := authority.ApplyOperationTx(ctx, tx, intent, req, "qualification", verify)
	if err != nil || result.Counts["hot"] != 1 || result.Revision != 1 {
		tx.Rollback(ctx)
		t.Fatalf("signed operation: %+v %v", result, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	public, err := authority.ReadReactions(ctx, intent.Pool)
	if err != nil || public.Counts["hot"] != 1 {
		t.Fatalf("committed reaction: %+v %v", public, err)
	}
	replayTx, err := authority.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := authority.ApplyOperationTx(ctx, replayTx, intent, req, "qualification", verify)
	if err != nil || replayed.Revision != result.Revision {
		replayTx.Rollback(ctx)
		t.Fatalf("idempotent operation: %+v %v", replayed, err)
	}
	if err := replayTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	wrong := req
	wrong.Path = "/v2/stats/" + strings.Repeat("0", 40) + "/reactions"
	badTx, err := authority.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer badTx.Rollback(ctx)
	if _, err := authority.ApplyOperationTx(ctx, badTx, intent, wrong, "qualification", verify); !errors.Is(err, auth.ErrScopeDenied) {
		t.Fatalf("changed route accepted: %v", err)
	}
	if _, err := authority.ApplyOperationTx(ctx, nil, intent, req, "qualification", verify); !errors.Is(err, auth.ErrScopeDenied) {
		t.Fatalf("missing transaction accepted: %v", err)
	}
}

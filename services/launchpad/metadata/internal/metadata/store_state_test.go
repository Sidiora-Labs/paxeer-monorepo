package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSignedStoreStateBoundaries(t *testing.T) {
	ctx := context.Background()
	pool := edgePool(t, "")
	schema := edgeSchema(t, pool, "store_state")
	store, err := NewProductionStore(pool, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()); err != nil {
		t.Fatal(err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	token := "0x" + strings.Repeat("a", 40)
	foreign := "0x" + strings.Repeat("b", 40)
	const audience = "https://metadata.example"
	now := time.Now().UTC()
	store.Verifier = V2SignedVerifier{Store: store, Audience: audience, Clock: func() time.Time { return now }}
	sign := func(raw []byte, target, version string) VerifiedAction {
		t.Helper()
		sum := sha256.Sum256(raw)
		operation := uuid.NewString()
		nonce := sha256.Sum256([]byte(operation))
		a := VerifiedAction{Wallet: wallet, OperationID: operation, Method: http.MethodPatch, Path: "/" + version + "/tokens/" + target + "/metadata", BodyHash: hex.EncodeToString(sum[:]), Nonce: hex.EncodeToString(nonce[:]), ExpiresAt: strconv.FormatInt(now.Add(time.Minute).Unix(), 10), Audience: audience, ChainID: 125}
		signature, err := crypto.Sign(accounts.TextHash([]byte(V2SigningMessage(a))), key)
		if err != nil {
			t.Fatal(err)
		}
		a.Signature = "0x" + hex.EncodeToString(signature)
		return a
	}
	v2raw := []byte(`{"expectedRevision":"0","name":"Creator name"}`)
	legacyRaw := []byte(`{"expectedRevision":"0","description":"Creator text"}`)
	for _, tc := range []struct {
		name    string
		target  string
		prepare func()
		want    error
	}{
		{"unknown token", foreign, func() {}, ErrMissing},
		{"foreign creator", token, func() {
			if _, err := pool.Exec(ctx, "UPDATE "+store.table("pools")+" SET creator=$2 WHERE token=$1", token, foreign); err != nil {
				t.Fatal(err)
			}
		}, ErrForbidden},
	} {
		if tc.name == "foreign creator" {
			if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$2,$1)", token, wallet); err != nil {
				t.Fatal(err)
			}
		}
		tc.prepare()
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.ApplyV2(ctx, tc.target, v2raw, sign(v2raw, tc.target, "v2"), now); !errors.Is(err, tc.want) {
				t.Fatalf("V2 state = %v, want %v", err, tc.want)
			}
			if _, err := store.Apply(ctx, tc.target, legacyRaw, sign(legacyRaw, tc.target, "v1")); !errors.Is(err, tc.want) {
				t.Fatalf("legacy state = %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("pools")+" SET creator=$2 WHERE token=$1", token, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("documents")+"(pool,blocked) VALUES($1,true)", token); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyV2(ctx, token, v2raw, sign(v2raw, token, "v2"), now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("blocked V2 write = %v", err)
	}
	if _, err := store.Apply(ctx, token, legacyRaw, sign(legacyRaw, token, "v1")); !errors.Is(err, ErrForbidden) {
		t.Fatalf("blocked legacy write = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("documents")+" SET blocked=false WHERE pool=$1", token); err != nil {
		t.Fatal(err)
	}
	logo := uuid.NewString()
	v2WithLogo := []byte(`{"expectedRevision":"0","logoAssetId":"` + logo + `"}`)
	legacyWithLogo := []byte(`{"expectedRevision":"0","logoAssetId":"` + logo + `"}`)
	if _, err := store.ApplyV2(ctx, token, v2WithLogo, sign(v2WithLogo, token, "v2"), now); !errors.Is(err, ErrMissing) {
		t.Fatalf("V2 missing logo = %v", err)
	}
	if _, err := store.Apply(ctx, token, legacyWithLogo, sign(legacyWithLogo, token, "v1")); !errors.Is(err, ErrMissing) {
		t.Fatalf("legacy missing logo = %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("assets")+"(id,pool,state,hash,size,object_key,scanner_engine,scanner_revision) VALUES($1,$2,'REJECTED',$3,10,$4,'','')", logo, token, strings.Repeat("0", 64), "quarantine/"+logo); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyV2(ctx, token, v2WithLogo, sign(v2WithLogo, token, "v2"), now); !errors.Is(err, ErrMissing) {
		t.Fatalf("V2 rejected logo = %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("v2_documents")+"(pool,document) VALUES($1,'\"malformed\"'::jsonb)", token); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadV2(ctx, token); !errors.Is(err, ErrUnrepresentable) {
		t.Fatalf("malformed V2 document = %v", err)
	}
	undefinedTable := func(err error) bool {
		var databaseError *pgconn.PgError
		return errors.As(err, &databaseError) && databaseError.Code == "42P01"
	}
	for _, tc := range []struct {
		name  string
		calls []func() error
	}{
		{"pools", []func() error{
			func() error { _, err := store.ReadV2(ctx, token); return err },
			func() error { _, err := store.ApplyV2(ctx, token, v2raw, sign(v2raw, token, "v2"), now); return err },
			func() error { _, err := store.Apply(ctx, token, legacyRaw, sign(legacyRaw, token, "v1")); return err },
			func() error { return store.Restrict(ctx, token, "1", false) },
		}},
		{"documents", []func() error{
			func() error { _, err := store.ReadV2(ctx, token); return err },
			func() error { _, err := store.ApplyV2(ctx, token, v2raw, sign(v2raw, token, "v2"), now); return err },
			func() error { _, err := store.Apply(ctx, token, legacyRaw, sign(legacyRaw, token, "v1")); return err },
			func() error { return store.Restrict(ctx, token, "1", false) },
		}},
		{"v2_documents", []func() error{
			func() error { _, err := store.ReadV2(ctx, token); return err },
			func() error { _, err := store.ApplyV2(ctx, token, v2raw, sign(v2raw, token, "v2"), now); return err },
		}},
	} {
		t.Run(tc.name+" unavailable", func(t *testing.T) {
			original := store.table(tc.name)
			hidden := tc.name + "_hidden"
			if _, err := pool.Exec(ctx, "ALTER TABLE "+original+" RENAME TO "+hidden); err != nil {
				t.Fatal(err)
			}
			for _, call := range tc.calls {
				if err := call(); !undefinedTable(err) {
					t.Fatalf("missing %s = %v", tc.name, err)
				}
			}
			if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table(hidden)+" RENAME TO "+tc.name); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_documents")+` SET document='{"description":"","tags":[],"socials":[]}'::jsonb WHERE pool=$1`, token); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("v2_outbox")+" RENAME TO v2_outbox_hidden"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyV2(ctx, token, v2raw, sign(v2raw, token, "v2"), now); !undefinedTable(err) {
		t.Fatalf("V2 write without outbox = %v", err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("v2_outbox_hidden")+" RENAME TO v2_outbox"); err != nil {
		t.Fatal(err)
	}
	var revision string
	if err := pool.QueryRow(ctx, "SELECT revision::text FROM "+store.table("v2_documents")+" WHERE pool=$1", token).Scan(&revision); err != nil || revision != "0" {
		t.Fatalf("V2 revision after unavailable outbox = %q, %v", revision, err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE "+store.table("v2_operations")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyV2(ctx, token, v2raw, sign(v2raw, token, "v2"), now); err == nil {
		t.Fatal("V2 write accepted without operation ledger")
	}
	if _, err := pool.Exec(ctx, "DROP TABLE "+store.table("operations")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Apply(ctx, token, legacyRaw, sign(legacyRaw, token, "v1")); err == nil {
		t.Fatal("legacy write accepted without operation ledger")
	}
}

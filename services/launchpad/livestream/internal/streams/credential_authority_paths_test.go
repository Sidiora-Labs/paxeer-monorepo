package streams

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestEncryptedCredentialsRejectUnboundVaultAndProviderState(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	o, _ := f.create("credential-authority")
	if _, err := f.store.BeginProvision(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	cipher, err := f.transit.Seal(ctx, o.ID, "provider_1", "stream_key_1")
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.store.BindEncryptedProvider(ctx, o.ID, "provider_1", cipher, f.transit.Key)
	if err != nil {
		t.Fatal(err)
	}
	keys := &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "http://rtmp.livepeer.studio/live"}
	if _, err := keys.LoadKey(ctx, o); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("plaintext ingest endpoint = %v", err)
	}
	keys.IngestEndpoint = "rtmps://rtmp.livepeer.studio/live"
	wrongKey := *f.transit
	wrongKey.Key = "different_key"
	keys.Transit = &wrongKey
	if _, err := keys.LoadKey(ctx, o); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("different transit key = %v", err)
	}
	if err := keys.Rewrap(ctx, o.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("different rewrap key = %v", err)
	}
	wrongToken := *f.transit
	wrongToken.Token = "wrong-token-value"
	keys.Transit = &wrongToken
	if _, err := keys.LoadKey(ctx, o); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unauthorized transit read = %v", err)
	}
	if err := keys.Rewrap(ctx, o.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unauthorized transit rewrap = %v", err)
	}
	keys.Transit = f.transit
	if err := keys.Rewrap(ctx, "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid rewrap identity = %v", err)
	}
	if err := keys.Rewrap(ctx, "11111111-1111-4111-8111-111111111111"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("absent credential = %v", err)
	}
	if _, err := f.store.BindEncryptedProvider(ctx, o.ID, "provider_1", "not-ciphertext", f.transit.Key); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbound ciphertext = %v", err)
	}
	if _, err := f.transit.Seal(ctx, "invalid", "provider_1", "stream_key_1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid encryption context = %v", err)
	}
	if _, err := f.transit.Open(ctx, o.ID, "provider_1", "not-ciphertext"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid decrypt ciphertext = %v", err)
	}
}

func TestReplacementIntentRequiresOwnerRevisionAndEligibleState(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.RotationSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	o, _ := f.create("rotation-boundaries")
	if _, err := f.store.RequestRotationTx(ctx, nil, o.ID, f.wallet, o.Revision); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing transaction = %v", err)
	}
	for _, tc := range []struct {
		wallet   string
		revision int64
		restore  bool
		want     error
	}{
		{"0x1111111111111111111111111111111111111111", o.Revision, false, ErrDenied},
		{f.wallet, o.Revision + 1, false, ErrConflict},
		{f.wallet, o.Revision, false, ErrConflict},
		{f.wallet, o.Revision, true, ErrConflict},
	} {
		tx, err := f.database.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var result Operation
		if tc.restore {
			result, err = f.store.RequestRestorationTx(ctx, tx, o.ID, tc.wallet, tc.revision)
		} else {
			result, err = f.store.RequestRotationTx(ctx, tx, o.ID, tc.wallet, tc.revision)
		}
		if !errors.Is(err, tc.want) || result.ID != "" {
			t.Fatalf("replacement state = %+v, %v", result, err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

package streams

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestViewerTokenBindsSessionPolicyAndExpiry(t *testing.T) {
	const stream = "11111111-1111-4111-8111-111111111111"
	const session = "22222222-2222-4222-8222-222222222222"
	authority := &ViewerTokenAuthority{Key: []byte(strings.Repeat("v", 32))}
	now := time.Now().UTC()
	token, expiry, err := authority.IssuePolicyBound(stream, session, 4, now)
	if err != nil || expiry.Before(now.Add(14*time.Minute)) || authority.VerifyPolicyBound(token, stream, session, 4, now) != nil {
		t.Fatalf("issued viewer authority = %q %s, %v", token, expiry, err)
	}
	for _, changed := range []struct {
		token, stream, session string
		revision               int64
		at                     time.Time
	}{
		{token, stream, session, 5, now},
		{token, stream, session, 4, expiry.Add(time.Second)},
		{token, "33333333-3333-4333-8333-333333333333", session, 4, now},
		{token, stream, "33333333-3333-4333-8333-333333333333", 4, now},
		{"broken", stream, session, 4, now},
		{"%%%.%%%", stream, session, 4, now},
		{base64.RawURLEncoding.EncodeToString([]byte("malformed")) + ".%%", stream, session, 4, now},
	} {
		if err := authority.VerifyPolicyBound(changed.token, changed.stream, changed.session, changed.revision, changed.at); !errors.Is(err, ErrDenied) {
			t.Fatalf("unbound viewer authority = %v", err)
		}
	}
}

func TestVaultTransitRequiresBoundCiphertextAndPlaintext(t *testing.T) {
	const operation = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"provider unavailable", `{}`, http.StatusServiceUnavailable},
		{"malformed response", `{`, http.StatusOK},
		{"missing ciphertext", `{"data":{"ciphertext":"not-a-cipher"}}`, http.StatusOK},
		{"oversized response", `{"data":{"ciphertext":"` + string(make([]byte, 16384)) + `"}}`, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/transit/encrypt/key_1" || r.Header.Get("X-Vault-Token") != "fixture-transit-token" {
					t.Errorf("transit request = %s, token=%q", r.URL.Path, r.Header.Get("X-Vault-Token"))
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			vault := &VaultTransit{Endpoint: server.URL, Token: "fixture-transit-token", Key: "key_1", Client: server.Client()}
			if _, err := vault.Seal(t.Context(), operation, "provider_1", "stream_key_1"); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("unbound transit ciphertext = %v", err)
			}
		})
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/transit/decrypt/key_1" {
			t.Errorf("transit decrypt route = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"plaintext":"!!!"}}`))
	}))
	defer server.Close()
	vault := &VaultTransit{Endpoint: server.URL, Token: "fixture-transit-token", Key: "key_1", Client: server.Client()}
	if _, err := vault.Open(t.Context(), operation, "provider_1", "vault:v1:YQ=="); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound plaintext = %v", err)
	}
}

func TestEncryptedCredentialRewrapPreservesProviderBinding(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	o, _ := f.create("credential-rewrap")
	if _, err := f.store.BeginProvision(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	const provider = "rewrap_provider"
	const secret = "rewrap_stream_key"
	cipher, err := f.transit.Seal(ctx, o.ID, provider, secret)
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.store.BindEncryptedProvider(ctx, o.ID, provider, cipher, f.transit.Key)
	if err != nil {
		t.Fatal(err)
	}
	keys := &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"}
	if err := keys.Rewrap(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	var current string
	if err := f.database.QueryRow(ctx, "SELECT ciphertext FROM livestream.stream_credentials WHERE operation_id=$1", o.ID).Scan(&current); err != nil || current == "" || current == cipher {
		t.Fatalf("rewrapped ciphertext = %q, %v", current, err)
	}
	key, err := keys.LoadKey(ctx, o)
	if err != nil || key.StreamKey != secret {
		t.Fatalf("rewrapped provider key = %+v, %v", key, err)
	}
}

func TestEncryptedCredentialLoadRequiresCurrentProviderAndRevision(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	o, _ := f.create("credential-current-binding")
	if _, err := f.store.BeginProvision(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	cipher, err := f.transit.Seal(ctx, o.ID, "bound_provider", "bound_key")
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.store.BindEncryptedProvider(ctx, o.ID, "bound_provider", cipher, f.transit.Key)
	if err != nil {
		t.Fatal(err)
	}
	keys := &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"}
	if key, err := keys.LoadKey(ctx, o); err != nil || key.StreamKey != "bound_key" {
		t.Fatalf("current private key = %+v, %v", key, err)
	}
	keys.IngestEndpoint = "http://rtmp.livepeer.studio/live"
	if _, err := keys.LoadKey(ctx, o); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("insecure ingest endpoint = %v", err)
	}
	keys.IngestEndpoint = "rtmps://rtmp.livepeer.studio/live"
	changed := o
	changed.ProviderID = "other_provider"
	if _, err := keys.LoadKey(ctx, changed); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("changed provider identity = %v", err)
	}
	wrongKey := *f.transit
	wrongKey.Key = "other_transit_key"
	keys.Transit = &wrongKey
	if err := keys.Rewrap(ctx, o.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("changed transit key = %v", err)
	}
	keys.Transit = f.transit
	if _, err := f.database.Exec(ctx, "UPDATE livestream.stream_operations SET revision=revision+1 WHERE id=$1", o.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := keys.LoadKey(ctx, o); !errors.Is(err, ErrDenied) {
		t.Fatalf("stale credential revision = %v", err)
	}
}

func TestSuspendedStreamRequestsRestorationAfterPolicyDecision(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.RotationSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	o, _ := f.create("credential-restoration")
	o, err := f.store.BindProvider(ctx, o.ID, "restoration_provider")
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.store.Suspend(ctx, o.ID, SuspensionDecision{OperationID: o.ID, Revision: o.Revision, Actor: "policy_reconciler", Reason: "active_restriction", Allowed: true, ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil || o.State != stateSuspended {
		t.Fatalf("suspended operation = %+v, %v", o, err)
	}
	tx, err := f.database.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := f.store.RequestRestorationTx(ctx, tx, o.ID, o.Wallet, o.Revision)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if restored.State != stateEnding || restored.Intent != rotateAction {
		t.Fatalf("restoration intent = %+v", restored)
	}
	rotation, err := f.store.GetKeyRotation(ctx, o.ID)
	if err != nil || rotation.Status != "revoking" {
		t.Fatalf("restoration rotation = %+v, %v", rotation, err)
	}
}

func TestEncryptedBindingRetainsPolicySuspensionAndProviderIdentity(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	o, _ := f.create("encrypted-policy-suspension")
	if _, err := f.store.BeginProvision(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	cipher, err := f.transit.Seal(ctx, o.ID, "suspended_provider", "suspended_key")
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.store.bindEncryptedProvider(ctx, o.ID, "suspended_provider", cipher, f.transit.Key, true)
	if err != nil || o.State != stateSuspended || o.Intent != endAction {
		t.Fatalf("suspended encrypted binding = %+v, %v", o, err)
	}
	var count int
	if err := f.database.QueryRow(ctx, "SELECT count(*) FROM livestream.stream_outbox WHERE operation_id=$1 AND kind='suspend'", o.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("suspension outbox count = %d, %v", count, err)
	}
	again, err := f.store.BindEncryptedProvider(ctx, o.ID, "suspended_provider", cipher, f.transit.Key)
	if err != nil || again.Revision != o.Revision {
		t.Fatalf("idempotent provider binding = %+v, %v", again, err)
	}
	if _, err := f.store.BindEncryptedProvider(ctx, o.ID, "other_provider", cipher, f.transit.Key); !errors.Is(err, ErrConflict) {
		t.Fatalf("provider identity substitution = %v", err)
	}
}

package streams

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
)

func TestRotationWorkerCreatesReplacementAfterProviderRevocation(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.RotationSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	o, _ := f.create("rotation-worker")
	if _, err := f.store.BeginProvision(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	cipher, err := f.transit.Seal(ctx, o.ID, "provider_1", "key_1")
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.store.BindEncryptedProvider(ctx, o.ID, "provider_1", cipher, f.transit.Key)
	if err != nil {
		t.Fatal(err)
	}
	privacyTx, err := f.database.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.SetPlaybackPrivacyTx(ctx, privacyTx, o, true); err != nil {
		_ = privacyTx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := privacyTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err := f.database.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requested, err := f.store.RequestRotationTx(ctx, tx, o.ID, o.Wallet, o.Revision)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if requested.State != stateEnding || requested.Intent != rotateAction {
		t.Fatalf("rotation request = %+v", requested)
	}
	rotationDocument := f.exchange(http.MethodGet, "/v2/streams/"+o.ID+"/key-rotation", nil, map[string]string{"Authorization": "Bearer " + f.session(f.key)}, http.StatusOK)
	if !bytes.Contains(rotationDocument, []byte(`"status":"revoking"`)) {
		t.Fatalf("owner rotation document = %s", rotationDocument)
	}
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stream/provider_1" {
			t.Errorf("provider path = %s", r.URL.Path)
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("provider method = %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer providerServer.Close()
	provider := livepeer.New("fixture-key", providerServer.URL, nil)
	restrictions := &RestrictionReader{Pool: f.database}
	worker := &ProvisionWorker{Store: f.store, Provider: provider, Restrictions: restrictions}
	if err := worker.rotations(ctx); err != nil {
		t.Fatal(err)
	}
	queued, err := f.store.GetKeyRotation(ctx, o.ID)
	if err != nil || queued.Status != "revoking" || queued.NewStreamID != nil {
		t.Fatalf("queued rotation before revocation = %+v, %v", queued, err)
	}
	if err := worker.RotationStep(ctx, o.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("replacement before provider revocation = %v", err)
	}
	reconcile := &ReconcileWorker{Store: f.store, Provider: provider, Restrictions: restrictions}
	if err := reconcile.Step(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	ended, err := f.store.Get(ctx, o.ID)
	if err != nil || ended.State != stateEnded {
		t.Fatalf("revoked stream = %+v, %v", ended, err)
	}
	if _, err := f.database.Exec(ctx, "INSERT INTO chat.safety_bans(wallet,scope) VALUES($1,'global')", f.wallet); err != nil {
		t.Fatal(err)
	}
	if err := worker.RotationStep(ctx, o.ID); !errors.Is(err, ErrDenied) {
		t.Fatalf("replacement under active restriction = %v", err)
	}
	if _, err := f.database.Exec(ctx, "DELETE FROM chat.safety_bans WHERE wallet=$1", f.wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := f.database.Exec(ctx, "UPDATE indexer.creations SET creator=$2 WHERE pool=$1", o.Pool, "0x1111111111111111111111111111111111111111"); err != nil {
		t.Fatal(err)
	}
	if err := worker.RotationStep(ctx, o.ID); !errors.Is(err, ErrDenied) {
		t.Fatalf("replacement after creator change = %v", err)
	}
	if _, err := f.database.Exec(ctx, "UPDATE indexer.creations SET creator=$2 WHERE pool=$1", o.Pool, f.wallet); err != nil {
		t.Fatal(err)
	}
	if err := worker.RotationStep(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	rotation, err := f.store.GetKeyRotation(ctx, o.ID)
	if err != nil || rotation.NewStreamID == nil || rotation.Status != "provisioning" {
		t.Fatalf("replacement rotation = %+v, %v", rotation, err)
	}
	next, err := f.store.Get(ctx, *rotation.NewStreamID)
	if err != nil || next.State != stateRequested || next.Wallet != o.Wallet || next.Pool != o.Pool {
		t.Fatalf("replacement stream = %+v, %v", next, err)
	}
	private, _, err := f.store.PlaybackPrivacy(ctx, next.ID)
	if err != nil || !private {
		t.Fatalf("replacement playback privacy = %t, %v", private, err)
	}
	if err := worker.RotationStep(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	stillPending, err := f.store.GetKeyRotation(ctx, o.ID)
	if err != nil || stillPending.Status != "provisioning" {
		t.Fatalf("pending replacement = %+v, %v", stillPending, err)
	}
	if _, err := f.store.BeginProvision(ctx, next.ID); err != nil {
		t.Fatal(err)
	}
	newCipher, err := f.transit.Seal(ctx, next.ID, "provider_2", "key_2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.BindEncryptedProvider(ctx, next.ID, "provider_2", newCipher, f.transit.Key); err != nil {
		t.Fatal(err)
	}
	if err := worker.RotationStep(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	rotation, err = f.store.GetKeyRotation(ctx, o.ID)
	if err != nil || rotation.Status != "ready" || *rotation.NewStreamID != next.ID {
		t.Fatalf("ready rotation = %+v, %v", rotation, err)
	}
}

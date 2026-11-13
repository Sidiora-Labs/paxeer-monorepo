package app

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/google/uuid"
)

func TestConnectedAccountRuntimeCopiesBoundariesAndLooksUpPools(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	objects := internaltest.NewStore(t, "user-account-runtime")
	store := account.Store{DB: pool}
	key := bytes.Repeat([]byte{22}, 32)
	policy := delivery.Policy{Subjects: map[string]bool{}, EventTypes: map[string]bool{}}
	subjects := map[string]string{}
	for _, kind := range []string{"profile.updated", "preferences.updated", "watchlist.changed", "consent.updated", "endpoint.bound", "endpoint.revoked", "account.export.requested", "account.deletion.requested", "account.deletion.job", "account.backfill.completed"} {
		subjects[kind] = "users." + kind
		policy.Subjects[subjects[kind]] = true
		policy.EventTypes[kind] = true
	}
	ddl, err := delivery.SchemaDDL("public")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), ddl); err != nil {
		t.Fatal(err)
	}
	durable, err := delivery.NewStore(pool, "public", policy)
	if err != nil {
		t.Fatal(err)
	}
	runtime := account.Runtime{
		Authorizer: &sharedauth.OperationAuthorizer{Ledger: sharedauth.Ledger{Pool: pool}, Verifier: sharedauth.CommittedProofVerifier{}},
		Objects:    objects, Delivery: durable, Subjects: subjects,
		ExportKey: key, EndpointKey: key, EndpointFingerprintKey: key,
		DeviceVerifier:    account.CompactDeviceProofVerifier(key),
		IdentityAuthority: sharedauth.HTTPIdentityAuthority{Endpoint: "https://identity.example.org"},
		Assets:            accountAssetsAuthority{Store: store}, DownloadBaseURL: "https://download.example.org",
		DeletionOwners: map[string]account.DeletionOwner{
			"images":        account.ImagePurger{Store: store, Objects: objects, Exports: objects},
			"notifications": notificationDeletionOwner{},
		},
	}
	h, err := account.NewConnectedHTTP(store, runtime, key)
	if err != nil || h.Runtime == nil || h.PoolLookup == nil || h.Runtime.Metrics == nil {
		t.Fatalf("connected account runtime = %+v, %v", h, err)
	}
	key[0] = 99
	subjects["profile.updated"] = "changed"
	if h.CursorKey[0] != 22 || h.Runtime.ExportKey[0] != 22 || h.Runtime.EndpointKey[0] != 22 || h.Runtime.EndpointFingerprintKey[0] != 22 || h.Runtime.Subjects["profile.updated"] != "users.profile.updated" {
		t.Fatal("connected runtime retained mutable setup inputs")
	}
	const wallet = "0x1111111111111111111111111111111111111111"
	const address = "0x2222222222222222222222222222222222222222"
	if _, err := pool.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS indexer.indexer_canonical_creations_v1(token text NOT NULL,pool text NOT NULL,creator text NOT NULL,height bigint NOT NULL,transaction_index integer NOT NULL,log_index integer NOT NULL,block_timestamp bigint NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO indexer.indexer_canonical_creations_v1 VALUES($1,$2,$3,1,0,0,1760000000)`, address, address, wallet); err != nil {
		t.Fatal(err)
	}
	if found, err := h.PoolLookup(context.Background(), 125, address); err != nil || !found {
		t.Fatalf("known pool = %t, %v", found, err)
	}
	if found, err := h.PoolLookup(context.Background(), 125, wallet); err != nil || found {
		t.Fatalf("unknown pool = %t, %v", found, err)
	}
	if _, err := h.PoolLookup(context.Background(), 1, address); !errors.Is(err, account.ErrInvalid) {
		t.Fatalf("foreign chain pool = %v", err)
	}
	if _, err := h.PoolLookup(context.Background(), 125, uuid.NewString()); !errors.Is(err, account.ErrInvalid) {
		t.Fatalf("malformed pool = %v", err)
	}
}

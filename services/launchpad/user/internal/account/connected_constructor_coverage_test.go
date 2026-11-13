package account_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/google/uuid"
)

func TestConnectedAccountConstructorCopiesRuntimeAndResolvesPools(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	objects := internaltest.NewStore(t, "user-connected-account")
	ddl, err := delivery.SchemaDDL("public")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE VIEW indexer.indexer_canonical_creations_v1 AS SELECT pool_address AS pool,token_address AS token,creator,created_at AS block_timestamp,0::bigint AS height,0::integer AS transaction_index,0::integer AS log_index FROM indexer.pools`); err != nil {
		t.Fatal(err)
	}
	kinds := []string{"profile.updated", "preferences.updated", "watchlist.changed", "consent.updated", "endpoint.bound", "endpoint.revoked", "account.export.requested", "account.deletion.requested", "account.deletion.job", "account.backfill.completed"}
	subjects, allowed, events := map[string]string{}, map[string]bool{}, map[string]bool{}
	for _, kind := range kinds {
		subject := "users." + kind
		subjects[kind], allowed[subject], events[kind] = subject, true, true
	}
	publisher, err := delivery.NewStore(pool, "public", delivery.Policy{Subjects: allowed, EventTypes: events})
	if err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{7}, 32)
	store := account.Store{DB: pool}
	const wallet = "0x1111111111111111111111111111111111111111"
	assetID := uuid.NewString()
	var approved atomic.Bool
	approved.Store(true)
	_, signingKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/v2/accounts/"+wallet+"/assets/"+assetID || r.Header.Get("X-Test-Assertion") == "" || r.Header.Get("X-Test-Signature") == "" {
			t.Errorf("metadata asset request = %s %s", r.Method, r.URL.Path)
			http.Error(w, "invalid asset request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		state := "review"
		if approved.Load() {
			state = "approved"
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"assetId": assetID, "ownerWallet": wallet, "sha256": "0x" + strings.Repeat("a", 64), "mediaType": "image/png", "bytes": 12, "state": state, "publicUrl": "https://example.org/image.png"}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(metadata.Close)
	owner := account.ImagePurger{Store: store, Objects: objects, Exports: objects}
	operations := &sharedauth.OperationAuthorizer{Ledger: sharedauth.Ledger{Pool: pool}, Verifier: sharedauth.CommittedProofVerifier{}}
	assets := app.AccountAssetsAuthority{Store: store, Client: metadata.Client(), Endpoint: metadata.URL, Signer: sharedauth.ServiceRequestIssuer{Issuer: "user", Audience: "metadata", Key: signingKey, Transport: sharedauth.AssertionTransport{AssertionHeader: "X-Test-Assertion", SignatureHeader: "X-Test-Signature"}}}
	runtime := account.Runtime{
		Authorizer: operations, Objects: objects, ExportKey: key, EndpointKey: key, EndpointFingerprintKey: key,
		DeviceVerifier:    account.CompactDeviceProofVerifier(ed25519.PublicKey(key)),
		IdentityAuthority: sharedauth.HTTPIdentityAuthority{Endpoint: "http://localhost", ServiceToken: "account-constructor-token", AllowPlainHTTP: true},
		Assets:            assets,
		DownloadBaseURL:   "https://localhost", Delivery: publisher, Subjects: subjects,
		DeletionOwners: map[string]account.DeletionOwner{"images": owner, "notifications": app.NotificationDeletionOwner{Client: http.DefaultClient, Endpoint: "https://localhost", Token: "account-constructor-notification-token"}},
	}
	h, err := account.NewConnectedHTTP(store, runtime, key)
	if err != nil || h.Runtime == nil || h.Runtime.Metrics == nil || h.PoolLookup == nil || h.Store.DB != pool {
		t.Fatalf("connected account = %+v, %v", h, err)
	}
	key[0] = 9
	subjects["profile.updated"] = "changed"
	if h.CursorKey[0] != 7 || h.Runtime.ExportKey[0] != 7 || h.Runtime.EndpointKey[0] != 7 || h.Runtime.EndpointFingerprintKey[0] != 7 || h.Runtime.Subjects["profile.updated"] != "users.profile.updated" {
		t.Fatal("connected runtime retained caller-owned keys or subjects")
	}
	if _, err := h.PoolLookup(ctx, 1, wallet); !errors.Is(err, account.ErrInvalid) {
		t.Fatalf("foreign chain pool lookup = %v", err)
	}
	if _, err := h.PoolLookup(ctx, 125, "invalid"); !errors.Is(err, account.ErrInvalid) {
		t.Fatalf("invalid pool lookup = %v", err)
	}
	if exists, err := h.PoolLookup(ctx, 125, wallet); err != nil || exists {
		t.Fatalf("unknown pool = %t, %v", exists, err)
	}
	internaltest.SeedPool(t, pool, wallet, wallet, wallet, 1760000000)
	if exists, err := h.PoolLookup(ctx, 125, wallet); err != nil || !exists {
		t.Fatalf("created pool = %t, %v", exists, err)
	}
	accountID := uuid.New()
	if err := store.Create(ctx, accountID, wallet); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPatch, "/v3/profiles/"+wallet, strings.NewReader(`{"expectedRevision":"0","avatarAssetId":"`+assetID+`"}`))
	r.SetPathValue("wallet", wallet)
	r = r.WithContext(account.WithPrincipal(r.Context(), account.Principal{AccountID: accountID, ProofAt: time.Now()}))
	w := httptest.NewRecorder()
	h.ExtendedProfileHandler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("approved avatar update = %d %s", w.Code, w.Body.String())
	}
	profile, err := store.Public(ctx, wallet)
	if err != nil || profile.AvatarAssetID == nil || *profile.AvatarAssetID != assetID {
		t.Fatalf("approved account avatar = %+v, %v", profile, err)
	}
	patchBanner := func(revision string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodPatch, "/v3/profiles/"+wallet, strings.NewReader(`{"expectedRevision":"`+revision+`","bannerAssetId":"`+assetID+`"}`))
		r.SetPathValue("wallet", wallet)
		r = r.WithContext(account.WithPrincipal(r.Context(), account.Principal{AccountID: accountID, ProofAt: time.Now()}))
		w := httptest.NewRecorder()
		h.ExtendedProfileHandler().ServeHTTP(w, r)
		return w
	}
	approved.Store(false)
	if rejected := patchBanner("1"); rejected.Code != http.StatusForbidden {
		t.Fatalf("unapproved banner = %d %s", rejected.Code, rejected.Body.String())
	}
	profile, err = store.Public(ctx, wallet)
	if err != nil || profile.BannerAssetID != nil || profile.Revision != "1" {
		t.Fatalf("rejected banner changed profile = %+v, %v", profile, err)
	}
	approved.Store(true)
	if accepted := patchBanner("1"); accepted.Code != http.StatusOK {
		t.Fatalf("approved banner = %d %s", accepted.Code, accepted.Body.String())
	}
	profile, err = store.Public(ctx, wallet)
	if err != nil || profile.BannerAssetID == nil || *profile.BannerAssetID != assetID || profile.Revision != "2" {
		t.Fatalf("approved account banner = %+v, %v", profile, err)
	}
	patchAvatar := func() *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodPatch, "/v1/profiles/"+wallet, strings.NewReader(`{"expectedRevision":"2","avatarAssetId":"`+assetID+`"}`))
		r = r.WithContext(account.WithPrincipal(r.Context(), account.Principal{AccountID: accountID, ProofAt: time.Now()}))
		w := httptest.NewRecorder()
		h.Handler().ServeHTTP(w, r)
		return w
	}
	approved.Store(false)
	if rejected := patchAvatar(); rejected.Code != http.StatusForbidden {
		t.Fatalf("unapproved avatar = %d %s", rejected.Code, rejected.Body.String())
	}
	approved.Store(true)
	if accepted := patchAvatar(); accepted.Code != http.StatusOK {
		t.Fatalf("approved avatar revision = %d %s", accepted.Code, accepted.Body.String())
	}
	profile, err = store.Public(ctx, wallet)
	if err != nil || profile.AvatarAssetID == nil || *profile.AvatarAssetID != assetID || profile.Revision != "3" {
		t.Fatalf("approved avatar revision = %+v, %v", profile, err)
	}
}

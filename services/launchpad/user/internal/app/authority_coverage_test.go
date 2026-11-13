package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/google/uuid"
)

func TestAccountAssetAuthorityRequests(t *testing.T) {
	fixture := repairM1062New(t)
	wallet := "0x1062106210621062106210621062106210621062"
	assetID := uuid.NewString()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		t.Fatal(err)
	}
	var state sync.Mutex
	status := http.StatusOK
	response := metadataAccountAsset{AssetID: assetID, OwnerWallet: wallet, SHA256: "0x" + strings.Repeat("a", 64), MediaType: "image/png", Bytes: 12, State: "approved"}
	link := "https://example.org/image.png"
	response.PublicURL = &link
	deleted := true
	rawBody := ""
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.Lock()
		defer state.Unlock()
		requests++
		if r.Header.Get("X-Request-ID") == "" || r.Header.Get("X-Test-Assertion") == "" || r.Header.Get("X-Test-Signature") == "" {
			t.Error("asset authority request was unsigned")
		}
		if r.Method == http.MethodGet && r.URL.Path != "/internal/v2/accounts/"+wallet+"/assets/"+assetID {
			t.Errorf("asset request path = %s", r.URL.Path)
		}
		if r.Method == http.MethodDelete && (r.URL.Path != "/internal/v2/accounts/"+wallet+"/assets" || r.URL.Query().Get("deletionId") == "") {
			t.Errorf("asset deletion path = %s", r.URL.String())
		}
		w.WriteHeader(status)
		if status != http.StatusOK {
			return
		}
		if rawBody != "" {
			_, _ = w.Write([]byte(rawBody))
			return
		}
		if r.Method == http.MethodDelete {
			_ = json.NewEncoder(w).Encode(struct{ Deleted bool }{deleted})
			return
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	authority := accountAssetsAuthority{
		Store: fixture.store, Client: server.Client(), Endpoint: server.URL,
		Signer: sharedauth.ServiceRequestIssuer{Issuer: "user", Audience: "metadata", Key: privateKey, Transport: sharedauth.AssertionTransport{AssertionHeader: "X-Test-Assertion", SignatureHeader: "X-Test-Signature"}},
	}
	ctx := context.Background()
	if err := authority.ApprovedAccountAsset(ctx, wallet, assetID); err != nil {
		t.Fatalf("approved account asset = %v", err)
	}
	state.Lock()
	requestCount := requests
	state.Unlock()
	if requestCount != 1 {
		t.Fatalf("asset requests = %d", requestCount)
	}
	if err := authority.ApprovedAccountAsset(ctx, "invalid", assetID); !errors.Is(err, account.ErrInvalid) {
		t.Fatalf("invalid wallet = %v", err)
	}
	if err := authority.ApprovedAccountAsset(ctx, wallet, "invalid"); !errors.Is(err, account.ErrInvalid) {
		t.Fatalf("invalid asset ID = %v", err)
	}
	state.Lock()
	response.State = "quarantined"
	state.Unlock()
	if err := authority.ApprovedAccountAsset(ctx, wallet, assetID); !errors.Is(err, account.ErrDenied) {
		t.Fatalf("quarantined account asset = %v", err)
	}
	state.Lock()
	response.State = "approved"
	state.Unlock()
	for _, tc := range []struct {
		code int
		want error
	}{
		{http.StatusNotFound, account.ErrMissing},
		{http.StatusForbidden, account.ErrDenied},
		{http.StatusServiceUnavailable, nil},
	} {
		state.Lock()
		status = tc.code
		state.Unlock()
		err := authority.ApprovedAccountAsset(ctx, wallet, assetID)
		if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Fatalf("asset authority status %d = %v", tc.code, err)
		}
	}
	state.Lock()
	status = http.StatusOK
	state.Unlock()
	deletionID := uuid.New()
	if err := authority.DeleteAccount(ctx, fixture.principal.AccountID, deletionID); !errors.Is(err, account.ErrState) {
		t.Fatalf("active account deletion = %v", err)
	}
	if _, err := fixture.store.DB.Exec(ctx, `UPDATE account_state SET deleted=true WHERE account_id=$1`, fixture.principal.AccountID); err != nil {
		t.Fatal(err)
	}
	if err := authority.DeleteAccount(ctx, fixture.principal.AccountID, deletionID); err != nil {
		t.Fatalf("asset deletion = %v", err)
	}
	state.Lock()
	deleted = false
	state.Unlock()
	if err := authority.DeleteAccount(ctx, fixture.principal.AccountID, deletionID); !errors.Is(err, account.ErrState) {
		t.Fatalf("unconfirmed asset deletion = %v", err)
	}
	withoutClient := authority
	withoutClient.Client = nil
	if err := withoutClient.ApprovedAccountAsset(ctx, wallet, assetID); err == nil || err.Error() != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("asset authority without client = %v", err)
	}
	withoutSigner := authority
	withoutSigner.Signer.Key = nil
	if err := withoutSigner.ApprovedAccountAsset(ctx, wallet, assetID); err == nil {
		t.Fatalf("unsigned asset authority request = %v", err)
	}
	for _, body := range []string{"{", strings.Repeat("x", 16385)} {
		state.Lock()
		rawBody = body
		state.Unlock()
		if err := authority.ApprovedAccountAsset(ctx, wallet, assetID); err == nil {
			t.Fatalf("invalid asset authority response accepted")
		}
	}
	server.Close()
	if err := authority.ApprovedAccountAsset(ctx, wallet, assetID); err == nil || err.Error() != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("unavailable asset authority = %v", err)
	}
}

func TestImageDeletionOwnerClearsLegacyProfileAndCache(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	objects := internaltest.NewStore(t, "user-deletion-assets")
	redisURL := internaltest.NewRedisURL(t)
	redis, err := sharedredis.NewClient(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = redis.Close() })
	ctx := context.Background()
	accountID, deletionID := uuid.New(), uuid.New()
	const wallet = "0x7777777777777777777777777777777777777777"
	store := account.Store{DB: pool}
	if err := store.Create(ctx, accountID, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE account_state SET deleted=true WHERE account_id=$1`, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,created_at,updated_at) VALUES($1,'Ada',1,1)`, wallet); err != nil {
		t.Fatal(err)
	}
	cacheKey := "user:json:" + wallet
	if err := redis.Set(ctx, cacheKey, "legacy", 0).Err(); err != nil {
		t.Fatal(err)
	}
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/internal/v2/accounts/"+wallet+"/assets" || r.URL.Query().Get("deletionId") != deletionID.String() {
			t.Errorf("asset deletion request = %s %s", r.Method, r.URL.String())
		}
		requested <- struct{}{}
		_ = json.NewEncoder(w).Encode(struct{ Deleted bool }{true})
	}))
	defer server.Close()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner := accountImageDeletionOwner{
		Redis:  redis,
		Legacy: account.ImagePurger{Store: store, Objects: objects, Exports: objects},
		Assets: accountAssetsAuthority{Store: store, Client: server.Client(), Endpoint: server.URL,
			Signer: sharedauth.ServiceRequestIssuer{Issuer: "user", Audience: "metadata", Key: privateKey, Transport: sharedauth.AssertionTransport{AssertionHeader: "X-Test-Assertion", SignatureHeader: "X-Test-Signature"}}},
	}
	if err := owner.DeleteAccount(ctx, accountID, deletionID); err != nil {
		t.Fatalf("image deletion owner = %v", err)
	}
	select {
	case <-requested:
	default:
		t.Fatal("asset deletion authority was not called")
	}
	var profiles int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users.user_profiles WHERE wallet_address=$1`, wallet).Scan(&profiles); err != nil || profiles != 0 {
		t.Fatalf("legacy profiles after deletion = %d, %v", profiles, err)
	}
	if count, err := redis.Exists(ctx, cacheKey).Result(); err != nil || count != 0 {
		t.Fatalf("profile cache after deletion = %d, %v", count, err)
	}
}

func TestNotificationDeletionOwnerRequests(t *testing.T) {
	id, deletion := uuid.New(), uuid.New()
	var state sync.Mutex
	status, deleted := http.StatusOK, true
	rawBody := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.Lock()
		defer state.Unlock()
		if r.Method != http.MethodDelete || r.URL.Path != "/internal/v2/accounts/"+id.String()+"/notification-data" || r.URL.Query().Get("deletionId") != deletion.String() || r.Header.Get("Authorization") != "Bearer "+repairM1062Token {
			t.Errorf("notification deletion request = %s %s", r.Method, r.URL.String())
		}
		w.WriteHeader(status)
		if status == http.StatusOK {
			if rawBody != "" {
				_, _ = w.Write([]byte(rawBody))
				return
			}
			_ = json.NewEncoder(w).Encode(struct{ Deleted bool }{deleted})
		}
	}))
	defer server.Close()
	owner := notificationDeletionOwner{Client: server.Client(), Endpoint: server.URL, Token: repairM1062Token}
	if err := owner.DeleteAccount(context.Background(), id, deletion); err != nil {
		t.Fatalf("notification deletion = %v", err)
	}
	state.Lock()
	deleted = false
	state.Unlock()
	if err := owner.DeleteAccount(context.Background(), id, deletion); !errors.Is(err, account.ErrState) {
		t.Fatalf("unconfirmed notification deletion = %v", err)
	}
	state.Lock()
	status = http.StatusForbidden
	state.Unlock()
	if err := owner.DeleteAccount(context.Background(), id, deletion); err == nil {
		t.Fatal("notification authority denial accepted")
	}
	if err := owner.DeleteAccount(context.Background(), uuid.Nil, deletion); err == nil {
		t.Fatal("missing account ID accepted")
	}
	missingClient := owner
	missingClient.Client = nil
	if err := missingClient.DeleteAccount(context.Background(), id, deletion); err == nil || err.Error() != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("notification authority without client = %v", err)
	}
	state.Lock()
	status = http.StatusOK
	rawBody = "{"
	state.Unlock()
	if err := owner.DeleteAccount(context.Background(), id, deletion); err == nil {
		t.Fatal("invalid notification authority response accepted")
	}
	server.Close()
	if err := owner.DeleteAccount(context.Background(), id, deletion); err == nil || err.Error() != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("unavailable notification authority = %v", err)
	}
}

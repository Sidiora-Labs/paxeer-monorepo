package cache

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func TestIdentityScopeBindsNetworkAndAuthenticatedWallet(t *testing.T) {
	const wallet = "0x1111111111111111111111111111111111111111"
	const serviceToken = "service-token-0123456789-0123456789-0123456789"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v2/identity/request:authenticate" || r.Header.Get("Authorization") != "Bearer "+serviceToken {
			http.Error(w, "identity request rejected", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sharedauth.IdentityPrincipal{AccountID: "00000000-0000-4000-8000-000000000001", SessionID: "00000000-0000-4000-8000-000000000002", Wallet: wallet, AuthEpoch: 7, ProofAt: time.Now().Add(-time.Second), AccessExpiresAt: time.Now().Add(time.Minute)})
	}))
	defer server.Close()
	resolve := IdentityScope(sharedauth.HTTPIdentityAuthority{Endpoint: server.URL, ServiceToken: serviceToken})
	request := httptest.NewRequest(http.MethodGet, "/v2/private", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Authorization", "Bearer access-token")
	scope, err := resolve(request)
	if err != nil || scope.NetworkID != "127.0.0.1" || scope.Principal != "00000000-0000-4000-8000-000000000001:"+wallet || scope.AuthorizationRevision != "7" {
		t.Fatalf("identity scope: %+v, %v", scope, err)
	}
	request.RemoteAddr = "invalid"
	if _, err := resolve(request); err == nil {
		t.Fatal("invalid network address accepted")
	}
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Del("Authorization")
	if _, err := resolve(request); err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("request without authenticated session: %v", err)
	}
}

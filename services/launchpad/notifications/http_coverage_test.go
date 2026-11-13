package notifications

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestLegacyInboxRoutesPersistReadAndOwnedDeletion(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	store := Store{DB: db}
	item, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: "route-account", TriggerEventID: "route-event", TemplateVersion: "v2"}, Category: "chat", Summary: "New message", CreatedAt: time.Now().UTC()}, []DeliveryAttempt{})
	if err != nil || !created {
		t.Fatalf("inbox route item: %+v %v %v", item, created, err)
	}
	h := HTTP{Store: store, Authenticate: func(r *http.Request) (string, error) { return r.Header.Get("X-Account"), nil }}
	route := h.Handler()
	request := func(method, path, body, actor string) (int, []byte) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if actor != "" {
			r.Header.Set("X-Account", actor)
		}
		w := httptest.NewRecorder()
		route.ServeHTTP(w, r)
		response := w.Result()
		defer response.Body.Close()
		raw, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		return response.StatusCode, raw
	}
	if status, _ := request(http.MethodGet, "/v2/discovery/notifications", "", ""); status != http.StatusUnauthorized {
		t.Fatalf("anonymous list: %d", status)
	}
	status, raw := request(http.MethodGet, "/v2/discovery/notifications", "", item.Identity.AccountID)
	var items []InboxItem
	if status != http.StatusOK || json.Unmarshal(raw, &items) != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("list: %d %s", status, raw)
	}
	path := "/v2/discovery/notifications/" + item.ID
	if status, _ := request(http.MethodPatch, path, `{"unknown":true}`, item.Identity.AccountID); status != http.StatusBadRequest {
		t.Fatalf("unknown read field: %d", status)
	}
	if status, _ := request(http.MethodPatch, path, `{"read":true} {}`, item.Identity.AccountID); status != http.StatusBadRequest {
		t.Fatalf("trailing read body: %d", status)
	}
	if status, raw := request(http.MethodPatch, path, `{"read":true}`, item.Identity.AccountID); status != http.StatusOK || !strings.Contains(string(raw), `"read":true`) {
		t.Fatalf("read update: %d %s", status, raw)
	}
	if status, _ := request(http.MethodDelete, path, "", "another-account"); status == http.StatusOK {
		t.Fatalf("foreign deletion accepted: %d", status)
	}
	if status, raw := request(http.MethodDelete, path, "", item.Identity.AccountID); status != http.StatusOK || !strings.Contains(string(raw), `"deleted":true`) {
		t.Fatalf("owned deletion: %d %s", status, raw)
	}
	if status, raw := request(http.MethodGet, "/v2/discovery/notifications", "", item.Identity.AccountID); status != http.StatusOK || string(raw) != "[]\n" {
		t.Fatalf("empty inbox: %d %s", status, raw)
	}
	internal := h.InternalHandler()
	internalStatus := func(method, path, body string) int {
		t.Helper()
		w := httptest.NewRecorder()
		internal.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w.Code
	}
	if got := internalStatus(http.MethodPost, "/internal/v2/notifications/chat", `{"unknown":true}`); got != http.StatusBadRequest {
		t.Fatalf("invalid internal chat: %d", got)
	}
	if got := internalStatus(http.MethodPost, "/internal/v2/notifications/chat", `{"AccountID":"route-account"} {}`); got != http.StatusBadRequest {
		t.Fatalf("trailing internal chat data: %d", got)
	}
	if got := internalStatus(http.MethodPost, "/internal/v2/notifications/chat", `{"AccountID":"route-account"}`); got != http.StatusServiceUnavailable {
		t.Fatalf("missing internal authority: %d", got)
	}
	if got := internalStatus(http.MethodDelete, "/internal/v2/accounts/invalid/notification-data?deletionId=invalid", ""); got != http.StatusBadRequest {
		t.Fatalf("invalid deletion identity: %d", got)
	}
	if got := internalStatus(http.MethodDelete, "/internal/v2/accounts/00000000-0000-0000-0000-000000000001/notification-data?deletionId=00000000-0000-0000-0000-000000000002", ""); got != http.StatusServiceUnavailable {
		t.Fatalf("missing deletion authority: %d", got)
	}
}

func TestInternalPolicyAuthorizesVerifiedPeerAndBearer(t *testing.T) {
	ctx := context.Background()
	t.Setenv("RELEASE_INTERNAL_TOKEN", "shared-secret")
	credentials, err := NewCredentials(ctx, secrets.Options{Provider: secrets.ProviderEnv}, "RELEASE_INTERNAL_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("server", "server.example")
	deleteLeaf := pki.Leaf("delete", "delete.example")
	policy := InternalPolicy{Credentials: credentials, TokenRef: "RELEASE_INTERNAL_TOKEN", DeletePeerDNS: "delete.example", ChatPeerDNS: "chat.example"}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := policy.Authorize(r, "account"); err != nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots(), Certificates: []tls.Certificate{serverLeaf.Pair}}
	server.StartTLS()
	defer server.Close()
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.Roots(), Certificates: []tls.Certificate{deleteLeaf.Pair}}}}
	defer client.CloseIdleConnections()
	for _, scenario := range []struct {
		token string
		want  int
	}{{"shared-secret", http.StatusNoContent}, {"incorrect", http.StatusForbidden}} {
		request, err := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/internal/v2/accounts/account/notification-data", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+scenario.token)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != scenario.want {
			t.Fatalf("peer authorization: got %d want %d", response.StatusCode, scenario.want)
		}
	}
}

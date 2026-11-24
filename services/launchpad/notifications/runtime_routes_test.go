package notifications

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
)

func TestHTTPRuntimeRoutesFrozenNotificationPaths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "list", method: http.MethodGet, path: "/v2/me/notifications"},
		{name: "read", method: http.MethodPut, path: "/v2/me/notifications/00000000-0000-7000-8000-000000000001/read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authenticated := false
			runtime := HTTPRuntime{Notifications: HTTP{
				Authenticate: func(*http.Request) (string, error) {
					authenticated = true
					return "", errors.New("authentication rejected")
				},
			}}
			request := httptest.NewRequest(tc.method, tc.path, nil)
			response := httptest.NewRecorder()
			runtime.Handler().ServeHTTP(response, request)

			if !authenticated {
				t.Error("request did not reach notifications authentication")
			}
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401: %s", response.Code, response.Body.String())
			}
			if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
				t.Fatalf("invalid frozen Error: %v: %s", err, response.Body.String())
			}
		})
	}
}

func TestInternalHandlerServesOnlyNotificationRoutes(t *testing.T) {
	ctx := context.Background()
	database := coordinatortest.Bootstrap(t, os.Getenv("NOTIFICATIONS_TEST_DATABASE_URL"), "api_rest_runtime")
	db, err := sql.Open("pgx", database.Runtimes["api_rest_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	peer := notificationDeliveryAuthorities(t)
	credentials, err := NewCredentials(ctx, notificationDeliveryCredentials(t), "REPAIR958_D11", "REPAIR958_INTERNAL")
	if err != nil {
		t.Fatal(err)
	}
	account, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	policy := InternalPolicy{Credentials: credentials, TokenRef: "REPAIR958_INTERNAL", DeletePeerDNS: "delete.example", ChatPeerDNS: "chat.example"}
	store := Store{DB: db}
	runtime := HTTPRuntime{Notifications: HTTP{
		Store: store, Discovery: discovery.Store{DB: db},
		Recipient:         RecipientConsumer{Store: store, Authority: account},
		InternalAuthorize: policy.Authorize,
	}}
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("api", "api.example")
	server := httptest.NewUnstartedServer(runtime.InternalHandler())
	server.TLS = &tls.Config{
		MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs: pki.Roots(), Certificates: []tls.Certificate{serverLeaf.Pair},
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	client := func(name string) *http.Client {
		leaf := pki.Leaf(name, name+".example")
		c := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), ServerName: "api.example",
			Certificates: []tls.Certificate{leaf.Pair},
		}}}
		t.Cleanup(c.CloseIdleConnections)
		return c
	}
	chatClient, err := NewChatClient(server.URL, client("chat"), credentials, "REPAIR958_INTERNAL")
	if err != nil {
		t.Fatal(err)
	}
	id, created, err := chatClient.EnqueueChat(ctx, "chat-event", peer.account.AccountID, "message-1", time.Now().UTC())
	if err != nil || !created || id == "" {
		t.Fatalf("chat intent: id=%q created=%v err=%v", id, created, err)
	}
	if replayID, replayCreated, err := chatClient.EnqueueChat(ctx, "chat-event", peer.account.AccountID, "message-1", time.Now().UTC()); err != nil || replayCreated || replayID != id {
		t.Fatalf("chat replay: id=%q created=%v err=%v", replayID, replayCreated, err)
	}
	items, err := store.List(ctx, peer.account.AccountID, 10)
	if err != nil || len(items) != 1 || items[0].ID != id {
		t.Fatalf("chat inbox: %+v err=%v", items, err)
	}
	deleteClient := client("delete")
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/internal/v2/accounts/"+peer.account.AccountID+"/notification-data?deletionId="+uuid.NewString(), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer repair958-owner-token")
	response, err := deleteClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var removed struct{ Deleted bool }
	err = json.NewDecoder(response.Body).Decode(&removed)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || err != nil || !removed.Deleted {
		t.Fatalf("account removal: status=%d deleted=%v err=%v", response.StatusCode, removed.Deleted, err)
	}
	items, err = store.List(ctx, peer.account.AccountID, 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("removed inbox: %+v err=%v", items, err)
	}
	if _, _, err := chatClient.EnqueueChat(ctx, "late-chat-event", peer.account.AccountID, "message-2", time.Now().UTC()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("chat after account removal: %v", err)
	}
	for _, path := range []string{
		"/", "/internal", "/internal/v2/unknown", "/internal/v2/notifications/other",
		"/internal/v2/notifications/chat/extra", "/internal/v2/accounts/" + peer.account.AccountID + "/other",
		"/v2/me/notifications", "/v2/discovery/notifications", "/v2/discovery/feed",
		"/health/live", "/health/ready",
	} {
		t.Run(path, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
				request, err := http.NewRequestWithContext(ctx, method, server.URL+path, nil)
				if err != nil {
					t.Fatal(err)
				}
				request.Header.Set("Authorization", "Bearer repair958-owner-token")
				response, err := deleteClient.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != http.StatusNotFound {
					t.Fatalf("%s %s: status=%d, want 404", method, path, response.StatusCode)
				}
			}
		})
	}
}

package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	apiconfig "github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/notifications"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
)

func TestPrivateNotificationListenerRequiresMutualTLSAndPeerPolicy(t *testing.T) {
	t.Setenv("PRIVATE_NOTIFICATION_TOKEN", "private-notification-token")
	credentials, err := notifications.NewCredentials(context.Background(), secrets.Options{Provider: secrets.ProviderEnv}, "PRIVATE_NOTIFICATION_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("api", "api.example")
	deleteLeaf := pki.Leaf("accounts", "delete.example")
	chatLeaf := pki.Leaf("social", "chat.example")
	otherPKI := testpki.New(t)
	otherLeaf := otherPKI.Leaf("other", "delete.example")
	configCA, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	config := apiconfig.Config{IdentityCAFile: pki.CAFile, IdentityClientCertPEM: string(serverLeaf.CertPEM), IdentityClientKeyPEM: string(serverLeaf.KeyPEM)}
	if len(configCA) == 0 {
		t.Fatal("empty test authority")
	}
	serverTLS, err := privateNotificationTLS(config)
	if err != nil {
		t.Fatal(err)
	}
	if serverTLS.MinVersion != tls.VersionTLS13 || serverTLS.ClientAuth != tls.RequireAndVerifyClientCert || serverTLS.ClientCAs == nil {
		t.Fatalf("private TLS policy: %+v", serverTLS)
	}
	policy := notifications.InternalPolicy{Credentials: credentials, TokenRef: "PRIVATE_NOTIFICATION_TOKEN", DeletePeerDNS: "delete.example", ChatPeerDNS: "chat.example"}
	private := httptest.NewUnstartedServer((notifications.HTTP{InternalAuthorize: policy.Authorize}).InternalHandler())
	private.TLS = serverTLS
	private.StartTLS()
	t.Cleanup(private.Close)
	publicHandler := (notifications.HTTP{}).Handler()
	public := httptest.NewServer(publicHandler)
	t.Cleanup(public.Close)

	client := func(leaf testpki.Leaf) *http.Client {
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), ServerName: "api.example", Certificates: []tls.Certificate{leaf.Pair}}}}
	}
	request := func(client *http.Client, method, path, token string, body any) (int, error) {
		var payload []byte
		if body != nil {
			payload, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, private.URL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		defer response.Body.Close()
		return response.StatusCode, nil
	}
	deletionPath := "/internal/v2/accounts/" + uuid.NewString() + "/notification-data?deletionId=" + uuid.NewString()
	chatBody := notifications.ChatRequest{EventID: "event", AccountID: uuid.NewString(), ResourceID: "resource", CreatedAt: time.Now().UTC()}
	if _, err := request(&http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), ServerName: "api.example"}}}, http.MethodDelete, deletionPath, "private-notification-token", nil); err == nil {
		t.Fatal("connection without a client certificate was accepted")
	}
	deleteClient := client(deleteLeaf)
	chatClient := client(chatLeaf)
	otherClient := client(otherLeaf)
	t.Cleanup(deleteClient.CloseIdleConnections)
	t.Cleanup(chatClient.CloseIdleConnections)
	t.Cleanup(otherClient.CloseIdleConnections)
	if status, err := request(deleteClient, http.MethodDelete, deletionPath, "private-notification-token", nil); err != nil || status != http.StatusServiceUnavailable {
		t.Fatalf("delete peer was not admitted: status=%d err=%v", status, err)
	}
	if status, err := request(deleteClient, http.MethodPost, "/internal/v2/notifications/chat", "private-notification-token", chatBody); err != nil || status != http.StatusForbidden {
		t.Fatalf("delete peer reached chat route: status=%d err=%v", status, err)
	}
	if status, err := request(chatClient, http.MethodPost, "/internal/v2/notifications/chat", "private-notification-token", chatBody); err != nil || status != http.StatusServiceUnavailable {
		t.Fatalf("chat peer was not admitted: status=%d err=%v", status, err)
	}
	if status, err := request(chatClient, http.MethodDelete, deletionPath, "private-notification-token", nil); err != nil || status != http.StatusForbidden {
		t.Fatalf("chat peer reached delete route: status=%d err=%v", status, err)
	}
	if status, err := request(deleteClient, http.MethodDelete, deletionPath, "wrong-token", nil); err != nil || status != http.StatusForbidden {
		t.Fatalf("wrong token was accepted: status=%d err=%v", status, err)
	}
	if _, err := request(otherClient, http.MethodDelete, deletionPath, "private-notification-token", nil); err == nil {
		t.Fatal("certificate from another authority was accepted")
	}
	publicResponse := httptest.NewRecorder()
	publicHandler.ServeHTTP(publicResponse, httptest.NewRequest(http.MethodGet, "/internal/v2/notifications/chat", nil))
	if publicResponse.Code != http.StatusNotFound {
		t.Fatalf("public listener served an internal path: %d", publicResponse.Code)
	}
	privateResponse := httptest.NewRecorder()
	private.Config.Handler.ServeHTTP(privateResponse, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if privateResponse.Code != http.StatusNotFound {
		t.Fatalf("private listener served a health path: %d", privateResponse.Code)
	}
}

package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	sharedsecrets "github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/google/uuid"
)

func TestD12VisibilityClientReadsPinnedAuthority(t *testing.T) {
	const token = "visibility-service-token"
	const key = "METADATA_VISIBILITY_TEST_TOKEN"
	t.Setenv(key, token)
	pool := "0x" + strings.Repeat("a", 40)
	asset := uuid.NewString()
	var mu sync.Mutex
	status := http.StatusOK
	answer := D12Visibility{TargetType: metadataTokenTarget, TargetID: pool, Status: metadataVisibilityAllowed}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+token || r.URL.Path != "/internal/v2/safety/visibility" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("targetType") != answer.TargetType || r.URL.Query().Get("targetId") != answer.TargetID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(answer); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client := D12VisibilityClient{Endpoint: server.URL, Secrets: sharedsecrets.EnvProvider{}, TokenKey: key, Client: server.Client()}
	got, err := client.Read(context.Background(), pool)
	if err != nil || got != answer {
		t.Fatalf("initial visibility = %+v, %v", got, err)
	}
	mu.Lock()
	answer = D12Visibility{TargetType: metadataAssetTarget, TargetID: asset, Status: metadataVisibilityRemoved, CaseID: uuid.NewString(), Revision: 7}
	mu.Unlock()
	got, err = client.ReadTarget(context.Background(), metadataAssetTarget, asset)
	if err != nil || got != answer {
		t.Fatalf("asset visibility = %+v, %v", got, err)
	}
	mu.Lock()
	answer.Status = "unknown"
	mu.Unlock()
	if _, err := client.ReadTarget(context.Background(), metadataAssetTarget, asset); err != ErrUnrepresentable {
		t.Fatalf("unknown status = %v", err)
	}
	mu.Lock()
	answer.Status = metadataVisibilityRemoved
	status = http.StatusServiceUnavailable
	mu.Unlock()
	if _, err := client.ReadTarget(context.Background(), metadataAssetTarget, asset); err == nil {
		t.Fatal("unavailable authority accepted")
	}
	if _, err := client.ReadTarget(context.Background(), metadataAssetTarget, "not-an-asset"); err == nil {
		t.Fatal("invalid asset accepted")
	}
	client.Endpoint = "http://127.0.0.1"
	if _, err := client.Read(context.Background(), pool); err == nil {
		t.Fatal("plain HTTP accepted")
	}
}

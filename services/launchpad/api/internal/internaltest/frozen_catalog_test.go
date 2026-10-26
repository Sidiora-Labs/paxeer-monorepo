package internaltest

import (
	"net/http"
	"strings"
	"testing"
)

func TestFrozenV2OperationsDecodeCatalog(t *testing.T) {
	operations := FrozenV2Operations(t)
	if len(operations) != 201 {
		t.Fatalf("decoded %d operations, want 201", len(operations))
	}
	byID := make(map[string]FrozenOperation, len(operations))
	for _, op := range operations {
		if op.ID == "" || op.Method == "" || op.Service == "" {
			t.Errorf("operation missing identity, method or service: %+v", op)
		}
		if !strings.HasPrefix(op.Path, "/v") {
			t.Errorf("%s path %q does not start with /v", op.ID, op.Path)
		}
		if _, exists := byID[op.ID]; exists {
			t.Errorf("duplicate operation ID %q", op.ID)
		}
		byID[op.ID] = op
	}
	want := FrozenOperation{ID: "IF_RANKINGS", Method: http.MethodGet, Path: "/v2/rankings/{category}", Service: "api", Response: "RankingsResponse"}
	if got := byID[want.ID]; got != want {
		t.Errorf("%s = %+v, want %+v", want.ID, got, want)
	}
	want = FrozenOperation{ID: "IF_ACCOUNT_DELETE", Method: http.MethodPost, Path: "/v2/me/deletion-requests", Service: "api", Body: "AccountDeleteBody", Response: "AccountDeleteResponse", Authenticated: true}
	if got := byID[want.ID]; got != want {
		t.Errorf("%s = %+v, want %+v", want.ID, got, want)
	}
}

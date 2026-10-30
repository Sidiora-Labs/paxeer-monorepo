package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

// frozenV2Operation resolves a frozen v2 catalog operation and returns the
// response schema name embedded by the generated Go validator.
func frozenV2Operation(t *testing.T, path, operationID string, expected ...internaltest.FrozenOperation) string {
	t.Helper()
	want := internaltest.FrozenOperation{Method: http.MethodGet, Authenticated: false}
	if len(expected) > 1 {
		t.Fatal("at most one operation expectation is allowed")
	}
	if len(expected) == 1 {
		want = expected[0]
	}
	for _, operation := range internaltest.FrozenV2Operations(t) {
		if operation.Path != path || operation.ID != operationID {
			continue
		}
		if operation.Method != want.Method || operation.Authenticated != want.Authenticated {
			t.Fatalf("%s frozen ID %s method=%s authenticated=%t, want method=%s authenticated=%t", path, operationID, operation.Method, operation.Authenticated, want.Method, want.Authenticated)
		}
		if operation.Response == "" {
			t.Fatalf("GET %s frozen ID %s has no response schema", path, operationID)
		}
		return operation.Response
	}
	t.Fatalf("GET %s lacks frozen ID %s in the catalog", path, operationID)
	return ""
}

func TestFrozenV2OperationLooksUpCatalog(t *testing.T) {
	operations := make(map[string]internaltest.FrozenOperation)
	for _, operation := range internaltest.FrozenV2Operations(t) {
		key := operation.Method + " " + operation.Path
		if _, exists := operations[key]; exists {
			t.Fatalf("duplicate catalog route %s", key)
		}
		operations[key] = operation
	}

	admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err := rest.RegisterProductionReads(router, rest.ProductionReadOptions{
		Model: &store.AccountingReadModel{}, Stream: &broker.PostgresStream{},
		Admission: admission, RegisterSnapshots: true,
	}); err != nil {
		t.Fatal(err)
	}
	rest.RegisterDevelopmentHistorySearchDiscovery(router, store.New(nil, nil), admission)
	count, authenticated := 0, 0
	err = chi.Walk(router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		operation, ok := operations[method+" "+path]
		if !ok {
			t.Errorf("mounted route absent from the v2 catalog: %s %s", method, path)
			return nil
		}
		t.Run(operation.ID, func(t *testing.T) {
			if got := frozenV2Operation(t, path, operation.ID, operation); got != operation.Response {
				t.Errorf("%s %s response=%q, want %q", method, path, got, operation.Response)
			}
		})
		count++
		if operation.Authenticated {
			authenticated++
		}
		return nil
	})
	if err != nil || count != 21 {
		t.Fatalf("walk = %v, %d catalog operations, want 21", err, count)
	}
	if authenticated != 2 {
		t.Fatalf("mounted authenticated operations = %d, want 2", authenticated)
	}
}

func TestFrozenV2ResponseSchemasMatchHandlerBytes(t *testing.T) {
	t.Run("GET /v2/leaderboards/{category}", func(t *testing.T) {
		schema := frozenV2Operation(t, "/v2/leaderboards/{category}", "IF_LEADERBOARDS")
		f := newPoolReadsFixture(t, "")
		now := time.Now().Unix()
		walletA, walletB := leaderboardWalletA, leaderboardWalletB
		insertWindowFill(t, f.pool, f.first, 10, "buy", "1000000", "500000000000000000000", "3000", walletA, &walletA, now-3600)
		insertWindowFill(t, f.pool, f.first, 11, "sell", "2500000", "400000000000000000000", "1000", walletA, &walletA, now-1800)
		insertWindowFill(t, f.pool, f.first, 12, "buy", "4000000", "900000000000000000000", "12000", walletB, nil, now-900)
		// The validator is not vacuous: a handler body missing a required
		// envelope field is rejected by the same frozen schema.
		if interfaces.ValidateJSON(schema, []byte(`{"data":{"snapshot":null}}`)) == nil {
			t.Fatal("frozen schema accepted an invalid envelope")
		}
		for _, path := range []string{"/v2/leaderboards/trader?window=24h", "/v2/leaderboards/trader?window=24h&limit=1", "/v2/leaderboards/volume?window=7d", "/v2/leaderboards/creator?window=30d"} {
			got := getV2(t, f.srv.URL+path)
			if got.status != 200 {
				t.Fatalf("%s status=%d body=%s", path, got.status, got.body)
			}
			if err := interfaces.ValidateJSON(schema, got.body); err != nil {
				t.Fatalf("%s handler bytes violate frozen %s: %v body=%s", path, schema, err, got.body)
			}
		}
	})
	t.Run("GET /v2/snapshots", func(t *testing.T) {
		schema := frozenV2Operation(t, "/v2/snapshots", "IF_TOPIC_SNAPSHOT_READ")
		_, model, stream, address, _ := productionReadsFixture(t)
		admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		router := chi.NewRouter()
		if err = rest.RegisterProductionReads(router, rest.ProductionReadOptions{Model: model, Stream: stream, Admission: admission, RegisterSnapshots: true}); err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(router)
		defer server.Close()
		srv := server.URL
		for _, path := range []string{"/v2/snapshots?topics=market.stats." + address, "/v2/snapshots?topics=market.stats." + address + "&normalizedFilters=" + `%7B%22poolAddress%22%3A%22` + address + `%22%7D`} {
			got := getV2(t, srv+path)
			if got.status != 200 {
				t.Fatalf("%s status=%d body=%s", path, got.status, got.body)
			}
			if err := interfaces.ValidateJSON(schema, got.body); err != nil {
				t.Fatalf("%s handler bytes violate frozen %s: %v body=%s", path, schema, err, got.body)
			}
		}
	})
}

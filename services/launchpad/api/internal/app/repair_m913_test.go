package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/ratelimit"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func repairM913REST(t *testing.T) (*App, *httptest.Server, string) {
	t.Helper()
	c := repair27Node(t, 125)
	hash := c.events(t)
	rpcClient, err := dialNetworkRPC(context.Background(), c.endpoint, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpcClient.Close)
	admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{
		cfg:     config.Config{NodeEnv: "production", APIRole: "rest", RateLimitMax: 3, RateLimitWindowSec: 1, StreamSigningKey: strings.Repeat("k", 32), StatsURL: c.endpoint},
		limiter: ratelimit.NewLimiter(10), admission: admission,
		authority:     &networkAuthority{rpc: rpcClient, config: networkAuthorityConfig{Manifest: deployment.Current()}},
		privateStream: &broker.PostgresStream{},
	}
	router := chi.NewRouter()
	if err := a.registerREST(context.Background(), router, store.New(nil, nil), &store.AccountingReadModel{}, nil, map[string]sharedhttp.Check{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	// Walk the actual New registration tree: even discovery's nested routers
	// and the search proxy must inherit both outer request-budget middleware.
	count := 0
	if err := chi.Walk(router, func(method, path string, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
		count++
		if len(middleware) < 2 {
			t.Errorf("%s %s bypasses the REST budget group", method, path)
		}
		return nil
	}); err != nil || count < 40 {
		t.Fatalf("production route coverage: %d %v", count, err)
	}
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return a, server, hash.Hex()
}

func repairM913Request(t *testing.T, server *httptest.Server, method, path, key string) (int, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	client := server.Client()
	client.Timeout = 5 * time.Second
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, body, response.Header
}

func TestRepairM_913_ProductionRESTQuota(t *testing.T) {
	a, server, hash := repairM913REST(t)
	routes := []struct {
		method, path, document string
		status                 int
	}{
		{http.MethodGet, "/v2/network/deployment", "DeploymentResponse", 200},
		{http.MethodGet, "/v2/transactions/" + hash, "TxStatusResponse", 200},
		{http.MethodPost, "/v2/preflight/trade?unexpected=1", "Error", 400},
	}
	checkAdmitted := func() {
		t.Helper()
		for i, route := range routes {
			status, body, header := repairM913Request(t, server, route.method, route.path, "shared-quota")
			if status != route.status || interfaces.ValidateJSON(route.document, body) != nil || header.Get("X-RateLimit-Limit") != "3" || header.Get("X-RateLimit-Remaining") != strconv.Itoa(2-i) {
				t.Fatalf("admitted %s: %d %s headers %v", route.path, status, body, header)
			}
		}
	}
	checkAdmitted()
	before := a.admission.Stats().Admitted
	for _, route := range routes {
		status, body, header := repairM913Request(t, server, route.method, route.path, "shared-quota")
		if status != http.StatusTooManyRequests || header.Get("X-RateLimit-Remaining") != "0" || header.Get("Retry-After") == "" || interfaces.ValidateJSON("Error", body) != nil {
			t.Fatalf("exhausted %s: %d %s headers %v", route.path, status, body, header)
		}
	}
	if a.admission.Stats().Admitted != before || a.limiter.InFlight() != 0 {
		t.Fatal("quota rejection entered workload admission or leaked an in-flight slot")
	}
	// API keys on the same client address have independent quotas.
	status, body, _ := repairM913Request(t, server, http.MethodGet, routes[0].path, "independent-quota")
	if status != http.StatusOK {
		t.Fatalf("independent client blocked: %d %s", status, body)
	}
	// The configured quota uses a one-second fixed window. Reuse the same
	// router and principal to prove expiry, rather than recreating its store.
	time.Sleep(1100 * time.Millisecond)
	checkAdmitted()
	if a.admission.Stats().Admitted != before+4 || a.limiter.InFlight() != 0 {
		t.Fatalf("recovered requests did not reach the producers: %+v", a.admission.Stats())
	}
}

func TestRepairM_913_TransactionPositionCoverage(t *testing.T) {
	TestProductionNetworkReceiptPositions(t)
}

func TestRepairM_913_RelayReviewedGrants(t *testing.T) {
	reviewed := reviewedRelayRole(t)
	original, err := json.Marshal(reviewed)
	if err != nil {
		t.Fatal(err)
	}
	grants, err := reviewed.GrantSQL()
	if err != nil {
		t.Fatal(err)
	}
	fence := `GRANT EXECUTE ON FUNCTION "recovery"."assert_epoch"(bigint, text) TO "api_rest_runtime"`
	found := 0
	for _, statement := range grants {
		if statement == fence {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("reviewed deployment must already grant the restoration fence: %v", grants)
	}

	const restrictedName = "repair_m913_no_fence"
	statements := relayRestrictedRoleStatements(t, restrictedName)
	if statements[0] != `CREATE ROLE "repair_m913_no_fence" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS` {
		t.Fatalf("restricted role can inherit authority or escalate: %s", statements[0])
	}
	// Compare the entire SQL plan used by the integration test to the reviewed
	// planned-schema grants, with exactly one omission and a separate recipient.
	want := []string{}
	for _, statement := range grants {
		if strings.Contains(statement, `"api_stream"`) || strings.Contains(statement, `"recovery"`) {
			if statement != fence {
				want = append(want, strings.ReplaceAll(statement, `TO "api_rest_runtime"`, `TO "repair_m913_no_fence"`))
			}
		}
	}
	if len(want) == 0 || !reflect.DeepEqual(statements[1:], want) {
		t.Fatalf("restricted role differs beyond its missing fence: got %v want %v", statements, want)
	}
	after, err := json.Marshal(reviewedRelayRole(t))
	if err != nil || string(after) != string(original) {
		t.Fatal("restricted role setup changed the reviewed deployment grants")
	}
	// Feed the relay's real config loader the same bound document shape as
	// the service-backed qualification, preserving startup bounds and routing.
	document := streamRelayDocument{
		Routes: []delivery.StreamRoute{{Stream: "LIVE_STREAM", Subjects: []string{"market.>", "private.portfolio.*"}}},
		Epoch:  1, Region: "fixture-loopback", Batch: 50, MaxAttempts: 5,
		LeaseDuration: 10 * time.Second, PublishTimeout: 2 * time.Second, RetryDelay: time.Second,
		IdleInterval: 100 * time.Millisecond, MaxLag: time.Minute, Concurrency: 1, EventsPerSecond: 500, Burst: 50,
	}
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "relay.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{StreamRelayFile: path, StreamRelaySHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	loaded, err := readStreamRelayDocument(cfg)
	if err != nil || loaded == nil || !reflect.DeepEqual(*loaded, document) {
		t.Fatalf("bound relay startup document: %+v %v", loaded, err)
	}
	cfg.StreamRelaySHA256 = strings.Repeat("0", 64)
	if _, err := readStreamRelayDocument(cfg); err == nil {
		t.Fatal("relay accepted a document outside its reviewed binding")
	}
}

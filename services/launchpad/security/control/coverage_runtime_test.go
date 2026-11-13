package control

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRuntimeConfigurationRequiresIndependentAuthorities(t *testing.T) {
	store, _ := fixture(t, nil)
	ctx := context.Background()
	identityPool, err := pgxpool.New(ctx, fixtureDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(identityPool.Close)
	schema := strings.Trim(store.schema, `"`)
	config := RuntimeConfig{ControlPool: store.pool, IdentityPool: identityPool, ControlSchema: schema, AuthoritySchema: schema, Observe: func(string, error) {}}
	runtime, err := NewRuntime(config)
	if err != nil || runtime.Store == nil || runtime.Identity == nil || runtime.HTTP == nil {
		t.Fatalf("independent runtime authorities: %+v, %v", runtime, err)
	}
	if runtime.Handler() == nil {
		t.Fatal("runtime handler unavailable")
	}
	if err := runtime.Ready(ctx); !errors.Is(err, Denied) {
		t.Fatalf("administrative role admitted as runtime: %v", err)
	}
	if err := runtime.Run(ctx); !errors.Is(err, Denied) {
		t.Fatalf("runtime started with administrative role: %v", err)
	}
	shared := config
	shared.IdentityPool = shared.ControlPool
	if _, err := NewRuntime(shared); !errors.Is(err, Invalid) {
		t.Fatalf("shared pool admitted: %v", err)
	}
	invalidAsset := config
	invalidAsset.Assets = []Asset{{Chain: 1, Address: NativePAX, Decimals: 18}}
	if _, err := NewRuntime(invalidAsset); !errors.Is(err, Invalid) {
		t.Fatalf("invalid asset admitted: %v", err)
	}
	duplicate := config
	duplicate.Assets = []Asset{usdl, usdl}
	if _, err := NewRuntime(duplicate); !errors.Is(err, Invalid) {
		t.Fatalf("duplicate asset admitted: %v", err)
	}
	needsVerifier := config
	needsVerifier.Assets = []Asset{usdl}
	if _, err := NewRuntime(needsVerifier); !errors.Is(err, Unavailable) {
		t.Fatalf("inventory without verifier admitted: %v", err)
	}
	needsReplayCredential := config
	needsReplayCredential.Replay = repairM1020ReplayProvider(t, nil)
	if _, err := NewRuntime(needsReplayCredential); !errors.Is(err, Unavailable) {
		t.Fatalf("replay without credential admitted: %v", err)
	}
	tooMany := config
	tooMany.Assets = []Asset{pax, usdl, pax}
	if _, err := NewRuntime(tooMany); !errors.Is(err, Invalid) {
		t.Fatalf("three inventory assets: %v", err)
	}
	invalidSchema := config
	invalidSchema.AuthoritySchema = "invalid schema"
	if _, err := NewRuntime(invalidSchema); !errors.Is(err, Invalid) {
		t.Fatalf("invalid authority schema: %v", err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("payout request: %s", r.Method)
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	payout, err := NewPayoutProvider(server.URL, strings.Repeat("p", 32), "settlement controller", custody, repairM1020Transport(server))
	if err != nil {
		t.Fatal(err)
	}
	needsSettlementAuthority := config
	needsSettlementAuthority.Payout = payout
	if _, err := NewRuntime(needsSettlementAuthority); !errors.Is(err, Unavailable) {
		t.Fatalf("payout without inventory verifier: %v", err)
	}
	withSettlement := config
	withSettlement.Assets = []Asset{pax}
	withSettlement.Verifier = chain(t, nil)
	withSettlement.InventoryCredential = "inventory-credential"
	withSettlement.SettlementCredential = "settlement-credential"
	withSettlement.Payout = payout
	settlementRuntime, err := NewRuntime(withSettlement)
	if err != nil || settlementRuntime.settlement == nil || settlementRuntime.settlement.provider != payout {
		t.Fatalf("bound settlement runtime: %+v, %v", settlementRuntime, err)
	}
	needsIncidentCredential := config
	needsIncidentCredential.Revocation, err = NewRevocationProvider(server.URL, strings.Repeat("r", 32), repairM1020Transport(server))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewRuntime(needsIncidentCredential); !errors.Is(err, Unavailable) {
		t.Fatalf("revocation without credential: %v", err)
	}
	if err := (*Runtime)(nil).Run(ctx); !errors.Is(err, Invalid) {
		t.Fatalf("nil runtime run: %v", err)
	}
	if err := (*Runtime)(nil).Ready(ctx); !errors.Is(err, Invalid) {
		t.Fatalf("nil runtime readiness: %v", err)
	}
}

func TestRuntimeRunsAuthorizedWorkersUntilCancellation(t *testing.T) {
	env, runtime, _ := phase2ProductionEnvironmentFor(t)
	noWorkers, cancelNoWorkers := context.WithCancel(context.Background())
	noWorkersTimer := time.AfterFunc(50*time.Millisecond, cancelNoWorkers)
	defer noWorkersTimer.Stop()
	defer cancelNoWorkers()
	if err := runtime.Run(noWorkers); !errors.Is(err, context.Canceled) {
		t.Fatalf("idle runtime cancellation: %v", err)
	}
	var observations atomic.Int32
	runtime.config.Assets = []Asset{pax}
	runtime.config.InventoryCredential = env.credentials["infrastructure"]
	runtime.config.IncidentCredential = env.credentials["infrastructure"]
	runtime.config.Replay = repairM1020ReplayProvider(t, nil)
	runtime.config.ReplayCredential = env.credentials["infrastructure"]
	runtime.config.Observe = func(string, error) { observations.Add(1) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(200*time.Millisecond, cancel)
	defer timer.Stop()
	if err := runtime.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("runtime cancellation: %v", err)
	}
	if observations.Load() == 0 {
		t.Fatal("runtime worker did not observe reconciliation")
	}
}

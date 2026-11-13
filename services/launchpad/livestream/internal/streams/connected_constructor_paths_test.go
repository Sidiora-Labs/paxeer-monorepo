package streams

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConnectedRuntimeRequiresBoundProviderAndIdentityInputs(t *testing.T) {
	f := repair57New(t)
	f.useConnectedRuntime()
	cfg := *f.connectedConfig
	for _, tc := range []struct {
		name string
		edit func()
	}{
		{"provider endpoint", func() { cfg.LivepeerAPIURL = "https://other.test/api" }},
		{"budget document", func() { cfg.BudgetPolicySHA256 = strings.Repeat("0", 64) }},
		{"identity certificate", func() { cfg.IdentityCAFile = filepath.Join(t.TempDir(), "missing-ca.pem") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg = *f.connectedConfig
			tc.edit()
			if runtime, err := NewConnectedRuntime(context.Background(), cfg, f.database, f.redis, f.verifier.Backend, DecodeStudioWebhook); err == nil || runtime != nil {
				t.Fatalf("%s admitted unbound runtime: %+v, %v", tc.name, runtime, err)
			}
		})
	}
	cfg = *f.connectedConfig
	if runtime, err := NewConnectedRuntime(context.Background(), cfg, nil, f.redis, f.verifier.Backend, DecodeStudioWebhook); err == nil || runtime != nil {
		t.Fatalf("missing store admitted runtime: %+v, %v", runtime, err)
	}
	if runtime, err := NewConnectedRuntime(context.Background(), cfg, f.database, nil, f.verifier.Backend, DecodeStudioWebhook); err == nil || runtime != nil {
		t.Fatalf("missing viewer store admitted runtime: %+v, %v", runtime, err)
	}
	if runtime, err := NewConnectedRuntime(context.Background(), cfg, f.database, f.redis, nil, DecodeStudioWebhook); err == nil || runtime != nil {
		t.Fatalf("missing chain admitted runtime: %+v, %v", runtime, err)
	}
	if runtime, err := NewConnectedRuntime(context.Background(), cfg, f.database, f.redis, f.verifier.Backend, nil); err == nil || runtime != nil {
		t.Fatalf("missing provider decoder admitted runtime: %+v, %v", runtime, err)
	}
	raw, err := os.ReadFile(cfg.BudgetPolicyFile)
	if err != nil {
		t.Fatal(err)
	}
	var policy BudgetPolicy
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	policy.MonthlyUnits = "0"
	raw, err = json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "budget.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cfg.BudgetPolicyFile, cfg.BudgetPolicySHA256 = path, fmt.Sprintf("%x", sha256.Sum256(raw))
	if runtime, err := NewConnectedRuntime(context.Background(), cfg, f.database, f.redis, f.verifier.Backend, DecodeStudioWebhook); err == nil || runtime != nil {
		t.Fatalf("zero monthly budget admitted runtime: %+v, %v", runtime, err)
	}
	cfg = *f.connectedConfig
	runtime, err := NewConnectedRuntime(context.Background(), cfg, f.database, f.redis, f.verifier.Backend, DecodeStudioWebhook)
	if err != nil || runtime.Deps.Authorize == nil || runtime.Ready == nil {
		t.Fatalf("connected runtime = %+v, %v", runtime, err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v2/streams", strings.NewReader(`{}`))
	if decision, err := runtime.Deps.Authorize(context.Background(), request, "stream.create", Operation{}); err == nil || decision.Allowed {
		t.Fatalf("unsigned creator authority = %+v, %v", decision, err)
	}
	readyCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runtime.Ready(readyCtx); err == nil {
		t.Fatal("canceled provider readiness accepted")
	}
}

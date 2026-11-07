package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tmtypes "github.com/sei-protocol/sei-chain/sei-tendermint/types"
	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/rpc"
	"github.com/sei-protocol/seictl/sidecar/tasks"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRepairM_840_DistinctRenderedWitnesses(t *testing.T) {
	for _, tc := range []struct {
		name      string
		endpoints []string
		sets      []string
	}{
		{name: "one", endpoints: []string{"a:26657"}},
		{name: "duplicate", endpoints: []string{"a:26657", "a:26657"}},
		{name: "dns-alias", endpoints: []string{"A.:26657", "a:26657"}},
		{name: "port-alias", endpoints: []string{"a:026657", "a:26657"}},
		{name: "url", endpoints: []string{"http://a:26657", "b:26657"}},
		{name: "port-range", endpoints: []string{"a:65536", "b:26657"}},
		{name: "set-override", endpoints: []string{"a:26657", "b:26657"}, sets: []string{"spec.stateSync.rpcServers[1]=a:26657"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj, err := render(renderArgs{preset: stateSyncPreset, name: "n-state-sync", target: "n", rpcServers: tc.endpoints, sets: tc.sets})
			if err == nil || obj != nil {
				t.Fatalf("unsafe explicit witnesses rendered: object=%v error=%v", obj, err)
			}
		})
	}
	for _, endpoints := range [][]string{nil, {"a:26657", "b:26657"}} {
		if _, err := render(renderArgs{preset: stateSyncPreset, name: "n-state-sync", target: "n", rpcServers: endpoints}); err != nil {
			t.Fatalf("valid witness selection refused: %v", err)
		}
	}
}

// The committed RPC transcript is served over loopback. Only peer identity,
// chain, hash and availability vary; the component under repair is never replaced.
func repairM840Witness(t *testing.T, identity, chain, hash string) (string, *atomic.Bool) {
	t.Helper()
	raw, err := os.ReadFile("testdata/repair_m840_witness.json")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(raw), strings.Repeat("1", 40), identity)
	text = strings.ReplaceAll(text, "repair-m840", chain)
	text = strings.ReplaceAll(text, strings.Repeat("A", 64), hash)
	var transcript map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &transcript); err != nil {
		t.Fatal(err)
	}
	down := &atomic.Bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			http.Error(w, "witness unavailable", http.StatusServiceUnavailable)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		response, ok := transcript[name]
		if !ok || (name == "block" && r.URL.Query().Get("height") != "3000") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(response); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://"), down
}

func repairM840Params(t *testing.T, endpoints []string) map[string]any {
	t.Helper()
	obj, err := render(renderArgs{preset: stateSyncPreset, name: "n-state-sync", namespace: "pax", target: "n", rpcServers: endpoints})
	if err != nil {
		t.Fatal(err)
	}
	// Cross the actual workflow JSON contract, then use the sidecar's production
	// typed-handler codec. Controller plan wiring is an explicit separate request.
	raw, err := obj.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var received unstructured.Unstructured
	if err := received.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	params, found, err := unstructured.NestedMap(received.Object, "spec", "stateSync")
	if err != nil || !found {
		t.Fatalf("missing stateSync wire params: %v", err)
	}
	return params
}

func repairM840Submit(t *testing.T, e *engine.Engine, typ engine.TaskType, params map[string]any) *engine.TaskResult {
	t.Helper()
	id, err := e.Submit(engine.Task{Type: typ, Params: params})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result, err := e.GetResult(id)
		if err != nil {
			t.Fatal(err)
		}
		if result != nil && result.Status != engine.TaskStatusRunning {
			return result
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s did not finish", typ)
	return nil
}

func repairM840HeldNode(t *testing.T) (string, *engine.SQLiteStore, *engine.Engine, context.CancelFunc) {
	t.Helper()
	home := t.TempDir()
	for _, dir := range []string{"config", "data"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, "config", "config.toml"), []byte("mode = \"full\"\n[p2p]\npersistent-peers = \"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc := &tmtypes.GenesisDoc{ChainID: "repair-m840", GenesisTime: time.Now().UTC(), ConsensusParams: tmtypes.DefaultConsensusParams(), AppState: json.RawMessage(`{}`)}
	if err := doc.SaveAs(filepath.Join(home, "config", "genesis.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "data", "chain-state"), []byte("retained chain data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "data", "priv_validator_state.json"), []byte(`{"height":"987","round":2,"step":3}`), 0600); err != nil {
		t.Fatal(err)
	}
	// The absent local daemon is a loopback HTTP peer, never a live endpoint.
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "paxd stopped", http.StatusServiceUnavailable)
	}))
	t.Cleanup(local.Close)
	oldEndpoint := rpc.DefaultEndpoint
	rpc.DefaultEndpoint = local.URL
	t.Cleanup(func() { rpc.DefaultEndpoint = oldEndpoint })
	store, err := engine.NewSQLiteStore(filepath.Join(home, "sidecar.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := engine.NewEngine(ctx, map[engine.TaskType]engine.TaskHandler{
		engine.TaskMarkNotReady:       tasks.NewMarkNotReadier(store).Handler(),
		engine.TaskStopSeid:           tasks.NewStopSeider().Handler(),
		engine.TaskResetData:          tasks.NewResetDataer(home).Handler(),
		engine.TaskConfigureStateSync: tasks.NewStateSyncConfigurer(home, nil).Handler(),
	}, store)
	for _, typ := range []engine.TaskType{engine.TaskMarkNotReady, engine.TaskStopSeid} {
		if result := repairM840Submit(t, e, typ, nil); result.Status != engine.TaskStatusCompleted {
			t.Fatalf("%s failed: %+v", typ, result)
		}
	}
	return home, store, e, cancel
}

func TestRepairM_840_WitnessFailureBeforeWipe(t *testing.T) {
	for _, tc := range []struct {
		name, identity, chain, hash, want string
		down                              bool
	}{
		{name: "unreachable", identity: strings.Repeat("2", 40), chain: "repair-m840", hash: strings.Repeat("A", 64), down: true, want: "two distinct reachable"},
		{name: "same-node", identity: strings.Repeat("1", 40), chain: "repair-m840", hash: strings.Repeat("A", 64), want: "share node identity"},
		{name: "wrong-chain", identity: strings.Repeat("2", 40), chain: "other-chain", hash: strings.Repeat("A", 64), want: "expected"},
		{name: "disagreement", identity: strings.Repeat("2", 40), chain: "repair-m840", hash: strings.Repeat("B", 64), want: "trust hashes disagree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, _ := repairM840Witness(t, strings.Repeat("1", 40), "repair-m840", strings.Repeat("A", 64))
			second, down := repairM840Witness(t, tc.identity, tc.chain, tc.hash)
			home, store, e, cancel := repairM840HeldNode(t)
			defer cancel()
			defer store.Close()
			params := repairM840Params(t, []string{first, second})
			down.Store(tc.down)
			result := repairM840Submit(t, e, engine.TaskResetData, params)
			if result.Status != engine.TaskStatusFailed || !strings.Contains(result.Error, "witness preflight") || !strings.Contains(result.Error, tc.want) {
				t.Fatalf("wrong refusal: %+v", result)
			}
			data, err := os.ReadFile(filepath.Join(home, "data", "chain-state"))
			if err != nil || string(data) != "retained chain data" {
				t.Fatalf("preflight changed chain data: %q, %v", data, err)
			}
			if e.Healthz() {
				t.Fatal("failed preflight released the hold")
			}
			release, err := store.LatestByType(string(engine.TaskMarkReady))
			if err != nil || release != nil {
				t.Fatalf("failure created a release: %+v, %v", release, err)
			}
		})
	}
}

func TestRepairM_840_QuorumLossAfterWipeRetainsHoldOnReopen(t *testing.T) {
	first, _ := repairM840Witness(t, strings.Repeat("1", 40), "repair-m840", strings.Repeat("A", 64))
	second, down := repairM840Witness(t, strings.Repeat("2", 40), "repair-m840", strings.Repeat("A", 64))
	home, store, e, cancel := repairM840HeldNode(t)
	defer cancel()
	params := repairM840Params(t, []string{first, second})
	result := repairM840Submit(t, e, engine.TaskResetData, params)
	if result.Status != engine.TaskStatusCompleted {
		t.Fatalf("healthy pre-reset quorum refused: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(home, "data", "chain-state")); !os.IsNotExist(err) {
		t.Fatalf("real reset did not erase chain data: %v", err)
	}
	signPath := filepath.Join(home, "data", "priv_validator_state.json")
	sign, err := os.ReadFile(signPath)
	if err != nil || string(sign) != `{"height":"987","round":2,"step":3}` {
		t.Fatalf("last-sign state changed: %q, %v", sign, err)
	}
	configPath := filepath.Join(home, "config", "config.toml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	down.Store(true)
	failed := repairM840Submit(t, e, engine.TaskConfigureStateSync, params)
	if failed.Status != engine.TaskStatusFailed || !strings.Contains(failed.Error, "two distinct reachable") {
		t.Fatalf("configuration padded a lost witness: %+v", failed)
	}
	after, err := os.ReadFile(configPath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("failed configuration changed config: %s, %v", after, err)
	}
	if e.Healthz() {
		t.Fatal("quorum loss released paxd")
	}
	cancel()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := engine.NewSQLiteStore(filepath.Join(home, "sidecar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	recovered := engine.NewEngine(context.Background(), nil, reopened)
	recovered.RehydrateStaleTasks()
	if recovered.Healthz() {
		t.Fatal("reopen released held paxd")
	}
	hold, err := reopened.LatestByType(string(engine.TaskMarkNotReady))
	if err != nil || hold == nil || hold.Status != engine.TaskStatusCompleted {
		t.Fatalf("durable hold lost: %+v, %v", hold, err)
	}
	release, err := reopened.LatestByType(string(engine.TaskMarkReady))
	if err != nil || release != nil {
		t.Fatalf("recovery manufactured a release: %+v, %v", release, err)
	}
	failure, err := reopened.Get(failed.ID)
	if err != nil || failure == nil || failure.Status != engine.TaskStatusFailed || failure.Error != failed.Error {
		t.Fatalf("failure not recovered: %+v, %v", failure, err)
	}
	// Restoring quorum allows the actual configure handler to make progress while
	// still held. Only the controller's later explicit release may start paxd.
	down.Store(false)
	if _, err := tasks.NewStateSyncConfigurer(home, nil).Handler()(context.Background(), params); err != nil {
		t.Fatalf("restored independent witnesses did not recover configuration: %v", err)
	}
	cfg, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{first, second, strings.Repeat("A", 64), "3000"} {
		if !strings.Contains(string(cfg), want) {
			t.Errorf("configured trust point missing %q: %s", want, cfg)
		}
	}
	if recovered.Healthz() {
		t.Fatal("configuration alone released the hold")
	}
}

func TestRepairM_840_RecoveryGuidanceOverAPI(t *testing.T) {
	obj, err := render(renderArgs{preset: stateSyncPreset, name: "n-state-sync", namespace: "pax", target: "n"})
	if err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(obj.Object, phaseFailed, "status", "phase"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := fmt.Sprintf("/apis/%s/%s/namespaces/pax/%s/n-state-sync", kind.GVK.Group, kind.GVK.Version, kind.GVR.Resource)
		if r.Method != http.MethodGet || r.URL.Path != want {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(obj.Object); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{kind.GVK.GroupVersion()})
	mapper.AddSpecific(kind.GVK, kind.GVR, kind.GVR, meta.RESTScopeNamespace)
	kcli, err := client.New(&rest.Config{Host: srv.URL}, client.Options{Mapper: mapper})
	if err != nil {
		t.Fatal(err)
	}
	err = preflight(context.Background(), kcli, "pax", "n-state-sync")
	if err == nil {
		t.Fatal("live API Failed workflow was allowed")
	}
	for _, want := range []string{"still holds the node", forceDeleteAnnotation, "remove it, then re-run", "--name alone does not recover", "paxd can restart safely"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("recovery guidance missing %q: %v", want, err)
		}
	}
	var handoff strings.Builder
	emitCompleteHandoff(&handoff, "pax", "n")
	if !strings.Contains(handoff.String(), "paxd is restarting") || !strings.Contains(handoff.String(), "seictl node watch n --until=caught-up -n pax") {
		t.Fatalf("invalid deployed-daemon handoff: %s", &handoff)
	}
	guide, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"before any erasure", "hold retained", "A lone surviving witness is never duplicated", "Plan-time endpoint count does not prove runtime availability", "Force-deletion releases the hold", "`paxd`", "both** `reset-data` and `configure-state-sync`"} {
		if !strings.Contains(string(guide), want) {
			t.Errorf("guide missing runtime/recovery contract %q", want)
		}
	}
}

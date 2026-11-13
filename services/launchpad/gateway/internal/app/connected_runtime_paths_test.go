package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	mediastore "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage"
	control "github.com/Sidiora-Technologies/KindleLaunch/security/control"
	sharedsecrets "github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConnectedRuntimeMetricsAndFailureState(t *testing.T) {
	runtime := &gatewayConnectedRuntime{}
	runtime.purgeAge.Store(31000)
	runtime.purgeErrors.Store(2)
	w := httptest.NewRecorder()
	runtime.writeMediaMetrics(w)
	for _, want := range []string{
		"media_purge_backlog_age_seconds 31.000",
		"media_purge_backlog_alert 1",
		"media_purge_provider_errors_total 2",
	} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("media metrics omit %q: %s", want, w.Body.String())
		}
	}
	runtime.purgeAge.Store(30000)
	w = httptest.NewRecorder()
	runtime.writeMediaMetrics(w)
	if !strings.Contains(w.Body.String(), "media_purge_backlog_alert 0") {
		t.Fatalf("backlog threshold: %s", w.Body.String())
	}
	runtime.fail(nil)
	if runtime.failure.Load() != nil {
		t.Fatal("nil failure changed readiness")
	}
	first := errors.New("durable broker unavailable")
	runtime.fail(first)
	runtime.fail(errors.New("later failure"))
	if got := runtime.ready(context.Background()); !errors.Is(got, first) {
		t.Fatalf("first runtime failure: %v", got)
	}
}

func TestConnectedRuntimeRejectsIncompleteObjectBindings(t *testing.T) {
	cases := []struct {
		name     string
		bindings []gatewayObjectWorkerBinding
		want     string
	}{
		{"missing workers", nil, "explicit scoped object workers required"},
		{"too many workers", make([]gatewayObjectWorkerBinding, 5), "explicit scoped object workers required"},
		{"unknown kind", []gatewayObjectWorkerBinding{{Kind: "unknown", Prefix: "approved/", Bucket: "media", InventoryLimit: 1}}, "object worker authority binding invalid"},
		{"wrong prefix", []gatewayObjectWorkerBinding{{Kind: "variant", Prefix: "private/", Bucket: "media", InventoryLimit: 1}}, "object worker authority binding invalid"},
		{"missing bucket", []gatewayObjectWorkerBinding{{Kind: "variant", Prefix: "approved/", InventoryLimit: 1}}, "object worker authority binding invalid"},
		{"unbounded inventory", []gatewayObjectWorkerBinding{{Kind: "variant", Prefix: "approved/", Bucket: "media", InventoryLimit: 100001}}, "object worker authority binding invalid"},
		{"missing role references", []gatewayObjectWorkerBinding{{Kind: "variant", Prefix: "approved/", Bucket: "media", InventoryLimit: 1}}, "distinct object IAM role references required"},
		{"shared role reference", []gatewayObjectWorkerBinding{{Kind: "variant", Prefix: "approved/", Bucket: "media", InventoryLimit: 1, DeleteAccessKeyReference: "S3_ACCESS_KEY_ID", DeleteSecretKeyReference: "delete-secret", InventoryAccessKeyReference: "inventory-key", InventorySecretKeyReference: "inventory-secret"}}, "distinct object IAM role references required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &gatewayConnectedRuntime{binding: gatewayRuntimeBinding{ObjectWorkers: tc.bindings}}
			if err := runtime.configureObjects(context.Background(), config.Config{}, nil); err == nil || !strings.Contains(err.Error(), tc.want) || len(runtime.objects) != 0 {
				t.Fatalf("object binding: error=%v workers=%d", err, len(runtime.objects))
			}
		})
	}
}

func TestGatewayControlRouteInventory(t *testing.T) {
	routes := gatewayControlRoutes()
	if len(routes) != 21 {
		t.Fatalf("control routes=%d", len(routes))
	}
	seen := map[string]bool{}
	for _, route := range routes {
		key := route.Method + " " + route.Template
		if seen[key] {
			t.Fatalf("duplicate control route %q", key)
		}
		seen[key] = true
		if route.Method == http.MethodPost && !route.Authenticate {
			t.Fatalf("control write without authentication: %q", key)
		}
	}
	for _, key := range []string{"GET /v2/control/caps/{capability}", "GET /v2/control/settlement-capability", "POST /v2/control/settlements"} {
		if !seen[key] {
			t.Fatalf("missing control route %q", key)
		}
	}
}

func TestConnectedRuntimeUsesBoundSecretReferences(t *testing.T) {
	for name, value := range map[string]string{
		"GATEWAY_TEST_DELETE_KEY":       "fixture-access-key",
		"GATEWAY_TEST_DELETE_SECRET":    "fixture-secret-key",
		"GATEWAY_TEST_INVENTORY_KEY":    "fixture-access-key",
		"GATEWAY_TEST_INVENTORY_SECRET": "fixture-secret-key",
	} {
		t.Setenv(name, value)
	}
	provider, err := sharedsecrets.NewChecked(sharedsecrets.Options{Provider: "env"})
	if err != nil {
		t.Fatal(err)
	}
	resolver := identityKeyResolver{provider: provider}
	if key, err := resolver.ResolveIdentityKey(context.Background(), "GATEWAY_TEST_DELETE_KEY"); err != nil || string(key) != "fixture-access-key" {
		t.Fatalf("bound identity key: key=%q err=%v", key, err)
	}
	if _, err := resolver.ResolveIdentityKey(context.Background(), "GATEWAY_TEST_ABSENT_KEY"); err == nil {
		t.Fatal("missing identity key reference was accepted")
	}
	secret := func(reference string) (string, error) { return provider.GetRequired(context.Background(), reference) }
	worker := gatewayObjectWorkerBinding{
		Kind: "variant", Prefix: "approved/", Bucket: "approved", InventoryLimit: 1,
		DeleteAccessKeyReference: "GATEWAY_TEST_DELETE_KEY", DeleteSecretKeyReference: "GATEWAY_TEST_DELETE_SECRET",
		InventoryAccessKeyReference: "GATEWAY_TEST_INVENTORY_KEY", InventorySecretKeyReference: "GATEWAY_TEST_INVENTORY_SECRET",
	}
	runtime := &gatewayConnectedRuntime{binding: gatewayRuntimeBinding{ObjectWorkers: []gatewayObjectWorkerBinding{worker}}}
	if err := runtime.configureObjects(context.Background(), config.Config{}, secret); err == nil || !strings.Contains(err.Error(), "object IAM roles must be independently provisioned") {
		t.Fatalf("reused object roles: %v", err)
	}
	completeWorker := worker
	for _, tc := range []struct {
		name string
		edit func(*gatewayObjectWorkerBinding)
	}{
		{"delete access key", func(b *gatewayObjectWorkerBinding) { b.DeleteAccessKeyReference = "GATEWAY_TEST_ABSENT_KEY" }},
		{"delete secret key", func(b *gatewayObjectWorkerBinding) { b.DeleteSecretKeyReference = "GATEWAY_TEST_ABSENT_KEY" }},
		{"inventory access key", func(b *gatewayObjectWorkerBinding) { b.InventoryAccessKeyReference = "GATEWAY_TEST_ABSENT_KEY" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := completeWorker
			tc.edit(&candidate)
			bounded := &gatewayConnectedRuntime{binding: gatewayRuntimeBinding{ObjectWorkers: []gatewayObjectWorkerBinding{candidate}}}
			if err := bounded.configureObjects(context.Background(), config.Config{}, secret); err == nil || len(bounded.objects) != 0 {
				t.Fatalf("missing scoped object credential: workers=%d err=%v", len(bounded.objects), err)
			}
		})
	}
	worker.InventorySecretKeyReference = "GATEWAY_TEST_ABSENT_KEY"
	runtime.binding.ObjectWorkers[0] = worker
	if err := runtime.configureObjects(context.Background(), config.Config{}, secret); err == nil {
		t.Fatal("missing scoped object secret was accepted")
	}
}

func TestConnectedRuntimeConstructsScopedObjectWorkers(t *testing.T) {
	bucket := "gateway-workers-" + uuid.NewString()
	endpoint := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(endpoint.Close)
	bindings := []gatewayObjectWorkerBinding{}
	for index, kind := range []string{"variant", "original", "temporary"} {
		references := [4]string{}
		for role := range references {
			reference := fmt.Sprintf("GATEWAY_TEST_WORKER_%d_%d", index, role)
			t.Setenv(reference, fmt.Sprintf("role-%d-%d", index, role))
			references[role] = reference
		}
		bindings = append(bindings, gatewayObjectWorkerBinding{
			Kind: kind, Bucket: bucket, Prefix: map[string]string{"variant": "approved/", "original": "quarantine/", "temporary": "temporary/"}[kind], InventoryLimit: 100,
			DeleteAccessKeyReference: references[0], DeleteSecretKeyReference: references[1], InventoryAccessKeyReference: references[2], InventorySecretKeyReference: references[3],
		})
	}
	provider, err := sharedsecrets.NewChecked(sharedsecrets.Options{Provider: "env"})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &gatewayConnectedRuntime{binding: gatewayRuntimeBinding{ObjectWorkers: bindings}}
	cfg := config.Config{S3Endpoint: endpoint.URL, S3Region: "us-east-1", S3AccessKeyID: uuid.NewString()}
	secret := func(reference string) (string, error) { return provider.GetRequired(context.Background(), reference) }
	if err := runtime.configureObjects(context.Background(), cfg, secret); err != nil {
		t.Fatalf("scoped worker binding: %v", err)
	}
	if len(runtime.objects) != len(bindings) {
		t.Fatalf("scoped workers=%d, want=%d", len(runtime.objects), len(bindings))
	}
	for index, object := range runtime.objects {
		if object.binding != bindings[index] || object.deleteStore == nil || object.inventory == nil {
			t.Fatalf("worker %d has incomplete role or object binding: %+v", index, object.binding)
		}
	}
	incomplete := &gatewayConnectedRuntime{binding: gatewayRuntimeBinding{ObjectWorkers: bindings[:1]}}
	if err := incomplete.configureObjects(context.Background(), cfg, secret); err == nil || !strings.Contains(err.Error(), "complete asset lineage stores required") || len(incomplete.objects) != 1 {
		t.Fatalf("incomplete object lineage: workers=%d err=%v", len(incomplete.objects), err)
	}
}

func TestConnectedRuntimeStopsCanceledWorkers(t *testing.T) {
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("gateway database fixture is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	controlPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controlPool.Close)
	authority, err := mediastore.NewAuthority(pool, "gateway", "metadata", "chat")
	if err != nil {
		t.Fatal(err)
	}
	controlRuntime, err := control.NewRuntime(control.RuntimeConfig{
		ControlPool: controlPool, IdentityPool: pool, ControlSchema: "pnl", AuthoritySchema: "pnl",
		Observe: func(name string, event error) { t.Logf("%s: %v", name, event) },
	})
	if err != nil {
		t.Fatal(err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	runtime := &gatewayConnectedRuntime{ctx: canceled, stop: stop, mediaAuthority: authority, control: controlRuntime}
	runtime.start(nil)
	completed := make(chan struct{})
	go func() { runtime.close(); close(completed) }()
	select {
	case <-completed:
	case <-time.After(3 * time.Second):
		t.Fatal("canceled workers did not stop")
	}
	if runtime.failure.Load() != nil || runtime.ctx.Err() != context.Canceled {
		t.Fatalf("canceled runtime state: failure=%v context=%v", runtime.failure.Load(), runtime.ctx.Err())
	}
}

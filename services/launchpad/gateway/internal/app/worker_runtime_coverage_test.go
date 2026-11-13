package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	mediastore "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage"
	sharedstorage "github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConnectedWorkersReportUnavailableMediaAuthority(t *testing.T) {
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("gateway database fixture is required")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	authority, err := mediastore.NewAuthority(pool, "gateway_worker_fixture", "metadata", "chat")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := httptest.NewTLSServer(http.NotFoundHandler())
	defer endpoint.Close()
	_, inventory, err := mediastore.NewScopedObjectClient(context.Background(), sharedstorage.Config{
		Endpoint: endpoint.URL, Region: "us-east-1", Bucket: "gateway-worker-fixture", AccessKeyID: "worker-access", SecretAccessKey: "worker-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	ctx, stop := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer stop()
	runtime := &gatewayConnectedRuntime{
		ctx: ctx, stop: stop, mediaAuthority: authority, logger: logger,
		objects: []gatewayObjectWorker{{binding: gatewayObjectWorkerBinding{Kind: "variant", Bucket: "gateway-worker-fixture", Prefix: "approved/", InventoryLimit: 1}, inventory: inventory}},
	}
	runtime.runObjectWorkers()
	if !strings.Contains(output.String(), "media object deletion pending") || !strings.Contains(output.String(), "media inventory pending") {
		t.Fatalf("worker dependency state was not reported: %s", output.String())
	}
	output.Reset()
	intakeCtx, stopIntake := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer stopIntake()
	runtime.ctx = intakeCtx
	runtime.runRestrictionIntake()
	if !strings.Contains(output.String(), "media restriction intake pending") {
		t.Fatalf("restriction intake dependency state was not reported: %s", output.String())
	}
}

package sdkcontract

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func TestMetadataSDKContract(t *testing.T) {
	startObjects(t)
	pool, databaseURL := internaltest.NewPostgresURL(t)
	redisURL := internaltest.NewRedisURL(t)
	objects := internaltest.NewMinIO(t, "metadata-sdk")
	gateway := internaltest.NewGateway(t, databaseURL, redisURL, objects, "https://metadata.example")
	wallet, accessToken := gateway.SignIn(t)
	_, otherToken := gateway.SignIn(t)
	token := "0x" + strings.Repeat("a", 40)
	poolAddress := "0x" + strings.Repeat("b", 40)
	internaltest.SeedPool(t, pool, poolAddress, token, wallet)
	clock := time.UnixMilli(1735689600123)
	deps := metadata.Deps{
		Queries: sqlcdb.New(pool), LegacyDB: pool, Store: objects.Store,
		Identity: sharedauth.HTTPIdentityAuthority{Endpoint: gateway.URL, ServiceToken: gateway.ServiceToken, AllowPlainHTTP: true},
		Clock:    func() time.Time { return clock }, PoolLookupAttempts: 1,
		MaxLogoSize: 64, MaxBanner: 64,
	}
	router := chi.NewRouter()
	metadata.New(deps).RegisterRoutes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	unavailableRouter := chi.NewRouter()
	metadata.New(metadata.Deps{}).RegisterRoutes(unavailableRouter)
	unavailable := httptest.NewServer(unavailableRouter)
	t.Cleanup(unavailable.Close)
	productionRouter := chi.NewRouter()
	metadata.NewProductionLegacyReads(metadata.ConnectedMedia{}, "").RegisterRoutes(productionRouter)
	production := httptest.NewServer(productionRouter)
	t.Cleanup(production.Close)

	input, err := json.Marshal(map[string]any{
		"baseUrl": server.URL, "productionUrl": production.URL, "unavailableUrl": unavailable.URL,
		"wallet": wallet, "accessToken": accessToken, "otherToken": otherToken,
		"token": token, "poolAddress": poolAddress, "timestamp": clock.UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("node", "--experimental-strip-types", os.Getenv("METADATA_SDK_CONSUMER"))
	command.Stdin = bytes.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("metadata client: %v\n%s", err, output)
	}
}

func startObjects(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	key, secret := uuid.NewString(), uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, "minio", "server", "--address", address, "--console-address", "127.0.0.1:0", t.TempDir())
	command.Env = append(os.Environ(), "MINIO_ROOT_USER="+key, "MINIO_ROOT_PASSWORD="+secret)
	if err = command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { _ = command.Wait(); close(exited) }()
	t.Cleanup(func() { cancel(); <-exited })
	endpoint := "http://" + address
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for {
		select {
		case <-exited:
			t.Fatal("object server exited")
		default:
		}
		response, requestError := client.Get(endpoint + "/minio/health/live")
		if requestError == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("object server did not become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Setenv("METADATA_TEST_S3_ENDPOINT", endpoint)
	t.Setenv("METADATA_TEST_S3_ACCESS_KEY_ID", key)
	t.Setenv("METADATA_TEST_S3_SECRET_ACCESS_KEY", secret)
}

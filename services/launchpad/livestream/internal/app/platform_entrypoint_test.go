package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/streams"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// exerciseComposedPlatformEntrypoint runs the container's command with composed
// startup documents and the coordinator's runtime credential. The platform
// profile serves reads over private mTLS and keeps viewer sessions across restarts.
func exerciseComposedPlatformEntrypoint(t *testing.T, db coordinatortest.Database, admin *pgxpool.Pool) {
	t.Helper()
	root := coordinatortest.RepositoryRoot(t)
	values, output := t.TempDir(), t.TempDir()
	for _, name := range []string{"common.example.json", "livestream.example.json"} {
		raw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/documents/values", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(values, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime := db.Runtimes["livestream_runtime"]
	if err := os.WriteFile(filepath.Join(values, "expected-schema/livestream.json"), runtime.ExpectedSchema, 0o600); err != nil {
		t.Fatal(err)
	}
	compose := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--examples", values, "--out", output, "--service", "livestream")
	if raw, err := compose.CombinedOutput(); err != nil {
		t.Fatalf("compose startup documents: %v\n%s", err, raw)
	}
	raw, err := os.ReadFile(filepath.Join(output, "livestream.json"))
	if err != nil {
		t.Fatal(err)
	}
	environment := map[string]string{}
	if err = json.Unmarshal(raw, &environment); err != nil {
		t.Fatal(err)
	}
	if environment["EXPECTED_SCHEMA_SHA256"] != runtime.ExpectedSchemaSHA256 || environment["DB_BUDGET_DOCUMENT"] == "" {
		t.Fatal("composed startup documents do not bind the coordinator schema and budget")
	}
	exampleRaw, err := os.ReadFile(filepath.Join(values, "livestream.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var example struct {
		Endpoints struct {
			API string `json:"livepeer_api_url"`
		} `json:"endpoints"`
		Limits struct {
			Wallet int `json:"max_streams_per_wallet"`
			Global int `json:"max_global_streams"`
		} `json:"limits"`
		Periods struct {
			Refresh int `json:"secrets_refresh_seconds"`
		} `json:"periods"`
	}
	if err = json.Unmarshal(exampleRaw, &example); err != nil {
		t.Fatal(err)
	}
	pki := newPlatformPKI(t)
	freeAddress := func(network, address string) string {
		t.Helper()
		listener, err := net.Listen(network, address)
		if err != nil {
			t.Fatal(err)
		}
		value := listener.Addr().String()
		if err = listener.Close(); err != nil {
			t.Fatal(err)
		}
		return value
	}
	publicAddress := freeAddress("tcp6", "[::1]:0")
	_, publicPort, err := net.SplitHostPort(publicAddress)
	if err != nil {
		t.Fatal(err)
	}
	privateAddress := freeAddress("tcp", "127.0.0.1:0")
	for key, value := range map[string]string{
		"NODE_ENV": "production", "RUNTIME_PROFILE": "platform", "BIND_ADDRESS": "::", "PORT": publicPort,
		"ZONE": "zone-a", "SECRETS_PROVIDER": "env", "SECRETS_REFRESH_SECONDS": strconv.Itoa(example.Periods.Refresh),
		"DATABASE_URL": runtime.DSN, "REDIS_URL": os.Getenv("L40_TEST_REDIS_URL"), "DB_MAX_CONNS": "4",
		"LIVEPEER_PROVIDER_MODE": "unavailable", "LIVEPEER_API_URL": example.Endpoints.API,
		"MAX_STREAMS_PER_WALLET": strconv.Itoa(example.Limits.Wallet), "MAX_GLOBAL_STREAMS": strconv.Itoa(example.Limits.Global),
		"LIVESTREAM_VIEWER_TOKEN_KEY": platformViewerKey, "CORS_ALLOWED_ORIGINS": "https://web.example",
		"LIVESTREAM_PRIVATE_LISTEN_ADDRESS": privateAddress, "LIVESTREAM_TLS_CERT_FILE": pki.server.CertFile,
		"LIVESTREAM_TLS_KEY_FILE": pki.server.KeyFile, "LIVESTREAM_TLS_CLIENT_CA_FILE": pki.ca.CAFile,
		"GATEWAY_PEER_CERT_SHA256": pki.gatewayClient.SHA256,
	} {
		environment[key] = value
	}
	// Process configuration is explicit; fixture credentials and settings of the
	// parent process do not participate in the deployed service's configuration.
	configurationNames := map[string]bool{}
	configuration := reflect.TypeOf(config.Config{})
	for i := 0; i < configuration.NumField(); i++ {
		configurationNames[strings.Split(configuration.Field(i).Tag.Get("env"), ",")[0]] = true
	}
	var inherited []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !configurationNames[name] {
			inherited = append(inherited, entry)
		}
	}
	for name, value := range environment {
		inherited = append(inherited, name+"="+value)
	}
	binary := filepath.Join(t.TempDir(), "app")
	build := exec.Command("go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-o", binary, "./cmd/livestreamd")
	build.Dir = filepath.Join(root, "services/launchpad/livestream")
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off")
	if raw, err = build.CombinedOutput(); err != nil {
		t.Fatalf("compile container entrypoint: %v\n%s", err, raw)
	}
	privateClient := pki.client(pki.gatewayClient)
	t.Cleanup(privateClient.Transport.(*http.Transport).CloseIdleConnections)
	publicClient := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	t.Cleanup(publicClient.Transport.(*http.Transport).CloseIdleConnections)
	request := func(client *http.Client, method, target, body string, status int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, target, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != status {
			t.Fatalf("%s %s: status=%d body=%s read=%v close=%v", method, target, response.StatusCode, raw, readErr, closeErr)
		}
		return raw
	}
	start := func() func() {
		t.Helper()
		command := exec.Command(binary)
		command.Env = inherited
		var logs bytes.Buffer
		command.Stdout, command.Stderr = &logs, &logs
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		finished := make(chan struct{})
		var result error
		go func() { result = command.Wait(); close(finished) }()
		stopped := false
		stop := func() {
			t.Helper()
			if stopped {
				return
			}
			stopped = true
			_ = command.Process.Signal(syscall.SIGTERM)
			select {
			case <-finished:
				if result != nil {
					t.Errorf("entrypoint exit: %v\n%s", result, logs.String())
				}
			case <-time.After(15 * time.Second):
				_ = command.Process.Kill()
				<-finished
				t.Errorf("entrypoint did not drain\n%s", logs.String())
			}
		}
		t.Cleanup(stop)
		deadline := time.NewTimer(20 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-finished:
				t.Fatalf("entrypoint exited before ready: %v\n%s", result, logs.String())
			case <-deadline.C:
				t.Fatal("entrypoint readiness deadline")
			case <-ticker.C:
				response, err := publicClient.Get("http://" + publicAddress + "/health/ready")
				if err != nil {
					continue
				}
				raw, readErr := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if readErr == nil && response.StatusCode == http.StatusOK && bytes.Contains(raw, []byte(`"stream_read_schema":"ok"`)) {
					request(privateClient, http.MethodGet, "https://"+privateAddress+"/health/ready", "", http.StatusOK)
					return stop
				}
			}
		}
	}
	stop := start()
	privateURL := "https://" + privateAddress
	page := request(privateClient, http.MethodGet, privateURL+"/v2/streams", "", http.StatusOK)
	if err = v2.ValidateJSON("StreamListResponse", page); err != nil {
		t.Fatal(err)
	}
	var first v2.StreamListResponse
	if err = json.Unmarshal(page, &first); err != nil || len(first.Data.Items) != 0 {
		t.Fatalf("empty directory: %s %v", page, err)
	}
	request(publicClient, http.MethodGet, "http://"+publicAddress+"/v2/streams", "", http.StatusForbidden)
	ended, live := uuid4(t), uuid4(t)
	wallet, pool := address(t), address(t)
	insertOperation(t, admin, ended, wallet, pool, "ENDED")
	read := func() {
		t.Helper()
		raw := request(privateClient, http.MethodGet, privateURL+"/v2/streams/"+ended, "", http.StatusOK)
		var document streams.V2Stream
		if err := json.Unmarshal(raw, &document); err != nil || document.StreamID != ended || document.Status != "ended" {
			t.Fatalf("persisted stream: %s %v", raw, err)
		}
	}
	read()
	insertOperation(t, admin, live, wallet, pool, "LIVE")
	var session struct {
		SessionID string `json:"sessionId"`
		Token     string `json:"token"`
	}
	if err = json.Unmarshal(request(privateClient, http.MethodPost, privateURL+"/v2/streams/"+live+"/viewer-session", "{}", http.StatusOK), &session); err != nil || session.SessionID == "" || session.Token == "" {
		t.Fatalf("viewer session: %+v %v", session, err)
	}
	stop()
	stop = start()
	defer stop()
	read()
	body, err := json.Marshal(map[string]string{"sessionId": session.SessionID, "token": session.Token})
	if err != nil {
		t.Fatal(err)
	}
	raw = request(privateClient, http.MethodPost, privateURL+"/v2/streams/"+live+"/heartbeat", string(body), http.StatusOK)
	if !bytes.Contains(raw, []byte(`"viewerCount":1`)) {
		t.Fatalf("viewer session after restart: %s", raw)
	}
	var goose int
	if err = admin.QueryRow(context.Background(), "SELECT count(*) FROM pg_class WHERE relname='goose_db_version'").Scan(&goose); err != nil || goose != 0 {
		t.Fatalf("runtime migrated the coordinator database: %d %v", goose, err)
	}
}

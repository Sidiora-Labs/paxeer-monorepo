package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/brokerprovision"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func accountDeploymentLifecycle(t *testing.T, transport config.Config) {
	t.Helper()
	ctx := context.Background()
	root := coordinatortest.RepositoryRoot(t)
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "users_runtime")
	runtime := database.Runtimes["users_runtime"]
	dbConfig, err := pgx.ParseConfig(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	values, output := t.TempDir(), t.TempDir()
	write := func(path string, raw []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	encode := func(value any) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	for _, pair := range [][2]*string{
		{&transport.AccountTLSCertFile, &transport.AccountTLSCertPEM},
		{&transport.AccountTLSKeyFile, &transport.AccountTLSKeyPEM},
		{&transport.AccountTLSCAFile, &transport.AccountTLSCAPEM},
		{&transport.AccountNATSCredentialsFile, &transport.AccountNATSCreds},
		{&transport.AccountNATSCAFile, &transport.AccountNATSCAPEM},
		{&transport.AccountNATSCertFile, &transport.AccountNATSCertPEM},
		{&transport.AccountNATSKeyFile, &transport.AccountNATSKeyPEM},
	} {
		raw, err := config.Material(*pair[0], *pair[1])
		if err != nil {
			t.Fatal(err)
		}
		*pair[0] = filepath.Join(t.TempDir(), "material")
		write(*pair[0], raw)
		*pair[1] = ""
	}
	transport.AccountSubjectsFile, transport.AccountSubjectsDocument, transport.AccountSubjectsSHA256 = "", "", ""
	documents := filepath.Join(root, "deploy/launchpad/documents")
	var budget dbproduction.ConnectionBudget
	if err := json.Unmarshal(read(filepath.Join(documents, "values/common.example.json")), &budget); err != nil {
		t.Fatal(err)
	}
	for i := range budget.Targets {
		budget.Targets[i].Host, budget.Targets[i].Port = dbConfig.Host, dbConfig.Port
	}
	for i := range budget.Clients {
		client := &budget.Clients[i]
		if client.Role == "userd" {
			client.Transport, client.Pooler = shareddb.TransportDirect, ""
			client.Alias, client.User, client.MaxConnections = dbConfig.Database, runtime.Role, 8
		}
	}
	write(filepath.Join(values, "common.json"), encode(budget))
	write(filepath.Join(values, "user.example.json"), read(filepath.Join(documents, "values/user.example.json")))
	write(filepath.Join(values, "expected-schema/user.json"), runtime.ExpectedSchema)
	compose := exec.Command("python3", filepath.Join(documents, "compose.py"), "--examples", values, "--out", output, "--service", "user")
	if raw, err := compose.CombinedOutput(); err != nil {
		t.Fatalf("compose user documents: %v: %s", err, raw)
	}
	var composed map[string]string
	if err := json.Unmarshal(read(filepath.Join(output, "user.json")), &composed); err != nil {
		t.Fatal(err)
	}
	var subjects map[string]string
	var example struct {
		Subjects map[string]string `json:"subjects"`
	}
	if err := json.Unmarshal(read(filepath.Join(values, "user.example.json")), &example); err != nil {
		t.Fatal(err)
	}
	subjects = example.Subjects
	var brokerExample brokerprovision.Document
	if err := json.Unmarshal(read(filepath.Join(documents, "values/broker.example.json")), &brokerExample); err != nil {
		t.Fatal(err)
	}
	stream := brokerExample.Streams[0]
	stream.Name, stream.Subjects = transport.AccountNATSStream, []string{}
	for _, subject := range subjects {
		stream.Subjects = append(stream.Subjects, subject)
	}
	sort.Strings(stream.Subjects)
	brokerDocument := filepath.Join(t.TempDir(), "broker.json")
	write(brokerDocument, encode(brokerprovision.Document{Streams: []brokerprovision.Stream{stream}, Durables: []natsdriver.ConsumerConfig{}}))
	provision := exec.Command("go", "run", filepath.Join(root, "sdk/launchpad/shared/brokerprovision/cmd/brokerprovision"), "provision", "--document", brokerDocument, "--broker-env", "ACCOUNT_NATS_URL", "--token-env", "ACCOUNT_PROVISION_TOKEN")
	provision.Env = append(os.Environ(), "SSL_CERT_FILE="+transport.AccountTLSCAFile, "ACCOUNT_NATS_URL="+transport.AccountNATSURL, "ACCOUNT_PROVISION_TOKEN=test-only-provision")
	if raw, err := provision.CombinedOutput(); err != nil {
		t.Fatalf("provision user broker: %v: %s", err, raw)
	}

	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	if err := json.Unmarshal(read(filepath.Join(root, "deploy/launchpad/railway/variables.json")), &contract); err != nil {
		t.Fatal(err)
	}
	fields, source := reflect.TypeOf(transport), reflect.ValueOf(transport)
	for i := 0; i < fields.NumField(); i++ {
		name := strings.Split(fields.Field(i).Tag.Get("env"), ",")[0]
		if name == "" {
			continue
		}
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		if value := source.Field(i); !value.IsZero() {
			t.Setenv(name, fmt.Sprint(value.Interface()))
		}
	}
	for name, value := range contract["user"].Fixed {
		t.Setenv(name, value)
	}
	t.Setenv("DATABASE_URL", runtime.DSN)
	t.Setenv("REDIS_URL", os.Getenv("USER_TEST_REDIS_URL"))
	t.Setenv("DB_POOL_ID", "userd_pool")
	t.Setenv("INSTANCE_ID", "user-deployment")
	t.Setenv("ACCOUNT_DELIVERY_SCHEMA", "users")
	t.Setenv("ACCOUNT_ASSERTION_AUDIENCE", "account")
	t.Setenv("USER_PRIVATE_LISTEN_ADDRESS", accountDeploymentAddress(t))
	t.Setenv("ZONE", "zone-a")
	for name, value := range composed {
		t.Setenv(name, value)
	}
	for _, name := range contract["user"].Required {
		if os.Getenv(name) == "" {
			t.Fatalf("deployment input %s is empty", name)
		}
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	broken := cfg
	broken.AccountNATSURL = "tls://localhost:1"
	for i := 0; i < 2; i++ {
		application, err := New(ctx, broken, logger)
		if err == nil {
			application.Close()
			t.Fatal("unavailable account broker accepted")
		}
		if !strings.Contains(err.Error(), "nats:") {
			t.Fatalf("account broker startup response = %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		application, err := New(ctx, cfg, logger)
		if err != nil {
			t.Fatalf("open account instance %d: %v", i, err)
		}
		application.Close()
	}
	pool, err := cfg.OpenRuntimePool(ctx, "userd", shareddb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer shareddb.ClosePool(pool)
	var role, schema string
	if err := pool.QueryRow(ctx, "SELECT current_user,current_schema()").Scan(&role, &schema); err != nil || role != runtime.Role || schema != "users" {
		t.Fatalf("runtime database route = %s/%s: %v", role, schema, err)
	}
	store := account.Store{DB: pool}
	owner := account.Principal{AccountID: uuid.New()}
	wallet := "0x1111111111111111111111111111111111111111"
	if err := store.Create(ctx, owner.AccountID, wallet); err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{}, EventTypes: map[string]bool{}}
	for kind, subject := range subjects {
		policy.Subjects[subject], policy.EventTypes[kind] = true, true
	}
	durable, err := delivery.NewStore(pool, "users", policy)
	if err != nil {
		t.Fatal(err)
	}
	handler := (account.HTTP{Store: store, Runtime: &account.Runtime{Delivery: durable, Subjects: subjects, Metrics: &account.Metrics{}}}).Handler()
	request := httptest.NewRequest(http.MethodPatch, "/v1/profiles/"+wallet, strings.NewReader(`{"expectedRevision":"0","displayName":"Builder"}`))
	request = request.WithContext(account.WithPrincipal(request.Context(), owner))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("publish account profile = %d %s", response.Code, response.Body.String())
	}
	shareddb.ClosePool(pool)

	binary := filepath.Join(t.TempDir(), "userd")
	build := exec.Command("go", "build", "-o", binary, "./cmd/userd")
	build.Dir = filepath.Join(root, "services/launchpad/user")
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build user entrypoint: %v: %s", err, raw)
	}
	certificate, roots, err := materialTLS(cfg.AccountTLSCertFile, cfg.AccountTLSCertPEM, cfg.AccountTLSKeyFile, cfg.AccountTLSKeyPEM, cfg.AccountTLSCAFile, cfg.AccountTLSCAPEM)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{certificate}}}}
	defer client.CloseIdleConnections()
	for restart := 0; restart < 2; restart++ {
		command := exec.Command(binary)
		var logs bytes.Buffer
		command.Stdout, command.Stderr = &logs, &logs
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		exited := make(chan error, 1)
		go func() { exited <- command.Wait() }()
		stopped := false
		t.Cleanup(func() {
			if !stopped {
				_ = command.Process.Kill()
				<-exited
			}
		})
		ready := false
		for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
			select {
			case err := <-exited:
				stopped = true
				t.Fatalf("user entrypoint exited: %v: %s", err, logs.String())
			default:
			}
			res, err := client.Get("http://127.0.0.1:8080/health/ready")
			if err == nil {
				var health struct {
					Status string            `json:"status"`
					Checks map[string]string `json:"checks"`
				}
				err = json.NewDecoder(res.Body).Decode(&health)
				res.Body.Close()
				if err == nil && res.StatusCode == http.StatusOK && health.Status == "ok" && health.Checks["account_workers"] == "ok" && health.Checks["database"] == "ok" {
					ready = true
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !ready {
			_ = command.Process.Kill()
			<-exited
			stopped = true
			t.Fatalf("user readiness did not become ok: %s", logs.String())
		}
		res, err := client.Get("https://" + cfg.PrivateListenAddress + "/v2/profiles/" + wallet)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK || interfaces.ValidateJSON("ProfileGetResponse", raw) != nil {
			t.Fatalf("profile after restart %d = %d %s: %v", restart, res.StatusCode, raw, err)
		}
		var answer struct {
			Data struct {
				Profile account.PublicProfile `json:"profile"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &answer); err != nil || answer.Data.Profile.DisplayName != "Builder" || answer.Data.Profile.Revision != "1" {
			t.Fatalf("stored profile = %s: %v", raw, err)
		}
		if err := command.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-exited:
			stopped = true
			if err != nil {
				t.Fatalf("user shutdown = %v: %s", err, logs.String())
			}
		case <-time.After(15 * time.Second):
			t.Fatal("user shutdown did not drain")
		}
	}
	options, err := accountNATSOptions(cfg)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := nats.Connect(cfg.AccountNATSURL, options...)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	js, err := jetstream.New(connection)
	if err != nil {
		t.Fatal(err)
	}
	streamHandle, err := js.Stream(ctx, cfg.AccountNATSStream)
	if err != nil {
		t.Fatal(err)
	}
	message, err := streamHandle.GetLastMsgForSubject(ctx, subjects["profile.updated"])
	if err != nil {
		t.Fatal(err)
	}
	event, err := delivery.Decode(message.Data, policy, message.Subject)
	if err != nil || event.EventType != "profile.updated" || event.SourceOrder.Height != "1" {
		t.Fatalf("profile event = %+v: %v", event, err)
	}
	var payload struct {
		AccountID uuid.UUID `json:"accountId"`
		Revision  string    `json:"revision"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.AccountID != owner.AccountID || payload.Revision != "1" {
		t.Fatalf("profile event payload = %s: %v", event.Payload, err)
	}
}

func accountDeploymentAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/brokerprovision"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func assertComposedInventoryLifecycle(t *testing.T, root string, database coordinatortest.Database, cfg config.Config) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	values, output := t.TempDir(), t.TempDir()
	read := func(path string, out any) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, out); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name string, value any) string {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(values, name)
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	var budget production.ConnectionBudget
	read(filepath.Join(root, "deploy/launchpad/documents/values/common.example.json"), &budget)
	route, err := url.Parse(database.Runtimes["pnl_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(route.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	budget.Targets[0].Host, budget.Targets[0].Port = route.Hostname(), uint16(port)
	for i := range budget.Clients {
		if budget.Clients[i].Role == "pnld" {
			budget.Clients[i].Transport = shareddb.TransportDirect
			budget.Clients[i].Pooler = ""
			budget.Clients[i].Alias = strings.TrimPrefix(route.Path, "/")
			budget.Clients[i].User = route.User.Username()
			budget.Clients[i].MaxConnections = 2
		}
	}
	write("common.json", budget)
	if err = os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(values, "expected-schema/pnl.json"), database.Runtimes["pnl_runtime"].ExpectedSchema, 0600); err != nil {
		t.Fatal(err)
	}
	var pnlValues map[string]any
	read(filepath.Join(root, "deploy/launchpad/documents/values/pnl.example.json"), &pnlValues)
	stream := "PNL_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	pnlValues["region"] = coordinatortest.Region
	pnlValues["epoch"] = coordinatortest.Epoch
	pnlValues["consumer"].(map[string]any)["stream"] = stream
	write("pnl.json", pnlValues)
	composer := exec.CommandContext(ctx, "python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--examples", values, "--service", "pnl", "--out", output)
	composer.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if raw, err := composer.CombinedOutput(); err != nil {
		t.Fatalf("compose inventory deployment: %v: %s", err, raw)
	}
	var documents map[string]string
	read(filepath.Join(output, "pnl.json"), &documents)
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	read(filepath.Join(root, "deploy/launchpad/railway/variables.json"), &contract)
	credentials := filepath.Join(values, "pnl.creds")
	if err = os.WriteFile(credentials, []byte(cfg.NATSCredentialsCreds), 0600); err != nil {
		t.Fatal(err)
	}
	environment := map[string]string{}
	for name, value := range contract["pnl"].Fixed {
		environment[name] = value
	}
	for name, value := range map[string]string{
		"DATABASE_URL": database.Runtimes["pnl_runtime"].DSN,
		"DB_POOL_ID":   "pnld_pool", "INSTANCE_ID": "pnl-1",
		"NATS_URL": cfg.NATSURL, "NATS_CREDENTIALS": credentials,
		"REDIS_URL": cfg.RedisURL, "REDIS_BULL_URL": cfg.RedisURL,
		"RPC_URL": cfg.RPCURL, "RPC_URL_FALLBACK": cfg.RPCURLFallback,
		"WEBHOOK_HMAC_SECRET": cfg.WebhookHMACSecret, "ZONE": cfg.RuntimeZone,
	} {
		environment[name] = value
	}
	for name, value := range documents {
		environment[name] = value
	}
	for _, name := range contract["pnl"].Required {
		if environment[name] == "" {
			t.Fatalf("required deployment variable missing: %s", name)
		}
	}
	known := map[string]bool{}
	var fields func(reflect.Type)
	fields = func(typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Anonymous {
				fields(field.Type)
			} else if name := strings.Split(field.Tag.Get("env"), ",")[0]; name != "" {
				known[name] = true
			}
		}
	}
	fields(reflect.TypeOf(config.Config{}))
	childEnv := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !known[name] {
			childEnv = append(childEnv, entry)
		}
	}
	for name, value := range environment {
		childEnv = append(childEnv, name+"="+value)
	}
	var broker brokerprovision.Document
	read(filepath.Join(root, "deploy/launchpad/documents/values/broker.example.json"), &broker)
	selected := brokerprovision.Document{}
	for _, durable := range broker.Durables {
		if durable.Durable == "pnl_inventory" {
			original := durable.Stream
			durable.Stream = stream
			selected.Durables = append(selected.Durables, durable)
			for _, item := range broker.Streams {
				if item.Name == original {
					item.Name = stream
					selected.Streams = append(selected.Streams, item)
				}
			}
		}
	}
	if len(selected.Streams) != 1 || len(selected.Durables) != 1 {
		t.Fatal("inventory broker resources must be declared")
	}
	brokerPath := write("broker.json", selected)
	for name, value := range map[string]string{"PNL_DEPLOYMENT_BROKER": cfg.NATSURL, "PNL_DEPLOYMENT_CREDS": cfg.NATSCredentialsCreds} {
		t.Setenv(name, value)
	}
	provision := func() {
		t.Helper()
		var report bytes.Buffer
		if status := brokerprovision.Run(ctx, []string{"provision", "--document", brokerPath, "--broker-env", "PNL_DEPLOYMENT_BROKER", "--credentials-env", "PNL_DEPLOYMENT_CREDS"}, &report, &report); status != 0 {
			t.Fatalf("inventory provisioning status=%d: %s", status, report.String())
		}
		if status := brokerprovision.Run(ctx, []string{"check", "--document", brokerPath, "--broker-env", "PNL_DEPLOYMENT_BROKER", "--credentials-env", "PNL_DEPLOYMENT_CREDS"}, &report, &report); status != 0 {
			t.Fatalf("inventory broker status=%d: %s", status, report.String())
		}
	}
	provision()
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := js.DeleteStream(context.Background(), stream); err != nil {
			t.Error(err)
		}
	}()
	pool, err := pgxpool.New(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	wallet := strings.ToLower(common.BytesToAddress(crypto.Keccak256([]byte(uuid.NewString()))).Hex())
	poolAddress := strings.ToLower(common.BytesToAddress(crypto.Keccak256([]byte(uuid.NewString()))).Hex())
	token := strings.ToLower(common.BytesToAddress(crypto.Keccak256([]byte(uuid.NewString()))).Hex())
	st := store.New(pool)
	if inserted, err := st.FoldTrade(ctx, store.TradeInput{ID: "deployment-0", UserAddress: wallet, PoolAddress: poolAddress, TokenAddress: token, IsBuy: true, UsdlAmount: "1000000", TokenAmount: "1000000000000000000", Price: "1000000", Fee: "0", BlockNumber: 1, BlockTimestamp: 1, TxHash: "deployment"}); err != nil || !inserted {
		t.Fatal(inserted, err)
	}
	binary := filepath.Join(t.TempDir(), "pnld")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/pnld")
	build.Dir = filepath.Join(root, "services/launchpad/pnl-tracker")
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile service entrypoint: %v: %s", err, raw)
	}
	client := &http.Client{Timeout: time.Second}
	get := func(path string) (int, []byte, error) {
		response, err := client.Get("http://127.0.0.1:" + environment["PORT"] + path)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return response.StatusCode, raw, err
	}
	var first []byte
	for restart := 0; restart < 3; restart++ {
		func() {
			process := exec.CommandContext(ctx, binary)
			process.Env = childEnv
			process.Dir = output
			log, err := os.CreateTemp(t.TempDir(), "pnld-*")
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close()
			process.Stdout, process.Stderr = log, log
			if err = process.Start(); err != nil {
				t.Fatal(err)
			}
			exited := false
			done := make(chan error, 1)
			go func() { done <- process.Wait() }()
			defer func() {
				if !exited {
					_ = process.Process.Kill()
					<-done
				}
			}()
			ready := false
			for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
				if status, raw, err := get("/health/ready"); err == nil && status == 200 {
					var health struct{ Checks map[string]string }
					if err = json.Unmarshal(raw, &health); err != nil || health.Checks["broker"] != "ok" || health.Checks["database"] != "ok" || health.Checks["redis"] != "ok" {
						t.Fatal(string(raw), err)
					}
					ready = true
					break
				}
				select {
				case err := <-done:
					exited = true
					raw, _ := os.ReadFile(log.Name())
					t.Fatalf("entrypoint exited before readiness: %v: %s", err, raw)
				case <-time.After(20 * time.Millisecond):
				}
			}
			if !ready {
				raw, _ := os.ReadFile(log.Name())
				t.Fatalf("entrypoint readiness unavailable: %s", raw)
			}
			status, raw, err := get("/users/" + wallet + "/positions")
			var positions struct {
				User      string
				Positions []store.PositionRow
			}
			if err != nil || status != 200 || json.Unmarshal(raw, &positions) != nil || positions.User != wallet || len(positions.Positions) != 1 || positions.Positions[0].CurrentHoldings != "1000000000000000000" || positions.Positions[0].TotalUsdlSpent != "1000000" || positions.Positions[0].TokenAddress != token {
				t.Fatal(status, string(raw), err)
			}
			if restart == 0 {
				first = append([]byte(nil), raw...)
			} else if !bytes.Equal(first, raw) {
				t.Fatal("persisted position changed across restart", string(raw))
			}
			if restart == 1 {
				if err = js.DeleteConsumer(ctx, stream, "pnl_inventory"); err != nil {
					t.Fatal(err)
				}
			} else if err = process.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-done:
				exited = true
				if restart == 1 {
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != 1 {
						t.Fatalf("inventory failure must exit 1: %v", err)
					}
					raw, _ := os.ReadFile(log.Name())
					if !bytes.Contains(raw, []byte("inventory consumer failed")) || !bytes.Contains(raw, []byte("consumer deleted")) {
						t.Fatalf("inventory failure cause: %s", raw)
					}
				} else if err != nil {
					t.Fatalf("graceful shutdown: %v", err)
				}
			case <-time.After(sharedprocess.PropagationDelay + 10*time.Second):
				t.Fatal("entrypoint did not stop")
			}
		}()
		if restart == 1 {
			provision()
		}
	}
}

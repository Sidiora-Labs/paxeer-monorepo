package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/brokerprovision"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

func exerciseComposedProductionStartup(t *testing.T, documents map[string]RenderedDocument) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := coordinatortest.RepositoryRoot(t)
	module := filepath.Join(root, "services/launchpad/indexer")
	database := coordinatortest.Bootstrap(t, os.Getenv("L06_POSTGRES_DSN"), "indexer_runtime")
	runtime := database.Runtimes["indexer_runtime"]
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rpcURL := startupRecordedChain(t, ctx)
	brokerURL, credentials := status1695Broker(t)
	values := t.TempDir()
	read := func(path string, value any) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, value); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var inputs map[string]any
	read(filepath.Join(root, "deploy/launchpad/documents/values/indexer.example.json"), &inputs)
	var receiptDoc receiptRuntimeDocument
	if err := json.Unmarshal(documents["receipt.json"].Bytes, &receiptDoc); err != nil {
		t.Fatal(err)
	}
	inputs["deployment_id"] = receiptDoc.Production.Manifest.DeploymentVersion
	inputs["manifest_digest"] = receiptDoc.Production.Manifest.SourceManifestSHA256
	inputs["anchor_height"] = receiptDoc.Production.Anchor.Height
	inputs["anchor_hash"] = strings.ToLower(receiptDoc.Production.Anchor.Hash.Hex())
	inputs["region"] = coordinatortest.Region
	inputs["owner"] = "composed-indexer"
	for _, name := range []string{"receipt", "lifecycle"} {
		path := filepath.Join(values, name+".json")
		if err := os.WriteFile(path, documents[name+".json"].Bytes, 0600); err != nil {
			t.Fatal(err)
		}
		inputs[name+"_rendered_file"] = name + ".json"
		inputs[name+"_runtime_path"] = path
	}
	write(filepath.Join(values, "indexer.json"), inputs)
	var budget production.ConnectionBudget
	read(filepath.Join(root, "deploy/launchpad/documents/values/common.example.json"), &budget)
	parsed, err := url.Parse(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	budget.Targets[0].Host, budget.Targets[0].Port = parsed.Hostname(), uint16(port)
	budget.Targets[0].MaxConnections = 1000
	budget.Poolers = nil
	for i := range budget.Clients {
		client := &budget.Clients[i]
		client.Transport, client.Pooler, client.QueryClass = shareddb.TransportDirect, "", shareddb.Batch
		if client.Role == "indexerd" {
			client.Alias, client.User, client.MaxConnections = database.Name, runtime.Role, 3
		}
	}
	write(filepath.Join(values, "common.json"), budget)
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "expected-schema/indexer.json"), runtime.ExpectedSchema, 0600); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	compose := exec.CommandContext(ctx, "python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--examples", values, "--out", out, "--service", "indexer")
	if output, err := compose.CombinedOutput(); err != nil {
		t.Fatalf("compose production documents: %v: %s", err, output)
	}
	var environment map[string]string
	read(filepath.Join(out, "indexer.json"), &environment)
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	read(filepath.Join(root, "deploy/launchpad/railway/variables.json"), &contract)
	for key, value := range contract["indexer"].Fixed {
		environment[key] = value
	}
	credentialsFile := filepath.Join(values, "broker.creds")
	if err := os.WriteFile(credentialsFile, []byte(credentials), 0600); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"DATABASE_URL": runtime.DSN, "DB_POOL_ID": "indexerd_pool", "INSTANCE_ID": "composed-indexer",
		"ZONE": "zone-a", "RPC_URL": rpcURL, "NATS_URL": brokerURL, "NATS_CREDENTIALS": credentialsFile,
		"REDIS_URL": os.Getenv("INDEXER_TEST_REDIS_URL"), "REDIS_BULL_URL": os.Getenv("INDEXER_TEST_REDIS_URL"),
		"INDEXER_BATCH_SIZE": "1",
	} {
		environment[key] = value
	}
	for _, key := range contract["indexer"].Required {
		if environment[key] == "" {
			t.Fatalf("deployment requires %s", key)
		}
	}
	var deliveryDoc deliveryRuntimeDocument
	// Load the exact composed bytes through the service's document reader.
	for _, key := range startupConfigurationKeys() {
		t.Setenv(key, "")
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err = config.ReadBoundDocument(cfg.DeliveryConfigFile, cfg.DeliveryConfigSHA256, &deliveryDoc); err != nil {
		t.Fatal(err)
	}
	broker := provisionStartupBroker(t, ctx, root, brokerURL, credentialsFile, deliveryDoc.Stream)
	nc, err := nats.Connect(brokerURL, nats.UserCredentials(credentialsFile))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	var creationConsumer *natsdriver.Consumer
	for _, binding := range broker.Durables {
		if binding.Durable == "stats_search_creation" {
			creationConsumer, err = natsdriver.NewConsumer(ctx, nc, binding)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if creationConsumer == nil {
		t.Fatal("canonical creation durable unavailable")
	}
	defer creationConsumer.Close()
	var fence int64
	binary := filepath.Join(t.TempDir(), "app")
	build := exec.CommandContext(ctx, "go", "build", "-p", "1", "-mod=readonly", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", binary, "./cmd/indexerd")
	build.Dir = module
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile container entrypoint: %v: %s", err, output)
	}
	client := &http.Client{Timeout: time.Second}
	for restart := range 2 {
		command := exec.CommandContext(ctx, binary)
		command.Dir = module
		command.Env = startupEnvironment(environment)
		log, err := os.Create(filepath.Join(t.TempDir(), "indexer.log"))
		if err != nil {
			t.Fatal(err)
		}
		command.Stdout, command.Stderr = log, log
		if err := command.Start(); err != nil {
			log.Close()
			t.Fatal(err)
		}
		done := make(chan struct{})
		var exit error
		go func() { exit = command.Wait(); close(done) }()
		t.Cleanup(func() {
			_ = command.Process.Kill()
			<-done
			_ = log.Close()
			if t.Failed() {
				if raw, err := os.ReadFile(log.Name()); err == nil {
					t.Log(string(raw))
				}
			}
		})
		deadline := time.NewTimer(30 * time.Second)
		for {
			ready := startupResponse(client, "/health/ready")
			status := startupResponse(client, "/status")
			if ready["status"] == "ok" && status["lastProcessedBlock"] == float64(33963717) {
				checks, ok := ready["checks"].(map[string]any)
				if !ok || checks["broker"] != "ok" || checks["ingestion"] != "ok" || checks["canonical_source"] != "ok" || status["mode"] != "canonical" || status["canonicalStreamId"] != deliveryDoc.CanonicalStreamID || status["totalPoolsIndexed"] != float64(1) || status["totalSwapsIndexed"] != float64(0) {
					t.Fatalf("production readiness=%v status=%v", ready, status)
				}
				break
			}
			select {
			case <-done:
				t.Fatalf("entrypoint ended before readiness: %v", exit)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-deadline.C:
				t.Fatalf("production readiness=%v status=%v", ready, status)
			case <-time.After(20 * time.Millisecond):
			}
		}
		deadline.Stop()
		if restart == 0 {
			messages, err := creationConsumer.Fetch(ctx)
			if err != nil || len(messages) != 1 {
				t.Fatalf("canonical creation delivery: %d: %v", len(messages), err)
			}
			message := messages[0]
			envelope, err := delivery.Decode(message.Wire, deliveryDoc.Policy, message.Subject)
			if err != nil || envelope.EventID != message.EventID || envelope.EventType != "canonical_creation" || envelope.SourceOrder.Height != "33963716" {
				t.Fatalf("canonical creation envelope: %+v: %v", envelope, err)
			}
			var fact store.CreationFact
			if err := json.Unmarshal(envelope.Payload, &fact); err != nil || fact.Token != "0x0f92fdd087c96dd3413ef98646b94d0d986192fa" || fact.Pool != "0xce20da50f92d21d8bee497910c782e9aa25b6263" || fact.Metadata == nil || fact.Provenance.DeploymentID != deliveryDoc.DeploymentID {
				t.Fatalf("canonical creation payload: %+v: %v", fact, err)
			}
			var same bool
			if err := pool.QueryRow(ctx, `SELECT payload=$2::jsonb FROM indexer.indexer_canonical_creations WHERE token=$1`, fact.Token, []byte(envelope.Payload)).Scan(&same); err != nil || !same {
				t.Fatalf("canonical creation source: %v: %v", same, err)
			}
			if err := message.Ack(ctx); err != nil {
				t.Fatal(err)
			}
		} else {
			messages, err := creationConsumer.Fetch(ctx)
			if err != nil || len(messages) != 0 {
				t.Fatalf("restart repeated canonical creation: %d: %v", len(messages), err)
			}
		}
		var currentFence int64
		if err := pool.QueryRow(ctx, `SELECT fence_token FROM indexer.indexer_ingest_leases WHERE chain_id=125 AND stream_id=$1 AND expires_at>clock_timestamp()`, deliveryDoc.CanonicalStreamID).Scan(&currentFence); err != nil || currentFence <= fence {
			t.Fatalf("restart %d canonical lease %d after %d: %v", restart, currentFence, fence, err)
		}
		fence = currentFence
		if err := command.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case <-done:
			if exit != nil {
				t.Fatalf("container entrypoint shutdown: %v", exit)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		var held bool
		if err := pool.QueryRow(ctx, `SELECT expires_at>clock_timestamp() FROM indexer.indexer_ingest_leases WHERE chain_id=125 AND stream_id=$1`, deliveryDoc.CanonicalStreamID).Scan(&held); err != nil || held {
			t.Fatalf("shutdown retained canonical lease: %v: %v", held, err)
		}
		var committed int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_ingest_commits WHERE chain_id=125 AND stream_id=$1`, deliveryDoc.CanonicalStreamID).Scan(&committed); err != nil || committed != 2 {
			t.Fatalf("restart duplicated canonical progress: %d: %v", committed, err)
		}
		var publications int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM indexer.delivery_outbox WHERE subject='canonical.creation.v1' AND state='published'`).Scan(&publications); err != nil || publications != 1 {
			t.Fatalf("canonical creation publication count: %d: %v", publications, err)
		}
		listener, err := net.Listen("tcp", "0.0.0.0:8080")
		if err != nil {
			t.Fatalf("shutdown retained container listener: %v", err)
		}
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func startupEnvironment(values map[string]string) []string {
	clean := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		clean[key] = value
	}
	for _, key := range startupConfigurationKeys() {
		delete(clean, key)
	}
	for key, value := range values {
		clean[key] = value
	}
	result := make([]string, 0, len(clean))
	for key, value := range clean {
		result = append(result, key+"="+value)
	}
	return result
}

func startupConfigurationKeys() []string {
	var keys []string
	var collect func(reflect.Type)
	collect = func(typ reflect.Type) {
		for i := range typ.NumField() {
			field := typ.Field(i)
			if field.Anonymous && field.Type.Kind() == reflect.Struct {
				collect(field.Type)
			}
			key, _, _ := strings.Cut(field.Tag.Get("env"), ",")
			if key != "" {
				keys = append(keys, key)
			}
		}
	}
	collect(reflect.TypeOf(config.Config{}))
	return keys
}

func startupResponse(client *http.Client, path string) map[string]any {
	response, err := client.Get("http://127.0.0.1:8080" + path)
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil || response.StatusCode != http.StatusOK {
		return nil
	}
	return body
}

func startupRecordedChain(t *testing.T, ctx context.Context) string {
	t.Helper()
	t.Setenv(chainfixture.RecordEnv, "")
	endpoint := marketRecoveryServe(t, chainfixture.New(t))
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	// An empty block has the same transaction body for either RPC encoding.
	var anchor struct{ Transactions []json.RawMessage }
	if err := client.Client().CallContext(ctx, &anchor, "eth_getBlockByNumber", "0x2063ec3", false); err != nil || anchor.Transactions == nil || len(anchor.Transactions) != 0 {
		t.Fatalf("recorded empty anchor required: %v", err)
	}
	target, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	forward := httputil.NewSingleHostReverseProxy(target)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			t.Error(err)
			return
		}
		var call struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(raw, &call); err != nil {
			t.Error(err)
			return
		}
		if call.Method == "eth_getBlockByNumber" && len(call.Params) == 2 && string(call.Params[0]) == `"latest"` {
			call.Params[0], call.Params[1] = json.RawMessage(`"0x2063ec5"`), json.RawMessage("true")
			raw, err = json.Marshal(call)
			if err != nil {
				t.Error(err)
				return
			}
		}
		if call.Method == "eth_getBlockByNumber" && len(call.Params) == 2 && string(call.Params[0]) == `"0x2063ec3"` && string(call.Params[1]) == "true" {
			call.Params[1] = json.RawMessage("false")
			raw, err = json.Marshal(call)
			if err != nil {
				t.Error(err)
				return
			}
		}
		if call.Method == "eth_getLogs" && len(call.Params) == 1 {
			var filter map[string]json.RawMessage
			if err := json.Unmarshal(call.Params[0], &filter); err != nil {
				t.Error(err)
				return
			}
			if string(filter["fromBlock"]) == `"0x2063ec5"` && string(filter["toBlock"]) == `"0x2063ec5"` {
				var addresses []string
				if err := json.Unmarshal(filter["address"], &addresses); err != nil {
					t.Error(err)
					return
				}
				for _, address := range []string{"0x0f92fdd087c96dd3413ef98646b94d0d986192fa", "0xce20da50f92d21d8bee497910c782e9aa25b6263"} {
					found := false
					for _, existing := range addresses {
						found = found || existing == address
					}
					if !found {
						addresses = append(addresses, address)
					}
				}
				sort.Strings(addresses)
				filter["address"], err = json.Marshal(addresses)
				if err == nil {
					call.Params[0], err = json.Marshal(filter)
				}
				if err == nil {
					raw, err = json.Marshal(call)
				}
				if err != nil {
					t.Error(err)
					return
				}
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		r.ContentLength = int64(len(raw))
		forward.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func provisionStartupBroker(t *testing.T, ctx context.Context, root, endpoint, credentials, stream string) brokerprovision.Document {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/documents/values/broker.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	all, err := brokerprovision.Read(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	document := brokerprovision.Document{Streams: []brokerprovision.Stream{}, Durables: []natsdriver.ConsumerConfig{}}
	for _, item := range all.Streams {
		if item.Name == stream {
			document.Streams = append(document.Streams, item)
		}
	}
	for _, item := range all.Durables {
		if item.Stream == stream {
			document.Durables = append(document.Durables, item)
		}
	}
	if len(document.Streams) != 1 || len(document.Durables) != 4 {
		t.Fatalf("indexer stream inventory: %+v", document)
	}
	path := filepath.Join(t.TempDir(), "broker.json")
	raw, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NATS_URL", endpoint)
	for _, action := range []string{"provision", "check"} {
		var output bytes.Buffer
		code := brokerprovision.Run(ctx, []string{action, "--document", path, "--broker-env", "NATS_URL", "--credentials-file", credentials}, &output, &output)
		if code != 0 {
			t.Fatalf("broker %s: %d: %s", action, code, output.String())
		}
	}
	nc, err := nats.Connect(endpoint, nats.UserCredentials(credentials))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	for _, binding := range document.Durables {
		consumer, err := natsdriver.NewConsumer(ctx, nc, binding)
		if err != nil {
			t.Fatalf("consumer %s: %v", binding.Durable, err)
		}
		consumer.Close()
	}
	return document
}

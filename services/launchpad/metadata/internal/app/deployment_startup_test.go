package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/pkg/types"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

// The production entrypoint consumes composer output and coordinator-owned
// schemas as metadata_runtime. Identity and indexer producers supply only
// their private protocol and recorded canonical rows; storage, scanning,
// Redis, PostgreSQL, broker provisioning and the raster process are real.
func TestComposedProductionStartupAndRestart(t *testing.T) {
	ctx := context.Background()
	root := coordinatortest.RepositoryRoot(t)
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "metadata_runtime")
	runtime := database.Runtimes["metadata_runtime"]
	admin, err := pgxpool.New(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	runtimePool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimePool.Close()
	store, err := metadata.NewProductionStore(runtimePool, "metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	creation := seedDeploymentCreation(t, admin, "chain125-canonical")
	chain := newDevChain(t)
	head, err := chain.backend.Client().HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,$1,$2,$3,'healthy','chain125')", "chain125-canonical", head.Number.Int64(), strings.ToLower(head.Hash().Hex())); err != nil {
		t.Fatal(err)
	}
	provisionDeploymentBroker(t, root)

	approved := internaltest.NewMinIO(t, "metadata-startup-approved")
	quarantine := internaltest.NewMinIO(t, "metadata-startup-quarantine")
	redisURL := internaltest.NewRedisURL(t)
	scannerHost, scannerPort := internaltest.NewClamd(t)
	if scannerHost == "" || scannerPort == 0 {
		t.Fatal("a running scanner is required")
	}
	pki := testpki.New(t)
	service := pki.Leaf("metadata", "localhost")
	gateway := pki.Leaf("gateway", "localhost")
	identity := pki.Leaf("identity", "localhost")
	user := pki.Leaf("user", "localhost")
	signer := testpki.Ed25519(t)
	gatewaySigner := testpki.Ed25519(t)
	userSigner := testpki.Ed25519(t)
	verifier := sharedauth.ServiceRequestVerifier{Issuer: "metadata", Audience: "identity", PeerCertSHA256: service.SHA256, Key: signer.Private.Public().(ed25519.PublicKey), Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}}
	var rejectIdentity atomic.Bool
	var probes atomic.Int64
	authority := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 16385))
		if err != nil || r.Method != http.MethodPost || r.URL.Path != "/internal/v2/identity/session:check" || verifier.Authenticate(r, raw) != nil || rejectIdentity.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if string(raw) != "[]" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		probes.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	authority.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots(), Certificates: []tls.Certificate{identity.Pair}}
	authority.StartTLS()
	defer authority.Close()

	executable := filepath.Join(t.TempDir(), "app")
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", executable, "./cmd/metadatad")
	build.Dir = filepath.Join(root, "services/launchpad/metadata")
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=readonly", "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("metadata executable: %v: %s", err, output)
	}
	env := composedDeploymentEnvironment(t, root, runtime)
	ca, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"DATABASE_URL": runtime.DSN, "DB_POOL_ID": "metadatad_pool", "INSTANCE_ID": "metadata_1", "ZONE": "zone-a",
		"CANONICAL_STREAM_ID": "chain125-canonical", "RPC_URL": chain.url, "REDIS_URL": redisURL,
		"CLAMAV_HOST": scannerHost, "CLAMAV_PORT": strconv.Itoa(scannerPort),
		"S3_ENDPOINT": approved.Endpoint, "S3_BUCKET": approved.Bucket, "S3_APPROVED_BUCKET": approved.Bucket,
		"S3_QUARANTINE_BUCKET": quarantine.Bucket, "S3_ACCESS_KEY_ID": approved.AccessKeyID, "S3_SECRET_ACCESS_KEY": approved.SecretAccessKey,
		"PUBLIC_URL": "https://metadata.example", "IDENTITY_AUDIENCE": "metadata", "IDENTITY_ENDPOINT": authority.URL,
		"IDENTITY_PEER_CERT_SHA256": identity.SHA256, "GATEWAY_PEER_CERT_SHA256": gateway.SHA256, "USER_PEER_CERT_SHA256": user.SHA256,
		"GATEWAY_ASSERTION_PUBLIC_KEY_HEX":   hex.EncodeToString(gatewaySigner.Private.Public().(ed25519.PublicKey)),
		"USER_ASSERTION_PUBLIC_KEY_HEX":      hex.EncodeToString(userSigner.Private.Public().(ed25519.PublicKey)),
		"METADATA_ASSERTION_PRIVATE_KEY_HEX": hex.EncodeToString(signer.Private),
		"METADATA_TLS_CERT_PEM":              string(service.CertPEM), "METADATA_TLS_KEY_PEM": string(service.KeyPEM), "METADATA_TLS_CLIENT_CA_PEM": string(ca),
		"METADATA_PUBLICATION_AUDIT_KEY": base64.StdEncoding.EncodeToString(signer.Private.Seed()), "RASTER_EXECUTABLE": executable,
	} {
		env[name] = value
	}
	assertDeploymentRequired(t, root, env)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{gateway.Pair}}}}
	defer client.CloseIdleConnections()
	healthClient := &http.Client{Timeout: 4 * time.Second}
	defer healthClient.CloseIdleConnections()
	health := "http://127.0.0.1:8080"
	private := "https://127.0.0.1:8443"
	read := func(path string, target any) {
		t.Helper()
		response, err := client.Get(private + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("metadata read %s = %d: %s (%v)", path, response.StatusCode, body, err)
		}
		if target != nil && json.Unmarshal(body, target) != nil {
			t.Fatalf("metadata response: %s", body)
		}
		if strings.HasPrefix(path, "/v2/") && iface.ValidateJSON("MetadataGetResponse", body) != nil {
			t.Fatalf("metadata contract response: %s", body)
		}
	}
	pendingAsset := ""
	for cycle := 0; cycle < 2; cycle++ {
		stop := startDeploymentProcess(t, executable, env)
		waitDeploymentReady(t, healthClient, health, true)
		if probes.Load() == 0 {
			t.Fatal("readiness did not authenticate the identity channel")
		}
		var legacy types.PublicMetadata
		read("/metadata/"+creation.Token, &legacy)
		if legacy.Name == nil || *legacy.Name != creation.Name || legacy.Symbol == nil || *legacy.Symbol != creation.Symbol || legacy.TotalSupply != "1000000000" || legacy.Decimals != 6 {
			t.Fatalf("canonical legacy metadata = %+v", legacy)
		}
		read("/v2/tokens/"+creation.Token+"/metadata", nil)
		response, err := healthClient.Get(health + "/metadata/" + creation.Token)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("health listener exposed metadata: %d", response.StatusCode)
		}
		if cycle == 0 {
			rejectIdentity.Store(true)
			waitDeploymentReady(t, healthClient, health, false)
			rejectIdentity.Store(false)
			waitDeploymentReady(t, healthClient, health, true)
			if _, err := runtimePool.Exec(ctx, "INSERT INTO metadata.pools(pool,token,creator) VALUES($1,$2,$3);", creation.Pool, creation.Token, creation.Creator); err != nil {
				t.Fatal(err)
			}
			if _, err := runtimePool.Exec(ctx, `INSERT INTO metadata.v2_documents(pool,revision,document) VALUES($1,7,'{"description":"Published description","tags":[],"socials":[]}')`, creation.Pool); err != nil {
				t.Fatal(err)
			}
			stop()
			var original bytes.Buffer
			if err := png.Encode(&original, image.NewNRGBA(image.Rect(0, 0, 16, 12))); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(original.Bytes())
			pendingAsset = uuid.NewString()
			key := "quarantine/" + pendingAsset
			if err := quarantine.Store.Put(ctx, key, bytes.NewReader(original.Bytes()), int64(original.Len()), "image/png"); err != nil {
				t.Fatal(err)
			}
			asset := metadata.AssetRecord{ID: pendingAsset, Pool: creation.Pool, Hash: hex.EncodeToString(digest[:]), Size: int64(original.Len()), ObjectKey: key}
			if err := store.CreateConnectedAsset(ctx, asset, "image/png", 16, 12); err != nil {
				t.Fatal(err)
			}
			if err := store.TransitionAsset(ctx, pendingAsset, "CREATED", "UPLOADING"); err != nil {
				t.Fatal(err)
			}
			if err := store.TransitionAsset(ctx, pendingAsset, "UPLOADING", "QUARANTINED"); err != nil {
				t.Fatal(err)
			}
		} else {
			if legacy.Description == nil || *legacy.Description != "Published description" {
				t.Fatalf("published metadata after restart = %+v", legacy)
			}
			deadline := time.Now().Add(30 * time.Second)
			state := ""
			var variants map[string]string
			for time.Now().Before(deadline) {
				var raw []byte
				if err := runtimePool.QueryRow(ctx, "SELECT a.state,d.variants FROM metadata.assets a JOIN metadata.asset_details_v2 d ON d.asset_id=a.id WHERE a.id=$1", pendingAsset).Scan(&state, &raw); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, &variants); err != nil {
					t.Fatal(err)
				}
				if state == "APPROVED" && len(variants) == 3 {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if state != "APPROVED" || len(variants) != 3 {
				t.Fatalf("pending media recovery: state=%s variants=%v", state, variants)
			}
			if _, err := runtimePool.Exec(ctx, "UPDATE metadata.v2_documents SET asset_id=$1 WHERE pool=$2", pendingAsset, creation.Pool); err != nil {
				t.Fatal(err)
			}
			response, err := client.Get(private + "/logo/" + creation.Token + ".png")
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			digest := sha256.Sum256(body)
			if err != nil || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "image/png" || !strings.HasSuffix(variants["512"], "/"+hex.EncodeToString(digest[:])+".png") {
				t.Fatalf("recovered logo: status=%d bytes=%d error=%v", response.StatusCode, len(body), err)
			}
			waitDeploymentReady(t, healthClient, health, true)
			stop()
		}
	}
}

func readDeploymentJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, value); err != nil {
		t.Fatal(err)
	}
}

func writeDeploymentJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func composedDeploymentEnvironment(t *testing.T, root string, runtime coordinatortest.Runtime) map[string]string {
	t.Helper()
	documents := filepath.Join(root, "deploy/launchpad/documents")
	values := t.TempDir()
	var budget dbproduction.ConnectionBudget
	readDeploymentJSON(t, filepath.Join(documents, "values/common.example.json"), &budget)
	parsed, err := url.Parse(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	for i := range budget.Targets {
		if budget.Targets[i].ID == "primary" {
			budget.Targets[i].Host, budget.Targets[i].Port = parsed.Hostname(), uint16(port)
		}
	}
	found := false
	for i := range budget.Clients {
		client := &budget.Clients[i]
		if client.Role == "metadatad" {
			client.Transport, client.Pooler, client.Alias, client.User, client.MaxConnections = runtimedb.TransportDirect, "", strings.TrimPrefix(parsed.Path, "/"), parsed.User.Username(), 8
			found = true
		}
	}
	if !found {
		t.Fatal("metadata database budget is required")
	}
	if err := budget.Validate(); err != nil {
		t.Fatal(err)
	}
	writeDeploymentJSON(t, filepath.Join(values, "common.json"), budget)
	raw, err := os.ReadFile(filepath.Join(documents, "values/metadata.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "metadata.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "expected-schema/metadata.json"), runtime.ExpectedSchema, 0600); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	compose := exec.Command("python3", filepath.Join(documents, "compose.py"), "--values", values, "--out", output, "--service", "metadata")
	if result, err := compose.CombinedOutput(); err != nil {
		t.Fatalf("metadata documents: %v: %s", err, result)
	}
	var env map[string]string
	readDeploymentJSON(t, filepath.Join(output, "metadata.json"), &env)
	var contract map[string]struct {
		Fixed map[string]string `json:"fixed"`
	}
	readDeploymentJSON(t, filepath.Join(root, "deploy/launchpad/railway/variables.json"), &contract)
	for name, value := range contract["metadata"].Fixed {
		env[name] = value
	}
	return env
}

func assertDeploymentRequired(t *testing.T, root string, env map[string]string) {
	t.Helper()
	var contract map[string]struct {
		Required []string `json:"required"`
	}
	readDeploymentJSON(t, filepath.Join(root, "deploy/launchpad/railway/variables.json"), &contract)
	for _, name := range contract["metadata"].Required {
		if env[name] == "" {
			t.Fatalf("metadata setting %s is required", name)
		}
	}
}

func startDeploymentProcess(t *testing.T, executable string, env map[string]string) func() {
	t.Helper()
	log, err := os.CreateTemp(t.TempDir(), "metadata-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	command := exec.Command(executable)
	command.Dir = filepath.Dir(executable)
	configNames := map[string]bool{}
	typ := reflect.TypeOf(config.Config{})
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("env"), ",")
		configNames[name] = true
	}
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if !configNames[name] {
			command.Env = append(command.Env, value)
		}
	}
	for name, value := range env {
		command.Env = append(command.Env, name+"="+value)
	}
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	stopped := false
	stop := func() {
		t.Helper()
		if stopped {
			return
		}
		stopped = true
		_ = command.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-done:
			if err != nil {
				raw, _ := os.ReadFile(log.Name())
				t.Errorf("metadata process: %v: %s", err, raw)
			}
		case <-time.After(20 * time.Second):
			_ = command.Process.Kill()
			<-done
			t.Error("metadata did not drain on SIGTERM")
		}
	}
	t.Cleanup(stop)
	return stop
}

func waitDeploymentReady(t *testing.T, client *http.Client, base string, healthy bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		response, err := client.Get(base + "/health/ready")
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			last = string(body)
			var result struct {
				Status string            `json:"status"`
				Checks map[string]string `json:"checks"`
			}
			if json.Unmarshal(body, &result) == nil {
				if healthy && response.StatusCode == http.StatusOK && result.Status == "ok" && result.Checks["media_identity"] == "ok" && result.Checks["database"] == "ok" && result.Checks["metadata_publication"] == "ok" {
					return
				}
				if !healthy && response.StatusCode == http.StatusServiceUnavailable && result.Checks["media_identity"] == "failed" {
					return
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("metadata readiness healthy=%v: %s", healthy, last)
}

func provisionDeploymentBroker(t *testing.T, root string) {
	t.Helper()
	address := os.Getenv("CORE_API_TEST_NATS_URL")
	if address == "" {
		t.Fatal("CORE_API_TEST_NATS_URL is required")
	}
	connection, err := nats.Connect(address)
	if err != nil {
		t.Fatal("broker connection unavailable")
	}
	js, err := connection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	prefix := "metadata_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	var document map[string]any
	readDeploymentJSON(t, filepath.Join(root, "deploy/launchpad/documents/values/broker.example.json"), &document)
	streams := []string{}
	for _, value := range document["streams"].([]any) {
		stream := value.(map[string]any)
		stream["name"] = prefix + "_" + stream["name"].(string)
		streams = append(streams, stream["name"].(string))
		for i, subject := range stream["subjects"].([]any) {
			stream["subjects"].([]any)[i] = prefix + "." + subject.(string)
		}
	}
	for _, value := range document["durables"].([]any) {
		durable := value.(map[string]any)
		durable["Stream"] = prefix + "_" + durable["Stream"].(string)
		durable["Durable"] = prefix + "_" + durable["Durable"].(string)
		durable["Subject"] = prefix + "." + durable["Subject"].(string)
	}
	t.Cleanup(func() {
		for _, name := range streams {
			if err := js.DeleteStream(name); err != nil {
				t.Errorf("remove broker stream %s: %v", name, err)
			}
		}
		connection.Close()
	})
	path := filepath.Join(t.TempDir(), "broker.json")
	writeDeploymentJSON(t, path, document)
	for _, action := range []string{"provision", "check"} {
		command := exec.Command("go", "run", "./brokerprovision/cmd/brokerprovision", action, "--document", path, "--broker-env", "METADATA_STARTUP_BROKER", "--token-env", "METADATA_STARTUP_BROKER_TOKEN")
		command.Dir = filepath.Join(root, "sdk/launchpad/shared")
		command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=readonly", "METADATA_STARTUP_BROKER="+address, "METADATA_STARTUP_BROKER_TOKEN="+uuid.NewString())
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("broker %s: %v: %s", action, err, output)
		}
	}
}

const deploymentCreationEventsABI = `[{"type":"event","name":"MarketCreated","anonymous":false,"inputs":[{"name":"token","type":"address","indexed":true},{"name":"pool","type":"address","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"nftId","type":"uint256","indexed":false},{"name":"optical","type":"address","indexed":false}]},{"type":"event","name":"TokenDeployed","anonymous":false,"inputs":[{"name":"token","type":"address","indexed":true},{"name":"pool","type":"address","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"salt","type":"bytes32","indexed":false},{"name":"name","type":"string","indexed":false},{"name":"symbol","type":"string","indexed":false},{"name":"decimals","type":"uint8","indexed":false},{"name":"totalSupply","type":"uint256","indexed":false},{"name":"timestamp","type":"uint256","indexed":false},{"name":"blockNumber","type":"uint256","indexed":false}]}]`

type fixtureLog struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	LogIndex         string   `json:"logIndex"`
	TransactionIndex string   `json:"transactionIndex"`
	TransactionHash  string   `json:"transactionHash"`
	BlockHash        string   `json:"blockHash"`
	BlockNumber      string   `json:"blockNumber"`
}

type creationFixture struct {
	Token, Pool, Creator, Factory, Optical, NFTID, Emitter string
	Name, Symbol, Supply                                   string
	Decimals                                               uint8
	Height, TxIndex, Timestamp                             uint64
	BlockHash, TxHash                                      string
	Market, Deployed                                       fixtureLog
}

func hexUint(t *testing.T, s string) uint64 {
	t.Helper()
	v, e := strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 64)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

// loadCreationFixture decodes the recorded receipt and block of the pool
// creation transaction 0xe200…cbfca (block 33963716), captured from the
// Paxeer mainnet RPC: MarketCreated (factory) and TokenDeployed (emitter).
func loadCreationFixture(t *testing.T) creationFixture {
	t.Helper()
	raw, e := os.ReadFile("../metadata/testdata/paxeer125-creation-0x0f92fdd0.json")
	if e != nil {
		t.Fatal(e)
	}
	var recorded struct {
		Receipt struct {
			Logs             []fixtureLog `json:"logs"`
			Status           string       `json:"status"`
			TransactionHash  string       `json:"transactionHash"`
			TransactionIndex string       `json:"transactionIndex"`
			BlockHash        string       `json:"blockHash"`
			BlockNumber      string       `json:"blockNumber"`
		} `json:"receipt"`
		Block struct {
			Hash      string `json:"hash"`
			Number    string `json:"number"`
			Timestamp string `json:"timestamp"`
		} `json:"block"`
	}
	if e = json.Unmarshal(raw, &recorded); e != nil {
		t.Fatal(e)
	}
	parsed, e := abi.JSON(strings.NewReader(deploymentCreationEventsABI))
	if e != nil {
		t.Fatal(e)
	}
	f := creationFixture{TxHash: recorded.Receipt.TransactionHash, BlockHash: recorded.Block.Hash, Height: hexUint(t, recorded.Block.Number), TxIndex: hexUint(t, recorded.Receipt.TransactionIndex), Timestamp: hexUint(t, recorded.Block.Timestamp)}
	if recorded.Receipt.Status != "0x1" || recorded.Receipt.BlockHash != f.BlockHash {
		t.Fatal("fixture receipt is not the successful creation in its block")
	}
	for _, l := range recorded.Receipt.Logs {
		switch l.Topics[0] {
		case parsed.Events["MarketCreated"].ID.Hex():
			f.Market = l
		case parsed.Events["TokenDeployed"].ID.Hex():
			f.Deployed = l
		}
	}
	if f.Market.Address == "" || f.Deployed.Address == "" {
		t.Fatal("fixture lacks the creation events")
	}
	topic := func(s string) string { return strings.ToLower(common.HexToAddress(s).Hex()) }
	f.Token, f.Pool, f.Creator, f.Factory = topic(f.Market.Topics[1]), topic(f.Market.Topics[2]), topic(f.Market.Topics[3]), strings.ToLower(f.Market.Address)
	f.Emitter = strings.ToLower(f.Deployed.Address)
	market := map[string]any{}
	if e = parsed.Events["MarketCreated"].Inputs.NonIndexed().UnpackIntoMap(market, common.FromHex(f.Market.Data)); e != nil {
		t.Fatal(e)
	}
	f.NFTID = market["nftId"].(*big.Int).String()
	f.Optical = strings.ToLower(market["optical"].(common.Address).Hex())
	deployed := map[string]any{}
	if e = parsed.Events["TokenDeployed"].Inputs.NonIndexed().UnpackIntoMap(deployed, common.FromHex(f.Deployed.Data)); e != nil {
		t.Fatal(e)
	}
	f.Name, f.Symbol, f.Decimals, f.Supply = deployed["name"].(string), deployed["symbol"].(string), deployed["decimals"].(uint8), deployed["totalSupply"].(*big.Int).String()
	if f.Token != "0x0f92fdd087c96dd3413ef98646b94d0d986192fa" || f.Pool != "0xce20da50f92d21d8bee497910c782e9aa25b6263" || f.Creator != "0x1dae78b53c9f39930a2ff2b6b7aac12ec75fb154" || f.Decimals != 6 {
		t.Fatalf("fixture identity drifted: %+v", f)
	}
	return f
}

func (f creationFixture) position(l fixtureLog) map[string]string {
	return map[string]string{"chainId": "125", "height": strconv.FormatUint(f.Height, 10), "blockHash": f.BlockHash, "transactionHash": f.TxHash, "transactionIndex": strconv.FormatUint(f.TxIndex, 10), "logIndex": strconv.FormatUint(uint64(mustHex(l.LogIndex)), 10)}
}

func mustHex(s string) int64 {
	v, e := strconv.ParseInt(strings.TrimPrefix(s, "0x"), 16, 64)
	if e != nil {
		panic(e)
	}
	return v
}

func seedDeploymentCreation(t *testing.T, pool *pgxpool.Pool, stream string) creationFixture {
	t.Helper()
	f := loadCreationFixture(t)
	ctx := context.Background()
	q := func(name string) string { return pgx.Identifier{"indexer", name}.Sanitize() }
	for _, l := range []fixtureLog{f.Market, f.Deployed} {
		topics, _ := json.Marshal(l.Topics)
		if _, e := pool.Exec(ctx, "INSERT INTO "+q("indexer_raw_logs")+" VALUES(125,$1,$2,$3,$4,$5,$6,$7::jsonb,$8)", f.TxHash, mustHex(l.LogIndex), f.Height, f.TxIndex, f.BlockHash, strings.ToLower(l.Address), string(topics), common.FromHex(l.Data)); e != nil {
			t.Fatal(e)
		}
	}
	receipt := sha256.Sum256([]byte(f.TxHash))
	header := sha256.Sum256([]byte(f.BlockHash))
	provenance := map[string]string{"deploymentId": "paxeer125-a35ff7c6f532d2f9", "abiRevision": "1", "emitterCodeEpoch": "1", "poolCodeEpoch": "1", "receiptSHA256": hex.EncodeToString(receipt[:]), "headerSHA256": hex.EncodeToString(header[:])}
	payload := map[string]any{"token": f.Token, "pool": f.Pool, "creator": f.Creator, "factory": f.Factory, "optical": f.Optical, "nftId": f.NFTID, "position": f.position(f.Market), "blockTimestampSeconds": strconv.FormatUint(f.Timestamp, 10), "provenance": provenance,
		"metadata": map[string]any{"emitter": f.Emitter, "name": f.Name, "symbol": f.Symbol, "decimals": f.Decimals, "totalSupply": f.Supply, "position": f.position(f.Deployed), "provenance": provenance}}
	rawPayload, _ := json.Marshal(payload)
	rawProvenance, _ := json.Marshal(provenance)
	if _, e := pool.Exec(ctx, "INSERT INTO "+q("indexer_canonical_creations")+" VALUES($1,$2,$3,$4,$5,$6::numeric,$7,$8,$9,$10,$11,$12,$13::jsonb,$14::jsonb)", f.Token, f.Pool, f.Creator, f.Factory, f.Optical, f.NFTID, f.Height, f.BlockHash, f.TxHash, f.TxIndex, mustHex(f.Market.LogIndex), f.Timestamp, string(rawProvenance), string(rawPayload)); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, "INSERT INTO "+q("indexer_block_completeness")+"(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,15,true,true)", stream, f.Height, f.BlockHash); e != nil {
		t.Fatal(e)
	}
	return f
}

package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
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

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/brokerprovision"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nkeys"
)

func TestComposedStatsContainerStartupAndRecovery(t *testing.T) {
	if os.Getenv("STATS_COMPOSED_STARTUP_CHILD") == "1" {
		if err := Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	root := coordinatortest.RepositoryRoot(t)
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "stats_runtime")
	runtime := database.Runtimes["stats_runtime"]
	admin, err := pgx.Connect(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	values, output := t.TempDir(), t.TempDir()
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
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
	examples := filepath.Join(root, "deploy/launchpad/documents/values")
	common := read(filepath.Join(examples, "common.example.json"))
	write(filepath.Join(values, "common.example.json"), common)
	var document statsRuntimeDocument
	if err := json.Unmarshal(read(filepath.Join(examples, "stats.example.json")), &document); err != nil {
		t.Fatal(err)
	}
	document.Region = coordinatortest.Region
	document.Epoch = coordinatortest.Epoch
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(values, "stats.example.json"), raw)
	write(filepath.Join(values, "expected-schema/stats.json"), runtime.ExpectedSchema)
	compose := func() map[string]string {
		t.Helper()
		command := exec.CommandContext(ctx, "python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--examples", values, "--out", output, "--service", "stats")
		if raw, err := command.CombinedOutput(); err != nil {
			t.Fatalf("compose statistics documents: %v %s", err, raw)
		}
		var environment map[string]string
		if err := json.Unmarshal(read(filepath.Join(output, "stats.json")), &environment); err != nil {
			t.Fatal(err)
		}
		return environment
	}
	composed := compose()
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	if err := json.Unmarshal(read(filepath.Join(root, "deploy/launchpad/railway/variables.json")), &contract); err != nil {
		t.Fatal(err)
	}
	// Each process receives only the declared runtime values and their defaults.
	var clearEnvironment func(reflect.Type)
	clearEnvironment = func(typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Anonymous && field.Type.Kind() == reflect.Struct {
				clearEnvironment(field.Type)
			}
			if name := strings.Split(field.Tag.Get("env"), ",")[0]; name != "" {
				t.Setenv(name, "")
			}
		}
	}
	clearEnvironment(reflect.TypeOf(config.Config{}))
	for name, value := range contract["stats"].Fixed {
		t.Setenv(name, value)
	}
	for name, value := range composed {
		t.Setenv(name, value)
	}
	credentials, cert, key, signing, peerPin := composedStatsCredentials(t)
	credentialsPath := filepath.Join(values, "broker.creds")
	certPath := filepath.Join(values, "identity.pem")
	write(credentialsPath, []byte(credentials))
	write(certPath, []byte(cert))
	brokerPort, rpcPort := statsFreePort(t), statsFreePort(t)
	natsBinary := os.Getenv("NATS_SERVER_BINARY")
	if natsBinary == "" {
		natsBinary = "nats-server"
	}
	startStatsDependency(t, natsBinary, brokerPort, "-js", "-sd", t.TempDir(), "-a", "127.0.0.1", "-p", brokerPort)
	startStatsDependency(t, "anvil", rpcPort, "--host", "127.0.0.1", "--port", rpcPort, "--chain-id", "125", "--silent")
	redisURL := internaltest.NewRedisURL(t)
	for name, value := range map[string]string{
		"DATABASE_URL": runtime.DSN, "DB_POOL_ID": "statsd_pool", "INSTANCE_ID": "stats-composed", "ZONE": "zone-a",
		"NATS_URL": "nats://127.0.0.1:" + brokerPort, "NATS_CREDENTIALS": credentialsPath,
		"REDIS_URL": redisURL, "REDIS_BULL_URL": redisURL, "RPC_URL": "http://127.0.0.1:" + rpcPort,
		"STATS_CURSOR_KEY": strings.Repeat("a", 64), "STATS_AUDIT_KEY": strings.Repeat("b", 64), "WEBHOOK_HMAC_SECRET": strings.Repeat("h", 32),
		"IDENTITY_SERVICE_URL": "https://127.0.0.1:8443", "IDENTITY_MTLS_CA_FILE": certPath, "IDENTITY_MTLS_CLIENT_CERT_FILE": certPath,
		"IDENTITY_MTLS_CLIENT_KEY_PEM": key, "IDENTITY_ASSERTION_SIGNING_KEY": signing, "IDENTITY_MTLS_PEER_CERT_SHA256": peerPin,
	} {
		t.Setenv(name, value)
	}
	for _, name := range contract["stats"].Required {
		if os.Getenv(name) == "" {
			t.Fatalf("required statistics setting missing: %s", name)
		}
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if pool, err := cfg.OpenRuntimePool(ctx, "statsd", shareddb.PoolOptions{}); err == nil {
		pool.Close()
		t.Fatal("transaction transport requires an admitted runtime transaction boundary")
	} else if err.Error() != "runtime_transaction_pooler_seam_unavailable" {
		t.Fatalf("example transport: %v", err)
	}
	// The direct route uses the provisioned role and the example's single connection.
	var budget production.ConnectionBudget
	if err := json.Unmarshal(common, &budget); err != nil {
		t.Fatal(err)
	}
	route, err := pgx.ParseConfig(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	budget.Targets[0].Host, budget.Targets[0].Port = route.Host, route.Port
	for i := range budget.Clients {
		c := &budget.Clients[i]
		if c.Role == "statsd" {
			c.Transport, c.Pooler, c.Alias, c.User, c.QueryClass = shareddb.TransportDirect, "", route.Database, route.User, shareddb.Batch
		}
	}
	raw, err = json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(values, "common.example.json"), raw)
	for name, value := range compose() {
		t.Setenv(name, value)
	}
	var provision brokerprovision.Document
	if err := json.Unmarshal(read(filepath.Join(examples, "broker.example.json")), &provision); err != nil {
		t.Fatal(err)
	}
	selected := brokerprovision.Document{}
	for _, stream := range provision.Streams {
		if stream.Name == document.Consumers[0].Stream || stream.Name == document.Phase2.OutputStream {
			selected.Streams = append(selected.Streams, stream)
		}
	}
	for _, durable := range provision.Durables {
		if durable.Durable == document.Consumers[0].Durable {
			selected.Durables = append(selected.Durables, durable)
		}
		for _, sub := range document.Phase2.SearchConsumers {
			if durable.Durable == sub.Consumer.Durable {
				selected.Durables = append(selected.Durables, durable)
			}
		}
	}
	if len(selected.Streams) != 2 || len(selected.Durables) != 3 {
		t.Fatal("statistics broker resources must be present in the fleet document")
	}
	raw, err = json.Marshal(selected)
	if err != nil {
		t.Fatal(err)
	}
	brokerPath := filepath.Join(values, "broker.json")
	write(brokerPath, raw)
	var report bytes.Buffer
	for _, action := range []string{"provision", "check"} {
		if code := brokerprovision.Run(ctx, []string{action, "--document", brokerPath, "--broker-env", "NATS_URL", "--credentials-file", credentialsPath}, &report, &report); code != 0 {
			t.Fatalf("statistics broker resources: %d %s", code, report.String())
		}
	}
	chain, err := ethclient.DialContext(ctx, os.Getenv("RPC_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()
	hashes := map[int]string{}
	for _, height := range []int{1, 2} {
		if err := chain.Client().CallContext(ctx, new(any), "evm_mine"); err != nil {
			t.Fatal(err)
		}
		header, err := chain.HeaderByNumber(ctx, big.NewInt(int64(height)))
		if err != nil {
			t.Fatal(err)
		}
		hashes[height] = header.Hash().Hex()
	}
	blockHash := hashes[1]
	if _, err := admin.Exec(ctx, `INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'indexer',1,clock_timestamp()+interval '1 hour')`, document.SourceStream); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,1,1)`, document.SourceStream); err != nil {
		t.Fatal(err)
	}
	complete := func(height int) {
		t.Helper()
		if _, err := admin.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,0,true,true)`, document.SourceStream, height, hashes[height]); err != nil {
			t.Fatal(err)
		}
	}
	complete(1)
	t.Setenv("STATS_COMPOSED_STARTUP_CHILD", "1")
	for _, height := range []int{1, 2} {
		blockHash = hashes[height]
		if height == 2 {
			complete(2)
			if _, err := admin.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=2,typed_complete_height=2 WHERE chain_id=125 AND stream_id=$1`, document.SourceStream); err != nil {
				t.Fatal(err)
			}
		}
		child := exec.Command(os.Args[0], "-test.run=^TestComposedStatsContainerStartupAndRecovery$")
		child.Env = os.Environ()
		logPath := filepath.Join(output, fmt.Sprintf("stats-%d.txt", height))
		log, err := os.Create(logPath)
		if err != nil {
			t.Fatal(err)
		}
		child.Stdout, child.Stderr = log, log
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- child.Wait() }()
		stopped := false
		t.Cleanup(func() {
			if !stopped {
				_ = child.Process.Kill()
				<-done
			}
			_ = log.Close()
		})
		client := &http.Client{Timeout: 2 * time.Second}
		ready := false
		var last []byte
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
			response, err := client.Get("http://127.0.0.1:8080/health/ready")
			if err == nil {
				last, _ = io.ReadAll(response.Body)
				_ = response.Body.Close()
				if response.StatusCode == 200 {
					var status struct {
						Checks map[string]string `json:"checks"`
					}
					if err := json.Unmarshal(last, &status); err != nil {
						t.Fatal(err)
					}
					for _, name := range []string{"database", "redis", "broker", "source", "projections", "search", "secrets"} {
						if status.Checks[name] != "ok" {
							t.Fatalf("readiness %s: %s", name, last)
						}
					}
					ready = true
					break
				}
			}
			select {
			case err := <-done:
				stopped = true
				t.Fatalf("statistics process stopped: %v %s", err, read(logPath))
			case <-time.After(50 * time.Millisecond):
			}
		}
		if !ready {
			t.Fatalf("statistics readiness: %s\n%s", last, read(logPath))
		}
		response, err := client.Get("http://127.0.0.1:8080/v2/stats/platform")
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		var snapshot projectionstats.WindowSnapshot
		if err := json.Unmarshal(body, &snapshot); err != nil || response.StatusCode != 200 {
			t.Fatalf("platform response: %d %s %v", response.StatusCode, body, err)
		}
		if snapshot.VolumeUSDLRaw != "0" || snapshot.PoolFillCount != "0" || snapshot.UniqueActors != "0" || snapshot.Stale || snapshot.Fees == nil || snapshot.Watermark.Height != strconv.Itoa(height) || snapshot.Watermark.BlockHash != blockHash || snapshot.Watermark.SourceStream != document.SourceStream || snapshot.Watermark.ProjectionVersion != document.ProjectionVersion || snapshot.UserActionCount != nil || snapshot.UserActionUnavailableReason != "qualified_route_identity_unavailable" {
			t.Fatalf("computed platform statistics: %s", body)
		}
		if err := child.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			stopped = true
			if err != nil {
				t.Fatalf("statistics shutdown: %v %s", err, read(logPath))
			}
		case <-time.After(10 * time.Second):
			t.Fatal("statistics shutdown exceeded drain deadline")
		}
	}
}

func statsFreePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func startStatsDependency(t *testing.T, binary, port string, args ...string) {
	t.Helper()
	command := exec.Command(binary, args...)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("statistics dependency did not listen")
}

func composedStatsCredentials(t *testing.T) (credentials, certificate, privateKey, signingKey, peerPin string) {
	t.Helper()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "statistics-client"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, public, key)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	privateKey = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey}))
	signingKey = base64.StdEncoding.EncodeToString(key)
	digest := sha256.Sum256(der)
	peerPin = hex.EncodeToString(digest[:])
	account, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	defer account.Wipe()
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	defer user.Wipe()
	issuer, err := account.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	subject, err := user.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	unsigned := encode([]byte(`{"typ":"JWT","alg":"ed25519-nkey"}`)) + "." + encode([]byte(`{"sub":"`+subject+`","iss":"`+issuer+`","nats":{"type":"user","version":2}}`))
	signature, err := account.Sign([]byte(unsigned))
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	credentials = "-----BEGIN NATS USER JWT-----\n" + unsigned + "." + encode(signature) + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	return
}

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

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	madmin "github.com/minio/madmin-go/v3"
)

func exerciseComposedGatewayEntrypoint(t *testing.T, values, output string, pki *testpki.PKI, source map[string]any) {
	t.Helper()
	ctx := context.Background()
	root := coordinatortest.RepositoryRoot(t)
	documents := filepath.Join(root, "deploy/launchpad/documents")
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	write := func(path string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	encode := func(value any) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "identity_runtime", "control_runtime")
	admin, err := pgx.Connect(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	for _, role := range []string{"identity_runtime", "control_runtime"} {
		runtime := database.Runtimes[role]
		if _, err := admin.Exec(ctx, "ALTER ROLE "+pgx.Identifier{runtime.Role}.Sanitize()+" RENAME TO "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := admin.Exec(context.Background(), "ALTER ROLE "+pgx.Identifier{role}.Sanitize()+" RENAME TO "+pgx.Identifier{runtime.Role}.Sanitize()); err != nil {
				t.Error(err)
			}
		})
		endpoint, err := url.Parse(runtime.DSN)
		if err != nil {
			t.Fatal(err)
		}
		password, _ := endpoint.User.Password()
		endpoint.User = url.UserPassword(role, password)
		runtime.DSN = endpoint.String()
		var expected production.ExpectedSchema
		if err := json.Unmarshal(runtime.ExpectedSchema, &expected); err != nil {
			t.Fatal(err)
		}
		expected.RuntimeRole = role
		if err := production.RecordRuntimeReadiness(ctx, admin, expected); err != nil {
			t.Fatal(err)
		}
		runtime.ExpectedSchema = encode(expected)
		runtime.ExpectedSchemaSHA256 = production.Checksum(runtime.ExpectedSchema)
		database.Runtimes[role] = runtime
	}
	write(filepath.Join(values, "expected-schema/gateway.json"), database.Runtimes["identity_runtime"].ExpectedSchema)
	var budget production.ConnectionBudget
	if err := json.Unmarshal(read(filepath.Join(values, "common.json")), &budget); err != nil {
		t.Fatal(err)
	}
	endpoint, err := pgx.ParseConfig(database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	for i := range budget.Targets {
		budget.Targets[i].Host, budget.Targets[i].Port = endpoint.Host, endpoint.Port
	}
	for i := range budget.Clients {
		if budget.Clients[i].Role == "mediagatewayd" {
			budget.Clients[i].Alias = database.Name
		}
	}
	write(filepath.Join(values, "common.json"), encode(budget))
	runtimeValues := source["runtime"].(map[string]any)
	runtimeValues["recoveryEpoch"], runtimeValues["recoveryRegion"] = coordinatortest.Epoch, coordinatortest.Region
	runtimeValues["safetySchema"] = "chat"
	runtimeValues["audit"].(map[string]any)["sinkDeliverySchema"] = "gateway_audit_sink"
	ports := internaltest.FreePorts(t, 2)
	privateAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(ports[0]))
	runtimeValues["identityListenAddress"] = privateAddress
	runtimeValues["identityEndpoint"] = "https://" + privateAddress
	brokerURL := startGatewayBrokerFixture(t, pki, pki.Leaf("entrypoint-broker", "localhost"))
	runtimeValues["natsUrl"] = brokerURL
	t.Setenv("GATEWAY_NATS_CREDENTIALS", gatewayBrokerCredentials(t))
	t.Setenv("GATEWAY_CONTROL_DATABASE_URL", database.Runtimes["control_runtime"].DSN)
	identityValues := source["identity"].(map[string]any)
	keyValues := identityValues["keys"].([]any)[0].(map[string]any)
	keyValues["activeAt"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	keyValues["retireAt"] = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	objectEndpoint, servingKey, servingSecret := gatewayEntrypointObjects(t, pki, runtimeValues)
	write(filepath.Join(values, "gateway.json"), encode(source))
	compose := exec.Command("python3", filepath.Join(documents, "compose.py"), "--values", values, "--out", output, "--service", "gateway")
	if raw, err := compose.CombinedOutput(); err != nil {
		t.Fatalf("compose gateway startup: %v\n%s", err, raw)
	}
	var environment map[string]string
	if err := json.Unmarshal(read(filepath.Join(output, "gateway.json")), &environment); err != nil {
		t.Fatal(err)
	}
	var brokerDocument struct {
		Streams  []json.RawMessage `json:"streams"`
		Durables []json.RawMessage `json:"durables"`
	}
	if err := json.Unmarshal(read(filepath.Join(documents, "values/broker.example.json")), &brokerDocument); err != nil {
		t.Fatal(err)
	}
	streams, durables := []json.RawMessage{}, []json.RawMessage{}
	for _, raw := range brokerDocument.Streams {
		var stream struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &stream); err != nil {
			t.Fatal(err)
		}
		if stream.Name == runtimeValues["stream"] || stream.Name == runtimeValues["audit"].(map[string]any)["stream"] {
			streams = append(streams, raw)
		}
	}
	for _, raw := range brokerDocument.Durables {
		var durable struct{ Stream string }
		if err := json.Unmarshal(raw, &durable); err != nil {
			t.Fatal(err)
		}
		if durable.Stream == runtimeValues["stream"] || durable.Stream == runtimeValues["audit"].(map[string]any)["stream"] {
			durables = append(durables, raw)
		}
	}
	if len(streams) != 2 || len(durables) != 2 {
		t.Fatal("gateway streams and durables must be present in the broker document")
	}
	brokerDocument.Streams, brokerDocument.Durables = streams, durables
	brokerPath := filepath.Join(output, "broker.json")
	write(brokerPath, encode(brokerDocument))
	for _, action := range []string{"provision", "check"} {
		command := exec.Command("go", "run", "./brokerprovision/cmd/brokerprovision", action, "--document", brokerPath, "--broker-env", "GATEWAY_ENTRYPOINT_BROKER", "--credentials-env", "GATEWAY_NATS_CREDENTIALS")
		command.Dir = filepath.Join(root, "sdk/launchpad/shared")
		command.Env = append(os.Environ(), "GATEWAY_ENTRYPOINT_BROKER="+brokerURL, "SSL_CERT_FILE="+pki.CAFile, "GOWORK=off")
		if raw, err := command.CombinedOutput(); err != nil {
			t.Fatalf("broker %s: %v\n%s", action, err, raw)
		}
	}
	_, redisURL := internaltest.NewRedis(t)
	rpcURL := "http://127.0.0.1:" + strconv.Itoa(ports[1])
	chain := exec.Command("anvil", "--host", "127.0.0.1", "--port", strconv.Itoa(ports[1]), "--chain-id", "125", "--silent")
	stopChain, _ := gatewayEntrypointProcess(t, chain)
	defer stopChain()
	gatewayEntrypointAwait(t, func() bool {
		client, err := ethclient.DialContext(ctx, rpcURL)
		if err != nil {
			return false
		}
		defer client.Close()
		chainID, err := client.ChainID(ctx)
		return err == nil && chainID.Int64() == 125
	})
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Defaults map[string]string `json:"defaults"`
	}
	if err := json.Unmarshal(read(filepath.Join(root, "deploy/launchpad/railway/variables.json")), &contract); err != nil {
		t.Fatal(err)
	}
	for name, value := range contract["gateway"].Defaults {
		environment[name] = value
	}
	for name, value := range contract["gateway"].Fixed {
		environment[name] = value
	}
	for name, value := range map[string]string{
		"DATABASE_URL": database.Runtimes["identity_runtime"].DSN,
		"DB_POOL_ID":   "mediagatewayd_pool", "INSTANCE_ID": "gateway_entrypoint", "ZONE": "zone-a",
		"METRICS_ENVIRONMENT": "qualification", "REDIS_URL": redisURL, "RPC_URL": rpcURL,
		"GATEWAY_JWT_SECRET": uuid.NewString(), "IDENTITY_SERVICE_TOKEN": uuid.NewString(),
		"IDENTITY_SIGNING_SECRET": uuid.NewString(), "IDENTITY_LIMITER_KEY": uuid.NewString(),
		"CORS_ALLOWED_ORIGINS": "https://app.kindlelaunch.example", "PUBLIC_URL": "https://app.kindlelaunch.example",
		"SOCIAL_HTTP_URL": "https://social.kindlelaunch.example", "SOCIAL_WS_URL": "wss://social.kindlelaunch.example",
		"METADATA_UPLOAD_URL": "https://metadata.kindlelaunch.example", "S3_ENDPOINT": objectEndpoint,
		"S3_REGION": "us-east-1", "S3_ACCESS_KEY_ID": servingKey, "S3_SECRET_ACCESS_KEY": servingSecret,
		"METADATA_BUCKET": runtimeValues["objectWorkers"].([]any)[0].(map[string]any)["bucket"].(string),
	} {
		environment[name] = value
	}
	configurationNames := map[string]bool{}
	configuration := reflect.TypeOf(config.Config{})
	for i := 0; i < configuration.NumField(); i++ {
		configurationNames[strings.Split(configuration.Field(i).Tag.Get("env"), ",")[0]] = true
	}
	for name := range configurationNames {
		t.Setenv(name, environment[name])
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	var binding gatewayRuntimeBinding
	if err := config.ReadBoundDocument(cfg.GatewayRuntimeFile, cfg.GatewayRuntimeSHA256, &binding); err != nil {
		t.Fatal(err)
	}
	t.Run("pool recovery", func(t *testing.T) {
		t.Setenv("GATEWAY_CONTROL_DATABASE_URL", cfg.DatabaseURL)
		identity, control, err := cfg.OpenSplitRuntimePools(ctx, binding.Pools)
		if err == nil || identity != nil || control != nil {
			t.Fatal("identity credential was admitted for the control pool")
		}
		t.Setenv("GATEWAY_CONTROL_DATABASE_URL", database.Runtimes["control_runtime"].DSN)
		invalid := cfg
		invalid.ExpectedSchemaSHA256 = strings.Repeat("0", 64)
		if identity, control, err := invalid.OpenSplitRuntimePools(ctx, binding.Pools); err == nil || identity != nil || control != nil {
			t.Fatal("unbound expected schema was admitted")
		}
		identity, control, err = cfg.OpenSplitRuntimePools(ctx, binding.Pools)
		if err != nil {
			t.Fatalf("open pools after rejected bindings: %v", err)
		}
		defer runtimedb.ClosePool(identity)
		defer runtimedb.ClosePool(control)
		var schema, role string
		if err := identity.QueryRow(ctx, "SELECT current_schema(),current_user").Scan(&schema, &role); err != nil || schema != "users" || role != "identity_runtime" {
			t.Fatalf("identity schema and role: %s %s %v", schema, role, err)
		}
	})
	invalidIdentity := cfg
	invalidIdentity.IdentityConfigSHA256 = strings.Repeat("0", 64)
	if application, err := New(ctx, invalidIdentity, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil || application != nil {
		if application != nil {
			application.Close()
		}
		t.Fatal("gateway admitted an unbound identity document")
	}
	for i := 0; i < 2; i++ {
		application, err := New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err != nil {
			t.Fatalf("construct gateway: %v", err)
		}
		application.Close()
		if err := application.tracker.Ready(ctx); err == nil {
			t.Fatal("closed gateway remained ready")
		}
	}
	binary := filepath.Join(t.TempDir(), "app")
	build := exec.Command("go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-o", binary, "./cmd/mediagatewayd")
	build.Dir = filepath.Join(root, "services/launchpad/gateway")
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off")
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile gateway entrypoint: %v\n%s", err, raw)
	}
	var inherited []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !configurationNames[name] && name != "SSL_CERT_FILE" {
			inherited = append(inherited, entry)
		}
	}
	for name, value := range environment {
		inherited = append(inherited, name+"="+value)
	}
	inherited = append(inherited, "SSL_CERT_FILE="+pki.CAFile)
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	t.Cleanup(client.Transport.(*http.Transport).CloseIdleConnections)
	privateClient := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{pki.Leaf("entrypoint-probe", "localhost").Pair}}}}
	t.Cleanup(privateClient.Transport.(*http.Transport).CloseIdleConnections)
	publicURL := "http://127.0.0.1:8080"
	start := func() func() {
		t.Helper()
		command := exec.Command(binary)
		command.Env = inherited
		stop, _ := gatewayEntrypointProcess(t, command)
		gatewayEntrypointAwait(t, func() bool {
			response, err := client.Get(publicURL + "/health/ready")
			if err != nil {
				return false
			}
			defer response.Body.Close()
			var answer struct {
				Status string            `json:"status"`
				Checks map[string]string `json:"checks"`
			}
			if err := json.NewDecoder(response.Body).Decode(&answer); err != nil || response.StatusCode != http.StatusOK || answer.Status != "ok" {
				return false
			}
			for _, name := range []string{"database", "redis", "secrets", "connected_runtime", "identity_signing_keys", "lifecycle"} {
				if answer.Checks[name] != "ok" {
					return false
				}
			}
			return true
		})
		response, err := privateClient.Get("https://" + privateAddress + "/health/ready")
		if err != nil {
			t.Fatalf("private readiness: %v", err)
		}
		var answer map[string]string
		err = json.NewDecoder(response.Body).Decode(&answer)
		_ = response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK || answer["status"] != "ok" {
			t.Fatalf("private readiness: %d %v %v", response.StatusCode, answer, err)
		}
		return stop
	}
	request := func(method, path, bearer, key string, body []byte, want int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, publicURL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", cfg.PublicURL)
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != want {
			t.Fatalf("%s %s: status=%d body=%s error=%v", method, path, response.StatusCode, raw, err)
		}
		return raw
	}
	stop := start()
	wallet := internaltest.NewWallet(t)
	challengeBody := encode(auth.ChallengeRequest{Wallet: wallet.Addr, Origin: cfg.PublicURL})
	key := uuid.NewString()
	var challenge auth.Envelope[struct {
		Challenge auth.Challenge `json:"challenge"`
	}]
	if err := json.Unmarshal(request(http.MethodPost, "/v2/auth/challenges", "", key, challengeBody, http.StatusCreated), &challenge); err != nil {
		t.Fatal(err)
	}
	if challenge.Data.Challenge.Wallet != wallet.Addr || challenge.Data.Challenge.Domain != identityValues["domain"] || challenge.Data.Challenge.URI != identityValues["uri"] || challenge.Data.Challenge.ChainID != "125" || challenge.Quality != "complete" {
		t.Fatal("issued challenge differs from the identity deployment document")
	}
	stop()
	stop = start()
	var replay auth.Envelope[struct {
		Challenge auth.Challenge `json:"challenge"`
	}]
	if err := json.Unmarshal(request(http.MethodPost, "/v2/auth/challenges", "", key, challengeBody, http.StatusCreated), &replay); err != nil || !reflect.DeepEqual(challenge.Data.Challenge, replay.Data.Challenge) {
		t.Fatal("challenge idempotency did not survive restart")
	}
	proof := auth.WalletProof{Kind: "eoa_eip191", Wallet: wallet.Addr, Signature: wallet.Sign(t, replay.Data.Challenge.Message)}
	var login auth.Envelope[auth.LoginData]
	if err := json.Unmarshal(request(http.MethodPost, "/v2/auth/sessions", "", "", encode(auth.LoginRequest{ChallengeID: replay.Data.Challenge.ChallengeID, Proof: proof, DeviceLabel: "Browser wallet"}), http.StatusOK), &login); err != nil || login.Data.AccessToken == "" || login.Data.Session.Wallet != wallet.Addr {
		t.Fatal("signed login did not create the wallet session")
	}
	gatewayEntrypointAwait(t, func() bool {
		var revision int64
		var audits, inventories int
		if err := admin.QueryRow(ctx, "SELECT revision FROM users.identity_remote_revisions").Scan(&revision); err != nil {
			return false
		}
		if err := admin.QueryRow(ctx, "SELECT count(*) FROM gateway_audit_sink.security_audit_sink").Scan(&audits); err != nil {
			return false
		}
		if err := admin.QueryRow(ctx, "SELECT count(DISTINCT prefix) FROM gateway.inventory_reports WHERE bucket=$1", cfg.TokenBucket).Scan(&inventories); err != nil {
			return false
		}
		return revision == 1 && audits > 0 && inventories == 3
	})
	stop()
	stop = start()
	defer stop()
	var links auth.Envelope[struct {
		Items []auth.WalletLink `json:"items"`
	}]
	if err := json.Unmarshal(request(http.MethodGet, "/v2/me/wallet-links", login.Data.AccessToken, "", nil, http.StatusOK), &links); err != nil || len(links.Data.Items) != 1 || links.Data.Items[0].Wallet != wallet.Addr || links.Data.Items[0].State != "verified" {
		t.Fatal("verified wallet and session did not survive restart")
	}
	request(http.MethodGet, "/internal/v2/identity/session", login.Data.AccessToken, "", nil, http.StatusNotFound)
	var migrations int
	if err := admin.QueryRow(ctx, "SELECT count(*) FROM pg_class WHERE relname='goose_db_version'").Scan(&migrations); err != nil || migrations != 0 {
		t.Fatal("gateway entrypoint must use the coordinator schema")
	}
}

func gatewayEntrypointObjects(t *testing.T, pki *testpki.PKI, runtimeValues map[string]any) (string, string, string) {
	t.Helper()
	ctx := context.Background()
	certificate := pki.Leaf("entrypoint-objects", "localhost")
	directory := t.TempDir()
	for name, raw := range map[string][]byte{"public.crt": certificate.CertPEM, "private.key": certificate.KeyPEM} {
		if err := os.WriteFile(filepath.Join(directory, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	port := internaltest.FreePorts(t, 1)[0]
	address := "127.0.0.1:" + strconv.Itoa(port)
	endpoint := "https://" + address
	rootKey, rootSecret := uuid.NewString(), uuid.NewString()
	command := exec.Command("minio", "server", "--certs-dir", directory, "--address", address, "--console-address", "127.0.0.1:0", t.TempDir())
	command.Env = append(os.Environ(), "MINIO_ROOT_USER="+rootKey, "MINIO_ROOT_PASSWORD="+rootSecret, "MINIO_BROWSER=off")
	gatewayEntrypointProcess(t, command)
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.Roots()}}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	gatewayEntrypointAwait(t, func() bool {
		response, err := client.Get(endpoint + "/minio/health/ready")
		if err != nil {
			return false
		}
		_ = response.Body.Close()
		return response.StatusCode == http.StatusOK
	})
	bucket := "gateway-" + uuid.NewString()
	s3client := s3.New(s3.Options{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider(rootKey, rootSecret, ""), BaseEndpoint: aws.String(endpoint), UsePathStyle: true, HTTPClient: client})
	if _, err := s3client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	admin, err := madmin.New(address, rootKey, rootSecret, true)
	if err != nil {
		t.Fatal(err)
	}
	admin.SetCustomTransport(transport)
	provision := func(prefix, accessReference, secretReference string, actions []string, list bool) (string, string) {
		t.Helper()
		access, secret := uuid.NewString(), uuid.NewString()
		statements := []map[string]any{{"Effect": "Allow", "Action": actions, "Resource": []string{"arn:aws:s3:::" + bucket + "/" + prefix + "*"}}}
		if list {
			statements = append(statements, map[string]any{"Effect": "Allow", "Action": []string{"s3:ListBucket"}, "Resource": []string{"arn:aws:s3:::" + bucket}, "Condition": map[string]any{"StringLike": map[string][]string{"s3:prefix": {prefix + "*"}}}})
		}
		raw, err := json.Marshal(map[string]any{"Version": "2012-10-17", "Statement": statements})
		if err != nil {
			t.Fatal(err)
		}
		if err := admin.AddUser(ctx, access, secret); err != nil {
			t.Fatal(err)
		}
		if err := admin.AddCannedPolicy(ctx, access, raw); err != nil {
			t.Fatal(err)
		}
		if err := admin.SetPolicy(ctx, access, access, false); err != nil {
			t.Fatal(err)
		}
		t.Setenv(accessReference, access)
		t.Setenv(secretReference, secret)
		return access, secret
	}
	servingKey, servingSecret := provision("approved/", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", []string{"s3:GetObject"}, false)
	credentialReferences := map[string]string{}
	for name, credential := range map[string]string{
		"deleteVariantAccess": "GATEWAY_DELETE_VARIANT_ACCESS", "deleteVariantSecret": "GATEWAY_DELETE_VARIANT_SECRET",
		"deleteOriginalAccess": "GATEWAY_DELETE_ORIGINAL_ACCESS", "deleteOriginalSecret": "GATEWAY_DELETE_ORIGINAL_SECRET",
		"deleteTemporaryAccess": "GATEWAY_DELETE_TEMPORARY_ACCESS", "deleteTemporarySecret": "GATEWAY_DELETE_TEMPORARY_SECRET",
		"inventoryVariantAccess": "GATEWAY_INVENTORY_VARIANT_ACCESS", "inventoryVariantSecret": "GATEWAY_INVENTORY_VARIANT_SECRET",
		"inventoryOriginalAccess": "GATEWAY_INVENTORY_ORIGINAL_ACCESS", "inventoryOriginalSecret": "GATEWAY_INVENTORY_ORIGINAL_SECRET",
		"inventoryTemporaryAccess": "GATEWAY_INVENTORY_TEMPORARY_ACCESS", "inventoryTemporarySecret": "GATEWAY_INVENTORY_TEMPORARY_SECRET",
	} {
		credentialReferences[name] = credential
	}
	for _, worker := range runtimeValues["objectWorkers"].([]any) {
		binding := worker.(map[string]any)
		binding["bucket"] = bucket
		prefix := binding["prefix"].(string)
		provision(prefix, credentialReferences[binding["deleteAccessKeyCredential"].(string)], credentialReferences[binding["deleteSecretKeyCredential"].(string)], []string{"s3:DeleteObject"}, false)
		provision(prefix, credentialReferences[binding["inventoryAccessKeyCredential"].(string)], credentialReferences[binding["inventorySecretKeyCredential"].(string)], []string{"s3:GetObject"}, true)
	}
	return endpoint, servingKey, servingSecret
}

func gatewayEntrypointProcess(t *testing.T, command *exec.Cmd) (func(), string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "process.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	done := make(chan struct{})
	var result error
	go func() { result = command.Wait(); close(done) }()
	stopped := false
	stop := func() {
		t.Helper()
		if stopped {
			return
		}
		stopped = true
		_ = command.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
			if result != nil {
				t.Errorf("process exit: %v", result)
			}
		case <-time.After(15 * time.Second):
			_ = command.Process.Kill()
			<-done
			t.Error("process did not drain")
		}
		_ = log.Close()
		if t.Failed() {
			raw, err := os.ReadFile(logPath)
			if err == nil {
				t.Log(string(raw))
			}
		}
	}
	t.Cleanup(stop)
	return stop, logPath
}

func gatewayEntrypointAwait(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !ready() {
		if !time.Now().Before(deadline) {
			t.Fatal("gateway dependency did not become ready")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestGatewayRuntimeReportsDependencyCause(t *testing.T) {
	var output bytes.Buffer
	runtime := &gatewayConnectedRuntime{logger: slog.New(slog.NewJSONHandler(&output, nil))}
	runtime.fail(fmt.Errorf("identity delivery consumer unavailable"))
	runtime.fail(fmt.Errorf("later dependency"))
	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil || event["err"] != "identity delivery consumer unavailable" || event["level"] != "ERROR" {
		t.Fatalf("runtime diagnostic: %s %v", output.String(), err)
	}
	if err := runtime.ready(context.Background()); err == nil || err.Error() != event["err"] {
		t.Fatal("readiness and runtime diagnostic disagree")
	}
}

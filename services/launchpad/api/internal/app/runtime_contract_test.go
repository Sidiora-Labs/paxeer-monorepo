package app

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

func contractWrite(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func contractRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func contractJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	contractWrite(t, path, raw)
}

// contractProcess owns the actual service executable and its graceful stop.
func contractProcess(t *testing.T, command *exec.Cmd) func() {
	t.Helper()
	log, err := os.Create(filepath.Join(t.TempDir(), "service.log"))
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
				t.Errorf("service exit: %v", result)
			}
		case <-time.After(15 * time.Second):
			_ = command.Process.Kill()
			<-done
			t.Error("service did not drain on SIGTERM")
		}
		_ = log.Close()
		if t.Failed() {
			t.Log(string(contractRead(t, log.Name())))
		}
	}
	t.Cleanup(stop)
	return stop
}

func contractBroker(t *testing.T, pki *testpki.PKI) (*nats.Conn, jetstream.JetStream, string, string) {
	t.Helper()
	key := func(kind nkeys.PrefixByte) (nkeys.KeyPair, string) {
		t.Helper()
		pair, err := nkeys.CreatePair(kind)
		if err != nil {
			t.Fatal(err)
		}
		public, err := pair.PublicKey()
		if err != nil {
			t.Fatal(err)
		}
		return pair, public
	}
	operator, operatorPublic := key(nkeys.PrefixByteOperator)
	account, accountPublic := key(nkeys.PrefixByteAccount)
	_, systemPublic := key(nkeys.PrefixByteAccount)
	user, userPublic := key(nkeys.PrefixByteUser)
	op := jwt.NewOperatorClaims(operatorPublic)
	op.SystemAccount = systemPublic
	operatorJWT, err := op.Encode(operator)
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.NewAccountClaims(accountPublic)
	claims.Limits.JetStreamLimits = jwt.JetStreamLimits{MemoryStorage: -1, DiskStorage: -1, Streams: -1, Consumer: -1}
	accountJWT, err := claims.Encode(operator)
	if err != nil {
		t.Fatal(err)
	}
	systemJWT, err := jwt.NewAccountClaims(systemPublic).Encode(operator)
	if err != nil {
		t.Fatal(err)
	}
	userJWT, err := jwt.NewUserClaims(userPublic).Encode(account)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := jwt.FormatUserConfig(userJWT, seed)
	if err != nil {
		t.Fatal(err)
	}
	creds := filepath.Join(t.TempDir(), "broker.creds")
	contractWrite(t, creds, credentials)
	leaf := pki.Leaf("broker", "broker.test")
	listeners := make([]net.Listener, 6)
	ports := make([]int, 6)
	for i := range listeners {
		listeners[i], err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		ports[i] = listeners[i].Addr().(*net.TCPAddr).Port
		t.Cleanup(func() { _ = listeners[i].Close() })
	}
	binary := os.Getenv("NATS_SERVER_BINARY")
	if binary == "" {
		binary = "nats-server"
	}
	for i := 0; i < 3; i++ {
		dir := t.TempDir()
		routes := []string{}
		for j := 0; j < 3; j++ {
			if i != j {
				routes = append(routes, fmt.Sprintf("nats-route://127.0.0.1:%d", ports[2*j+1]))
			}
		}
		text := fmt.Sprintf("server_name: api_contract_%d\nhost: 127.0.0.1\nport: %d\nmax_payload: 2097152\njetstream { store_dir: %q }\ntls { cert_file: %q, key_file: %q, ca_file: %q }\noperator: %q\nsystem_account: %q\nresolver: MEMORY\nresolver_preload: { %q: %q, %q: %q }\ncluster { name: api_contract\nlisten: 127.0.0.1:%d\nroutes: [%s] }\n", i, ports[2*i], filepath.Join(dir, "store"), leaf.CertFile, leaf.KeyFile, pki.CAFile, operatorJWT, systemPublic, accountPublic, accountJWT, systemPublic, systemJWT, ports[2*i+1], strings.Join(routes, ","))
		path := filepath.Join(dir, "server.conf")
		contractWrite(t, path, []byte(text))
		_ = listeners[2*i].Close()
		_ = listeners[2*i+1].Close()
		contractProcess(t, exec.Command(binary, "-c", path))
	}
	address := fmt.Sprintf("tls://127.0.0.1:%d", ports[0])
	var nc *nats.Conn
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		nc, err = nats.Connect(address, nats.UserCredentials(creds), nats.RootCAs(pki.CAFile), nats.NoReconnect(), nats.Timeout(200*time.Millisecond))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal("authenticated broker connection", err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	for {
		bounded, stop := context.WithTimeout(ctx, 500*time.Millisecond)
		_, err = js.CreateStream(bounded, jetstream.StreamConfig{Name: "API_CONTRACT_READY", Subjects: []string{"api.contract.ready"}, Replicas: 3, Storage: jetstream.FileStorage})
		stop()
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("broker cluster election", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := js.DeleteStream(ctx, "API_CONTRACT_READY"); err != nil {
		t.Fatal(err)
	}
	return nc, js, address, creds
}

func contractEnvironment(values map[string]string) []string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		env[key] = value
	}
	fields := reflect.TypeOf(config.Config{})
	for i := 0; i < fields.NumField(); i++ {
		delete(env, strings.Split(fields.Field(i).Tag.Get("env"), ",")[0])
	}
	for key, value := range values {
		env[key] = value
	}
	result := make([]string, 0, len(env))
	for key, value := range env {
		result = append(result, key+"="+value)
	}
	return result
}

func productionSocketRuntime(t *testing.T) {
	root := internaltest.RepositoryRoot(t)
	module := filepath.Join(root, "services", "launchpad", "api")
	database := internaltest.NewAPIRuntimeDatabase(t, "api_sockets_runtime", "api_stream", "recovery", "migration_control")
	schemaPath, _ := productionConstructorSchema(t, database)
	values, out, bin := t.TempDir(), t.TempDir(), t.TempDir()
	examples := filepath.Join(root, "deploy", "launchpad", "documents", "values")
	for _, name := range []string{"common.example.json", "api-sockets.example.json"} {
		contractWrite(t, filepath.Join(values, name), contractRead(t, filepath.Join(examples, name)))
	}
	var budget production.ConnectionBudget
	if err := json.Unmarshal(contractRead(t, filepath.Join(values, "common.example.json")), &budget); err != nil {
		t.Fatal(err)
	}
	budget.Targets[0].Host, budget.Targets[0].Port = database.Host, database.Port
	// The direct route binds this deployment to the registered runtime login.
	for i := range budget.Clients {
		client := &budget.Clients[i]
		if client.Role == "apid:sockets" {
			client.Transport, client.Pooler = runtimedb.TransportDirect, ""
			client.Alias, client.User = database.Database, database.RuntimeRole
		}
	}
	if err := budget.Validate(); err != nil {
		t.Fatal(err)
	}
	contractJSON(t, filepath.Join(values, "common.json"), budget)
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	contractWrite(t, filepath.Join(values, "expected-schema", "api-sockets.json"), contractRead(t, schemaPath))
	compose := exec.Command("python3", filepath.Join(root, "deploy", "launchpad", "documents", "compose.py"), "--examples", values, "--out", out, "--service", "api-sockets")
	compose.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if output, err := compose.CombinedOutput(); err != nil {
		t.Fatalf("compose sockets: %v: %s", err, output)
	}
	var environment map[string]string
	if err := json.Unmarshal(contractRead(t, filepath.Join(out, "api-sockets.json")), &environment); err != nil {
		t.Fatal(err)
	}
	var contracts map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	if err := json.Unmarshal(contractRead(t, filepath.Join(root, "deploy", "launchpad", "railway", "variables.json")), &contracts); err != nil {
		t.Fatal(err)
	}
	for key, value := range contracts["api-sockets"].Fixed {
		environment[key] = value
	}
	pki := testpki.New(t)
	client := pki.Leaf("api", "api.test")
	identityLeaf := pki.Leaf("identity", "identity.test")
	identity := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	identity.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{identityLeaf.Pair}, ClientCAs: pki.Roots(), ClientAuth: tls.RequireAndVerifyClientCert}
	identity.StartTLS()
	defer identity.Close()
	nc, js, address, credentials := contractBroker(t, pki)
	for _, target := range []struct{ name, path string }{{"apid", "./cmd/apid"}, {"provision", "github.com/Sidiora-Technologies/KindleLaunch/shared/brokerprovision/cmd/brokerprovision"}} {
		build := exec.Command("go", "build", "-o", filepath.Join(bin, target.name), target.path)
		build.Dir = module
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build executable: %v: %s", err, output)
		}
	}
	provision := func() {
		t.Helper()
		command := exec.Command(filepath.Join(bin, "provision"), "provision", "--document", filepath.Join(examples, "broker.example.json"), "--broker-env", "API_CONTRACT_BROKER", "--credentials-file", credentials, "--timeout", "2m")
		command.Env = append(os.Environ(), "SSL_CERT_FILE="+pki.CAFile, "API_CONTRACT_BROKER="+address)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("provision broker: %v: %s", err, output)
		}
	}
	provision()
	variables := map[string]string{
		"DATABASE_URL": database.RuntimeDSN, "REDIS_URL": internaltest.NewRedisURL(t),
		"DB_POOL_ID": "apid_sockets_pool", "INSTANCE_ID": "api_contract_sockets", "ZONE": "zone-a",
		"IDENTITY_SERVICE_URL": identity.URL, "IDENTITY_CA_FILE": pki.CAFile,
		"IDENTITY_CLIENT_CERT_FILE": client.CertFile, "IDENTITY_CLIENT_KEY_FILE": client.KeyFile,
		"IDENTITY_PEER_CERT_SHA256": identityLeaf.SHA256,
		"IDENTITY_ASSERTION_ISSUER": "api", "IDENTITY_SIGNING_KEY": base64.RawStdEncoding.EncodeToString(testpki.Ed25519(t).Private),
		"STREAM_SIGNING_KEY": strings.Repeat("s", 32), "NATS_URL": address, "NATS_CREDENTIALS": credentials,
		"CORS_ALLOWED_ORIGINS": "https://web.example", "SSL_CERT_FILE": pki.CAFile,
	}
	for key, value := range variables {
		environment[key] = value
	}
	for _, key := range contracts["api-sockets"].Required {
		if environment[key] == "" {
			t.Fatalf("required socket setting %s missing", key)
		}
	}
	var consumers []natsdriver.ConsumerConfig
	raw, err := base64.StdEncoding.DecodeString(environment["STREAM_CONSUMER_CONFIG_DOCUMENT"])
	if err != nil || json.Unmarshal(raw, &consumers) != nil || len(consumers) != 1 {
		t.Fatal("composed socket consumer document")
	}
	assignment := consumers[0]
	if err := readyStreamConsumers(context.Background(), nc, consumers); err != nil {
		t.Fatal(err)
	}
	missing := assignment
	missing.Durable = "unassigned_durable"
	if err := readyStreamConsumers(context.Background(), nc, append(append([]natsdriver.ConsumerConfig{}, consumers...), missing)); err == nil {
		t.Fatal("missing second durable reported ready")
	}
	stream, err := js.Stream(context.Background(), assignment.Stream)
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	shortRetention := info.Config
	shortRetention.MaxAge = 71 * time.Hour
	if _, err := js.UpdateStream(context.Background(), shortRetention); err != nil {
		t.Fatal(err)
	}
	if err := readyStreamConsumers(context.Background(), nc, consumers); err == nil {
		t.Fatal("insufficient replay retention reported ready")
	}
	if _, err := js.UpdateStream(context.Background(), info.Config); err != nil {
		t.Fatal(err)
	}
	consumerConfig, err := assignment.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	drifted := consumerConfig
	drifted.AckWait += time.Second
	if _, err := js.UpdateConsumer(context.Background(), assignment.Stream, drifted); err != nil {
		t.Fatal(err)
	}
	if err := readyStreamConsumers(context.Background(), nc, consumers); err == nil {
		t.Fatal("durable configuration drift reported ready")
	}
	if _, err := js.UpdateConsumer(context.Background(), assignment.Stream, consumerConfig); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), database.RuntimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	admin, err := pgxpool.New(context.Background(), database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	writer, err := broker.NewPostgresStream(admin, "api_stream", []byte(variables["STREAM_SIGNING_KEY"]), sharedauth.HTTPStreamAuthority{})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := broker.NewPostgresStream(pool, "api_stream", []byte(variables["STREAM_SIGNING_KEY"]), sharedauth.HTTPStreamAuthority{})
	if err != nil {
		t.Fatal(err)
	}
	frames := map[string]json.RawMessage{}
	if err := json.Unmarshal(contractRead(t, filepath.Join(module, "internal", "broker", "frozen_stream_fixtures_test.json")), &frames); err != nil {
		t.Fatal(err)
	}
	var template broker.Frame
	if err := json.Unmarshal(frames["StatsEvent"], &template); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(template.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["poolAddress"] = strings.TrimPrefix(assignment.Subject, "market.stats.")
	template.Payload, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	template.Topic = assignment.Subject
	publisher, err := natsdriver.NewPublisher(nc, assignment.Stream)
	if err != nil {
		t.Fatal(err)
	}
	publish := func(sequence int) broker.Frame {
		t.Helper()
		frame := template
		frame.EventID, frame.Sequence, frame.Revision = uuid.NewString(), fmt.Sprint(sequence), fmt.Sprint(sequence)
		var watermark map[string]any
		if err := json.Unmarshal(template.Watermark, &watermark); err != nil {
			t.Fatal(err)
		}
		watermark["projectionRevision"] = frame.Revision
		frame.Watermark, err = json.Marshal(watermark)
		if err != nil {
			t.Fatal(err)
		}
		payload["watermark"] = watermark
		frame.Payload, err = json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := admin.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if err := writer.AppendProjectionTx(context.Background(), tx, frame); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		wire, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		if ack, err := publisher.Publish(context.Background(), frame.Topic, delivery.Hash([]byte(frame.EventID)), wire); err != nil || ack.Sequence != uint64(sequence) {
			t.Fatalf("committed publication: %+v %v", ack, err)
		}
		return frame
	}
	start := func() func() {
		command := exec.Command(filepath.Join(bin, "apid"))
		command.Env = contractEnvironment(environment)
		return contractProcess(t, command)
	}
	httpClient := &http.Client{Timeout: 3 * time.Second}
	ready := func(want int) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		var body []byte
		for time.Now().Before(deadline) {
			response, err := httpClient.Get("http://127.0.0.1:8080/health/ready")
			if err == nil {
				body, err = io.ReadAll(response.Body)
				_ = response.Body.Close()
				var health struct {
					Status string            `json:"status"`
					Checks map[string]string `json:"checks"`
				}
				if err == nil && response.StatusCode == want && json.Unmarshal(body, &health) == nil {
					if want == 200 && health.Status == "ok" && health.Checks["jetstream"] == "ok" || want == 503 && health.Checks["jetstream"] == "failed" && health.Checks["database"] == "ok" && health.Checks["redis"] == "ok" {
						return
					}
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
		t.Fatalf("readiness did not reach %d: %s", want, body)
	}
	acknowledged := func(sequence uint64) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			consumer, err := js.Consumer(context.Background(), assignment.Stream, assignment.Durable)
			if err == nil {
				info, err := consumer.Info(context.Background())
				if err == nil && info.AckFloor.Stream == sequence && info.NumAckPending == 0 {
					return
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
		t.Fatalf("durable did not acknowledge sequence %d", sequence)
	}
	open := func(cursor string) (*http.Response, *bufio.Reader) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v2/stream/sse?topics="+assignment.Subject, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", variables["CORS_ALLOWED_ORIGINS"])
		req.Header.Set("Last-Event-ID", cursor)
		response, err := httpClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = response.Body.Close() })
		if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" || response.Header.Get("X-Schema-Version") != "2" {
			t.Fatalf("stream response: %d %v", response.StatusCode, response.Header)
		}
		return response, bufio.NewReader(response.Body)
	}
	event := func(input *bufio.Reader, op string) (map[string]any, string) {
		t.Helper()
		cursor := ""
		for {
			line, err := input.ReadString('\n')
			if err != nil {
				t.Fatal("stream event", err)
			}
			if strings.HasPrefix(line, "id: ") {
				cursor = strings.TrimSpace(strings.TrimPrefix(line, "id: "))
			}
			if strings.HasPrefix(line, "data: ") {
				raw := []byte(strings.TrimSpace(strings.TrimPrefix(line, "data: ")))
				var frame map[string]any
				if err := json.Unmarshal(raw, &frame); err != nil {
					t.Fatal(err)
				}
				if op != "" {
					if err := v2.ValidateJSON("StreamControl", raw); err != nil || frame["op"] != op {
						t.Fatalf("stream control: %s %v", raw, err)
					}
				} else if err := v2.ValidateJSON("StatsEvent", raw); err != nil {
					t.Fatal(err)
				}
				return frame, cursor
			}
		}
	}
	stop := start()
	ready(200)
	publish(1)
	acknowledged(1)
	snapshot, err := reader.PublicSnapshot(context.Background(), []string{assignment.Subject})
	if err != nil {
		t.Fatal(err)
	}
	var installed struct {
		ResumeCursor string `json:"resumeCursor"`
	}
	if err := json.Unmarshal(snapshot, &installed); err != nil || installed.ResumeCursor == "" {
		t.Fatalf("public snapshot: %s %v", snapshot, err)
	}
	response, events := open(installed.ResumeCursor)
	event(events, "subscribed")
	event(events, "checkpoint")
	second := publish(2)
	frame, cursor := event(events, "")
	if frame["eventId"] != second.EventID || frame["sequence"] != "2" || cursor == "" {
		t.Fatalf("committed stream event: %v cursor=%q", frame, cursor)
	}
	_ = response.Body.Close()
	acknowledged(2)
	stop()
	third := publish(3)
	stop = start()
	ready(200)
	acknowledged(3)
	response, events = open(cursor)
	event(events, "subscribed")
	frame, _ = event(events, "")
	if frame["eventId"] != third.EventID || frame["sequence"] != "3" {
		t.Fatalf("recovered stream event: %v", frame)
	}
	event(events, "checkpoint")
	_ = response.Body.Close()
	if err := js.DeleteConsumer(context.Background(), assignment.Stream, assignment.Durable); err != nil {
		t.Fatal(err)
	}
	ready(503)
	provision()
	ready(200)
	acknowledged(3)
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM api_stream.stream_delivery_outbox JOIN api_stream.stream_projection USING (partition_id) WHERE topic=$1", assignment.Subject).Scan(&count); err != nil || count != 3 {
		t.Fatalf("retained committed events: %d %v", count, err)
	}
	stop()
}

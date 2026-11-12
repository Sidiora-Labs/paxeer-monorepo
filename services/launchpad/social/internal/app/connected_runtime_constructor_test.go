package app

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

func TestConnectedRuntimeValidatesBrokerAfterWiringAuthorities(t *testing.T) {
	h := newPlatformHarness(t)
	key := func(path string) string {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	invalidMaterial := func(name, body string) (string, string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path, fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
	}
	badCA, badCADigest := invalidMaterial("ca.pem", "not a certificate")
	badCert, badCertDigest := invalidMaterial("cert.pem", "not a certificate")
	t.Setenv("SOCIAL_INVALID_SIGNING_KEY", "invalid-signing-key")
	t.Setenv("SOCIAL_INVALID_CURSOR_KEY", base64.RawURLEncoding.EncodeToString([]byte("short")))
	digest := func(path string) string {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	t.Setenv("IDENTITY_ASSERTION_SIGNING_KEY", base64.RawURLEncoding.EncodeToString(h.pki.socialKey.Private))
	t.Setenv("IDENTITY_MTLS_CLIENT_KEY_PEM", key(h.pki.socialClient.KeyFile))
	t.Setenv("PRIVATE_TLS_KEY_PEM", key(h.pki.socialServer.KeyFile))
	binding := config.SocialRuntimeBinding{
		Version: 2, GatewayDNS: "gateway", GatewayCertSHA256: h.pki.gatewayClient.SHA256,
		CAFile: h.pki.ca.CAFile, CASHA256: digest(h.pki.ca.CAFile),
		ServerCertFile: h.pki.socialServer.CertFile, ServerCertSHA256: digest(h.pki.socialServer.CertFile), ServerKeyReference: "PRIVATE_TLS_KEY_PEM",
		ClientCertFile: h.pki.socialClient.CertFile, ClientCertSHA256: digest(h.pki.socialClient.CertFile), ClientKeyReference: "IDENTITY_MTLS_CLIENT_KEY_PEM",
		IdentityEndpoint: h.wire.server.URL, IdentityServerPeerCertSHA256: h.pki.gatewayServer.SHA256, IdentitySigningKeyReference: "IDENTITY_ASSERTION_SIGNING_KEY",
		IdentitySchema: "users", AccountSchema: "users", IndexerSchema: "indexer", SocialSchema: "chat", DeliverySchema: "chat",
		RecoverySchema: "recovery", RecoveryEpoch: 1, RecoveryRegion: "zone-a", ConsumerGeneration: "social-v2",
		InteractionSubjects: map[string]string{"social.edge.changed": "social.edge", "social.comment.changed": "social.comment", "social.callout.changed": "social.callout"},
		MentionSubject:      "social.mention", ErasureSubject: "social.erasure", PolicySubject: "social.policy",
		CursorKeyReference: "SOCIAL_CURSOR_KEY", NotificationEndpoint: h.wire.server.URL, NotificationTokenReference: "CHAT_ADMIN_API_KEY",
		PendingOutboxLimit: 100, MaxOutboxAgeSeconds: 3600, NATSURL: "nats:broker",
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "social-runtime.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{SecretsProvider: "env", SocialRuntimeFile: path, SocialRuntimeSHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	runtime, err := newSocialConnectedRuntime(context.Background(), cfg, h.app.pool, h.app.redis, slog.Default())
	if runtime != nil || err == nil || !strings.Contains(err.Error(), "private TLS broker required") {
		t.Fatalf("broker transport binding = %v, runtime %v", err, runtime)
	}
	for _, tc := range []struct {
		name   string
		change func(*config.SocialRuntimeBinding)
		want   string
	}{
		{"owner schema", func(b *config.SocialRuntimeBinding) { b.SocialSchema = "other" }, "owner schema binding mismatch"},
		{"version", func(b *config.SocialRuntimeBinding) { b.Version = 1 }, "runtime binding incomplete"},
		{"CA binding", func(b *config.SocialRuntimeBinding) { b.CASHA256 = strings.Repeat("0", 64) }, "bound"},
		{"CA certificate", func(b *config.SocialRuntimeBinding) { b.CAFile, b.CASHA256 = badCA, badCADigest }, "private CA invalid"},
		{"server certificate", func(b *config.SocialRuntimeBinding) { b.ServerCertFile, b.ServerCertSHA256 = badCert, badCertDigest }, "failed to find any PEM data"},
		{"client certificate", func(b *config.SocialRuntimeBinding) { b.ClientCertFile, b.ClientCertSHA256 = badCert, badCertDigest }, "failed to find any PEM data"},
		{"server key reference", func(b *config.SocialRuntimeBinding) { b.ServerKeyReference = "" }, "secret reference required"},
		{"client certificate binding", func(b *config.SocialRuntimeBinding) { b.ClientCertSHA256 = strings.Repeat("0", 64) }, "bound"},
		{"identity endpoint", func(b *config.SocialRuntimeBinding) {
			b.IdentityEndpoint = "http://" + strings.TrimPrefix(h.wire.server.URL, "https://")
		}, "private identity endpoint required"},
		{"identity signing reference", func(b *config.SocialRuntimeBinding) { b.IdentitySigningKeyReference = "" }, "secret reference required"},
		{"identity signing key", func(b *config.SocialRuntimeBinding) { b.IdentitySigningKeyReference = "SOCIAL_INVALID_SIGNING_KEY" }, "identity signing key invalid"},
		{"event subject", func(b *config.SocialRuntimeBinding) { b.InteractionSubjects["invalid.event"] = "social.invalid" }, "event type invalid"},
		{"missing event subjects", func(b *config.SocialRuntimeBinding) { b.InteractionSubjects = nil }, "subjects missing"},
		{"cursor reference", func(b *config.SocialRuntimeBinding) { b.CursorKeyReference = "" }, "secret reference required"},
		{"recovery epoch", func(b *config.SocialRuntimeBinding) { b.RecoveryEpoch = 0 }, "invalid recovery authority"},
		{"recovery region", func(b *config.SocialRuntimeBinding) { b.RecoveryRegion = "" }, ""},
		{"delivery schema", func(b *config.SocialRuntimeBinding) { b.DeliverySchema = "" }, "invalid store configuration"},
		{"notification endpoint", func(b *config.SocialRuntimeBinding) { b.NotificationEndpoint = "http:" }, "notification"},
		{"notification token", func(b *config.SocialRuntimeBinding) { b.NotificationTokenReference = "UNBOUND_NOTIFICATION_TOKEN" }, ""},
		{"cursor key", func(b *config.SocialRuntimeBinding) { b.CursorKeyReference = "SOCIAL_INVALID_CURSOR_KEY" }, "social cursor key invalid"},
		{"voice qualification", func(b *config.SocialRuntimeBinding) { b.Voice = &config.VoiceRuntimeBinding{} }, "voice unavailable"},
		{"broker credentials", func(b *config.SocialRuntimeBinding) {
			b.NATSURL = "tls://" + strings.TrimPrefix(os.Getenv("SOCIAL_TEST_NATS_URL"), "nats://")
			b.NATSCredentialsReference = ""
		}, "secret reference required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := binding
			candidate.InteractionSubjects = make(map[string]string, len(binding.InteractionSubjects))
			for kind, subject := range binding.InteractionSubjects {
				candidate.InteractionSubjects[kind] = subject
			}
			tc.change(&candidate)
			body, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "social-runtime.json")
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			candidateConfig := cfg
			candidateConfig.SocialRuntimeFile = path
			candidateConfig.SocialRuntimeSHA256 = fmt.Sprintf("%x", sha256.Sum256(body))
			value, err := newSocialConnectedRuntime(context.Background(), candidateConfig, h.app.pool, h.app.redis, slog.Default())
			if value != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("runtime binding = %v, runtime %v; want %s", err, value, tc.want)
			}
		})
	}
}

func TestConnectedRuntimeBindsRealBrokerAndConsumers(t *testing.T) {
	h := newPlatformHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	key := func(path string) string {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	digest := func(path string) string {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	t.Setenv("IDENTITY_ASSERTION_SIGNING_KEY", base64.RawURLEncoding.EncodeToString(h.pki.socialKey.Private))
	t.Setenv("IDENTITY_MTLS_CLIENT_KEY_PEM", key(h.pki.socialClient.KeyFile))
	t.Setenv("PRIVATE_TLS_KEY_PEM", key(h.pki.socialServer.KeyFile))
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	defer user.Wipe()
	account, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	defer account.Wipe()
	accountKey, err := account.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	userKey, err := user.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]any{"iss": accountKey, "sub": userKey, "iat": time.Now().Unix(), "nats": map[string]any{"type": "user", "pub": map[string]any{}, "sub": map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	jwt := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ed25519-nkey"}`)) + "." + base64.RawURLEncoding.EncodeToString(claims)
	signature, err := account.Sign([]byte(jwt))
	if err != nil {
		t.Fatal(err)
	}
	jwt += "." + base64.RawURLEncoding.EncodeToString(signature)
	credentials := "-----BEGIN NATS USER JWT-----\n" + jwt + "\n------END NATS USER JWT------\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	t.Setenv("SOCIAL_TEST_BROKER_CREDENTIALS", credentials)
	leaf := h.pki.ca.Leaf("social-broker", "localhost")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	upstream := strings.TrimPrefix(os.Getenv("SOCIAL_TEST_NATS_URL"), "nats://")
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer client.Close()
				broker, err := net.Dial("tcp", upstream)
				if err != nil {
					return
				}
				defer broker.Close()
				info, err := bufio.NewReader(broker).ReadBytes('\n')
				if err != nil {
					return
				}
				var protocol map[string]any
				if err := json.Unmarshal(info[5:], &protocol); err != nil {
					return
				}
				protocol["tls_required"] = true
				wire, err := json.Marshal(protocol)
				if err != nil {
					return
				}
				if _, err := client.Write(append(append([]byte("INFO "), wire...), '\r', '\n')); err != nil {
					return
				}
				secure := tls.Server(client, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{leaf.Pair}})
				if err := secure.Handshake(); err != nil {
					return
				}
				go func() { _, _ = io.Copy(broker, secure); _ = broker.Close() }()
				_, _ = io.Copy(secure, broker)
			}()
		}
	}()
	nc, err := nats.Connect(os.Getenv("SOCIAL_TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	var recoveryEpoch int64
	var recoveryRegion string
	if err := h.dbConn.QueryRow(ctx, `SELECT epoch,region FROM recovery.recovery_control WHERE singleton=true`).Scan(&recoveryEpoch, &recoveryRegion); err != nil {
		t.Fatal(err)
	}
	stream, subject := "SOCIAL_"+suffix, "fixture.social."+suffix
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: stream, Subjects: []string{subject + ".>"}, Storage: jetstream.FileStorage, Replicas: 3, Discard: jetstream.DiscardNew, MaxBytes: 8 * 1024 * 1024, MaxMsgs: 1000, MaxAge: time.Hour}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = js.DeleteStream(context.Background(), stream) }()
	for _, route := range []struct{ durable, subject string }{{"chat_" + suffix, subject + ".chat"}, {"erase_" + suffix, subject + ".erase"}, {"policy_" + suffix, subject + ".policy"}} {
		cfg := natsdriver.ConsumerConfig{Stream: stream, Durable: route.durable, Subject: route.subject, Batch: 32, MaxAckPending: 128, MaxWaiting: 4, MaxDeliver: 10, FetchWait: 500 * time.Millisecond, AckWait: 30 * time.Second}
		brokerConfig, err := cfg.BrokerConsumerConfig()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := js.CreateConsumer(ctx, stream, brokerConfig); err != nil {
			t.Fatal(err)
		}
	}
	binding := config.SocialRuntimeBinding{
		Version: 2, GatewayDNS: "gateway.railway.internal", GatewayCertSHA256: h.pki.gatewayClient.SHA256,
		CAFile: h.pki.ca.CAFile, CASHA256: digest(h.pki.ca.CAFile),
		ServerCertFile: h.pki.socialServer.CertFile, ServerCertSHA256: digest(h.pki.socialServer.CertFile), ServerKeyReference: "PRIVATE_TLS_KEY_PEM",
		ClientCertFile: h.pki.socialClient.CertFile, ClientCertSHA256: digest(h.pki.socialClient.CertFile), ClientKeyReference: "IDENTITY_MTLS_CLIENT_KEY_PEM",
		IdentityEndpoint: h.wire.server.URL, IdentityServerPeerCertSHA256: h.pki.gatewayServer.SHA256, IdentitySigningKeyReference: "IDENTITY_ASSERTION_SIGNING_KEY",
		IdentitySchema: "users", AccountSchema: "users", IndexerSchema: "indexer", SocialSchema: "chat", DeliverySchema: "chat",
		RecoverySchema: "recovery", RecoveryEpoch: recoveryEpoch, RecoveryRegion: recoveryRegion, ConsumerGeneration: "social-v2-" + suffix,
		InteractionSubjects: map[string]string{"social.edge.changed": subject + ".edge", "social.comment.changed": subject + ".comment", "social.callout.changed": subject + ".callout"},
		MentionSubject:      subject + ".mention", ErasureSubject: subject + ".erase", PolicySubject: subject + ".policy",
		CursorKeyReference: "SOCIAL_CURSOR_KEY", NotificationEndpoint: h.wire.server.URL, NotificationTokenReference: "CHAT_ADMIN_API_KEY",
		PendingOutboxLimit: 100, MaxOutboxAgeSeconds: 3600, NATSURL: "tls://localhost:" + fmt.Sprint(listener.Addr().(*net.TCPAddr).Port),
		NATSCredentialsReference: "SOCIAL_TEST_BROKER_CREDENTIALS", ChatStream: stream, ChatSubject: subject + ".chat", ChatDurable: "chat_" + suffix,
		ErasureStream: stream, ErasureDurable: "erase_" + suffix, PolicyStream: stream, PolicyDurable: "policy_" + suffix, EventStream: stream,
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "social-runtime.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{SecretsProvider: "env", SocialRuntimeFile: path, SocialRuntimeSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), RuntimeInstanceID: "social-fixture"}
	runtime, err := newSocialConnectedRuntime(ctx, cfg, h.app.pool, h.app.redis, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); runtime.close() }()
	if runtime.chat == nil || runtime.chatConsumer == nil || runtime.erasureConsumer == nil || runtime.policyConsumer == nil || runtime.broker.Stream != stream {
		t.Fatalf("connected broker binding incomplete: %+v", runtime)
	}
	hub, err := fanout.NewConnected(fanout.Deps{Durable: runtime.chat, Pub: h.app.redis, Sub: h.app.sub, Authenticate: runtime.ticket, ProtocolVersion: 2})
	if err != nil {
		t.Fatal(err)
	}
	runtime.start(hub)
	sender, recipient := "0x"+suffix+"00000000", "0x"+suffix+"00000001"
	h.wire.session("Bearer connected-sender", sender)
	h.wire.session("Bearer connected-recipient", recipient)
	channel := fanout.WalletChannel(runtime.chat.ChannelNamespace, recipient)
	subscription := h.app.redis.Subscribe(ctx, channel)
	defer subscription.Close()
	if _, err := subscription.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v2/chat/send", nil)
	req.Header.Set("Authorization", "Bearer connected-sender")
	principal, err := runtime.authenticateRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	message, err := runtime.chat.Send(interactions.WithIdentity(ctx, principal), fanout.SendInput{Actor: sender, Target: recipient, Kind: "dm", ClientID: "connected-runtime-message", Content: "committed connected message"})
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := subscription.ReceiveMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var event fanout.DurableMessage
	if err := json.Unmarshal([]byte(delivered.Payload), &event); err != nil || event.ID != message.ID || event.Content != message.Content {
		t.Fatalf("connected runtime delivery = %+v: %v", event, err)
	}
	caseID := uuid.NewString()
	visibility := safety.Visibility{TargetType: "message", TargetID: message.ID, Status: "removed", CaseID: caseID, Revision: 1}
	visibilityWire, err := json.Marshal(visibility)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,0,101,0,$2,$3,'[]','\x'::bytea)`, "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64), h.creator); err != nil {
		t.Fatal(err)
	}
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'social',101,$1,1,true,true)`, "0x"+strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	connection, err := pgx.ParseConfig(h.db.Runtimes["chat_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	budget := dbproduction.ConnectionBudget{
		Identity:    "social_connected_runtime",
		Targets:     []dbproduction.PostgresTarget{{ID: "primary", Member: "social_database", Host: connection.Host, Port: connection.Port, MaxConnections: 100}},
		Operational: []dbproduction.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}},
	}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := strings.ReplaceAll(role, ":", "_")
		if role == "sociald" {
			user = connection.User
		}
		budget.Clients = append(budget.Clients, dbproduction.ClientBudget{Role: role, PoolID: "interactive", Target: "primary", Transport: shareddb.TransportDirect, Alias: connection.Database, User: user, QueryClass: shareddb.Interactive, Processes: dbproduction.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 1})
	}
	budgetWire, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	boot := platformConfig(h, h.db)
	boot.RuntimeProfile = "owned_local"
	boot.BindAddress = "127.0.0.1"
	boot.RuntimePoolID = "interactive"
	boot.RuntimeInstanceID = "social-connected"
	boot.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString(budgetWire)
	boot.RuntimeBudgetSHA256 = fmt.Sprintf("%x", sha256.Sum256(budgetWire))
	boot.SocialRuntimeFile = path
	boot.SocialRuntimeSHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	app, err := New(ctx, boot, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if app.connected == nil || app.platform != nil || app.dev != nil || app.Hub == nil || app.Router == nil {
		t.Fatalf("connected application wiring = connected %t platform %t development %t", app.connected != nil, app.platform != nil, app.dev != nil)
	}
	probe := httptest.NewRecorder()
	app.Router.ServeHTTP(probe, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if probe.Code != http.StatusOK {
		t.Fatalf("connected liveness = %d: %s", probe.Code, probe.Body.String())
	}
	app.Close()
	role := pgx.Identifier{h.app.pool.Config().ConnConfig.User}.Sanitize()
	if _, err := h.dbConn.Exec(ctx, `GRANT SELECT,UPDATE ON chat.safety_outbox TO `+role); err != nil {
		t.Fatal(err)
	}
	tx, err := h.dbConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `INSERT INTO chat.safety_cases(id,target_type,target_id,reporter,category,description) VALUES($1,'message',$2,$3,'spam','reviewed message')`, caseID, message.ID, sender); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat.safety_visibility(target_type,target_id,revision,status,case_id) VALUES('message',$1,1,'removed',$2)`, message.ID, caseID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat.safety_outbox(id,kind,entity_id,revision,payload) VALUES($1,'visibility_changed',$2,1,$3)`, uuid.NewString(), message.ID, visibilityWire); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var projectedRevision int64
	for {
		err = h.dbConn.QueryRow(ctx, `SELECT revision FROM chat.interaction_feed_projection WHERE generation='social_safety_visibility_v1' AND aggregate_id=$1`, "message:"+message.ID).Scan(&projectedRevision)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("committed visibility projection: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	var messageRevision int64
	if err := h.dbConn.QueryRow(ctx, `SELECT revision FROM chat.chat_messages WHERE id=$1`, message.ID).Scan(&messageRevision); err != nil || projectedRevision != 1 || messageRevision != 2 {
		t.Fatalf("removed message projection = %d, message revision %d: %v", projectedRevision, messageRevision, err)
	}
	if failure := runtime.failure.Load(); failure != nil {
		t.Fatalf("connected worker = %v", *failure)
	}

}

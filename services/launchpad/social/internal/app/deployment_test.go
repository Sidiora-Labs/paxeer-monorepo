package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/notifications"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/brokerprovision"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func assertComposedSocialDeployment(t *testing.T, h *platformHarness, binding config.SocialRuntimeBinding) {
	t.Helper()
	root := coordinatortest.RepositoryRoot(t)
	values, output := t.TempDir(), t.TempDir()
	read := func(path string, out any) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	examples := filepath.Join(root, "deploy/launchpad/documents/values")
	for _, credential := range []struct {
		reference *string
		name      string
	}{
		{&binding.ServerKeyReference, "SOCIAL_TLS_KEY_PEM"},
		{&binding.NATSCredentialsReference, "SOCIAL_NATS_CREDENTIALS"},
		{&binding.NotificationTokenReference, "NOTIFICATION_SERVICE_TOKEN"},
	} {
		value := os.Getenv(*credential.reference)
		if value == "" {
			t.Fatalf("social credential missing: %s", *credential.reference)
		}
		t.Setenv(credential.name, value)
		*credential.reference = credential.name
	}
	var notificationCalls atomic.Int64
	notification := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in notifications.ChatRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v2/notifications/chat" || r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || r.Header.Get("Authorization") != "Bearer "+os.Getenv(binding.NotificationTokenReference) || decoder.Decode(&in) != nil || in.EventID == "" || in.AccountID == "" || in.ResourceID == "" || in.CreatedAt.IsZero() {
			http.Error(w, "invalid chat notification", http.StatusBadRequest)
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			http.Error(w, "invalid chat notification", http.StatusBadRequest)
			return
		}
		notificationCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(notifications.ChatResponse{NotificationID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(in.EventID+":"+in.AccountID)).String(), Created: true})
	}))
	notification.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: h.pki.ca.Roots(), Certificates: []tls.Certificate{h.pki.gatewayServer.Pair}}
	notification.StartTLS()
	defer notification.Close()
	binding.NotificationEndpoint = notification.URL
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	var document brokerprovision.Document
	read(filepath.Join(examples, "broker.example.json"), &document)
	streams := document.Streams[:0]
	for _, stream := range document.Streams {
		if stream.Name != "SOCIAL_CHAT" && stream.Name != "SOCIAL_EVENTS" {
			continue
		}
		stream.Name += "_" + suffix
		for i := range stream.Subjects {
			stream.Subjects[i] = "deployment." + suffix + "." + stream.Subjects[i]
		}
		streams = append(streams, stream)
	}
	document.Streams = streams
	durables := document.Durables[:0]
	for _, durable := range document.Durables {
		if durable.Stream != "SOCIAL_CHAT" && durable.Stream != "SOCIAL_EVENTS" {
			continue
		}
		durable.Stream += "_" + suffix
		durable.Durable += "_" + suffix
		durable.Subject = "deployment." + suffix + "." + durable.Subject
		durables = append(durables, durable)
		switch {
		case strings.HasPrefix(durable.Durable, "social-chat-v2_"):
			binding.ChatStream, binding.ChatDurable, binding.ChatSubject = durable.Stream, durable.Durable, durable.Subject
		case strings.HasPrefix(durable.Durable, "social-erasure-v2_"):
			binding.ErasureStream, binding.ErasureDurable, binding.ErasureSubject = durable.Stream, durable.Durable, durable.Subject
		case strings.HasPrefix(durable.Durable, "social-policy-v2_"):
			binding.PolicyStream, binding.PolicyDurable, binding.PolicySubject = durable.Stream, durable.Durable, durable.Subject
		}
	}
	document.Durables = durables
	brokerDocument := filepath.Join(values, "broker.json")
	write(brokerDocument, document)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var provisioning bytes.Buffer
	for _, action := range []string{"provision", "check"} {
		if code := brokerprovision.Run(ctx, []string{action, "--document", brokerDocument, "--broker-env", "SOCIAL_TEST_NATS_URL", "--credentials-env", "SOCIAL_TEST_BROKER_CREDENTIALS"}, &provisioning, &provisioning); code != 0 {
			t.Fatalf("stream %s = %d: %s", action, code, provisioning.String())
		}
	}
	nc, err := nats.Connect(os.Getenv("SOCIAL_TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, stream := range document.Streams {
			if err := js.DeleteStream(context.Background(), stream.Name); err != nil {
				t.Error(err)
			}
		}
	}()
	binding.EventStream = binding.ErasureStream
	binding.ConsumerGeneration = "social-v2-" + suffix
	binding.AuthorityPeers = map[string]string{"ranking.railway.internal": h.pki.rankingClient.SHA256}
	for event := range binding.InteractionSubjects {
		binding.InteractionSubjects[event] = "deployment." + suffix + "." + strings.TrimSuffix(event, ".changed")
	}
	binding.MentionSubject = "deployment." + suffix + ".social.mention"
	var socialValues map[string]any
	read(filepath.Join(examples, "social.example.json"), &socialValues)
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	var boundValues map[string]any
	if err := json.Unmarshal(raw, &boundValues); err != nil {
		t.Fatal(err)
	}
	for name := range socialValues {
		value, ok := boundValues[name]
		if !ok {
			t.Fatalf("social document field missing: %s", name)
		}
		if strings.HasSuffix(name, "Reference") {
			value = map[string]any{"value": value, "test": true}
		}
		socialValues[name] = value
	}
	write(filepath.Join(values, "social.json"), socialValues)
	var budget dbproduction.ConnectionBudget
	read(filepath.Join(examples, "common.example.json"), &budget)
	connection, err := pgx.ParseConfig(h.db.Runtimes["chat_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	budget.Identity = "social_deployment_" + suffix
	budget.Poolers = nil
	budget.Targets[0].Host, budget.Targets[0].Port = connection.Host, connection.Port
	for i := range budget.Clients {
		client := &budget.Clients[i]
		client.Transport, client.Pooler, client.Alias = shareddb.TransportDirect, "", connection.Database
		if client.Role == "sociald" {
			client.User = connection.User
		}
	}
	write(filepath.Join(values, "common.json"), budget)
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0o700); err != nil {
		t.Fatal(err)
	}
	schemaFile, _ := h.db.ExpectedSchemaFile(t, "chat_runtime")
	schema, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "expected-schema/social.json"), schema, 0o600); err != nil {
		t.Fatal(err)
	}
	composer := exec.CommandContext(ctx, "python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", output, "--service", "social")
	if result, err := composer.CombinedOutput(); err != nil {
		t.Fatalf("compose social documents: %v: %s", err, result)
	}
	var env map[string]string
	read(filepath.Join(output, "social.json"), &env)
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	read(filepath.Join(root, "deploy/launchpad/railway/variables.json"), &contract)
	for key, value := range contract["social"].Fixed {
		env[key] = value
	}
	for key, value := range map[string]string{
		"DATABASE_URL": h.db.Runtimes["chat_runtime"].DSN,
		"DB_POOL_ID":   "sociald_pool", "INSTANCE_ID": "social-deployment-" + suffix,
		"ZONE": "zone-a", "DB_BUDGET_FILE": "", "EXPECTED_SCHEMA_FILE": "", "SOCIAL_RUNTIME_FILE": "",
	} {
		env[key] = value
	}
	for _, key := range contract["social"].Required {
		value := os.Getenv(key)
		if composed, ok := env[key]; ok {
			value = composed
		}
		if value == "" {
			t.Fatalf("required social setting missing: %s", key)
		}
	}
	binary := filepath.Join(t.TempDir(), "sociald")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/sociald")
	build.Dir = filepath.Join(root, "services/launchpad/social")
	if result, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build social entrypoint: %v: %s", err, result)
	}
	processEnvironment := make([]string, 0, len(os.Environ())+len(env))
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if _, replaced := env[key]; !replaced {
			processEnvironment = append(processEnvironment, value)
		}
	}
	for key, value := range env {
		processEnvironment = append(processEnvironment, key+"="+value)
	}
	client := h.pki.mtlsClient(h.pki.gatewayClient)
	client.Timeout = 3 * time.Second
	defer client.CloseIdleConnections()
	health := &http.Client{Timeout: time.Second}
	commentID, clientID := "", uuid.NewString()
	h.wire.session("Bearer deployment-social", addr(0x878))
	for boot := 0; boot < 2; boot++ {
		func() {
			logFile, err := os.CreateTemp(t.TempDir(), "social-*.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			defer logFile.Close()
			command := exec.CommandContext(ctx, binary)
			command.Env = processEnvironment
			command.Stdout, command.Stderr = logFile, logFile
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			defer func() {
				_ = command.Process.Kill()
				<-done
			}()
			fail := func(format string, args ...any) {
				t.Helper()
				logs, _ := os.ReadFile(logFile.Name())
				t.Fatalf("%s\n%s", fmt.Sprintf(format, args...), logs)
			}
			deadline := time.Now().Add(15 * time.Second)
			stable := time.Time{}
			var last []byte
			for {
				response, err := health.Get("http://127.0.0.1:" + env["PORT"] + "/health/ready")
				if err == nil {
					last, err = io.ReadAll(response.Body)
					response.Body.Close()
					if err == nil && response.StatusCode == http.StatusOK {
						if stable.IsZero() {
							stable = time.Now()
						}
						if time.Since(stable) >= time.Second {
							break
						}
					} else {
						stable = time.Time{}
					}
				}
				if time.Now().After(deadline) {
					fail("social readiness did not stabilize: %s", last)
				}
				select {
				case err := <-done:
					done <- err
					fail("social entrypoint exited: %v", err)
				case <-time.After(50 * time.Millisecond):
				}
			}

			var readiness struct {
				Status string            `json:"status"`
				Checks map[string]string `json:"checks"`
			}
			if err := json.Unmarshal(last, &readiness); err != nil || readiness.Status != "ok" {
				fail("social readiness answer = %s: %v", last, err)
			}
			for _, check := range []string{"database", "redis", "lifecycle", "social_runtime", "social_broker", "social_durable", "social_notification", "social_secrets"} {
				if readiness.Checks[check] != "ok" {
					fail("social readiness check %s = %s", check, last)
				}
			}
			public, err := health.Get("http://127.0.0.1:" + env["PORT"] + "/v2/pools/" + h.pool + "/comments")
			if err != nil {
				fail("plain social listener: %v", err)
			}
			public.Body.Close()
			if public.StatusCode != http.StatusNotFound {
				fail("plain listener admitted social API: %d", public.StatusCode)
			}
			metricResponse, err := health.Get("http://127.0.0.1:" + env["PORT"] + "/metrics")
			if err != nil {
				fail("social metrics: %v", err)
			}
			metricBody, err := io.ReadAll(metricResponse.Body)
			metricResponse.Body.Close()
			if err != nil || metricResponse.StatusCode != http.StatusOK || !bytes.Contains(metricBody, []byte(`service="sociald"`)) {
				fail("social metrics answer = %d: %s: %v", metricResponse.StatusCode, metricBody, err)
			}
			if boot == 0 {
				body := []byte(`{"body":"durable deployment comment","clientCommentId":"` + clientID + `"}`)
				req := mustRequest(t, http.MethodPost, "https://127.0.0.1:8443/v2/pools/"+h.pool+"/comments", body)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer deployment-social")
				response, err := client.Do(req)
				if err != nil {
					fail("create deployment comment: %v", err)
				}
				raw, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil || response.StatusCode != http.StatusCreated || v2.ValidateJSON("CommentCreateResponse", raw) != nil {
					fail("create deployment comment = %d: %s: %v", response.StatusCode, raw, err)
				}
				var created v2.CommentCreateResponse
				if err := json.Unmarshal(raw, &created); err != nil {
					t.Fatal(err)
				}
				commentID = created.Data.Comment.CommentId
			}
			response, err := client.Get("https://127.0.0.1:8443/v2/pools/" + h.pool + "/comments")
			if err != nil {
				fail("read deployment comment: %v", err)
			}
			raw, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != http.StatusOK || v2.ValidateJSON("CommentsListResponse", raw) != nil {
				fail("deployment comments after boot %d = %d: %s: %v", boot, response.StatusCode, raw, err)
			}
			var listed v2.CommentsListResponse
			if err := json.Unmarshal(raw, &listed); err != nil {
				fail("decode deployment comments: %v", err)
			}
			found := false
			for _, comment := range listed.Data.Items {
				if comment.CommentId == commentID && comment.Body == "durable deployment comment" {
					found = true
				}
			}
			if !found {
				fail("committed deployment comment missing after boot %d: %s", boot, raw)
			}
			if err := command.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				done <- err
				if err != nil {
					fail("social shutdown: %v", err)
				}
			case <-time.After(10 * time.Second):
				fail("social listeners did not drain")
			}
		}()
	}
	if notificationCalls.Load() == 0 {
		t.Fatal("committed chat notification was not delivered")
	}
}

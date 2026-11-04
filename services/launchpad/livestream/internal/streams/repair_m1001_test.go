package streams

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const repairM1001OperationID = "11111111-1111-4111-8111-111111111111"

// These frames were emitted by the pinned Studio serialization expression in
// testdata/studio-72187/producer.txt. Only the producer's input bindings change.
func repairM1001StudioBody(t *testing.T, id, provider, kind string, observed, delivered time.Time) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "studio-72187", kind+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var frame map[string]json.RawMessage
	if err = json.Unmarshal(raw, &frame); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]any{"id": id, "stream": map[string]string{"id": provider}, "createdAt": observed.UnixMilli(), "timestamp": delivered.UnixMilli()} {
		frame[key], err = json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	raw, err = json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func repairM1001StudioSignature(raw []byte, secret string, delivered time.Time) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	return fmt.Sprintf("t=%d,v1=%s", delivered.UnixMilli(), hex.EncodeToString(mac.Sum(nil)))
}

func repairM1001Normalize(t *testing.T, raw []byte, secret string, now time.Time) NormalizedProviderWebhook {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/webhooks/livepeer", bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Livepeer-Signature", repairM1001StudioSignature(raw, secret, now))
	rec := httptest.NewRecorder()
	wire, ok := NormalizeStudioWebhook(rec, request, secret, now)
	if !ok {
		t.Fatalf("production normalizer: %d %s", rec.Code, rec.Body.String())
	}
	if err := v2.ValidateJSON("LiveWebhookBody", wire.Body); err != nil {
		t.Fatal(err)
	}
	return wire
}

func repairM1001Post(t *testing.T, server *httptest.Server, path string, body []byte, headers map[string]string, status int) []byte {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	if err != nil || response.StatusCode != status {
		t.Fatalf("%s: status=%d want=%d body=%s error=%v", path, response.StatusCode, status, raw, err)
	}
	if status != http.StatusOK {
		if err = v2.ValidateJSON("Error", raw); err != nil {
			t.Fatalf("invalid refusal: %s %v", raw, err)
		}
	} else {
		if err = v2.ValidateJSON("LiveWebhookResponse", raw); err != nil {
			t.Fatalf("invalid response: %s %v", raw, err)
		}
		var result v2.LiveWebhookResponse
		if err = json.Unmarshal(raw, &result); err != nil || !result.Data.Accepted || result.Quality != "complete" {
			t.Fatalf("consumer rejected ack: %s %v", raw, err)
		}
	}
	return raw
}

func repairM1001AssertAliases(t *testing.T, handler http.Handler, secret string, now time.Time, lastOutcome *string) {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()
	raw := repairM1001StudioBody(t, "webhook_test_12", "streamid", "stream.started", now.Add(-time.Minute), now)
	wire := repairM1001Normalize(t, raw, secret, now)
	normalizedHeaders := func(timestamp, signature string) map[string]string {
		return map[string]string{NormalizedWebhookTimestampHeader: timestamp, NormalizedWebhookSignatureHeader: signature}
	}
	studioHeader := func(body []byte, at time.Time) map[string]string {
		return map[string]string{"Livepeer-Signature": repairM1001StudioSignature(body, secret, at)}
	}
	malformed := []byte(`{"timestamp":` + strconv.FormatInt(now.UnixMilli(), 10) + `,"event":`)
	malformedFrozen := []byte(`{"eventId":"webhook_test_12","streamId":"streamid","event":"unknown","occurredAt":"` + now.Format(v2LifecycleTimeLayout) + `"}`)
	duplicate := bytes.Replace(raw, []byte(`"id":"webhook_test_12"`), []byte(`"id":"webhook_test_12","id":"another_event"`), 1)
	nestedDuplicate := bytes.Replace(raw, []byte(`"stream":{"id":"streamid"}`), []byte(`"stream":{"id":"streamid","id":"other"}`), 1)
	expired := repairM1001StudioBody(t, "webhook_test_12", "streamid", "stream.started", now.Add(-time.Hour), now.Add(-301*time.Second))
	future := repairM1001StudioBody(t, "webhook_test_12", "streamid", "stream.started", now.Add(-time.Minute), now.Add(301*time.Second))
	seconds := bytes.Replace(raw, []byte(strconv.FormatInt(now.UnixMilli(), 10)), []byte(strconv.FormatInt(now.Unix(), 10)), 1)
	signature := repairM1001StudioSignature(raw, secret, now)
	_, mac, _ := strings.Cut(signature, ",v1=")
	*lastOutcome = ""
	for _, tc := range []struct {
		name, path string
		body       []byte
		headers    map[string]string
		status     int
	}{
		{"unsigned_studio", "/webhooks/livepeer", raw, nil, 401},
		{"wrong_signature", "/webhooks/livepeer", raw, map[string]string{"Livepeer-Signature": fmt.Sprintf("t=%d,v1=%s", now.UnixMilli(), strings.Repeat("0", 64))}, 401},
		{"tampered", "/webhooks/livepeer", bytes.Replace(raw, []byte("stream.started"), []byte("stream.idle"), 1), studioHeader(raw, now), 401},
		{"substituted_timestamp", "/webhooks/livepeer", raw, map[string]string{"Livepeer-Signature": fmt.Sprintf("t=%d,v1=%s", now.Add(time.Second).UnixMilli(), mac)}, 401},
		{"seconds_refused", "/webhooks/livepeer", seconds, map[string]string{"Livepeer-Signature": strings.Replace(repairM1001StudioSignature(seconds, secret, now), strconv.FormatInt(now.UnixMilli(), 10), strconv.FormatInt(now.Unix(), 10), 1)}, 401},
		{"stale", "/webhooks/livepeer", expired, studioHeader(expired, now.Add(-301*time.Second)), 401},
		{"future", "/webhooks/livepeer", future, studioHeader(future, now.Add(301*time.Second)), 401},
		{"malformed", "/webhooks/livepeer", malformed, studioHeader(malformed, now), 400},
		{"duplicate_key", "/webhooks/livepeer", duplicate, studioHeader(duplicate, now), 400},
		{"nested_duplicate_key", "/webhooks/livepeer", nestedDuplicate, studioHeader(nestedDuplicate, now), 400},
		{"frozen_unsigned", "/v2/provider-webhooks/livepeer", wire.Body, nil, 401},
		{"frozen_tampered", "/v2/provider-webhooks/livepeer", bytes.Replace(wire.Body, []byte(`"active"`), []byte(`"inactive"`), 1), normalizedHeaders(wire.Timestamp, wire.Signature), 401},
		{"frozen_substituted_timestamp", "/v2/provider-webhooks/livepeer", wire.Body, normalizedHeaders(strconv.FormatInt(now.Unix()+1, 10), wire.Signature), 401},
		{"frozen_stale", "/v2/provider-webhooks/livepeer", wire.Body, normalizedHeaders(strconv.FormatInt(now.Unix()-301, 10), sharedauth.SignWebhook(normalizedWebhookKey(secret), strconv.FormatInt(now.Unix()-301, 10), string(wire.Body))), 401},
		{"frozen_future", "/v2/provider-webhooks/livepeer", wire.Body, normalizedHeaders(strconv.FormatInt(now.Unix()+301, 10), sharedauth.SignWebhook(normalizedWebhookKey(secret), strconv.FormatInt(now.Unix()+301, 10), string(wire.Body))), 401},
		{"malformed_frozen", "/v2/provider-webhooks/livepeer", malformedFrozen, normalizedHeaders(wire.Timestamp, sharedauth.SignWebhook(normalizedWebhookKey(secret), wire.Timestamp, string(malformedFrozen))), 400},
		{"studio_signature_on_frozen", "/v2/provider-webhooks/livepeer", wire.Body, normalizedHeaders(wire.Timestamp, "sha256="+mac), 401},
		{"studio_body_on_frozen", "/v2/provider-webhooks/livepeer", raw, normalizedHeaders(wire.Timestamp, sharedauth.SignWebhook(normalizedWebhookKey(secret), wire.Timestamp, string(raw))), 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repairM1001Post(t, server, tc.path, tc.body, tc.headers, tc.status)
			if *lastOutcome != "" {
				t.Fatalf("refused delivery reached observer: %s", *lastOutcome)
			}
		})
	}
	repairM1001Post(t, server, "/webhooks/livepeer", raw, studioHeader(raw, now), 200)
	if *lastOutcome != "applied" {
		t.Fatalf("producer callback outcome=%s", *lastOutcome)
	}
	repairM1001Post(t, server, "/v2/provider-webhooks/livepeer", wire.Body, normalizedHeaders(wire.Timestamp, wire.Signature), 200)
	if *lastOutcome != "duplicate" {
		t.Fatalf("frozen redelivery outcome=%s", *lastOutcome)
	}
	repairM1001Post(t, server, "/webhooks/livepeer", raw, studioHeader(raw, now), 200)
	if *lastOutcome != "duplicate" {
		t.Fatalf("Studio redelivery outcome=%s", *lastOutcome)
	}
}

// A committed inbox peer gives the HTTP adapter durable observable effects. The
// PostgreSQL lifecycle assertions remain in the existing integration proofs.
type repairM1001Inbox struct {
	path      string
	mu        sync.Mutex
	operation Operation
	events    map[string]string
}

func repairM1001OpenInbox(t *testing.T, path string) *repairM1001Inbox {
	t.Helper()
	peer := &repairM1001Inbox{path: path, operation: Operation{ID: repairM1001OperationID, ProviderID: "streamid", State: "READY", Revision: 1}, events: map[string]string{}}
	raw, err := os.ReadFile(path)
	if err == nil {
		var state struct {
			Operation Operation
			Events    map[string]string
		}
		if err = json.Unmarshal(raw, &state); err != nil {
			t.Fatal(err)
		}
		peer.operation, peer.events = state.Operation, state.Events
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return peer
}

func (p *repairM1001Inbox) Observe(ctx context.Context, event ProviderEvent) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	fingerprint := digest(event)
	if prior, ok := p.events[event.EventID]; ok {
		if prior != fingerprint {
			return "", ErrConflict
		}
		return "duplicate", nil
	}
	if event.ProviderID != p.operation.ProviderID {
		return "", ErrConflict
	}
	operation := p.operation
	operation.State, operation.ProviderTime = event.State, &event.ObservedAt
	operation.Revision++
	events := make(map[string]string, len(p.events)+1)
	for k, v := range p.events {
		events[k] = v
	}
	events[event.EventID] = fingerprint
	raw, err := json.Marshal(struct {
		Operation Operation
		Events    map[string]string
	}{operation, events})
	if err != nil {
		return "", err
	}
	if err = repairM1001WriteSnapshot(p.path, raw); err != nil {
		return "", err
	}
	p.operation, p.events = operation, events
	return "applied", nil
}

func repairM1001WriteSnapshot(path string, raw []byte) error {
	file, err := os.OpenFile(path+".next", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(path+".next", path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func repairM1001AdapterRouter(peer ProviderEventObserver, secret string, now time.Time, outcome *string) http.Handler {
	adapter := ProviderWebhookAdapter{Observer: peer, Secret: secret, Clock: func() time.Time { return now }, ObserveOutcome: func(value string) { *outcome = value }}
	router := chi.NewRouter()
	router.Post("/webhooks/livepeer", adapter.Studio)
	router.Post("/v2/provider-webhooks/livepeer", adapter.Normalized)
	return router
}

func TestRepairM_1001_006_StudioAliases(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	peer := repairM1001OpenInbox(t, filepath.Join(t.TempDir(), "inbox.json"))
	var outcome string
	repairM1001AssertAliases(t, repairM1001AdapterRouter(peer, strings.Repeat("k", 32), now, &outcome), strings.Repeat("k", 32), now, &outcome)
	if len(peer.events) != 1 || peer.operation.State != "LIVE" || peer.operation.Revision != 2 {
		t.Fatalf("alias effects: %+v %v", peer.operation, peer.events)
	}
}

func TestRepairM_1001_007_StudioRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.json")
	now := time.Now().UTC().Truncate(time.Millisecond)
	secret := strings.Repeat("k", 32)
	peer := repairM1001OpenInbox(t, path)
	var outcome string
	server := httptest.NewServer(repairM1001AdapterRouter(peer, secret, now, &outcome))
	raw := repairM1001StudioBody(t, "webhook_test_12", "streamid", "stream.started", now.Add(-time.Minute), now)
	headers := map[string]string{"Livepeer-Signature": repairM1001StudioSignature(raw, secret, now)}
	repairM1001Post(t, server, "/webhooks/livepeer", raw, map[string]string{"Livepeer-Signature": repairM1001StudioSignature(raw, strings.Repeat("x", 32), now)}, 401)
	if len(peer.events) != 0 || peer.operation.Revision != 1 {
		t.Fatal("HMAC refusal mutated durable peer")
	}
	repairM1001Post(t, server, "/webhooks/livepeer", raw, headers, 200)
	if outcome != "applied" {
		t.Fatal(outcome)
	}
	server.Close()
	peer = nil
	reopened := repairM1001OpenInbox(t, path)
	server = httptest.NewServer(repairM1001AdapterRouter(reopened, secret, now, &outcome))
	defer server.Close()
	repairM1001Post(t, server, "/webhooks/livepeer", raw, headers, 200)
	if outcome != "duplicate" || len(reopened.events) != 1 || reopened.operation.Revision != 2 || reopened.operation.ProviderTime == nil || !reopened.operation.ProviderTime.Equal(now.Add(-time.Minute)) {
		t.Fatalf("reopened delivery: %s %+v", outcome, reopened.operation)
	}
	idle := repairM1001StudioBody(t, "webhook_test_42", "streamid", "stream.idle", now, now)
	repairM1001Post(t, server, "/webhooks/livepeer", idle, map[string]string{"Livepeer-Signature": repairM1001StudioSignature(idle, secret, now)}, 200)
	if outcome != "applied" || reopened.operation.State != "ENDED" || reopened.operation.Revision != 3 {
		t.Fatalf("idle producer not consumed: %s %+v", outcome, reopened.operation)
	}
}

// The resolver's repository peer atomically checkpoints the shared production
// transition and its receipt, so reopening discards all component memory.
type repairM1001RecoveryPeer struct {
	mu       sync.Mutex
	path     string
	state    ProvisionRecoveryState
	owner    string
	receipts []ProvisionResolutionReceipt
}

func repairM1001OpenRecovery(t *testing.T, path string) *repairM1001RecoveryPeer {
	t.Helper()
	peer := &repairM1001RecoveryPeer{path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		State    ProvisionRecoveryState
		Receipts []ProvisionResolutionReceipt
		Owner    string
	}
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	peer.state, peer.receipts, peer.owner = saved.State, saved.Receipts, saved.Owner
	return peer
}

func repairM1001NewRecovery(t *testing.T, state ProvisionRecoveryState) *repairM1001RecoveryPeer {
	t.Helper()
	path := filepath.Join(t.TempDir(), "recovery.json")
	raw, err := json.Marshal(struct {
		State    ProvisionRecoveryState
		Receipts []ProvisionResolutionReceipt
	}{State: state})
	if err != nil {
		t.Fatal(err)
	}
	if err = repairM1001WriteSnapshot(path, raw); err != nil {
		t.Fatal(err)
	}
	return repairM1001OpenRecovery(t, path)
}

func (p *repairM1001RecoveryPeer) ProvisionRecovery(ctx context.Context, id string) (ProvisionRecoveryState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return ProvisionRecoveryState{}, err
	}
	if id != p.state.Operation.ID {
		return ProvisionRecoveryState{}, ErrInvalid
	}
	return p.state, nil
}

func (p *repairM1001RecoveryPeer) applyProvisionResolution(ctx context.Context, d ProvisionResolutionDecision, receipt ProvisionResolutionReceipt) (Operation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Operation{}, err
	}
	next, err := resolvedProvisionState(p.state, d, time.Now())
	if err != nil {
		return Operation{}, err
	}
	receipts := append(append([]ProvisionResolutionReceipt{}, p.receipts...), receipt)
	raw, err := json.Marshal(struct {
		State    ProvisionRecoveryState
		Receipts []ProvisionResolutionReceipt
		Owner    string
	}{State: next, Receipts: receipts})
	if err != nil {
		return Operation{}, err
	}
	if err = repairM1001WriteSnapshot(p.path, raw); err != nil {
		return Operation{}, err
	}
	p.state, p.receipts, p.owner = next, receipts, ""
	return next.Operation, nil
}

func (p *repairM1001RecoveryPeer) save() error {
	raw, err := json.Marshal(struct {
		State    ProvisionRecoveryState
		Receipts []ProvisionResolutionReceipt
		Owner    string
	}{p.state, p.receipts, p.owner})
	if err != nil {
		return err
	}
	return repairM1001WriteSnapshot(p.path, raw)
}

func (p *repairM1001RecoveryPeer) claim(owner string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state.LeaseActive {
		return ErrConflict
	}
	p.owner = owner
	p.state.LeaseActive = true
	p.state.Attempt++
	return p.save()
}

func (p *repairM1001RecoveryPeer) release() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.owner = ""
	p.state.LeaseActive = false
	return p.save()
}

func (p *repairM1001RecoveryPeer) reserveProviderCreate(ctx context.Context, id, owner string, revision int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.state.Operation.ID != id || p.state.Operation.Revision != revision || p.state.Operation.State != "PROVISIONING" || !p.state.LeaseActive || p.owner != owner || p.state.CreateSent {
		return ErrUnavailable
	}
	p.state.CreateSent = true
	return p.save()
}

func (p *repairM1001RecoveryPeer) latestProvisionResolution(ctx context.Context, id string) (*ProvisionResolutionReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if id != p.state.Operation.ID {
		return nil, ErrInvalid
	}
	return p.state.Resolution, nil
}

func repairM1001AmbiguousState() ProvisionRecoveryState {
	return ProvisionRecoveryState{Operation: Operation{ID: repairM1001OperationID, Wallet: "0x" + strings.Repeat("1", 40), Pool: "0x" + strings.Repeat("2", 40), State: "PROVISIONING", Intent: "create", Revision: 2}, Attempt: 1, CreateSent: true, LastReason: "provider_create_ambiguous"}
}

func repairM1001Decision(state ProvisionRecoveryState, action ProvisionResolutionAction) ProvisionResolutionDecision {
	return ProvisionResolutionDecision{OperationID: state.Operation.ID, Revision: state.Operation.Revision, Attempt: state.Attempt, Action: action, Actor: "stream_operator", Allowed: true, NonallocationConfirmed: action != ProvisionAdopt, ConfirmationReference: "provider_receipt_123", ConfirmedAt: time.Now().UTC().Add(-time.Second), ExpiresAt: time.Now().Add(time.Minute)}
}

func repairM1001Provider(t *testing.T, serve http.HandlerFunc) (*livepeer.Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer repair-fixture-key" {
			t.Error("provider authorization missing")
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		serve(w, r)
	}))
	t.Cleanup(server.Close)
	return livepeer.New("repair-fixture-key", server.URL, nil, livepeer.WithHTTPClient(server.Client())), server
}

func TestRepairM_1001_008_CrashBeforeSendResume(t *testing.T) {
	state := repairM1001AmbiguousState()
	peer := repairM1001NewRecovery(t, state)
	path := peer.path
	peer = nil // crash after the durable send reservation, before an HTTP create
	peer = repairM1001OpenRecovery(t, path)
	if !reflect.DeepEqual(peer.state.Actions(), []ProvisionResolutionAction{ProvisionResume, ProvisionRelease, ProvisionAdopt}) {
		t.Fatal("no actionable recovery", peer.state)
	}
	var creates int
	provider, _ := repairM1001Provider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			creates++
			_, _ = fmt.Fprintf(w, `{"id":"resumed_stream","name":%q,"streamKey":"provider_secret","playbackId":"playback_id"}`, livepeer.CorrelationName(state.Operation.ID))
			return
		}
		if r.URL.RequestURI() != "/stream?streamsonly=true" {
			t.Error("unexpected discovery route", r.URL)
			http.Error(w, "bad route", 400)
			return
		}
		_, _ = io.WriteString(w, `[]`)
	})
	allocation := provisionAllocation{Store: peer, Provider: provider}
	if _, err := allocation.resolve(context.Background(), state.Operation, "crashed_worker"); !errors.Is(err, errProvisionAmbiguous) || creates != 0 {
		t.Fatalf("crash checkpoint caused blind create: %v creates=%d", err, creates)
	}
	resolver := &ProvisionAmbiguityResolver{Store: peer, Provider: provider}
	d := repairM1001Decision(state, ProvisionResume)
	unconfirmed := d
	unconfirmed.NonallocationConfirmed = false
	if _, err := resolver.Resolve(context.Background(), unconfirmed); !errors.Is(err, ErrDenied) {
		t.Fatalf("empty listing re-armed create without confirmation: %v", err)
	}
	o, err := resolver.Resolve(context.Background(), d)
	if err != nil || o.Revision != state.Operation.Revision+1 || o.State != "PROVISIONING" || peer.state.CreateSent || creates != 0 {
		t.Fatalf("confirmed resume: %+v sent=%v creates=%d error=%v", o, peer.state.CreateSent, creates, err)
	}
	resolver = nil
	peer = nil
	reopened := repairM1001OpenRecovery(t, path)
	if reopened.state.CreateSent || reopened.state.LastReason != "provider_nonallocation_resume" || len(reopened.receipts) != 1 || reopened.receipts[0].ConfirmationReference != d.ConfirmationReference {
		t.Fatalf("resume lost on restart: %+v %+v", reopened.state, reopened.receipts)
	}
	resolver = &ProvisionAmbiguityResolver{Store: reopened, Provider: provider}
	if _, err = resolver.Resolve(context.Background(), d); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale resolution replay admitted: %v", err)
	}
	if creates != 0 {
		t.Fatal("resolution retried create", creates)
	}
	if err = reopened.claim("resumed_worker"); err != nil {
		t.Fatal(err)
	}
	allocation.Store = reopened
	remote, err := allocation.resolve(context.Background(), reopened.state.Operation, "resumed_worker")
	if err != nil || remote.ID != "resumed_stream" || creates != 1 {
		t.Fatalf("explicitly resumed worker: %+v creates=%d %v", remote, creates, err)
	}
	// Discovery is still absent in this transcript. The consumed reservation
	// must prevent a second create, including after an explicitly resumed send.
	if _, err = allocation.resolve(context.Background(), reopened.state.Operation, "resumed_worker"); !errors.Is(err, errProvisionAmbiguous) || creates != 1 {
		t.Fatalf("blind create after resumed send: %v creates=%d", err, creates)
	}
}

func TestRepairM_1001_008_RejectionRelease(t *testing.T) {
	state := repairM1001AmbiguousState()
	peer := repairM1001NewRecovery(t, state)
	var creates int
	provider, _ := repairM1001Provider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			creates++
			http.Error(w, `{"error":"quota exceeded"}`, http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, `[]`)
	})
	// A definite rejection crosses the worker's actual allocation component.
	peer.state.CreateSent = false
	if err := peer.claim("rejected_worker"); err != nil {
		t.Fatal(err)
	}
	allocation := provisionAllocation{Store: peer, Provider: provider}
	if _, err := allocation.resolve(context.Background(), state.Operation, "rejected_worker"); !errors.Is(err, errProvisionAmbiguous) {
		t.Fatalf("provider rejection not retained as ambiguous: %v", err)
	}
	if err := peer.release(); err != nil {
		t.Fatal(err)
	}
	state = peer.state
	resolver := &ProvisionAmbiguityResolver{Store: peer, Provider: provider}
	d := repairM1001Decision(state, ProvisionRelease)
	o, err := resolver.Resolve(context.Background(), d)
	if err != nil || o.State != "FAILED" || o.Revision != 3 || creates != 1 {
		t.Fatalf("rejected allocation release: %+v creates=%d %v", o, creates, err)
	}
	peer = repairM1001OpenRecovery(t, peer.path)
	if peer.state.Operation.State != "FAILED" || len(peer.state.Actions()) != 0 || len(peer.receipts) != 1 || peer.receipts[0].Action != ProvisionRelease {
		t.Fatalf("terminal release lost: %+v %+v", peer.state, peer.receipts)
	}
	// Ending and suspended operations release to ENDED without resurrecting them.
	for _, terminalIntent := range []string{"ENDING", "SUSPENDED"} {
		state.Operation.State = terminalIntent
		repository := repairM1001NewRecovery(t, state)
		resolver.Store = repository
		d = repairM1001Decision(state, ProvisionRelease)
		o, err = resolver.Resolve(context.Background(), d)
		if err != nil || o.State != "ENDED" || creates != 1 {
			t.Fatalf("%s release: %+v %v", terminalIntent, o, err)
		}
	}
}

func TestRepairM_1001_008_VerifiedAdoptionRestart(t *testing.T) {
	state := repairM1001AmbiguousState()
	peer := repairM1001NewRecovery(t, state)
	var creates int
	allocationID := "allocated_stream"
	provider, _ := repairM1001Provider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			creates++
			http.Error(w, "create forbidden", 500)
			return
		}
		_, _ = fmt.Fprintf(w, `[{"id":%q,"name":%q,"streamKey":"provider_secret","playbackId":"playback_id"}]`, allocationID, livepeer.CorrelationName(state.Operation.ID))
	})
	resolver := &ProvisionAmbiguityResolver{Store: peer, Provider: provider}
	absent := repairM1001Decision(state, ProvisionRelease)
	if _, err := resolver.Resolve(context.Background(), absent); !errors.Is(err, ErrConflict) {
		t.Fatalf("existing allocation released admission: %v", err)
	}
	d := repairM1001Decision(state, ProvisionAdopt)
	d.ProviderID = "other_stream"
	if _, err := resolver.Resolve(context.Background(), d); !errors.Is(err, ErrConflict) {
		t.Fatalf("wrong allocation adopted: %v", err)
	}
	d.ProviderID = "allocated_stream"
	o, err := resolver.Resolve(context.Background(), d)
	if err != nil || o.State != "PROVISIONING" || o.ProviderID != "" || o.Revision != 3 || !peer.state.CreateSent || creates != 0 {
		t.Fatalf("adoption did not resume policy/encryption worker safely: %+v %+v %v", o, peer.state, err)
	}
	path := peer.path
	resolver = nil
	peer = nil
	peer = repairM1001OpenRecovery(t, path)
	if len(peer.receipts) != 1 || peer.receipts[0].ProviderID != d.ProviderID || peer.state.LastReason != "provider_adoption_verified" || !peer.state.CreateSent {
		t.Fatalf("adoption evidence lost: %+v %+v", peer.state, peer.receipts)
	}
	raw, err := os.ReadFile(path)
	if err != nil || bytes.Contains(raw, []byte("provider_secret")) {
		t.Fatal("provider credentials retained in resolution evidence", err)
	}
	// The worker's normal correlation lookup can rediscover this allocation after
	// restart; the create reservation remains consumed throughout adoption.
	if err = peer.claim("adoption_worker"); err != nil {
		t.Fatal(err)
	}
	allocation := provisionAllocation{Store: peer, Provider: provider}
	remote, err := allocation.resolve(context.Background(), peer.state.Operation, "adoption_worker")
	if err != nil || remote.ID != d.ProviderID || creates != 0 {
		t.Fatalf("reopened worker discovery: %+v %v creates=%d", remote, err, creates)
	}
	allocationID = "replacement_stream"
	if _, err = allocation.resolve(context.Background(), peer.state.Operation, "adoption_worker"); !errors.Is(err, ErrConflict) || creates != 0 {
		t.Fatalf("worker discarded adoption identity: %v creates=%d", err, creates)
	}
}

func TestRepairM_1001_008_RevisionLeaseAndProviderRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ProvisionRecoveryState, *ProvisionResolutionDecision)
		want   error
	}{
		{"stale_revision", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) { d.Revision-- }, ErrConflict},
		{"stale_attempt", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) { d.Attempt++ }, ErrConflict},
		{"active_lease", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) { s.LeaseActive = true }, ErrConflict},
		{"denied", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) { d.Allowed = false }, ErrDenied},
		{"expired", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) {
			d.ExpiresAt = time.Now().Add(-time.Second)
		}, ErrDenied},
		{"old_evidence", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) {
			d.ConfirmedAt = time.Now().Add(-6 * time.Minute)
		}, ErrDenied},
		{"future_evidence", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) {
			d.ConfirmedAt = time.Now().Add(time.Minute)
		}, ErrDenied},
		{"missing_receipt", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) { d.ConfirmationReference = "" }, ErrDenied},
		{"already_bound", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) { s.Operation.ProviderID = "existing" }, ErrConflict},
		{"end_cannot_resume", func(s *ProvisionRecoveryState, d *ProvisionResolutionDecision) { s.Operation.State = "ENDING" }, ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := repairM1001AmbiguousState()
			d := repairM1001Decision(state, ProvisionResume)
			tc.change(&state, &d)
			peer := repairM1001NewRecovery(t, state)
			provider, _ := repairM1001Provider(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("refused authority reached provider")
				_, _ = io.WriteString(w, `[]`)
			})
			resolver := ProvisionAmbiguityResolver{Store: peer, Provider: provider}
			if _, err := resolver.Resolve(context.Background(), d); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			reopened := repairM1001OpenRecovery(t, peer.path)
			if !reflect.DeepEqual(state, reopened.state) || len(reopened.receipts) != 0 {
				t.Fatal("refused resolution changed durable state")
			}
		})
	}
	for _, body := range []string{`null`, `[{"id":"one","playbackId":"pb","streamKey":"key","name":"KindleLaunch:` + repairM1001OperationID + `"},{"id":"two","playbackId":"pb","streamKey":"key","name":"KindleLaunch:` + repairM1001OperationID + `"}]`, `not-json`} {
		t.Run(body, func(t *testing.T) {
			state := repairM1001AmbiguousState()
			peer := repairM1001NewRecovery(t, state)
			provider, _ := repairM1001Provider(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) })
			resolver := ProvisionAmbiguityResolver{Store: peer, Provider: provider}
			if _, err := resolver.Resolve(context.Background(), repairM1001Decision(state, ProvisionResume)); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("ambiguous provider evidence accepted: %v", err)
			}
			if !reflect.DeepEqual(state, peer.state) || len(peer.receipts) != 0 {
				t.Fatal("provider refusal changed state")
			}
		})
	}
}

func TestRepairM_1001_008_LeaseAcquiredDuringDiscovery(t *testing.T) {
	state := repairM1001AmbiguousState()
	peer := repairM1001NewRecovery(t, state)
	provider, _ := repairM1001Provider(t, func(w http.ResponseWriter, r *http.Request) {
		if err := peer.claim("competing_worker"); err != nil {
			t.Error(err)
		}
		_, _ = io.WriteString(w, `[]`)
	})
	resolver := ProvisionAmbiguityResolver{Store: peer, Provider: provider}
	if _, err := resolver.Resolve(context.Background(), repairM1001Decision(state, ProvisionResume)); !errors.Is(err, ErrConflict) {
		t.Fatalf("lost lease race overwrote worker: %v", err)
	}
	if !peer.state.CreateSent || len(peer.receipts) != 0 {
		t.Fatal("racing resolution re-armed create")
	}
	reopened := repairM1001OpenRecovery(t, peer.path)
	if !reopened.state.LeaseActive || !reopened.state.CreateSent || reopened.state.Attempt != state.Attempt+1 {
		t.Fatal("lease race lost after reopening", reopened.state)
	}
}

func repairM1001Budget() BudgetPolicy {
	return BudgetPolicy{VendorUserID: "studio_user", DeliveryPricePerMinute: "0.25", TranscodePricePerMinute: "0.5", StoragePricePerMinute: "0.125", MonthlyUnits: "10.25", BroadcastReserveUnits: "0.25", DeliveryPricePerTB: "7.25", Currency: "USD", PeakDurationSeconds: 60, ApprovalReference: "approved_policy"}
}

func repairM1001Usage(value string) UsageRecord {
	return UsageRecord{ProviderEvent: "usage_receipt", OperationID: repairM1001OperationID, ViewerMinutes: value, TranscodeMinutes: value, StorageBytes: value, DeliveryBytes: value, CostUnits: value}
}

func TestRepairM_1001_016_DecimalNumericRoundTrip(t *testing.T) {
	codec := pgtype.NewMap()
	for _, value := range []string{"0", "1", "0.25", "10.2500", "0.000000000000000001", strings.Repeat("9", usageIntegerDigits) + "." + strings.Repeat("9", usageFractionDigits)} {
		t.Run(value, func(t *testing.T) {
			if err := repairM1001Usage(value).Validate(); err != nil {
				t.Fatal("valid usage refused", err)
			}
			rational, err := parseUsageDecimal(value)
			if err != nil {
				t.Fatal(err)
			}
			var numeric pgtype.Numeric
			if err = numeric.Scan(value); err != nil {
				t.Fatal("PostgreSQL numeric rejected accepted usage", err)
			}
			encoded, err := codec.Encode(pgtype.NumericOID, pgtype.BinaryFormatCode, numeric, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) < 8 || int(binary.BigEndian.Uint16(encoded[6:8])) > usageFractionDigits {
				t.Fatal("unsupported numeric scale", encoded)
			}
			var decoded pgtype.Numeric
			if err = codec.Scan(pgtype.NumericOID, pgtype.BinaryFormatCode, encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			text, err := decoded.Value()
			if err != nil {
				t.Fatal(err)
			}
			result, ok := new(big.Rat).SetString(text.(string))
			if !ok || result.Cmp(rational) != 0 {
				t.Fatalf("numeric round trip changed units: %q -> %v", value, text)
			}
			policy := repairM1001Budget()
			policy.DeliveryPricePerMinute = value
			policy.TranscodePricePerMinute = value
			policy.StoragePricePerMinute = value
			if rational.Sign() > 0 {
				policy.MonthlyUnits = value
				policy.BroadcastReserveUnits = value
				policy.DeliveryPricePerTB = value
			}
			if err = policy.ValidateVendor(); err != nil {
				t.Fatal("shared policy representation rejected usage decimal", err)
			}
		})
	}
	for _, tc := range []struct {
		spent, budget string
		want          []int
	}{
		{"5.124999999999999999", "10.25", []int{}},
		{"5.125", "10.25", []int{50}},
		{"8.2", "10.25", []int{50, 80}},
		{"10.25", "10.25", []int{50, 80, 100}},
	} {
		actual, err := BudgetThresholds(tc.spent, tc.budget)
		if err != nil || !reflect.DeepEqual(actual, tc.want) {
			t.Fatalf("decimal alert boundary spent=%s budget=%s got=%v want=%v error=%v", tc.spent, tc.budget, actual, tc.want, err)
		}
	}
	exactCost, err := calculateUsageCost([]string{"0.5", "0.25", "1"}, []string{"0.25", "0.5", "0.125"})
	if err != nil || exactCost != "0.375000000000000000" {
		t.Fatalf("vendor decimal cost changed units: %s %v", exactCost, err)
	}
	if _, err = calculateUsageCost([]string{"0.000000000000000001", "0", "0"}, []string{"0.1", "0", "0"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("vendor cost silently rounded beyond supported scale", err)
	}

	estimate, err := EstimateSpend(SpendScenario{Viewers: 20000, BitrateBitsPerSecond: 2000000, DurationSeconds: 3600, DeliveryPricePerTB: "7.25"})
	if err != nil || estimate.DeliveryCost != "130.500000" {
		t.Fatal("shared decimal spend estimate", estimate, err)
	}
}

func TestRepairM_1001_016_RejectNonDecimalBeforeStorage(t *testing.T) {
	invalid := []string{"1/2", "2/1", "0/1", "1e3", "1E-3", "1e-999999999", "1e999999999", "0x10", "+1", "-0", "-1", ".5", "1.", "01", "00.1", " 1", "1 ", "NaN", "Infinity", "", strings.Repeat("9", usageIntegerDigits+1), "0." + strings.Repeat("1", usageFractionDigits+1), strings.Repeat("0", 100000)}
	for i, value := range invalid {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if _, err := parseUsageDecimal(value); !errors.Is(err, ErrInvalid) {
				t.Fatalf("invalid decimal accepted: %.120q %v", value, err)
			}
			record := repairM1001Usage("0.25")
			fields := []*string{&record.ViewerMinutes, &record.TranscodeMinutes, &record.StorageBytes, &record.DeliveryBytes, &record.CostUnits}
			for _, field := range fields {
				*field = value
				// A nil database is deliberate: validation must refuse the string before
				// the production ingestion path attempts any persistence.
				if err := new(LifecycleStore).RecordUsage(context.Background(), record); !errors.Is(err, ErrInvalid) {
					t.Fatalf("ingestion accepted invalid numeric: %.120q %v", value, err)
				}
				*field = "0.25"
			}
			for field := 0; field < 6; field++ {
				policy := repairM1001Budget()
				amounts := []*string{&policy.MonthlyUnits, &policy.BroadcastReserveUnits, &policy.DeliveryPricePerTB, &policy.DeliveryPricePerMinute, &policy.TranscodePricePerMinute, &policy.StoragePricePerMinute}
				*amounts[field] = value
				if err := policy.ValidateVendor(); !errors.Is(err, ErrInvalid) {
					t.Fatalf("policy accepted invalid numeric: %.120q %v", value, err)
				}
				if field < 3 {
					if err := (&LifecycleStore{Budget: &policy}).EmitBudgetAlerts(context.Background()); !errors.Is(err, ErrInvalid) {
						t.Fatalf("alert path accepted invalid numeric: %.120q %v", value, err)
					}
				}
			}
			if _, err := BudgetThresholds(value, "10"); !errors.Is(err, ErrInvalid) {
				t.Fatal("threshold accepted invalid spent", err)
			}
			if _, err := BudgetThresholds("1", value); !errors.Is(err, ErrInvalid) {
				t.Fatal("threshold accepted invalid budget", err)
			}
			if _, err := EstimateSpend(SpendScenario{Viewers: 1, BitrateBitsPerSecond: 1, DurationSeconds: 1, DeliveryPricePerTB: value}); !errors.Is(err, ErrInvalid) {
				t.Fatal("estimate accepted invalid price", err)
			}
		})
	}
	policy := repairM1001Budget()
	policy.MonthlyUnits = "0"
	if !errors.Is(policy.Validate(), ErrInvalid) {
		t.Fatal("zero budget admitted")
	}
	if _, err := BudgetThresholds("0", "0"); !errors.Is(err, ErrInvalid) {
		t.Fatal("zero alert budget admitted", err)
	}
}

func TestRepairM_1001_006_PinnedProducerFrames(t *testing.T) {
	for _, frame := range []struct{ kind, hash, state string }{
		{"stream.started", "f7ea5b5a9c0ae4a74273801d407eb9f810a4b5a798c789d9b94a8e6327da6883", "LIVE"},
		{"stream.idle", "3225d15f23fe929d833ce87f57c29d4803e3d7ff7b064d8a1110b03a26b86deb", "ENDED"},
	} {
		raw, err := os.ReadFile(filepath.Join("testdata", "studio-72187", frame.kind+".json"))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != frame.hash {
			t.Fatal("pinned producer frame changed", frame.kind)
		}
		event, err := DecodeStudioWebhook(raw)
		if err != nil || event.State != frame.state || event.ProviderID != "streamid" || !event.ObservedAt.Equal(time.UnixMilli(1793780000000)) {
			t.Fatalf("producer decoder mismatch: %+v %v", event, err)
		}
		now := time.UnixMilli(1793780060000).UTC()
		wire := repairM1001Normalize(t, raw, strings.Repeat("k", 32), now)
		request := httptest.NewRequest(http.MethodPost, "/v2/provider-webhooks/livepeer", bytes.NewReader(wire.Body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(NormalizedWebhookTimestampHeader, wire.Timestamp)
		request.Header.Set(NormalizedWebhookSignatureHeader, wire.Signature)
		decoded, ok := VerifyNormalizedProviderWebhook(httptest.NewRecorder(), request, strings.Repeat("k", 32), now)
		if !ok || !reflect.DeepEqual(decoded, event) {
			t.Fatalf("producer -> normalized consumer changed identity/time/state: %+v %+v %t", event, decoded, ok)
		}
	}
}

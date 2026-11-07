package telemetry_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
)

var (
	repairInvalid    = errors.New("invalid account")
	repairOutboxFull = errors.New("outbox capacity exceeded")
	repairMissingAck = errors.New("delivery acknowledgement missing")
	repairPolicy     = delivery.Policy{
		Subjects:   map[string]bool{"account.changed": true},
		EventTypes: map[string]bool{"account.changed": true},
	}
)

// This fixture stores authoritative profiles and their production-encoded outbox
// bytes atomically. Capacity refusal leaves both collections unchanged.
type repairStore struct {
	mu             sync.Mutex
	profiles       map[string]string
	outbox         map[string][]byte
	capacity, used int
}

func (s *repairStore) transact(ctx context.Context, account string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, exists := s.profiles[account]; exists {
		return errors.New("account already exists")
	}
	payload, err := json.Marshal(map[string]string{"accountId": account, "revision": "1"})
	if err != nil {
		return err
	}
	envelope := delivery.Envelope{
		EventID: "mutation_" + account, EventType: "account.changed", SchemaVersion: 1,
		ChainID: 125, SourceOrder: delivery.SourceOrder{Height: "1", TransactionIndex: "0", LogIndex: "0"},
		EmittedAt: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC), TraceID: "repair_m1013",
		Payload: payload, PayloadHash: delivery.Hash(payload),
	}
	wire, err := envelope.Encode(repairPolicy, "account.changed")
	if err != nil {
		return err
	}
	if len(wire) > s.capacity-s.used {
		return repairOutboxFull
	}
	s.profiles[account] = "1"
	s.outbox[account] = append([]byte(nil), wire...)
	s.used += len(wire)
	return nil
}

func (s *repairStore) snapshot(account string) (string, []byte, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.profiles[account], append([]byte(nil), s.outbox[account]...), len(s.profiles), len(s.outbox)
}

type repairFixture struct {
	metrics            *telemetry.Metrics
	store              *repairStore
	consumer, exporter *httptest.Server
	mu                 sync.Mutex
	received           map[string]delivery.Envelope
}

func newRepairFixture(t *testing.T, capacity int, ackAvailable bool) *repairFixture {
	t.Helper()
	m, err := telemetry.New("userd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	f := &repairFixture{
		metrics:  m,
		store:    &repairStore{profiles: make(map[string]string), outbox: make(map[string][]byte), capacity: capacity},
		received: make(map[string]delivery.Envelope),
	}
	f.consumer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/events" {
			http.Error(w, "invalid event request", http.StatusBadRequest)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, delivery.MaxEnvelopeBytes+1))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		event, err := delivery.Decode(raw, repairPolicy, "account.changed")
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		f.mu.Lock()
		f.received[event.EventID] = event
		f.mu.Unlock()
		if !ackAvailable {
			http.Error(w, "acknowledgement unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	f.exporter = httptest.NewServer(m)
	t.Cleanup(f.consumer.Close)
	t.Cleanup(f.exporter.Close)
	return f
}

func (f *repairFixture) run(ctx context.Context, account string) error {
	return f.metrics.WithMutation(ctx, "account_mutation", func(ctx context.Context) error {
		if account == "" {
			return repairInvalid
		}
		return ctx.Err()
	}, func(ctx context.Context) error {
		return f.store.transact(ctx, account)
	}, func(ctx context.Context) error {
		return f.acknowledge(ctx, account)
	})
}

func (f *repairFixture) acknowledge(ctx context.Context, account string) error {
	_, wire, _, _ := f.store.snapshot(account)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.consumer.URL+"/events", bytes.NewReader(wire))
	if err != nil {
		return err
	}
	response, err := f.consumer.Client().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return repairMissingAck
	}
	return nil
}

func (f *repairFixture) assertState(t *testing.T, account string, committed, delivered bool) {
	t.Helper()
	revision, wire, profiles, outbox := f.store.snapshot(account)
	f.mu.Lock()
	event, received := f.received["mutation_"+account]
	f.mu.Unlock()
	if received != delivered {
		t.Fatalf("delivered = %v, want %v", received, delivered)
	}
	if !committed {
		if revision != "" || len(wire) != 0 || profiles != 0 || outbox != 0 {
			t.Fatalf("rollback left authoritative/outbox state: %q %q %d %d", revision, wire, profiles, outbox)
		}
		return
	}
	if revision != "1" || len(wire) == 0 || profiles != 1 || outbox != 1 {
		t.Fatalf("committed state: %q %q %d %d", revision, wire, profiles, outbox)
	}
	decoded, err := delivery.Decode(wire, repairPolicy, "account.changed")
	if err != nil {
		t.Fatal(err)
	}
	if delivered && (event.EventID != decoded.EventID || event.PayloadHash != decoded.PayloadHash || !bytes.Equal(event.Payload, decoded.Payload)) {
		t.Fatalf("consumer did not receive committed outbox: %+v != %+v", event, decoded)
	}
	var payload map[string]string
	if err := json.Unmarshal(decoded.Payload, &payload); err != nil || payload["accountId"] != account || payload["revision"] != revision {
		t.Fatalf("outbox does not describe authoritative state: %s, %v", decoded.Payload, err)
	}
}

func (f *repairFixture) assertMetrics(t *testing.T, eligible, good, commits, rollbacks uint64) {
	t.Helper()
	response, err := f.exporter.Client().Get(f.exporter.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("metrics response: %d %v", response.StatusCode, response.Header)
	}
	names := []string{
		"sli_read_eligible_requests_total", "sli_read_good_requests_total", "sli_read_fast150_requests_total", "sli_read_fast400_requests_total",
		"sli_mutation_eligible_requests_total", "sli_mutation_good_requests_total", "durable_transaction_commits_total", "durable_transaction_rollbacks_total",
	}
	routes := []string{"browse", "quote", "wallet_challenge", "metadata", "websocket_replay", "account_mutation", "other"}
	expected := make(map[string]uint64)
	types := make(map[string]bool)
	for _, name := range names {
		types["# TYPE "+name+" counter"] = true
		for _, route := range routes {
			var count uint64
			switch {
			case name == "sli_mutation_eligible_requests_total" && route == "account_mutation":
				count = eligible
			case name == "sli_mutation_good_requests_total" && route == "account_mutation":
				count = good
			case name == "durable_transaction_commits_total" && route == "other":
				count = commits
			case name == "durable_transaction_rollbacks_total" && route == "other":
				count = rollbacks
			}
			expected[fmt.Sprintf("%s{service=%q,environment=%q,zone=%q,route=%q}", name, "userd", "qualification", "zone-a", route)] = count
		}
	}
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			if !types[line] {
				t.Fatalf("unexpected or duplicate metric declaration %q", line)
			}
			delete(types, line)
			continue
		}
		key, rawCount, found := strings.Cut(line, " ")
		want, known := expected[key]
		count, err := strconv.ParseUint(rawCount, 10, 64)
		if !found || !known || err != nil || count != want {
			t.Fatalf("metric %q: value %d, want %d; bounded series=%v; error=%v", line, count, want, known, err)
		}
		delete(expected, key)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(expected) != 0 || len(types) != 0 {
		t.Fatalf("missing metric series/declarations: %v %v", expected, types)
	}
}

func TestRepairM_1013_Commit(t *testing.T) {
	f := newRepairFixture(t, 1<<20, true)
	if err := f.run(context.Background(), "alice"); err != nil {
		t.Fatal(err)
	}
	f.assertState(t, "alice", true, true)
	f.assertMetrics(t, 1, 1, 1, 0)
}

func TestRepairM_1013_Rollback(t *testing.T) {
	f := newRepairFixture(t, 1, true)
	if err := f.run(context.Background(), "alice"); !errors.Is(err, repairOutboxFull) {
		t.Fatalf("lost transaction error: %v", err)
	}
	f.assertState(t, "alice", false, false)
	f.assertMetrics(t, 1, 0, 0, 1)
}

func TestRepairM_1013_MissingAck(t *testing.T) {
	f := newRepairFixture(t, 1<<20, false)
	if err := f.run(context.Background(), "alice"); !errors.Is(err, repairMissingAck) {
		t.Fatalf("lost acknowledgement error: %v", err)
	}
	f.assertState(t, "alice", true, true)
	f.assertMetrics(t, 1, 0, 1, 0)
}

func TestRepairM_1013_CommitBeforeAck(t *testing.T) {
	f := newRepairFixture(t, 1<<20, true)
	err := f.metrics.WithMutation(context.Background(), "account_mutation", func(context.Context) error {
		return nil
	}, func(ctx context.Context) error {
		return f.store.transact(ctx, "alice")
	}, func(ctx context.Context) error {
		f.assertState(t, "alice", true, false)
		f.assertMetrics(t, 0, 0, 1, 0)
		return f.acknowledge(ctx, "alice")
	})
	if err != nil {
		t.Fatal(err)
	}
	f.assertState(t, "alice", true, true)
	f.assertMetrics(t, 1, 1, 1, 0)
}

func TestRepairM_1013_InvalidOperation(t *testing.T) {
	f := newRepairFixture(t, 1<<20, true)
	if err := f.run(context.Background(), ""); !errors.Is(err, repairInvalid) {
		t.Fatalf("lost validation error: %v", err)
	}
	f.assertState(t, "", false, false)
	f.assertMetrics(t, 0, 0, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.run(ctx, "alice"); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	f.assertState(t, "alice", false, false)
	invoked := false
	operation := func(context.Context) error { invoked = true; return nil }
	for _, route := range []string{"/accounts/alice", "account_mutation{wallet=alice}", ""} {
		if err := f.metrics.WithMutation(context.Background(), route, operation, operation, operation); err == nil {
			t.Fatalf("accepted unbounded route %q", route)
		}
	}
	if err := f.metrics.WithMutation(context.Background(), "account_mutation", operation, nil, operation); err == nil {
		t.Fatal("accepted missing transaction")
	}
	if err := f.metrics.WithMutation(context.Background(), "account_mutation", nil, operation, operation); err == nil {
		t.Fatal("accepted missing validation")
	}
	if err := f.metrics.WithMutation(context.Background(), "account_mutation", operation, operation, nil); err == nil {
		t.Fatal("accepted missing acknowledgement")
	}
	if err := f.metrics.WithMutation(nil, "account_mutation", operation, operation, operation); err == nil {
		t.Fatal("accepted missing context")
	}
	var uninitialized *telemetry.Metrics
	if err := uninitialized.WithMutation(context.Background(), "account_mutation", operation, operation, operation); err == nil {
		t.Fatal("accepted missing metrics")
	}
	if invoked {
		t.Fatal("invalid configuration invoked operation")
	}
	if _, err := telemetry.New("userd_alice", "qualification", "zone-a"); err == nil {
		t.Fatal("accepted unbounded service")
	}
	f.assertMetrics(t, 0, 0, 0, 0)
}

func TestRepairM_1013_Panic(t *testing.T) {
	for _, stage := range []string{"transaction", "acknowledgement"} {
		t.Run(stage, func(t *testing.T) {
			f := newRepairFixture(t, 1<<20, true)
			acknowledged := false
			func() {
				defer func() {
					if value := recover(); value != "operation panic" {
						t.Fatalf("panic changed or swallowed: %v", value)
					}
				}()
				_ = f.metrics.WithMutation(context.Background(), "account_mutation", func(context.Context) error { return nil }, func(ctx context.Context) error {
					if stage == "transaction" {
						panic("operation panic")
					}
					return f.store.transact(ctx, "alice")
				}, func(context.Context) error {
					acknowledged = true
					panic("operation panic")
				})
			}()
			if stage == "transaction" {
				if acknowledged {
					t.Fatal("acknowledged uncommitted transaction")
				}
				f.assertState(t, "alice", false, false)
				f.assertMetrics(t, 1, 0, 0, 1)
			} else {
				f.assertState(t, "alice", true, false)
				f.assertMetrics(t, 1, 0, 1, 0)
			}
		})
	}
}

func TestRepairM_1013_ConcurrentOperations(t *testing.T) {
	f := newRepairFixture(t, 1<<20, true)
	const operations = 24
	results := make(chan error, operations)
	for i := 0; i < operations; i++ {
		go func(i int) { results <- f.run(context.Background(), fmt.Sprintf("account_%d", i)) }(i)
	}
	for i := 0; i < operations; i++ {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	_, _, profiles, outbox := f.store.snapshot("")
	f.mu.Lock()
	received := len(f.received)
	f.mu.Unlock()
	if profiles != operations || outbox != operations || received != operations {
		t.Fatalf("operations lost: profiles=%d outbox=%d consumed=%d", profiles, outbox, received)
	}
	f.assertMetrics(t, operations, operations, operations, 0)
}

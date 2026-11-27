package consumerhealth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
)

func healthState() (*State, *time.Time, *bytes.Buffer) {
	now := time.Unix(1700000000, 0).UTC()
	var output bytes.Buffer
	state := New("accounting_consumer", time.Minute, slog.New(slog.NewJSONHandler(&output, nil)), func() time.Time { return now })
	return state, &now, &output
}

func TestRecordDeliveryCancellationAndProgress(t *testing.T) {
	state, now, _ := healthState()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state.RecordDelivery(ctx, context.Canceled, "cancelled", nil)
	if got := state.Snapshot(); got != (Snapshot{}) {
		t.Fatalf("shutdown cancellation changed delivery health: %+v", got)
	}
	state.RecordDelivery(ctx, delivery.ErrIntegrity, "integrity", nil)
	if got := state.Snapshot(); got.ConsecutiveFailures != 1 || got.LastError != *now || got.ErrorClass != "integrity" {
		t.Fatalf("integrity observation = %+v", got)
	}
	*now = now.Add(time.Second)
	state.RecordDelivery(context.Background(), nil, "", nil)
	if got := state.Snapshot(); got.ConsecutiveFailures != 0 || got.LastSuccess != *now || got.RetryPartitions != 0 {
		t.Fatalf("delivery progress = %+v", got)
	}
}

func TestStateProgressBoundAndRecovery(t *testing.T) {
	state, now, output := healthState()
	ctx := context.Background()
	if state.Snapshot() != (Snapshot{}) {
		t.Fatal(state.Snapshot())
	}
	*now = now.Add(time.Hour)
	if err := state.Check(ctx); err != nil {
		t.Fatalf("idle consumer: %v", err)
	}
	state.Record(nil, "", "125:pool")
	success := *now
	state.Record(delivery.ErrLease, ErrorClass(delivery.ErrLease), "125:pool")
	*now = now.Add(time.Minute)
	if err := state.Check(ctx); err != nil {
		t.Fatalf("at progress bound: %v", err)
	}
	*now = now.Add(time.Nanosecond)
	state.Record(delivery.ErrLease, "lease", "125:pool")
	for i := 0; i < 2; i++ {
		if err := state.Check(ctx); !errors.Is(err, ErrNoProgress) {
			t.Fatalf("beyond progress bound: %v", err)
		}
	}
	want := Snapshot{LastSuccess: success, LastError: *now, ErrorClass: "lease", ConsecutiveFailures: 2, RetryPartitions: 1}
	if got := state.Snapshot(); got != want {
		t.Fatalf("snapshot = %+v, want %+v", got, want)
	}
	if strings.Count(output.String(), "accounting consumer readiness changed") != 1 {
		t.Fatal(output.String())
	}
	state.Record(nil, "", "125:pool")
	if err := state.Check(ctx); err != nil {
		t.Fatalf("recovery: %v", err)
	}
	want.LastSuccess, want.ConsecutiveFailures, want.RetryPartitions = *now, 0, 0
	if got := state.Snapshot(); got != want {
		t.Fatalf("recovered snapshot = %+v, want %+v", got, want)
	}
	if strings.Count(output.String(), "accounting consumer readiness changed") != 2 || !strings.Contains(output.String(), `"ready":false`) || !strings.Contains(output.String(), `"ready":true`) {
		t.Fatal(output.String())
	}
	if strings.Contains(output.String(), "125:pool") {
		t.Fatal("retry identity appears in readiness log")
	}
}

func TestStateStartupSourceFailureAndCancellation(t *testing.T) {
	state, now, _ := healthState()
	state.Record(ErrNoProgress, SourceUnavailableClass, "125:pool")
	if got := state.Snapshot(); got.ErrorClass != "source_unavailable" || !got.LastSuccess.IsZero() || got.LastError != *now {
		t.Fatal(got)
	}
	*now = now.Add(time.Minute)
	if err := state.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Nanosecond)
	if err := state.Check(context.Background()); !errors.Is(err, ErrNoProgress) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := state.Check(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	state.Record(nil, "", "125:other")
	state.Record(ErrNoProgress, SourceUnavailableClass, "125:pool")
	if err := state.Check(context.Background()); err != nil || state.Snapshot().RetryPartitions != 1 {
		t.Fatalf("progress on another partition: %+v %v", state.Snapshot(), err)
	}
	*now = now.Add(-time.Hour)
	if state.age(*now) != 0 {
		t.Fatal("negative success age")
	}
}

func TestStateDefaultsAndSaturatedFailures(t *testing.T) {
	state := New("accounting_consumer", time.Minute, nil, nil)
	if state.logger == nil || state.started.IsZero() || state.now == nil {
		t.Fatal("constructor defaults are absent")
	}
	state, _, _ = healthState()
	state.snapshot.ConsecutiveFailures = ^uint64(0)
	state.Record(delivery.ErrLease, "lease")
	if state.Snapshot().ConsecutiveFailures != ^uint64(0) {
		t.Fatal("failure counter wrapped")
	}
	state.Record(nil, "")
	if state.Snapshot().ConsecutiveFailures != 0 {
		t.Fatal("success did not reset counter")
	}
	var absent *State
	absent.Record(delivery.ErrLease, "lease", "pool")
}

func TestRetryGroupsAndBoundedHashedIdentities(t *testing.T) {
	state, _, _ := healthState()
	state.Record(delivery.ErrLease, "lease", "", "125:wallet:token_a", "125:wallet:token_b", "125:other:token_a", "standalone")
	state.Record(delivery.ErrLease, "lease", "125:wallet:token_a")
	if got := state.Snapshot().RetryPartitions; got != 3 || len(state.retries) != 4 {
		t.Fatalf("retry groups = %d, identities = %d", got, len(state.retries))
	}
	if state.retries[sha256.Sum256([]byte("125:wallet:token_a"))] != sha256.Sum256([]byte("wallet")) {
		t.Fatal("retry identity or group was not hashed")
	}
	for i, key := range []string{"125:wallet:token_a", "125:wallet:token_b", "125:other:token_a", "standalone"} {
		state.Record(nil, "", key, "missing", "")
		if got, want := state.Snapshot().RetryPartitions, []int{3, 2, 1, 0}[i]; got != want {
			t.Fatalf("after %s: %d groups, want %d", key, got, want)
		}
	}
	for i := 0; i < MaxRetryPartitions; i++ {
		state.Record(delivery.ErrLease, "lease", fmt.Sprintf("125:partition_%d", i))
	}
	if state.Snapshot().RetryOverflow || state.Snapshot().RetryPartitions != MaxRetryPartitions {
		t.Fatal(state.Snapshot())
	}
	state.Record(delivery.ErrLease, "lease", "125:partition_0")
	if state.Snapshot().RetryOverflow {
		t.Fatal("duplicate retry exhausted capacity")
	}
	state.Record(delivery.ErrLease, "lease", "125:overflow")
	if got := state.Snapshot(); !got.RetryOverflow || got.RetryPartitions != MaxRetryPartitions || len(state.retries) != MaxRetryPartitions {
		t.Fatal(got)
	}
	state.Record(nil, "", "125:partition_0")
	state.Record(delivery.ErrLease, "lease", "125:overflow")
	if got := state.Snapshot(); !got.RetryOverflow || got.RetryPartitions != MaxRetryPartitions {
		t.Fatal(got)
	}
}

func TestErrorClassPreservesWrappedFailures(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{delivery.ErrAuthorization, "authorization"}, {delivery.ErrSchema, "schema"},
		{delivery.ErrIntegrity, "integrity"}, {delivery.ErrLease, "lease"},
		{delivery.ErrGap, "gap"}, {context.DeadlineExceeded, "timeout"},
		{context.Canceled, "cancelled"}, {errors.New("connection lost"), "transient"},
		{&delivery.IntegrityFailure{Class: delivery.PartitionBinding, Cause: delivery.ErrIntegrity}, "partition_binding"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			if got := ErrorClass(fmt.Errorf("delivery failed: %w", tc.err)); got != tc.want {
				t.Fatalf("class = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRetryKeyFromCanonicalPublication(t *testing.T) {
	fill := accounting.FillV1{
		Version:       accounting.FillVersion,
		Position:      accounting.SourcePosition{ChainID: "125", Height: "100", TransactionIndex: "0", LogIndex: "7", TransactionHash: "0x" + strings.Repeat("a", 64), BlockHash: "0x" + strings.Repeat("b", 64)},
		ActualEmitter: "0x" + strings.Repeat("1", 40), PoolAddress: "0x" + strings.Repeat("2", 40),
		Token: accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 18}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6},
		Side: "buy", TokenAmountRaw: "1000000000000000000", USDLAmountRaw: "1000000", PaidInputRaw: "1000000", FeeRaw: "3000", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl",
		Attribution: accounting.Attribution{TransactionSender: "0x" + strings.Repeat("5", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Execution:   accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:  accounting.Provenance{DeploymentID: "private-125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)}, BlockTimestampSeconds: "1700000000",
	}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	key := "125:" + fill.PoolAddress
	position := accounting.PartitionPosition{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: key, Sequence: "1", SourceEventID: fill.SourceFillID, PayloadDigest: digest, SourceOrder: fill.Position, EffectIdentity: accounting.EffectIdentity(fill.SourceFillID, "market", "market-v1", "", "market", key)}
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: accounting.PublicationVersion, SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: "100", TransactionIndex: "0", LogIndex: "7"}, EmittedAt: time.Unix(1700000000, 0).UTC(), TraceID: "consumer-health", PayloadHash: delivery.Hash(payload), Payload: payload}
	wire, err := envelope.Encode(delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{accounting.PublicationVersion: true}}, accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	if got := RetryKey(wire); got != key {
		t.Fatalf("key = %q, want %q", got, key)
	}
	publication.Positions = nil
	emptyPositions, err := json.Marshal(publication)
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range [][]byte{[]byte(`{}`), emptyPositions, []byte(`null`)} {
		envelope.Payload = payload
		invalid, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if got := RetryKey(invalid); got != "" {
			t.Fatalf("invalid publication key = %q", got)
		}
	}
	for _, invalid := range []string{"", "{", "null", `{"payload":"invalid"}`} {
		if got := RetryKey([]byte(invalid)); got != "" {
			t.Fatalf("invalid envelope key = %q", got)
		}
	}
}

func TestMetricsStatesAndMethods(t *testing.T) {
	state, now, _ := healthState()
	idle, _, _ := healthState()
	idle.name = "idle_consumer"
	base, err := telemetry.New("chartsd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	handler := Metrics(base, "chartsd", "qualification", "zone-a", nil, idle, state)
	check := func(success, failure, count, partitions, overflow string, class string) {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		body := response.Body.String()
		labels := `service="chartsd",environment="qualification",zone="zone-a",consumer="accounting_consumer"`
		for name, value := range map[string]string{"consumer_last_success_timestamp_seconds": success, "consumer_last_error_timestamp_seconds": failure, "consumer_consecutive_failures": count, "consumer_retry_partitions": partitions, "consumer_retry_tracking_overflow": overflow} {
			if !strings.Contains(body, name+"{"+labels+"} "+value+"\n") {
				t.Fatalf("missing %s=%s: %s", name, value, body)
			}
		}
		if !strings.Contains(body, "consumer_last_error_info{"+labels+fmt.Sprintf(",error_class=%q} 1\n", class)) || !strings.Contains(body, `consumer="idle_consumer"} 0`) || !strings.Contains(body, "sli_read_eligible_requests_total{") {
			t.Fatal(body)
		}
		if strings.Contains(body, "125:partition_") {
			t.Fatal("retry identity appears in metrics")
		}
	}
	check("0", "0", "0", "0", "0", "")
	state.Record(nil, "")
	*now = now.Add(time.Second)
	for i := 0; i <= MaxRetryPartitions; i++ {
		state.Record(ErrNoProgress, SourceUnavailableClass, fmt.Sprintf("125:partition_%d", i))
	}
	check("1.7e+09", "1.700000001e+09", "10001", "10000", "1", SourceUnavailableClass)
	for _, method := range []string{http.MethodHead, http.MethodPost} {
		actual, expected := httptest.NewRecorder(), httptest.NewRecorder()
		handler.ServeHTTP(actual, httptest.NewRequest(method, "/metrics", nil))
		base.ServeHTTP(expected, httptest.NewRequest(method, "/metrics", nil))
		if actual.Code != expected.Code || actual.Body.String() != expected.Body.String() {
			t.Fatalf("%s changed underlying response", method)
		}
	}
}

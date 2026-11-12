package policy

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func TestDeadLetterInspectionPaginatesValidatedRecords(t *testing.T) {
	pool, schema := realSecurityDatabase(t)
	ctx := context.Background()
	table := pgx.Identifier{schema, "delivery_quarantine"}.Sanitize()
	_, err := pool.Exec(ctx, "CREATE TABLE "+table+"(consumer_id text,projection_generation text,envelope_hash text,failure_class text,created_at timestamptz)")
	if err != nil {
		t.Fatal(err)
	}
	firstHash := strings.Repeat("a", 64)
	secondHash := strings.Repeat("b", 64)
	for _, record := range []struct{ consumer, hash, failure string }{{"candles", firstHash, "handler-error"}, {"trades", secondHash, "other"}} {
		if _, err := pool.Exec(ctx, "INSERT INTO "+table+" VALUES($1,'v1',$2,$3,$4)", record.consumer, record.hash, record.failure, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	reader := DeadLetters{Pool: pool, DeliverySchema: schema}
	first, err := reader.Inspect(ctx, "", 1)
	if err != nil || len(first) != 1 || first[0].Consumer != "candles" || first[0].FailureClass != "handler-error" {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	cursor, err := json.Marshal(DeadLetterMetadata{Consumer: first[0].Consumer, Generation: first[0].Generation, EnvelopeHash: first[0].EnvelopeHash})
	if err != nil {
		t.Fatal(err)
	}
	second, err := reader.Inspect(ctx, base64.RawURLEncoding.EncodeToString(cursor), 1)
	if err != nil || len(second) != 1 || second[0].Consumer != "trades" || second[0].FailureClass != "unclassified" {
		t.Fatalf("second page: %+v, %v", second, err)
	}
	for _, sample := range []struct {
		name, cursor string
		limit        int
	}{
		{"invalid base64", "!", 1},
		{"unknown field", base64.RawURLEncoding.EncodeToString([]byte(`{"unknown":true}`)), 1},
		{"invalid digest", base64.RawURLEncoding.EncodeToString([]byte(`{"envelopeHash":"short"}`)), 1},
		{"extra object", base64.RawURLEncoding.EncodeToString(append(cursor, []byte(` {}`)...)), 1},
		{"zero limit", "", 0},
		{"large limit", "", 101},
	} {
		if _, err := reader.Inspect(ctx, sample.cursor, sample.limit); !errors.Is(err, ErrDenied) {
			t.Fatalf("%s: %v", sample.name, err)
		}
	}
	if _, err := (DeadLetters{Pool: pool, DeliverySchema: "invalid schema"}).Inspect(ctx, "", 1); !errors.Is(err, ErrDenied) {
		t.Fatalf("invalid delivery schema: %v", err)
	}
}

func TestMeasuredPublicAdmissionRecordsLimiterDecision(t *testing.T) {
	socket := localSecurityRedis(t)
	client := redis.NewClient(&redis.Options{Network: "unix", Addr: socket})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	capture := &MiddlewareCapture{Writer: &output, Key: []byte(strings.Repeat("k", 32)), RunID: "coverage-run", ReplicaID: "public-a", CandidateSHA256: strings.Repeat("a", 64), PolicyVersion: "v1"}
	served := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { served++; w.WriteHeader(http.StatusNoContent) })
	options := AdmissionOptions{Redis: client, Namespace: "measured-public", Class: classPublicRead, Cost: 1}
	missingNamespace := options
	missingNamespace.Namespace = ""
	if _, err := Admission(missingNamespace, next); !errors.Is(err, ErrDenied) {
		t.Fatalf("missing budget namespace: %v", err)
	}
	unknownClass := options
	unknownClass.Class = "unknown"
	if _, err := Admission(unknownClass, next); !errors.Is(err, ErrDenied) {
		t.Fatalf("unknown budget class: %v", err)
	}
	if _, err := MeasuredAdmission(options, next, nil); !errors.Is(err, ErrDenied) {
		t.Fatalf("missing capture: %v", err)
	}
	handler, err := MeasuredAdmission(options, next, capture)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/public", nil)
	request.Header.Set("X-Request-ID", "measured-read")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || served != 1 {
		t.Fatalf("measured read: %d, %d", response.Code, served)
	}
	var record MiddlewareRecord
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil || record.Stage != "limiter" || !record.Allowed {
		t.Fatalf("limiter record: %+v, %v", record, err)
	}
	request.RemoteAddr = "invalid-address"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || served != 1 {
		t.Fatalf("invalid principal: %d, %d", response.Code, served)
	}
}

func TestPublicPayloadUnicodeAndExportPaths(t *testing.T) {
	for _, sample := range []struct {
		name    string
		payload string
		valid   bool
	}{
		{"plain", `{"name":"hello"}`, true},
		{"paired surrogate", `{"name":"\uD83D\uDE03"}`, true},
		{"lone high surrogate", `{"name":"\uD83D"}`, false},
		{"lone low surrogate", `{"name":"\uDE03"}`, false},
		{"invalid hex", `{"name":"\uZZZZ"}`, false},
		{"truncated escape", `{"name":"\u12`, false},
		{"trailing slash", `{"name":"abc\`, false},
		{"invalid utf8", string([]byte{0xff}), false},
	} {
		if got := validUnicode([]byte(sample.payload)); got != sample.valid {
			t.Fatalf("%s: valid=%v", sample.name, got)
		}
	}
	for _, prefix := range []string{"", "/audit", "audit/", "audit//day", "audit/../day", "audit/*"} {
		if safeExportPrefix(prefix) {
			t.Fatalf("invalid export prefix %q", prefix)
		}
	}
	if !safeExportPrefix("audit/daily") {
		t.Fatal("daily export prefix rejected")
	}
	ddl, err := AuditExportDDL("audit_schema")
	if err != nil || !strings.Contains(ddl, `"audit_schema"."audit_export_days"`) {
		t.Fatalf("export state DDL: %q, %v", ddl, err)
	}
	if _, err := AuditExportDDL("invalid schema"); !errors.Is(err, ErrDenied) {
		t.Fatalf("invalid export schema: %v", err)
	}
}

func TestRequestBoundsAppliesReadAndMutationLimits(t *testing.T) {
	var served atomic.Int32
	handler := RequestBounds(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, sample := range []struct {
		name   string
		change func(*http.Request)
		want   int
	}{
		{"read", func(*http.Request) {}, http.StatusNoContent},
		{"query", func(r *http.Request) { r.URL.RawQuery = "q=" + strings.Repeat("x", 8192) }, http.StatusRequestURITooLong},
		{"header", func(r *http.Request) { r.Header.Set("X-Large", strings.Repeat("x", 16384)) }, http.StatusRequestHeaderFieldsTooLarge},
	} {
		r := httptest.NewRequest(http.MethodGet, "/resource", nil)
		sample.change(r)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		if response.Code != sample.want {
			t.Fatalf("%s: %d", sample.name, response.Code)
		}
	}
	mutation := httptest.NewRequest(http.MethodPost, "/resource", strings.NewReader(`{"Name":"value"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, mutation)
	if response.Code != http.StatusServiceUnavailable || served.Load() != 1 {
		t.Fatalf("unavailable request deadline: %d, served=%d", response.Code, served.Load())
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/resource", strings.NewReader(`{"Name":"value"}`))
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer accepted.Body.Close()
	if accepted.StatusCode != http.StatusNoContent || served.Load() != 2 {
		t.Fatalf("bounded mutation: %d, served=%d", accepted.StatusCode, served.Load())
	}
	if _, err := io.Copy(io.Discard, accepted.Body); err != nil {
		t.Fatal(err)
	}
}

func TestPublicClientRedirectAndDestinationBounds(t *testing.T) {
	client := PublicClient(map[string]bool{"PUBLIC.EXAMPLE": true})
	allowed, err := url.Parse("https://public.example/resource")
	if err != nil {
		t.Fatal(err)
	}
	request := &http.Request{URL: allowed}
	if err := client.CheckRedirect(request, nil); err != nil {
		t.Fatalf("declared redirect: %v", err)
	}
	if err := client.CheckRedirect(request, []*http.Request{request, request, request, request}); !errors.Is(err, ErrDenied) {
		t.Fatalf("redirect ceiling: %v", err)
	}
	private, err := url.Parse("https://127.0.0.1/resource")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(&http.Request{URL: private}, nil); !errors.Is(err, ErrDenied) {
		t.Fatalf("private redirect: %v", err)
	}
	if conn, err := DialPublic(context.Background(), "udp", "public.example:443"); !errors.Is(err, ErrDenied) || conn != nil {
		t.Fatalf("unsupported public dial: %v", err)
	}
	if _, err := SecurityDDL("invalid schema"); !errors.Is(err, ErrDenied) {
		t.Fatalf("invalid security schema: %v", err)
	}
}

func TestExporterReportsRoleAuthorityBeforeWriting(t *testing.T) {
	pool, schema := realSecurityDatabase(t)
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	exporter := AuditExporter{Pool: pool, StatePool: pool, Schema: schema, Bucket: "audit-bucket", Prefix: "security/daily", KeyID: "signer-a", Client: s3.NewFromConfig(aws.Config{Region: "us-east-1"}), Signer: signer}
	alerts := 0
	err = exporter.Run(context.Background(), time.Now().Add(-48*time.Hour), func(_ context.Context, cause error) error {
		alerts++
		if !errors.Is(cause, ErrDenied) {
			t.Errorf("role authority alert: %v", cause)
		}
		return nil
	})
	if !errors.Is(err, ErrDenied) || alerts != 1 {
		t.Fatalf("export role check: %v, alerts=%d", err, alerts)
	}
	if _, err := exporter.Export(context.Background(), time.Now().Add(time.Hour)); !errors.Is(err, ErrDenied) {
		t.Fatalf("future audit export: %v", err)
	}
}

func TestKeyRotationRejectsAmbiguousActiveWindows(t *testing.T) {
	ring := &KeyRing{}
	if err := ring.Replace(nil); !errors.Is(err, ErrDenied) {
		t.Fatalf("empty rotation: %v", err)
	}
	now := time.Now()
	first := Key{ID: "webhook", Version: "first", Audience: "vendor", Activate: now, Retire: now.Add(time.Hour), Value: []byte(strings.Repeat("a", 32))}
	second := Key{ID: "webhook", Version: "second", Audience: "vendor", Activate: now.Add(time.Second), Retire: now.Add(time.Hour), Value: []byte(strings.Repeat("b", 32))}
	invalid := first
	invalid.Value = []byte("short")
	if err := ring.Replace([]Key{invalid}); !errors.Is(err, ErrDenied) {
		t.Fatalf("short signing value: %v", err)
	}
	if err := ring.Replace([]Key{first, second}); !errors.Is(err, ErrDenied) {
		t.Fatalf("overlapping rotation window: %v", err)
	}
	first.Retire = now.Add(5 * time.Minute)
	if err := ring.Replace([]Key{first, second}); err != nil {
		t.Fatalf("bounded key rotation: %v", err)
	}
	if budget, ok := frozenBudget(classAccountRead); !ok || budget.Burst != 100 {
		t.Fatalf("account read budget: %+v, %v", budget, ok)
	}
	if got := MutationStatus(context.DeadlineExceeded); got != http.StatusRequestTimeout {
		t.Fatalf("deadline mutation status: %d", got)
	}
}

func TestEdgeLeaseRejectsMissingAndReversedTime(t *testing.T) {
	if (*EdgeAdmission)(nil).Take(time.Now()) {
		t.Fatal("missing edge lease admitted read")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Second)
	lease := EdgeLease{Origin: "public-cache", PolicyVersion: "v1", ID: "bounded-lease", Issued: now.Add(-time.Second), Expires: now.Add(50 * time.Second), PerSecond: 2}
	wire, err := json.Marshal(lease)
	if err != nil {
		t.Fatal(err)
	}
	edge, err := NewEdgeAdmission(lease, ed25519.Sign(private, wire), public, lease.Origin, now)
	if err != nil {
		t.Fatal(err)
	}
	if !edge.Take(now.Add(time.Second)) || edge.Take(now) {
		t.Fatal("edge lease accepted a reversed second")
	}
}

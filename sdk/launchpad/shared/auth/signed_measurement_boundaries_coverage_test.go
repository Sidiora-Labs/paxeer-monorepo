package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
)

func TestSignedMeasurementClassifiesAuthenticatedHTTPResponses(t *testing.T) {
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("authorization", "localhost")
	clientLeaf := pki.Leaf("generator", "localhost")
	key := testpki.Ed25519(t)
	walletKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	principal := IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: stringsLowerAddress(walletKey), AuthEpoch: 1, ProofAt: time.Now(), AccessExpiresAt: time.Now().Add(time.Hour)}
	signer := ServiceAssertionIssuer{Issuer: "gateway", Audience: "account", Key: key.Private, Transport: AssertionTransport{ActorAssertionHeader, ActorSignatureHeader}}
	verifier := ServiceAssertionVerifier{Issuer: signer.Issuer, Audience: signer.Audience, Key: key.Private.Public().(ed25519.PublicKey), Transport: signer.Transport, PeerCertSHA256: clientLeaf.SHA256}
	listener := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		if _, err := verifier.Authenticate(r, body); err != nil {
			t.Error("signed load assertion rejected", err)
			w.WriteHeader(401)
			return
		}
		raw, err := base64.RawURLEncoding.DecodeString(r.Header.Get(OperationEnvelopeHeader))
		if err != nil {
			t.Error(err)
			return
		}
		var envelope OperationEnvelope
		if json.Unmarshal(raw, &envelope) != nil {
			t.Error("missing operation envelope")
			return
		}
		statement, err := envelope.Statement()
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := VerifyEOA(principal.Wallet, statement, r.Header.Get(OperationSignatureHeader), nil); err != nil {
			t.Error(err)
			return
		}
		switch r.URL.Path {
		case "/status":
			w.WriteHeader(503)
		case "/overload":
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(503)
		case "/malformed":
			_, _ = io.WriteString(w, `{"requestId":`)
		case "/oversize":
			_, _ = io.WriteString(w, strings.Repeat("x", (1<<20)+1))
		case "/identity":
			_, _ = io.WriteString(w, `{"requestId":"other","data":{}}`)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"requestId": r.Header.Get("X-Request-ID"), "data": map[string]bool{"accepted": true}})
		}
	}))
	listener.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{serverLeaf.Pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots()}
	listener.StartTLS()
	defer listener.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{clientLeaf.Pair}}}
	defer transport.CloseIdleConnections()
	pool, _ := auditCoverageDatabase(t)
	base := SignedLoadHarness{Client: &http.Client{Transport: transport}, Endpoint: listener.URL, Identity: principal, Signer: signer, WalletKey: walletKey, Pool: pool, Action: "profile.update", Method: http.MethodPatch, Path: "/accepted", Body: []byte(`{"name":"owner"}`), Transport: OperationTransport{OperationEnvelopeHeader, OperationSignatureHeader}, ServerCertificateSHA256: serverLeaf.SHA256, Verification: &AuthVerificationMeasurements{}}
	digest := strings.Repeat("a", 64)
	plan := SignedLoadPlan{RunID: uuid.NewString(), CandidateSHA256: digest, SpecSHA256: digest, SourceSHA256: map[string]string{"auth": digest}, Environment: "test", Region: "private", ProofKind: valueEoaEip191, DurationSeconds: 1, TargetRPS: 1, Concurrency: 1, QueueCapacity: 1, RequestTimeoutMS: 1000, SampleIntervalMS: 100}
	plan.WorkloadSHA256 = SignedLoadWorkloadDigest(plan)
	if err := base.validate(plan); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ path, outcome string }{{"/accepted", valueAccepted}, {"/status", "valid_operation_failed"}, {"/malformed", valueResponseInvalid}, {"/identity", valueResponseInvalid}, {"/oversize", valueResponseInvalid}} {
		candidate := base
		candidate.Path = test.path
		now := time.Now()
		sample := candidate.execute(context.Background(), plan, 1, now, now)
		if sample.Outcome != test.outcome || sample.ServiceLatencyNS <= 0 || sample.CorrectedLatencyNS < sample.ServiceLatencyNS || sample.FinishedAt.Before(sample.StartedAt) {
			t.Fatalf("signed response %s: %+v", test.path, sample)
		}
	}
	for name, edit := range map[string]func(*SignedLoadHarness){
		"body":      func(h *SignedLoadHarness) { h.Body = []byte(`{"key":1,"key":2}`) },
		"method":    func(h *SignedLoadHarness) { h.Method = "invalid method" },
		"signature": func(h *SignedLoadHarness) { h.Signer.Key = nil },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			edit(&candidate)
			now := time.Now()
			sample := candidate.execute(context.Background(), plan, 1, now, now)
			if sample.Outcome != "request_invalid" {
				t.Fatalf("invalid request outcome: %+v", sample)
			}
		})
	}
	candidate := base
	candidate.ServerCertificateSHA256 = strings.Repeat("0", 64)
	now := time.Now()
	if sample := candidate.execute(context.Background(), plan, 1, now, now); sample.Outcome != valueResponseInvalid {
		t.Fatalf("server certificate identity: %+v", sample)
	}
	for name, edit := range map[string]func(*SignedLoadHarness){"expiry": func(h *SignedLoadHarness) { h.Identity.AccessExpiresAt = time.Now().Add(time.Millisecond) }, "body": func(h *SignedLoadHarness) { h.Body = []byte("{") }, "endpoint": func(h *SignedLoadHarness) { h.Endpoint += "/path" }, "observer": func(h *SignedLoadHarness) { h.Verification = nil }} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			edit(&candidate)
			if err := candidate.validate(plan); err == nil {
				t.Fatal("unbound signed workload admitted")
			}
		})
	}
	changed := plan
	changed.WorkloadSHA256 = digest
	if err := base.validate(changed); err == nil {
		t.Fatal("changed workload identity admitted")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := base.Run(cancelled, plan, &bytes.Buffer{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, path := range []string{"/status", "/overload"} {
		candidate := base
		candidate.Path = path
		load := plan
		if path == "/overload" {
			load.TargetRPS = 1000
		}
		load.WorkloadSHA256 = SignedLoadWorkloadDigest(load)
		var raw bytes.Buffer
		result, err := candidate.Run(context.Background(), load, &raw)
		if err == nil || result.Failures == 0 || result.SuccessfulRequests != 0 || result.ScheduledRequests != int64(load.TargetRPS) || result.QueueObservations == 0 || len(result.AssertionFailures) == 0 || raw.Len() == 0 {
			t.Fatalf("failed signed workload admitted: %+v %v", result, err)
		}
		if path == "/overload" && result.GeneratorRejected == 0 {
			t.Fatal("bounded generator did not report saturation")
		}
	}
	listener.Close()
	now = time.Now()
	if sample := base.execute(context.Background(), plan, 1, now, now); sample.Outcome != "transport_failure" {
		t.Fatalf("unavailable endpoint: %+v", sample)
	}
}

func TestSignedRecorderPreservesRawSamplesAndWriteErrors(t *testing.T) {
	var raw bytes.Buffer
	var result SignedLoadResult
	recorder := &signedMeasurementRecorder{writer: &raw, digest: sha256.New(), result: &result}
	recorder.sample(SignedLoadSample{Outcome: "generator_saturated", Warmup: true})
	recorder.sample(SignedLoadSample{RequestID: uuid.NewString(), Outcome: valueAccepted, CorrectedLatencyNS: 2, ServiceLatencyNS: 1})
	recorder.sample(SignedLoadSample{Outcome: "transport_failure", CorrectedLatencyNS: 3, ServiceLatencyNS: 2})
	sum := sha256.Sum256(raw.Bytes())
	if result.GeneratorRejected != 1 || result.WarmupRequests != 1 || result.MeasuredRequests != 2 || result.SuccessfulRequests != 1 || result.Failures != 1 || len(recorder.acknowledged) != 1 || !bytes.Equal(recorder.digest.Sum(nil), sum[:]) {
		t.Fatalf("raw measurement accounting: %+v", result)
	}
	recorder.write(make(chan int))
	if recorder.err == nil {
		t.Fatal("non-JSON sample recorded")
	}
	file, err := os.CreateTemp(t.TempDir(), "raw")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	failed := &signedMeasurementRecorder{writer: file, digest: sha256.New(), result: &SignedLoadResult{}}
	failed.write(map[string]int{"sample": 1})
	if !errors.Is(failed.err, os.ErrClosed) || hex.EncodeToString(failed.digest.Sum(nil)) != hex.EncodeToString(sha256.New().Sum(nil)) {
		t.Fatalf("raw write failure: %v", failed.err)
	}
}

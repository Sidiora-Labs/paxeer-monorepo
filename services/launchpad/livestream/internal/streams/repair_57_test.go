package streams

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/go-chi/chi/v5"
)

const repair57RequestID = "22222222-2222-4222-8222-222222222222"
const repair57StreamID = "11111111-1111-4111-8111-111111111111"

// This is a real signed ingress and reverse proxy, using the same shared
// request issuer/verifier and certificate policy as the integration proofs.
func repair57Ingress(t *testing.T, next http.Handler) *httptest.Server {
	t.Helper()
	pki := testpki.New(t)
	upstream := pki.Leaf("livestream", "livestream.test")
	gateway := pki.Leaf("gateway", "gateway.test")
	assertion := testpki.Ed25519(t)
	transport := sharedauth.AssertionTransport{AssertionHeader: sharedauth.ActorAssertionHeader, SignatureHeader: sharedauth.ActorSignatureHeader}
	issuer := sharedauth.ServiceRequestIssuer{Issuer: "gateway", Audience: "livestream", Key: assertion.Private, Transport: transport}
	verifier := sharedauth.ServiceRequestVerifier{Issuer: "gateway", Audience: "livestream", Key: assertion.Private.Public().(ed25519.PublicKey), PeerCertSHA256: gateway.SHA256, Transport: transport}
	private := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
		if err != nil || verifier.Authenticate(r, raw) != nil {
			http.Error(w, "untrusted ingress", http.StatusForbidden)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		next.ServeHTTP(w, r)
	}))
	private.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots(), Certificates: []tls.Certificate{upstream.Pair}}
	private.StartTLS()
	t.Cleanup(private.Close)
	target, err := url.Parse(private.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	connection := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{gateway.Pair}}}
	t.Cleanup(connection.CloseIdleConnections)
	proxy.Transport = connection
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		var raw []byte
		if r.Body != nil {
			var err error
			raw, err = io.ReadAll(r.Body)
			if err != nil {
				panic(err)
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		r.Header.Set("X-Request-ID", repair57RequestID)
		if err := issuer.SignRequest(r, raw); err != nil {
			panic(err)
		}
	}
	public := httptest.NewServer(proxy)
	t.Cleanup(public.Close)
	return public
}

func repair57Envelope(t *testing.T, schema string, raw []byte) {
	t.Helper()
	if err := v2.ValidateJSON(schema, raw); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 7 || string(fields["requestId"]) != strconv.Quote(repair57RequestID) || string(fields["quality"]) != `"complete"` || string(fields["watermark"]) != "null" || string(fields["nextCursor"]) != "null" {
		t.Fatalf("incomplete frozen envelope: %s", raw)
	}
	if bytes.Contains(raw, []byte("private_")) {
		t.Fatalf("private operation material leaked: %s", raw)
	}
}

func repair57ResponseServer(t *testing.T, status int, produce func() ([]byte, error)) *httptest.Server {
	t.Helper()
	// The codec is the component under test. HTTP and the actual SDK remain real;
	// no lifecycle store, SQL driver, ledger, provider or RPC double is installed.
	return repair57Ingress(t, alias(func(w http.ResponseWriter, r *http.Request) {
		raw, err := produce()
		if err != nil {
			New(Deps{}).lifecycleError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(raw)
	}))
}

func TestRepair_57_001_RetiredCredentialRecovery(t *testing.T) {
	router := chi.NewRouter()
	New(Deps{}).RegisterRoutes(router)
	server := repair57Ingress(t, router)
	client := &http.Client{Timeout: 5 * time.Second}
	path := "/v2/streams/" + repair57StreamID + "/ingest-credential"
	var first []byte
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest("POST", server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusGone || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Link") != "</v2/streams/"+repair57StreamID+"/ingest-key>; rel=\"successor-version\"" {
			t.Fatalf("unsupported credential was not explicitly retired: %d %v %s", response.StatusCode, response.Header, raw)
		}
		var refusal v2.Error
		if err := v2.ValidateJSON("Error", raw); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &refusal); err != nil {
			t.Fatal(err)
		}
		if refusal.Code != "INGEST_CREDENTIAL_RETIRED" || refusal.Retryable || bytes.Contains(raw, []byte("ingestCredential")) || bytes.Contains(raw, []byte("streamKey")) {
			t.Fatalf("retirement exposed or promised credentials: %s", raw)
		}
		first = raw
		server.Close()
		// Reopen the actual mounted handler and its signed HTTP ingress. Retirement
		// must remain explicit even without a database or an available provider.
		router = chi.NewRouter()
		New(Deps{}).RegisterRoutes(router)
		server = repair57Ingress(t, router)
	}
	var decoded v2.Error
	repair57Consumer(t).decode("Error", first, &decoded)
	if decoded.Code != "INGEST_CREDENTIAL_RETIRED" {
		t.Fatal(decoded)
	}
}

func TestRepair_57_002_CreateEnvelopeTransportReplay(t *testing.T) {
	consumer := repair57Consumer(t)
	o := Operation{ID: repair57StreamID, State: "REQUESTED", Revision: 9007199254740992, ProviderID: "private_provider", Wallet: "private_wallet", Title: "private_title"}
	raw, err := AdaptV2StreamCreate(repair57RequestID, o)
	if err != nil {
		t.Fatal(err)
	}
	repair57Envelope(t, "StreamCreateResponse", raw)
	var response v2.StreamCreateResponse
	consumer.decode("StreamCreateResponse", raw, &response)
	if response.Data.Operation.OperationId != o.ID || response.Data.Operation.StreamId != o.ID || response.Data.Operation.Status != "requested" || response.Revision == nil || *response.Revision != response.Data.Operation.Revision || string(*response.Revision) != "9007199254740992" || response.Data.Operation.RetryAt != nil {
		t.Fatal(response)
	}
	// Persist the actual serialized producer bytes, close the transport, and
	// reopen both transport and SDK consumer. The full signed SQL-ledger restart
	// assertions remain in TestRepairIntegration_57_002_CreateEnvelopeReplay.
	path := filepath.Join(t.TempDir(), "response.json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err = file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	serve := func() ([]byte, error) { return os.ReadFile(path) }
	first := repair57ResponseServer(t, http.StatusAccepted, serve)
	decoded := consumer.run("operation", map[string]any{"origin": first.URL, "operation": "IF_STREAM_CREATE", "input": map[string]any{"body": map[string]string{"poolAddress": "0x2222222222222222222222222222222222222222", "title": "Codec replay"}}})
	if err = json.Unmarshal(decoded, &response); err != nil {
		t.Fatal(err)
	}
	first.Close()
	second := repair57ResponseServer(t, http.StatusAccepted, serve)
	replay := consumer.run("operation", map[string]any{"origin": second.URL, "operation": "IF_STREAM_CREATE", "input": map[string]any{"body": map[string]string{"poolAddress": "0x2222222222222222222222222222222222222222", "title": "Codec replay"}}})
	if !bytes.Equal(decoded, replay) {
		t.Fatal("reopened SDK consumer observed different frozen response")
	}
	wireResponse, err := http.Get(second.URL + "/v2/streams")
	if err != nil {
		t.Fatal(err)
	}
	wire, err := io.ReadAll(wireResponse.Body)
	wireResponse.Body.Close()
	if err != nil || !bytes.Equal(raw, wire) || wireResponse.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("serialized bytes changed across transport restart: %s %v", wire, err)
	}
	for _, invalid := range []Operation{{ID: "invalid", State: "REQUESTED", Revision: 1}, {ID: o.ID, State: "unknown", Revision: 1}, {ID: o.ID, State: "READY", Revision: 0}} {
		if wire, err := AdaptV2StreamCreate(repair57RequestID, invalid); err == nil || wire != nil {
			t.Fatalf("invalid operation serialized: %s %v", wire, err)
		}
	}
	if wire, err := AdaptV2StreamCreate("invalid", o); err == nil || wire != nil {
		t.Fatal("invalid ledger request identity serialized")
	}
}

func TestRepair_57_003_OperationPollingEnvelope(t *testing.T) {
	consumer := repair57Consumer(t)
	at := time.Now().UTC().Truncate(time.Millisecond)
	retry := at.Add(time.Minute)
	for _, state := range []string{"REQUESTED", "PROVISIONING", "READY", "LIVE", "ENDING", "ENDED", "FAILED", "SUSPENDED"} {
		t.Run(state, func(t *testing.T) {
			o := Operation{ID: repair57StreamID, State: state, Revision: 9007199254740992, RetryAt: &retry, ProviderID: "private_provider", Wallet: "private_wallet"}
			raw, err := AdaptV2StreamOperationGet(repair57RequestID, o, at)
			if err != nil {
				t.Fatal(err)
			}
			repair57Envelope(t, "StreamOperationGetResponse", raw)
			server := repair57ResponseServer(t, http.StatusOK, func() ([]byte, error) { return AdaptV2StreamOperationGet(repair57RequestID, o, at) })
			decoded := consumer.run("operation", map[string]any{"origin": server.URL, "operation": "IF_STREAM_OPERATION_GET", "input": map[string]any{"path": map[string]string{"operationId": o.ID}}})
			var response v2.StreamOperationGetResponse
			if err = json.Unmarshal(decoded, &response); err != nil {
				t.Fatal(err)
			}
			if response.Data.Operation.OperationId != o.ID || response.Data.Operation.StreamId != o.ID || response.Data.Operation.Status != strings.ToLower(state) || response.Revision == nil || string(*response.Revision) != "9007199254740992" || *response.Revision != response.Data.Operation.Revision || response.Data.Operation.RetryAt == nil || *response.Data.Operation.RetryAt != retry.Format(v2LifecycleTimeLayout) || response.GeneratedAt != at.Format(v2LifecycleTimeLayout) {
				t.Fatal(response)
			}
			server.Close()
		})
	}
	for _, o := range []Operation{{ID: "invalid", State: "READY", Revision: 1}, {ID: repair57StreamID, State: "unknown", Revision: 1}, {ID: repair57StreamID, State: "READY", Revision: -1}} {
		if raw, err := AdaptV2StreamOperationGet(repair57RequestID, o, at); err == nil || raw != nil {
			t.Fatalf("invalid polling observation serialized: %s %v", raw, err)
		}
	}
	if raw, err := AdaptV2StreamOperationGet(repair57RequestID, Operation{ID: repair57StreamID, State: "READY", Revision: 1}, time.Time{}); err == nil || raw != nil {
		t.Fatal("missing observation time serialized")
	}
}

func TestRepair_57_004_BodylessRetirement(t *testing.T) {
	bodyless := make(chan bool, 1)
	router := chi.NewRouter()
	New(Deps{}).RegisterRoutes(router)
	server := repair57Ingress(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "unreadable request", 400)
			return
		}
		bodyless <- len(raw) == 0 && r.Header.Get("Content-Type") == ""
		r.Body = io.NopCloser(bytes.NewReader(raw))
		router.ServeHTTP(w, r)
	}))
	raw := repair57Consumer(t).run("retired", map[string]string{"origin": server.URL, "id": repair57StreamID, "session": "owner-session"})
	var response v2.Error
	if err := json.Unmarshal(raw, &response); err != nil || response.Code != "INGEST_CREDENTIAL_RETIRED" || response.Retryable {
		t.Fatalf("SDK did not decode retirement: %s %v", raw, err)
	}
	select {
	case correct := <-bodyless:
		if !correct {
			t.Fatal("SDK bodyless contract acquired an undocumented JSON body or content type")
		}
	default:
		t.Fatal("SDK did not cross the authenticated ingress")
	}
	// Retirement must not broaden the successor's existing signature contract.
	digest, err := sharedauth.BodyDigest([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	envelope := sharedauth.OperationEnvelope{Version: 1, ChainID: "125", Environment: "test", Actor: "0x1111111111111111111111111111111111111111", Action: "stream.credential", Method: "POST", Path: "/v2/streams/" + repair57StreamID + "/ingest-key", BodyHash: digest, Nonce: strings.Repeat("a", 64), IssuedAt: now, ExpiresAt: now + 300}
	if err = envelope.Validate(time.Unix(now, 0), "test", envelope.Actor, envelope.Action, envelope.Method, envelope.Path, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err = envelope.Validate(time.Unix(now, 0), "test", envelope.Actor, envelope.Action, envelope.Method, envelope.Path, nil); err == nil {
		t.Fatal("bodyless request reused the signed JSON authorization")
	}
}

func TestRepair_57_005_NormalizedStudioWebhookContract(t *testing.T) {
	consumer := repair57Consumer(t)
	secret := strings.Repeat("s", 32)
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, tc := range []struct{ studio, state, event string }{{"stream.started", "LIVE", "active"}, {"stream.idle", "ENDED", "inactive"}} {
		raw := []byte(fmt.Sprintf(`{"id":"studio_event","webhookId":"studio_callback","createdAt":%d,"timestamp":%d,"event":%q,"stream":{"id":"studio_provider"}}`, now.Add(-time.Minute).UnixMilli(), now.UnixMilli(), tc.studio))
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(raw)
		signature := fmt.Sprintf("t=%d,v1=%s", now.UnixMilli(), hex.EncodeToString(mac.Sum(nil)))
		request := httptest.NewRequest("POST", "/webhooks/livepeer", bytes.NewReader(raw))
		request.Header.Set("Livepeer-Signature", signature)
		wire, ok := NormalizeStudioWebhook(httptest.NewRecorder(), request, secret, now)
		if !ok {
			t.Fatal("real Studio normalizer rejected signed producer bytes")
		}
		var body v2.LiveWebhookBody
		consumer.decode("LiveWebhookBody", wire.Body, &body)
		if body.Event != tc.event || body.StreamId != "studio_provider" || body.EventId != "studio_event" || body.OccurredAt != now.Add(-time.Minute).Format(v2LifecycleTimeLayout) {
			t.Fatal(body)
		}
		expected := ProviderEvent{EventID: body.EventId, ProviderID: body.StreamId, State: tc.state, ObservedAt: now.Add(-time.Minute)}
		verify := func(raw []byte, timestamp, signature, contentType string, clock time.Time) (ProviderEvent, bool, int) {
			request := httptest.NewRequest("POST", "/v2/provider-webhooks/livepeer", bytes.NewReader(raw))
			request.Header.Set("Content-Type", contentType)
			request.Header.Set(NormalizedWebhookTimestampHeader, timestamp)
			request.Header.Set(NormalizedWebhookSignatureHeader, signature)
			rec := httptest.NewRecorder()
			event, ok := VerifyNormalizedProviderWebhook(rec, request, secret, clock)
			return event, ok, rec.Code
		}
		for i := 0; i < 2; i++ {
			event, ok, status := verify(wire.Body, wire.Timestamp, wire.Signature, "application/json", now)
			if !ok || status != http.StatusOK || !reflect.DeepEqual(event, expected) {
				t.Fatalf("frozen consumer lost provider event: %+v %t %d", event, ok, status)
			}
		}
		for _, invalid := range []struct {
			raw                         []byte
			timestamp, signature, media string
			clock                       time.Time
			status                      int
		}{
			{wire.Body, wire.Timestamp, "sha256=" + strings.Repeat("0", 64), "application/json", now, 401},
			{append(append([]byte{}, wire.Body...), byte(' ')), wire.Timestamp, wire.Signature, "application/json", now, 401},
			{wire.Body, strconv.FormatInt(now.Unix()+1, 10), wire.Signature, "application/json", now, 401},
			{wire.Body, wire.Timestamp, wire.Signature, "application/json", now.Add(301 * time.Second), 401},
			{wire.Body, wire.Timestamp, wire.Signature, "application/json", now.Add(-302 * time.Second), 401},
			{wire.Body, wire.Timestamp, wire.Signature, "text/plain", now, 415},
			{raw, wire.Timestamp, signature, "application/json", now, 401},
		} {
			event, ok, status := verify(invalid.raw, invalid.timestamp, invalid.signature, invalid.media, invalid.clock)
			if ok || event != (ProviderEvent{}) || status != invalid.status {
				t.Fatalf("invalid normalized delivery accepted: %+v %t %d", event, ok, status)
			}
		}
		response, err := AdaptV2LiveWebhook(repair57RequestID, now)
		if err != nil {
			t.Fatal(err)
		}
		repair57Envelope(t, "LiveWebhookResponse", response)
		server := repair57Ingress(t, alias(func(w http.ResponseWriter, r *http.Request) {
			event, ok := VerifyNormalizedProviderWebhook(w, r, secret, now)
			if !ok {
				return
			}
			if !reflect.DeepEqual(event, expected) {
				t.Errorf("HTTP authentication changed the normalized event: %+v", event)
				return
			}
			raw, err := AdaptV2LiveWebhook(repair57RequestID, now)
			if err != nil {
				New(Deps{}).lifecycleError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, json.RawMessage(raw))
		}))
		request, err = http.NewRequest("POST", server.URL+"/v2/provider-webhooks/livepeer", bytes.NewReader(wire.Body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(NormalizedWebhookTimestampHeader, wire.Timestamp)
		request.Header.Set(NormalizedWebhookSignatureHeader, wire.Signature)
		delivered, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(delivered.Body)
		delivered.Body.Close()
		if err != nil || delivered.StatusCode != http.StatusOK || delivered.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("webhook HTTP authentication failed: %s %v", decoded, err)
		}
		var accepted v2.LiveWebhookResponse
		consumer.decode("LiveWebhookResponse", decoded, &accepted)
		if !accepted.Data.Accepted || accepted.GeneratedAt != now.Format(v2LifecycleTimeLayout) {
			t.Fatalf("SDK rejected webhook acknowledgment: %s", decoded)
		}
		server.Close()
		expired := httptest.NewRequest("POST", "/webhooks/livepeer", bytes.NewReader(raw))
		expired.Header.Set("Livepeer-Signature", signature)
		if _, ok := NormalizeStudioWebhook(httptest.NewRecorder(), expired, secret, now.Add(301*time.Second)); ok {
			t.Fatal("expired raw callback refreshed into a normalized signature")
		}
	}
	if raw, err := AdaptV2LiveWebhook("invalid", now); err == nil || raw != nil {
		t.Fatal("invalid webhook request identity serialized")
	}
}

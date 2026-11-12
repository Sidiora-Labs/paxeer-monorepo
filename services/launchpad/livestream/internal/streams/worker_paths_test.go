package streams

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
)

func TestProvisionAndReconcileWorkersPersistProviderState(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	o, _ := f.create("worker-provider-state")
	var created atomic.Bool
	var deleted atomic.Bool
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /stream":
			if !created.Load() {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":"provider_1","playbackId":"playback_1","streamKey":"key_1","name":"KindleLaunch:` + o.ID + `"}]`))
		case "POST /stream":
			var body struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name != livepeer.CorrelationName(o.ID) {
				t.Errorf("provision name = %+v, %v", body, err)
			}
			created.Store(true)
			_, _ = w.Write([]byte(`{"id":"provider_1","playbackId":"playback_1","streamKey":"key_1","name":"KindleLaunch:` + o.ID + `"}`))
		case "PATCH /stream/provider_1":
			w.WriteHeader(http.StatusNoContent)
		case "GET /stream/provider_1":
			if deleted.Load() {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"id":"provider_1","playbackId":"playback_1","streamKey":"key_1","isActive":true,"playbackPolicy":{"type":"webhook","webhookId":"playback_hook","webhookContext":{"operationId":"` + o.ID + `"},"refreshInterval":1}}`))
		case "DELETE /stream/provider_1":
			deleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected provider call %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer providerServer.Close()
	provider := livepeer.New("fixture-key", providerServer.URL, nil, livepeer.WithWebhookPlayback("playback_hook"))
	restrictions := &RestrictionReader{Pool: f.database}
	provision := &ProvisionWorker{Store: f.store, Provider: provider, Credentials: &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"}, Restrictions: restrictions}
	if err := provision.Step(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	ready, err := f.store.Get(ctx, o.ID)
	if err != nil || ready.State != stateReady || ready.ProviderID != "provider_1" {
		t.Fatalf("provisioned operation = %+v, %v", ready, err)
	}
	var cipher string
	if err := f.database.QueryRow(ctx, "SELECT ciphertext FROM livestream.stream_credentials WHERE operation_id=$1", o.ID).Scan(&cipher); err != nil || cipher == "" || cipher == "key_1" {
		t.Fatalf("encrypted credential = %q, %v", cipher, err)
	}
	if err := provision.Step(ctx, o.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("worker lease was not retained: %v", err)
	}
	f.close()
	f.open()
	credentialPath := "/v2/streams/" + o.ID + "/ingest-key"
	credential := f.exchange(http.MethodPost, credentialPath, []byte(`{}`), f.signed(credentialPath, "stream.credential", []byte(`{}`), f.key), http.StatusOK)
	var keyDocument struct {
		StreamID  string `json:"streamId"`
		IngestKey struct {
			StreamKey string `json:"streamKey"`
		} `json:"ingestKey"`
	}
	if err := json.Unmarshal(credential, &keyDocument); err != nil || keyDocument.StreamID != o.ID || keyDocument.IngestKey.StreamKey != "key_1" {
		t.Fatalf("owner credential = %s, %v", credential, err)
	}
	editPath := "/v2/streams/" + o.ID
	editBody := []byte(fmt.Sprintf(`{"title":"Updated stream","revision":"%d","private":false}`, ready.Revision))
	editHeaders := f.signed(editPath, "stream.edit", editBody, f.key)
	envelopeRaw, err := base64.RawURLEncoding.DecodeString(editHeaders["X-Kindle-Operation"])
	if err != nil {
		t.Fatal(err)
	}
	var envelope sharedauth.OperationEnvelope
	if err := json.Unmarshal(envelopeRaw, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Method = http.MethodPatch
	statement, err := envelope.Statement()
	if err != nil {
		t.Fatal(err)
	}
	signature, err := crypto.Sign(accounts.TextHash([]byte(statement)), f.key)
	if err != nil {
		t.Fatal(err)
	}
	envelopeRaw, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	editHeaders["X-Kindle-Operation"] = base64.RawURLEncoding.EncodeToString(envelopeRaw)
	editHeaders["X-Kindle-Signature"] = hexutil.Encode(signature)
	editResponse := f.exchange(http.MethodPatch, editPath, editBody, editHeaders, http.StatusAccepted)
	edited, err := f.store.Get(ctx, o.ID)
	if err != nil || edited.Title != "Updated stream" || edited.Revision != ready.Revision+1 || !bytes.Contains(editResponse, []byte(`"operation"`)) {
		t.Fatalf("edited stream = %+v, %s, %v", edited, editResponse, err)
	}
	restrictions.Pool = f.database
	if _, err := f.database.Exec(ctx, "UPDATE livestream.stream_worker_attempts SET retry_at='-infinity', lease_until='-infinity' WHERE operation_id=$1", o.ID); err != nil {
		t.Fatal(err)
	}
	reconcile := &ReconcileWorker{Store: f.store, Provider: provider, Restrictions: restrictions}
	if err := reconcile.Step(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	live, err := f.store.Get(ctx, o.ID)
	if err != nil || live.State != stateLive {
		t.Fatalf("live operation = %+v, %v", live, err)
	}
	viewers, err := NewViewerCounter(f.redis, "livestream", 100000)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	New(Deps{Lifecycle: f.store, Restrictions: restrictions, Viewers: viewers, ViewerTokens: &ViewerTokenAuthority{Key: []byte(strings.Repeat("v", 32))}, PlaybackWebhookSecret: strings.Repeat("p", 32)}).RegisterRoutes(router)
	viewerServer := httptest.NewServer(router)
	defer viewerServer.Close()
	sessionResponse, err := http.Post(viewerServer.URL+"/v2/streams/"+o.ID+"/viewer-session", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var session struct {
		SessionID string `json:"sessionId"`
		Token     string `json:"token"`
	}
	if err := json.NewDecoder(sessionResponse.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if err := sessionResponse.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if sessionResponse.StatusCode != http.StatusOK || session.SessionID == "" || session.Token == "" {
		t.Fatalf("viewer session = %+v, status=%d", session, sessionResponse.StatusCode)
	}
	heartbeatBody, err := json.Marshal(map[string]string{"sessionId": session.SessionID, "token": session.Token})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		response, err := http.Post(viewerServer.URL+"/v2/streams/"+o.ID+"/heartbeat", "application/json", bytes.NewReader(heartbeatBody))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want || i == 0 && !bytes.Contains(raw, []byte(`"viewerCount":1`)) {
			t.Fatalf("heartbeat %d = %d %s", i, response.StatusCode, raw)
		}
	}
	stamp := time.Now().UnixMilli()
	playbackBody := []byte(fmt.Sprintf(`{"timestamp":%d,"context":{"operationId":"%s"},"accessKey":"%s"}`, stamp, o.ID, session.Token))
	mac := hmac.New(sha256.New, []byte(strings.Repeat("p", 32)))
	mac.Write(playbackBody)
	playbackRequest, err := http.NewRequest(http.MethodPost, viewerServer.URL+"/webhooks/livepeer/playback-access", bytes.NewReader(playbackBody))
	if err != nil {
		t.Fatal(err)
	}
	playbackRequest.Header.Set("Livepeer-Signature", fmt.Sprintf("t=%d,v1=%s", stamp, hex.EncodeToString(mac.Sum(nil))))
	playbackResponse, err := http.DefaultClient.Do(playbackRequest)
	if err != nil {
		t.Fatal(err)
	}
	if err := playbackResponse.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if playbackResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("provider playback authorization status = %d", playbackResponse.StatusCode)
	}
	checkPlayback := func(accessKey string, want int) {
		t.Helper()
		stamp := time.Now().UnixMilli()
		body := []byte(fmt.Sprintf(`{"timestamp":%d,"context":{"operationId":"%s"},"accessKey":%q}`, stamp, o.ID, accessKey))
		mac := hmac.New(sha256.New, []byte(strings.Repeat("p", 32)))
		_, _ = mac.Write(body)
		request, err := http.NewRequest(http.MethodPost, viewerServer.URL+"/webhooks/livepeer/playback-access", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Livepeer-Signature", fmt.Sprintf("t=%d,v1=%s", stamp, hex.EncodeToString(mac.Sum(nil))))
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			raw, _ := io.ReadAll(response.Body)
			t.Fatalf("playback access decision = %d %s", response.StatusCode, raw)
		}
	}
	checkPlayback("broken", http.StatusForbidden)
	checkPlayback("invalid.invalid", http.StatusForbidden)
	checkPlayback(strings.Repeat("x", 513), http.StatusForbidden)
	if _, err := f.database.Exec(ctx, "INSERT INTO chat.safety_bans(wallet,scope,resource) VALUES($1,'pool',$2)", strings.ToLower(o.Wallet), strings.ToLower(o.Pool)); err != nil {
		t.Fatal(err)
	}
	checkPlayback(session.Token, http.StatusForbidden)
	if _, err := f.database.Exec(ctx, "UPDATE livestream.stream_worker_attempts SET retry_at='-infinity', lease_until='-infinity' WHERE operation_id=$1", o.ID); err != nil {
		t.Fatal(err)
	}
	if err := reconcile.Step(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	ended, err := f.store.Get(ctx, o.ID)
	if err != nil || ended.State != stateEnded || !deleted.Load() {
		t.Fatalf("restricted operation = %+v, deleted=%t, %v", ended, deleted.Load(), err)
	}
}

func TestViewerHeartbeatsPruneExpiredMembersAndPersistCount(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, "CREATE TABLE livestream.streams(id text PRIMARY KEY, viewer_count bigint NOT NULL DEFAULT 0)"); err != nil {
		t.Fatal(err)
	}
	stream := generateID(time.UnixMilli(1700000000000))
	if !strings.HasPrefix(stream, "loyw3v28-") || len(strings.Split(stream, "-")) != 2 {
		t.Fatalf("stream id = %q", stream)
	}
	if _, err := f.database.Exec(ctx, "INSERT INTO livestream.streams(id) VALUES($1)", stream); err != nil {
		t.Fatal(err)
	}
	h := &Handlers{redis: f.redis, q: sqlcdb.New(f.database)}
	for _, session := range []string{"viewer_1", "viewer_1", "viewer_2"} {
		count, err := h.countViewers(ctx, stream, session)
		if err != nil || count != map[string]int{"viewer_1": 1, "viewer_2": 2}[session] {
			t.Fatalf("viewer count for %s = %d, %v", session, count, err)
		}
	}
	if err := f.redis.Del(ctx, viewerEntryKey(stream, "viewer_1")).Err(); err != nil {
		t.Fatal(err)
	}
	count, err := h.countViewers(ctx, stream, "viewer_2")
	if err != nil || count != 1 {
		t.Fatalf("stale viewer remained counted: %d, %v", count, err)
	}
	if members, err := f.redis.SMembers(ctx, "stream:viewers:"+stream).Result(); err != nil || len(members) != 1 || members[0] != "viewer_2" {
		t.Fatalf("viewer members = %v, %v", members, err)
	}
	var persisted int
	if err := f.database.QueryRow(ctx, "SELECT viewer_count FROM livestream.streams WHERE id=$1", stream).Scan(&persisted); err != nil || persisted != 1 {
		t.Fatalf("persisted viewer count = %d, %v", persisted, err)
	}
}

func TestVendorUsageWorkerPersistsAccountUsageAndAlerts(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.BudgetSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	policy := repairM1001Budget()
	policy.VendorUserID = "account_1"
	policy.MonthlyUnits = "10"
	policy.DeliveryPricePerMinute = "1"
	policy.TranscodePricePerMinute = "1"
	policy.StoragePricePerMinute = "1"
	f.store.Budget = &policy
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/data/usage/query":
			_, _ = w.Write([]byte(`{"UserID":"account_1","DeliveryUsageMins":6,"TotalUsageMins":0,"StorageUsageMins":0}`))
		case "/user/me":
			_, _ = w.Write([]byte(`{"id":"account_1"}`))
		case "/data/views/query":
			_, _ = w.Write([]byte(`[{"playtimeMins":2.5},{"playtimeMins":1.5}]`))
		default:
			t.Errorf("unexpected usage request: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	worker := &VendorUsageWorker{Store: f.store, Provider: livepeer.New("fixture-key", server.URL, nil)}
	if err := worker.Step(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	var minutes, cost string
	if err := f.database.QueryRow(ctx, "SELECT viewer_minutes::text,cost_units::text FROM livestream.stream_vendor_usage WHERE vendor_user_id=$1", policy.VendorUserID).Scan(&minutes, &cost); err != nil || minutes != "4.000000000000000000" || cost != "6.000000000000000000" {
		t.Fatalf("persisted usage = %q %q, %v", minutes, cost, err)
	}
	var thresholds []int
	rows, err := f.database.Query(ctx, "SELECT threshold FROM livestream.stream_budget_alerts ORDER BY threshold")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var threshold int
		if err := rows.Scan(&threshold); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		thresholds = append(thresholds, threshold)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(thresholds) != 1 || thresholds[0] != 50 {
		t.Fatalf("budget alerts = %v, %v", thresholds, err)
	}
	if err := worker.DeliverAlert(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unconfigured alert endpoint accepted: %v", err)
	}
	alertServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("untrusted alert endpoint received a budget record")
	}))
	defer alertServer.Close()
	worker.AlertEndpoint = alertServer.URL
	worker.AlertKey = []byte(strings.Repeat("k", 32))
	if err := worker.DeliverAlert(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("untrusted alert endpoint accepted: %v", err)
	}
	var attempt int
	var delivered bool
	if err := f.database.QueryRow(ctx, "SELECT attempt,delivered_at IS NOT NULL FROM livestream.stream_budget_alerts WHERE threshold=50").Scan(&attempt, &delivered); err != nil || attempt != 1 || delivered {
		t.Fatalf("budget delivery state = %d %t, %v", attempt, delivered, err)
	}
	if _, err := f.database.Exec(ctx, "UPDATE livestream.stream_budget_alerts SET retry_at='-infinity' WHERE threshold=50"); err != nil {
		t.Fatal(err)
	}
	trustedAlert := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		mac := hmac.New(sha256.New, worker.AlertKey)
		_, _ = mac.Write(body)
		var event struct {
			EventID string `json:"eventId"`
		}
		if err := json.Unmarshal(body, &event); err != nil || r.Header.Get("X-Kindle-Alert-Signature") != hex.EncodeToString(mac.Sum(nil)) || r.Header.Get("Idempotency-Key") != event.EventID {
			t.Errorf("budget alert binding = %q, %v", body, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"eventId":%q,"accepted":true}`, event.EventID)
	}))
	defer trustedAlert.Close()
	previousTransport := http.DefaultTransport
	http.DefaultTransport = trustedAlert.Client().Transport
	defer func() { http.DefaultTransport = previousTransport }()
	worker.AlertEndpoint = trustedAlert.URL
	if err := worker.DeliverAlert(ctx); err != nil {
		t.Fatalf("trusted budget alert = %v", err)
	}
	if err := f.database.QueryRow(ctx, "SELECT attempt,delivered_at IS NOT NULL FROM livestream.stream_budget_alerts WHERE threshold=50").Scan(&attempt, &delivered); err != nil || attempt != 2 || !delivered {
		t.Fatalf("accepted budget delivery = %d %t, %v", attempt, delivered, err)
	}
	worker.AlertEndpoint = ""
	if _, err := f.database.Exec(ctx, f.store.RotationSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	runtime := &ConnectedRuntime{
		Usage:     worker,
		Provision: &ProvisionWorker{Store: f.store, Provider: worker.Provider, Restrictions: &RestrictionReader{Pool: f.database}},
		Reconcile: &ReconcileWorker{Store: f.store, Provider: worker.Provider, Restrictions: &RestrictionReader{Pool: f.database}},
	}
	runCtx, cancel := context.WithTimeout(ctx, 5500*time.Millisecond)
	defer cancel()
	if err := runtime.Run(runCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("connected worker cancellation = %v", err)
	}
}

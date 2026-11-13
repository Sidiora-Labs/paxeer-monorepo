package notifications

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	pglite "github.com/elliots/go-pglite"
	"github.com/google/uuid"
)

// The SQL peer embeds its WASI engine and uses the actual pgx transport. It
// executes the production schema unchanged, with no external database process.
func repair958Open(t *testing.T, dir string) (*sql.DB, func()) {
	t.Helper()
	engine, e := pglite.New(pglite.Config{DataDir: dir})
	if e != nil {
		t.Fatal("open in-process SQL peer:", e)
	}
	var once sync.Once
	closeDB := func() {
		once.Do(func() {
			if e := engine.Close(); e != nil {
				t.Error("close SQL peer:", e)
			}
		})
	}
	t.Cleanup(closeDB)
	return engine.DB(), closeDB
}
func repair958Schema(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, e := db.Exec(Schema); e != nil {
		t.Fatal("production notification schema:", e)
	}
	if _, e := db.Exec(discovery.Schema); e != nil {
		t.Fatal("production discovery schema:", e)
	}
}
func repair958PEM(t *testing.T) string {
	t.Helper()
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}))
}

// Committed loopback peers supply the owner protocols; the production clients
// still validate mTLS, account/target revisions, consent, and signed identity.
type repair958Peers struct {
	server     *httptest.Server
	client     *http.Client
	identity   sharedauth.MTLSIdentityAuthority
	account    Principal
	target     PushTarget
	endpoint   Endpoint
	preference DeliveryPreference
	calls      atomic.Int64
}

func repair958Authorities(t *testing.T) *repair958Peers {
	t.Helper()
	p := &repair958Peers{account: Principal{AccountID: uuid.NewString(), Wallet: "0x1111111111111111111111111111111111111111"}}
	p.target = PushTarget{AccountID: p.account.AccountID, DeviceInstallationID: "repair958-device", Provider: "apns", EncryptedEndpointRef: "fixture-endpoint", Status: "active", ConsentRevision: 7, Revision: 11}
	p.preference = DeliveryPreference{AccountID: p.account.AccountID, Timezone: "UTC", Language: "en", Revision: 7, Linked: true, CategoryChannels: map[string]map[string]bool{"chat": {"push": true}, "watchlist": {"push": true}}}
	pki := testpki.New(t)
	serverPeer := pki.Leaf("identity", "identity.example")
	apiPeer := pki.Leaf("api", "api.example")
	signing := testpki.Ed25519(t)
	verifier := sharedauth.ServiceRequestVerifier{Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}, Issuer: "api", Audience: "identity", PeerCertSHA256: apiPeer.SHA256, Key: signing.Private.Public().(ed25519.PublicKey)}
	p.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, e := io.ReadAll(r.Body)
		if e != nil {
			t.Error(e)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/internal/v2/identity/request:authenticate" {
			if e := verifier.Authenticate(r, raw); e != nil {
				t.Error("identity service assertion:", e)
				w.WriteHeader(401)
				return
			}
			var in sharedauth.IdentityAuthenticationRequest
			if json.Unmarshal(raw, &in) != nil || sharedauth.ValidateIdentityAuthenticationRequest(in) != nil || in.Authorization != "Bearer repair958-session" {
				w.WriteHeader(401)
				return
			}
			p.calls.Add(1)
			principal := sharedauth.IdentityPrincipal{AccountID: p.account.AccountID, SessionID: "01900000-0000-7000-8000-000000000958", Wallet: p.account.Wallet, AuthEpoch: 3, ProofAt: time.Now().UTC().Add(-time.Minute), AccessExpiresAt: time.Now().UTC().Add(time.Hour)}
			if e := principal.Validate(time.Now()); e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			json.NewEncoder(w).Encode(principal)
			return
		}
		if r.Header.Get("Authorization") != "Bearer repair958-owner-token" {
			w.WriteHeader(401)
			return
		}
		base := accountPath(p.account.AccountID)
		switch r.URL.Path {
		case base + "/targets":
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
			} else {
				json.NewEncoder(w).Encode([]PushTarget{p.target})
			}
		case base + "/targets/" + p.target.DeviceInstallationID:
			json.NewEncoder(w).Encode(p.target)
		case base + "/targets/" + p.target.DeviceInstallationID + ":resolve":
			var target PushTarget
			if r.Method != "POST" || json.Unmarshal(raw, &target) != nil || target != p.target {
				t.Error("invalid endpoint resolution")
				w.WriteHeader(400)
				return
			}
			endpoint := p.endpoint
			if endpoint.Token == "" && endpoint.URL == "" {
				endpoint.Token = strings.Repeat("a", 64)
			}
			json.NewEncoder(w).Encode(endpoint)
		case base + "/delivery-preference":
			json.NewEncoder(w).Encode(p.preference)
		case base + "/targets/" + p.target.DeviceInstallationID + ":disable":
			var input struct {
				Target PushTarget
				Reason string
			}
			if r.Method != http.MethodPost || json.Unmarshal(raw, &input) != nil || input.Target != p.target || input.Reason == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case base + "/discovery-preferences":
			json.NewEncoder(w).Encode(discovery.PreferenceSnapshot{AccountID: p.account.AccountID, Revision: 1, ObservedAt: time.Now().UTC(), Linked: true})
		case base + "/watchlist":
			json.NewEncoder(w).Encode(discovery.WatchlistSnapshot{AccountID: p.account.AccountID, Revision: 1, Entries: []discovery.WatchlistEntry{{PoolAddress: "0x3333333333333333333333333333333333333333", AlertEnabled: true}}})
		case base + "/verified-account":
			json.NewEncoder(w).Encode(struct{ Verified bool }{true})
		case base + "/attribution":
			json.NewEncoder(w).Encode(discovery.WalletAttribution{AccountID: p.account.AccountID, ProfileID: p.account.Wallet, Verified: true, OptedIn: true})
		case "/internal/v2/wallets/" + p.account.Wallet + "/attribution":
			json.NewEncoder(w).Encode(discovery.WalletAttribution{AccountID: p.account.AccountID, ProfileID: p.account.Wallet, Verified: true, OptedIn: true})
		case "/internal/v2/notifications/chat":
			var input ChatRequest
			if r.Method != http.MethodPost || json.Unmarshal(raw, &input) != nil || input.AccountID != p.account.AccountID || input.EventID == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(ChatResponse{NotificationID: "chat-1", Created: true})
		case "/internal/v2/social/graph/" + p.account.AccountID:
			json.NewEncoder(w).Encode(discovery.GraphSnapshot{AccountID: p.account.AccountID, Revision: 1, ObservedAt: time.Now().UTC()})
		case "/internal/v2/social/public/callout/callout-1":
			json.NewEncoder(w).Encode(discovery.PublicResource{Ref: discovery.SourceRef{ID: "callout-1", EventID: "event-1", Kind: "callout", SourceRevision: 1, VisibilityRevision: 1, PublishedAt: time.Now().UTC(), Public: true, Authorized: true}})
		case "/internal/v2/social/callouts:review":
			json.NewEncoder(w).Encode(discovery.ModerationDecision{ResourceID: "callout-1", Revision: 1, Visible: true, ObservedAt: time.Now().UTC()})
		case "/internal/v2/social/callout-authority":
			if r.Method != http.MethodPost || !json.Valid(raw) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case accountPath("foreign-account") + "/discovery-preferences", base + "/targets/foreign-device":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected authority request: %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	p.server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots(), Certificates: []tls.Certificate{serverPeer.Pair}}
	p.server.StartTLS()
	t.Cleanup(p.server.Close)
	p.client = &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{apiPeer.Pair}}}}
	t.Cleanup(p.client.CloseIdleConnections)
	var e error
	p.identity, e = sharedauth.NewMTLSIdentityAuthority(p.server.URL, serverPeer.SHA256, p.client, sharedauth.ServiceRequestIssuer{Issuer: "api", Audience: "identity", Key: signing.Private, Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.identity.Client.CloseIdleConnections)
	return p
}
func (p *repair958Peers) principal(r *http.Request) (Principal, error) {
	actor, e := p.identity.AuthenticateRequest(r.Context(), r)
	return Principal{AccountID: actor.AccountID, Wallet: actor.Wallet}, e
}
func repair958Credentials(t *testing.T) secrets.Options {
	t.Helper()
	for _, key := range []string{"REPAIR958_D11", "REPAIR958_D12", "REPAIR958_INTERNAL"} {
		t.Setenv(key, "repair958-owner-token")
	}
	t.Setenv("REPAIR958_CURSOR", strings.Repeat("c", 32))
	return secrets.Options{Provider: secrets.ProviderEnv}
}

func TestRepairM_958_PushOriginSlash(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	credentials, e := NewCredentials(ctx, options, "REPAIR958_D11")
	if e != nil {
		t.Fatal(e)
	}
	authority, e := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("REPAIR958_APNS", repair958PEM(t))
	t.Setenv("REPAIR958_KID", "key-id")
	t.Setenv("REPAIR958_TEAM", "team-id")
	t.Setenv("REPAIR958_TOPIC", "example.notifications")
	links := make(chan string, 2)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DeepLink string `json:"deepLink"`
			ID       string `json:"notificationId"`
			APS      struct {
				Alert struct {
					Body string `json:"body"`
				} `json:"alert"`
			} `json:"aps"`
		}
		if r.Method != "POST" || r.URL.Path != "/3/device/"+strings.Repeat("a", 64) || !strings.HasPrefix(r.Header.Get("Authorization"), "bearer ") || r.Header.Get("Apns-Topic") != "example.notifications" || json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid APNS producer request")
			w.WriteHeader(400)
			return
		}
		if body.DeepLink != "https://app.example:443/notifications/"+body.ID || body.APS.Alert.Body != "You have a new notification" {
			t.Errorf("provider payload: %+v", body)
			w.WriteHeader(400)
			return
		}
		links <- body.DeepLink
		w.Header().Set("apns-id", "repair958-receipt")
		w.WriteHeader(200)
	}))
	defer provider.Close()
	transport := provider.Client().Transport.(*http.Transport).Clone()
	destination := strings.TrimPrefix(provider.URL, "https://")
	providerHost, _, e := net.SplitHostPort(destination)
	if e != nil {
		t.Fatal(e)
	}
	transport.TLSClientConfig.ServerName = providerHost
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "api.push.apple.com:443" {
			return nil, fmt.Errorf("unexpected provider %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, destination)
	}
	t.Cleanup(transport.CloseIdleConnections)
	for _, origin := range []string{"https://app.example:443", "https://app.example:443/"} {
		t.Run(origin, func(t *testing.T) {
			runtime, e := NewPushRuntime(ctx, db, authority, RuntimeOptions{Secrets: options, Owner: "repair958-worker", DeepLinkOrigin: origin, EnableAPNS: true, APNSKey: "REPAIR958_APNS", APNSKeyID: "REPAIR958_KID", APNSTeamID: "REPAIR958_TEAM", APNSTopic: "REPAIR958_TOPIC"})
			if e != nil {
				t.Fatal(e)
			}
			if runtime.Worker.DeepLinkOrigin != "https://app.example:443" {
				t.Fatalf("origin retained path: %q", runtime.Worker.DeepLinkOrigin)
			}
			runtime.Worker.Adapters["apns"].(*APNS).client = &http.Client{Timeout: time.Second, Transport: transport}
			item, created, e := (RecipientConsumer{Store: runtime.Worker.Store, Authority: authority}).EnqueueChat(ctx, "chat-"+origin, peer.account.AccountID, "conversation-958", time.Now().UTC())
			if e != nil || !created {
				t.Fatalf("real chat producer: %+v %v %v", item, created, e)
			}
			if e = runtime.Worker.Once(ctx, 1); e != nil {
				t.Fatal("worker to APNS:", e)
			}
			select {
			case link := <-links:
				if link != "https://app.example:443/notifications/"+item.ID {
					t.Fatal(link)
				}
			default:
				t.Fatal("APNS peer received no delivery")
			}
			page, e := runtime.Worker.Store.ListPage(ctx, peer.account.AccountID, false, "", 100)
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, n := range page.Items {
				if n.ID == item.ID {
					found = true
					if n.DeliveryState != "sent" {
						t.Fatalf("delivery did not commit: %+v", n)
					}
				}
			}
			if !found {
				t.Fatal("sent item missing")
			}
		})
	}
	for _, origin := range []string{"http://app.example", "https://app.example/path", "https://app.example//", "https://user@app.example", "https://app.example/?query=x", "https://app.example/#fragment"} {
		if _, e := NewPushRuntime(ctx, db, authority, RuntimeOptions{Secrets: options, Owner: "worker", DeepLinkOrigin: origin}); !errors.Is(e, ErrInvalid) {
			t.Fatalf("invalid origin %q admitted: %v", origin, e)
		}
	}
	for _, link := range []string{"https://app.example//notifications/id", "http://app.example/notifications/id", "https://user@app.example/notifications/id", "https://app.example/other/id"} {
		payload := Payload{Summary: "notification", IdempotencyKey: "id", AuthenticatedDeepLink: link}
		for _, adapter := range []ChannelAdapter{&APNS{}, &FCM{}, &WebPush{}} {
			if _, e := adapter.Send(ctx, PushTarget{Provider: adapter.Provider(), Status: "active"}, payload); !errors.Is(e, ErrInvalid) {
				t.Fatalf("%s accepted invalid link %q: %v", adapter.Provider(), link, e)
			}
		}
	}
}

func TestRepairM_958_RuntimePositivePath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	dataDir := t.TempDir()
	db, closeDB := repair958Open(t, dataDir)
	repair958Schema(t, db)
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	if _, e := db.Exec(`CREATE SCHEMA IF NOT EXISTS indexer;
 CREATE TABLE IF NOT EXISTS indexer.indexer_canonical_creations_v1(pool text PRIMARY KEY,token text,creator text,height bigint,block_hash text,transaction_hash text,log_index bigint,block_timestamp bigint);
 CREATE SCHEMA IF NOT EXISTS pnl;
 CREATE TABLE IF NOT EXISTS pnl.inventory_l16_wallet_ranges_v3(snapshot_id uuid PRIMARY KEY,wallet text,generation text,start_at timestamptz,end_at timestamptz,input_digest text,watermark jsonb);
 CREATE TABLE IF NOT EXISTS pnl.inventory_l16_wallet_intervals_v3(snapshot_id uuid REFERENCES pnl.inventory_l16_wallet_ranges_v3,interval text,bucket timestamptz,payload jsonb);`); e != nil {
		t.Fatal(e)
	}
	pool := "0x3333333333333333333333333333333333333333"
	if _, e := db.Exec(`INSERT INTO indexer.indexer_canonical_creations_v1 VALUES($1,$2,$3,10,$4,$5,0,$6)`, pool, "0x4444444444444444444444444444444444444444", peer.account.Wallet, "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64), time.Now().Add(-time.Minute).Unix()); e != nil {
		t.Fatal(e)
	}
	config := RuntimeConfig{Principal: peer.principal, Secrets: options, Client: peer.client, D11Base: peer.server.URL, D12Base: peer.server.URL, D11TokenRef: "REPAIR958_D11", D12TokenRef: "REPAIR958_D12", CursorKeyRef: "REPAIR958_CURSOR", InternalTokenRef: "REPAIR958_INTERNAL", DeletePeerDNS: "delete.example", ChatPeerDNS: "chat.example"}
	auth := func(r *http.Request) (string, error) { p, e := peer.principal(r); return p.AccountID, e }
	construct := func() *HTTPRuntime {
		t.Helper()
		var e error
		config.Pools, config.Intervals, e = NewRuntimeAuthorities(ctx, db, config, "0x5555555555555555555555555555555555555555")
		if e != nil {
			t.Fatal(e)
		}
		runtime, e := NewHTTPRuntime(ctx, db, auth, config)
		if e != nil {
			t.Fatal(e)
		}
		if !errors.Is(runtime.Ready(ctx), ErrUnavailable) {
			t.Fatal("stopped runtime ready")
		}
		return runtime
	}
	runtime := construct()
	withRetiredPnL := config
	withRetiredPnL.PnLBase = peer.server.URL
	if _, e := NewHTTPRuntime(ctx, db, auth, withRetiredPnL); !errors.Is(e, ErrInvalid) {
		t.Fatalf("retired PnL authority admitted: %v", e)
	}
	for _, missing := range []string{"principal", "pools", "intervals"} {
		bad := config
		switch missing {
		case "principal":
			bad.Principal = nil
		case "pools":
			bad.Pools = nil
		case "intervals":
			bad.Intervals = nil
		}
		if _, e := NewHTTPRuntime(ctx, db, auth, bad); !errors.Is(e, ErrInvalid) {
			t.Fatalf("missing %s admitted: %v", missing, e)
		}
	}
	source, e := runtime.Discovery.Feed.Sources.ResolvePublic(ctx, "pool", pool)
	if e != nil || source.Ref.ID != pool || source.Ref.SourceRevision != 10 {
		t.Fatalf("real canonical authority: %+v %v", source, e)
	}
	if e = runtime.refreshSources(ctx); e != nil {
		t.Fatal(e)
	}
	stored, e := runtime.Discovery.Store.Source(ctx, pool)
	if e != nil || !reflect.DeepEqual(stored, source.Ref) {
		t.Fatalf("source projection: %+v %v", stored, e)
	}
	produced, created, e := runtime.Notifications.Store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: peer.account.AccountID, TriggerEventID: "repair958-crossing", TemplateVersion: "market-crossing-v2"}, Category: "watchlist", Summary: "A watched pool crossed your threshold", ResourceID: pool, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}, nil)
	if e != nil || !created {
		t.Fatalf("runtime inbox producer: %+v %v %v", produced, created, e)
	}
	page, e := runtime.Notifications.Store.ListPage(ctx, peer.account.AccountID, false, "", 50)
	if e != nil || len(page.Items) != 1 {
		t.Fatalf("producer inbox: %+v %v", page, e)
	}
	id := page.Items[0].V2ID
	dependencyClosed := false
	start := func(r *HTTPRuntime) (*httptest.Server, func()) {
		t.Helper()
		run, cancelRun := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- r.Run(run) }()
		deadline := time.Now().Add(5 * time.Second)
		for r.Ready(ctx) != nil && time.Now().Before(deadline) {
			select {
			case e := <-done:
				t.Fatalf("runtime exited before ready: %v", e)
			default:
			}
			time.Sleep(10 * time.Millisecond)
		}
		if e := r.Ready(ctx); e != nil {
			t.Fatal("positive runtime readiness:", e)
		}
		server := httptest.NewServer(r.Handler())
		var stopped sync.Once
		stop := func() {
			stopped.Do(func() {
				server.Close()
				cancelRun()
				select {
				case e := <-done:
					if !errors.Is(e, context.Canceled) && !(dependencyClosed && e != nil && strings.Contains(e.Error(), "database is closed")) {
						t.Error("runtime stop:", e)
					}
				case <-time.After(5 * time.Second):
					t.Error("runtime did not stop")
				}
				if !errors.Is(r.Ready(ctx), ErrUnavailable) {
					t.Error("stopped runtime remained ready")
				}
			})
		}
		t.Cleanup(stop)
		return server, stop
	}
	server, stop := start(runtime)
	request := func(method, path, token string, status int, headers http.Header) []byte {
		t.Helper()
		req, e := http.NewRequestWithContext(ctx, method, server.URL+path, nil)
		if e != nil {
			t.Fatal(e)
		}
		req.Header = headers.Clone()
		if req.Header == nil {
			req.Header = make(http.Header)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("X-Request-Id", notificationErrorRequestID)
		response, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		raw, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		if response.StatusCode != status {
			t.Fatalf("%s %s: %d want %d: %s", method, path, response.StatusCode, status, raw)
		}
		return raw
	}
	list := func(unread bool) v2.NotificationsResponse {
		t.Helper()
		path := "/v2/me/notifications"
		if unread {
			path += "?unreadOnly=true"
		}
		raw := request("GET", path, "repair958-session", 200, nil)
		out, e := v2.DecodeNotificationsResponse(raw)
		if e != nil {
			t.Fatalf("frozen list consumer: %s %v", raw, e)
		}
		if out.RequestId != notificationErrorRequestID || out.Quality != "complete" {
			t.Fatalf("list envelope: %+v", out)
		}
		return out
	}
	before := list(false)
	if len(before.Data.Items) != 1 || before.Data.Items[0].NotificationId != id || before.Data.Items[0].Wallet != peer.account.Wallet || before.Data.Items[0].ReadAt != nil {
		t.Fatalf("authenticated list: %+v", before)
	}
	readRaw := request("PUT", "/v2/me/notifications/"+id+"/read", "repair958-session", 200, nil)
	read, e := v2.DecodeNotificationReadResponse(readRaw)
	if e != nil || read.Data.ReadAt == "" {
		t.Fatalf("frozen read consumer: %s %v", readRaw, e)
	}
	if unread := list(true); len(unread.Data.Items) != 0 {
		t.Fatalf("read item remained unread: %+v", unread)
	}
	for _, method := range []string{"GET", "PUT"} {
		path := "/v2/me/notifications"
		if method == "PUT" {
			path += "/" + id + "/read"
		}
		for _, token := range []string{"", "invalid-session"} {
			raw := request(method, path, token, 401, nil)
			if _, e := v2.DecodeError(raw); e != nil {
				t.Fatal(e)
			}
		}
		request(method, path, "repair958-session", 403, http.Header{"X-Actor-Account": []string{uuid.NewString()}})
	}
	if peer.calls.Load() < 4 {
		t.Fatal("authenticated operations bypassed real identity authority")
	}
	recoveredPage, e := runtime.Notifications.Store.ListPage(ctx, peer.account.AccountID, false, "", 50)
	if e != nil {
		t.Fatal(e)
	}
	stop()
	closeDB()
	if e = db.PingContext(ctx); e == nil {
		t.Fatal("closed store remained usable")
	}
	db, closeDB = repair958Open(t, dataDir)
	runtime = construct()
	after, e := runtime.Notifications.Store.ListPage(ctx, peer.account.AccountID, false, "", 50)
	if e != nil || !reflect.DeepEqual(after, recoveredPage) {
		t.Fatalf("reopened inbox: %+v want %+v: %v", after, recoveredPage, e)
	}
	recoveredSource, e := runtime.Discovery.Store.Source(ctx, pool)
	if e != nil || !reflect.DeepEqual(recoveredSource, stored) {
		t.Fatalf("reopened source: %+v %v", recoveredSource, e)
	}
	replay, created, e := runtime.Notifications.Store.CommitTrigger(ctx, produced, nil)
	if e != nil || created || replay.ID != produced.ID || !replay.Read {
		t.Fatalf("recovered producer replay: %+v %v %v", replay, created, e)
	}
	server, stop = start(runtime)
	defer stop()
	recovered := list(false)
	if len(recovered.Data.Items) != 1 || recovered.Data.Items[0].ReadAt == nil || *recovered.Data.Items[0].ReadAt != read.Data.ReadAt {
		t.Fatalf("HTTP recovered first read: %+v", recovered)
	}
	again, e := v2.DecodeNotificationReadResponse(request("PUT", "/v2/me/notifications/"+id+"/read", "repair958-session", 200, nil))
	if e != nil || again.Data.ReadAt != read.Data.ReadAt {
		t.Fatalf("reopened idempotent read: %+v %v", again, e)
	}
	dependencyClosed = true
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	if e = runtime.Ready(ctx); e == nil {
		t.Fatal("closed dependency remained ready")
	}
	request("GET", "/v2/me/notifications", "repair958-session", 503, nil)
}

package notifications

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestProviderClientsSendOverVerifiedTLS(t *testing.T) {
	ctx := context.Background()
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	pki := testpki.New(t)
	var oauthCalls, pushCalls, webStatus, oauthStatus atomic.Int64
	var apnsMissingReceipt, fcmMissingReceipt atomic.Bool
	oauth := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("OAuth request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		oauthCalls.Add(1)
		if oauthStatus.Load() == http.StatusServiceUnavailable {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{"access_token":"signed-access"}`); err != nil {
			t.Error(err)
		}
	}))
	oauth.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.Leaf("oauth", "oauth2.googleapis.com").Pair}}
	oauth.StartTLS()
	defer oauth.Close()
	provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/projects/fixture-project/messages:send":
			var body struct {
				Message struct {
					Token string
					Data  map[string]string
				}
			}
			if r.Header.Get("Authorization") != "Bearer signed-access" || json.NewDecoder(r.Body).Decode(&body) != nil || body.Message.Token != strings.Repeat("a", 64) || body.Message.Data["notificationId"] != "notification-1" {
				t.Errorf("FCM message: %+v %v", body, r.Header)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			pushCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if fcmMissingReceipt.Load() {
				if _, err := io.WriteString(w, `{}`); err != nil {
					t.Error(err)
				}
				return
			}
			if _, err := io.WriteString(w, `{"name":"projects/fixture-project/messages/1"}`); err != nil {
				t.Error(err)
			}
		case "/fcm/send/installation":
			body, err := io.ReadAll(r.Body)
			if err != nil || len(body) < 100 || r.Header.Get("Content-Encoding") != "aes128gcm" || !strings.HasPrefix(r.Header.Get("Authorization"), "vapid t=") {
				t.Errorf("encrypted Web Push: size=%d err=%v headers=%v", len(body), err, r.Header)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			pushCalls.Add(1)
			if status := webStatus.Load(); status != 0 {
				w.WriteHeader(int(status))
				return
			}
			w.Header().Set("Location", "receipt-1")
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("provider path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	provider.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.Leaf("push", "fcm.googleapis.com").Pair}}
	provider.StartTLS()
	defer provider.Close()
	apnsServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/3/device/"+strings.Repeat("a", 64) || r.Header.Get("apns-topic") != "com.example.app" || !strings.HasPrefix(r.Header.Get("Authorization"), "bearer ") {
			t.Errorf("APNS request: %s %s %v", r.Method, r.URL, r.Header)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !apnsMissingReceipt.Load() {
			w.Header().Set("apns-id", "apns-receipt")
		}
		w.WriteHeader(http.StatusOK)
	}))
	apnsServer.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.Leaf("apns", "api.sandbox.push.apple.com").Pair}}
	apnsServer.StartTLS()
	defer apnsServer.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.Roots()}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if host == "oauth2.googleapis.com" {
			return (&net.Dialer{}).DialContext(ctx, network, oauth.Listener.Addr().String())
		}
		if host == "api.sandbox.push.apple.com" {
			return (&net.Dialer{}).DialContext(ctx, network, apnsServer.Listener.Addr().String())
		}
		return (&net.Dialer{}).DialContext(ctx, network, provider.Listener.Addr().String())
	}}
	client := &http.Client{Timeout: 5 * time.Second, Transport: transport}
	defer client.CloseIdleConnections()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	accountJSON, err := json.Marshal(serviceAccount{Type: "service_account", Project: "fixture-project", Email: "push@example.test", Key: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})), TokenURI: "https://oauth2.googleapis.com/token"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RELEASE_FCM_ACCOUNT", string(accountJSON))
	if _, _, err := fcmAccount(`not-json`); err == nil {
		t.Fatal("malformed service account accepted")
	}
	invalidAccount := serviceAccount{Type: "service_account", Project: "fixture-project", Email: "push@example.test", Key: "invalid-pem", TokenURI: "https://oauth2.googleapis.com/token"}
	invalidJSON, err := json.Marshal(invalidAccount)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := fcmAccount(string(invalidJSON)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid service key accepted: %v", err)
	}
	if _, err := NewFCM(nil, nil, "RELEASE_FCM_ACCOUNT"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("FCM without authority admitted: %v", err)
	}
	credentials, err := NewCredentials(ctx, options, "REPAIR958_D11", "RELEASE_FCM_ACCOUNT")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	peer.target.Provider = providerFCM
	fcm, err := NewFCM(credentials, authority, "RELEASE_FCM_ACCOUNT")
	if err != nil {
		t.Fatal(err)
	}
	fcm.client = client
	if _, err := NewFCM(credentials, authority, "missing-service-account"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing FCM credential admitted: %v", err)
	}
	payload := Payload{Summary: "New notification", AuthenticatedDeepLink: "https://example.test/notifications/notification-1", IdempotencyKey: "notification-1"}
	got, err := fcm.Send(ctx, peer.target, payload)
	if err != nil || got.State != stateAccepted || got.Receipt != "projects/fixture-project/messages/1" || oauthCalls.Load() != 1 {
		t.Fatalf("FCM delivery: %+v, %v, oauth=%d", got, err, oauthCalls.Load())
	}
	if got, err := fcm.Send(ctx, PushTarget{}, payload); err == nil || got.State != stateUnknown {
		t.Fatalf("invalid FCM target: %+v %v", got, err)
	}
	if fcm.MaxPayloadBytes() != 4096 {
		t.Fatal("FCM payload bound changed")
	}
	if _, _, err := fcmAccount(`{"type":"service_account"}`); !errors.Is(err, ErrInvalid) {
		t.Fatalf("incomplete service account accepted: %v", err)
	}
	peer.endpoint.Token = strings.Repeat("x", 4097)
	if got, err := fcm.Send(ctx, peer.target, payload); !errors.Is(err, ErrInvalid) || got.State != stateUnknown {
		t.Fatalf("oversized provider token: %+v %v", got, err)
	}
	peer.endpoint = Endpoint{}
	oauthStatus.Store(http.StatusServiceUnavailable)
	if got, err := fcm.Send(ctx, peer.target, payload); err != nil || got.State != stateRetry {
		t.Fatalf("OAuth outage retry: %+v %v", got, err)
	}
	oauthStatus.Store(0)
	fcmMissingReceipt.Store(true)
	if got, err := fcm.Send(ctx, peer.target, payload); !errors.Is(err, ErrUnavailable) || got.State != stateUnknown {
		t.Fatalf("missing FCM receipt: %+v %v", got, err)
	}
	fcmMissingReceipt.Store(false)
	webKey := repair958PEM(t)
	invalidAccount.Key = webKey
	invalidJSON, err = json.Marshal(invalidAccount)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := fcmAccount(string(invalidJSON)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("non-RSA service key accepted: %v", err)
	}
	if _, err := NewWebPush(nil, nil, "RELEASE_VAPID_KEY", "RELEASE_VAPID_SUBJECT"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Web Push without authority admitted: %v", err)
	}
	t.Setenv("RELEASE_VAPID_KEY", webKey)
	t.Setenv("RELEASE_VAPID_SUBJECT", "mailto:push@example.test")
	credentials, err = NewCredentials(ctx, options, "REPAIR958_D11", "RELEASE_VAPID_KEY", "RELEASE_VAPID_SUBJECT")
	if err != nil {
		t.Fatal(err)
	}
	peer.target.Provider = providerWebPush
	userKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	peer.endpoint = Endpoint{URL: "https://fcm.googleapis.com/fcm/send/installation", PublicKey: base64.RawURLEncoding.EncodeToString(userKey.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")), Permission: "granted"}
	authority, err = NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	web, err := NewWebPush(credentials, authority, "RELEASE_VAPID_KEY", "RELEASE_VAPID_SUBJECT")
	if err != nil {
		t.Fatal(err)
	}
	web.client = client
	if _, err := NewWebPush(credentials, authority, "missing-vapid-key", "RELEASE_VAPID_SUBJECT"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing VAPID key admitted: %v", err)
	}
	t.Setenv("RELEASE_VAPID_SUBJECT", "invalid-subject")
	if err := credentials.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := NewWebPush(credentials, authority, "RELEASE_VAPID_KEY", "RELEASE_VAPID_SUBJECT"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbound VAPID subject admitted: %v", err)
	}
	t.Setenv("RELEASE_VAPID_SUBJECT", "mailto:push@example.test")
	if err := credentials.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = web.Send(ctx, peer.target, payload)
	if err != nil || got.State != stateAccepted || got.Receipt != "receipt-1" || pushCalls.Load() != 3 {
		t.Fatalf("Web Push delivery: %+v, %v, sends=%d", got, err, pushCalls.Load())
	}
	if got, err := web.Send(ctx, PushTarget{}, payload); err == nil || got.State != stateUnknown {
		t.Fatalf("invalid Web Push target: %+v %v", got, err)
	}
	peer.endpoint.Permission = "denied"
	if got, err := web.Send(ctx, peer.target, payload); !errors.Is(err, ErrForbidden) || got.State != stateUnknown {
		t.Fatalf("denied subscription: %+v %v", got, err)
	}
	peer.endpoint.Permission = "granted"
	validPublicKey := peer.endpoint.PublicKey
	peer.endpoint.PublicKey = "invalid-key"
	if got, err := web.Send(ctx, peer.target, payload); !errors.Is(err, ErrInvalid) || got.State != stateUnknown {
		t.Fatalf("invalid subscription key: %+v %v", got, err)
	}
	peer.endpoint.PublicKey = validPublicKey
	validAuth := peer.endpoint.Auth
	peer.endpoint.Auth = "invalid-auth"
	if got, err := web.Send(ctx, peer.target, payload); !errors.Is(err, ErrInvalid) || got.State != stateUnknown {
		t.Fatalf("invalid subscription auth: %+v %v", got, err)
	}
	peer.endpoint.Auth = validAuth
	webStatus.Store(http.StatusServiceUnavailable)
	if got, err := web.Send(ctx, peer.target, payload); err != nil || got.State != stateRetry {
		t.Fatalf("provider outage retry: %+v %v", got, err)
	}
	webStatus.Store(http.StatusBadRequest)
	if got, err := web.Send(ctx, peer.target, payload); err != nil || got.State != stateRejected || got.DisableTarget {
		t.Fatalf("provider refusal: %+v %v", got, err)
	}
	webStatus.Store(0)
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	t.Setenv("RELEASE_APNS_KEY", webKey)
	t.Setenv("RELEASE_APNS_KEY_ID", "KEY123")
	t.Setenv("RELEASE_APNS_TEAM_ID", "TEAM123")
	t.Setenv("RELEASE_APNS_TOPIC", "com.example.app")
	pushRuntime, err := NewPushRuntime(ctx, db, authority, RuntimeOptions{Secrets: options, Owner: "push-worker", DeepLinkOrigin: "https://example.test", EnableAPNS: true, EnableFCM: true, EnableWebPush: true, APNSKey: "RELEASE_APNS_KEY", APNSKeyID: "RELEASE_APNS_KEY_ID", APNSTeamID: "RELEASE_APNS_TEAM_ID", APNSTopic: "RELEASE_APNS_TOPIC", FCMServiceAccount: "RELEASE_FCM_ACCOUNT", VAPIDKey: "RELEASE_VAPID_KEY", VAPIDSubject: "RELEASE_VAPID_SUBJECT"})
	if err != nil || len(pushRuntime.Worker.Adapters) != 3 {
		t.Fatalf("push adapters: %+v %v", pushRuntime, err)
	}
	apnsCredentials, err := NewCredentials(ctx, options, "REPAIR958_D11", "RELEASE_APNS_KEY", "RELEASE_APNS_KEY_ID", "RELEASE_APNS_TEAM_ID", "RELEASE_APNS_TOPIC")
	if err != nil {
		t.Fatal(err)
	}
	apnsAuthority, err := NewAccountClient(peer.server.URL, peer.client, apnsCredentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	apns, err := NewAPNS(apnsCredentials, apnsAuthority, "RELEASE_APNS_KEY", "RELEASE_APNS_KEY_ID", "RELEASE_APNS_TEAM_ID", "RELEASE_APNS_TOPIC", true)
	if err != nil {
		t.Fatal(err)
	}
	apns.client = client
	if _, err := NewAPNS(apnsCredentials, apnsAuthority, "missing-apns-key", "RELEASE_APNS_KEY_ID", "RELEASE_APNS_TEAM_ID", "RELEASE_APNS_TOPIC", true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing APNS key admitted: %v", err)
	}
	peer.target.Provider = providerAPNS
	savedEndpoint := peer.endpoint
	peer.endpoint = Endpoint{Token: strings.Repeat("a", 64)}
	got, err = apns.Send(ctx, peer.target, payload)
	if err != nil || got.State != stateAccepted || got.Receipt != "apns-receipt" {
		t.Fatalf("APNS delivery: %+v %v", got, err)
	}
	apnsMissingReceipt.Store(true)
	if got, err := apns.Send(ctx, peer.target, payload); !errors.Is(err, ErrUnavailable) || got.State != stateUnknown {
		t.Fatalf("missing APNS receipt: %+v %v", got, err)
	}
	apnsMissingReceipt.Store(false)
	if _, err := NewAPNS(nil, nil, "", "", "", "", true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("APNS without authority admitted: %v", err)
	}
	peer.endpoint.Token = "invalid-token"
	if got, err := apns.Send(ctx, peer.target, payload); !errors.Is(err, ErrInvalid) || got.State != stateUnknown {
		t.Fatalf("invalid APNS token: %+v %v", got, err)
	}
	if got, err := apns.Send(ctx, PushTarget{}, payload); !errors.Is(err, ErrInvalid) || got.State != stateUnknown {
		t.Fatalf("invalid APNS target: %+v %v", got, err)
	}
	peer.target.Provider = providerWebPush
	peer.endpoint = savedEndpoint
	store := Store{DB: db}
	now := time.Now().UTC()
	item, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: peer.account.AccountID, TriggerEventID: "worker-event", TemplateVersion: "v2"}, Category: categoryWatchlist, Summary: "Watched pool changed", CreatedAt: now}, []DeliveryAttempt{{Channel: channelPush, TargetID: peer.target.DeviceInstallationID, TargetRevision: peer.target.Revision, ConsentRevision: peer.target.ConsentRevision}})
	if err != nil || !created {
		t.Fatalf("worker trigger: %+v %v %v", item, created, err)
	}
	observed := 0
	worker := Worker{Store: store, Authority: authority, Adapters: map[string]ChannelAdapter{providerWebPush: web}, Owner: "push-worker", DeepLinkOrigin: "https://example.test", Observe: func(provider string, elapsed time.Duration, state string) {
		if provider != providerWebPush || elapsed < 0 || state == "" {
			t.Errorf("delivery observation: %s %s %s", provider, elapsed, state)
		}
		observed++
	}}
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRowContext(ctx, `SELECT deliveries->0->>'State' FROM discovery_notifications.outbox WHERE id=$1`, item.ID).Scan(&state); err != nil || state != stateAccepted {
		t.Fatalf("persisted delivery: %s %v", state, err)
	}
	webStatus.Store(http.StatusGone)
	rejected, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: peer.account.AccountID, TriggerEventID: "revoked-subscription", TemplateVersion: "v2"}, Category: categoryWatchlist, Summary: "Watched pool changed", CreatedAt: time.Now().UTC()}, []DeliveryAttempt{{Channel: channelPush, TargetID: peer.target.DeviceInstallationID, TargetRevision: peer.target.Revision, ConsentRevision: peer.target.ConsentRevision}})
	if err != nil || !created {
		t.Fatalf("rejected trigger: %+v %v %v", rejected, created, err)
	}
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT deliveries->0->>'State' FROM discovery_notifications.outbox WHERE id=$1`, rejected.ID).Scan(&state); err != nil || state != stateRejected {
		t.Fatalf("rejected delivery: %s %v", state, err)
	}
	if observed != 2 {
		t.Fatalf("delivery observations: %d", observed)
	}
	webStatus.Store(0)
	provider.Close()
	uncertain, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: peer.account.AccountID, TriggerEventID: "provider-interruption", TemplateVersion: "v2"}, Category: categoryWatchlist, Summary: "Watched pool changed", CreatedAt: time.Now().UTC()}, []DeliveryAttempt{{Channel: channelPush, TargetID: peer.target.DeviceInstallationID, TargetRevision: peer.target.Revision, ConsentRevision: peer.target.ConsentRevision}})
	if err != nil || !created {
		t.Fatalf("interrupted trigger: %+v %v %v", uncertain, created, err)
	}
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT deliveries->0->>'State' FROM discovery_notifications.outbox WHERE id=$1`, uncertain.ID).Scan(&state); err != nil || state != stateUnknown {
		t.Fatalf("uncertain delivery: %s %v", state, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := worker.Run(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled worker: %v", err)
	}
	if err := worker.DeleteAccount(ctx, peer.account.AccountID); err != nil {
		t.Fatal(err)
	}
	items, err := store.List(ctx, peer.account.AccountID, 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("deleted account inbox: %+v %v", items, err)
	}
}

func TestProviderClientRefusesLoopbackDestination(t *testing.T) {
	client := providerClient()
	defer client.CloseIdleConnections()
	dial := client.Transport.(*http.Transport).DialContext
	if _, err := dial(context.Background(), "tcp", "missing-port"); err == nil {
		t.Fatal("malformed provider destination accepted")
	}
	if _, err := dial(context.Background(), "tcp", "localhost:443"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("loopback provider destination: %v", err)
	}
}

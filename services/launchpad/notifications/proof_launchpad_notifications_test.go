// Proves: Store.CommitTrigger/SetReadV2/ListPage -> HTTPRuntime.Handler/HTTP.Handler/AdaptV2Notifications/writeError -> v2.DecodeNotificationsResponse/DecodeNotificationReadResponse/DecodeError.
// The consumer uses real PostgreSQL and HTTPS with verified client certificates;
// its authenticated account is the client certificate's subject. Closing the
// server and database between production and consumption proves durable recovery.
// The proof starts its own isolated PostgreSQL cluster using pg_config's binaries;
// no externally provisioned database or environment configuration is required.
// Repairs: store.go SetReadV2 and http.go readV2Notification now update and return
// the first-read timestamp atomically, removing the lookup/deletion race;
// v2_wire.go AdaptV2Notifications now exposes ErrInvalid for inconsistent read
// state instead of an untyped error. errors.go writeError shares request-ID
// normalization with successful responses. The exported signatures exercised here
// are the existing Go surface, including SetReadV2's string ID and read flag.
package notifications

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestProofLaunchpadNotifications(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dsn := proofNotificationsPostgres(t, ctx)
	account, other := uuid.NewString(), uuid.NewString()
	const wallet = "0x0123456789abcdef0123456789abcdef01234567"
	requestID := uuid.NewString()
	var store Store
	var server *httptest.Server
	var client, foreignClient *http.Client
	open := func() {
		t.Helper()
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal(err)
		}
		store = Store{DB: db}
		if err = db.PingContext(ctx); err != nil {
			t.Fatal(err)
		}
	}
	stop := func() {
		t.Helper()
		if server != nil {
			client.CloseIdleConnections()
			foreignClient.CloseIdleConnections()
			server.Close()
			server = nil
		}
		if err := store.DB.Close(); err != nil {
			t.Fatal(err)
		}
		if err := store.DB.PingContext(ctx); !errors.Is(err, sql.ErrConnDone) && (err == nil || err.Error() != "sql: database is closed") {
			t.Fatalf("producer database remained usable after close: %v", err)
		}
	}
	open()
	t.Cleanup(func() {
		if server != nil {
			server.Close()
		}
		defer store.DB.Close()
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		for _, actor := range []string{account, other} {
			if err := store.DeleteAccount(cleanup, actor); err != nil {
				t.Error("delete proof account:", err)
			}
		}
	})
	if _, err := store.DB.ExecContext(ctx, Schema); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UTC().Truncate(time.Second).Add(123456789 * time.Nanosecond)
	var produced []InboxItem
	for _, event := range []string{"crossing-a", "crossing-b", "crossing-c"} {
		item, created, err := store.CommitTrigger(ctx, InboxItem{
			Identity: Identity{AccountID: account, TriggerEventID: event, TemplateVersion: "market-crossing-v2"},
			Category: "watchlist", Summary: "A watched pool crossed your threshold",
			ResourceID: event, CreatedAt: createdAt,
		}, nil)
		if err != nil || !created {
			t.Fatalf("produce %s: created=%v err=%v", event, created, err)
		}
		produced = append(produced, item)
	}
	sort.Slice(produced, func(i, j int) bool { return produced[i].ID < produced[j].ID })
	foreign, created, err := store.CommitTrigger(ctx, InboxItem{
		Identity: Identity{AccountID: other, TriggerEventID: "foreign", TemplateVersion: "v2"},
		Category: "reward", Summary: "Other account", ResourceID: "foreign-reward", CreatedAt: createdAt.Add(time.Hour),
	}, nil)
	if err != nil || !created || foreign.Identity.AccountID != other {
		t.Fatalf("foreign producer: item=%+v created=%v err=%v", foreign, created, err)
	}
	page, err := store.ListPage(ctx, account, false, "", 100)
	if err != nil || len(page.Items) != len(produced) || page.NextCursor != "" {
		t.Fatalf("producer page=%+v err=%v", page, err)
	}
	for i, item := range page.Items {
		if item.InboxItem != produced[i] || item.V2ID != expectedV2ID(produced[i].ID, produced[i].CreatedAt) || item.ReadAt != nil || item.DeliveryState != "sent" {
			t.Fatalf("producer item %d: %+v", i, item)
		}
	}
	if err = store.SetReadV2(ctx, account, page.Items[0].V2ID, true); err != nil {
		t.Fatal("producer read:", err)
	}
	before, err := store.ListPage(ctx, account, false, "", 100)
	if err != nil || len(before.Items) != 3 || before.Items[0].ReadAt == nil || !before.Items[0].Read {
		t.Fatalf("producer read state=%+v err=%v", before, err)
	}
	stop()
	open()
	recovered, err := store.ListPage(ctx, account, false, "", 100)
	if err != nil || !reflect.DeepEqual(recovered, before) {
		t.Fatalf("restart changed persisted page: got=%+v want=%+v err=%v", recovered, before, err)
	}
	// Replay the real producer after restart with a changed payload: the original
	// document, including the consumer's read state, must win the identity conflict.
	replay := produced[0]
	replay.Summary = "Replay must not overwrite the original"
	duplicate, created, err := store.CommitTrigger(ctx, replay, nil)
	if err != nil || created || duplicate != recovered.Items[0].InboxItem {
		t.Fatalf("replay overwrote durable state: item=%+v created=%v err=%v", duplicate, created, err)
	}
	start := func() {
		t.Helper()
		server, client, foreignClient = proofNotificationsServer(t, store, account, other)
	}
	start()
	request := func(c *http.Client, method, path, trace string) (int, []byte) {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, method, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("X-Actor-Wallet", wallet)
		if trace != "" {
			r.Header.Set("X-Request-Id", trace)
		}
		response, err := c.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" || response.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("consumer headers: %v", response.Header)
		}
		return response.StatusCode, raw
	}
	list := func(path string) v2.NotificationsResponse {
		t.Helper()
		status, raw := request(client, http.MethodGet, path, requestID)
		got, err := v2.DecodeNotificationsResponse(raw)
		if status != http.StatusOK || err != nil || got.RequestId != requestID || got.Quality != "complete" || got.Watermark != nil || got.Revision != nil || got.GeneratedAt == "" {
			t.Fatalf("consumer list: status=%d body=%s err=%v", status, raw, err)
		}
		return got
	}
	first := list("/v2/me/notifications?limit=2")
	if len(first.Data.Items) != 2 || first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatalf("consumer first page: %+v", first)
	}
	second := list("/v2/me/notifications?limit=2&cursor=" + url.QueryEscape(*first.NextCursor))
	if len(second.Data.Items) != 1 || second.NextCursor != nil {
		t.Fatalf("consumer last page: %+v", second)
	}
	observed := append(first.Data.Items, second.Data.Items...)
	for i, item := range observed {
		want := recovered.Items[i]
		if item.NotificationId != want.V2ID || item.Wallet != wallet || item.EntityId != want.ResourceID || item.Kind != "price_alert" || item.CreatedAt != want.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z") || item.DeliveryState != "sent" {
			t.Fatalf("consumer item %d: got=%+v persisted=%+v", i, item, want)
		}
		if i == 0 {
			if item.ReadAt == nil || *item.ReadAt != want.ReadAt.UTC().Format("2006-01-02T15:04:05.000Z") {
				t.Fatalf("consumer lost first read time: %+v", item)
			}
		} else if item.ReadAt != nil {
			t.Fatalf("consumer unexpectedly read item: %+v", item)
		}
	}
	// The exported adapter receives actual Store.ListPage output and the frozen
	// consumer must see the same body as the HTTP crossing (apart from generation).
	wire := V2NotificationsPage{}
	for _, item := range recovered.Items {
		wire.Items = append(wire.Items, V2NotificationItem{InboxItem: item.InboxItem, V2ID: item.V2ID, Wallet: wallet, ReadAt: item.ReadAt, DeliveryState: item.DeliveryState})
	}
	raw, err := AdaptV2Notifications(requestID, wire, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	adapted, err := v2.DecodeNotificationsResponse(raw)
	if err != nil || !reflect.DeepEqual(adapted.Data.Items, observed) {
		t.Fatalf("exported adapter differs from HTTP consumer: %+v err=%v", adapted, err)
	}
	readPath := "/v2/me/notifications/" + strings.ToUpper(observed[1].NotificationId) + "/read"
	read := func() string {
		t.Helper()
		status, raw := request(client, http.MethodPut, readPath, requestID)
		got, err := v2.DecodeNotificationReadResponse(raw)
		if status != http.StatusOK || err != nil || got.RequestId != requestID || got.Data.ReadAt == "" || got.Quality != "complete" || got.NextCursor != nil {
			t.Fatalf("consumer read: status=%d body=%s err=%v", status, raw, err)
		}
		return got.Data.ReadAt
	}
	readAt := read()
	afterRead, err := store.ListPage(ctx, account, false, "", 100)
	if err != nil || len(afterRead.Items) != 3 || afterRead.Items[1].ReadAt == nil || readAt != afterRead.Items[1].ReadAt.UTC().Format("2006-01-02T15:04:05.000Z") {
		t.Fatalf("HTTP read differs from committed state: %+v err=%v", afterRead, err)
	}
	stop()
	open()
	start()
	if repeated := read(); repeated != readAt {
		t.Fatalf("read retry after restart changed first timestamp: %q -> %q", readAt, repeated)
	}
	persisted, err := store.ListPage(ctx, account, false, "", 100)
	if err != nil || !reflect.DeepEqual(persisted, afterRead) {
		t.Fatalf("read retry mutated durable state: %+v err=%v", persisted, err)
	}
	unread := list("/v2/me/notifications?unreadOnly=true")
	if len(unread.Data.Items) != 1 || unread.Data.Items[0].NotificationId != observed[2].NotificationId || unread.Data.Items[0].ReadAt != nil || unread.NextCursor != nil {
		t.Fatalf("consumer unread result: %+v", unread)
	}
	for _, tc := range []struct {
		client              *http.Client
		method, path, trace string
		status              int
		code                string
	}{
		{client, http.MethodPut, "/v2/me/notifications/" + uuid.NewString() + "/read", requestID, http.StatusNotFound, "NOT_FOUND"},
		{foreignClient, http.MethodPut, readPath, requestID, http.StatusNotFound, "NOT_FOUND"},
		{client, http.MethodGet, "/v2/me/notifications?cursor=malformed!", requestID, http.StatusBadRequest, "INVALID_REQUEST"},
		{client, http.MethodPut, "/v2/me/notifications/not-a-uuid/read", "invalid-trace", http.StatusBadRequest, "INVALID_REQUEST"},
		{client, http.MethodGet, "/v2/me/notifications?limit=0", "", http.StatusBadRequest, "INVALID_REQUEST"},
	} {
		status, raw := request(tc.client, tc.method, tc.path, tc.trace)
		got, err := v2.DecodeError(raw)
		if err != nil || status != tc.status || got.Code != tc.code || got.Message != http.StatusText(tc.status) || got.Retryable || got.RetryAfterMs != nil || got.Details != nil || got.FieldErrors == nil || len(got.FieldErrors) != 0 {
			t.Fatalf("consumer refusal: status=%d body=%s err=%v", status, raw, err)
		}
		if tc.trace == requestID {
			if got.RequestId != requestID {
				t.Fatalf("refusal lost request ID: %+v", got)
			}
		} else if id, err := uuid.Parse(got.RequestId); err != nil || id.Version() != 4 || id.Variant() != uuid.RFC4122 {
			t.Fatalf("refusal did not generate a request ID: %+v err=%v", got, err)
		}
	}
	final, err := store.ListPage(ctx, account, false, "", 100)
	if err != nil || !reflect.DeepEqual(final, persisted) {
		t.Fatalf("refusals mutated recipient state: %+v err=%v", final, err)
	}
	// Real InboxItem.SetRead changes the flag without supplying a durable timestamp.
	// The exported adapter must expose a typed refusal for that malformed input.
	malformed := wire
	malformed.Items = append([]V2NotificationItem(nil), wire.Items...)
	malformed.Items[2].InboxItem, err = malformed.Items[2].InboxItem.SetRead(account, true)
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := AdaptV2Notifications(requestID, malformed, createdAt); !errors.Is(err, ErrInvalid) || raw != nil {
		t.Fatalf("adapter malformed read-state refusal: body=%s err=%v", raw, err)
	}
	var extensions struct {
		Data struct {
			Items []struct {
				Summary string
				Read    bool
			}
		}
	}
	if err := json.Unmarshal(raw, &extensions); err != nil || len(extensions.Data.Items) != 3 || extensions.Data.Items[0].Summary != produced[0].Summary || !extensions.Data.Items[0].Read || extensions.Data.Items[1].Read {
		t.Fatalf("adapter lost public summary/read fields: %+v err=%v", extensions, err)
	}
	// A stopped real dependency must reach the consumer as the frozen retryable
	// error envelope, never a successful empty list or a private database error.
	if err := store.DB.Close(); err != nil {
		t.Fatal(err)
	}
	status, unavailableRaw := request(client, http.MethodGet, "/v2/me/notifications", requestID)
	open()
	unavailable, err := v2.DecodeError(unavailableRaw)
	if err != nil || status != http.StatusServiceUnavailable || unavailable.Code != "DEPENDENCY_UNAVAILABLE" || unavailable.Message != http.StatusText(status) || unavailable.RequestId != requestID || !unavailable.Retryable || unavailable.RetryAfterMs != nil || unavailable.Details != nil || unavailable.FieldErrors == nil || len(unavailable.FieldErrors) != 0 {
		t.Fatalf("consumer dependency refusal: status=%d body=%s err=%v", status, unavailableRaw, err)
	}
}

func proofNotificationsPostgres(t *testing.T, ctx context.Context) string {
	t.Helper()
	bindir, err := exec.CommandContext(ctx, "pg_config", "--bindir").Output()
	if err != nil {
		t.Fatalf("services/launchpad/notifications proof requires PostgreSQL pg_config: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp", "launchpad-notifications-proof-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error("remove isolated PostgreSQL cluster:", err)
		}
	})
	var credential *syscall.Credential
	// PostgreSQL refuses root. Give only this temporary cluster to the local
	// postgres account and run its child processes with that account's identity.
	if os.Geteuid() == 0 {
		actor, err := user.Lookup("postgres")
		if err != nil {
			t.Fatalf("services/launchpad/notifications proof requires a non-root postgres account: %v", err)
		}
		uid, err := strconv.ParseUint(actor.Uid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		gid, err := strconv.ParseUint(actor.Gid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(dir, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, filepath.Join(strings.TrimSpace(string(bindir)), name), args...)
		if credential != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		}
		return cmd.CombinedOutput()
	}
	data := filepath.Join(dir, "data")
	if output, err := run(ctx, "initdb", "-D", data, "--auth=trust", "--username=postgres", "--no-locale", "--encoding=UTF8"); err != nil {
		t.Fatalf("initialize isolated PostgreSQL producer: %v\n%s", err, output)
	}
	// A private Unix socket directory removes the need to reserve and race a TCP
	// port. Listen addresses are empty, so this cluster accepts no TCP connections.
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if output, err := run(cleanup, "pg_ctl", "-D", data, "-m", "immediate", "-w", "-t", "5", "stop"); err != nil {
			t.Errorf("stop isolated PostgreSQL producer: %v\n%s", err, output)
		}
	})
	if output, err := run(ctx, "pg_ctl", "-D", data, "-l", filepath.Join(dir, "postgres.log"), "-o", "-k "+dir+" -h '' -p 5432", "-w", "-t", "10", "start"); err != nil {
		log, logErr := os.ReadFile(filepath.Join(dir, "postgres.log"))
		t.Fatalf("start isolated PostgreSQL producer: %v\n%s\n%s (log read: %v)", err, output, log, logErr)
	}
	return "host=" + dir + " port=5432 user=postgres dbname=postgres sslmode=disable"
}

func proofNotificationsServer(t *testing.T, store Store, account, other string) (*httptest.Server, *http.Client, *http.Client) {
	t.Helper()
	now := time.Now()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "notifications-proof-ca"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	// This is the Authenticate boundary supplied by the embedding server: resolve
	// the actor only from the actual TLS verifier's authenticated certificate.
	h := HTTP{Store: store, Principal: func(r *http.Request) (Principal, error) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
			return Principal{}, ErrForbidden
		}
		return Principal{AccountID: r.TLS.PeerCertificates[0].Subject.CommonName, Wallet: "0x0123456789abcdef0123456789abcdef01234567"}, nil
	}, Authenticate: func(r *http.Request) (string, error) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
			return "", ErrForbidden
		}
		return r.TLS.PeerCertificates[0].Subject.CommonName, nil
	}}
	server := httptest.NewUnstartedServer((&HTTPRuntime{DB: store.DB, Notifications: h}).Handler())
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	server.StartTLS()
	t.Cleanup(server.Close)
	clients := make([]*http.Client, 0, 2)
	for i, actor := range []string{account, other} {
		clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		leaf := &x509.Certificate{SerialNumber: big.NewInt(int64(i + 2)), Subject: pkix.Name{CommonName: actor}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
		cert, err := x509.CreateCertificate(rand.Reader, leaf, root, &clientKey.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		transport := server.Client().Transport.(*http.Transport).Clone()
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		transport.TLSClientConfig.MinVersion = tls.VersionTLS13
		transport.TLSClientConfig.Certificates = []tls.Certificate{{Certificate: [][]byte{cert, der}, PrivateKey: clientKey}}
		client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
		t.Cleanup(client.CloseIdleConnections)
		clients = append(clients, client)
	}
	return server, clients[0], clients[1]
}

package notifications

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Reuse the real PostgreSQL fixture already required by the package gate.
// An isolated database lets the authority proof install the owner interfaces
// without touching another test's schemas. All setup uses the database driver;
// no subprocess or additional service is started by these repair tests.
func repair40Database(t *testing.T, ctx context.Context) string {
	t.Helper()
	fixture, _ := notificationDatabase(t)
	config, err := pgx.ParseConfig(os.Getenv("NOTIFICATIONS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	name := "repair40_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = fixture.DB.ExecContext(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal("create isolated repair database:", err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := fixture.DB.ExecContext(cleanup, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Error("remove isolated repair database:", err)
		}
	})
	config.Database = name
	dsn := stdlib.RegisterConnConfig(config)
	t.Cleanup(func() { stdlib.UnregisterConnConfig(dsn) })
	return dsn
}

func repair40Open(t *testing.T, ctx context.Context, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// The embedding application supplies a trusted principal, not client actor
// headers. Here that boundary is a real CA-verified TLS certificate carrying
// both identity fields; all notification handlers, persistence and codecs run
// in the test process. No identity or SDK toolchain is launched.
func repair40Principal(r *http.Request) (Principal, error) {
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
		return Principal{}, ErrForbidden
	}
	names := strings.Split(r.TLS.PeerCertificates[0].Subject.CommonName, ".")
	if len(names) != 4 || names[2] != "identity" || names[3] != "example" {
		return Principal{}, ErrForbidden
	}
	if _, err := uuid.Parse(names[0]); err != nil || !notificationWalletPattern.MatchString(names[1]) {
		return Principal{}, ErrForbidden
	}
	return Principal{AccountID: names[0], Wallet: names[1]}, nil
}

func repair40Request(t *testing.T, ctx context.Context, client *http.Client, method, endpoint string, headers http.Header, status int) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header = headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("X-Request-Id", notificationErrorRequestID)
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status {
		t.Fatalf("%s %s: status %d want %d: %s", method, endpoint, response.StatusCode, status, raw)
	}
	return raw
}

// Consume the same frozen list/read envelopes used by IF_NOTIFICATIONS and
// IF_NOTIFICATION_READ, using the generated Go validators and decoders.
func repair40Consume(t *testing.T, ctx context.Context, client *http.Client, endpoint string, account Principal, mode, id, readAt string) string {
	t.Helper()
	list := func(query string) v2.NotificationsResponse {
		t.Helper()
		raw := repair40Request(t, ctx, client, http.MethodGet, endpoint+"/v2/me/notifications"+query, nil, http.StatusOK)
		out, err := v2.DecodeNotificationsResponse(raw)
		if err != nil {
			t.Fatalf("frozen list consumer: %s: %v", raw, err)
		}
		if out.RequestId != notificationErrorRequestID || out.Quality != "complete" {
			t.Fatalf("list envelope: %+v", out)
		}
		return out
	}
	if mode == "foreign" {
		raw := repair40Request(t, ctx, client, http.MethodPut, endpoint+"/v2/me/notifications/"+id+"/read", nil, http.StatusNotFound)
		out, err := v2.DecodeError(raw)
		if err != nil || out.Code != "NOT_FOUND" {
			t.Fatalf("foreign read: %s %v", raw, err)
		}
		if page := list(""); len(page.Data.Items) != 0 {
			t.Fatalf("foreign inbox leaked: %+v", page)
		}
		return ""
	}
	page := list("")
	if len(page.Data.Items) != 1 {
		t.Fatalf("producer item missing: %+v", page)
	}
	item := page.Data.Items[0]
	if item.NotificationId != id || item.Wallet != account.Wallet || item.Kind != "price_alert" {
		t.Fatalf("consumer identity: %+v", item)
	}
	if readAt == "" {
		if item.ReadAt != nil {
			t.Fatalf("new item was read: %+v", item)
		}
	} else if item.ReadAt == nil || *item.ReadAt != readAt {
		t.Fatalf("recovered read state: %+v want %s", item, readAt)
	}
	raw := repair40Request(t, ctx, client, http.MethodPut, endpoint+"/v2/me/notifications/"+id+"/read", nil, http.StatusOK)
	read, err := v2.DecodeNotificationReadResponse(raw)
	if err != nil || read.RequestId != notificationErrorRequestID || read.Quality != "complete" || read.Data.ReadAt == "" {
		t.Fatalf("frozen read consumer: %s: %v", raw, err)
	}
	if readAt != "" && read.Data.ReadAt != readAt {
		t.Fatalf("first read changed: %s -> %s", readAt, read.Data.ReadAt)
	}
	if unread := list("?unreadOnly=true"); len(unread.Data.Items) != 0 {
		t.Fatalf("read item remained unread: %+v", unread)
	}
	return read.Data.ReadAt
}

func TestRepair_40_NotificationWalletBoundary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	dsn := repair40Database(t, ctx)
	account := Principal{AccountID: uuid.NewString(), Wallet: "0x1111111111111111111111111111111111111111"}
	other := Principal{AccountID: uuid.NewString(), Wallet: "0x2222222222222222222222222222222222222222"}
	pki := testpki.New(t)
	serverPeer := pki.Leaf("api", "api.example")
	clients := make([]*http.Client, 0, 2)
	for i, p := range []Principal{account, other} {
		peer := pki.Leaf([]string{"actor", "foreign"}[i], p.AccountID+"."+p.Wallet+".identity.example")
		client := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{peer.Pair}}}}
		t.Cleanup(client.CloseIdleConnections)
		clients = append(clients, client)
	}
	db := repair40Open(t, ctx, dsn)
	if _, err := db.ExecContext(ctx, Schema); err != nil {
		t.Fatal(err)
	}
	store := Store{DB: db}
	produced, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: account.AccountID, TriggerEventID: "repair40-real-producer", TemplateVersion: "market-crossing-v2"}, Category: "watchlist", Summary: "A watched pool crossed your threshold", ResourceID: "0x1111111111111111111111111111111111111111", CreatedAt: time.Now().UTC()}, nil)
	if err != nil || !created {
		t.Fatalf("producer: %+v %v %v", produced, created, err)
	}
	page, err := store.ListPage(ctx, account.AccountID, false, "", 50)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("producer page: %+v %v", page, err)
	}
	id := page.Items[0].V2ID
	start := func() *httptest.Server {
		server := httptest.NewUnstartedServer((&HTTPRuntime{Notifications: HTTP{Store: store, Principal: repair40Principal}}).Handler())
		server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.VerifyClientCertIfGiven, ClientCAs: pki.Roots(), Certificates: []tls.Certificate{serverPeer.Pair}}
		server.StartTLS()
		return server
	}
	server := start()
	defer func() { server.Close() }()
	readAt := repair40Consume(t, ctx, clients[0], server.URL, account, "read", id, "")
	read, err := store.ListPage(ctx, account.AccountID, false, "", 50)
	if err != nil || len(read.Items) != 1 || read.Items[0].ReadAt == nil || read.Items[0].ReadAt.UTC().Format("2006-01-02T15:04:05.000Z") != readAt {
		t.Fatalf("committed consumer read: %+v %v", read, err)
	}
	server.Close()
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if err = db.PingContext(ctx); err == nil {
		t.Fatal("closed producer database remained usable")
	}
	db = repair40Open(t, ctx, dsn)
	store = Store{DB: db}
	recovered, err := store.ListPage(ctx, account.AccountID, false, "", 50)
	if err != nil || !reflect.DeepEqual(recovered, read) {
		t.Fatalf("durable recovery: %+v want %+v: %v", recovered, read, err)
	}
	replay, created, err := store.CommitTrigger(ctx, produced, nil)
	if err != nil || created || replay.ID != produced.ID || !replay.Read {
		t.Fatalf("replay changed recovered inbox: %+v %v %v", replay, created, err)
	}
	server = start()
	if after := repair40Consume(t, ctx, clients[0], server.URL, account, "recovered", id, readAt); after != readAt {
		t.Fatalf("first read changed on restart: %s -> %s", readAt, after)
	}
	repair40Consume(t, ctx, clients[1], server.URL, other, "foreign", id, "")
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		path := "/v2/me/notifications"
		if method == http.MethodPut {
			path += "/" + id + "/read"
		}
		for _, headers := range []http.Header{
			{"X-Actor-Wallet": []string{other.Wallet}},
			{"X-Actor-Account": []string{other.AccountID}},
			{"X-Actor-Wallet": []string{account.Wallet, other.Wallet}},
			{"X-Actor-Wallet": []string{account.Wallet, account.Wallet}},
			{"X-Actor-Account": []string{account.AccountID, account.AccountID}},
		} {
			req, err := http.NewRequestWithContext(ctx, method, server.URL+path, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header = headers.Clone()
			response, err := clients[0].Do(req)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			refusal, err := v2.DecodeError(raw)
			if response.StatusCode != http.StatusForbidden || err != nil || refusal.Code != "SCOPE_DENIED" {
				t.Fatalf("spoof accepted: %d %s %v", response.StatusCode, raw, err)
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/v2/me/notifications", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Actor-Wallet", account.Wallet)
	anonymous := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots()}}}
	defer anonymous.CloseIdleConnections()
	response, err := anonymous.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("header authenticated an anonymous request: %d", response.StatusCode)
	}
	final, err := store.ListPage(ctx, account.AccountID, false, "", 50)
	if err != nil || !reflect.DeepEqual(final, recovered) {
		t.Fatalf("refusals mutated recovered state: %+v %v", final, err)
	}
}

func TestRepair_40_RuntimeAuthorities(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dsn := repair40Database(t, ctx)
	db := repair40Open(t, ctx, dsn)
	if _, err := db.ExecContext(ctx, Schema+discovery.Schema); err != nil {
		t.Fatal(err)
	}
	const pool = "0x1111111111111111111111111111111111111111"
	const quote = "0x2222222222222222222222222222222222222222"
	// The source interface is the indexer's canonical committed view; intervals
	// use the PnL owner's published L16 range and bucket tables.
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA indexer;
 CREATE TABLE indexer.indexer_canonical_creations(pool text PRIMARY KEY,token text,creator text,height bigint,block_hash text,transaction_hash text,log_index bigint,block_timestamp bigint);
 CREATE VIEW indexer.indexer_canonical_creations_v1 AS SELECT * FROM indexer.indexer_canonical_creations;
 CREATE SCHEMA pnl;
 CREATE TABLE pnl.inventory_l16_wallet_ranges_v3(snapshot_id uuid PRIMARY KEY,wallet text,generation text,start_at timestamptz,end_at timestamptz,input_digest text,watermark jsonb);
 CREATE TABLE pnl.inventory_l16_wallet_intervals_v3(snapshot_id uuid REFERENCES pnl.inventory_l16_wallet_ranges_v3,interval text,bucket timestamptz,payload jsonb);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO indexer.indexer_canonical_creations VALUES($1,$2,$3,10,$4,$5,0,$6)`, pool, quote, "0x3333333333333333333333333333333333333333", "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64), time.Now().Add(-time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"REPAIR40_D11", "REPAIR40_D12", "REPAIR40_CURSOR", "REPAIR40_INTERNAL"} {
		t.Setenv(key, strings.Repeat("k", 48))
	}
	pki := testpki.New(t)
	peer := pki.Leaf("api", "api.example")
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{peer.Pair}}}}
	defer client.CloseIdleConnections()
	principal := AuthenticatePrincipal(repair40Principal)
	config := RuntimeConfig{Principal: principal, Secrets: secrets.Options{Provider: secrets.ProviderEnv}, Client: client, D11Base: "https://accounts.example", D12Base: "https://social.example", D11TokenRef: "REPAIR40_D11", D12TokenRef: "REPAIR40_D12", CursorKeyRef: "REPAIR40_CURSOR", InternalTokenRef: "REPAIR40_INTERNAL", DeletePeerDNS: "accounts.example", ChatPeerDNS: "social.example"}
	var err error
	config.Pools, config.Intervals, err = NewRuntimeAuthorities(ctx, db, config, quote)
	if err != nil {
		t.Fatal("production authority injection:", err)
	}
	if _, ok := config.Pools.(CanonicalPools); !ok {
		t.Fatalf("non-concrete pool authority: %T", config.Pools)
	}
	if _, ok := config.Intervals.(discovery.SQLIntervalAuthority); !ok {
		t.Fatalf("non-concrete interval authority: %T", config.Intervals)
	}
	authenticate := func(r *http.Request) (string, error) { p, err := principal(r); return p.AccountID, err }
	runtime, err := NewHTTPRuntime(ctx, db, authenticate, config)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Config.Pools.(CanonicalPools).Accounts != runtime.Accounts || runtime.Config.Intervals.(discovery.SQLIntervalAuthority).Attribution != runtime.Accounts {
		t.Fatal("owned authorities do not share the running credential refresh lifecycle")
	}

	for _, absent := range []string{"pools", "intervals", "principal"} {
		missing := config
		switch absent {
		case "pools":
			missing.Pools = nil
		case "intervals":
			missing.Intervals = nil
		case "principal":
			missing.Principal = nil
		}
		if _, err := NewHTTPRuntime(ctx, db, authenticate, missing); !errors.Is(err, ErrInvalid) {
			t.Fatalf("startup admitted absent %s: %v", absent, err)
		}
	}
	if err = runtime.Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("stopped runtime ready: %v", err)
	}
	canonical, err := runtime.Config.Pools.CanonicalPool(ctx, pool)
	if err != nil || !canonical {
		t.Fatalf("canonical producer not resolved: %v %v", canonical, err)
	}
	resource, err := runtime.Discovery.Feed.Sources.ResolvePublic(ctx, "pool", pool)
	if err != nil || resource.Ref.ID != pool || resource.Ref.SourceRevision != 10 || !resource.Ref.Public || resource.Ref.EventID != "0x"+strings.Repeat("b", 64)+":0" {
		t.Fatalf("real pool source: %+v %v", resource, err)
	}
	if err = runtime.refreshSources(ctx); err != nil {
		t.Fatal("producer to discovery store:", err)
	}
	stored, err := runtime.Discovery.Store.Source(ctx, pool)
	if err != nil || !reflect.DeepEqual(stored, resource.Ref) {
		t.Fatalf("discovery lost source: %+v %v", stored, err)
	}
	run, cancelRun := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- runtime.Run(run) }()
	defer func() {
		cancelRun()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("stop runtime: %v", err)
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for runtime.Ready(ctx) != nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if err = runtime.Ready(ctx); err != nil {
		t.Fatal("API runtime readiness:", err)
	}
	server := httptest.NewServer(runtime.Handler())
	defer server.Close()
	request := func(path string, status int) []byte {
		t.Helper()
		response, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != status {
			t.Fatalf("%s: status %d want %d: %s", path, response.StatusCode, status, raw)
		}
		return raw
	}
	raw := request("/v2/discovery/feed", http.StatusOK)
	var feed discovery.FeedPage
	if err = json.Unmarshal(raw, &feed); err != nil || feed.Snapshot == "" || feed.Algorithm != "chronological-v1" {
		t.Fatalf("real feed request: %s %v", raw, err)
	}
	start := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	path := "/v2/discovery/leaderboards?start=" + url.QueryEscape(start.Format(time.RFC3339)) + "&end=" + url.QueryEscape(start.Add(time.Hour).Format(time.RFC3339))
	raw = request(path, http.StatusOK)
	var leaderboard discovery.LeaderboardV2
	if err = json.Unmarshal(raw, &leaderboard); err != nil || leaderboard.SchemaVersion != 2 || !leaderboard.Start.Equal(start) || len(leaderboard.Entries) != 0 {
		t.Fatalf("real SQL interval request: %s %v", raw, err)
	}
	runtime.Config.Pools = nil
	if err = runtime.Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing pool ready: %v", err)
	}
	for _, path := range []string{"/v2/discovery/feed", "/v2/discovery/callouts"} {
		refusal, err := v2.DecodeError(request(path, http.StatusServiceUnavailable))
		if err != nil || refusal.Code != "DEPENDENCY_UNAVAILABLE" {
			t.Fatalf("missing pool refusal: %+v %v", refusal, err)
		}
	}
	runtime.Config.Pools = config.Pools
	runtime.Discovery.Intervals = nil
	if err = runtime.Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing interval ready: %v", err)
	}
	refusal, err := v2.DecodeError(request(path, http.StatusServiceUnavailable))
	if err != nil || refusal.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("missing interval refusal: %+v %v", refusal, err)
	}
	runtime.Discovery.Intervals = config.Intervals
	if _, err = db.ExecContext(ctx, `DROP VIEW indexer.indexer_canonical_creations_v1`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = NewRuntimeAuthorities(ctx, db, config, quote); err == nil {
		t.Fatal("startup accepted a missing owner interface")
	}
	if _, err = config.Pools.ResolvePublic(ctx, "pool", pool); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing source did not refuse: %v", err)
	}
}

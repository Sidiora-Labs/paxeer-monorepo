// Proves: Options, Server.chainHealth and Server.explorerHealth -> Server.Handler HTTP routes -> srvRead JSON consumer.
//
// The command and e2e consumer now mount the same Handler used here. Real
// Postgres rows, counter triggers and cursor timestamps produce every response;
// neither the database nor HTTP transport is replaced. The server owns no
// persistent health state: repeated reads observe committed changes immediately,
// including after closing and reopening its read-only database connection.
// The test owns its private Postgres process and socket, applies the repository
// migrations, and needs no database URL or operator-provisioned service.
//
// Repairs server.go chainHealth/explorerHealth advertising max-age=2 for live
// health, which let a consumer cache an obsolete result. Both now use no-store,
// including when status refuses a malformed producer row (a missing counter
// becomes SQL NULL and cannot be decoded into its required int64).
package explorer

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

// proofExplorerPostgres starts a real, isolated database using the installed
// Postgres tools. Missing tools fail with their paths rather than skip the proof.
func proofExplorerPostgres(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "pg_config", "--bindir").CombinedOutput()
	if err != nil {
		t.Fatalf("explorer producer pg_config --bindir: %v: %s", err, raw)
	}
	bin := strings.TrimSpace(string(raw))
	for _, name := range []string{"initdb", "pg_ctl", "postgres"} {
		path := filepath.Join(bin, name)
		if st, err := os.Stat(path); err != nil || st.Mode()&0111 == 0 {
			t.Fatalf("explorer producer requires executable %s: %v", path, err)
		}
	}
	// Use a short, directly accessible socket directory; t.TempDir may have
	// root-only ancestors, and Postgres refuses to run with root credentials.
	dir, err := os.MkdirTemp("/tmp", "proof-explorer-pg-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove private Postgres directory %s: %v", dir, err)
		}
	})
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		account, err := user.Lookup("nobody")
		if err != nil {
			t.Fatal(err)
		}
		uid, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		gid, err := strconv.ParseUint(account.Gid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err := os.Chown(dir, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, filepath.Join(bin, name), args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		return cmd.CombinedOutput()
	}
	data, log := filepath.Join(dir, "data"), filepath.Join(dir, "postgres.log")
	if out, err := run("initdb", "-D", data, "-U", "postgres", "-A", "trust", "--no-locale", "-E", "UTF8"); err != nil {
		t.Fatalf("explorer producer %s/initdb: %v: %s", bin, err, out)
	}
	// No TCP listener or shared socket: this producer belongs only to this test.
	options := "-h '' -p 5432 -k " + dir
	if out, err := run("pg_ctl", "-D", data, "-l", log, "-o", options, "-w", "-t", "15", "start"); err != nil {
		serverLog, readErr := os.ReadFile(log)
		t.Fatalf("explorer producer %s/pg_ctl start: %v: %s; log %s (%v): %s", bin, err, out, log, readErr, serverLog)
	}
	t.Cleanup(func() {
		if out, err := run("pg_ctl", "-D", data, "-m", "fast", "-w", "-t", "15", "stop"); err != nil {
			t.Errorf("explorer producer %s/pg_ctl stop: %v: %s", bin, err, out)
		}
	})
	u := url.URL{Scheme: "postgres", User: url.User("postgres"), Path: "/postgres"}
	q := url.Values{"host": {dir}, "port": {"5432"}, "sslmode": {"disable"}, "connect_timeout": {"5"}}
	u.RawQuery = q.Encode()
	return u.String()
}

func TestProofExplorer(t *testing.T) {
	dsn := proofExplorerPostgres(t)
	producer := srvOpenDSN(t, dsn)
	srvSeed(t, producer, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
	if err := producer.Close(); err != nil {
		t.Fatal(err)
	}
	open := func(dsn string) *sql.DB {
		t.Helper()
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		return db
	}
	producer = open(dsn)
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatalf("proof requires a Postgres URL: %v", err)
	}
	q := u.Query()
	q.Set("default_transaction_read_only", "on")
	u.RawQuery = q.Encode()
	reader := open(u.String())
	var readOnly string
	if err := reader.QueryRow(`SHOW default_transaction_read_only`).Scan(&readOnly); err != nil || readOnly != "on" {
		t.Fatalf("consumer session read-only=%q: %v", readOnly, err)
	}
	server := httptest.NewServer(srvNew(t, reader, 120).Handler())
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 5 * time.Second
	base := server.URL

	execProducer := func(query string, args ...any) {
		t.Helper()
		if _, err := producer.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	assertHealth := func(latest, blocks, actions int64, connected bool) {
		t.Helper()
		for range 2 {
			before := time.Now().UnixMilli()
			resp, chain := srvRead(t, client, base+"/api/chain/health", nil)
			status := "degraded"
			if connected {
				status = "ok"
			}
			ts, ok := chain["ts"].(float64)
			if resp.StatusCode != http.StatusOK || len(chain) != 5 || chain["status"] != status ||
				chain["chain_connected"] != connected || chain["blocks_seen"] != float64(blocks) ||
				chain["txs_seen"] != float64(actions) || !ok || int64(ts) < before || int64(ts) > time.Now().UnixMilli() {
				t.Fatalf("chain consumer observed %d %v", resp.StatusCode, chain)
			}
			if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("chain health headers: %v", resp.Header)
			}
			resp, explorer := srvRead(t, client, base+"/api/explorer/health", nil)
			if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-store" ||
				resp.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("explorer health status=%d headers=%v", resp.StatusCode, resp.Header)
			}
			srvEqual(t, explorer, map[string]any{"latest_block": latest, "chain_connected": connected})
		}
	}

	assertHealth(102, 3, 5, true)
	execProducer(`INSERT INTO perps.blocks (number, hash, parent_hash, time, proposer, tx_count)
SELECT 103, $1, hash, time + interval '1 second', proposer, 1 FROM perps.blocks WHERE number = 102`, srvBlockHash(103))
	execProducer(`INSERT INTO perps.tx_actions
    (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time, emitter, kind, account, tx_from, action)
SELECT 103, $1, tx_index, log_index, sub, $2, receipt_kind, block_time + interval '3 seconds', emitter, kind, account, tx_from, action
FROM perps.tx_actions WHERE block = 100`, srvBlockHash(103), srvBytes(0x05, 32))
	assertHealth(103, 4, 6, true)
	execProducer(`UPDATE perps.cursors SET updated_at = now() - interval '1 minute' WHERE name = 'head'`)
	assertHealth(103, 4, 6, false)

	// Stop the HTTP consumer and its database pool. Only the committed producer
	// state survives; a fresh server must still report the stale head and counts.
	server.Close()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	reader = open(u.String())
	server = httptest.NewServer(srvNew(t, reader, 120).Handler())
	t.Cleanup(server.Close)
	client, base = server.Client(), server.URL
	client.Timeout = 5 * time.Second
	assertHealth(103, 4, 6, false)
	execProducer(`UPDATE perps.cursors SET updated_at = now() WHERE name = 'head'`)
	assertHealth(103, 4, 6, true)

	resp, body := srvRead(t, client, base+"/api/chain/tx/0x1234", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed hash: %d %v", resp.StatusCode, body)
	}
	srvEqual(t, body, map[string]any{"error": "invalid transaction hash"})
	resp, body = srvRead(t, client, base+"/api/chain/tx/"+srvHex(srvBytes(0xee, 32)), nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown hash: %d %v", resp.StatusCode, body)
	}
	srvEqual(t, body, map[string]any{"error": "unknown transaction"})

	// A real missing producer row yields NULL at status.Scan, not a cached
	// healthy answer or a fabricated zero. Restore it from the actual table.
	for _, counter := range []string{"blocks", "tx_actions"} {
		execProducer(`DELETE FROM perps.chain_counters WHERE name = $1`, counter)
		for range 2 {
			for _, path := range []string{"/api/chain/health", "/api/explorer/health"} {
				resp, body := srvRead(t, client, base+path, nil)
				if resp.StatusCode != http.StatusInternalServerError || resp.Header.Get("Cache-Control") != "no-store" {
					t.Fatalf("malformed %s producer through %s: %d %v %v", counter, path, resp.StatusCode, resp.Header, body)
				}
				srvEqual(t, body, map[string]any{"error": "internal error"})
			}
		}
		if counter == "blocks" {
			execProducer(`INSERT INTO perps.chain_counters (name, value) SELECT 'blocks', count(*) FROM perps.blocks`)
		} else {
			execProducer(`INSERT INTO perps.chain_counters (name, value) SELECT 'tx_actions', count(*) FROM perps.tx_actions`)
		}
		assertHealth(103, 4, 6, true)
	}

	t.Run("Options", func(t *testing.T) {
		metrics, err := telemetry.New(config.Explorer)
		if err != nil {
			t.Fatal(err)
		}
		proxies := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
		opts := Options{LinkBase: srvLinkBase, TrustedProxies: proxies, Requests: 2, Window: time.Minute, Metrics: metrics}
		limited, err := NewServer(reader, opts)
		if err != nil {
			t.Fatal(err)
		}
		// NewServer must own its proxy policy rather than borrow this slice.
		proxies[0] = netip.MustParsePrefix("192.0.2.0/24")
		limitedHTTP := httptest.NewServer(limited.Handler())
		defer limitedHTTP.Close()
		limitedClient := limitedHTTP.Client()
		limitedClient.Timeout = 5 * time.Second
		header := http.Header{"X-Forwarded-For": {"203.0.113.10"}}
		for _, path := range []string{"/api/chain/health", "/api/explorer/health"} {
			resp, body := srvRead(t, limitedClient, limitedHTTP.URL+path, header)
			if resp.StatusCode != http.StatusOK || body["chain_connected"] != true {
				t.Fatalf("Options consumer through %s: %d %v", path, resp.StatusCode, body)
			}
		}
		resp, body := srvRead(t, limitedClient, limitedHTTP.URL+"/api/explorer/health", header)
		if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "60" {
			t.Fatalf("Options rate refusal: %d %v %v", resp.StatusCode, resp.Header, body)
		}
		srvEqual(t, body, map[string]any{"error": "rate limit exceeded"})
		header.Set("X-Forwarded-For", "203.0.113.11")
		resp, body = srvRead(t, limitedClient, limitedHTTP.URL+"/api/explorer/health", header)
		if resp.StatusCode != http.StatusOK || body["chain_connected"] != true {
			t.Fatalf("independent trusted client: %d %v", resp.StatusCode, body)
		}
		resp, body = srvRead(t, limitedClient, limitedHTTP.URL+"/api/chain/tx/"+srvHex(srvTxPlace), header)
		if resp.StatusCode != http.StatusOK || body["link"] != srvLinkBase+"/tx/"+srvHex(srvTxPlace) {
			t.Fatalf("Options link consumer: %d %v", resp.StatusCode, body)
		}
		for _, invalid := range []Options{
			{Requests: 2, Window: time.Minute},
			{Requests: 0, Window: time.Minute, Metrics: metrics},
			{Requests: 2, Window: 0, Metrics: metrics},
		} {
			if server, err := NewServer(reader, invalid); err == nil || server != nil {
				t.Fatalf("invalid Options accepted: server=%v err=%v", server, err)
			}
		}
	})
}

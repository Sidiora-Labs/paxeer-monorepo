// Proves: rest.Markets, rest.Symbol, rest.SpotUsers.StreamItems -> gateway.Server.Handler HTTP and ws.SpotUser -> portal.Pages.Markets/Market and REST/socket JSON consumers.
//
// Repairs portal/pages.go accepting an empty market identity, markets.go
// omitting cache refusal for live records, symbol.go ignoring duplicate limit
// values, venue.go differing from Markets.Record's case-insensitive lookup,
// and spotusers.go StreamItems silently losing an event with no stored trade.
// Producer state comes from a private PostgreSQL process and store.WriteFacts /
// store.Project; envelopes come from their outbox, never a hand-built message.
// Each consumer reads twice, then observes changed producer state. A malformed
// record emitted by the real route is refused by the portal before rendering.
// Outside scope: records/records.go MarketRecord and store market ingestion
// admit an empty symbol; the portal must refuse that producer result.
package rest_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	evmruntime "github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	gethnode "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/wpax"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/gateway"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/portal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/ws"
)

func TestProofRestProducerTransportConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	databaseURL := proofRESTPostgres(t)
	db, err := store.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	rest.ProofRESTSeedMarkets(t, db, at)
	env := rest.ProofRESTProduceSpot(t, db, at, 50)
	// Reopen the connection pool to demonstrate that only PostgreSQL carries
	// producer state; no constructor or test-side response cache supplies it.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = store.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go ns.Start()
	t.Cleanup(func() { ns.Shutdown(); ns.WaitForShutdown() })
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("gateway NATS producer failed to start")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	node := proofRESTNode(t)
	metrics, err := telemetry.New(config.Gateway)
	if err != nil {
		t.Fatal(err)
	}
	gw, err := gateway.NewServer(ctx, gateway.Options{DB: db, NATS: nc, Chain: 125, AuthSecret: "proof-rest-local-secret-at-least-32-bytes", Node: node, Metrics: metrics})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(gw.Handler())
	t.Cleanup(httpServer.Close)
	client := httpServer.Client()
	client.Timeout = 10 * time.Second
	pages, err := portal.NewPages(httpServer.URL, httpServer.URL+"/tx", client)
	if err != nil {
		t.Fatal(err)
	}
	spot, err := rest.NewSpotUsers(db, node)
	if err != nil {
		t.Fatal(err)
	}

	for round := 1; round <= 2; round++ {
		mark, index := float64(70000), float64(69999)
		if round == 2 {
			proofRESTExec(t, db, `UPDATE perps.mark_prices SET mark_price=71000*1e18::numeric, index_price=70999*1e18::numeric WHERE market_id=0`)
			env = rest.ProofRESTProduceSpot(t, db, at.Add(time.Second), 53)
			mark, index = 71000, 70999
		}
		for repeat := 0; repeat < 2; repeat++ {
			rest.ProofRESTObserveMarkets(t, client, httpServer.URL, mark)
			rest.ProofRESTObserveSymbol(t, client, httpServer.URL, index)
			rest.ProofRESTObserveSpot(t, client, httpServer.URL, round)
			rest.ProofRESTObserveVenue(t, client, httpServer.URL, mark)
			var home, market bytes.Buffer
			if err := pages.Markets(ctx, &home); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(home.String(), "Bitcoin") || !strings.Contains(home.String(), strconv.Itoa(int(mark))) {
				t.Fatalf("portal did not observe market: %s", home.String())
			}
			if err := pages.Market(ctx, &market, "btc"); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(market.String(), strconv.Itoa(int(index))) || !strings.Contains(market.String(), "140000") {
				t.Fatalf("portal market did not observe producer: %s", market.String())
			}
			proofRESTStream(t, ctx, spot, env, client, httpServer.URL)
		}
	}
	var missing bytes.Buffer
	if err := pages.Market(ctx, &missing, "unknown"); !errors.Is(err, portal.ErrNotFound) || missing.Len() != 0 {
		t.Fatalf("unknown market: %v, wrote %d", err, missing.Len())
	}

	// Both venue lookups must agree with Markets.Record for stored mixed case.
	proofRESTExec(t, db, `UPDATE perps.markets SET symbol='bTc' WHERE market_id=0`)
	rest.ProofRESTGet(t, client, httpServer.URL, "/api/ticker/BTC", http.StatusOK)
	rest.ProofRESTGet(t, client, httpServer.URL, "/api/orderbook/BTC", http.StatusOK)
	proofRESTExec(t, db, `UPDATE perps.markets SET symbol='' WHERE market_id=0`)
	// The route really emits this malformed identity; no HTTP layer is replaced.
	body := rest.ProofRESTGet(t, client, httpServer.URL, "/api/markets", http.StatusOK)
	var records []struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(body["markets"], &records); err != nil || len(records) == 0 || records[0].Symbol != "" {
		t.Fatalf("malformed producer record: %s, %v", body["markets"], err)
	}
	var rejected bytes.Buffer
	if err := pages.Markets(ctx, &rejected); err == nil || !strings.Contains(err.Error(), "invalid market identity") || rejected.Len() != 0 {
		t.Fatalf("malformed record was not refused: %v, wrote %d", err, rejected.Len())
	}

	// The valid outbox event now names a missing producer fact. StreamItems
	// must fail so the gateway can close sockets and require REST resync.
	proofRESTExec(t, db, `DELETE FROM perps.spot_swaps WHERE block=55`)
	if items, err := spot.StreamItems(ctx, env); err == nil || !strings.Contains(err.Error(), "not found") || len(items) != 0 {
		t.Fatalf("missing producer trade accepted: %+v, %v", items, err)
	}
}

func proofRESTExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func proofRESTStream(t *testing.T, ctx context.Context, spot *rest.SpotUsers, env delivery.Envelope, client *http.Client, base string) {
	t.Helper()
	items, err := spot.StreamItems(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	consumer := &ws.SpotUser{}
	ts := httptest.NewServer(consumer)
	defer ts.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var control struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&control); err != nil || control.Type != "hello" {
		t.Fatalf("socket hello: %+v %v", control, err)
	}
	address := "0x00000000000000000000000000000000000000a1"
	if err := conn.WriteJSON(map[string]string{"op": "subscribe", "stream": "trade", "address": address}); err != nil {
		t.Fatal(err)
	}
	if err := conn.ReadJSON(&control); err != nil || control.Type != "subscribed" {
		t.Fatalf("socket subscribe: %+v %v", control, err)
	}
	streams := map[string]bool{}
	var trade json.RawMessage
	for _, item := range items {
		streams[item.Stream] = true
		if item.Address != address {
			t.Fatalf("stream account %s", item.Address)
		}
		if err := consumer.Deliver(item.Stream, item.Address, item.Data); err != nil {
			t.Fatal(err)
		}
		if item.Stream == "trade" {
			trade = item.Data
		}
	}
	for _, name := range []string{"trade", "pnl", "closed-leg"} {
		if !streams[name] {
			t.Fatalf("StreamItems missing %s: %+v", name, items)
		}
	}
	var frame struct {
		Stream  string
		Address string
		Data    json.RawMessage
	}
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatal(err)
	}
	if frame.Stream != "trade" || frame.Address != address || !bytes.Equal(frame.Data, trade) {
		t.Fatalf("socket consumer: %+v, want %s", frame, trade)
	}
	body := rest.ProofRESTGet(t, client, base, "/v1/users/"+address+"/trades?limit=1", http.StatusOK)
	var trades []json.RawMessage
	if err := json.Unmarshal(body["items"], &trades); err != nil || len(trades) != 1 {
		t.Fatalf("REST trade consumer: %s %v", body["items"], err)
	}
	if !bytes.Equal(trades[0], trade) {
		t.Fatalf("REST/socket trade mismatch: %s / %s", trades[0], trade)
	}
}

// proofRESTPostgres owns a temporary database process. It reads no environment
// URL, path or configuration name and reports a missing executable as failure.
func proofRESTPostgres(t *testing.T) string {
	t.Helper()
	bins, err := filepath.Glob("/usr/lib/postgresql/*/bin/initdb")
	if err != nil || len(bins) == 0 {
		t.Fatalf("producer PostgreSQL missing: /usr/lib/postgresql/*/bin/initdb: %v", err)
	}
	bin := filepath.Dir(bins[len(bins)-1])
	dir, err := os.MkdirTemp("", "proof-rest-postgres-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		account, err := user.Lookup("postgres")
		if err != nil {
			t.Fatalf("PostgreSQL producer service account: %v", err)
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
	run := func(name string, args ...string) error {
		cmd := exec.Command(filepath.Join(bin, name), args...)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "HOME=" + dir}
		if credential != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w: %s", cmd.Path, err, out)
		}
		return nil
	}
	data := filepath.Join(dir, "data")
	if err := run("initdb", "-D", data, "-A", "trust", "-U", "postgres", "--no-locale"); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	options := fmt.Sprintf("-h 127.0.0.1 -p %d -k %s -F -c max_connections=30", port, dir)
	if err := run("pg_ctl", "-D", data, "-l", filepath.Join(dir, "postgres.log"), "-o", options, "-w", "-t", "15", "start"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := run("pg_ctl", "-D", data, "-m", "immediate", "-w", "stop"); err != nil {
			t.Error(err)
		}
	})
	return fmt.Sprintf("postgres://postgres@127.0.0.1:%d/postgres?sslmode=disable&connect_timeout=5", port)
}

// proofRESTNode hosts the repository's Ethereum service and real WPAX EVM
// bytecode, whose balanceOf reads the empty holdings after the fully sold leg.
func proofRESTNode(t *testing.T) *ethclient.Client {
	t.Helper()
	code, _, _, err := evmruntime.Create(wpax.GetBin(), &evmruntime.Config{GasLimit: 30000000})
	if err != nil || len(code) == 0 {
		t.Fatalf("modules/evm/artifacts/wpax producer deployment: %v", err)
	}
	stack, err := gethnode.New(&gethnode.Config{P2P: p2p.Config{NoDiscovery: true, MaxPeers: 0, ListenAddr: "127.0.0.1:0"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stack.Close(); err != nil {
			t.Error(err)
		}
	})
	genesis := &core.Genesis{Config: params.AllDevChainProtocolChanges, GasLimit: 30000000, BaseFee: big.NewInt(params.InitialBaseFee), Alloc: types.GenesisAlloc{
		common.HexToAddress("0x000000000000000000000000000000000000a202"): {Code: code, Balance: new(big.Int)},
	}}
	if _, err := eth.New(stack, &ethconfig.Config{Genesis: genesis, RPCGasCap: 30000000}); err != nil {
		t.Fatal(err)
	}
	if err := stack.Start(); err != nil {
		t.Fatal(err)
	}
	client := ethclient.NewClient(stack.Attach())
	t.Cleanup(client.Close)
	return client
}

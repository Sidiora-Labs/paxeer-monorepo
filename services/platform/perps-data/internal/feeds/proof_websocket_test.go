// Proves: feeds.NewProviders/records.Symbol -> gateway.Server/ws.Hub /ws -> venueStream/useSidPaxMarketStream/useSpotMarketStream decoders.
//
// The proof uses migrated Postgres state, a real JetStream broker, the gateway's
// HTTP routes and Node's native WebSocket. It imports the application modules
// themselves; no producer, transport or decoder is replaced. Each process is
// stopped and reopened over the same database, then changed facts must be seen.
// Repairs: gateway.NewServer omitted the served-market gauge; parseVenueBook
// accepted incomplete books; the dedicated hooks invented trades from malformed
// fields and read nonexistent volume_24h_usd/oi_usd instead of the stats fields.
// Outside touches, internal/records/records.go Symbol.PerpTrades emits a
// zero-price fill with zero contracts. The expected consumer behavior is refusal,
// which this proof checks on that actual producer result over the socket.
// Node >= 22.15 and the application's installed react/viem dependencies are
// required; PERPS_WEBSOCKET_NODE_MODULES may name a separate dependency directory.
package feeds_test

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/feeds"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/gateway"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/stream"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

func proofWebsocketExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("seed websocket producer: %v", err)
	}
}

// Facts have the same common columns and domain constraints as indexed events.
func proofWebsocketFact(t *testing.T, db *sql.DB, table, kind string, block, logIndex int, cols []string, values ...any) {
	t.Helper()
	common := []string{"block", "block_hash", "tx_index", "log_index", "sub", "tx_hash", "receipt_kind", "block_time", "emitter", "kind", "account", "tx_from"}
	hash := []byte(fmt.Sprintf("%032d", block))
	address := make([]byte, 20)
	address[18], address[19] = 0x10, 0x15
	args := append([]any{block, hash, 0, logIndex, 0, hash, "tx", time.Now().UTC(), address, kind, address, address}, values...)
	names := append(common, cols...)
	params := make([]string, len(names))
	for i := range names {
		names[i] = `"` + names[i] + `"`
		params[i] = fmt.Sprintf("$%d", i+1)
	}
	proofWebsocketExec(t, db, "INSERT INTO perps."+table+" ("+strings.Join(names, ",")+") VALUES ("+strings.Join(params, ",")+")", args...)
}

func TestProofWebsocketProducerTransportConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	databaseURL := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is required")
	}
	db, err := store.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	proofWebsocketExec(t, db, "DROP SCHEMA IF EXISTS perps CASCADE")
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	broker, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: natsserver.RANDOM_PORT, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go broker.Start()
	defer func() { broker.Shutdown(); broker.WaitForShutdown() }()
	if !broker.ReadyForConnections(5 * time.Second) {
		t.Fatal("JetStream did not start")
	}
	nc, err := nats.Connect(broker.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	streamConfig := stream.Config(125)
	streamConfig.Replicas = 1
	if _, err := js.CreateStream(ctx, streamConfig); err != nil {
		t.Fatal(err)
	}

	// These routes do not call the node. Use the real RPC client, without a
	// server double or any network request to an external chain.
	node, err := ethclient.DialContext(ctx, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	address := make([]byte, 20)
	address[18], address[19] = 0x10, 0x15
	token := func(n byte) []byte { b := make([]byte, 20); b[19] = n; return b }
	for i := 0; i < 3; i++ {
		proofWebsocketFact(t, db, "spot_token_registry", "token_registered", 1, i,
			[]string{"token", "decimals", "is_stablecoin"}, token(byte(i+1)), 6, i == 0)
	}
	proofWebsocketExec(t, db, `INSERT INTO perps.cursors(name,height,hash) VALUES ('head',100,$1)`, make([]byte, 32))
	proofWebsocketExec(t, db, `INSERT INTO perps.orders(order_id,account,market_id,order_type,is_long,trigger_price,size_usd,limit_price,remaining_size_usd,status,placed_block)
        VALUES (1,$1,0,1,true,0,1000000000000000000000,100000000000000000000,1000000000000000000000,'active',1)`, address)
	proofWebsocketFact(t, db, "open_interest", "oi", 3, 0,
		[]string{"market_id", "long_oi", "short_oi", "delta_usd", "is_increase"},
		0, "100000000000000000000", "50000000000000000000", "150000000000000000000", true)
	proofWebsocketExec(t, db, `INSERT INTO perps.spot_orders(order_id,account,kind,side,token,stablecoin,amount,target_price,status,placed_block)
        VALUES (1,$1,'limit','sell',$2,$3,2000000,2000000000000000000,'open',1)`, address, token(2), token(1))
	// A malformed producer result, legal in the stored uint256 domain, must
	// reach the real browser decoder and be refused there.
	proofWebsocketFact(t, db, "fills", "fill", 2, 0,
		[]string{"market_id", "maker_order_id", "taker_order_id", "maker", "taker", "taker_is_long", "price", "size_usd", "maker_fee_usd", "taker_fee_usd"},
		1, 1, 2, address, address, true, "0", "1000000000000000000", "0", "0")

	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../.."))
	var previousEpoch string
	for round := 1; round <= 2; round++ {
		// Reopening must read the changed durable facts rather than replay a
		// frame held by an earlier gateway or a consumer's cached snapshot.
		price := 100 + round
		scaled := fmt.Sprintf("%d000000000000000000", price)
		notional := fmt.Sprintf("%d000000000000000000", 2*price)
		proofWebsocketExec(t, db, `UPDATE perps.orders SET limit_price=$1, remaining_size_usd=$2`, scaled, notional)
		proofWebsocketFact(t, db, "mark_prices", "mark", 10+round, 0,
			[]string{"market_id", "mark_price", "index_price", "mid_price"}, 0, scaled, scaled, scaled)
		for _, market := range []int{0, 11} {
			proofWebsocketFact(t, db, "fills", "fill", 20+round, market,
				[]string{"market_id", "maker_order_id", "taker_order_id", "maker", "taker", "taker_is_long", "price", "size_usd", "maker_fee_usd", "taker_fee_usd"},
				market, 1, 2, address, address, true, scaled, notional, "0", "0")
		}
		proofWebsocketFact(t, db, "spot_swaps", "market_order", 30+round, 0,
			[]string{"user", "stablecoin", "token", "stablecoin_amount", "token_amount", "is_buy"}, address, token(1), token(2), 3000000, 2000000, true)
		metrics, err := telemetry.New(config.Gateway)
		if err != nil {
			t.Fatal(err)
		}
		svc, err := gateway.NewServer(ctx, gateway.Options{DB: db, NATS: nc, Chain: 125, Node: node, AuthSecret: strings.Repeat("p", 32), Metrics: metrics, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
		if err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(svc.Handler())
		runCtx, stop := context.WithCancel(ctx)
		stopped := make(chan error, 1)
		go func() { stopped <- svc.Run(runCtx) }()
		func() {
			defer server.Close()
			defer func() {
				stop()
				if err := <-stopped; err != nil && !errors.Is(err, context.Canceled) {
					t.Errorf("gateway Run: %v", err)
				}
			}()
			deadline := time.Now().Add(10 * time.Second)
			for !svc.Ready() {
				if time.Now().After(deadline) {
					t.Fatal("gateway did not become ready")
				}
				time.Sleep(10 * time.Millisecond)
			}
			resp, err := http.Get(server.URL + "/metrics")
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(body), feeds.GatewayMarketsMetric+" 18\n") {
				t.Fatalf("gateway market gauge: %s (%v)", body, err)
			}
			cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", proofWebsocketConsumer)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "NEXT_PUBLIC_PERPS_GATEWAY_URL="+server.URL, "PROOF_ROUND="+fmt.Sprint(round))
			var stderr strings.Builder
			cmd.Stderr = &stderr
			output, err := cmd.Output()
			if err != nil {
				t.Fatalf("application websocket consumer: %v\n%s\n%s", err, stderr.String(), output)
			}
			var result struct {
				Epoch       string `json:"epoch"`
				Topics      int    `json:"topics"`
				Connections int    `json:"connections"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("consumer output %s: %v", output, err)
			}
			if result.Epoch == "" || result.Epoch == previousEpoch || result.Topics != 9 || result.Connections != 2 {
				t.Fatalf("consumer result %+v; previous epoch %q", result, previousEpoch)
			}
			previousEpoch = result.Epoch
		}()
	}
}

// The loader only resolves the application's @ alias and strips TypeScript;
// every imported runtime dependency and every decoding function is real code.
const proofWebsocketConsumer = `
import assert from 'node:assert/strict';
import { registerHooks, stripTypeScriptTypes, createRequire } from 'node:module';
import { readFileSync, existsSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { pathToFileURL, fileURLToPath } from 'node:url';
const src = resolve('apps/perps/src');
const dependencies = createRequire(pathToFileURL(process.env.PERPS_WEBSOCKET_NODE_MODULES
  ? resolve(process.env.PERPS_WEBSOCKET_NODE_MODULES, '../package.json') : resolve('apps/perps/package.json')));
const runtimeDependencies = new Map(['react','viem'].map(name => [name,pathToFileURL(dependencies.resolve(name)).href]));
registerHooks({
  resolve(specifier, context, next) {
    let path;
    if (specifier.startsWith('@/')) path = resolve(src, specifier.slice(2));
    else if (specifier.startsWith('.') && context.parentURL?.startsWith('file:') && context.parentURL.includes('/apps/perps/src/'))
      path = resolve(dirname(fileURLToPath(context.parentURL)), specifier);
    if (path) {
      for (const candidate of [path, path + '.ts', path + '/index.ts']) {
        if (existsSync(candidate) && candidate.endsWith('.ts')) return {url:pathToFileURL(candidate).href,shortCircuit:true};
      }
    }
    if (runtimeDependencies.has(specifier)) return {url:runtimeDependencies.get(specifier),shortCircuit:true};
    return next(specifier, context);
  },
  load(url, context, next) {
    if (url.startsWith('file:') && url.endsWith('.ts')) return {format:'module',source:stripTypeScriptTypes(readFileSync(fileURLToPath(url),'utf8')),shortCircuit:true};
    return next(url,context);
  }
});
const venue = await import(pathToFileURL(resolve(src,'core/net/venueStream.ts')));
const perp = await import(pathToFileURL(resolve(src,'hooks/perps/useSidPaxMarketStream.ts')));
const spot = await import(pathToFileURL(resolve(src,'hooks/spot/useSpotMarketStream.ts')));
const markets = await import(pathToFileURL(resolve(src,'hooks/perps/useMarketsStream.ts')));
const {sequenceGuard} = await import(pathToFileURL(resolve(src,'core/net/realtime.ts')));
const {gatewayUrls} = await import(pathToFileURL(resolve(src,'core/network/gateway.ts')));
const round = Number(process.env.PROOF_ROUND), price = 100+round;
const volume = round === 1 ? 202 : 406;
const topics = [venue.perpBookTopic('btc'),venue.perpTradeTopic('btc'),'BTC@perp_stats','PAX@stats','BTC@market',venue.spotBookTopic('pax'),venue.spotTradeTopic('pax'),'ETH@perp_trade','markets@all'];
let epoch;
for (let connection=0; connection<2; connection++) {
  const ws = new WebSocket(gatewayUrls().venueWsUrl);
  const accept = sequenceGuard(), seen = new Set(), acknowledged = new Set();
  let refused = false, settled = false;
  await new Promise((done,fail) => {
    const timer = setTimeout(()=>finish(new Error('websocket proof timed out: '+[...seen])),10000);
    function finish(error) { if (settled) return; settled=true; clearTimeout(timer); ws.close(); error ? fail(error) : done(); }
    ws.addEventListener('error',()=>finish(new Error('websocket error')));
    ws.addEventListener('close',()=>{ if (!settled) finish(new Error('socket closed before snapshots')); });
    ws.addEventListener('open',()=>{
      for (const topic of topics) ws.send(JSON.stringify({event:'subscribe',topic}));
      ws.send(JSON.stringify({event:'subscribe',topic:'UNKNOWN@perp_trade'}));
    });
    ws.addEventListener('message',event=>{
      try {
        const frame = JSON.parse(event.data);
        assert(accept(frame),'real sequenceGuard refused the gateway frame');
        if (frame.event === 'welcome') { assert(frame.snapshotId); if (epoch) assert.equal(frame.snapshotId,epoch); epoch=frame.snapshotId; }
        if (frame.event === 'subscribed') acknowledged.add(frame.topic);
        if (frame.event === 'error') { assert.equal(frame.topic,'UNKNOWN@perp_trade'); assert.equal(frame.message,'unknown_topic'); refused=true; }
        if (frame.snapshot && topics.includes(frame.topic) && !seen.has(frame.topic)) {
          assert(acknowledged.has(frame.topic),'snapshot preceded subscribe acknowledgement');
          assert.equal(frame.snapshotId,epoch); assert(Number.isInteger(frame.seq)); assert(frame.ts>0);
          const d=frame.data;
          switch(frame.topic) {
            case 'BTC@perp_orderbook': {
              const book=venue.parseVenueBook(d,frame.ts); assert(book); assert.equal(book.bids.length,1);
              assert.equal(book.bids[0].price,price); assert.equal(book.bids[0].size,2); assert.equal(book.markPrice,price); break;
            }
            case 'BTC@perp_trade': {
              assert.equal(d.trades.length,round);
              const a=venue.parseVenueFill(d.trades[0]), b=perp.parseSidPaxTrade(d.trades[0]);
              assert(a); assert(b); assert.equal(a.price,price); assert.equal(a.contracts,2); assert.equal(b.notional,2*price);
              assert.equal(b.trade_id,(20+round)+'-0'); break;
            }
            case 'BTC@perp_stats': {
              const stats=perp.parseSidPaxStats(d,'BTC'); assert(stats);
              assert.equal(stats.mark_price,price); assert.equal(stats.volume_24h_usd,volume); assert.equal(stats.oi_usd,150); assert.equal(stats.long_short_ratio,2); break;
            }
            case 'PAX@stats': { const stats=spot.parseSpotStats(d,'PAX'); assert(stats); assert.equal(stats.volume_24h,volume); break; }
            case 'BTC@market': { const market=markets.normalizeMarket(d); assert(market); assert.equal(market.price,price); assert.equal(market.volume_24h,volume); break; }
            case 'PAX@orderbook': { const book=venue.parseVenueBook(d,frame.ts); assert(book); assert.equal(book.asks[0].price,2); assert.equal(book.asks[0].size,2); break; }
            case 'PAX@trade': { const a=venue.parseSpotVenueTrade(d.trades[0]),b=spot.parseSpotTrade(d.trades[0]); assert(a); assert(b); assert.equal(b.price,1.5); assert.equal(b.size,2); assert.equal(a.id,(30+round)+'-0'); break; }
            case 'ETH@perp_trade': { assert.equal(d.trades.length,1); assert.equal(d.trades[0].price,0); assert.equal(venue.parseVenueFill(d.trades[0]),null); assert.equal(perp.parseSidPaxTrade(d.trades[0]),null); assert.equal(spot.parseSpotTrade(d.trades[0]),null); assert.equal(venue.parseVenueBook(d.trades[0],frame.ts),null); assert.equal(perp.parseSidPaxStats(d.trades[0],'ETH'),null); assert.equal(spot.parseSpotStats(d.trades[0],'ETH'),null); break; }
            case 'markets@all': { const rows=markets.normalizeMarketsFrame(d); assert.equal(rows.length,18); assert.equal(rows.find(r=>r.symbol==='BTC').price,price); break; }
          }
          seen.add(frame.topic);
        }
        if (seen.size === topics.length && refused) {
          // The refusal leaves this real socket usable.
          ws.send(JSON.stringify({event:'ping'}));
        }
        if (frame.event === 'pong' && seen.size === topics.length && refused) finish();
      } catch(error) { finish(error); }
    });
  });
}
console.log(JSON.stringify({epoch,topics:topics.length,connections:2}));
`

func TestMarketsNullableSnapshotsReachMountedView(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	databaseURL := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is required")
	}
	db, err := store.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	proofWebsocketExec(t, db, "DROP SCHEMA IF EXISTS perps CASCADE")
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	proofWebsocketExec(t, db, `UPDATE perps.markets SET listed_at=now() WHERE market_id < 8`)
	proofWebsocketExec(t, db, `INSERT INTO perps.blocks(number,hash,parent_hash,time,proposer,tx_count)
 VALUES (10,$1,$1,now(),repeat('0',40),0)`, make([]byte, 32))
	proofWebsocketExec(t, db, `INSERT INTO perps.cursors(name,height,hash) VALUES ('head',100,$1)`, make([]byte, 32))
	address := make([]byte, 20)
	address[19] = 1
	for id := 0; id < 8; id++ {
		proofWebsocketFact(t, db, "fills", "fill", 2, id,
			[]string{"market_id", "maker_order_id", "taker_order_id", "maker", "taker", "taker_is_long", "price", "size_usd", "maker_fee_usd", "taker_fee_usd"},
			id, 1, 2, address, address, true, "100000000000000000000", fmt.Sprintf("%d000000000000000000", 1200+id), "0", "0")
		proofWebsocketFact(t, db, "open_interest", "oi", 3, id,
			[]string{"market_id", "long_oi", "short_oi", "delta_usd", "is_increase"},
			id, fmt.Sprintf("%d000000000000000000", 100+id), "50000000000000000000", "0", true)
		if id < 7 {
			// Deliberately beyond the exact integer range of a JS number, with a
			// nonzero final decimal digit and both signs across sibling rows.
			rate := "123456789012345678901"
			if id%2 == 1 {
				rate = "-" + rate
			}
			proofWebsocketFact(t, db, "funding_settlements", "settled", 4, id,
				[]string{"market_id", "funding_rate", "long_payment", "short_payment"}, id, rate, "0", "0")
		}
	}
	setPhase := func(phase int) {
		proofWebsocketExec(t, db, `DELETE FROM perps.mark_prices`)
		for id := 0; id < 8; id++ {
			mask := 7
			if phase == 1 {
				mask = id
			}
			if phase == 2 {
				mask = 0
			}
			if mask&1 == 0 {
				price := fmt.Sprintf("%d000000000000000000", 100+id+phase)
				proofWebsocketFact(t, db, "mark_prices", "mark", 20+phase, id,
					[]string{"market_id", "mark_price", "index_price", "mid_price"}, id, price, price, price)
			}
			var opening any
			if mask&2 == 0 {
				opening = "100000000000000000000"
			}
			updated := 10
			if mask&4 != 0 {
				updated = 11
			}
			proofWebsocketExec(t, db, `INSERT INTO perps.market_stats_rolling
    (market_id,volume_24h,volume_7d,volume_30d,trades_count,open_interest_long,open_interest_short,last_price,open_24h,updated_block)
    VALUES ($1,0,700000000000000000000,900000000000000000000,1,0,0,$2,$3,$4)
    ON CONFLICT (market_id) DO UPDATE SET last_price=EXCLUDED.last_price,open_24h=EXCLUDED.open_24h,updated_block=EXCLUDED.updated_block`,
				id, fmt.Sprintf("%d000000000000000000", id+110), opening, updated)
		}
	}
	setPhase(0)
	broker, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: natsserver.RANDOM_PORT, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go broker.Start()
	defer func() { broker.Shutdown(); broker.WaitForShutdown() }()
	if !broker.ReadyForConnections(5 * time.Second) {
		t.Fatal("JetStream did not start")
	}
	nc, err := nats.Connect(broker.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	streamConfig := stream.Config(125)
	streamConfig.Replicas = 1
	if _, err := js.CreateStream(ctx, streamConfig); err != nil {
		t.Fatal(err)
	}
	node, err := ethclient.DialContext(ctx, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	metrics, err := telemetry.New(config.Gateway)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := gateway.NewServer(ctx, gateway.Options{DB: db, NATS: nc, Chain: 125, Node: node, AuthSecret: strings.Repeat("p", 32), Metrics: metrics, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(svc.Handler())
	defer server.Close()
	runCtx, stop := context.WithCancel(ctx)
	stopped := make(chan error, 1)
	go func() { stopped <- svc.Run(runCtx) }()
	defer func() {
		stop()
		if err := <-stopped; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("gateway Run: %v", err)
		}
	}()
	for !svc.Ready() {
		select {
		case <-ctx.Done():
			t.Fatal("gateway did not become ready")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../.."))
	cmd := exec.CommandContext(ctx, "node", "--import", "tsx", "--test", "--test-concurrency=1", "--test-reporter=tap", "--test-name-pattern=^(marketsNullableSnapshotsReachMountedView|marketsRowsCrossExportsIntoTheRealTableWithoutStaleResults)$", "tests/proof_widgets.test.ts")
	cmd.Dir = filepath.Join(root, "apps/perps")
	controlDir := t.TempDir()
	cmd.Env = append(os.Environ(), "NEXT_PUBLIC_PERPS_GATEWAY_URL="+server.URL, "NEXT_PUBLIC_MARKET_PROXY_HOST="+server.URL, "PROOF_NULLABLE_MARKETS=1", "PROOF_PHASE_DIR="+controlDir)
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr, transcript strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	scanner := bufio.NewScanner(output)
	phases := 0
	for scanner.Scan() {
		line := scanner.Text()
		transcript.WriteString(line + "\n")
		if strings.Contains(line, "PROOF_ADVANCE=") {
			var phase int
			part := line[strings.Index(line, "PROOF_ADVANCE="):]
			if _, err := fmt.Sscanf(part, "PROOF_ADVANCE=%d", &phase); err != nil || phase != phases+1 || phase > 3 {
				t.Fatalf("invalid phase request %s", line)
			}
			setPhase(phase)
			phases = phase
			if err := os.WriteFile(filepath.Join(controlDir, fmt.Sprint(phase)), []byte(fmt.Sprint(phase)), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("mounted markets consumer: %v\n%s\n%s", err, transcript.String(), stderr.String())
	}
	if phases != 3 || !strings.Contains(transcript.String(), "# pass 2") {
		t.Fatalf("both consumer proofs must execute through all phases:\n%s", transcript.String())
	}
}

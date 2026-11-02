// Proves: graphql.Resolver.Market/Markets and Market.MarginTiers -> gateway.Server.Handler POST /graphql -> useSubgraph.market/fetchMarkets and usePortfolioStats.fetchPortfolioPositions.
//
// Repairs useSubgraph.subgraphQuery trusting malformed data, market queries
// omitting margin tiers, and usePortfolioStats accepting unvalidated positions.
// Repairs graphql.Resolver.Market treating a negative identifier as an unknown
// market; schema.go documents that boundary. The proof uses the real Postgres
// schema, gateway, HTTP fetch policy and TypeScript consumer functions. Aliases
// make the real producer emit a response incompatible with the consumer's
// projection, so decoding refusal needs no fabricated response or transport.
package graphql_test

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/gateway"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

func TestProofGraphqlProducerTransportConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	databaseURL := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is required (a dedicated test database)")
	}
	db, err := store.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	proofGraphqlExec(t, db, `INSERT INTO perps.markets
    (market_id, name, symbol, oracle_symbol, max_leverage, maintenance_margin_bps,
     max_open_interest, enabled, source, sz_decimals, px_decimals) VALUES
    (211071, 'Proof Bitcoin', 'PROOFBTC', 'BTC', 100, 50, 1000000, true, 'chain', 5, 2),
    (211072, 'Proof disabled', 'PROOFOFF', 'OFF', 20, 100, 1000000, false, 'chain', 0, 0);
INSERT INTO perps.market_tiers
    (market_id, tier, lower_bound_usd, max_leverage, maintenance_bps, deduction_usd, refreshed_block) VALUES
    (211071, 2, 9007199254740993, 20000000000000000000, 100, 9007199254740993, 42),
    (211071, 0, 0, 100000000000000000000, 50, 0, 42),
    (211071, 10, 115792089237316195423570985008687907853269984665640564039457584007913129639935,
        1000000000000000000, 500, 1000000000000000000, 42);
INSERT INTO perps.positions_current
    (position_id, account, market_id, is_long, size_usd, collateral_usd, collateral_amount,
     collateral_token, entry_price, leverage, realized_pnl, status, opened_block) VALUES
    (211071, decode('0000000000000000000000000000000000211071', 'hex'), 211071, true,
     9007199254740993, 1000000000000000000, 1000000,
     decode('0000000000000000000000000000000000000001', 'hex'),
     50000000000000000000000, 10000000000000000000, 0, 'open', 42)`)
	// Close and reopen the producer's connection pool before the HTTP consumer
	// starts; no state is carried in a resolver or a test-side response fixture.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = store.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, cleanupErr := db.Exec(`DELETE FROM perps.positions_current WHERE position_id = 211071;
DELETE FROM perps.market_tiers WHERE market_id IN (211071, 211072);
DELETE FROM perps.markets WHERE market_id IN (211071, 211072)`)
		if cleanupErr != nil {
			t.Errorf("clean proof state: %v", cleanupErr)
		}
		db.Close()
	})

	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT,
		JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go ns.Start()
	t.Cleanup(func() { ns.Shutdown(); ns.WaitForShutdown() })
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("real NATS server did not start")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	// GraphQL uses Postgres; the real Ethereum client is an unused dependency
	// of the gateway's separate spot routes and never supplies proof data.
	nodeURL := os.Getenv("PERPS_DATA_TEST_RPC")
	if nodeURL == "" {
		nodeURL = "http://127.0.0.1:8545"
	}
	node, err := ethclient.DialContext(ctx, nodeURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(node.Close)
	metrics, err := telemetry.New(config.Gateway)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := gateway.NewServer(ctx, gateway.Options{DB: db, NATS: nc, Chain: 125,
		AuthSecret: "graphql-proof-local-secret-at-least-32-bytes", Node: node, Metrics: metrics})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(srv.Handler())
	t.Cleanup(httpServer.Close)

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate TypeScript consumer sources")
	}
	app := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../../apps/perps"))
	cmd := exec.CommandContext(ctx, "node", "--import", "tsx", "--eval", proofGraphqlConsumer)
	cmd.Dir = app
	cmd.Env = append(os.Environ(), "NEXT_PUBLIC_PERPS_GATEWAY_URL="+httpServer.URL,
		"NEXT_PUBLIC_PAXEER_RPC_URLS="+httpServer.URL, "NODE_ENV=test")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	scanner := bufio.NewScanner(stdout)
	if err := proofGraphqlPhase(scanner, "initial"); err != nil {
		_ = cmd.Wait()
		t.Fatalf("initial consumer reads: %v\n%s", err, stderr.String())
	}
	// Update state while the SAME gateway and TypeScript modules remain alive.
	// A cache shortcut in either side would return the old name, leverage,
	// tier maintenance, or position size on the next two consumer reads.
	proofGraphqlExec(t, db, `UPDATE perps.markets SET name = 'Proof refreshed', max_leverage = 75 WHERE market_id = 211071;
UPDATE perps.market_tiers SET maintenance_bps = 125 WHERE market_id = 211071 AND tier = 2;
UPDATE perps.positions_current SET size_usd = 9007199254740995 WHERE position_id = 211071`)
	if _, err := io.WriteString(stdin, "updated\n"); err != nil {
		t.Fatal(err)
	}
	_ = stdin.Close()
	phaseErr := proofGraphqlPhase(scanner, "complete")
	waitErr := cmd.Wait()
	if phaseErr != nil || waitErr != nil {
		t.Fatalf("consumer proof: phase=%v exit=%v\n%s", phaseErr, waitErr, stderr.String())
	}
	var exposition bytes.Buffer
	if err := metrics.WritePrometheus(&exposition); err != nil {
		t.Fatal(err)
	}
	// Four real POSTs per snapshot, two snapshots per phase, plus five refusal
	// and unknown-ID requests. This also proves repeated reads crossed HTTP.
	const observed = `perps_http_requests_total{route="POST /graphql",status="200"} 21`
	if !strings.Contains(exposition.String(), observed+"\n") {
		t.Fatalf("expected 21 real HTTP reads, metrics:\n%s", exposition.String())
	}
}

func proofGraphqlExec(t *testing.T, db *sql.DB, statement string) {
	t.Helper()
	if _, err := db.Exec(statement); err != nil {
		t.Fatal(err)
	}
}

func proofGraphqlPhase(scanner *bufio.Scanner, want string) error {
	for scanner.Scan() {
		if scanner.Text() == "PROOF:"+want {
			return nil
		}
	}
	return fmt.Errorf("consumer never observed %s: %v", want, scanner.Err())
}

// This is an executable driver, not a reimplementation of the consumers.
// tsx loads the application's own functions, aliases, fetch policy and zod
// decoders. There is no overridden fetch, injected response, or relay server.
const proofGraphqlConsumer = `
const assert = require("node:assert/strict");
const readline = require("node:readline");
const { market, fetchMarkets, subgraphQuery, MARKET_QUERY, POSITIONS_QUERY,
        decodeMarketResult } = require("./src/hooks/perps/useSubgraph.ts");
const { fetchPortfolioPositions, decodePortfolioPositions } = require("./src/hooks/perps/usePortfolioStats.ts");
const address = "0x0000000000000000000000000000000000211071";
const uintMax = "115792089237316195423570985008687907853269984665640564039457584007913129639935";

async function snapshot(updated) {
  const expected = {
    marketId: 211071, name: updated ? "Proof refreshed" : "Proof Bitcoin",
    symbol: "PROOFBTC", oracleSymbol: "BTC", maxLeverage: updated ? "75" : "100", enabled: true,
    marginTiers: [
      {tier: 0, lowerBoundUsd: "0", maxLeverage: "100000000000000000000", maintenanceBps: "50", deductionUsd: "0"},
      {tier: 2, lowerBoundUsd: "9007199254740993", maxLeverage: "20000000000000000000", maintenanceBps: updated ? "125" : "100", deductionUsd: "9007199254740993"},
      {tier: 10, lowerBoundUsd: uintMax, maxLeverage: "1000000000000000000", maintenanceBps: "500", deductionUsd: "1000000000000000000"}
    ]
  };
  assert.deepEqual(await market(211071), expected);
  const markets = await fetchMarkets();
  assert.deepEqual(markets.find(m => m.marketId === 211071), expected);
  assert.deepEqual(markets.map(m => m.marketId), markets.map(m => m.marketId).sort((a,b) => a-b));
  const disabled = await market(211072);
  assert.deepEqual(disabled, {
    marketId: 211072, name: "Proof disabled", symbol: "PROOFOFF", oracleSymbol: "OFF",
    maxLeverage: "20", enabled: false, marginTiers: []
  });
  const positions = await fetchPortfolioPositions(address);
  assert.deepEqual(positions, [{
    positionId: "211071", userAddress: address, marketId: 211071, isLong: true,
    sizeUsd: updated ? "9007199254740995" : "9007199254740993",
    leverage: "10000000000000000000", entryPrice: "50000000000000000000000",
    collateralToken: "0x0000000000000000000000000000000000000001",
    collateralAmount: "1000000", collateralUsd: "1000000000000000000", status: "open",
    openedAt: null, openTxHash: null, market: {symbol: "PROOFBTC"}
  }]);
}

(async () => {
  const lines = readline.createInterface({input: process.stdin});
  const next = lines[Symbol.asyncIterator]();
  await snapshot(false);
  await snapshot(false);
  console.log("PROOF:initial");
  assert.equal((await next.next()).value, "updated");
  lines.close();
  await snapshot(true);
  await snapshot(true);
  assert.equal(await market(2147483647), null);
  await assert.rejects(market(-1), /marketId must be non-negative/);
  await assert.rejects(subgraphQuery(MARKET_QUERY, {marketId: "malformed"}), {
    message: 'could not unmarshal "malformed" (string) into int32: incompatible type: string'
  });
  // Alias values emitted by the real producers into incompatible fields.
  // These HTTP responses are valid GraphQL but invalid consumer results.
  await assert.rejects(subgraphQuery(
    MARKET_QUERY.replace("maxLeverage enabled", "maxLeverage: name enabled"),
    {marketId: 211071}, decodeMarketResult), {name: "ZodError"});
  await assert.rejects(subgraphQuery(
    POSITIONS_QUERY.replace("sizeUsd leverage", "sizeUsd: status leverage"),
    {userAddress: address}, decodePortfolioPositions), {name: "ZodError"});
  console.log("PROOF:complete");
  process.exit(0);
})().catch(error => { console.error(error); process.exit(1); });
`

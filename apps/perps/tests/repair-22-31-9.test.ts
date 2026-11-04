import assert from "node:assert/strict";
import { spawn, execFileSync, type ChildProcess } from "node:child_process";
import { once } from "node:events";
import { chmodSync, chownSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir, userInfo } from "node:os";
import { join, resolve } from "node:path";
import { createInterface } from "node:readline";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { SpotPosition } from "../src/core/spot/api";

// f038: real PostgreSQL -> SpotUsers -> HTTP -> spotApi -> TradeHistoryTab.
// The Go overlay supplies only the scenario/entry point, leaving every
// producer query, JSON encoder, REST client and display decoder unchanged.
const producer = String.raw`package main
import (
    "context"
    "encoding/json"
    "fmt"
    "math/big"
    "net"
    "net/http"
    "os"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/rest"
    "github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
)
type token struct { Address string; Decimals int; Stable bool }
type swap struct { TokenIn, TokenOut, AmountIn, AmountOut, Price string }
type scenario struct { User string; Tokens []token; Swaps []swap }
func must(err error) { if err != nil { panic(err) } }
func main() {
    var seed scenario
    must(json.NewDecoder(os.Stdin).Decode(&seed))
    db, err := store.Open(os.Args[1]); must(err); defer db.Close()
    must(store.Migrate(context.Background(), db))
    account := common.HexToAddress(seed.User).Bytes()
    for i, t := range seed.Tokens {
        _, err = db.Exec(` + "`" + `INSERT INTO perps.spot_token_registry
            (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time, emitter, kind, account, token, decimals, is_stablecoin)
            VALUES (0, $1, 0, $2, 0, $1, 'tx', '2026-01-01T00:00:00Z', $3, 'token_registered', $3, $4, $5, $6)` + "`" + `,
            make([]byte, 32), i, account, common.HexToAddress(t.Address).Bytes(), t.Decimals, t.Stable); must(err)
        _, err = db.Exec(` + "`" + `INSERT INTO perps.spot_orders
            (order_id, account, kind, side, token, stablecoin, amount, status, placed_block)
            VALUES ($1, $2, 'limit', 'sell', $3, $4, 1, 'open', 0)` + "`" + `,
            i + 1, account, common.HexToAddress(t.Address).Bytes(), common.HexToAddress(seed.Tokens[0].Address).Bytes()); must(err)
    }
    for i, s := range seed.Swaps {
        _, err = db.Exec(` + "`" + `INSERT INTO perps.spot_swaps
            (block, block_hash, tx_index, log_index, sub, tx_hash, receipt_kind, block_time, emitter, kind, account, "user",
             token_in, token_out, amount_in, amount_out, execution_price)
            VALUES ($1, $2, 0, 0, 0, $2, 'tx', '2026-01-01T00:00:00Z', $3, 'simple_swap', $3, $3, $4, $5, $6, $7, $8)` + "`" + `,
            i + 1, common.BigToHash(big.NewInt(int64(i + 1))).Bytes(), account,
            common.HexToAddress(s.TokenIn).Bytes(), common.HexToAddress(s.TokenOut).Bytes(), s.AmountIn, s.AmountOut, s.Price); must(err)
    }
    // SpotUsers requires a node provider; these database-only routes make no node calls.
    node, err := ethclient.Dial("http://127.0.0.1:1"); must(err); defer node.Close()
    users, err := rest.NewSpotUsers(db, node); must(err)
    mux := http.NewServeMux(); users.Register(mux)
    listener, err := net.Listen("tcp", "127.0.0.1:0"); must(err)
    fmt.Println("http://" + listener.Addr().String())
    must(http.Serve(listener, mux))
}
`;

async function f038SpotUsersRestRowsRenderNativeUnitsAndExecutionPrice() {
  const root = resolve("../..");
  const work = mkdtempSync(join(tmpdir(), "repair-22-31-9-"));
  const pgBin = execFileSync("pg_config", ["--bindir"], { encoding: "utf8" }).trim();
  const asPostgres = userInfo().uid === 0;
  if (asPostgres) {
    chownSync(work, Number(execFileSync("id", ["-u", "postgres"])), Number(execFileSync("id", ["-g", "postgres"])));
  }
  chmodSync(work, 0o755);
  const pg = (command: string, args: string[]) => execFileSync(
    asPostgres ? "runuser" : join(pgBin, command),
    asPostgres ? ["-u", "postgres", "--", join(pgBin, command), ...args] : args,
    { cwd: work, encoding: "utf8", stdio: "pipe" },
  );
  let process: ChildProcess | undefined;
  let started = false;
  const previousGateway = globalThis.process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
  try {
    pg("initdb", ["-D", join(work, "db"), "--auth=trust", "--no-locale", "--encoding=UTF8", "-U", "postgres"]);
    pg("pg_ctl", ["-D", join(work, "db"), "-l", join(work, "postgres.log"), "-o", `-F -k ${work} -h ''`, "-w", "start"]);
    started = true;
    const { SPOT_TOKENS } = await import("../src/core/spot/addresses");
    const { parsePrice, parseTokenAmount } = await import("../src/core/spot/slippage");
    const usdl = SPOT_TOKENS.USDL.address;
    const sid = SPOT_TOKENS.SID.address;
    const token18 = "0x0000000000000000000000000000000000000018";
    const token8 = "0x0000000000000000000000000000000000000008";
    const token0 = "0x0000000000000000000000000000000000000010";
    const unknown = "0x0000000000000000000000000000000000000099";
    const address = "0x00000000000000000000000000000000000000ab";
    const cases = [
      { tokenIn: sid, tokenOut: usdl, amountIn: "1", amountOut: "2.5", decimalsIn: 6, decimalsOut: 6, price: "2.5", cells: ["1", "2.5", "$2.50", "$2.50"] },
      { tokenIn: usdl, tokenOut: sid, amountIn: "2.5", amountOut: "1", decimalsIn: 6, decimalsOut: 6, price: "2.5", cells: ["2.5", "1", "$2.50", "$2.50"] },
      { tokenIn: token18, tokenOut: usdl, amountIn: "1", amountOut: "3.25", decimalsIn: 18, decimalsOut: 6, price: "3.25", cells: ["1", "3.25", "$3.25", "$3.25"] },
      { tokenIn: usdl, tokenOut: token18, amountIn: "3.25", amountOut: "1", decimalsIn: 6, decimalsOut: 18, price: "3.25", cells: ["3.25", "1", "$3.25", "$3.25"] },
      { tokenIn: token8, tokenOut: usdl, amountIn: "1.234567", amountOut: "0.125", decimalsIn: 8, decimalsOut: 6, price: "0.10125", cells: ["1.234567", "0.125", "$0.10", "$0.13"] },
      { tokenIn: token0, tokenOut: usdl, amountIn: "2", amountOut: "4.5", decimalsIn: 0, decimalsOut: 6, price: "2.25", cells: ["2", "4.5", "$2.25", "$4.50"] },
      { tokenIn: unknown, tokenOut: usdl, amountIn: "1", amountOut: "1", decimalsIn: 18, decimalsOut: 6, price: "1", cells: ["Amount unavailable", "1", "$1.00", "$1.00"] },
    ];
    const seed = {
      User: address,
      Tokens: [
        { Address: usdl, Decimals: 6, Stable: true },
        { Address: sid, Decimals: 6, Stable: false },
        { Address: token18, Decimals: 18, Stable: false },
        { Address: token8, Decimals: 8, Stable: false },
        { Address: token0, Decimals: 0, Stable: false },
      ],
      Swaps: cases.map((row) => ({
        TokenIn: row.tokenIn, TokenOut: row.tokenOut,
        AmountIn: parseTokenAmount(row.amountIn, row.decimalsIn).toString(),
        AmountOut: parseTokenAmount(row.amountOut, row.decimalsOut).toString(),
        Price: parsePrice(row.price).toString(),
      })),
    };
    const main = join(work, "main.go");
    const overlay = join(work, "overlay.json");
    writeFileSync(main, producer);
    writeFileSync(overlay, JSON.stringify({ Replace: {
      [join(root, "services/platform/perps-data/cmd/perps-gateway/main.go")]: main,
    } }));
    process = spawn("go", ["run", "-mod=readonly", "-overlay", overlay, "./services/platform/perps-data/cmd/perps-gateway", `postgresql://postgres@/postgres?host=${work}&sslmode=disable`], {
      cwd: root, detached: true, stdio: ["pipe", "pipe", "pipe"],
      env: { ...globalThis.process.env, GOMAXPROCS: "2" },
    });
    let stderr = "";
    process.stderr!.on("data", (chunk) => { stderr += chunk; });
    const lines = createInterface({ input: process.stdout! });
    const origin = new Promise<string>((resolveOrigin, reject) => {
      lines.once("line", resolveOrigin);
      process!.once("error", reject);
      process!.once("exit", (code) => reject(new Error(`SpotUsers producer exited ${code}: ${stderr}`)));
    });
    process.stdin!.end(JSON.stringify(seed));
    globalThis.process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = await origin;
    const { spotApi } = await import("../src/core/spot/api");
    const { TradeHistoryTab } = await import("../src/widgets/spot/SpotPositionsPanel");
    const orders = await spotApi.getOrders(address);
    const tokenMetadata: Array<Pick<SpotPosition, "token" | "decimals">> = orders.items.flatMap((order) =>
      order.token == null ? [] : [{ token: order.token.toUpperCase(), decimals: order.decimals }],
    );
    assert.equal(tokenMetadata.find((token) => token.token.toLowerCase() === token18)?.decimals, 18);
    assert.equal(tokenMetadata.find((token) => token.token.toLowerCase() === token0)?.decimals, 0);
    let cursor: { afterBlock: string; afterIndex: number } | null = null;
    const seen = new Set<string>();
    do {
      const page = await spotApi.getTrades(address.toUpperCase(), { limit: 2, ...cursor });
      assert.ok(page.items.length > 0);
      const before = JSON.stringify(page);
      const html = renderToStaticMarkup(createElement(TradeHistoryTab, {
        address, initial: page, initialError: null, refresh: () => {}, tokenMetadata,
      }));
      const rows = [...html.matchAll(/<tbody>([\s\S]*?)<\/tbody>/g)]
        .flatMap((body) => [...body[1].matchAll(/<tr\b[^>]*>([\s\S]*?)<\/tr>/g)]);
      assert.equal(rows.length, page.items.length);
      for (const [i, row] of page.items.entries()) {
        const index = Number(row.blockNumber) - 1;
        const expected = cases[index];
        assert.ok(expected);
        assert.equal(row.amountIn, seed.Swaps[index].AmountIn);
        assert.equal(row.amountOut, seed.Swaps[index].AmountOut);
        assert.equal(row.executionPrice, seed.Swaps[index].Price);
        const cells = [...rows[i][1].matchAll(/<td\b[^>]*>([\s\S]*?)<\/td>/g)]
          .map((cell) => cell[1].replace(/<[^>]*>/g, ""));
        assert.deepEqual(cells.slice(2, 6), expected.cells, `f038 REST block ${row.blockNumber}`);
        assert.equal(seen.has(String(row.id)), false);
        seen.add(String(row.id));
      }
      assert.equal(JSON.stringify(page), before, "display must retain raw gateway quantities");
      cursor = page.nextCursor;
    } while (cursor);
    assert.equal(seen.size, cases.length);
  } finally {
    if (process?.pid && process.exitCode === null) {
      const exited = once(process, "exit");
      globalThis.process.kill(-process.pid, "SIGTERM");
      await exited;
    }
    if (started) pg("pg_ctl", ["-D", join(work, "db"), "-m", "fast", "-w", "stop"]);
    if (previousGateway === undefined) delete globalThis.process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
    else globalThis.process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = previousGateway;
    rmSync(work, { recursive: true, force: true });
  }
}

test(f038SpotUsersRestRowsRenderNativeUnitsAndExecutionPrice);

import { spawn } from "node:child_process";
import { createServer, request as httpRequest } from "node:http";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { environment } from "./environment.mjs";

const require = createRequire(
  new URL("../../../../apps/perps/package.json", import.meta.url),
);
const { WebSocketServer } = require("next/dist/compiled/ws");
const ts = require("typescript");
const { decodeFunctionData, encodeFunctionResult } = require("viem");
const abiSource = readFileSync(
  "apps/perps/packages/paxeer-backend/src/core/network/abi/PerpsPrecompile.ts",
  "utf8",
);
const abiModule = { exports: {} };
new Function(
  "exports",
  ts.transpileModule(abiSource, {
    compilerOptions: { module: ts.ModuleKind.CommonJS },
  }).outputText,
)(abiModule.exports);
const abi = abiModule.exports.PerpsPrecompileABI;
const fixture = (name) =>
  JSON.parse(readFileSync(new URL(`answers/${name}.json`, import.meta.url)));
const tradingViewFixture = readFileSync(
  new URL("answers/tradingview.js", import.meta.url),
);
const catalog = fixture("markets").data.markets;
const app = spawn("pnpm", ["--dir", "apps/perps", "start"], {
  stdio: "inherit",
  env: { ...environment, HOSTNAME: "127.0.0.1" },
});
function reply(response, body, status = 200) {
  response.writeHead(status, {
    "content-type": "application/json",
    "cache-control": "no-store",
  });
  response.end(JSON.stringify(body));
}
function state(request) {
  return new URL(
    request.headers.referer ?? "/",
    environment.NEXT_PUBLIC_PERPS_GATEWAY_URL,
  ).searchParams.get("renderState");
}
function rpc(message) {
  let result;
  if (message.method === "eth_chainId") result = "0x7d";
  else if (message.method === "eth_blockNumber") result = "0x1";
  else if (message.method === "eth_call") {
    const { functionName, args } = decodeFunctionData({
      abi,
      data: message.params[0].data,
    });
    const marketId = Number(args?.[0] ?? 0);
    const row = catalog.find((row) => row.marketId === marketId);
    let value;
    if (functionName === "getMarketConfig" && row)
      value = {
        ...row,
        marketId: BigInt(marketId),
        tickSize: BigInt(row.tickSize),
        lotSize: BigInt(row.lotSize),
        maxLeverage: 20n,
        impactNotionalUsd: 100000n * 10n ** 18n,
        oiCapUsd: 1000000n * 10n ** 18n,
        fundingCapPpmPerHour: 100n,
        listedAt: 0n,
      };
    else if (functionName === "getMarginTiers")
      value = [
        {
          tier: 0n,
          lowerBoundUsd: 0n,
          maxLeverage: 20n,
          maintenanceBps: 500n,
          deductionUsd: 0n,
        },
      ];
    else if (functionName === "getMarkPriceDetails" && row) {
      const price = BigInt(row.latestPrice.price) * 10n ** 18n;
      value = [price, price, price, true];
    } else if (functionName === "getActiveMarketIds")
      value = catalog.map((row) => BigInt(row.marketId));
    else
      return {
        jsonrpc: "2.0",
        id: message.id,
        error: { code: -32601, message: `No fixture for ${functionName}` },
      };
    result = encodeFunctionResult({ abi, functionName, result: value });
  } else
    return {
      jsonrpc: "2.0",
      id: message.id,
      error: { code: -32601, message: `No fixture for ${message.method}` },
    };
  return { jsonrpc: "2.0", id: message.id, result };
}
const server = createServer(async (incoming, outgoing) => {
  const url = new URL(incoming.url, environment.NEXT_PUBLIC_PERPS_GATEWAY_URL);
  try {
    if (url.pathname === "/rpc") {
      let body = "";
      for await (const chunk of incoming) body += chunk;
      const messages = JSON.parse(body);
      return reply(
        outgoing,
        Array.isArray(messages) ? messages.map(rpc) : rpc(messages),
      );
    }
    if (
      url.pathname === "/static/charting_library/charting_library.standalone.js"
    ) {
      outgoing.writeHead(200, {
        "content-type": "text/javascript; charset=utf-8",
        "cache-control": "no-store",
      });
      return outgoing.end(tradingViewFixture);
    }
    if (url.pathname === "/graphql") {
      let body = "";
      for await (const chunk of incoming) body += chunk;
      const { query } = JSON.parse(body);
      if (/\bmarkets\s*\(/.test(query) || /\bmarkets\s*\{/.test(query)) {
        const selected = state(incoming);
        if (selected === "loading")
          await new Promise((resolve) => setTimeout(resolve, 3000));
        return reply(
          outgoing,
          fixture(
            selected === "empty" || selected === "error" ? selected : "markets",
          ),
        );
      }
      return reply(outgoing, fixture("stats"));
    }
    if (url.pathname === "/api/price-feed/ws-token") {
      const symbol = url.searchParams.get("symbol");
      return reply(outgoing, {
        symbol,
        token: "render-session",
        url: `${environment.NEXT_PUBLIC_PERPS_GATEWAY_URL.replace(/^http/, "ws")}/ws/${symbol}`,
        expiresAt: new Date(Date.now() + 300000).toISOString(),
      });
    }
    if (url.pathname === "/api/tickers") {
      const selected = state(incoming);
      if (selected === "loading")
        await new Promise((resolve) => setTimeout(resolve, 3000));
      if (selected === "error")
        return reply(outgoing, { success: false, ...fixture("error") });
      const body = fixture("tickers");
      return reply(outgoing, {
        ...body,
        data:
          selected === "empty"
            ? []
            : body.data.map((row) => ({ ...row, updated_at: Date.now() })),
      });
    }
    if (url.pathname.startsWith("/api/ticker/")) {
      const symbol = url.pathname.split("/").pop();
      const row = catalog.find((row) => row.symbol === symbol);
      if (!row) return reply(outgoing, { error: "Unknown market" }, 404);
      return reply(outgoing, {
        success: true,
        data: {
          mark_price: Number(row.latestPrice.price),
          index_price: Number(row.latestPrice.price),
          last_funding_rate_hourly: "0",
        },
      });
    }
    if (url.pathname.startsWith("/api/orderbook/"))
      return reply(outgoing, fixture("book"));
    if (url.pathname.startsWith("/api/trades/"))
      return reply(outgoing, fixture("trades"));
    if (url.pathname === "/api/open-interest")
      return reply(outgoing, {
        success: true,
        data: catalog.map((row) => ({
          paxeerSymbol: row.symbol,
          long_oi: 0,
          short_oi: 0,
        })),
      });
    if (/^\/api\/price-feed\/[^/]+\/history$/.test(url.pathname))
      return reply(outgoing, fixture("history"));
    if (/^\/api\/price-feed\/[^/]+\/config$/.test(url.pathname))
      return reply(outgoing, {
        supports_search: false,
        supports_group_request: false,
        supports_marks: false,
        supports_timescale_marks: false,
        supports_time: false,
        supported_resolutions: ["1", "5", "15", "60", "1D"],
      });
    if (/^\/api\/price-feed\/[^/]+\/symbols$/.test(url.pathname)) {
      const symbol = url.searchParams.get("symbol") ?? "ETH";
      return reply(outgoing, {
        name: symbol,
        ticker: symbol,
        description: symbol,
        type: "crypto",
        session: "24x7",
        timezone: "Etc/UTC",
        exchange: "Paxeer",
        minmov: 1,
        pricescale: 100,
        has_intraday: true,
        has_daily: true,
        supported_resolutions: ["1", "5", "15", "60", "1D"],
        volume_precision: 3,
        data_status: "streaming",
      });
    }
    const upstream = httpRequest(
      {
        hostname: "127.0.0.1",
        port: 3000,
        method: incoming.method,
        path: incoming.url,
        headers: { ...incoming.headers, connection: "close" },
        agent: false,
      },
      (response) => {
        outgoing.writeHead(response.statusCode, response.headers);
        response.pipe(outgoing);
      },
    );
    upstream.on("error", (error) => {
      if (!outgoing.headersSent) reply(outgoing, { error: error.message }, 502);
      else outgoing.end();
    });
    incoming.pipe(upstream);
  } catch (error) {
    reply(outgoing, { error: error.message }, 500);
  }
});
const sockets = new WebSocketServer({ noServer: true });
server.on("upgrade", (request, socket, head) => {
  if (request.url.startsWith("/ws"))
    sockets.handleUpgrade(request, socket, head, (ws) =>
      sockets.emit("connection", ws, request),
    );
  else socket.destroy();
});
sockets.on("connection", (ws, request) => {
  const selected = state(request);
  ws.on("message", (raw) => {
    const message = JSON.parse(String(raw));
    if (message.event === "ping")
      return ws.send(JSON.stringify({ event: "pong" }));
    if (message.event !== "subscribe") return;
    const topic = message.topic;
    let data;
    if (topic === "markets@all")
      data =
        selected === "empty"
          ? []
          : catalog.map((row) => ({
              symbol: row.symbol,
              price: Number(row.latestPrice.price),
              change_24h: 0,
              funding_8h: "0",
              perp_funding_rate: "0",
              last_funding_rate_hourly: "0",
              price_updated_at: Date.now(),
              price_24h: Number(row.latestPrice.price),
              high_24h: Number(row.latestPrice.price),
              low_24h: Number(row.latestPrice.price),
            }));
    else if (topic === "markets@totals")
      data = {
        total_trades: 0,
        volume_24h: 0,
        volume_7d: 0,
        volume_30d: 0,
        total_oi: 0,
      };
    else if (topic.endsWith("@perp_orderbook")) data = fixture("book").data;
    else if (topic.endsWith("@perp_trade"))
      data = { trades: fixture("trades").data };
    else if (topic.endsWith("@perp_stats")) {
      const symbol = topic.split("@")[0];
      const row = catalog.find((item) => item.symbol === symbol);
      data = {
        symbol,
        mark_price: Number(row?.latestPrice.price ?? 0),
        index_price: Number(row?.latestPrice.price ?? 0),
        last_funding_rate_hourly: "0",
        open_interest: 0,
        volume_24h: 0,
      };
    } else if (topic.includes("@kline_")) return;
    else throw new Error(`No fixture for topic ${topic}`);
    ws.send(
      JSON.stringify({ topic, data, ts: Date.now(), seq: 1, snapshot: true }),
    );
  });
});
server.listen(4173);
function stop() {
  for (const ws of sockets.clients) ws.terminate();
  sockets.close();
  server.close();
  app.kill("SIGTERM");
}
process.on("SIGTERM", stop);
process.on("SIGINT", stop);
app.on("exit", (code) => {
  if (code) process.exit(code);
});

/**
 * Proves: hooks/perps/useQuoter.useQuoter -> exported hooks over viem/HTTP/GraphQL -> usePosition.getClosePlan and trading/e2e consumers.
 * Also covers useFeeTier/useMarketConfig/usePerpsMarkets/useSpotTokenState
 * (hooks/perps and hooks/spot) -> their exported TypeScript modules and
 * useSubgraph.MARKETS_QUERY over viem/HTTP/GraphQL -> usePosition.getClosePlan,
 * PerpsOrderForm.orderLimitsFromConfig, PerpsPositionsPanel.quotePanelPartialClose
 * and positionMarketLabels, and e2e-local-node.quotedOpenWithTpSl.
 *
 * The application harness owns its read state and serves ABI-encoded reads on
 * loopback; it replaces no exported module, fetch, viem client or decoder.
 * Each pass fetches through a fresh QueryClient before rendering the real hooks.
 * Repairs: useQuoter refused no malformed/zero-price quotes and now retains
 * decodeOpenPositionQuote with uint256/int256 bounds; useSubgraph and
 * usePerpsMarkets trusted malformed/duplicate gateway identifiers; useMarketConfig
 * admitted unordered margin tiers; usePosition admitted inactive close plans.
 * The proof also checks ABI truncation at the fee and spot hook boundaries.
 * There is no persistent hook state to reopen, and no workspace dist imports.
 */
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { once } from "node:events";
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import { test } from "node:test";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  decodeFunctionData,
  encodeFunctionResult,
  type Address,
  type ContractFunctionReturnType,
  type Hex,
} from "viem";
import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { pecorVaultAbi } from "../src/core/network/spot_abi/PECORVault";
import type { MarketConfig, MarginTier } from "../src/core/perps/marketConfig";
import { nodeMarketConfig, nodeMarginTiers } from "./node-leverage-output";

const WAD = 10n ** 18n;
const ACCOUNT: Address = "0x1234567890123456789012345678901234567890";
const TOKEN: Address = "0x85FcD13735F4309833A503EE804ea32395851479";

type RpcRequest = {
  id: number;
  method: string;
  params: readonly [{ to: Address; data: Hex }];
};

class HooksApplication {
  readonly config: MarketConfig = { ...nodeMarketConfig };
  tiers: readonly MarginTier[] = nodeMarginTiers.map((tier) => ({ ...tier }));
  fee: ContractFunctionReturnType<
    typeof PerpsPrecompileABI,
    "view",
    "getFeeTier"
  > = [2, 250n, -50n, 9_007_199_254_740_993n];
  info: ContractFunctionReturnType<
    typeof pecorVaultAbi,
    "view",
    "getTokenInfo"
  > = [true, false, 6, 100_000_000n, 100_000_000n, 0n];
  paused = false;
  active = true;
  price = 2n * WAD;
  corruptMethod = "";
  corruptCatalog = false;
  duplicateCatalog = false;
  zeroQuote = false;
  marketsQuery = "";
  readonly reads = new Map<string, number>();
  readonly failures: unknown[] = [];

  count(method: string) {
    this.reads.set(method, (this.reads.get(method) ?? 0) + 1);
  }

  rpc(request: RpcRequest) {
    if (request.method === "net_listening") {
      return { jsonrpc: "2.0", id: request.id, result: true };
    }
    if (request.method === "eth_blockNumber")
      return { jsonrpc: "2.0", id: request.id, result: "0x1" };
    assert.equal(
      request.method,
      "eth_call",
      "hooks must use real eth_call reads",
    );
    const call = request.params[0];
    const vault = call.to.toLowerCase().endsWith("1025");
    if (vault) {
      const decoded = decodeFunctionData({
        abi: pecorVaultAbi,
        data: call.data,
      });
      this.count(decoded.functionName);
      if (decoded.functionName === this.corruptMethod) {
        return { jsonrpc: "2.0", id: request.id, result: "0x01" };
      }
      assert.equal(decoded.args?.[0], TOKEN);
      switch (decoded.functionName) {
        case "isTokenPaused":
          return {
            jsonrpc: "2.0",
            id: request.id,
            result: encodeFunctionResult({
              abi: pecorVaultAbi,
              functionName: "isTokenPaused",
              result: this.paused,
            }),
          };
        case "getTokenInfo":
          return {
            jsonrpc: "2.0",
            id: request.id,
            result: encodeFunctionResult({
              abi: pecorVaultAbi,
              functionName: "getTokenInfo",
              result: this.info,
            }),
          };
        default:
          throw new Error(`Unexpected vault producer: ${decoded.functionName}`);
      }
    }
    assert.ok(call.to.toLowerCase().endsWith("1015"));
    const decoded = decodeFunctionData({
      abi: PerpsPrecompileABI,
      data: call.data,
    });
    const name = decoded.functionName;
    this.count(name);
    if (name === this.corruptMethod)
      return { jsonrpc: "2.0", id: request.id, result: "0x01" };
    let result: Hex;
    switch (name) {
      case "getMarketConfig":
        if (decoded.args[0] !== this.config.marketId) {
          return {
            jsonrpc: "2.0",
            id: request.id,
            error: {
              code: -32000,
              message:
                "apps/perps/src/hooks/perps/useMarketConfig.ts: unknown market",
            },
          };
        }
        result = encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: name,
          result: this.config,
        });
        break;
      case "getMarginTiers":
        result = encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: name,
          result: this.tiers,
        });
        break;
      case "getFeeTier":
        assert.equal(decoded.args[0], ACCOUNT);
        result = encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: name,
          result: this.fee,
        });
        break;
      case "getMarkPriceDetails":
        result = encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: name,
          result: [this.price, this.price, this.price, true],
        });
        break;
      case "quoteOpenPosition": {
        const [, token, amount, leverage] = decoded.args;
        assert.equal(token, TOKEN);
        const collateralUsd = amount * 10n ** 12n;
        result = encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: name,
          result: {
            entryPrice: this.zeroQuote ? 0n : this.price,
            sizeUsd: (collateralUsd * leverage) / WAD,
            collateralUsd,
            leverage,
            tradingFee: (amount * this.fee[1]) / 1_000_000n,
            tradingFeeUsd: (collateralUsd * this.fee[1]) / 1_000_000n,
            priceImpact: 0n,
            liquidationPrice: 0n,
            estimatedFunding24h: -10n,
            maintenanceMarginBps: this.tiers[0].maintenanceBps,
          },
        });
        break;
      }
      case "quoteClosePosition":
        result = encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: name,
          result: {
            exitPrice: this.zeroQuote ? 0n : this.price,
            unrealizedPnl: -WAD,
            tradingFee: 1n,
            tradingFeeUsd: 10n ** 12n,
            fundingOwed: -1n,
            netPnl: -WAD,
            estimatedPayout: WAD,
          },
        });
        break;
      case "quotePartialClose":
        assert.equal(decoded.args[0], 7n);
        assert.equal(decoded.args[1], 100n * WAD);
        result = encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: name,
          result: [this.zeroQuote ? 0n : this.price, -WAD, 1n, WAD],
        });
        break;
      case "getPosition":
        result = encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: name,
          result: [
            ACCOUNT,
            this.config.marketId,
            true,
            200n * WAD,
            10n * WAD,
            TOKEN,
            10_000_000n,
            this.price,
            1_750_000_000n,
            this.active,
          ],
        });
        break;
      default:
        throw new Error(`Unexpected perps producer: ${name}`);
    }
    return { jsonrpc: "2.0", id: request.id, result };
  }

  async serve(request: IncomingMessage, response: ServerResponse) {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks).toString();
      response.setHeader("Content-Type", "application/json");
      if (request.url === "/rpc") {
        const rpc = JSON.parse(body) as RpcRequest | RpcRequest[];
        response.end(
          JSON.stringify(
            Array.isArray(rpc)
              ? rpc.map((value) => this.rpc(value))
              : this.rpc(rpc),
          ),
        );
      } else if (request.url === "/graphql") {
        const operation = JSON.parse(body) as { query: string };
        assert.equal(
          operation.query,
          this.marketsQuery,
          "the consumer sends the exported MARKETS_QUERY unchanged",
        );
        this.count("MARKETS_QUERY");
        const row = {
          ...this.config,
          marketId: this.corruptCatalog
            ? "invalid"
            : Number(this.config.marketId),
          maxLeverage: this.config.maxLeverage.toString(),
          tickSize: this.config.tickSize.toString(),
          lotSize: this.config.lotSize.toString(),
          createdAt: new Date(
            Number(this.config.listedAt) * 1000,
          ).toISOString(),
          marginTiers: this.tiers.map((tier, index) => ({
            tier: index,
            ...Object.fromEntries(
              Object.entries(tier).map(([key, value]) => [
                key,
                value.toString(),
              ]),
            ),
          })),
          latestPrice: {
            price: this.price.toString(),
            onchainTimestamp: 1_750_000_000,
          },
          fundingRate: { ratePerSecond: "0", rate24h: "0" },
        };
        // Serialize only the GraphQL selection; the store itself retains bigint state.
        const {
          impactNotionalUsd,
          oiCapUsd,
          fundingCapPpmPerHour,
          listedAt,
          ...selected
        } = row;
        response.end(
          JSON.stringify({
            data: {
              markets: this.duplicateCatalog
                ? [selected, selected]
                : [selected],
            },
          }),
        );
      } else if (request.url === `/api/markets/ticker/${this.config.symbol}`) {
        this.count("ticker");
        response.end(
          JSON.stringify({
            success: true,
            data: {
              index_price: Number(this.price / WAD),
              mark_price: Number(this.price / WAD),
              last_funding_rate_hourly: "-0.0001",
            },
          }),
        );
      } else if (request.url === "/api/markets/open-interest") {
        this.count("open-interest");
        response.end(
          JSON.stringify({
            success: true,
            data: [
              { paxeerSymbol: this.config.symbol, long_oi: 20, short_oi: 10 },
            ],
          }),
        );
      } else {
        throw new Error(`Unexpected hooks producer path: ${request.url}`);
      }
    } catch (error) {
      this.failures.push(error);
      response.statusCode = 500;
      response.end(JSON.stringify({ error: String(error) }));
    }
  }
}

async function runHooksPath() {
  const app = new HooksApplication();
  const server = createServer((request, response) => {
    void app.serve(request, response);
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(
    address && typeof address !== "string",
    "apps/perps/tests/proof_hooks.test.ts must host its producer",
  );
  const origin = `http://127.0.0.1:${address.port}`;
  // All endpoints are created here. No operator configuration is read.
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = `${origin}/rpc`;
  const previousReact = Object.getOwnPropertyDescriptor(globalThis, "React");
  Object.defineProperty(globalThis, "React", {
    configurable: true,
    value: React,
  });
  try {
    const fee = await import("../src/hooks/perps/useFeeTier");
    const config = await import("../src/hooks/perps/useMarketConfig");
    const markets = await import("../src/hooks/perps/usePerpsMarkets");
    const spot = await import("../src/hooks/spot/useSpotTokenState");
    const subgraph = await import("../src/hooks/perps/useSubgraph");
    const { useQuoter, decodeOpenPositionQuote } = await import(
      "../src/hooks/perps/useQuoter"
    );
    const { usePosition } = await import("../src/hooks/perps/usePosition");
    const form = await import("../src/widgets/trading/PerpsOrderForm");
    const panel = await import("../src/widgets/trading/PerpsPositionsPanel");
    const { quotedOpenWithTpSl } = await import("./e2e-local-node.test");
    const { marketLabel } = await import("../src/core/perps/marketLabel");
    const { PortfolioFeeTierPanel } = await import(
      "../src/widgets/portfolio/PortfolioFeeTierPanel"
    );
    app.marketsQuery = subgraph.MARKETS_QUERY;
    const quoter = useQuoter(`${origin}/api/markets`);
    const input = {
      marketId: app.config.marketId,
      collateralToken: TOKEN,
      collateralAmount: 10_000_000n,
      leverage: 20n * WAD,
      isLong: true,
    };

    for (let pass = 1; pass <= 2; pass++) {
      app.price = BigInt(pass + 1) * WAD;
      app.fee = [
        pass + 1,
        BigInt(pass * 250),
        -50n,
        9_007_199_254_740_993n + BigInt(pass),
      ];
      const before = new Map(app.reads);
      const client = new QueryClient();
      try {
        await Promise.all([
          client.fetchQuery(fee.feeTierQueryOptions(ACCOUNT)),
          client.fetchQuery(
            config.marketConfigQueryOptions(app.config.marketId),
          ),
          client.fetchQuery(markets.perpsMarketsQueryOptions()),
          client.fetchQuery(spot.spotTokenStateQueryOptions(TOKEN)),
        ]);
        let renders = 0;
        function Consumer() {
          renders++;
          const fees = fee.useFeeTier(ACCOUNT);
          const limits = config.useMarketConfig(app.config.marketId);
          const catalog = markets.usePerpsMarkets();
          const token = spot.useSpotTokenState(TOKEN);
          assert.equal(fees.error, null);
          assert.equal(fees.view?.takerBps, pass * 2.5);
          assert.ok(fees.view);
          assert.equal(
            fees.view.volume14dUsd,
            9_007_199_254_740_993n + BigInt(pass),
          );
          assert.ok(limits.data);
          assert.deepEqual(
            form.orderLimitsFromConfig(limits.data, 2_000_000n * WAD),
            {
              tickSize: app.config.tickSize,
              lotSize: app.config.lotSize,
              maxLeverage: 500n * WAD,
            },
          );
          assert.equal(
            marketLabel(
              app.config.marketId,
              panel.positionMarketLabels(catalog.catalog),
            ),
            app.config.symbol,
          );
          assert.deepEqual(
            catalog.tradeable.map((row) => row.marketId),
            [Number(app.config.marketId)],
          );
          assert.deepEqual(token.state, {
            tradeable: true,
            paused: false,
            reason: null,
          });
          return React.createElement(PortfolioFeeTierPanel, {
            address: ACCOUNT,
          });
        }
        const html = renderToStaticMarkup(
          React.createElement(
            QueryClientProvider,
            { client },
            React.createElement(Consumer),
          ),
        );
        assert.equal(renders, 1);
        assert.ok(
          html.includes(
            `Tier ${pass + 1} · Taker ${pass * 2.5} bps / Maker -0.5 bps`,
          ),
        );
        const quote = await quoter.quoteMarket(app.config.marketId);
        assert.equal(quote.markPrice, app.price);
        assert.equal(quote.lastSettledFundingRate1h, -100_000_000_000_000n);
        assert.equal(quote.fundingRate24h, -2_400_000_000_000_000n);
        assert.equal(quote.longOI, 20n * WAD);
        const opened = await quotedOpenWithTpSl(
          quoter,
          input,
          app.config.tickSize,
        );
        assert.equal(opened.quote.entryPrice, app.price);
        assert.equal(opened.quote.sizeUsd, 200n * WAD);
        assert.deepEqual(decodeOpenPositionQuote(opened.quote), opened.quote);
        for (const malformed of [
          { ...opened.quote, entryPrice: 1n << 256n },
          { ...opened.quote, estimatedFunding24h: 1n << 255n },
          { ...opened.quote, estimatedFunding24h: -(1n << 255n) - 1n },
        ]) {
          assert.throws(() => decodeOpenPositionQuote(malformed));
        }
        assert.equal(opened.plan[0].functionName, "placeOrder");
        const call = await panel.quotePanelPartialClose(
          { positionId: 7n, isLong: true, sizeUsd: 200n * WAD, bps: 5000 },
          quoter,
        );
        assert.equal(call.functionName, "closePositionWithParams");
        assert.equal(call.args[1], 100n * WAD);
        assert.equal(call.args[2], (app.price * 9950n) / 10_000n);
        const close = await usePosition().getClosePlan(7n);
        assert.equal(close[0].functionName, "closePositionWithParams");
        assert.equal(close[0].args[2], (app.price * 9950n) / 10_000n);
        for (const name of [
          "getFeeTier",
          "getMarketConfig",
          "getMarginTiers",
          "isTokenPaused",
          "getTokenInfo",
          "MARKETS_QUERY",
          "ticker",
          "quoteOpenPosition",
          "quotePartialClose",
          "quoteClosePosition",
        ]) {
          assert.ok(
            (app.reads.get(name) ?? 0) > (before.get(name) ?? 0),
            `${name}: pass ${pass} reaches the producer again`,
          );
        }
      } finally {
        client.clear();
      }
    }

    const client = new QueryClient();
    try {
      app.config.enabled = false;
      const disabled = await client.fetchQuery(
        config.marketConfigQueryOptions(app.config.marketId),
      );
      assert.equal(form.orderLimitsFromConfig(disabled, 200n * WAD), null);
      assert.deepEqual((await markets.loadPerpsMarkets()).tradeable, []);
      assert.equal(
        (await quoter.quoteMarket(app.config.marketId)).enabled,
        false,
      );
      app.config.enabled = true;
      app.paused = true;
      assert.deepEqual(
        await client.fetchQuery(spot.spotTokenStateQueryOptions(TOKEN)),
        { tradeable: false, paused: true, reason: "TokenPaused" },
      );
      app.paused = false;
      app.active = false;
      await assert.rejects(usePosition().getClosePlan(7n), /Inactive position/);
      app.active = true;
      await assert.rejects(quoter.quoteMarket(999n), /unknown market/);
      app.zeroQuote = true;
      await assert.rejects(
        quotedOpenWithTpSl(quoter, input, app.config.tickSize),
        /Too small|greater than|>0/,
      );
      await assert.rejects(
        panel.quotePanelPartialClose(
          { positionId: 7n, isLong: true, sizeUsd: 200n * WAD, bps: 5000 },
          quoter,
        ),
        /Too small|greater than|>0/,
      );
      await assert.rejects(
        usePosition().getClosePlan(7n),
        /Too small|greater than|>0/,
      );
      app.zeroQuote = false;
      app.corruptCatalog = true;
      await assert.rejects(markets.loadPerpsMarkets(), /marketId/);
      await assert.rejects(subgraph.fetchMarkets(), /marketId/);
      app.corruptCatalog = false;
      app.duplicateCatalog = true;
      await assert.rejects(
        markets.loadPerpsMarkets(),
        /Duplicate gateway market identifier/,
      );
      app.duplicateCatalog = false;
      app.tiers = [...app.tiers].reverse();
      const invalidTiers = new QueryClient();
      try {
        await assert.rejects(
          invalidTiers.fetchQuery(
            config.marketConfigQueryOptions(app.config.marketId),
          ),
          /Market order limits are invalid/,
        );
      } finally {
        invalidTiers.clear();
      }
      app.tiers = [...app.tiers].reverse();
      for (const method of ["getFeeTier", "getTokenInfo"]) {
        app.corruptMethod = method;
        const failed = new QueryClient();
        try {
          if (method === "getFeeTier") {
            await assert.rejects(
              failed.fetchQuery(fee.feeTierQueryOptions(ACCOUNT)),
            );
            function RefusedFee() {
              assert.equal(fee.useFeeTier(ACCOUNT).view, undefined);
              return React.createElement(PortfolioFeeTierPanel, {
                address: ACCOUNT,
              });
            }
            assert.match(
              renderToStaticMarkup(
                React.createElement(
                  QueryClientProvider,
                  { client: failed },
                  React.createElement(RefusedFee),
                ),
              ),
              /Unable to load fee tier/,
            );
          } else {
            await assert.rejects(
              failed.fetchQuery(spot.spotTokenStateQueryOptions(TOKEN)),
            );
            function RefusedSpot() {
              const refused = spot.useSpotTokenState(TOKEN);
              assert.equal(refused.state, null);
              assert.ok(refused.error);
              return null;
            }
            renderToStaticMarkup(
              React.createElement(
                QueryClientProvider,
                { client: failed },
                React.createElement(RefusedSpot),
              ),
            );
          }
        } finally {
          failed.clear();
        }
      }
      assert.deepEqual(
        app.failures,
        [],
        "every hosted producer handled its real crossing-layer request",
      );
    } finally {
      client.clear();
    }
  } finally {
    if (previousReact)
      Object.defineProperty(globalThis, "React", previousReact);
    else Reflect.deleteProperty(globalThis, "React");
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
}

// apps/perps/src/core/rpc.ts, buildPaxeerTransport: viem ranking owns a
// recurring timer without a disposal API. Expected: a released client can stop
// it. Assert the current retained timer and bound its lifetime to this worker.
function proofHooksProducerTransportConsumer() {
  const child = spawnSync(
    process.execPath,
    ["--import", "tsx", process.argv[1], "--hooks-proof-worker"],
    { cwd: process.cwd(), encoding: "utf8", timeout: 90_000 },
  );
  assert.equal(child.error, undefined, String(child.error));
  assert.equal(child.status, 0, `${child.stdout}\n${child.stderr}`);
  assert.match(
    child.stdout,
    /hooks producer transport consumer assertions passed/,
  );
}

if (process.argv.includes("--hooks-proof-worker")) {
  runHooksPath()
    .then(() => {
      assert.ok(
        process.getActiveResourcesInfo().includes("Timeout"),
        "buildPaxeerTransport currently retains its ranking timer",
      );
      console.log("hooks producer transport consumer assertions passed");
      process.exit(0);
    })
    .catch((error) => {
      console.error(error);
      process.exit(1);
    });
} else {
  test(proofHooksProducerTransportConsumer);
}

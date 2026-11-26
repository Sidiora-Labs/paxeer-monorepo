import assert from "node:assert/strict";
import test from "node:test";
import { spawn } from "node:child_process";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { createRequire } from "node:module";
import { pathToFileURL } from "node:url";
import {
  createPublicClient,
  custom,
  decodeFunctionData,
  encodeFunctionData,
  encodeFunctionResult,
  getAbiItem,
  getAddress,
  parseUnits,
  toFunctionSignature,
  type Address,
  type Hex,
} from "viem";
import ts from "typescript";
import {
  PerpsPrecompileABI,
  TimeInForce,
} from "@paxeer-network/paxeer-backend/core/network/abi/PerpsPrecompile";
import type {
  MarketConfig,
  MarginTier,
} from "@paxeer-network/paxeer-backend/core/perps/marketConfig";
import type { TradingCall } from "../packages/agent-kit/src/execution";
import type { WireTransaction } from "../packages/agent-kit/src/gateway";

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "http://localhost";
const kitModule =
  require("@paxeer-network/agent-kit") as typeof import("../packages/agent-kit/src/index");
const calls =
  require("@paxeer-network/paxeer-backend/core/perps/calls") as typeof import("@paxeer-network/paxeer-backend/core/perps/calls");
const { planCollateral } =
  require("@paxeer-network/paxeer-backend/core/perps/positionWrites") as typeof import("@paxeer-network/paxeer-backend/core/perps/positionWrites");
const { erc20Abi, COLLATERAL_TOKENS } =
  require("@paxeer-network/paxeer-backend/core/perps/config") as typeof import("@paxeer-network/paxeer-backend/core/perps/config");
const { buildPerpsAgentPreset, PERPS_AGENT_FUNCTIONS, PERPS_AGENT_APPROVAL } =
  require("@paxeer-network/paxeer-backend/core/wallet/embedded/agentPreset") as typeof import("@paxeer-network/paxeer-backend/core/wallet/embedded/agentPreset");
const root = path.resolve(__dirname, "..");
const kitRequire = createRequire(
  path.join(root, "packages/agent-kit/package.json"),
);
const { ed25519 } = kitRequire(
  "@noble/curves/ed25519.js",
) as typeof import("@noble/curves/ed25519.js");
const E18 = 10n ** 18n;
const USDL = COLLATERAL_TOKENS.usdl.address;
const PERPS = calls.buildDeposit(1n).address;
const WALLET: Address = getAddress(
  "0x00000000000000000000000000000000000000a1",
);
const OWNER: Address = "0x00000000000000000000000000000000000000a2";
const seed = Uint8Array.from({ length: 32 }, (_, i) => i + 1);
const actionId = `act_${"1".repeat(24)}`;
const txHash: Hex = `0x${"2".repeat(64)}`;
const clientOrderId: Hex = calls.ladderCloid(1n, 0);
const marketConfig: MarketConfig = {
  marketId: 1n,
  symbol: "ETH",
  name: "Ethereum",
  oracleSymbol: "ETH",
  szDecimals: 2,
  pxDecimals: 2,
  tickSize: E18 / 100n,
  lotSize: E18,
  maxLeverage: 20n,
  tierClass: 0,
  impactNotionalUsd: 10_000n * E18,
  oiCapUsd: 1_000_000n * E18,
  fundingCapPpmPerHour: 1000n,
  enabled: true,
  listedAt: 1n,
};
const tiers = [
  {
    lowerBoundUsd: 0n,
    maxLeverage: 20n,
    maintenanceBps: 500n,
    deductionUsd: 0n,
  },
];
const quote = {
  entryPrice: 3000n * E18,
  sizeUsd: 200n * E18,
  collateralUsd: 100n * E18,
  leverage: 2n * E18,
  tradingFee: 0n,
  tradingFeeUsd: 0n,
  priceImpact: 0n,
  liquidationPrice: 1500n * E18,
  estimatedFunding24h: 0n,
  maintenanceMarginBps: 500n,
};
const accountState = {
  collateral: 100_000_000n,
  equityUsd: 100n * E18,
  initialMarginUsd: 20n * E18,
  maintenanceMarginUsd: 10n * E18,
  withdrawable: 80_000_000n,
  feeTier: 0,
  volume14dUsd: 200n * E18,
  positionCount: 1n,
  openOrderCount: 1n,
};

interface CapturedRequest {
  method: string;
  route: string;
  body: Record<string, unknown>;
  headers: Headers;
  query: URLSearchParams;
}
interface Refusal {
  route: string;
  status: number;
  payload: unknown;
}

class GatewayStandIn {
  requests: CapturedRequest[] = [];
  allowance = 1_000_000_000n;
  refusal?: Refusal;
  invalidChallenge = false;
  nonces = new Set<string>();

  fetch: typeof fetch = async (input, init) => {
    const url = new URL(String(input));
    const method = init?.method ?? "GET";
    const bodyText = typeof init?.body === "string" ? init.body : "";
    const body = bodyText
      ? (JSON.parse(bodyText) as Record<string, unknown>)
      : {};
    const headers = new Headers(init?.headers);
    const route = url.pathname;
    this.requests.push({
      method,
      route,
      body,
      headers,
      query: url.searchParams,
    });
    if (route === "/v1/agent/auth/challenge") {
      return Response.json({
        did: body.did,
        nonce: "challenge-nonce",
        message: this.invalidChallenge
          ? "unrelated message"
          : `paxeer-agent-auth:${body.did}:challenge-nonce`,
      });
    }
    if (route === "/v1/agent/auth/verify") {
      assert.equal(body.did, kitModule.gatewayAgentDid(seed, "perps"));
      assert.equal(
        body.public_key,
        Buffer.from(ed25519.getPublicKey(seed)).toString("hex"),
      );
      assert.ok(
        ed25519.verify(
          Buffer.from(String(body.signature), "hex"),
          Buffer.from(`paxeer-agent-auth:${body.did}:${body.nonce}`),
          ed25519.getPublicKey(seed),
        ),
      );
      if (this.refusal?.route === route)
        return Response.json(this.refusal.payload, {
          status: this.refusal.status,
        });
      return Response.json({ token: "agent-read-session", expires_in: 600 });
    }
    const signed =
      route === "/v1/agent/send" ||
      route === "/v1/agent/provision" ||
      route === "/v1/agent/actions/allowance-and-call";
    if (signed) {
      const nonce = headers.get("x-agent-nonce")!;
      assert.match(nonce, /^[0-9a-f]{32}$/);
      assert.ok(!this.nonces.has(nonce));
      this.nonces.add(nonce);
      const digest = kitModule.agentRequestDigest({
        method: `${method} ${route}`,
        keyId: headers.get("x-agent-key")!,
        nonce: Buffer.from(nonce, "hex"),
        expiry: BigInt(headers.get("x-agent-expires")!),
        body: bodyText,
      });
      assert.ok(
        ed25519.verify(
          Buffer.from(headers.get("x-agent-signature")!, "hex"),
          digest,
          ed25519.getPublicKey(seed),
        ),
      );
      assert.equal(
        headers.get("x-agent-key"),
        kitModule.gatewayAgentDid(seed, "perps"),
      );
    } else {
      assert.equal(headers.get("authorization"), "Bearer agent-read-session");
      assert.equal(headers.get("x-agent-signature"), null);
    }
    if (this.refusal?.route === route)
      return Response.json(this.refusal.payload, {
        status: this.refusal.status,
      });
    switch (route) {
      case "/v1/agent/me":
        return Response.json({ wallet: { address: WALLET } });
      case "/v1/agent/allowance":
        assert.equal(url.searchParams.get("token"), USDL);
        assert.equal(url.searchParams.get("spender"), PERPS);
        return Response.json({ allowance: this.allowance.toString() });
      case "/v1/agent/simulate":
        return Response.json({
          ok: true,
          gas: "100000",
          error: null,
          return_data: "0x",
        });
      case "/v1/agent/send":
        return Response.json({ tx_hash: txHash });
      case "/v1/agent/provision":
        return Response.json({
          wallet: { address: WALLET },
          provision: { state: "active" },
        });
      case "/v1/agent/actions/allowance-and-call":
        return Response.json(
          {
            ok: null,
            action_id: actionId,
            status: "queued",
            terminal: false,
            error: null,
          },
          { status: 202 },
        );
      case `/v1/agent/actions/${actionId}`:
        return Response.json({
          ok: true,
          action_id: actionId,
          status: "confirmed",
          terminal: true,
          error: null,
        });
      case `/v1/agent/tx/${txHash}`:
        return Response.json({ transactionHash: txHash, status: "success" });
      default:
        throw new Error(`Unexpected gateway route ${route}`);
    }
  };
}

function environment() {
  const gateway = new GatewayStandIn();
  const rpcCalls: ReturnType<
    typeof decodeFunctionData<typeof PerpsPrecompileABI>
  >[] = [];
  let config = { ...marketConfig };
  let marginTiers: readonly MarginTier[] = tiers;
  let openQuote = { ...quote };
  let positionOwner = WALLET;
  let fresh = true;
  const rpc = createPublicClient({
    transport: custom(
      {
        async request({ method, params }) {
          assert.equal(method, "eth_call");
          const [tx] = params as readonly [{ to: Address; data: Hex }];
          assert.equal(tx.to.toLowerCase(), PERPS.toLowerCase());
          const decoded = decodeFunctionData({
            abi: PerpsPrecompileABI,
            data: tx.data,
          });
          rpcCalls.push(decoded);
          switch (decoded.functionName) {
            case "getMarketConfig":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getMarketConfig",
                result: config,
              });
            case "getMarginTiers":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getMarginTiers",
                result: marginTiers,
              });
            case "getMarkPriceDetails":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getMarkPriceDetails",
                result: [3000n * E18, 3000n * E18, 3000n * E18, fresh],
              });
            case "quoteOpenPosition":
              assert.deepEqual(decoded.args, [
                1n,
                USDL,
                openQuote.collateralUsd / 10n ** 12n,
                openQuote.leverage,
                true,
              ]);
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "quoteOpenPosition",
                result: openQuote,
              });
            case "getAccount":
              assert.deepEqual(decoded.args, [WALLET]);
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getAccount",
                result: accountState,
              });
            case "getUserPositionIds":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getUserPositionIds",
                result: [7n],
              });
            case "getPosition":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getPosition",
                result: [
                  positionOwner,
                  1n,
                  true,
                  200n * E18,
                  100n * E18,
                  USDL,
                  100_000_000n,
                  3000n * E18,
                  1n,
                  true,
                ],
              });
            case "getPositionDetails":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getPositionDetails",
                result: {
                  positionId: 7n,
                  user: WALLET,
                  marketId: 1n,
                  isLong: true,
                  sizeBase: E18,
                  sizeUsdAtMark: 200n * E18,
                  entryPrice: 3000n * E18,
                  leverage: 2n * E18,
                  unrealizedPnlUsd: 0n,
                  fundingOwedUsd: 0n,
                  liquidationPrice: 1500n * E18,
                  takeProfitPrice: 0n,
                  stopLossPrice: 0n,
                  openedAt: 1n,
                },
              });
            case "getPositionMargin":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getPositionMargin",
                result: {
                  token: USDL,
                  lockedAmount: 100_000_000n,
                  marketId: 1n,
                  isLong: true,
                },
              });
            case "getUserOrderIds":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getUserOrderIds",
                result: [12n],
              });
            case "getOrder":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "getOrder",
                result: [
                  WALLET,
                  1n,
                  true,
                  0,
                  0n,
                  2900n * E18,
                  200n * E18,
                  2n * E18,
                  USDL,
                  100_000_000n,
                  true,
                ],
              });
            case "quoteClosePosition":
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "quoteClosePosition",
                result: {
                  exitPrice: 3100n * E18,
                  unrealizedPnl: 0n,
                  tradingFee: 0n,
                  tradingFeeUsd: 0n,
                  fundingOwed: 0n,
                  netPnl: 0n,
                  estimatedPayout: 100_000_000n,
                },
              });
            case "quotePartialClose":
              assert.deepEqual(decoded.args, [7n, 50n * E18]);
              return encodeFunctionResult({
                abi: PerpsPrecompileABI,
                functionName: "quotePartialClose",
                result: [3100n * E18, 0n, 0n, 25_000_000n],
              });
            default:
              throw new Error(`Unexpected chain read ${decoded.functionName}`);
          }
        },
      },
      { retryCount: 0 },
    ),
  });
  const queries: { query: string; variables?: Record<string, unknown> }[] = [];
  const dataFetch: typeof fetch = async (input, init) => {
    assert.equal(String(input), "http://localhost/graphql");
    const body = JSON.parse(String(init?.body)) as {
      query: string;
      variables?: Record<string, unknown>;
    };
    queries.push(body);
    if (body.query.includes("query GetMarkets"))
      return Response.json({
        data: {
          markets: [
            {
              marketId: 1,
              symbol: "ETH",
              name: "Ethereum",
              oracleSymbol: "ETH",
              szDecimals: 2,
              pxDecimals: 2,
              tickSize: (E18 / 100n).toString(),
              lotSize: E18.toString(),
              tierClass: 0,
              maxLeverage: "20",
              enabled: true,
              createdAt: null,
              latestPrice: { price: "3000", onchainTimestamp: 1 },
              fundingRate: { ratePerSecond: "0.000001", rate24h: "0.0864" },
            },
          ],
        },
      });
    if (body.query.includes("query GetTrades"))
      return Response.json({ data: { trades: [{ id: "trade-1", txHash }] } });
    if (body.query.includes("query GetClosedPositions"))
      return Response.json({
        data: { positions: [{ positionId: "6", status: "closed" }] },
      });
    throw new Error("Unexpected GraphQL query");
  };
  const client = new kitModule.AgentGatewayClient({
    url: "http://localhost",
    privateKey: seed,
    label: "perps",
    fetch: gateway.fetch,
  });
  const data = new kitModule.PerpsDataClient({
    nodeUrl: "http://localhost",
    dataUrl: "http://localhost",
    publicClient: rpc,
    fetch: dataFetch,
  });
  return {
    gateway,
    client,
    data,
    kit: kitModule.createAgentTools(client, data),
    rpcCalls,
    queries,
    setConfig(value: Partial<MarketConfig>) {
      config = { ...config, ...value };
    },
    setTierLeverage(value: bigint) {
      marginTiers = [{ ...tiers[0], maxLeverage: value }];
    },
    setTiers(value: readonly MarginTier[]) {
      marginTiers = value;
    },
    setQuote(value: Partial<typeof quote>) {
      openQuote = { ...openQuote, ...value };
    },
    setOwner(value: Address) {
      positionOwner = value;
    },
    setFresh(value: boolean) {
      fresh = value;
    },
  };
}

function assertSuccess(
  result: Awaited<
    ReturnType<ReturnType<typeof environment>["kit"]["callTool"]>
  >,
) {
  assert.equal(result.ok, true, result.ok ? undefined : JSON.stringify(result));
  if (!result.ok) throw new Error(result.error.code);
  return result.data;
}

const restingInput = {
  marketId: "1",
  isLong: true,
  leverage: "2",
  sizeUsd: "200",
  limitPrice: "2900",
  clientOrderId,
};
const restingBuilder = {
  marketId: 1n,
  isLong: true,
  leverage: 2n * E18,
  sizeUsd: 200n * E18,
  limitPrice: 2900n * E18,
  collateralAmount: 100_000_000n,
  timeInForce: TimeInForce.GTC,
  reduceOnly: false,
  cloid: clientOrderId,
};
const scenarios: {
  name: string;
  input: Record<string, unknown>;
  expected: () => TradingCall;
}[] = [
  {
    name: "deposit",
    input: { amount: "100", idempotencyKey: "deposit-1" },
    expected: () => calls.buildDeposit(100_000_000n),
  },
  {
    name: "withdraw",
    input: { amount: "25.5" },
    expected: () => calls.buildWithdraw(25_500_000n),
  },
  {
    name: "open_market",
    input: {
      marketId: "1",
      isLong: true,
      collateralAmount: "100",
      leverage: "2",
    },
    expected: () =>
      calls.buildOpenPosition({
        marketId: 1n,
        isLong: true,
        collateralAmount: 100_000_000n,
        leverage: 2n * E18,
        quote,
      }),
  },
  {
    name: "place_limit",
    input: restingInput,
    expected: () => calls.buildPlaceLimitOrder(restingBuilder),
  },
  {
    name: "place_stop_limit",
    input: { ...restingInput, triggerPrice: "2950" },
    expected: () =>
      calls.buildPlaceStopLimitOrder({
        ...restingBuilder,
        triggerPrice: 2950n * E18,
      }),
  },
  {
    name: "cancel_order",
    input: { orderId: "12" },
    expected: () => calls.buildCancelOrder(12n),
  },
  {
    name: "set_tpsl",
    input: { positionId: "7", takeProfit: "3300", stopLoss: "2700" },
    expected: () =>
      calls.buildSetPositionTpSl({
        positionId: 7n,
        isLong: true,
        takeProfitPrice: 3300n * E18,
        stopLossPrice: 2700n * E18,
      }),
  },
  {
    name: "add_collateral",
    input: { positionId: "7", amount: "25", idempotencyKey: "collateral-1" },
    expected: () => calls.buildAddCollateral(7n, 25_000_000n),
  },
  {
    name: "remove_collateral",
    input: { positionId: "7", amount: "10" },
    expected: () => calls.buildRemoveCollateral(7n, 10_000_000n),
  },
  {
    name: "close_position",
    input: { positionId: "7" },
    expected: () =>
      calls.buildClosePosition({
        positionId: 7n,
        isLong: true,
        exitPrice: 3100n * E18,
      }),
  },
  {
    name: "partial_close",
    input: { positionId: "7", fractionBps: 2500 },
    expected: () =>
      calls.buildPartialClose({
        positionId: 7n,
        isLong: true,
        closeSizeUsd: 50n * E18,
        exitPrice: 3100n * E18,
      }),
  },
];

async function sdkReference() {
  const sdkRoot = path.resolve(root, "../../sdk/wallet/src");
  const errors = ts.transpileModule(
    await readFile(path.join(sdkRoot, "errors.ts"), "utf8"),
    {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ES2022,
      },
    },
  ).outputText;
  const errorsUrl = `data:text/javascript;base64,${Buffer.from(errors).toString("base64")}`;
  const source = (await readFile(path.join(sdkRoot, "agent.ts"), "utf8"))
    .replace("'./errors.js'", JSON.stringify(errorsUrl))
    .replace(
      "'@noble/curves/ed25519.js'",
      JSON.stringify(
        pathToFileURL(kitRequire.resolve("@noble/curves/ed25519.js")).href,
      ),
    )
    .replace(
      "'@noble/hashes/sha2.js'",
      JSON.stringify(
        pathToFileURL(kitRequire.resolve("@noble/hashes/sha2.js")).href,
      ),
    );
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
    },
  }).outputText;
  return (await import(
    `data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`
  )) as typeof import("../../../sdk/wallet/src/agent");
}

async function agentRequestSigningMatchesWalletSdk() {
  const sdk = await sdkReference();
  const nonce = Uint8Array.from({ length: 16 }, (_, i) => i + 1);
  for (const body of ["", '{"to":"0x01","value":"1"}', '{"label":"交易"}']) {
    const digestInput = {
      method: "POST /v1/agent/send",
      keyId: kitModule.gatewayAgentDid(seed, "perps"),
      nonce,
      expiry: 1_900_000_120n,
      body,
    };
    assert.deepEqual(
      kitModule.agentRequestDigest(digestInput),
      sdk.agentRequestDigest(digestInput),
    );
    const input = {
      privateKey: seed,
      keyId: digestInput.keyId,
      nonce,
      expiresAt: digestInput.expiry,
      body,
      method: "post",
      url: "http://localhost/v1/agent/send?trace=1#fragment",
    };
    assert.deepEqual(
      kitModule.signAgentRequest(input),
      sdk.signAgentRequest(input),
    );
  }
  assert.throws(() =>
    kitModule.agentRequestDigest({
      method: "GET /",
      keyId: "agent",
      nonce: new Uint8Array(15),
      expiry: 1n,
      body: "",
    }),
  );
  assert.throws(() =>
    kitModule.agentRequestDigest({
      method: "GET /",
      keyId: "agent",
      nonce,
      expiry: 1n << 64n,
      body: "",
    }),
  );
}
test(agentRequestSigningMatchesWalletSdk);

async function everyWriteUsesBackendCallAndGatewaySimulation() {
  assert.deepEqual(
    environment()
      .kit.tools.filter((tool) => !tool.readOnly)
      .map((tool) => tool.name)
      .sort(),
    scenarios.map((scenario) => scenario.name).sort(),
  );
  for (const scenario of scenarios) {
    const env = environment();
    const start = BigInt(Math.floor(Date.now() / 1000));
    assertSuccess(await env.kit.callTool(scenario.name, scenario.input));
    const end = BigInt(Math.floor(Date.now() / 1000));
    const requests = env.gateway.requests.filter(
      (request) =>
        request.route === "/v1/agent/simulate" ||
        request.route === "/v1/agent/send",
    );
    assert.deepEqual(
      requests.map((request) => [request.method, request.route]),
      [
        ["POST", "/v1/agent/simulate"],
        ["POST", "/v1/agent/send"],
      ],
      scenario.name,
    );
    const expected = scenario.expected();
    const sent = requests[1].body.tx as WireTransaction;
    assert.deepEqual(requests[0].body, requests[1].body, scenario.name);
    assert.equal(sent.to, expected.address);
    assert.equal(sent.value, "0");
    const decoded = decodeFunctionData({ abi: expected.abi, data: sent.data });
    assert.equal(decoded.functionName, expected.functionName);
    assert.ok(
      PERPS_AGENT_FUNCTIONS.some(
        (call) =>
          call.contract.toLowerCase() === sent.to.toLowerCase() &&
          call.selector === sent.data.slice(0, 10) &&
          call.name === decoded.functionName,
      ),
      `${scenario.name} sends a function in the shared agent inventory`,
    );
    // A deadline is chosen by the builder during this bounded invocation.
    if (expected.functionName === "closePositionWithParams") {
      const actualArgs = decoded.args as readonly [
        bigint,
        bigint,
        bigint,
        bigint,
      ];
      assert.ok(actualArgs[3] >= start + 60n && actualArgs[3] <= end + 60n);
      const callAtDeadline = {
        ...expected,
        args: [
          expected.args[0],
          expected.args[1],
          expected.args[2],
          actualArgs[3],
        ] as const,
      };
      assert.equal(sent.data, encodeFunctionData(callAtDeadline));
      assert.deepEqual(actualArgs, callAtDeadline.args);
    } else if (
      expected.functionName === "placeOrder" &&
      expected.args[0].deadline !== 0n
    ) {
      const actualArgs = decoded.args as readonly [(typeof expected.args)[0]];
      assert.ok(
        actualArgs[0].deadline >= start + 60n &&
          actualArgs[0].deadline <= end + 60n,
      );
      const callAtDeadline = {
        ...expected,
        args: [
          { ...expected.args[0], deadline: actualArgs[0].deadline },
        ] as const,
      };
      assert.equal(sent.data, encodeFunctionData(callAtDeadline));
      assert.deepEqual(actualArgs, callAtDeadline.args);
    } else {
      assert.equal(sent.data, encodeFunctionData(expected));
      assert.deepEqual(decoded.args, expected.args);
    }
    assert.equal(
      sent.data.slice(0, 10),
      encodeFunctionData(expected).slice(0, 10),
    );
    assert.equal(
      env.gateway.requests.filter(
        (request) => request.route === "/v1/agent/actions/allowance-and-call",
      ).length,
      0,
    );
  }
}
test(everyWriteUsesBackendCallAndGatewaySimulation);

async function collateralPlansUseExactDurableApprovalRequests() {
  for (const scenario of scenarios.filter(
    (item) => item.name === "deposit" || item.name === "add_collateral",
  )) {
    const env = environment();
    env.gateway.allowance = 0n;
    const result = assertSuccess(
      await env.kit.callTool(scenario.name, scenario.input),
    );
    assert.deepEqual(result, {
      ok: null,
      action_id: actionId,
      status: "queued",
      terminal: false,
      error: null,
    });
    const call = scenario.expected();
    const amount = call.args[1] as bigint;
    const backendPlan =
      scenario.name === "deposit"
        ? await calls.buildDepositPlan(amount, async () => 0n, new Set())
        : await planCollateral(
            { action: "add", positionId: 7n, amount },
            { allowance: async () => 0n, native: new Set() },
          );
    const approval = backendPlan[0];
    assert.equal(approval.functionName, "approve");
    if (approval.functionName !== "approve")
      throw new Error("Expected backend approval");
    const approve = { ...approval, abi: erc20Abi };
    assert.equal(
      approve.address.toLowerCase(),
      PERPS_AGENT_APPROVAL.contract.toLowerCase(),
    );
    assert.equal(
      approve.args[0].toLowerCase(),
      PERPS_AGENT_APPROVAL.spender.toLowerCase(),
    );
    assert.equal(
      encodeFunctionData(approve).slice(0, 10),
      PERPS_AGENT_APPROVAL.selector,
    );
    assert.ok(
      PERPS_AGENT_FUNCTIONS.some(
        (entry) =>
          entry.contract.toLowerCase() === call.address.toLowerCase() &&
          entry.selector === encodeFunctionData(call).slice(0, 10) &&
          entry.name === call.functionName,
      ),
    );
    const simulation = env.gateway.requests.find(
      (request) => request.route === "/v1/agent/simulate",
    )!;
    assert.deepEqual(simulation.body, {
      tx: { to: USDL, data: encodeFunctionData(approve), value: "0" },
    });
    const submission = env.gateway.requests.find(
      (request) => request.route === "/v1/agent/actions/allowance-and-call",
    )!;
    assert.equal(submission.method, "POST");
    assert.deepEqual(submission.body, {
      token: approve.address,
      amount: amount.toString(),
      spender: approve.args[0],
      contract: call.address,
      method: toFunctionSignature(
        getAbiItem({ abi: call.abi, name: call.functionName }),
      ),
      args: call.args.map((arg) => arg.toString()),
      idempotency_key: scenario.input.idempotencyKey,
    });
    assert.ok(
      env.gateway.requests.indexOf(simulation) <
        env.gateway.requests.indexOf(submission),
    );
    assert.equal(
      env.gateway.requests.filter(
        (request) => request.route === "/v1/agent/send",
      ).length,
      0,
    );
    assertSuccess(await env.kit.callTool("action_status", { actionId }));
  }
}
test(collateralPlansUseExactDurableApprovalRequests);

async function gatewayRefusalsKeepTheirCodesForEveryWrite() {
  for (const scenario of scenarios) {
    for (const route of ["/v1/agent/simulate", "/v1/agent/send"]) {
      const env = environment();
      env.gateway.refusal = {
        route,
        status: 403,
        payload: {
          error: "TX_VALUE_CAP",
          message: "per-transaction cap exceeded",
        },
      };
      const result = await env.kit.callTool(scenario.name, scenario.input);
      assert.equal(result.ok, false);
      if (result.ok) throw new Error("Expected refusal");
      assert.equal(result.error.code, "TX_VALUE_CAP", scenario.name);
      if (route === "/v1/agent/simulate")
        assert.equal(
          env.gateway.requests.filter(
            (request) => request.route === "/v1/agent/send",
          ).length,
          0,
        );
    }
  }
  for (const route of [
    "/v1/agent/actions/allowance-and-call",
    `/v1/agent/actions/${actionId}`,
  ]) {
    const env = environment();
    env.gateway.allowance = 0n;
    env.gateway.refusal = {
      route,
      status: 200,
      payload: {
        ok: false,
        terminal: true,
        error: { code: "APPROVE_CAP", message: "approval cap exceeded" },
        action_id: actionId,
      },
    };
    const result = await env.kit.callTool(
      route.endsWith("allowance-and-call") ? "deposit" : "action_status",
      route.endsWith("allowance-and-call")
        ? { amount: "100", idempotencyKey: "deposit-1" }
        : { actionId },
    );
    assert.equal(result.ok, false);
    if (result.ok) throw new Error("Expected refusal");
    assert.equal(result.error.code, "APPROVE_CAP");
    assert.deepEqual(result.error.details, {
      status: 200,
      response: env.gateway.refusal.payload,
    });
  }
}
test(gatewayRefusalsKeepTheirCodesForEveryWrite);

async function marketBoundsRefuseBeforeSigningOrSending() {
  const cases = [
    {
      name: "place_limit",
      input: { ...restingInput, leverage: "21" },
      code: "LEVERAGE_BOUND",
      message: /leverage.*between/,
    },
    {
      name: "place_limit",
      input: { ...restingInput, leverage: "0.5" },
      code: "LEVERAGE_BOUND",
      message: /leverage.*between/,
    },
    {
      name: "place_limit",
      input: { ...restingInput, limitPrice: "2900.001" },
      code: "PRICE_TICK_BOUND",
      message: /tickSize/,
    },
    {
      name: "place_stop_limit",
      input: { ...restingInput, triggerPrice: "2950.001" },
      code: "PRICE_TICK_BOUND",
      message: /tickSize/,
    },
    {
      name: "place_limit",
      input: { ...restingInput, sizeUsd: "200.5" },
      code: "SIZE_LOT_BOUND",
      message: /lotSize/,
    },
    {
      name: "place_limit",
      input: { ...restingInput, sizeUsd: "1000001" },
      code: "SIZE_CAP_BOUND",
      message: /oiCapUsd/,
    },
    {
      name: "place_limit",
      input: { ...restingInput, takeProfit: "3300.001" },
      code: "PRICE_TICK_BOUND",
      message: /tickSize/,
    },
    {
      name: "open_market",
      input: {
        marketId: "1",
        isLong: true,
        collateralAmount: "100",
        leverage: "21",
      },
      code: "LEVERAGE_BOUND",
      message: /leverage.*between/,
    },
    {
      name: "deposit",
      input: { amount: "1.0000001", idempotencyKey: "deposit-1" },
      code: "PRECISION_BOUND",
      message: /6 decimal/,
    },
    {
      name: "place_limit",
      input: { ...restingInput, limitPrice: "0" },
      code: "AMOUNT_BOUND",
      message: /positive/,
    },
  ];
  for (const item of cases) {
    const env = environment();
    const result = await env.kit.callTool(item.name, item.input);
    assert.equal(result.ok, false, item.code);
    if (result.ok) throw new Error("Expected bound refusal");
    assert.equal(result.error.code, item.code);
    assert.match(result.error.message, item.message);
    assert.equal(
      env.gateway.requests.length,
      0,
      "Invalid input must not authenticate, simulate or sign a request",
    );
  }
  for (const config of [{ enabled: false }, { lotSize: 0n }]) {
    const env = environment();
    env.setConfig(config);
    const result = await env.kit.callTool("place_limit", restingInput);
    assert.equal(result.ok, false);
    assert.equal(env.gateway.requests.length, 0);
  }
  const env = environment();
  env.setTierLeverage(1n);
  const result = await env.kit.callTool("place_limit", restingInput);
  assert.equal(result.ok, false);
  if (result.ok) throw new Error("Expected tier refusal");
  assert.equal(result.error.code, "LEVERAGE_BOUND");
  assert.equal(env.gateway.requests.length, 0);
}
test(marketBoundsRefuseBeforeSigningOrSending);

async function leverageBoundOfTheChainIsAPlainInteger() {
  for (const name of ["place_limit", "place_stop_limit", "open_market"]) {
    for (const multiple of [1n, 2n, 1000n]) {
      const env = environment();
      env.setConfig({ maxLeverage: 1000n });
      env.setTierLeverage(1000n);
      env.setQuote({
        leverage: multiple * E18,
        sizeUsd: 100n * multiple * E18,
      });
      const input =
        name === "open_market"
          ? {
              marketId: "1",
              isLong: true,
              collateralAmount: "100",
              leverage: multiple.toString(),
            }
          : {
              ...restingInput,
              leverage: multiple.toString(),
              ...(name === "place_stop_limit" ? { triggerPrice: "2950" } : {}),
            };
      assertSuccess(await env.kit.callTool(name, input));
      const sent = env.gateway.requests.find(
        (request) => request.route === "/v1/agent/send",
      )!.body.tx as WireTransaction;
      const decoded = decodeFunctionData({
        abi: PerpsPrecompileABI,
        data: sent.data,
      });
      assert.equal(decoded.functionName, "placeOrder");
      if (decoded.functionName !== "placeOrder")
        throw new Error("Expected order");
      assert.equal(decoded.args[0].leverage, multiple * E18);
      assert.equal(
        decoded.args[0].collateralAmount,
        name === "open_market" ? 100_000_000n : (200n * 10n ** 6n) / multiple,
      );
    }
    for (const leverage of ["1001", "1000.000000000000000001"]) {
      const env = environment();
      env.setConfig({ maxLeverage: 1000n });
      env.setTierLeverage(1000n);
      const input =
        name === "open_market"
          ? { marketId: "1", isLong: true, collateralAmount: "100", leverage }
          : {
              ...restingInput,
              leverage,
              ...(name === "place_stop_limit" ? { triggerPrice: "2950" } : {}),
            };
      const result = await env.kit.callTool(name, input);
      assert.equal(result.ok, false);
      if (result.ok) throw new Error("Expected leverage refusal");
      assert.equal(result.error.code, "LEVERAGE_BOUND");
      assert.equal(result.error.message, "leverage must be between 1 and 1000");
      assert.deepEqual(result.error.details, { minimum: "1", maximum: "1000" });
      assert.equal(env.gateway.requests.length, 0);
    }
  }

  for (const [sizeUsd, leverage, accepted] of [
    ["199", "1000", true],
    ["200", "2", true],
    ["201", "2", true],
    ["200", "2.000000000000000001", false],
    ["201", "3", false],
  ] as const) {
    const env = environment();
    env.setConfig({ maxLeverage: 1000n });
    env.setTiers([
      { ...tiers[0], maxLeverage: 1000n },
      { ...tiers[0], lowerBoundUsd: 200n * E18, maxLeverage: 2n },
    ]);
    const result = await env.kit.callTool("place_limit", {
      ...restingInput,
      sizeUsd,
      leverage,
    });
    if (accepted) assertSuccess(result);
    else {
      assert.equal(result.ok, false);
      if (result.ok) throw new Error("Expected tier refusal");
      assert.equal(result.error.code, "LEVERAGE_BOUND");
      assert.equal(result.error.message, "leverage must be between 1 and 2");
      assert.equal(env.gateway.requests.length, 0);
    }
  }

  for (const lowerBoundUsd of [200n * E18, 201n * E18]) {
    const env = environment();
    env.setConfig({ maxLeverage: 1000n });
    env.setTiers([
      { ...tiers[0], maxLeverage: 1000n },
      { ...tiers[0], lowerBoundUsd, maxLeverage: 1n },
    ]);
    const result = await env.kit.callTool("open_market", {
      marketId: "1",
      isLong: true,
      collateralAmount: "100",
      leverage: "2",
    });
    assert.ok(
      env.rpcCalls.some((call) => call.functionName === "quoteOpenPosition"),
    );
    if (lowerBoundUsd === 201n * E18) assertSuccess(result);
    else {
      assert.equal(result.ok, false);
      if (result.ok) throw new Error("Expected quoted-size tier refusal");
      assert.equal(result.error.code, "LEVERAGE_BOUND");
      assert.equal(result.error.message, "leverage must be between 1 and 1");
      assert.equal(env.gateway.requests.length, 0);
    }
  }

  for (const [marketCap, tierCap] of [
    [0n, 1000n],
    [1000n, 0n],
    [1n, 1000n],
    [1000n, 1n],
  ]) {
    const env = environment();
    env.setConfig({ maxLeverage: marketCap });
    env.setTierLeverage(tierCap);
    const result = await env.kit.callTool("place_limit", restingInput);
    assert.equal(result.ok, false);
    if (result.ok) throw new Error("Expected bound refusal");
    assert.equal(
      result.error.code,
      marketCap === 0n || tierCap === 0n
        ? "INVALID_MARKET_CONFIG"
        : "LEVERAGE_BOUND",
    );
    assert.equal(env.gateway.requests.length, 0);
    if (marketCap !== 0n && tierCap !== 0n) {
      assertSuccess(
        await env.kit.callTool("place_limit", {
          ...restingInput,
          leverage: "1",
        }),
      );
    }
  }
}
test(leverageBoundOfTheChainIsAPlainInteger);

async function positionActionsCheckOwnershipAndProtectionBounds() {
  for (const scenario of scenarios.filter((item) =>
    [
      "set_tpsl",
      "add_collateral",
      "remove_collateral",
      "close_position",
      "partial_close",
    ].includes(item.name),
  )) {
    const env = environment();
    env.setOwner(OWNER);
    const result = await env.kit.callTool(scenario.name, scenario.input);
    assert.equal(result.ok, false);
    if (result.ok) throw new Error("Expected ownership refusal");
    assert.equal(result.error.code, "POSITION_NOT_OWNED");
    assert.equal(
      env.gateway.requests.filter((request) =>
        request.headers.has("x-agent-signature"),
      ).length,
      0,
    );
  }
  for (const [name, input, code] of [
    [
      "partial_close",
      { positionId: "7", fractionBps: 10000 },
      "INVALID_ARGUMENTS",
    ],
    [
      "set_tpsl",
      { positionId: "7", takeProfit: "2900", stopLoss: "2700" },
      "TPSL_PRICE_BOUND",
    ],
    [
      "set_tpsl",
      { positionId: "7", takeProfit: "3300.001", stopLoss: "2700" },
      "PRICE_TICK_BOUND",
    ],
  ] as const) {
    const env = environment();
    const result = await env.kit.callTool(name, input);
    assert.equal(result.ok, false);
    if (result.ok) throw new Error("Expected action refusal");
    assert.equal(result.error.code, code);
    assert.equal(
      env.gateway.requests.filter((request) =>
        request.headers.has("x-agent-signature"),
      ).length,
      0,
    );
  }
  const env = environment();
  env.setFresh(false);
  const result = await env.kit.callTool("set_tpsl", {
    positionId: "7",
    takeProfit: "3300",
    stopLoss: "2700",
  });
  assert.equal(result.ok, false);
  if (result.ok) throw new Error("Expected fresh-price refusal");
  assert.equal(result.error.code, "MARK_PRICE_STALE");
}
test(positionActionsCheckOwnershipAndProtectionBounds);

async function readToolsUseTradingChainAndIndexerReads() {
  const env = environment();
  const markets = assertSuccess(
    await env.kit.callTool("markets", {}),
  ) as Awaited<ReturnType<typeof env.data.markets>>;
  assert.equal(markets[0].config.maxLeverage, 20n);
  assert.deepEqual(markets[0].config, marketConfig);
  assert.deepEqual(markets[0].tiers, tiers);
  assert.equal(markets[0].mark.markPrice, 3000n * E18);
  assert.equal(markets[0].fundingRate?.rate24h, "0.0864");
  const account = assertSuccess(
    await env.kit.callTool("account", {}),
  ) as Awaited<ReturnType<typeof env.data.account>>;
  assert.equal(account.collateral, 100_000_000n);
  assert.equal(account.initialMarginUsd, 20n * E18);
  assert.equal(account.withdrawable, 80_000_000n);
  assert.deepEqual(account.state, accountState);
  assert.equal(account.equityUsd, 100n * E18);
  assert.equal(account.maintenanceMarginUsd, 10n * E18);
  assert.equal(account.risk.headroomUsd, 90n * E18);
  assert.equal(account.risk.maintenanceUsageBps, 1000n);
  assert.equal(account.risk.display.equity, "100 USD");
  assert.equal(account.risk.display.maintenanceUsage, "10%");
  const positions = assertSuccess(
    await env.kit.callTool("positions", {}),
  ) as Awaited<ReturnType<typeof env.data.positions>>;
  assert.equal(positions[0].positionId, 7n);
  assert.equal(positions[0].margin.lockedAmount, 100_000_000n);
  assert.equal(positions[0].sizeUsd, 200n * E18);
  assert.equal(positions[0].collateralUsd, 100n * E18);
  assert.equal(positions[0].collateralAmount, 100_000_000n);
  assert.equal(positions[0].entryPrice, 3000n * E18);
  assert.equal(positions[0].details.leverage, 2n * E18);
  const orders = assertSuccess(
    await env.kit.callTool("open_orders", {}),
  ) as Awaited<ReturnType<typeof env.data.openOrders>>;
  assert.equal(orders[0].orderId, 12n);
  assert.equal(orders[0].sizeUsd, 200n * E18);
  assert.equal(orders[0].limitPrice, 2900n * E18);
  assert.equal(orders[0].leverage, 2n * E18);
  assert.equal(orders[0].collateralAmount, 100_000_000n);
  const history = assertSuccess(
    await env.kit.callTool("history", { limit: 10, offset: 20 }),
  ) as Awaited<ReturnType<typeof env.data.history>>;
  assert.equal(history.trades[0].id, "trade-1");
  assert.equal(history.closedPositions[0].positionId, "6");
  for (const query of env.queries.slice(1))
    assert.deepEqual(query.variables, {
      userAddress: WALLET.toLowerCase(),
      limit: 10,
      offset: 20,
    });
  assert.deepEqual(
    new Set(env.rpcCalls.map((call) => call.functionName)),
    new Set([
      "getMarketConfig",
      "getMarginTiers",
      "getMarkPriceDetails",
      "getAccount",
      "getUserPositionIds",
      "getPosition",
      "getPositionDetails",
      "getPositionMargin",
      "getUserOrderIds",
      "getOrder",
    ]),
  );
  assertSuccess(
    await env.kit.callTool("transaction_receipt", { hash: txHash }),
  );
  assert.equal(
    env.gateway.requests.filter((request) =>
      request.headers.has("x-agent-signature"),
    ).length,
    0,
  );
}
test(readToolsUseTradingChainAndIndexerReads);

async function tradingPresetContainsExactlyTheWriteCapabilities() {
  const preset = await kitModule.buildTradingPreset({
    ownerAddress: OWNER,
    maxTxValueWei: 10n,
    maxDailyValueWei: 100n,
    maxApproveWei: 100_000_000n,
    budgetWei: 500_000_000n,
    expiresInSeconds: 3600,
    rateLimitPerMin: 30,
  });
  assert.deepEqual(
    PERPS_AGENT_FUNCTIONS.map((call) => call.name).sort(),
    [
      "placeOrder",
      "closePositionWithParams",
      "setPositionTpSl",
      "addCollateral",
      "removeCollateral",
      "topUpPosition",
      "cancelOrder",
      "cancelByCloid",
      "deposit",
      "withdraw",
    ].sort(),
  );
  const capabilities = [...PERPS_AGENT_FUNCTIONS, PERPS_AGENT_APPROVAL];
  assert.deepEqual(
    new Set(
      preset.rules
        .filter((rule) => rule.subject === "contract")
        .map((rule) => rule.value),
    ),
    new Set(capabilities.map((call) => call.contract.toLowerCase())),
  );
  assert.deepEqual(
    new Set(
      preset.rules
        .filter((rule) => rule.subject === "selector")
        .map((rule) => rule.value),
    ),
    new Set(capabilities.map((call) => call.selector)),
  );
  assert.deepEqual(
    preset.rules
      .filter((rule) => rule.subject === "withdrawal")
      .map((rule) => rule.value),
    [OWNER.toLowerCase()],
  );
  assert.deepEqual(preset.approvalSpenders, [PERPS]);
  assert.deepEqual(preset.policy, {
    mode: "trade_only",
    max_tx_value_wei: "10",
    max_daily_value_wei: "100",
    max_approve_wei: "100000000",
    rate_limit_per_min: 30,
    allow_native_transfer: false,
    withdrawal_allowlist_only: true,
    daily_reset_utc_hour: 0,
  });
  assert.deepEqual(preset.budget, {
    target_contract: PERPS,
    token: USDL,
    cap_wei: "500000000",
    expires_in_seconds: 3600,
  });
  await assert.rejects(
    kitModule.buildTradingPreset({
      ownerAddress: OWNER,
      maxTxValueWei: 11n,
      maxDailyValueWei: 10n,
      maxApproveWei: 100n,
      budgetWei: 100n,
      expiresInSeconds: 3600,
      rateLimitPerMin: 30,
    }),
  );
}
test(tradingPresetContainsExactlyTheWriteCapabilities);

async function kitPresetAndPagePresetGiveTheSameRules() {
  for (const owner of [OWNER, getAddress(OWNER), WALLET]) {
    for (const [txCap, dailyCap] of [
      ["1", "10"],
      ["2.5", "25.75"],
    ]) {
      const page = buildPerpsAgentPreset(owner, txCap, dailyCap, 3600);
      const kit = await kitModule.buildTradingPreset({
        ownerAddress: owner,
        maxTxValueWei: parseUnits(txCap, 18),
        maxDailyValueWei: parseUnits(dailyCap, 18),
        maxApproveWei: parseUnits(txCap, COLLATERAL_TOKENS.usdl.decimals),
        budgetWei: parseUnits(dailyCap, COLLATERAL_TOKENS.usdl.decimals),
        expiresInSeconds: 3600,
        rateLimitPerMin: 60,
      });
      const ruleKeys = (rules: typeof page.rules) =>
        rules.map(
          (rule) =>
            `${rule.effect}:${rule.subject}:${rule.value.toLowerCase()}`,
        );
      assert.deepEqual(
        new Set(ruleKeys(kit.rules)),
        new Set(ruleKeys(page.rules)),
      );
      assert.equal(kit.rules.length, new Set(ruleKeys(kit.rules)).size);
      assert.equal(kit.rules.length, page.rules.length);
      for (const preset of [kit, page]) {
        assert.deepEqual(
          preset.rules
            .filter((rule) => rule.subject === "address")
            .map((rule) => ({
              effect: rule.effect,
              value: rule.value.toLowerCase(),
            })),
          [
            {
              effect: "allow",
              value: PERPS_AGENT_APPROVAL.spender.toLowerCase(),
            },
          ],
        );
        assert.deepEqual(
          preset.rules
            .filter((rule) => rule.subject === "withdrawal")
            .map((rule) => ({
              effect: rule.effect,
              value: rule.value.toLowerCase(),
            })),
          [{ effect: "allow", value: owner.toLowerCase() }],
        );
        assert.deepEqual(
          preset.rules
            .filter((rule) => rule.subject === "token")
            .map((rule) => ({
              effect: rule.effect,
              value: rule.value.toLowerCase(),
              max_value_wei: rule.max_value_wei,
            })),
          [
            {
              effect: "allow",
              value: PERPS_AGENT_APPROVAL.contract.toLowerCase(),
              max_value_wei: parseUnits(
                txCap,
                COLLATERAL_TOKENS.usdl.decimals,
              ).toString(),
            },
          ],
        );
        assert.ok(preset.rules.every((rule) => rule.effect === "allow"));
      }
      assert.deepEqual(kit.approvalSpenders, [PERPS_AGENT_APPROVAL.spender]);
      assert.deepEqual(kit.policy, page.policy);
      assert.deepEqual(kit.budget, page.budgets[0]);
    }
  }
}
test(kitPresetAndPagePresetGiveTheSameRules);

async function authenticationRejectsUnrelatedChallengesAndPropagatesRefusals() {
  const env = environment();
  env.gateway.invalidChallenge = true;
  await assert.rejects(env.client.authenticate(), {
    code: "INVALID_CHALLENGE",
  });
  assert.equal(env.gateway.requests.length, 1);
  const denied = environment();
  denied.gateway.refusal = {
    route: "/v1/agent/auth/verify",
    status: 503,
    payload: { error: "agent_lane_disabled" },
  };
  const result = await denied.kit.callTool("account", {});
  assert.equal(result.ok, false);
  if (result.ok) throw new Error("Expected auth refusal");
  assert.equal(result.error.code, "agent_lane_disabled");
  const live = environment();
  await Promise.all([live.client.authenticate(), live.client.authenticate()]);
  assert.equal(
    live.gateway.requests.filter(
      (request) => request.route === "/v1/agent/auth/verify",
    ).length,
    1,
  );
  await live.client.provision();
  assert.equal(live.gateway.requests.at(-1)?.route, "/v1/agent/provision");
  assert.equal(
    live.gateway.requests.at(-1)?.headers.has("x-agent-signature"),
    true,
  );
}
test(authenticationRejectsUnrelatedChallengesAndPropagatesRefusals);

async function mcpEntryListsEveryToolAndSchemaOverStdio() {
  const { Client } = kitRequire(
    "@modelcontextprotocol/sdk/client/index.js",
  ) as typeof import("@modelcontextprotocol/sdk/client/index.js");
  const { StdioClientTransport } = kitRequire(
    "@modelcontextprotocol/sdk/client/stdio.js",
  ) as typeof import("@modelcontextprotocol/sdk/client/stdio.js");
  const dir = await mkdtemp(path.join(tmpdir(), "perps-agent-"));
  const keyFile = path.join(dir, "agent-seed");
  const keyHex = Buffer.from(seed).toString("hex");
  await writeFile(keyFile, keyHex, { mode: 0o600 });
  const envVars = {
    ...process.env,
    PAXEER_AGENT_GATEWAY_URL: "http://localhost",
    PAXEER_AGENT_NODE_URL: "http://localhost",
    NEXT_PUBLIC_PERPS_GATEWAY_URL: "http://localhost",
    PAXEER_AGENT_KEY_FILE: keyFile,
  };
  const transport = new StdioClientTransport({
    command: process.execPath,
    args: ["--import", "tsx", "packages/agent-kit/src/mcp-cli.ts"],
    cwd: root,
    env: Object.fromEntries(
      Object.entries(envVars).filter(
        (entry): entry is [string, string] => typeof entry[1] === "string",
      ),
    ),
    stderr: "pipe",
  });
  const client = new Client({ name: "agent-kit-tools-test", version: "1.0.0" });
  let stderr = "";
  transport.stderr?.on("data", (chunk) => {
    stderr += String(chunk);
  });
  try {
    await client.connect(transport);
    const response = await client.listTools();
    const expected = environment().kit.tools;
    assert.deepEqual(
      response.tools.map((tool) => tool.name).sort(),
      expected.map((tool) => tool.name).sort(),
    );
    for (const tool of response.tools) {
      assert.ok(tool.description && tool.description.length > 30);
      assert.equal(tool.inputSchema.type, "object");
      assert.equal(tool.inputSchema.additionalProperties, false);
      const schema = expected.find(
        (entry) => entry.name === tool.name,
      )!.inputSchema;
      assert.deepEqual(
        Object.keys(tool.inputSchema.properties ?? {}).sort(),
        Object.keys(schema.shape).sort(),
      );
    }
    // Schema rejection exercises the real MCP consumer without network access.
    const refused = await client.callTool({
      name: "place_limit",
      arguments: { ...restingInput, leverage: "wrong" },
    });
    assert.equal(refused.isError, true);
    assert.ok(!JSON.stringify(refused).includes(keyHex));
    assert.ok(!JSON.stringify(response).includes(keyHex));
    assert.ok(!stderr.includes(keyHex));
  } finally {
    await client.close();
    await transport.close();
    await rm(dir, { recursive: true, force: true });
  }
}
test({ timeout: 30_000 }, mcpEntryListsEveryToolAndSchemaOverStdio);

async function mcpStartupErrorsDoNotPrintKeyMaterial() {
  const dir = await mkdtemp(path.join(tmpdir(), "perps-agent-config-"));
  const keyFile = path.join(dir, "agent-seed");
  const invalidKey = "private-agent-material";
  await writeFile(keyFile, invalidKey, { mode: 0o600 });
  try {
    const child = spawn(
      process.execPath,
      ["--import", "tsx", "packages/agent-kit/src/mcp-cli.ts"],
      {
        cwd: root,
        env: {
          ...process.env,
          PAXEER_AGENT_GATEWAY_URL: "http://localhost",
          PAXEER_AGENT_NODE_URL: "http://localhost",
          NEXT_PUBLIC_PERPS_GATEWAY_URL: "http://localhost",
          PAXEER_AGENT_KEY_FILE: keyFile,
        },
        stdio: ["pipe", "pipe", "pipe"],
      },
    );
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => {
      stdout += String(chunk);
    });
    child.stderr.on("data", (chunk) => {
      stderr += String(chunk);
    });
    const code = await new Promise<number | null>((resolve, reject) => {
      child.once("error", reject);
      child.once("exit", resolve);
    });
    assert.equal(code, 1);
    assert.equal(stdout, "");
    assert.match(stderr, /Unable to start/);
    assert.ok(!stderr.includes(invalidKey));
    assert.ok(!stderr.includes(keyFile));
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
}
test({ timeout: 30_000 }, mcpStartupErrorsDoNotPrintKeyMaterial);

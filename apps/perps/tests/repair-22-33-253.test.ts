import "./fixtures/repair-env";
import "./fixtures/react-runtime";
import "./proof_core.test";
import "./proof_widgets.test";
import assert from "node:assert/strict";
import { test } from "node:test";
import { once } from "node:events";
import { createPublicClient, decodeFunctionData, encodeFunctionData, http } from "viem";
import { auditLoopback } from "./fixtures/audit-loopback";
import { configuredLimitPlan } from "./fixtures/configured-limit-plan";
import type { OrderFormState } from "../src/core/perps/orderPlan";

const WAD = 10n ** 18n;
const OWNER = "0x1234567890123456789012345678901234567890";

async function repair024AuthoritativeRpcInventoryAndConfiguredTransport() {
  const { getPaxeerRpcUrls } = await import("../src/core/rpc");
  const { paxeerChain } = await import("../src/core/chain");
  const oldMulti = process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
  const oldSingle = process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
  const fixture = await auditLoopback();
  try {
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
    const expected = [...Array.from({ length: 14 }, (_, i) => `https://api${i + 1}.mainnet-beta.paxeer.network`),
      "https://api-mainnet-beta.paxeer.network"];
    assert.deepEqual(getPaxeerRpcUrls(), expected);
    assert.equal(paxeerChain.id, 125);
    assert.deepEqual(paxeerChain.rpcUrls.default.http, expected);
    for (const url of getPaxeerRpcUrls()) assert.equal(new URL(url).pathname, "/");
    process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = ` ${fixture.origin},${fixture.origin},https://old.rpcpaxeer.online `;
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL = "https://ignored.example";
    assert.deepEqual(getPaxeerRpcUrls(), [fixture.origin]);
    const client = createPublicClient({ chain: paxeerChain, transport: http(getPaxeerRpcUrls()[0], { retryCount: 0 }) });
    assert.equal(await client.getChainId(), 125);
    assert.ok(fixture.state.rpcMethods.includes("eth_chainId"));
    delete process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL = fixture.origin;
    assert.deepEqual(getPaxeerRpcUrls(), [fixture.origin, ...expected]);
    process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = "https://api.hyperpax.xyz,https://old.rpcpaxeer.online";
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL = "https://stats.paxscan.io";
    assert.deepEqual(getPaxeerRpcUrls(), expected);
  } finally {
    if (oldMulti === undefined) delete process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
    else process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = oldMulti;
    if (oldSingle === undefined) delete process.env.NEXT_PUBLIC_PAXEER_RPC_URL;
    else process.env.NEXT_PUBLIC_PAXEER_RPC_URL = oldSingle;
    await fixture.close();
  }
}

async function repair008DecodedConfigAndTiersReachTheChainOrderCodec() {
  const fixture = await auditLoopback();
  const { paxeerChain } = await import("../src/core/chain");
  const client = createPublicClient({ chain: paxeerChain, transport: http(fixture.origin, { retryCount: 0 }) });
  const state: OrderFormState = {
    orderType: "limit", marketId: 0n, isLong: true, collateralAmount: "", leverage: "20",
    triggerPrice: "100", limitPrice: "", sizeUsd: "1000", scaledStartPrice: "", scaledEndPrice: "",
    scaledTotalOrders: "2", scaledDistribution: "flat", tpslEnabled: false, tpPrice: "", slPrice: "",
    reduceOnly: false, postOnly: true, ioc: false, fok: false, ladderId: 1n,
  };
  try {
    const plan = await configuredLimitPlan(client, state);
    assert.equal(plan.error, null);
    assert.equal(plan.calls.length, 1);
    assert.deepEqual(new Set(fixture.state.contractReads), new Set(["getMarketConfig", "getMarginTiers"]));
    const call = plan.calls[0];
    const decoded = decodeFunctionData({ abi: call.abi, data: encodeFunctionData(call) });
    assert.equal(decoded.functionName, "placeOrder");
    if (decoded.functionName !== "placeOrder") throw new Error("Expected placeOrder");
    assert.equal(decoded.args[0].leverage, 20n * WAD);
    assert.equal(decoded.args[0].marketId, 0n);
    assert.equal(decoded.args[0].limitPrice, 100n * WAD);
    const above = await configuredLimitPlan(client, { ...state, leverage: "21" });
    assert.equal(above.error?.code, "VALIDATION_ABOVE_MAX");
    assert.deepEqual(above.calls, []);
    const offTick = await configuredLimitPlan(client, { ...state, triggerPrice: "100.000000000000000001" });
    assert.equal(offTick.error?.code, "VALIDATION_PRECISION");
    assert.deepEqual(offTick.calls, []);
    fixture.state.enabled = false;
    await assert.rejects(configuredLimitPlan(client, state), /market must be enabled/);
  } finally { await fixture.close(); }
}

async function repair023FundingProgressAndGatewayIndexingShareAWallDeadline() {
  const { fundingVisibleThroughGateway } = await import("./e2e-local-node.test");
  const { paxeerChain } = await import("../src/core/chain");
  const { gqlEndpoint, gqlQuery } = await import("../src/core/graphql/client");
  const { FUNDING_PAYMENTS_QUERY } = await import("../src/hooks/perps/useSubgraph");
  const fixture = await auditLoopback();
  const h = {
    publicClient: createPublicClient({ chain: paxeerChain, transport: http(fixture.origin, { retryCount: 0 }) }),
    owner: OWNER, gqlQuery, FUNDING_PAYMENTS_QUERY, endpoint: gqlEndpoint(`${fixture.origin}/graphql`),
  } as const;
  try {
    const progressed = once(fixture.events, "block").then(() => {
      fixture.state.height++;
      fixture.state.timestamp = 3601n;
    });
    const indexed = once(fixture.events, "gateway").then(() => {
      fixture.state.funding.push({ positionId: "7", userAddress: OWNER });
    });
    {
      const rows = await fundingVisibleThroughGateway(h, 7n, { timeoutMs: 1500, pollMs: 5 });
      assert.deepEqual(rows, [{ positionId: "7", userAddress: OWNER }]);
      assert.ok(fixture.state.blockReads >= 2);
      assert.ok(fixture.state.gatewayReads >= 2);
      await Promise.all([progressed, indexed]);
      assert.equal(fixture.state.queries[0].query, FUNDING_PAYMENTS_QUERY);
      assert.deepEqual(fixture.state.queries[0].variables, { positionId: "7", userAddress: OWNER.toLowerCase() });
    }
    for (const mode of ["stalled-clock", "stalled-block-read", "stalled-gateway"] as const) {
      fixture.state.timestamp = 3599n;
      fixture.state.blockReads = 0;
      fixture.state.holdBlocks = mode === "stalled-block-read";
      fixture.state.holdGateway = mode === "stalled-gateway";
      fixture.state.funding = [];
      const advance = mode === "stalled-gateway" ? once(fixture.events, "block").then(() => {
        fixture.state.timestamp = 3601n;
        fixture.state.height++;
      }) : undefined;
      const start = Date.now();
      try {
        await assert.rejects(fundingVisibleThroughGateway(h, 7n, { timeoutMs: 1100, pollMs: 5 }),
          /Funding deadline exceeded: position=7 lastHeight=\d+ lastTime=\d+ settlesAt=3600/);
        assert.ok(Date.now() - start < 2000, `${mode} must stop on the wall deadline`);
      } finally { await advance; }
    }
  } finally {
    await fixture.close();
  }
}

test(repair024AuthoritativeRpcInventoryAndConfiguredTransport);
test(repair008DecodedConfigAndTiersReachTheChainOrderCodec);
test(repair023FundingProgressAndGatewayIndexingShareAWallDeadline);

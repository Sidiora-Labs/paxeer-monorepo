import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import test, { type TestContext } from "node:test";
import { isMainThread, parentPort, Worker, workerData } from "node:worker_threads";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient } from "@tanstack/react-query";
import { decodeFunctionData, encodeFunctionData, encodeFunctionResult, type Address, type Hex } from "viem";

const WAD = 10n ** 18n;
const UNIT = 10n ** 6n;
const owner: Address = "0x0000000000000000000000000000000000000195";
const foreign: Address = "0x0000000000000000000000000000000000000196";
type Scenario = "tiered" | "refresh";
type RpcRequest = { id: number; method: string; params: unknown[] };

async function exerciseRemoval(scenario: Scenario) {
  const store = {
    equityUsd: 150_000n * WAD,
    withdrawable: 50_000n * UNIT,
    user: owner,
    emptyTiers: false,
    failedAccountRead: false,
    block: 100n,
  };
  const reads: { name: string; block: string }[] = [];
  const failures: unknown[] = [];
  const server = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const request = JSON.parse(Buffer.concat(chunks).toString()) as RpcRequest;
      res.setHeader("Content-Type", "application/json");
      res.end(JSON.stringify(respond(request)));
    } catch (error) {
      failures.push(error);
      res.writeHead(500);
      res.end(String(error));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address === "object");
  const origin = `http://127.0.0.1:${address.port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  const { PerpsPrecompileABI: abi } = await import("../src/core/network/abi/PerpsPrecompile");
  const { PRECOMPILES } = await import("../src/core/network/precompiles");
  const { COLLATERAL_TOKENS } = await import("../src/core/perps/config");
  const {
    readCollateralRemovalRisk,
    collateralRemovalQueryOptions,
    canRemoveCollateral,
    prepareCollateralRemoval,
    remainingRemovalEquity,
  } = await import("../src/widgets/trading/modals/collateralRemoval");
  const { CollateralRemovalPreview } = await import("../src/widgets/trading/modals/AdjustCollateralModal");
  const { toCollateralUnits } = await import("../src/core/perps/collateralInputs");

  function respond(request: RpcRequest) {
    const response = (result: unknown) => ({ jsonrpc: "2.0", id: request.id, result });
    if (request.method === "net_listening") return response(true);
    if (request.method === "eth_blockNumber") return response(`0x${store.block.toString(16)}`);
    assert.equal(request.method, "eth_call");
    const [call, block] = request.params as [{ data: Hex; to: Address }, string];
    assert.equal(call.to.toLowerCase(), PRECOMPILES.perps.toLowerCase());
    assert.equal(block, `0x${store.block.toString(16)}`, "all risk reads use the same fresh block");
    const decoded = decodeFunctionData({ abi, data: call.data });
    reads.push({ name: decoded.functionName, block });
    if (decoded.functionName === "getPositionDetails") {
      assert.deepEqual(decoded.args, [42n]);
      return response(encodeFunctionResult({
        abi, functionName: "getPositionDetails", result: {
          positionId: 42n, user: store.user, marketId: 901n, isLong: true,
          sizeBase: 40n * WAD, sizeUsdAtMark: 2_000_000n * WAD,
          entryPrice: 51_000n * WAD, leverage: 10n * WAD,
          unrealizedPnlUsd: -40_000n * WAD, fundingOwedUsd: 10_000n * WAD,
          liquidationPrice: 48_000n * WAD, takeProfitPrice: 0n, stopLossPrice: 0n, openedAt: 1n,
        }
      }));
    }
    if (decoded.functionName === "getAccount") {
      assert.equal(decoded.args[0].toLowerCase(), owner.toLowerCase());
      if (store.failedAccountRead) return {
        jsonrpc: "2.0", id: request.id, error: { code: -32000, message: "Account risk read unavailable" },
      };
      return response(encodeFunctionResult({
        abi, functionName: "getAccount", result: {
          collateral: 200_000n * UNIT, equityUsd: store.equityUsd,
          initialMarginUsd: 100_000n * WAD, maintenanceMarginUsd: 60_000n * WAD,
          withdrawable: store.withdrawable, feeTier: 0, volume14dUsd: 0n,
          positionCount: 2n, openOrderCount: 0n,
        }
      }));
    }
    assert.equal(decoded.functionName, "getMarginTiers");
    assert.deepEqual(decoded.args, [901n]);
    return response(encodeFunctionResult({
      abi, functionName: "getMarginTiers", result: store.emptyTiers ? [] : [
        { lowerBoundUsd: 0n, maxLeverage: 1000n, maintenanceBps: 5n, deductionUsd: 0n },
        { lowerBoundUsd: 1_000_000n * WAD, maxLeverage: 20n, maintenanceBps: 250n, deductionUsd: 1000n * WAD },
      ]
    }));
  }

  const queryClient = new QueryClient();
  try {
    const risk = await queryClient.fetchQuery(collateralRemovalQueryOptions(42n, owner));
    assert.equal(risk.blockNumber, 100n);
    assert.equal(risk.marketMaintenanceUsd, 49_000n * WAD, "tier deduction applies to marked notional");
    assert.equal(risk.maintenanceMarginUsd, 60_000n * WAD, "other positions retain their maintenance");
    assert.equal(risk.equityUsd, 150_000n * WAD, "account equity already includes PnL and funding");
    assert.equal(risk.maxRemoval, 50_000n * UNIT, "account withdrawable is authoritative, in native units");
    assert.deepEqual(reads.map((read) => read.name).sort(), ["getAccount", "getMarginTiers", "getPositionDetails"]);

    if (scenario === "tiered") {
      const unsafeAmount = toCollateralUnits("100000");
      assert.ok(200_000 - 100_000 > 2_000_000 * 0.01, "the previous fixed-percent check offered this removal");
      assert.equal(canRemoveCollateral(risk, unsafeAmount), false);
      assert.equal(remainingRemovalEquity(risk, unsafeAmount), 50_000n * WAD);
      const html = renderToStaticMarkup(createElement(CollateralRemovalPreview, { risk, amount: unsafeAmount, loading: false }));
      assert.match(html, /Maximum removal: \$50000/);
      assert.match(html, /Market tier maintenance: \$49000\.00/);
      assert.match(html, /Account maintenance: \$60000\.00/);
      assert.match(html, /Remaining account equity: \$50000\.00/);
      assert.match(html, /role="alert"/);
      await assert.rejects(prepareCollateralRemoval(42n, unsafeAmount, owner), /current account margin/);
      assert.equal(canRemoveCollateral(risk, risk.maxRemoval), true);
      assert.equal(canRemoveCollateral(risk, risk.maxRemoval + 1n), false);
      await assert.rejects(prepareCollateralRemoval(42n, risk.maxRemoval + 1n, owner), /current account margin/);
      const call = await prepareCollateralRemoval(42n, risk.maxRemoval, owner);
      const consumer = decodeFunctionData({ abi, data: encodeFunctionData(call) });
      assert.equal(consumer.functionName, "removeCollateral");
      assert.deepEqual(consumer.args, [42n, 50_000n * UNIT]);
      assert.equal(call.address, PRECOMPILES.perps);
      assert.equal(COLLATERAL_TOKENS.usdl.decimals, 6);
      for (const amount of [0n, -1n, toCollateralUnits("0.0000001")]) {
        assert.equal(canRemoveCollateral(risk, amount), false);
      }
    } else {
      store.block = 101n;
      store.equityUsd = 100_000n * WAD;
      store.withdrawable = 0n;
      await assert.rejects(prepareCollateralRemoval(42n, 1n, owner), /current account margin/);
      assert.ok(reads.some((read) => read.name === "getAccount" && read.block === "0x65"), "submission refreshes risk after the preview");
      const drained = await readCollateralRemovalRisk(42n, owner);
      assert.equal(drained.maxRemoval, 0n);
      store.equityUsd = 60_000n * WAD;
      assert.equal((await readCollateralRemovalRisk(42n, owner)).maxRemoval, 0n, "no withdrawal at liquidation maintenance");
      store.equityUsd = -WAD;
      assert.equal((await readCollateralRemovalRisk(42n, owner)).maxRemoval, 0n, "negative equity is not spendable");
      store.emptyTiers = true;
      await assert.rejects(prepareCollateralRemoval(42n, 1n, owner), /maintenance tiers are unavailable/);
      store.emptyTiers = false;
      store.user = foreign;
      await assert.rejects(prepareCollateralRemoval(42n, 1n, owner), /unavailable for this account/);
      store.user = owner;
      store.failedAccountRead = true;
      await assert.rejects(readCollateralRemovalRisk(42n, owner), /Account risk read unavailable/);
      assert.equal(canRemoveCollateral(undefined, 1n), false);
      assert.match(renderToStaticMarkup(createElement(CollateralRemovalPreview, { risk: undefined, amount: 1n, loading: false })), /Removal is disabled/);
      assert.match(renderToStaticMarkup(createElement(CollateralRemovalPreview, { risk: undefined, amount: 1n, loading: true })), /Loading current margin/);
    }
    assert.deepEqual(failures, []);
  } finally {
    queryClient.clear();
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

async function runScenario(t: TestContext, scenario: Scenario) {
  const worker = new Worker(new URL(import.meta.url), { execArgv: ["--import", "tsx"], workerData: scenario });
  t.after(async () => { await worker.terminate(); });
  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) => reject(new Error(`Assertions exited before completion: ${code}`)));
    worker.once("message", (message: unknown) => {
      try { assert.equal(message, "assertions-passed"); resolve(); } catch (error) { reject(error); }
    });
  });
}

async function f017TieredMaintenanceAndAccountRisk(t: TestContext) {
  await runScenario(t, "tiered");
}

async function f017RefreshBeforeSubmissionAndUnavailableRisk(t: TestContext) {
  await runScenario(t, "refresh");
}

if (isMainThread) {
  test(f017TieredMaintenanceAndAccountRisk);
  test(f017RefreshBeforeSubmissionAndUnavailableRisk);
} else {
  void exerciseRemoval(workerData as Scenario).then(() => {
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  });
}

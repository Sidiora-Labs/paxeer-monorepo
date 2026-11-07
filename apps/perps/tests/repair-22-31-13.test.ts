import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import test, { type TestContext } from "node:test";
import {
  isMainThread,
  parentPort,
  Worker,
  workerData,
} from "node:worker_threads";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createPublicClient,
  decodeFunctionData,
  formatUnits,
  http,
  type Hex,
} from "viem";
import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import type { OrderFormState } from "../src/core/perps/orderPlan";
import {
  nodeLeverageOutput,
  nodeMarginTiers,
  nodeMarketConfig,
} from "./node-leverage-output";

const WAD = 10n ** 18n;
type Scenario = "transport" | "hook" | "supported" | "boundary" | "above";

type RpcRequest = {
  jsonrpc: "2.0";
  id: number;
  method: string;
  params: unknown[];
};

async function replayNodeOutput(
  run: (origin: string, reads: Map<string, number>) => Promise<void>,
) {
  const failures: unknown[] = [];
  const reads = new Map<string, number>();
  function respond(request: RpcRequest) {
    const transcripts = [
      ...Object.values(nodeLeverageOutput.calls),
      ...Object.values(nodeLeverageOutput.metadata),
    ];
    const transcript = transcripts.find(({ request: recorded }) => {
      if (request.method !== recorded.method) return false;
      if (recorded.method !== "eth_call") {
        return (
          JSON.stringify(request.params ?? []) ===
          JSON.stringify(recorded.params)
        );
      }
      const [call, block] = request.params as [
        { to: string; data: string },
        string,
      ];
      const [recordedCall] = recorded.params as [
        { to: string; data: string },
        string,
      ];
      return (
        call.to.toLowerCase() === recordedCall.to.toLowerCase() &&
        call.data.toLowerCase() === recordedCall.data.toLowerCase() &&
        (block === "latest" || block === nodeLeverageOutput.blockNumber)
      );
    });
    assert.ok(transcript, `Unrecorded RPC request: ${JSON.stringify(request)}`);
    const name =
      request.method === "eth_call"
        ? decodeFunctionData({
            abi: PerpsPrecompileABI,
            data: (request.params[0] as { data: Hex }).data,
          }).functionName
        : request.method;
    reads.set(name, (reads.get(name) ?? 0) + 1);
    return { ...transcript.response, id: request.id };
  }
  const server = createServer(async (request, response) => {
    try {
      assert.equal(request.url, "/");
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const rpc = JSON.parse(Buffer.concat(chunks).toString()) as
        | RpcRequest
        | RpcRequest[];
      response.setHeader("Content-Type", "application/json");
      response.end(
        JSON.stringify(Array.isArray(rpc) ? rpc.map(respond) : respond(rpc)),
      );
    } catch (error) {
      failures.push(error);
      response.statusCode = 500;
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  try {
    await run(`http://127.0.0.1:${address.port}`, reads);
    assert.deepEqual(
      failures,
      [],
      "all RPC reads must match the recorded node transcript",
    );
  } finally {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
    });
  }
}

async function assertScenario(
  scenario: Scenario,
  origin: string,
  reads: Map<string, number>,
) {
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  // Only RPC reads are needed; importing the form also validates gateway configuration.
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;

  if (scenario === "transport") {
    const client = createPublicClient({
      transport: http(origin),
    });
    assert.equal(await client.getChainId(), nodeLeverageOutput.chainId);
    const block = await client.getBlock({
      blockNumber: BigInt(nodeLeverageOutput.blockNumber),
    });
    assert.equal(block.hash, nodeLeverageOutput.blockHash);
    for (const [name, call] of Object.entries(nodeLeverageOutput.calls)) {
      const [input, blockNumber] = call.request.params;
      assert.equal(typeof input, "object");
      assert.equal(typeof blockNumber, "string");
      assert.ok(
        input && typeof input === "object" && "to" in input && "data" in input,
      );
      assert.equal(typeof input.to, "string");
      assert.equal(typeof input.data, "string");
      const result = await client.request({
        method: "eth_call",
        params: [
          { to: input.to as Hex, data: input.data as Hex },
          nodeLeverageOutput.blockNumber as Hex,
        ],
      });
      assert.equal(
        result,
        call.response.result,
        `${name} bytes must come from the real node`,
      );
      const decoded = decodeFunctionData({
        abi: PerpsPrecompileABI,
        data: input.data as Hex,
      });
      assert.equal(decoded.functionName, name);
      assert.equal(decoded.args?.[0], nodeMarketConfig.marketId);
    }
    assert.equal(nodeMarketConfig.maxLeverage, 1000n);
    assert.deepEqual(
      nodeMarginTiers.map((tier) => tier.maxLeverage),
      [1000n, 500n, 200n, 100n, 40n, 20n],
    );
    return;
  }

  const { orderLimitsFromConfig } = await import(
    "../src/widgets/trading/PerpsOrderForm"
  );
  const { buildOrderPlan } = await import("../src/core/perps/orderPlan");
  let data = { config: nodeMarketConfig, tiers: nodeMarginTiers };
  if (scenario === "hook") {
    const { marketConfigQueryOptions, useMarketConfig } = await import(
      "../src/hooks/perps/useMarketConfig"
    );
    const client = new QueryClient();
    try {
      data = await client.fetchQuery(
        marketConfigQueryOptions(nodeMarketConfig.marketId),
      );
      let renders = 0;
      function Consumer() {
        renders++;
        const result = useMarketConfig(nodeMarketConfig.marketId);
        assert.equal(result.error, null);
        assert.deepEqual(result.data, data);
        return React.createElement("span", null, result.data?.config.symbol);
      }
      assert.equal(
        renderToStaticMarkup(
          React.createElement(
            QueryClientProvider,
            { client },
            React.createElement(Consumer),
          ),
        ),
        `<span>${data.config.symbol}</span>`,
      );
      assert.equal(renders, 1);
      assert.equal(data.config.maxLeverage, nodeMarketConfig.maxLeverage);
      assert.deepEqual(data.tiers, nodeMarginTiers);
      assert.equal(reads.get("getMarketConfig"), 1);
      assert.equal(reads.get("getMarginTiers"), 1);
    } finally {
      client.clear();
    }
  }

  assert.equal(data.config.enabled, true);
  // Exercise each real tier boundary; the form must convert the selected cap once.
  for (const tier of data.tiers) {
    const notional = tier.lowerBoundUsd === 0n ? WAD : tier.lowerBoundUsd;
    const cap =
      tier.maxLeverage < data.config.maxLeverage
        ? tier.maxLeverage
        : data.config.maxLeverage;
    const limits = orderLimitsFromConfig(data, notional);
    assert.ok(limits);
    const leverage: bigint =
      scenario === "above"
        ? cap * WAD + 1n
        : scenario === "supported"
          ? WAD
          : cap * WAD;
    const state: OrderFormState = {
      orderType: "limit",
      marketId: data.config.marketId,
      isLong: true,
      collateralAmount: "",
      leverage: formatUnits(leverage, 18),
      triggerPrice: formatUnits(data.config.tickSize * 100n, 18),
      limitPrice: "",
      sizeUsd: formatUnits(notional, 18),
      scaledStartPrice: "",
      scaledEndPrice: "",
      scaledTotalOrders: "",
      scaledDistribution: "flat",
      tpslEnabled: false,
      tpPrice: "",
      slPrice: "",
      reduceOnly: false,
      postOnly: false,
      ioc: false,
      fok: false,
      ladderId: 1n,
    };
    const plan = buildOrderPlan(state, null, limits);
    const context = `tier ${tier.lowerBoundUsd}, leverage ${state.leverage}`;
    if (scenario === "above") {
      assert.equal(plan.error?.code, "VALIDATION_ABOVE_MAX", context);
      assert.deepEqual(plan.calls, [], context);
    } else {
      assert.equal(
        plan.error,
        null,
        `${context}: supported integer caps must reach placeOrder`,
      );
      assert.equal(plan.calls.length, 1, context);
      assert.equal(plan.calls[0].functionName, "placeOrder", context);
      assert.equal(plan.calls[0].args[0].leverage, leverage, context);
      assert.equal(plan.calls[0].args[0].sizeUsd, notional, context);
    }
    assert.equal(
      limits.maxLeverage,
      cap * WAD,
      "convert integer cap to OrderLimits exactly once",
    );
  }
  assert.equal(
    orderLimitsFromConfig(
      { ...data, config: { ...data.config, enabled: false } },
      WAD,
    ),
    null,
  );
}

async function runScenario(t: TestContext, scenario: Scenario) {
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
    workerData: scenario,
  });
  t.after(async () => {
    await worker.terminate();
  });
  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) =>
      reject(new Error(`Assertions exited before completion: ${code}`)),
    );
    worker.once("message", (message: unknown) => {
      try {
        assert.equal(message, "assertions-passed");
        resolve();
      } catch (error) {
        reject(error);
      }
    });
  });
}

async function finding006RealNodeAbiOutput(t: TestContext) {
  await runScenario(t, "transport");
}
async function finding006NodeThroughMarketConfigHookAndForm(t: TestContext) {
  await runScenario(t, "hook");
}
async function finding006SupportedLeverageReachesPlaceOrder(t: TestContext) {
  await runScenario(t, "supported");
}
async function finding006ExactTierCapsReachPlaceOrder(t: TestContext) {
  await runScenario(t, "boundary");
}
async function finding006OneWeiAboveTierCapsIsRefused(t: TestContext) {
  await runScenario(t, "above");
}

if (isMainThread) {
  test(finding006RealNodeAbiOutput);
  test(finding006NodeThroughMarketConfigHookAndForm);
  test(finding006SupportedLeverageReachesPlaceOrder);
  test(finding006ExactTierCapsReachPlaceOrder);
  test(finding006OneWeiAboveTierCapsIsRefused);
} else {
  void replayNodeOutput((origin, reads) =>
    assertScenario(workerData as Scenario, origin, reads),
  ).then(() => {
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  });
}

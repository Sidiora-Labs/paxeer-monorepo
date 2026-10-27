import { strict as assert } from "node:assert";
import test, { type TestContext } from "node:test";
import { isMainThread, parentPort, Worker } from "node:worker_threads";
import type { ContractFunctionReturnType } from "viem";

import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { PRECOMPILES } from "../src/core/network/precompiles";
import { decodeMarketConfig as decodeMarketConfigFromContract } from "../src/core/perps/marketConfig";

const decodeMarketConfig = decodeMarketConfigFromContract;
const ENV_NAMES = ["LOCAL_NODE_RPC"] as const;
type EnvName = (typeof ENV_NAMES)[number];

function requireEnv(): Record<EnvName, string> {
  const env = {} as Record<EnvName, string>;
  for (const name of ENV_NAMES) {
    const value = process.env[name];
    if (!value) throw new Error(`${name} is not set`);
    env[name] = value;
  }
  return env;
}

async function assertActiveMarketConfigs() {
  const env = requireEnv();
  const originalRpcUrls = process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = env.LOCAL_NODE_RPC;

  try {
    const { readContract } = await import("../src/hooks/perps/useContract");
    const marketIds = (await readContract({
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      functionName: "getActiveMarketIds",
    })) as ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getActiveMarketIds">;

    assert.ok(Array.isArray(marketIds), "getActiveMarketIds must return an array");
    assert.ok(marketIds.length > 0, "The local node must have at least one active market");

    for (const marketId of marketIds) {
      assert.equal(typeof marketId, "bigint", "Active market IDs must be uint256 values");
      const result = (await readContract({
        address: PRECOMPILES.perps,
        abi: PerpsPrecompileABI,
        functionName: "getMarketConfig",
        args: [marketId],
      })) as ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getMarketConfig">;
      const config = decodeMarketConfig(result);
      const context = `Market ${marketId}`;

      assert.equal(config.marketId, marketId, `${context} must decode its own config`);
      assert.equal(typeof config.symbol, "string", `${context} must decode a symbol`);
      assert.ok(config.symbol.trim().length > 0, `${context} must have a nonempty symbol`);
      assert.equal(typeof config.tickSize, "bigint", `${context} tickSize must be uint256`);
      assert.ok(config.tickSize > 0n, `${context} tickSize must be positive`);
      assert.equal(typeof config.lotSize, "bigint", `${context} lotSize must be uint256`);
      assert.ok(config.lotSize > 0n, `${context} lotSize must be positive`);
    }
  } finally {
    if (originalRpcUrls === undefined) delete process.env.NEXT_PUBLIC_PAXEER_RPC_URLS;
    else process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = originalRpcUrls;
  }
}

async function localMarketConfigDecodes(t: TestContext) {
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
  });
  t.after(async () => {
    await worker.terminate();
  });

  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) => {
      reject(new Error(`Market config worker exited before completing assertions (code ${code})`));
    });
    worker.once("message", (message: unknown) => {
      try {
        assert.equal(message, "market-configs-verified");
        resolve();
      } catch (error) {
        reject(error);
      }
    });
  });
}

if (isMainThread) {
  test(localMarketConfigDecodes);
} else {
  void assertActiveMarketConfigs().then(() => {
    assert.ok(parentPort, "Market config assertions must run in a worker");
    parentPort.postMessage("market-configs-verified");
  });
}

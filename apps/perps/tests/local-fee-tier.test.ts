import assert from "node:assert/strict";
import { test } from "node:test";
import { Worker, isMainThread, parentPort } from "node:worker_threads";
import { isHex, size, type ContractFunctionReturnType, type Hex } from "viem";
import { privateKeyToAccount } from "viem/accounts";

import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { PRECOMPILES } from "../src/core/network/precompiles";
import { feeTierView as mapFeeTier } from "../src/core/perps/feeTier";

const feeTierView = mapFeeTier;
const ENV_NAMES = ["LOCAL_NODE_RPC", "LOCAL_NODE_KEY", "NEXT_PUBLIC_PERPS_GATEWAY_URL"] as const;
type EnvName = (typeof ENV_NAMES)[number];

function requireEnv(): Record<EnvName, string> {
  const env = {} as Record<EnvName, string>;
  for (const name of ENV_NAMES) {
    const value = process.env[name];
    if (!value) throw new Error(`${name} is not set`);
    env[name] = value;
  }
  if (!isHex(env.LOCAL_NODE_KEY) || size(env.LOCAL_NODE_KEY) !== 32) {
    throw new Error("LOCAL_NODE_KEY is not a 32-byte hex private key");
  }
  return env;
}

async function readLocalFeeTier() {
  const env = requireEnv();
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = env.LOCAL_NODE_RPC;

  const { readContract } = await import("../src/hooks/perps/useContract");
  const account = privateKeyToAccount(env.LOCAL_NODE_KEY as Hex);
  const tierResult = (await readContract({
    address: PRECOMPILES.perps,
    abi: PerpsPrecompileABI,
    functionName: "getFeeTier",
    args: [account.address],
  })) as ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getFeeTier">;
  const fees = (await readContract({
    address: PRECOMPILES.perps,
    abi: PerpsPrecompileABI,
    functionName: "getFees",
  })) as ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getFees">;

  return { tierResult, fees };
}

async function localFeeTierDecodes() {
  const worker = new Worker(
    `const { workerData } = require("node:worker_threads");
     require("tsx/cjs/api").register();
     import("tsx/esm/api").then(({ register }) => {
       register();
       return import(workerData.moduleUrl);
     });`,
    {
      eval: true,
      execArgv: [],
      workerData: { moduleUrl: import.meta.url },
    },
  );

  try {
    const { tierResult, fees } = await new Promise<
      Awaited<ReturnType<typeof readLocalFeeTier>>
    >((resolve, reject) => {
      worker.once("message", resolve);
      worker.once("error", reject);
      worker.once("exit", (code) => {
        reject(new Error(`Fee-tier worker exited before returning its reads (exit ${code})`));
      });
    });

    const tier = feeTierView(tierResult);
    assert.equal(tier.volume14dUsd, 0n, "the local account has no trading volume");
    assert.equal(tier.tier, 0, "a fresh local account has fee tier 0");
    assert.equal(tier.takerBps, Number(fees[0]) / 100, "tier 0 uses the default taker fee in bps");
  } finally {
    await worker.terminate();
  }
}

if (isMainThread) {
  test(localFeeTierDecodes);
} else {
  assert.ok(parentPort, "fee-tier reads require a parent worker port");
  const port = parentPort;
  void readLocalFeeTier().then((result) => port.postMessage(result));
}

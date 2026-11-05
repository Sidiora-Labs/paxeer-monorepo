import { strict as assert } from "node:assert";
import { fork } from "node:child_process";
import { once } from "node:events";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import type { Address } from "viem";

import { SPOT_TOKENS } from "../src/core/spot/addresses";
import { buildGetTokenInfo, buildIsTokenPaused } from "../src/core/spot/calls";
import {
  spotTokenState as mapSpotTokenState,
  type SpotTokenState,
  type VaultTokenInfo,
} from "../src/core/spot/tokenState";

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

async function spotTokenState(token: Address): Promise<SpotTokenState> {
  const { readContract } = await import("../src/hooks/perps/useContract");
  const [paused, info] = await Promise.all([
    readContract(buildIsTokenPaused(token)) as Promise<boolean>,
    readContract(buildGetTokenInfo(token)) as Promise<VaultTokenInfo>,
  ]);
  assert.equal(paused, false, "the local vault reports USDL as unpaused");
  assert.equal(info[0], true, "USDL is registered in the local vault");
  assert.equal(info[1], true, "USDL is a stablecoin in the local vault");
  return mapSpotTokenState(paused, info);
}

async function localStableTokenTradeable() {
  const env = requireEnv();
  // Configure the RPC before importing the contract reader and its public client.
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = env.LOCAL_NODE_RPC;

  const { getPaxeerRpcUrls } = await import("../src/core/rpc");
  assert.deepEqual(getPaxeerRpcUrls(), [env.LOCAL_NODE_RPC], "reads must use only LOCAL_NODE_RPC");

  // The application transport ranks RPCs on a recurring timer with no disposal API.
  // Own its lifetime in a child process and release it after receiving the real read result.
  const reader = fork(fileURLToPath(import.meta.url), ["--read-vault", SPOT_TOKENS.USDL.address], {
    execArgv: ["--import", "tsx"],
    env: { ...process.env, NODE_TEST_CONTEXT: undefined },
    stdio: ["ignore", "inherit", "inherit", "ipc"],
    timeout: 60_000,
  });
  const exited = once(reader, "exit");
  try {
    const state = await new Promise<SpotTokenState>((resolve, reject) => {
      reader.once("message", (message: { state: SpotTokenState } | { error: string }) => {
        if ("error" in message) reject(new Error(message.error));
        else resolve(message.state);
      });
      reader.once("error", reject);
      reader.once("exit", (code, signal) => {
        reject(new Error(`vault reader exited before returning state: code=${code}, signal=${signal}`));
      });
    });
    assert.deepEqual(state, { tradeable: true, paused: false, reason: null });
  } finally {
    reader.kill();
    await exited;
  }
}

if (process.argv[2] === "--read-vault") {
  const token = process.argv[3];
  assert.equal(token, SPOT_TOKENS.USDL.address);
  assert.ok(process.send, "the vault reader requires an IPC channel");
  void spotTokenState(SPOT_TOKENS.USDL.address).then(
    (state) => process.send!({ state }),
    (error: unknown) => process.send!({ error: error instanceof Error ? error.stack : String(error) }),
  );
} else {
  test(localStableTokenTradeable);
}

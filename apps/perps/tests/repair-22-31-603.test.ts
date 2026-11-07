import assert from "node:assert/strict";
import { test } from "node:test";
import {
  getPaxeerWallet,
  resolvePaxeerConfig,
} from "../src/core/wallet/embedded/client";
import { PaxeerWallet } from "../src/core/wallet/embedded/sdk";

const ENV_KEYS = [
  "NEXT_PUBLIC_PAXEER_WALLET_API",
  "NEXT_PUBLIC_SUPABASE_URL",
  "NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY",
] as const;

async function withBrowserConfiguration(run: () => void | Promise<void>) {
  const savedEnv = new Map(ENV_KEYS.map((key) => [key, process.env[key]]));
  const savedWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  Object.defineProperty(globalThis, "window", {
    value: {},
    configurable: true,
  });
  delete process.env.NEXT_PUBLIC_PAXEER_WALLET_API;
  process.env.NEXT_PUBLIC_SUPABASE_URL = "http://127.0.0.1:54321";
  process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY = "repair-22-31-603";
  try {
    await run();
  } finally {
    for (const [key, value] of savedEnv) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
    if (savedWindow) Object.defineProperty(globalThis, "window", savedWindow);
    else Reflect.deleteProperty(globalThis, "window");
  }
}

async function missing_endpoint_refuses_with_the_setting_name() {
  await withBrowserConfiguration(() => {
    for (const endpoint of [undefined, "", "   "]) {
      if (endpoint === undefined)
        delete process.env.NEXT_PUBLIC_PAXEER_WALLET_API;
      else process.env.NEXT_PUBLIC_PAXEER_WALLET_API = endpoint;
      assert.throws(resolvePaxeerConfig, /NEXT_PUBLIC_PAXEER_WALLET_API/);
      assert.throws(getPaxeerWallet, /NEXT_PUBLIC_PAXEER_WALLET_API/);
    }
  });
}

async function configured_endpoint_is_used_by_the_embedded_client() {
  await withBrowserConfiguration(async () => {
    process.env.NEXT_PUBLIC_PAXEER_WALLET_API =
      "https://wallet-api.example.test/connector";
    const config = resolvePaxeerConfig();
    assert.ok(config);
    assert.equal(config.apiUrl, "https://wallet-api.example.test/connector");
    const wallet = getPaxeerWallet();
    assert.ok(wallet instanceof PaxeerWallet);
    try {
      assert.equal(getPaxeerWallet(), wallet);
      process.env.NEXT_PUBLIC_PAXEER_WALLET_API =
        "https://wallet-api.example.test/reconfigured";
      assert.equal(
        resolvePaxeerConfig()?.apiUrl,
        "https://wallet-api.example.test/reconfigured",
      );
      const reconfigured = getPaxeerWallet();
      assert.ok(reconfigured instanceof PaxeerWallet);
      try {
        assert.notEqual(reconfigured, wallet);
        delete process.env.NEXT_PUBLIC_PAXEER_WALLET_API;
        assert.throws(getPaxeerWallet, /NEXT_PUBLIC_PAXEER_WALLET_API/);
      } finally {
        await reconfigured.supabase.auth.stopAutoRefresh();
      }
    } finally {
      await wallet.supabase.auth.stopAutoRefresh();
    }
  });
}

test(missing_endpoint_refuses_with_the_setting_name);
test(configured_endpoint_is_used_by_the_embedded_client);

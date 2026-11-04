import assert from "node:assert/strict";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import type { Session } from "@supabase/supabase-js";
import { build } from "esbuild";
import { PaxeerProvider } from "@paxeer/wallet/provider";
import {
  readSequencerReceipt as packageReadSequencerReceipt,
  waitForSequencerReceipt as packageWaitForSequencerReceipt,
} from "@paxeer/wallet/receipt";
import * as packageSchemas from "@paxeer/api/schemas/tx";
import {
  PaxeerWallet,
  readSequencerReceipt,
  waitForSequencerReceipt,
} from "../src/core/wallet/embedded/sdk";
import * as gatewaySchemas from "../src/core/wallet/gateway";

async function repair_22_31_444_package_identity() {
  const wallet = new PaxeerWallet({
    apiUrl: "https://wallet.example",
    supabaseUrl: "https://auth.example",
    supabaseAnonKey: "provider-identity-test",
  });
  const session: Session = {
    access_token: "provider-identity-test",
    refresh_token: "provider-identity-test",
    expires_in: 3600,
    token_type: "bearer",
    user: {
      id: "00000000-0000-4000-8000-000000000444",
      aud: "authenticated",
      app_metadata: {},
      user_metadata: {},
      created_at: "1970-01-01T00:00:00Z",
    },
  };
  try {
    const provider = wallet.createProvider(
      { rpcUrl: "https://rpc.example", chainId: 125 },
      session,
    );
    assert.equal(provider.constructor, PaxeerProvider);
    assert.equal(Object.getPrototypeOf(provider), PaxeerProvider.prototype);
    assert.equal(await provider.request({ method: "eth_chainId" }), "0x7d");
    assert.equal(readSequencerReceipt, packageReadSequencerReceipt);
    assert.equal(waitForSequencerReceipt, packageWaitForSequencerReceipt);
    assert.equal(gatewaySchemas.TxRequest, packageSchemas.TxRequest);
    assert.equal(gatewaySchemas.SignTxBody, packageSchemas.SignTxBody);
    assert.equal(gatewaySchemas.SendTxBody, packageSchemas.SendTxBody);
    const tx = {
      to: "0x4444444444444444444444444444444444444444",
      value: "1",
      chainId: 125,
    };
    assert.deepEqual(gatewaySchemas.SendTxBody.parse({ tx }), { tx });
    assert.equal(
      gatewaySchemas.SendTxBody.safeParse({ tx: { value: "-1" } }).success,
      false,
    );
  } finally {
    await wallet.supabase.auth.stopAutoRefresh();
  }
}

async function repair_22_31_444_browser_bundle() {
  const result = await build({
    absWorkingDir: fileURLToPath(new URL("..", import.meta.url)),
    entryPoints: [
      "src/core/wallet/embedded/sdk/index.ts",
      "src/core/wallet/gateway/index.ts",
    ],
    outdir: "wallet-dependency-check",
    bundle: true,
    platform: "browser",
    format: "esm",
    write: false,
    metafile: true,
    logLevel: "silent",
  });
  const inputs = Object.keys(result.metafile.inputs);
  for (const path of [
    "sdk/wallet/src/provider.ts",
    "sdk/wallet/src/receipt.ts",
    "services/wallet/gateway/src/schemas/tx.ts",
  ]) {
    assert.ok(inputs.some((input) => input.endsWith(path)), path);
  }
  assert.equal(result.outputFiles.length, 2);
  assert.ok(result.outputFiles.every((file) => file.contents.length > 0));
}

test(repair_22_31_444_package_identity);
test(repair_22_31_444_browser_bundle);

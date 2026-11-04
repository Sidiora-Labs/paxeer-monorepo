/*
Proves: deployment URL configuration -> process.env -> explorer/client.ts explorerLink, programRecord and gas-station.ts sidioraStationSubmitUrl, sendSponsoredBatch.

The three LAYERX_EXPLORER_* interfaces in the inventory have already been
retired by explorer/client.ts explorerBaseUrl and programRecord: only
PAXEER_X_EXPLORER_BASE_URL configures the public explorer, without a bearer.
Their proof asserts that current behavior, including refusal without the
public base, rather than restoring the retired origins or credential.

Repairs: explorer/client.ts explorerBaseUrl accepted empty query/fragment
delimiters; gas-station.ts sendSponsoredBatch accepted an empty fragment.
The latter now shares its env decoder with sidioraStationSubmitUrl, whose
decoded endpoint is observable without submitting a transaction. The proof
ends at the env consumers; it replaces no fetch, signer, SDK or decoder.
There is no persistent state: every reader is exercised twice, then after
changing its producer's env value, to rule out an import-time cached value.
*/
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import type { GasStationConfig, SponsoredBatch } from "../../../services/agent/sdk/typescript/src/gas-station.ts";

// Resolve source imports and Next's real server-condition marker for Node.
// All implementation modules execute unchanged.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") {
      return nextResolve("next/dist/compiled/server-only/empty.js", context);
    }
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      if (candidate.pathname.endsWith(".js")) {
        const source = new URL(candidate.href.slice(0, -3) + ".ts");
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.endsWith(".ts") && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const explorer = await import("../src/explorer/client.ts");
const station = await import("../src/api/gas-station.ts");
const sdk = await import("../../../services/agent/sdk/typescript/src/gas-station.ts");
const { privateKeyToAccount } = await import("viem/accounts");

const IDENTIFIER = "ab".repeat(32);
const TARGET = { kind: "receipt", receiptId: IDENTIFIER, transactionHash: `0x${"b".repeat(64)}` } as const;
const PUBLIC_BASE = "PAXEER_X_EXPLORER_BASE_URL";
const RETIRED_KEYS = [
  "LAYERX_EXPLORER_API_ORIGIN",
  "LAYERX_EXPLORER_PROGRAM_API_ORIGIN",
  "LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN",
] as const;

function preserveEnvironment(context: TestContext, keys: readonly string[]): void {
  const previous = keys.map((key) => [key, process.env[key]] as const);
  context.after(() => {
    for (const [key, value] of previous) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  });
  for (const key of keys) delete process.env[key];
}

async function proveRetiredExplorerSetting(context: TestContext, key: typeof RETIRED_KEYS[number]): Promise<void> {
  preserveEnvironment(context, [PUBLIC_BASE, ...RETIRED_KEYS]);
  for (const host of ["first.example", "second.example"]) {
    const producer = new URL(`https://${host}`);
    process.env[PUBLIC_BASE] = producer.href;
    process.env[key] = key.endsWith("TOKEN") ? "retired-credential" : "https://retired.example";
    for (let read = 0; read < 2; read += 1) {
      assert.equal(explorer.explorerLink(TARGET), new URL(`/tx/0x${"b".repeat(64)}`, producer).href);
    }
    // A malformed retired value cannot override the current public producer.
    process.env[key] = "\r\nmalformed";
    assert.equal(explorer.explorerLink(TARGET), new URL(`/tx/0x${"b".repeat(64)}`, producer).href);
    assert.throws(() => explorer.explorerLink({ kind: "receipt", receiptId: "malformed", transactionHash: `0x${"b".repeat(64)}` }), TypeError);

    // Every real read must decode the current env, even after a successful read.
    process.env[key] = key.endsWith("TOKEN") ? "retired-credential" : "https://retired.example";
    for (const malformed of [undefined, "not a URL", `${producer.origin}/?`, `${producer.origin}/#`]) {
      if (malformed === undefined) delete process.env[PUBLIC_BASE];
      else process.env[PUBLIC_BASE] = malformed;
      assert.throws(() => explorer.explorerLink(TARGET), explorer.ExplorerUnavailableError);
      await assert.rejects(explorer.receiptRecord(IDENTIFIER), explorer.ExplorerUnavailableError);
      await assert.rejects(explorer.programRecord(IDENTIFIER), explorer.ExplorerUnavailableError);
    }
  }
}

async function LAYERX_EXPLORER_API_ORIGIN(context: TestContext): Promise<void> {
  await proveRetiredExplorerSetting(context, "LAYERX_EXPLORER_API_ORIGIN");
}

async function LAYERX_EXPLORER_PROGRAM_API_ORIGIN(context: TestContext): Promise<void> {
  await proveRetiredExplorerSetting(context, "LAYERX_EXPLORER_PROGRAM_API_ORIGIN");
}

async function LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN(context: TestContext): Promise<void> {
  await proveRetiredExplorerSetting(context, "LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN");
}

async function PAXEER_GAS_STATION_SUBMIT_URL(context: TestContext): Promise<void> {
  preserveEnvironment(context, [
    "PAXEER_GAS_STATION_SUBMIT_URL", "PAXEER_GAS_STATION_QUOTE_URL",
    "PAXEER_GAS_STATION_CHAIN_ID", "PAXEER_GAS_STATION_SPONSOR", "PAXEER_GAS_STATION_PAYMASTER",
  ]);
  const account = privateKeyToAccount(`0x${"0b".repeat(32)}`);
  const sponsor = privateKeyToAccount(`0x${"0c".repeat(32)}`);
  const config: GasStationConfig = {
    chainId: 1325n, sponsor: sponsor.address, paymaster: `0x${"44".repeat(20)}`,
    token: sdk.SIDIORA_TOKEN, decimals: sdk.SIDIORA_DECIMALS,
  };
  Object.assign(process.env, {
    PAXEER_GAS_STATION_QUOTE_URL: new URL("/quote", "https://station.example").href,
    PAXEER_GAS_STATION_CHAIN_ID: config.chainId.toString(),
    PAXEER_GAS_STATION_SPONSOR: config.sponsor,
    PAXEER_GAS_STATION_PAYMASTER: config.paymaster,
  });
  const batch: SponsoredBatch = {
    chainId: config.chainId, account: account.address, nonce: 7n,
    calls: [{ to: `0x${"33".repeat(20)}`, value: 0n, data: "0x1234" }],
    quote: {
      sponsor: config.sponsor, token: config.token, decimals: config.decimals,
      maxTokenAmount: 2_100_000n, tokenAmount: 2_000_001n,
      deadline: BigInt(Math.floor(Date.now() / 1000)) + 300n, quoteNonce: 9n, gasCost: 10n ** 18n,
    },
  };
  const quoteDigest = sdk.gasQuoteDigest(config.chainId, account.address, batch.quote);
  const batchDigest = sdk.sponsoredBatchDigest(batch);
  const authorizationNonce = 8n;
  const authorizationDigest = sdk.eip7702AuthorizationDigest({
    chainId: config.chainId, address: config.paymaster, nonce: authorizationNonce,
  });
  assert.ok(quoteDigest.ok);
  assert.ok(batchDigest.ok);
  assert.ok(authorizationDigest.ok);
  const relayerSignature = await sponsor.sign({ hash: quoteDigest.value as `0x${string}` });
  const accountSignature = await account.sign({ hash: batchDigest.value as `0x${string}` });
  const authorizationSignature = await account.sign({ hash: authorizationDigest.value as `0x${string}` });
  const quoted = { batch, relayerSignature, paymaster: config.paymaster };
  assert.ok(sdk.sponsoredBatchCall(config, batch, accountSignature, relayerSignature).ok);
  assert.ok(sdk.assembleEip7702Authorization(config, account.address, authorizationNonce, authorizationSignature).ok);

  for (const host of ["first-station.example", "second-station.example"]) {
    const producer = new URL("/submit?network=1325", `https://${host}`);
    process.env.PAXEER_GAS_STATION_SUBMIT_URL = producer.href;
    for (let read = 0; read < 2; read += 1) {
      assert.equal(await station.sidioraStationSubmitUrl(), producer.href);
    }
    // Real signatures and configuration isolate refusal to the env decoder.
    for (const malformed of [undefined, "", "not a URL", `http://${host}/submit`,
      `https://user:password@${host}/submit`, `${producer.href}#fragment`, `${producer.href}#`]) {
      if (malformed === undefined) delete process.env.PAXEER_GAS_STATION_SUBMIT_URL;
      else process.env.PAXEER_GAS_STATION_SUBMIT_URL = malformed;
      assert.equal(await station.sidioraStationSubmitUrl(), undefined);
      assert.deepEqual(await station.sendSponsoredBatch(quoted, accountSignature, authorizationNonce, authorizationSignature), {
        outcome: "unavailable",
      });
    }
    process.env.PAXEER_GAS_STATION_SUBMIT_URL = producer.href;
    assert.equal(await station.sidioraStationSubmitUrl(), producer.href);
    const wrongSigner = await sponsor.sign({ hash: batchDigest.value as `0x${string}` });
    assert.deepEqual(await station.sendSponsoredBatch(quoted, wrongSigner, authorizationNonce, authorizationSignature), {
      outcome: "rejected",
    });
  }
}

test(LAYERX_EXPLORER_API_ORIGIN);
test(LAYERX_EXPLORER_PROGRAM_API_ORIGIN);
test(LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN);
test(PAXEER_GAS_STATION_SUBMIT_URL);

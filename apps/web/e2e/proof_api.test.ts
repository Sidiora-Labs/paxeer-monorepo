// Proves: api/wallet.ts (sendWalletPrecompileCall, sendWalletSponsoredBatch, resumeWalletSponsoredSubmission) and api/gas-station.ts (requestSidioraGasQuote, sendSponsoredBatch) -> api/sdk.ts exports and api/provider.ts windowWalletProvider -> bridge/sendBridgeOut, exchange/sendPerpsWrite, launchpad/sendWithApproval and _markets/SendOutcome.
// Repairs: SendOutcome silently accepted unknown variants and threw on malformed receipts;
// sidioraQuotePresentation threw on malformed quote envelopes instead of refusing them;
// windowWalletProvider accepted arrays with a request property. All are in task scope.
// The normal consent/cancellation path uses the actual PaxeerProvider and SDK builders.
// No RPC, fetch, API module, signature verifier, or consumer is replaced. These APIs
// have no retained anchor state: repeat calls and change the provider between them.
// Wallet/UI packages load their real sources; the only workspace dist dependency
// is @sidiora/layerx-sdk/browser, built by verify_cmd before this file runs.
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import type { PrecompileSendOutcome } from "../src/api/wallet.ts";
import type { SidioraGasQuote, GasQuoteOutcome } from "../src/api/gas-station.ts";

// Node's source loader, using the same real modules as Next's package resolver.
// The server-only marker uses Next's server entry, since this is a Node server.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "@paxeer/wallet" || specifier === "@paxeer/wallet/provider") {
      const source = specifier.endsWith("/provider") ? "provider.ts" : "index.ts";
      return nextResolve(new URL(`../../../sdk/wallet/src/${source}`, import.meta.url).href, context);
    }
    if (specifier.startsWith("@layerx/ui/")) {
      const source = specifier === "@layerx/ui/cn" ? "lib/utils" : specifier.slice("@layerx/ui/".length);
      for (const suffix of [".ts", ".tsx"]) {
        const url = new URL(`../packages/layerx-ui/src/${source}${suffix}`, import.meta.url);
        if (existsSync(url)) return nextResolve(url.href, context);
      }
    }
    if (specifier === "server-only") return nextResolve("next/dist/compiled/server-only/empty.js", context);
    if (specifier === "next/navigation" || specifier === "next/link") return nextResolve(`${specifier}.js`, context);
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const { privateKeyToAccount } = await import("viem/accounts");
const { PaxeerProvider, constructionDigest } = await import("@paxeer/wallet/provider");
const sdk = await import("../src/api/sdk.ts");
const wallet = await import("../src/api/wallet.ts");
const station = await import("../src/api/gas-station.ts");
const { windowWalletProvider } = await import("../src/api/provider.ts");
const { SendOutcome } = await import("../src/app/_markets/send-outcome.tsx");
const bridge = await import("../src/app/bridge/bridge-client.tsx");
const exchange = await import("../src/app/exchange/exchange-client.tsx");
const launchpad = await import("../src/app/launchpad/launchpad-client.tsx");

const signer = privateKeyToAccount(`0x${"0b".repeat(32)}`);
const sponsor = privateKeyToAccount(`0x${"0c".repeat(32)}`);
const stranger = privateKeyToAccount(`0x${"0d".repeat(32)}`);
const account = signer.address.toLowerCase();
const paymaster = `0x${"44".repeat(20)}`;
const call = sdk.bridgeOutCall(1n, sdk.SIDIORA_TOKEN, 1n, account);

function observed(outcome: PrecompileSendOutcome | undefined): string {
  return renderToStaticMarkup(createElement(SendOutcome, { outcome }));
}

function installProvider(provider: InstanceType<typeof PaxeerProvider>): void {
  // Only the browser host is supplied; ethereum is the production SDK provider.
  Object.defineProperty(globalThis, "window", { configurable: true, value: { ethereum: provider } });
}

function removeWindow(): void {
  Reflect.deleteProperty(globalThis, "window");
}

async function signedBatch(): Promise<SidioraGasQuote> {
  const quote = {
    sponsor: sponsor.address.toLowerCase(), token: sdk.SIDIORA_TOKEN, decimals: sdk.SIDIORA_DECIMALS,
    maxTokenAmount: 2_100_000n, tokenAmount: 2_000_001n,
    deadline: BigInt(Math.floor(Date.now() / 1000)) + 300n, quoteNonce: 7n, gasCost: 21_000n,
  };
  const digest = sdk.gasQuoteDigest(125n, account, quote);
  assert.ok(digest.ok);
  return {
    batch: { chainId: 125n, account, nonce: 0n, calls: [call], quote }, paymaster,
    relayerSignature: await sponsor.sign({ hash: digest.value as `0x${string}` }),
  };
}

async function api_provider_to_bridge_observes_each_real_cancellation(): Promise<void> {
  let approvals = 0;
  const provider = new PaxeerProvider({
    gatewayUrl: "http://127.0.0.1:1", rpcUrl: "http://127.0.0.1:1", token: () => null,
    confirm: () => { approvals += 1; return false; },
  });
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  try {
    wallet.chooseWalletFee("paxeer", "");
    installProvider(provider);
    for (let attempt = 1; attempt <= 2; attempt += 1) {
      assert.equal(windowWalletProvider(), provider);
      const result = await bridge.sendBridgeOut(account, "1", sdk.SIDIORA_TOKEN, 1n, account);
      assert.deepEqual(result, { outcome: "cancelled" });
      assert.match(observed(result), /You cancelled in the wallet\./u);
      assert.equal(approvals, attempt, "each call reaches the real provider consent boundary");
    }
    const disconnected = new PaxeerProvider({ gatewayUrl: "http://127.0.0.1:1", rpcUrl: "http://127.0.0.1:1", token: () => null });
    installProvider(disconnected);
    assert.equal(windowWalletProvider(), disconnected);
    const refused = await wallet.sendWalletPrecompileCall(account, () => call);
    assert.deepEqual(refused, { outcome: "rejected" });
    assert.match(observed(refused), /The wallet refused this account\./u);
    assert.equal(approvals, 2, "the previous provider is never cached");

    const malformed = Object.assign([], { request: provider.request.bind(provider) });
    Object.defineProperty(globalThis, "window", { configurable: true, value: { ethereum: malformed } });
    assert.equal(windowWalletProvider(), undefined);
  } finally {
    removeWindow();
    wallet.chooseWalletFee("paxeer", "");
    if (previousWindow !== undefined) Object.defineProperty(globalThis, "window", previousWindow);
  }
}

test(api_provider_to_bridge_observes_each_real_cancellation);

async function api_sponsored_consent_crosses_sdk_and_refuses_changed_or_malformed_quotes(): Promise<void> {
  const quoted = await signedBatch();
  const provider = new PaxeerProvider({ gatewayUrl: "http://127.0.0.1:1", rpcUrl: "http://127.0.0.1:1", token: () => null });
  const digest = sdk.sponsoredBatchDigest(quoted.batch);
  assert.ok(digest.ok);
  for (let attempt = 0; attempt < 2; attempt += 1) {
    const review = wallet.sidioraQuotePresentation(quoted);
    assert.ok(review);
    assert.equal(review.amount, "2.000001");
    assert.equal(review.maximum, "2.100000");
    assert.match(review.consent, /2\.000001/u);
    const construction = wallet.sponsoredWalletModule(quoted, provider).construction(quoted.batch);
    assert.equal(constructionDigest(construction), digest.value, "consumer constructs the same signed SDK batch");
    const cancelled = await wallet.sendWalletSponsoredBatch(quoted, undefined);
    assert.deepEqual(cancelled, { outcome: "cancelled" });
    assert.match(observed(cancelled), /You cancelled in the wallet\./u);
    const changed = { ...quoted, batch: { ...quoted.batch, nonce: 1n } };
    assert.notEqual(wallet.sidioraQuotePresentation(changed)?.identity, review.identity);
    const refused = await wallet.sendWalletSponsoredBatch(changed, review.identity);
    assert.deepEqual(refused, { outcome: "rejected" });
    assert.match(observed(refused), /The wallet refused this account\./u);
  }
  const malformed = { ...quoted, batch: null } as unknown as SidioraGasQuote;
  assert.equal(wallet.sidioraQuotePresentation(malformed), undefined);
  assert.deepEqual(await wallet.sendWalletSponsoredBatch(malformed, "invalid"), { outcome: "rejected" });
}

test(api_sponsored_consent_crosses_sdk_and_refuses_changed_or_malformed_quotes);

async function api_station_exports_preserve_real_refusals_and_signature_checks(): Promise<void> {
  const names = ["PAXEER_GAS_STATION_QUOTE_URL", "PAXEER_GAS_STATION_CHAIN_ID", "PAXEER_GAS_STATION_SPONSOR", "PAXEER_GAS_STATION_PAYMASTER", "PAXEER_GAS_STATION_SUBMIT_URL"];
  const previous = names.map((name) => process.env[name]);
  const quoted = await signedBatch();
  const request = { account, did: `did:layerx:${"55".repeat(32)}`, nonce: 0n, calls: [call], maxTokenAmount: 2_100_000n, gasCost: 21_000n };
  try {
    for (const name of names) delete process.env[name];
    for (let attempt = 0; attempt < 2; attempt += 1) {
      const unavailable: GasQuoteOutcome = await sdk.requestSidioraGasQuote(request, 125n);
      assert.deepEqual(unavailable, { ok: false, failure: { outcome: "unavailable" } });
      assert.equal(unavailable.ok, false);
      if (!unavailable.ok) assert.match(observed(unavailable.failure), /No Paxeer wallet is connected\./u);
    }
    Object.assign(process.env, {
      PAXEER_GAS_STATION_QUOTE_URL: "https://127.0.0.1:1/quote",
      PAXEER_GAS_STATION_SUBMIT_URL: "https://127.0.0.1:1/submit",
      PAXEER_GAS_STATION_CHAIN_ID: "125", PAXEER_GAS_STATION_SPONSOR: sponsor.address,
      PAXEER_GAS_STATION_PAYMASTER: paymaster,
    });
    assert.equal(sdk.requestSidioraGasQuote, station.requestSidioraGasQuote);
    assert.equal(sdk.sendSponsoredBatch, station.sendSponsoredBatch);
    for (let attempt = 0; attempt < 2; attempt += 1) {
      assert.deepEqual(await sdk.requestSidioraGasQuote(request, 1325n), { ok: false, failure: { outcome: "rejected" } });
      const malformed = await sdk.requestSidioraGasQuote({ ...request, did: "not-a-did" }, 125n);
      assert.deepEqual(malformed, { ok: false, failure: { outcome: "rejected" } });
      if (!malformed.ok) assert.match(observed(malformed.failure), /The wallet refused this account\./u);
      const batchDigest = sdk.sponsoredBatchDigest(quoted.batch);
      const authDigest = sdk.eip7702AuthorizationDigest({ chainId: 125n, address: paymaster, nonce: 0n });
      assert.ok(batchDigest.ok && authDigest.ok);
      const wrongSignature = await stranger.sign({ hash: batchDigest.value as `0x${string}` });
      const authorization = await signer.sign({ hash: authDigest.value as `0x${string}` });
      const refused = await sdk.sendSponsoredBatch(quoted, wrongSignature, 0n, authorization);
      assert.deepEqual(refused, { outcome: "rejected" });
      assert.match(observed(refused), /The wallet refused this account\./u);
      assert.deepEqual(await sdk.sendSponsoredBatch({ ...quoted, paymaster: null } as unknown as SidioraGasQuote, wrongSignature, 0n, authorization), { outcome: "rejected" });
    }
  } finally {
    names.forEach((name, index) => {
      const value = previous[index];
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    });
  }
}

test(api_station_exports_preserve_real_refusals_and_signature_checks);

async function api_consumers_observe_unavailable_and_resume_without_inventing_state(): Promise<void> {
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  removeWindow();
  try {
    const order = { pool: paymaster, amountIn: 1n, minOut: 1n, deadline: BigInt(Math.floor(Date.now() / 1000)) + 300n };
    for (let attempt = 0; attempt < 2; attempt += 1) {
      const results = [
        await bridge.sendBridgeOut(account, "1", sdk.SIDIORA_TOKEN, 1n, account),
        await exchange.sendPerpsWrite(account, { token: sdk.SIDIORA_TOKEN, symbol: "SID", decimals: 6 }, 0n, exchange.perpsOrderCancelCall(1n), () => assert.fail("no approval without a wallet")),
        await launchpad.sendWithApproval(account, { kind: "buy", order }, () => sdk.launchpadBuyCall(order), () => assert.fail("no approval without a wallet")),
        await wallet.resumeWalletSponsoredSubmission(account, call),
      ];
      for (const result of results) {
        assert.deepEqual(result, { outcome: "unavailable" });
        assert.match(observed(result), /No Paxeer wallet is connected\./u);
      }
    }
  } finally {
    if (previousWindow !== undefined) Object.defineProperty(globalThis, "window", previousWindow);
  }
}

test(api_consumers_observe_unavailable_and_resume_without_inventing_state);

async function api_consumer_refuses_corrupted_producer_results_at_decode(): Promise<void> {
  const quoted = await signedBatch();
  const produced = await wallet.sendWalletSponsoredBatch(quoted, undefined);
  assert.deepEqual(produced, { outcome: "cancelled" });
  const corruptions: readonly unknown[] = [
    null, [], { ...produced, outcome: "unknown" }, { ...produced, detail: 1 },
    { ...produced, outcome: "sent", transactionHash: "0x1234" },
    { ...produced, outcome: "sent", transactionHash: `0x${"11".repeat(32)}`, receipt: null },
    { ...produced, outcome: "reverted", transactionHash: `0x${"11".repeat(32)}` },
  ];
  assert.equal(observed(undefined), "");
  for (const malformed of corruptions) {
    assert.equal(wallet.isPrecompileSendOutcome(malformed), false);
    const html = observed(malformed as PrecompileSendOutcome);
    assert.match(html, /role="alert"/u);
    assert.match(html, /malformed result/u);
    assert.doesNotMatch(html, /Sent\. Transaction/u);
  }
  assert.match(observed(produced), /You cancelled in the wallet\./u, "a refused response does not poison the next decode");
}

test(api_consumer_refuses_corrupted_producer_results_at_decode);

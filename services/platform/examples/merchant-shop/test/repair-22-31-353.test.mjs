import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { join } from "node:path";
import test from "node:test";
import { decodeRawEip1559, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { SequencerReceiptClient } from "../../support/runtime.mjs";
import { loadMerchantShopConfig, merchantShopConfiguration } from "../config.mjs";

// The gate builds the SDK. Load the other production workspace modules from
// source, including their real decoders, without requiring prebuilt middleware.
const middlewareRoot = new URL("../../../middleware/", import.meta.url);
const packages = new Map([
  ["@sidiora/layerx-seller-middleware", new URL("seller/src/index.ts", middlewareRoot).href],
  ["@sidiora/layerx-merchant-middleware", new URL("merchant/src/index.ts", middlewareRoot).href],
]);
const hooks = registerHooks({
  resolve(specifier, context, nextResolve) {
    if (packages.has(specifier)) return { url: packages.get(specifier), shortCircuit: true };
    if (context.parentURL?.startsWith(middlewareRoot.href)
      && context.parentURL.endsWith(".ts") && specifier.startsWith(".") && specifier.endsWith(".js")) {
      return nextResolve(new URL(specifier.replace(/\.js$/u, ".ts"), context.parentURL).href, context);
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(middlewareRoot.href) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return nextLoad(url, context);
  },
});
const { decodePaymentRequiredHeader, decodeSettlementHeader, encodePaymentPayloadHeader, verifyPaymentReceipt } =
  await import("@sidiora/layerx-seller-middleware");
const { beta, document, bindEnvironment, bytes, createMerchantRepairFixture } = await import("../repair_22_31_353.mjs");
hooks.deregister();

async function merchantChainOrigin(context) {
  assert.equal(beta.rpcUrl, "https://api-mainnet-beta.paxeer.network");
  assert.equal(beta.network, "eip155:125");
  const app = await createMerchantRepairFixture(context);
  const receipt = await app.produce();
  const chainId = await fetch(app.config.rpcUrl, {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "eth_chainId", params: [] }),
    signal: AbortSignal.timeout(8000),
  });
  assert.equal(chainId.status, 200);
  assert.deepEqual(await chainId.json(), { jsonrpc: "2.0", id: 1, result: "0x7d" });
  assert.equal(decodeRawEip1559(bytes(receipt.rawTransaction)).chainId, 125n);
  const proof = await new SequencerReceiptClient(app.config.rpcUrl, [{
    keyId: app.key.keyId, publicKey: app.key.publicKey,
    first_height: app.key.firstHeight, last_height: app.key.lastHeight,
  }]).resolve(bytes(receipt.rawTransaction));
  const { txHash, ...expected } = receipt;
  assert.deepEqual(proof, expected);
  const verified = await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [{
    keyId: app.key.keyId, publicKey: app.key.publicKey,
    first_height: app.key.firstHeight, last_height: app.key.lastHeight,
  }]);
  assert.equal(verified.height, 7n);
  assert.deepEqual(app.chainRequests, [
    { jsonrpc: "2.0", id: 1, method: "eth_chainId", params: [] },
    { jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [txHash] },
  ]);
  assert.equal(app.settlementRequests.length, 0);
}

async function merchantSettlementOrigin(context) {
  const app = await createMerchantRepairFixture(context);
  const unauthenticated = await fetch(app.config.settlementUrl, {
    method: "POST", headers: { "content-type": "application/json" }, body: "{}",
    signal: AbortSignal.timeout(8000),
  });
  assert.equal(unauthenticated.status, 401);
  await unauthenticated.arrayBuffer();
  const quote = await app.checkout("checkout-offer");
  assert.equal(quote.status, 402);
  const offer = decodePaymentRequiredHeader(quote.headers.get("payment-required"));
  assert.equal(offer.accepts[0].network, "eip155:125");
  assert.equal((await quote.json()).order.state, "awaiting-payment");
  const receipt = await app.produce(offer.accepts[0].extra.layerx.purposeHash);
  const payment = encodePaymentPayloadHeader({
    x402Version: 2, resource: offer.resource, accepted: offer.accepts[0],
    payload: { transaction: receipt.txHash, receiptDigest: receipt.receiptDigest },
  });
  const paid = await app.checkout("checkout-offer", payment);
  const result = await paid.json();
  assert.equal(paid.status, 200, JSON.stringify(result));
  const settlement = decodeSettlementHeader(paid.headers.get("payment-response"));
  assert.equal(settlement.transaction, receipt.txHash);
  const { txHash, ...proof } = receipt;
  assert.deepEqual(settlement.extensions.layerx, proof);
  const verified = await verifyPaymentReceipt(proof, offer.accepts[0], [{
    keyId: app.key.keyId, publicKey: app.key.publicKey,
    first_height: app.key.firstHeight, last_height: app.key.lastHeight,
  }]);
  assert.equal(verified.txHash, txHash);
  assert.equal(result.order.state, "paid-verified");
  assert.equal(result.order.transaction, txHash);
  assert.equal(result.order.receiptDigest, proof.receiptDigest.slice(2));
  assert.deepEqual(await app.order("checkout-offer"), result.order);
  assert.deepEqual(JSON.parse(await readFile(join(app.directory, "state/orders/checkout-offer.json"), "utf8")), result.order);
  assert.equal(app.settlementRequests.length, 1);
  assert.deepEqual(app.settlementRequests[0].payload.resource, offer.resource);
  assert.deepEqual(app.settlementRequests[0].payload.payload, { transaction: txHash, receiptDigest: proof.receiptDigest });
  assert.deepEqual(app.chainRequests.map((request) => request.method), ["eth_getRawTransactionByHash", "pax_sequencerReceipt"]);
  await app.reopen();
  assert.deepEqual(await app.order("checkout-offer"), result.order);
  const replay = await app.checkout("checkout-offer", payment);
  assert.equal(replay.status, 200, JSON.stringify(await replay.clone().json()));
  assert.deepEqual(decodeSettlementHeader(replay.headers.get("payment-response")), settlement);
  assert.equal(app.settlementRequests.length, 1);
  assert.equal(app.chainRequests.length, 2);
}

async function merchantSettlementDecoderRejectsInvalidResponses(context) {
  const app = await createMerchantRepairFixture(context);
  const cases = [
    { id: "pending", response: { state: "pending" }, status: 202, state: "pending", order: "awaiting-payment" },
    { id: "refused", response: { state: "refused", reason: "not_authorized" }, status: 402, state: "refused", order: "refused" },
    { id: "unknown", response: { state: "unknown" }, status: 503, state: "unknown", error: "settlement_unknown", order: "awaiting-payment" },
    { id: "malformed", response: { state: "settled", transaction: "0x00" }, status: 503, state: "unknown", error: "invalid_settlement_response", order: "awaiting-payment" },
    { id: "chain-result", response: { jsonrpc: "2.0", id: 1, result: "0x7d" }, status: 503, state: "unknown", error: "invalid_application_data", order: "awaiting-payment" },
  ];
  for (const fixture of cases) {
    const quote = await app.checkout(fixture.id);
    assert.equal(quote.status, 402);
    const offer = decodePaymentRequiredHeader(quote.headers.get("payment-required"));
    await quote.arrayBuffer();
    const receipt = await app.produce(offer.accepts[0].extra.layerx.purposeHash);
    app.service.settlementResponse = fixture.response;
    const response = await app.checkout(fixture.id, encodePaymentPayloadHeader({
      x402Version: 2, resource: offer.resource, accepted: offer.accepts[0], payload: { transaction: receipt.txHash },
    }));
    const result = await response.json();
    assert.equal(response.status, fixture.status, JSON.stringify(result));
    assert.equal(result.state, fixture.state);
    assert.equal(result.error, fixture.error);
    assert.equal(response.headers.get("payment-response"), null);
    const order = await app.order(fixture.id);
    assert.equal(order.state, fixture.order);
    assert.equal(order.transaction, undefined);
    assert.equal(order.receiptDigest, undefined);
  }
  assert.equal(app.settlementRequests.length, cases.length);
  assert.equal(app.chainRequests.length, 0, "unsettled and malformed responses must not resolve or pay a transaction");
}

async function merchantSettlementConfiguration(context) {
  bindEnvironment(context, { LAYERX_EXAMPLE_ENDPOINT: undefined, LAYERX_EXAMPLE_CONFIG: undefined,
    [beta.settlementUrlEnvironment]: undefined });
  assert.equal(Object.hasOwn(beta, "settlementUrl"), false);
  assert.equal(beta.settlementUrlEnvironment, "LAYERX_TESTNET_SETTLEMENT_URL");
  assert.equal(beta.tokenEnvironment, "LAYERX_TESTNET_TOKEN");
  assert.throws(() => merchantShopConfiguration(beta), /missing_layerx_testnet_settlement_url/u);
  process.env[beta.settlementUrlEnvironment] = `${beta.rpcUrl}/v1/settle`;
  assert.throws(() => merchantShopConfiguration(beta), /invalid_merchant_settlement_origin/u);
  process.env[beta.settlementUrlEnvironment] = "https://merchant-gateway.example.invalid/v1/settle";
  const config = merchantShopConfiguration(beta);
  assert.equal(config.rpcUrl, beta.rpcUrl);
  assert.equal(config.settlementUrl, process.env[beta.settlementUrlEnvironment]);
  assert.ok(Object.isFrozen(config));
  assert.throws(() => merchantShopConfiguration({ ...beta, rpcUrl: `${beta.rpcUrl}/rpc` }), /invalid_merchant_chain_origin/u);
  assert.throws(() => merchantShopConfiguration({ ...beta, network: "eip155:402" }), /invalid_merchant_chain_origin/u);
  for (const [url, error] of [
    ["http://merchant-gateway.example.invalid/v1/settle", /^insecure_service_url$/u],
    ["https://merchant-gateway.example.invalid/rpc", /^invalid_merchant_settlement_origin$/u],
    ["https://merchant-gateway.example.invalid/v1/settle?token=secret", /^invalid_service_url$/u],
    ["https://user:secret@merchant-gateway.example.invalid/v1/settle", /^invalid_service_url$/u],
  ]) {
    process.env[beta.settlementUrlEnvironment] = url;
    assert.throws(() => merchantShopConfiguration(beta), { message: error });
  }
  process.env[beta.settlementUrlEnvironment] = config.settlementUrl;
  process.env.LAYERX_EXAMPLE_ENDPOINT = "https://merchant-gateway.example.invalid";
  assert.throws(() => merchantShopConfiguration(beta), /use_layerx_testnet_settlement_url_for_merchant_gateway/u);
  const argv = process.argv;
  context.after(() => { process.argv = argv; });
  process.argv = [process.execPath, "merchant-shop", "--environment", "beta"];
  await assert.rejects(loadMerchantShopConfig(new URL("../index.mjs", import.meta.url).href),
    /use_layerx_testnet_settlement_url_for_merchant_gateway/u);
  const emulator = { name: "emulator", ...document.environments.emulator };
  assert.strictEqual(merchantShopConfiguration(emulator), emulator);
}

test(merchantChainOrigin);
test(merchantSettlementOrigin);
test(merchantSettlementDecoderRejectsInvalidResponses);
test(merchantSettlementConfiguration);

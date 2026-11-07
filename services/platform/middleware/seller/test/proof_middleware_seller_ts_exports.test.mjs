import "./source-loader.mjs";
// Proves: conformance/receipts.ts buildSignedReceipt and scenarios.ts ConformancePaymentRpcSession, seller ReceiptPayloadAuthority, verifyPaymentReceipt, verifyPaymentCommitmentEvidence, verifyRpcPayment, PaymentRpc -> @sidiora/layerx-seller-middleware exports -> Express, Next, paid-api, merchant-app, conformance releasedDecision and MerchantMiddleware.
// Repairs: SellerMiddleware.paymentRequired exposed mutable offer state; both example
// fulfillment stores lost bigint/Uint8Array commitment evidence on disk; merchant
// quotes persisted malformed seller terms.
// Production TypeScript sources run through Node's TypeScript transform, without
// replacing any exports, verification, storage, signatures, or HTTP transport.
import assert from "node:assert/strict";
import { once } from "node:events";
import { existsSync, readFileSync } from "node:fs";
import { mkdtemp, readdir, readFile, rm } from "node:fs/promises";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { committedPaymentFixture, refusesUnboundCommitments } from "./repair-22-33-1478-support.mjs";
import { keccak256 } from "@sidiora/layerx-sdk";
import * as seller from "@sidiora/layerx-seller-middleware";

const root = new URL("../../../../../", import.meta.url);
const sourcePackages = new Map([
  ["@sidiora/layerx-buyer-middleware", "services/platform/middleware/buyer/src/index.ts"],
  ["@sidiora/layerx-merchant-middleware", "services/platform/middleware/merchant/src/index.ts"],
]);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (sourcePackages.has(specifier)) {
      return { url: new URL(sourcePackages.get(specifier), root).href, shortCircuit: true };
    }
    if (specifier.startsWith(".") && specifier.endsWith(".js") && context.parentURL?.endsWith(".ts")) {
      const url = new URL(specifier.slice(0, -3) + ".ts", context.parentURL);
      if (existsSync(url)) return { url: url.href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(root.href) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return nextLoad(url, context);
  },
});
const express = await import(new URL("sdk/integrations/express/src/index.ts", root));
const next = await import(new URL("sdk/integrations/next/src/index.ts", root));
const { ConformanceSequencer, buildSignedReceipt, buyerPayload, fixedBytes, offerFixture } =
  await import(new URL("services/platform/middleware/conformance/src/receipts.ts", root));
const { FixedReceiptKeys } = await import(new URL("services/platform/middleware/conformance/src/support.ts", root));
const { ConformancePaymentRpcSession, releasedDecision } = await import(new URL("services/platform/middleware/conformance/src/scenarios.ts", root));
const { FileFulfillmentRepository, createPaidApiServer } =
  await import(new URL("services/platform/examples/paid-api/index.mjs", root));
const { FileOrders, FileFulfillments, DeclaredCatalog, MerchantCheckout } =
  await import(new URL("services/platform/examples/support/merchant-app.mjs", root));
const { MerchantMiddleware, MerchantError } = await import("@sidiora/layerx-merchant-middleware");
const hex = (bytes) => `0x${Buffer.from(bytes).toString("hex")}`;
const refused = (error) => error instanceof seller.MiddlewareError && error.code === "verification-failure";

async function produce(purposeHash) {
  const sequencer = await ConformanceSequencer.generate();
  const resolver = new FixedReceiptKeys([sequencer.receiptKey()]);
  const keys = await resolver.resolve();
  const offer = offerFixture(fixedBytes(0xaa), fixedBytes(0xbb), 250_000n);
  if (purposeHash !== undefined) offer.requirements.extra.layerx.purposeHash = purposeHash;
  const facts = {
    asset: fixedBytes(0xbb), payTo: fixedBytes(0xaa), amount: 250_000n,
    from: fixedBytes(0xcc), batchId: fixedBytes(0xd1), previousStateRoot: fixedBytes(0xd2),
    resultingStateRoot: fixedBytes(0xd3), ...(purposeHash === undefined ? {} : { purposeHash }),
  };
  const receipt = await buildSignedReceipt(sequencer, facts);
  const payload = buyerPayload(offer, receipt, "exports-proof");
  return { sequencer, resolver, keys, offer, facts, receipt, payload };
}


function makeSeller(produced, repository, commitments) {
  return new seller.SellerMiddleware({ paymentRequired: produced.offer.paymentRequired, receiptKeys: produced.keys,
    authority: new seller.ReceiptPayloadAuthority(produced.keys), fulfillments: repository,
    ...(commitments === undefined ? {} : { commitments }) });
}

async function listen(server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${server.address().port}`;
}
async function close(server) {
  const closed = once(server, "close");
  server.close();
  server.closeAllConnections();
  await closed;
}

async function sellerExportsPersistCommittedPaymentsAcrossConsumers(t) {
  const directory = await mkdtemp(join(tmpdir(), "seller-exports-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const recorded = await committedPaymentFixture();
  const produced = { keys: recorded.keys, receipt: recorded.proof,
    offer: { requirements: recorded.requirements, paymentRequired: { x402Version: 2,
      resource: { url: "https://merchant.example/paid" }, accepts: [recorded.requirements] } } };
  const evidence = await recorded.resolver.resolve(recorded.vector.receiptJson.txHash, produced.offer.requirements.network);
  let verified;
  try {
    verified = await seller.verifyPaymentReceipt(produced.receipt, produced.offer.requirements, produced.keys, recorded.resolver);
    assert.deepEqual(verified.commitmentEvidence, evidence);
    assert.deepEqual(verified.canonicalBytes, recorded.verified.canonicalBytes);
    assert.deepEqual(verified.receiptDigest, recorded.verified.receiptDigest);
    await seller.verifyPaymentCommitmentEvidence(recorded.verified, "committed", evidence,
      produced.offer.requirements.network, recorded.resolver);
  } catch (error) {
    const causes = [];
    for (let inner = error; inner instanceof Error; inner = inner.cause) {
      causes.push({ name: inner.name, code: inner.code, check: inner.check, message: inner.message });
    }
    t.diagnostic(JSON.stringify({ committedExportsFailure: causes, innermost: causes.at(-1) }));
    throw error;
  }
  await refusesUnboundCommitments(seller.verifyPaymentCommitmentEvidence);
  const portable = seller.encodePaymentCommitmentEvidence(evidence);
  assert.deepEqual(seller.decodePaymentCommitmentEvidence(JSON.parse(JSON.stringify(portable))), evidence);
  for (const malformed of [null, { ...portable, extra: true }, { ...portable, landingBlockHeight: "0x00" },
    { ...portable, receiptInclusion: { ...portable.receiptInclusion, receiptDigest: "0x00" } }]) {
    assert.throws(() => seller.decodePaymentCommitmentEvidence(malformed), { code: "verification-failure" });
  }
  const badEvidence = structuredClone(evidence);
  badEvidence.commit.signatures[0].signature[0] ^= 1;
  await assert.rejects(seller.verifyPaymentCommitmentEvidence(recorded.verified, "committed", badEvidence, "eip155:125", recorded.resolver), { code: "verification-failure" });
  produced.receipt.verificationLevel = "committed";
  produced.payload = buyerPayload(produced.offer, produced.receipt, "exports-proof");
  const commitments = { ...recorded.resolver, resolve: async (hash, network) => {
    assert.equal(hash, verified.txHash); assert.equal(network, produced.offer.requirements.network);
    return structuredClone(evidence);
  } };
  const header = seller.encodePaymentPayloadHeader(produced.payload);
  const tampered = structuredClone(produced.payload);
  tampered.payload.receiptDigest = `0x${"00".repeat(32)}`;
  const badHeader = seller.encodePaymentPayloadHeader(tampered);
  for (const adapter of [express, next]) {
    assert.equal(adapter.SellerMiddleware, seller.SellerMiddleware);
    assert.equal(adapter.ReceiptPayloadAuthority, seller.ReceiptPayloadAuthority);
    assert.equal(adapter.verifyPaymentReceipt, seller.verifyPaymentReceipt);
    assert.equal(adapter.verifyPaymentCommitmentEvidence, seller.verifyPaymentCommitmentEvidence);
    assert.equal(adapter.verifyRpcPayment, seller.verifyRpcPayment);
    assert.equal(adapter.PaymentRpc, seller.PaymentRpc);
  }
  for (const [name, Repository] of [["paid-api", FileFulfillmentRepository], ["merchant-app", FileFulfillments]]) {
    const state = join(directory, name);
    let repository = new Repository(state);
    let middleware = makeSeller(produced, repository, commitments);
    const offered = middleware.paymentRequired();
    offered.body.accepts[0].amount = "1";
    assert.equal(middleware.paymentRequired().body.accepts[0].amount, recorded.payment.amount.toString());
    let releases = 0;
    const resource = { contentType: "application/json", body: '{"report":"paid"}' };
    const first = await middleware.handle("public-paid-api", header, async () => { releases += 1; return resource; });
    assert.equal(first.kind, "released");
    assert.equal(first.verification.rung, "committed");
    const [file] = await readdir(state);
    const key = file.slice(0, -5);
    const persisted = await repository.get(key);
    const diskRecord = JSON.parse(await readFile(join(state, file), "utf8"));
    assert.deepEqual(diskRecord.commitmentEvidence, portable);
    assert.deepEqual(persisted.commitmentEvidence, evidence);
    // Every file operation closes its descriptor. Drop both objects and reopen
    // the actual on-disk repository; recovery keeps the independently trusted validator anchor.
    middleware = undefined;
    repository = undefined;
    const reopened = new Repository(state);
    assert.deepEqual(await reopened.get(key), persisted);
    const recoveredSeller = makeSeller(produced, reopened, {
      resolveValidators: (...args) => recorded.resolver.resolveValidators(...args),
      async resolve() { assert.fail("recovery must use persisted inclusion evidence"); },
    });
    const repeated = await recoveredSeller.handle("public-paid-api", header, async () => { releases += 1; return resource; });
    assert.deepEqual(repeated, first);
    assert.equal(releases, 1);
    const runtime = { seller: recoveredSeller, principal: "public-paid-api", requirements: produced.offer.requirements,
      resources: { release: async () => { releases += 1; return resource; } } };
    await express.assertReceiptBacked(repeated, runtime.requirements);
    await next.assertReceiptBacked(repeated, runtime.requirements);
    const handler = express.layerXPaymentGate(runtime);
    // Real Express request/response objects carry the adapter over loopback HTTP.
    const { default: expressFramework } = await import("express");
    const app = expressFramework();
    app.get("/paid", handler);
    let server = app.listen(0, "127.0.0.1");
    await once(server, "listening");
    try {
      for (const [payment, status] of [[header, 200], [badHeader, 402]]) {
        const response = await fetch(`http://127.0.0.1:${server.address().port}/paid`, { headers: { "PAYMENT-SIGNATURE": payment } });
        assert.equal(response.status, status);
        if (status === 200) {
          assert.equal(await response.text(), resource.body);
          assert.equal(seller.decodeSettlementHeader(response.headers.get("PAYMENT-RESPONSE")).transaction, verified.txHash);
        } else assert.equal((await response.json()).error, "verification-failure");
      }
    } finally { await close(server); }
    // Next's real mount receives a replay and refuses tampered evidence.
    const receiptedRequirements = { ...recorded.requirements,
      extra: { layerx: { ...recorded.requirements.extra.layerx, commitment: "receipted" } } };
    const receiptedProduced = { ...produced, receipt: { ...produced.receipt, verificationLevel: "receipted" },
      offer: { requirements: receiptedRequirements, paymentRequired: { ...produced.offer.paymentRequired, accepts: [receiptedRequirements] } } };
    const environment = environmentFor(receiptedProduced);
    const mount = next.mountLayerX({ environment, resources: { release: async () => resource },
      fulfillments: new Repository(join(directory, `${name}-next`)),
      deliveries: new next.SingleProcessWebhookDeliveryStore(), events: { handle: async () => {} } });
    try {
      const offer = mount.seller.paymentRequired();
      assert.equal(offer.status, 402);
      const mountedOffer = { paymentRequired: offer.body, requirements: offer.body.accepts[0] };
      const receiptedPayload = buyerPayload(mountedOffer, receiptedProduced.receipt, "next-receipted-proof");
      const receiptedHeader = seller.encodePaymentPayloadHeader(receiptedPayload);
      const receiptedTampered = structuredClone(receiptedPayload);
      receiptedTampered.payload.receiptDigest = `0x${"00".repeat(32)}`;
      const receiptedBadHeader = seller.encodePaymentPayloadHeader(receiptedTampered);
      const challenge = await mount.resource.GET(new Request(environment.LAYERX_RESOURCE_URL));
      assert.equal(challenge.status, 402);
      assert.deepEqual(seller.decodePaymentRequiredHeader(challenge.headers.get("PAYMENT-REQUIRED")), offer.body);
      const committedRefusal = await mount.resource.GET(new Request(environment.LAYERX_RESOURCE_URL,
        { headers: { "PAYMENT-SIGNATURE": header } }));
      assert.equal(committedRefusal.status, 402);
      assert.equal((await committedRefusal.json()).error, "requirements-mismatch");
      await challenge.text();
      const paid = await mount.resource.GET(new Request(environment.LAYERX_RESOURCE_URL, { headers: { "PAYMENT-SIGNATURE": receiptedHeader } }));
      assert.equal(paid.status, 200);
      assert.equal(await paid.text(), resource.body);
      assert.equal(seller.decodeSettlementHeader(paid.headers.get("PAYMENT-RESPONSE")).transaction, verified.txHash);
      const malformed = await mount.resource.GET(new Request(environment.LAYERX_RESOURCE_URL, { headers: { "PAYMENT-SIGNATURE": receiptedBadHeader } }));
      assert.equal(malformed.status, 402);
      assert.equal((await malformed.json()).error, "verification-failure");
    } finally { mount.destroy(); }
    // Paid API uses the same recovered seller over a fresh HTTP session.
    server = createPaidApiServer(recoveredSeller, { report: "paid" });
    const base = await listen(server);
    try {
      const paid = await fetch(`${base}/paid`, { headers: { "PAYMENT-SIGNATURE": header } });
      assert.equal(paid.status, 200);
      assert.deepEqual(await paid.json(), resource);
      const refusedResponse = await fetch(`${base}/paid`, { headers: { "PAYMENT-SIGNATURE": badHeader } });
      assert.equal(refusedResponse.status, 400);
      assert.equal((await refusedResponse.json()).state, "refused");
      const rpc = new seller.PaymentRpc(`${base}/`);
      await assert.rejects(rpc.receipt(verified.txHash), /rpc-http-404/);
      await assert.rejects(rpc.send("0x02"));
      assert.throws(() => new seller.PaymentRpc(`${base}/paid`), /invalid-rpc-endpoint/);
    } finally { await close(server); }
    assert.equal(releases, 1);
  }
  // The conformance consumer uses its own exported helper and disk-backed state.
  const receipted = await produce();
  const conformanceDirectory = join(directory, "conformance");
  const first = await releasedDecision(receipted.offer, receipted.payload, receipted.resolver,
    new FileFulfillmentRepository(conformanceDirectory));
  const replay = await releasedDecision(receipted.offer, receipted.payload, receipted.resolver,
    new FileFulfillmentRepository(conformanceDirectory));
  assert.equal(first.resource, "the-paid-resource");
  assert.deepEqual(replay, first);
  const invalid = structuredClone(receipted.payload);
  invalid.payload.receiptDigest = `0x${"00".repeat(32)}`;
  await assert.rejects(releasedDecision(receipted.offer, invalid, receipted.resolver,
    new FileFulfillmentRepository(conformanceDirectory)), refused);
}

function environmentFor(produced) {
  return { LAYERX_PRINCIPAL: "public-paid-api", LAYERX_PROTECTED_PATH: "/paid",
    LAYERX_RESOURCE_URL: produced.offer.paymentRequired.resource.url,
    LAYERX_PROTOCOL_VERSION: "2", LAYERX_X402_SCHEME: "exact", LAYERX_X402_NETWORK: "eip155:125",
    LAYERX_PRICE: produced.offer.requirements.amount, LAYERX_ASSET: produced.offer.requirements.asset,
    LAYERX_PAY_TO: produced.offer.requirements.payTo, LAYERX_PAYMENT_TIMEOUT_SECONDS: "120",
    LAYERX_PAYMENT_PURPOSE_HASH: produced.offer.requirements.extra.layerx.purposeHash,
    LAYERX_RECEIPT_KEYS_JSON: JSON.stringify(produced.keys.map((key) => ({ keyId: hex(key.keyId), publicKey: hex(key.publicKey), firstHeight: key.first_height.toString(), lastHeight: key.last_height.toString() }))),
    LAYERX_WEBHOOK_PATH: "/webhook", LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ sequencer: Buffer.from(produced.keys[0].publicKey).toString("base64") }) };
}

async function merchantConsumerRecoversPaidOrdersAndRefusesUnknownSku(t) {
  const directory = await mkdtemp(join(tmpdir(), "merchant-exports-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const resourceUrl = "https://merchant.example/orders/proof";
  const purpose = hex(keccak256(new TextEncoder().encode(resourceUrl))).slice(2);
  const produced = await produce(purpose);
  const item = { sku: "report", title: "Paid report", unitAmount: produced.offer.requirements.amount,
    asset: produced.offer.requirements.asset, payTo: produced.offer.requirements.payTo,
    scheme: "exact", network: "eip155:125", maxTimeoutSeconds: 120, extra: produced.offer.requirements.extra };
  const sellers = { create: (paymentRequired) => new seller.SellerMiddleware({ paymentRequired, receiptKeys: produced.keys,
    authority: new seller.ReceiptPayloadAuthority(produced.keys), fulfillments: new FileFulfillments(join(directory, "fulfillments")) }) };
  const create = () => new MerchantCheckout({ catalog: new DeclaredCatalog(item), orders: new FileOrders(join(directory, "orders")),
    sellers, resourceUrl: () => resourceUrl });
  let merchant = create();
  assert.ok(merchant instanceof MerchantMiddleware);
  const lines = [{ sku: "report", quantity: 1 }];
  const checkout = await merchant.checkout("merchant", "proof", lines);
  assert.equal(checkout.kind, "payment-required");
  const payload = buyerPayload({ paymentRequired: checkout.order.quote.paymentRequired,
    requirements: checkout.order.quote.paymentRequired.accepts[0] }, produced.receipt, "merchant-proof");
  const header = seller.encodePaymentPayloadHeader(payload);
  const paid = await merchant.checkout("merchant", "proof", lines, header);
  assert.equal(paid.kind, "paid");
  assert.equal(paid.order.state, "paid-verified");
  assert.equal(paid.settlement.transaction, produced.receipt.txHash);
  merchant = undefined;
  merchant = create();
  const replay = await merchant.checkout("merchant", "proof", lines, header);
  assert.deepEqual(replay, paid);
  assert.deepEqual(await new FileOrders(join(directory, "orders")).get("proof"), paid.order);
  assert.equal((await readdir(join(directory, "fulfillments"))).length, 1);
  await assert.rejects(merchant.checkout("merchant", "unknown", [{ sku: "missing", quantity: 1 }]),
    (error) => error instanceof MerchantError && error.code === "catalog-item-missing");
  const bad = structuredClone(payload);
  bad.payload.receiptDigest = `0x${"00".repeat(32)}`;
  await assert.rejects(merchant.checkout("merchant", "proof", lines, seller.encodePaymentPayloadHeader(bad)), refused);
  assert.deepEqual(await new FileOrders(join(directory, "orders")).get("proof"), paid.order);
  const conflicting = buyerPayload({ paymentRequired: checkout.order.quote.paymentRequired,
    requirements: checkout.order.quote.paymentRequired.accepts[0] }, produced.receipt, "merchant-conflicting-replay");
  assert.notDeepEqual(conflicting, payload);
  assert.deepEqual(conflicting.payload, payload.payload);
  const refusedReplay = await merchant.checkout("merchant", "proof", lines, seller.encodePaymentPayloadHeader(conflicting));
  assert.equal(refusedReplay.kind, "refused");
  assert.deepEqual(refusedReplay.order, paid.order);
  assert.deepEqual(await new FileOrders(join(directory, "orders")).get("proof"), paid.order);
  const invalidCatalog = new MerchantMiddleware({ catalog: new DeclaredCatalog({ ...item, extra: { layerx: { commitment: "retired" } } }),
    orders: new FileOrders(join(directory, "invalid-orders")), sellers, resourceUrl: () => resourceUrl });
  await assert.rejects(invalidCatalog.checkout("merchant", "invalid", lines), { code: "invalid-payment-required" });
}

async function rpcProducerEvidenceCrossesThePublicNextExports() {
  const requests = [];
  const produced = await produce();
  const terms = produced.offer.requirements;
  const session = new ConformancePaymentRpcSession(produced.sequencer, produced.facts, terms);
  const original = await seller.verifyPaymentReceipt(produced.receipt, terms, produced.keys);
  const payer = hex(original.receipt.from).slice(2);
  // Only HTTP framing lives here. The repository's session issues and verifies
  // the signed receipt on submission; PaymentRpc performs the actual fetch.
  const server = createServer(async (request, response) => {
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/");
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const call = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      requests.push({ url: request.url, call });
      assert.equal(call.jsonrpc, "2.0");
      const result = await session.call(call.method, call.params);
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: call.id, result }));
    } catch (error) {
      response.writeHead(500).end(String(error));
    }
  });
  const base = await listen(server);
  try {
    const rpc = new next.PaymentRpc(`${base}/`);
    assert.doesNotThrow(() => new next.PaymentRpc("https://node.example/"));
    assert.throws(() => new next.PaymentRpc(`${base}/rpc`), /invalid-rpc-endpoint/u);
    assert.throws(() => rpc.call("px_resolveAccount", []), /invalid-node-method/u);
    assert.equal(await rpc.receipt(original.txHash), null);
    for (let i = 0; i < 2; i += 1) {
      assert.equal(await rpc.send(produced.receipt.rawTransaction), original.txHash);
      const result = await rpc.receipt(original.txHash);
      assert.ok(result, "conformance/src/scenarios.ts ConformancePaymentRpcSession must issue a receipt");
      assert.equal(result.receipt, produced.receipt.signedReceipt);
      const proof = await next.verifyRpcPayment(result, produced.receipt.rawTransaction, payer, terms, produced.keys);
      assert.equal(proof.kind, "verified");
      const verified = await express.verifyPaymentReceipt(proof, terms, produced.keys);
      assert.equal(verified.txHash, original.txHash);
      assert.equal(verified.payment.amount.toString(), terms.amount);
      await assert.rejects(next.verifyRpcPayment(result, produced.receipt.rawTransaction, "00".repeat(32), terms, produced.keys), refused);
      assert.equal(session.issuedCount, 1);
    }
    assert.equal(await rpc.receipt(`0x${"00".repeat(32)}`), null);
    await assert.rejects(rpc.send("0x02"));
    const other = await produce();
    await assert.rejects(rpc.send(other.receipt.rawTransaction), /rpc-http-500/);
    assert.equal(session.issuedCount, 1);
    for (const [index, request] of requests.entries()) {
      assert.equal(request.url, "/");
      assert.equal(request.call.id, index + 1);
      assert.equal(request.call.params.length, 1);
      assert.match(request.call.method, /^(?:eth_sendRawTransaction|pax_sequencerReceipt)$/u);
    }
    const hostedRequests = [];
    const gatewayServer = createServer(async (request, response) => {
      const chunks = []; for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      hostedRequests.push({ url: request.url, authorization: request.headers.authorization, body });
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result: { account: "agent:merchant:main" } }));
    });
    const gatewayBase = await listen(gatewayServer);
    try {
      const gateway = new seller.HostedGatewayRpc(`${gatewayBase}/rpc`, { authorization: "Bearer hosted-fixture" });
      assert.notEqual(new URL(gatewayBase).origin, new URL(base).origin);
      assert.deepEqual(await gateway.call("px_resolveAccount", ["merchant"]), { account: "agent:merchant:main" });
      assert.deepEqual(hostedRequests, [{ url: "/rpc", authorization: "Bearer hosted-fixture",
        body: { jsonrpc: "2.0", id: 1, method: "px_resolveAccount", params: ["merchant"] } }]);
      assert.throws(() => gateway.call("eth_sendRawTransaction", []), /invalid-gateway-method/u);
      assert.equal(hostedRequests.length, 1);
    } finally { await close(gatewayServer); }
  } finally { await close(server); }
}

test(sellerExportsPersistCommittedPaymentsAcrossConsumers);
test(merchantConsumerRecoversPaidOrdersAndRefusesUnknownSku);
test(rpcProducerEvidenceCrossesThePublicNextExports);

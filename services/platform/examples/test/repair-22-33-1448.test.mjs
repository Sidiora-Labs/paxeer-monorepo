import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

const root = new URL("../../../../", import.meta.url);
const sources = new Map([
  ["@sidiora/layerx-sdk", new URL("services/agent/sdk/typescript/src/index.ts", root).href],
  ["@sidiora/layerx-buyer-middleware", new URL("services/platform/middleware/buyer/src/index.ts", root).href],
  ["@sidiora/layerx-seller-middleware", new URL("services/platform/middleware/seller/src/index.ts", root).href],
]);
const sourceRoots = [...sources.values()].map((url) => new URL("./", url).href);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (sources.has(specifier)) return { url: sources.get(specifier), shortCircuit: true };
    if (sourceRoots.some((url) => context.parentURL?.startsWith(url))) {
      if (/^\.\.?\//u.test(specifier) && specifier.endsWith(".js")) {
        return { url: new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL).href, shortCircuit: true };
      }
      if (/^(?:@noble\/|@scure\/|ws$)/u.test(specifier)) {
        return nextResolve(specifier, { ...context, parentURL: import.meta.url });
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (sourceRoots.some((base) => url.startsWith(base)) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return nextLoad(url, context);
  },
});

const { BuyerMiddleware, LayerXPaymentHttpTransport } = await import("@sidiora/layerx-buyer-middleware");
const { ProductionClient, SecretBytes, verifySequencerReceipt } = await import("@sidiora/layerx-sdk");
const { decodePaymentExactLog, decodePaymentPayloadHeader, decodeSettlementHeader,
  PAYMENT_SIGNATURE_HEADER, PAYMENT_RESPONSE_HEADER, ReceiptPayloadAuthority, SellerMiddleware }
  = await import("@sidiora/layerx-seller-middleware");
const { paidMerchantOutcome, merchantCheckoutOutcome } = await import("../run-reference-apps.mjs");
const { qualifyReferenceAppOutcomes } = await import("../qualify-reference-apps.mjs");
const { SequencerReceiptClient } = await import("../support/runtime.mjs");
const { FileFulfillmentRepository } = await import("../paid-api/index.mjs");
const { programReceiptOutcome } = await import("../marketplace/receipt-outcome.mjs");

const vectors = JSON.parse(readFileSync(new URL("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", root), "utf8"));
const transcript = JSON.parse(readFileSync(new URL("../buyer-agent/test/fixtures/human-transcript.json", import.meta.url), "utf8"));
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const vectorFor = (id) => {
  assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
  const vector = vectors.cases.find((entry) => entry.id === id);
  assert.ok(vector, `missing producer transcript ${id}`);
  return vector;
};
const pinFor = (vector) => ({
  keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
});

async function merchantWalkthrough(t, receiptId = "V1-layerxledger-pay-exact") {
  const directory = await mkdtemp(join(tmpdir(), "merchant-result-code-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const vector = vectorFor(receiptId);
  const success = vectorFor("V1-layerxledger-pay-exact");
  const log = success.receiptJson.logs[0];
  const payment = decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
  const pin = pinFor(vector);
  const checkoutKey = "reference-merchant-result-code";
  const requestBody = { principal: "reference-buyer", checkout_key: checkoutKey,
    lines: [{ sku: "metered-report", quantity: 1 }] };
  const service = { requests: [], failures: [], releases: [], proof: undefined };
  const account = "agent:did:layerx:payee:main";
  const source = "acct:reference-buyer";
  const bearer = "loopback-reference-human-token";
  let seller;
  let receipts;
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = chunks.length === 0 ? undefined : JSON.parse(Buffer.concat(chunks).toString("utf8"));
      service.requests.push({ path: request.url, method: request.method, body });
      assert.equal(request.headers.accept, request.url === "/checkout" ? "*/*" : "application/json");
      assert.equal(request.headers.authorization, request.url.startsWith("/v1/") ? `Bearer ${bearer}` : undefined);
      if (request.method === "POST") assert.equal(request.headers["content-type"], "application/json");
      if (request.url === "/checkout") {
        assert.equal(request.method, "POST");
        assert.deepEqual(body, requestBody);
        const header = request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()];
        if (header !== undefined) {
          const payload = decodePaymentPayloadHeader(header);
          assert.deepEqual(payload.payload, service.proof);
          assert.deepEqual(payload.accepted, seller.paymentRequired().body.accepts[0]);
          assert.deepEqual(payload.resource, { url: `${service.url}/checkout` });
        }
        const decision = await seller.handle(body.principal, header, async (key) => {
          service.releases.push(key);
          return { state: "paid", order: { orderId: checkoutKey, state: "paid-verified",
            receiptDigest: service.proof.receiptDigest.slice(2) } };
        });
        service.decision = decision;
        response.writeHead(decision.status, { "content-type": "application/json", ...decision.headers });
        response.end(JSON.stringify(decision.kind === "released" ? decision.resource : decision.body));
        return;
      }
      if (request.url === "/rpc") {
        assert.equal(request.method, "POST");
        assert.deepEqual(body, { jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [vector.receiptJson.txHash] });
        response.writeHead(200, { "content-type": "application/json" });
        response.end(JSON.stringify({ jsonrpc: "2.0", id: 1, result: vector.receiptJson }));
        return;
      }
      let exchange;
      if (request.url === "/v1/moves/quote") {
        assert.equal(request.method, "POST");
        assert.deepEqual(body, { source, destination: account, money: { amount: payment.amount.toString(), currency: "LXP" } });
        assert.equal(request.headers["idempotency-key"], undefined);
        exchange = transcript["move.quote"];
      } else if (request.url === "/v1/moves") {
        assert.equal(request.method, "POST");
        assert.deepEqual(body, { quote_id: transcript["move.quote"].body.result.quote_id });
        assert.equal(request.headers["idempotency-key"], `${checkoutKey}-payment`);
        exchange = transcript["move.commit"];
      } else {
        assert.equal(request.method, "GET");
        assert.equal(request.url, `/v1/evidence/${transcript["evidence.get"].body.result.evidence_id}`);
        assert.equal(body, undefined);
        service.proof = await receipts.resolve(vector.rawTransaction);
        exchange = structuredClone(transcript["evidence.get"]);
        exchange.body.result.bytes_base64 = Buffer.from(JSON.stringify(service.proof)).toString("base64");
      }
      response.writeHead(exchange.status, { "content-type": "application/json" });
      response.end(JSON.stringify(exchange.body));
    } catch (error) {
      service.failures.push(error);
      response.writeHead(500).end();
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    server.closeAllConnections();
    await new Promise((accept, reject) => server.close((error) => error ? reject(error) : accept()));
    assert.deepEqual(service.failures, []);
  });
  service.url = `http://127.0.0.1:${server.address().port}`;
  receipts = new SequencerReceiptClient(`${service.url}/rpc`, [pin]);
  seller = new SellerMiddleware({
    receiptKeys: [pin], authority: new ReceiptPayloadAuthority([pin]),
    fulfillments: new FileFulfillmentRepository(join(directory, "fulfillments")),
    paymentRequired: { x402Version: 2, resource: { url: `${service.url}/checkout` }, accepts: [{
      scheme: "exact", network: "eip155:125", amount: payment.amount.toString(), asset: hex(payment.assetId),
      payTo: hex(payment.to), maxTimeoutSeconds: 120,
      extra: { layerx: { commitment: "receipted", account, currency: "LXP",
        payer: hex(payment.from).slice(2), purposeHash: hex(payment.paymentReference).slice(2) } },
    }] },
  });
  const token = new SecretBytes(new TextEncoder().encode(bearer));
  t.after(() => token.destroy());
  const buyer = new BuyerMiddleware({
    client: new ProductionClient(new LayerXPaymentHttpTransport({ baseUrl: service.url, bearerToken: token })),
    source, supported: [{ scheme: "exact", network: "eip155:125" }], receiptKeys: [pin],
    retry: { maximumQuoteAttempts: 1, maximumCommitAttempts: 1 },
  });
  return { service, vector, directory, checkoutKey,
    fetch: () => buyer.fetch(`${service.url}/checkout`, {
      method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(requestBody),
    }, `${checkoutKey}-payment`) };
}

async function companionOutcomes(merchant) {
  const resource = await readFile(new URL("../paid-api/resource.json", import.meta.url), "utf8");
  const verifiedOutcome = async (id) => {
    const vector = vectorFor(id);
    const verified = await verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), [pinFor(vector)]);
    return programReceiptOutcome(vector.receiptJson, verified, "receipted");
  };
  const buyer = await verifiedOutcome("V1-pax-transfer");
  const marketplace = await Promise.all([
    "V1-erc20-approve", "V1-layerxbridge-bridge-out", "V1-layerxprograms-call",
  ].map(async (id, index) => ({
    ...await verifiedOutcome(id), application: "marketplace", environment: merchant.environment,
    action: ["deploy", "list", "buy"][index],
  })));
  return [
    { application: "paid-api", environment: merchant.environment, path: "/paid", listening: 8080 },
    { ...buyer, application: "buyer-agent", environment: merchant.environment, state: "paid", status: 200, body: resource },
    merchant, ...marketplace,
    { environment: merchant.environment, state: "completed", applications: ["buyer-agent", "paid-api", "merchant-shop", "marketplace"] },
  ];
}

async function merchantResultCodeReachesQualifier(t) {
  const run = await merchantWalkthrough(t);
  const result = await run.fetch();
  assert.equal(result.kind, "paid");
  assert.equal(result.response.status, 200);
  const settlement = decodeSettlementHeader(result.response.headers.get(PAYMENT_RESPONSE_HEADER));
  assert.deepEqual(settlement, run.service.decision.settlement);
  assert.equal(settlement.success, true);
  assert.equal(settlement.transaction, run.vector.receiptJson.txHash);
  assert.deepEqual(settlement.extensions.layerx, run.service.proof);
  assert.equal(result.settlement.verification.receipt.resultCode, 0);
  assert.deepEqual(result.settlement.verification, run.service.decision.verification);
  const outcome = await paidMerchantOutcome(result, "emulator", run.checkoutKey);
  assert.deepEqual(outcome, {
    application: "merchant-shop", environment: "emulator", state: "paid", orderId: run.checkoutKey,
    receiptDigest: run.service.proof.receiptDigest.slice(2), rung: "receipted",
    transactionHash: run.vector.receiptJson.txHash, resultCode: result.settlement.verification.receipt.resultCode,
  });
  assert.equal(outcome.resultCode, 0);
  assert.equal(run.service.releases.length, 1);
  assert.deepEqual(run.service.requests.map(({ path }) => path), [
    "/checkout", "/v1/moves/quote", "/v1/moves",
    `/v1/evidence/${transcript["evidence.get"].body.result.evidence_id}`, "/rpc", "/checkout",
  ]);
  const outcomesFile = join(run.directory, "outcomes.json");
  for (const environment of ["emulator", "beta"]) {
    const merchant = { ...outcome, environment };
    const rows = await companionOutcomes(merchant);
    await writeFile(outcomesFile, JSON.stringify(rows));
    assert.deepEqual((await qualifyReferenceAppOutcomes(outcomesFile, environment)).merchant, merchant);
    for (const resultCode of [undefined, 1, "0", null, false]) {
      const invalid = structuredClone(rows);
      invalid[2].resultCode = resultCode;
      await writeFile(outcomesFile, JSON.stringify(invalid));
      await assert.rejects(qualifyReferenceAppOutcomes(outcomesFile, environment), {
        code: "ERR_ASSERTION", message: /merchant checkout result code must indicate success/u,
      });
    }
    const legacy = structuredClone(rows);
    delete legacy[2].resultCode;
    legacy[2].status = 1;
    await writeFile(outcomesFile, JSON.stringify(legacy));
    await assert.rejects(qualifyReferenceAppOutcomes(outcomesFile, environment), {
      code: "ERR_ASSERTION", message: /merchant checkout result code must indicate success/u,
    });
  }
}

async function merchantResultCodePreservesVerifiedFailure(t) {
  const run = await merchantWalkthrough(t, "V2-layerxledger-revert");
  await assert.rejects(run.fetch(), (error) => error.code === "verification-failure");
  assert.equal(run.service.releases.length, 0);
  assert.equal(run.service.requests.filter(({ path }) => path === "/checkout").length, 1);
  const verified = await verifySequencerReceipt(bytes(run.vector.signedReceipt), bytes(run.vector.rawTransaction), [pinFor(run.vector)]);
  assert.equal(verified.receipt.resultCode, 1);
  const outcome = merchantCheckoutOutcome("emulator", run.checkoutKey, run.service.proof.receiptDigest.slice(2), {
    txHash: run.vector.receiptJson.txHash, verification: { ...verified, rung: "receipted" },
  });
  assert.equal(outcome.resultCode, verified.receipt.resultCode);
  assert.equal(outcome.resultCode, 1);
  const rows = await companionOutcomes(outcome);
  const outcomesFile = join(run.directory, "failed-outcomes.json");
  await writeFile(outcomesFile, JSON.stringify(rows));
  await assert.rejects(qualifyReferenceAppOutcomes(outcomesFile, "emulator"), {
    code: "ERR_ASSERTION", message: /merchant checkout result code must indicate success/u,
  });
}

test(merchantResultCodeReachesQualifier);
test(merchantResultCodePreservesVerifiedFailure);

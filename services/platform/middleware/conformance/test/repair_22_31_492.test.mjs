import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { ProductionClient, SecretBytes, keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  resolveDaemonReceipt,
  verifyAgentPayment,
  verifyCommittedAgentPayment,
} from "@sidiora/layerx-agent-middleware";
import { BuyerMiddleware, LayerXPaymentHttpTransport } from "@sidiora/layerx-buyer-middleware";
import {
  DurableFulfillmentRepository,
  MiddlewareError,
  PAYMENT_REQUIRED_HEADER,
  PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER,
  ReceiptPayloadAuthority,
  SellerMiddleware,
  decodePaymentPayloadHeader,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
  encodeSettlementHeader,
  verifyRpcPayment,
} from "@sidiora/layerx-seller-middleware";
import { agentSpend, bytes, exact, hex, keys, offer } from "../pay6/node-fixtures.mjs";
import { ConformanceSequencer, buildSignedReceipt, fixedBytes, offerFixture } from "../dist/receipts.js";
import { ConformancePaymentRpcSession } from "../dist/scenarios.js";
import { FixedReceiptKeys } from "../dist/support.js";

const golden = (operation) => JSON.parse(readFileSync(new URL(
  `../../../../human/schema/human-api/golden/${operation}.response.json`, import.meta.url,
), "utf8")).body.result;

async function recordedAgentEvidence() {
  // Replay the committed Go producer's exact bytes through the production daemon adapter.
  const submission = await resolveDaemonReceipt({
    submission_ref: exact.receiptJson.txHash.slice(2),
    tx_hash: exact.receiptJson.txHash.slice(2),
    state: "Receipted",
    signed_receipt: exact.signedReceipt.slice(2),
  }, bytes(exact.rawTransaction), keys);
  assert.ok(submission.portableReceipt);
  return submission.portableReceipt;
}

async function receiptPeer(context, evidence) {
  const directory = await mkdtemp(join(tmpdir(), "receipt-digest-492-"));
  const requests = [];
  const failures = [];
  const sellerErrors = [];
  let releases = 0;
  let seller;
  let required;
  const quote = golden("move.quote");
  quote.money.amount = offer.amount;
  const material = golden("evidence.get");
  material.bytes_base64 = Buffer.from(JSON.stringify(evidence)).toString("base64");
  const reference = { evidence_id: material.evidence_id, class: material.class, verification: material.verification };
  const journey = golden("move.commit");
  journey.evidence = [reference];
  journey.stages = [{ ...journey.stages[0], evidence: [reference] }];
  const server = createServer(async (request, response) => {
    try {
      requests.push(`${request.method} ${request.url}`);
      if (request.url === "/paid") {
        assert.equal(request.method, "GET");
        try {
          const decision = await seller.handle("receipt-digest-payer", request.headers["payment-signature"], async () => {
            releases += 1;
            return "paid receipt resource";
          });
          response.writeHead(decision.status, decision.headers);
          response.end(decision.kind === "released" ? decision.resource : JSON.stringify(decision.body));
        } catch (error) {
          if (!(error instanceof MiddlewareError)) throw error;
          sellerErrors.push(error);
          response.writeHead(400, { "content-type": "application/json" });
          response.end(JSON.stringify({ code: error.code }));
        }
        return;
      }
      // Replay the human API wire contract over HTTP; the receipt payload comes from the adapter above.
      assert.equal(request.headers.authorization, "Bearer receipt-digest-recording");
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = chunks.length === 0 ? undefined : JSON.parse(Buffer.concat(chunks).toString("utf8"));
      let result;
      if (request.url === "/v1/moves/quote") {
        assert.equal(request.method, "POST");
        assert.deepEqual(body, { source: "acct:receipt-digest-buyer", destination: "agent:did:layerx:payee:main",
          money: { amount: offer.amount, currency: "LXP" } });
        result = quote;
      } else if (request.url === "/v1/moves") {
        assert.equal(request.method, "POST");
        assert.equal(request.headers["idempotency-key"], "receipt-digest-492");
        assert.deepEqual(body, { quote_id: quote.quote_id });
        result = journey;
      } else {
        assert.equal(request.method, "GET");
        assert.equal(request.url, `/v1/evidence/${material.evidence_id}`);
        result = material;
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ ok: true, result }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500).end(String(error));
    }
  });
  context.after(async () => {
    await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); });
    await rm(directory, { recursive: true, force: true });
    assert.deepEqual(failures, []);
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const endpoint = `http://127.0.0.1:${server.address().port}/`;
  const requirements = { ...offer, extra: { layerx: {
    ...offer.extra.layerx, account: "agent:did:layerx:payee:main", currency: "LXP",
  } } };
  required = { x402Version: 2, resource: { url: `${endpoint}paid` }, accepts: [requirements] };
  const reopen = () => {
    seller = new SellerMiddleware({ paymentRequired: required, receiptKeys: keys,
      authority: new ReceiptPayloadAuthority(keys), fulfillments: new DurableFulfillmentRepository(directory) });
  };
  reopen();
  const buyer = new BuyerMiddleware({
    client: new ProductionClient(new LayerXPaymentHttpTransport({ baseUrl: endpoint,
      bearerToken: new SecretBytes(new TextEncoder().encode("receipt-digest-recording")) })),
    source: "acct:receipt-digest-buyer", supported: [{ scheme: "exact", network: "eip155:125" }],
    receiptKeys: keys, now: () => Date.parse("2026-08-18T09:32:00Z"),
  });
  return { buyer, required, requests, sellerErrors, reopen, get releases() { return releases; },
    send: (header) => fetch(required.resource.url, { headers: { [PAYMENT_SIGNATURE_HEADER]: header } }) };
}

async function buyerReceiptSignatureDigestRoundTrip(context) {
  const evidence = await recordedAgentEvidence();
  const peer = await receiptPeer(context, evidence);
  const result = await peer.buyer.fetch(peer.required.resource.url, {}, "receipt-digest-492");
  assert.equal(result.kind, "paid");
  assert.equal(await result.response.text(), "paid receipt resource");
  const payload = decodePaymentPayloadHeader(result.payment.paymentHeader);
  assert.equal(payload.payload.receiptDigest, exact.receiptDigest);
  assert.equal(result.payment.receiptDigest, exact.receiptDigest);
  assert.equal(hex(result.payment.verification.receiptDigest), exact.receiptDigest);
  assert.equal(result.settlement.receiptDigest, exact.receiptDigest);
  assert.equal(result.settlement.response.transaction, exact.receiptJson.txHash);
  assert.equal(result.settlement.signedReceipt, exact.signedReceipt);
  assert.equal(result.settlement.rawTransaction, exact.rawTransaction);
  assert.equal(peer.releases, 1);
  assert.deepEqual(peer.requests, ["GET /paid", "POST /v1/moves/quote", "POST /v1/moves",
    `GET /v1/evidence/${golden("evidence.get").evidence_id}`, "GET /paid"]);
  const settlement = decodeSettlementHeader(result.response.headers.get(PAYMENT_RESPONSE_HEADER));
  await assert.rejects(peer.buyer.captureSettlement(encodeSettlementHeader({ ...settlement,
    extensions: { layerx: { ...evidence, receiptDigest: hex(keccak256(bytes(evidence.signedReceipt))) } },
  }), result.payment), (error) => error instanceof MiddlewareError && error.code === "verification-failure");
  peer.reopen();
  const replay = await peer.send(result.payment.paymentHeader);
  assert.equal(replay.status, 200);
  assert.equal(await replay.text(), "paid receipt resource");
  assert.equal((await peer.buyer.captureSettlement(replay.headers.get(PAYMENT_RESPONSE_HEADER), result.payment)).receiptDigest,
    exact.receiptDigest);
  assert.equal(peer.releases, 1);
  assert.deepEqual(peer.sellerErrors, []);
}

async function agentReceiptSignatureDigestReleasesSeller(context) {
  const evidence = await recordedAgentEvidence();
  const verification = await verifyAgentPayment(evidence, agentSpend, keys);
  const signed = await verifySequencerReceipt(bytes(exact.signedReceipt), bytes(exact.rawTransaction), keys);
  assert.deepEqual(verification.receiptDigest, signed.receiptDigest);
  assert.equal(evidence.receiptDigest, exact.receiptDigest);
  const reservation = { state: "committed", reservationId: "11".repeat(32), requestDigest: "22".repeat(32),
    amount: agentSpend.amount, asset: agentSpend.asset, receiptDigest: exact.receiptDigest.slice(2) };
  assert.equal((await verifyCommittedAgentPayment(evidence, reservation, agentSpend, keys)).kind, "verified");
  const peer = await receiptPeer(context, evidence);
  const offerResponse = await fetch(peer.required.resource.url);
  assert.equal(offerResponse.status, 402);
  const parsed = peer.buyer.parseOffer(offerResponse.headers.get(PAYMENT_REQUIRED_HEADER));
  await offerResponse.text();
  const header = encodePaymentPayloadHeader({ x402Version: 2, resource: parsed.required.resource,
    accepted: parsed.accepted, payload: evidence });
  const released = await peer.send(header);
  assert.equal(released.status, 200);
  assert.equal(await released.text(), "paid receipt resource");
  const settlement = decodeSettlementHeader(released.headers.get(PAYMENT_RESPONSE_HEADER));
  assert.equal(settlement.extensions.layerx.receiptDigest, exact.receiptDigest);
  assert.equal(settlement.transaction, verification.txHash);
  assert.equal(peer.releases, 1);
  assert.deepEqual(peer.sellerErrors, []);
}

async function sellerRefusesKeccakDerivedReceiptDigest(context) {
  const evidence = await recordedAgentEvidence();
  const verified = await verifyAgentPayment(evidence, agentSpend, keys);
  assert.equal(hex(verified.receiptDigest), exact.receiptDigest);
  const blobDigest = hex(keccak256(bytes(evidence.signedReceipt)));
  assert.notEqual(blobDigest, evidence.receiptDigest);
  const peer = await receiptPeer(context, evidence);
  const payload = { x402Version: 2, resource: peer.required.resource, accepted: peer.required.accepts[0], payload: evidence };
  const refused = await peer.send(encodePaymentPayloadHeader({ ...payload,
    payload: { ...evidence, receiptDigest: blobDigest } }));
  assert.equal(refused.status, 400);
  assert.deepEqual(await refused.json(), { code: "verification-failure" });
  assert.equal(peer.sellerErrors.length, 1);
  assert.ok(peer.sellerErrors[0] instanceof MiddlewareError);
  assert.equal(peer.sellerErrors[0].code, "verification-failure");
  assert.equal(peer.releases, 0);
  const released = await peer.send(encodePaymentPayloadHeader(payload));
  assert.equal(released.status, 200);
  assert.equal(await released.text(), "paid receipt resource");
  assert.equal(decodeSettlementHeader(released.headers.get(PAYMENT_RESPONSE_HEADER)).extensions.layerx.receiptDigest,
    exact.receiptDigest);
  assert.equal(peer.releases, 1);
}

test(buyerReceiptSignatureDigestRoundTrip);
test(agentReceiptSignatureDigestReleasesSeller);
test(sellerRefusesKeccakDerivedReceiptDigest);

async function conformanceRpcReceiptPassesStrictSellerConsumer(context) {
  const sequencer = await ConformanceSequencer.generate();
  const facts = { asset: fixedBytes(0x11), payTo: fixedBytes(0x22), amount: 25n,
    from: fixedBytes(0x33), batchId: fixedBytes(0x44),
    previousStateRoot: fixedBytes(0x55), resultingStateRoot: fixedBytes(0x66) };
  const required = offerFixture(facts.payTo, facts.asset, facts.amount);
  const recorded = await buildSignedReceipt(sequencer, facts);
  const receiptKeys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
  const session = new ConformancePaymentRpcSession(sequencer, facts, required.requirements);
  const failures = [];
  const methods = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      methods.push(message.method);
      const result = await session.call(message.method, message.params);
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500).end(String(error));
    }
  });
  context.after(async () => {
    await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); });
    assert.deepEqual(failures, []);
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const endpoint = `http://127.0.0.1:${server.address().port}/`;
  const rpc = async (method, params) => {
    const response = await fetch(endpoint, { method: "POST", headers: { "content-type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id: methods.length + 1, method, params }) });
    assert.equal(response.status, 200);
    return (await response.json()).result;
  };
  const transactionHash = await rpc("eth_sendRawTransaction", [recorded.rawTransaction]);
  assert.equal(transactionHash, recorded.txHash);
  const result = await rpc("pax_sequencerReceipt", [transactionHash]);
  assert.deepEqual(result, recorded.receiptJson);
  assert.deepEqual(Object.keys(result).sort(), ["rung", "txHash", "nonce", "seq", "height", "index", "timestamp",
    "status", "returnData", "logs", "gasUsed", "cumulativeGasUsed", "effectiveGasPrice", "paymentDenom", "feeDenom",
    "feeAmount", "sponsor", "from", "to", "contractAddress", "vmError", "keyId", "signingKey", "receipt"].sort());
  const signed = await verifySequencerReceipt(bytes(recorded.signedReceipt), bytes(recorded.rawTransaction), receiptKeys);
  const verified = await verifyRpcPayment(result, recorded.rawTransaction, hex(signed.receipt.from),
    required.requirements, receiptKeys);
  assert.equal(verified.kind, "verified");
  assert.equal(verified.receiptDigest, recorded.receiptDigest);
  assert.equal(verified.verification.txHash, transactionHash);
  assert.equal(verified.verification.payment.amount, facts.amount);
  assert.deepEqual(verified.verification.payment.to, facts.payTo);
  assert.equal(session.issuedCount, 1);
  assert.deepEqual(methods, ["eth_sendRawTransaction", "pax_sequencerReceipt"]);
}

test(conformanceRpcReceiptPassesStrictSellerConsumer);

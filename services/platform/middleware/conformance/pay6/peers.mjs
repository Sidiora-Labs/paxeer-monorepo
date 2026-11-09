import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DurableFulfillmentRepository, ReceiptPayloadAuthority, SellerMiddleware,
  decodePaymentExactLog, encodePaymentPayloadHeader } from "../../seller/dist/index.js";
import { committedVector, recordedCommittedReceipt } from "../../seller/test/committed-fixture.mjs";
import { AgentHttpTransport, ProductionClient } from "@sidiora/layerx-sdk";
import { verifyAgentPayment } from "@sidiora/layerx-agent-middleware";
import { BuyerMiddleware } from "@sidiora/layerx-buyer-middleware";
import { agentSpend, bytes, exact, hex, keys, offer, payloadFor, pinsFor, proof, txHash } from "./node-fixtures.mjs";

export async function loopback(context, handler) {
  const requests = [];
  const failures = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const raw = Buffer.concat(chunks).toString("utf8");
      const body = raw ? JSON.parse(raw) : undefined;
      requests.push({ method: request.method, url: request.url, headers: request.headers, body });
      const answer = await handler(body, request);
      response.writeHead(answer.status ?? 200, answer.headers ?? { "content-type": "application/json" });
      response.end(answer.text ?? JSON.stringify({ jsonrpc: "2.0", id: body?.id, ...answer }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500).end(String(error));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const close = async () => {
    await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); });
    assert.deepEqual(failures, []);
  };
  context?.after(close);
  return { endpoint: `http://127.0.0.1:${server.address().port}/`, requests, close };
}

export async function paymentPeer(context) {
  const directory = await mkdtemp(join(tmpdir(), "pay6-1479-"));
  const recorded = await recordedCommittedReceipt();
  const committedKeys = pinsFor(committedVector);
  const committedProof = {
    format: "layerx-receipt-proof-v2", verificationLevel: "committed",
    rawTransaction: committedVector.rawTransaction, signedReceipt: committedVector.signedReceipt,
    receiptDigest: hex(recorded.verified.receiptDigest),
    receiptKey: { keyId: committedVector.pinnedKey.keyId, publicKey: committedVector.pinnedKey.publicKey,
      firstHeight: `0x${committedKeys[0].first_height.toString(16)}`,
      lastHeight: `0x${committedKeys[0].last_height.toString(16)}` },
  };
  const committedLog = committedVector.receiptJson.logs.find((entry) =>
    entry.topics[0] === exact.receiptJson.logs[0].topics[0]);
  const committedPayment = decodePaymentExactLog({ address: committedLog.address,
    topics: committedLog.topics.map(bytes), data: bytes(committedLog.data) });
  const committedOffer = { ...offer, amount: committedPayment.amount.toString(),
    asset: hex(committedPayment.assetId), payTo: hex(committedPayment.to),
    extra: { layerx: { commitment: "committed", payer: hex(committedPayment.from).slice(2),
      purposeHash: hex(committedPayment.paymentReference).slice(2) } } };
  const committedTxHash = hex(recorded.verified.receipt.activityId);
  const committedResult = { ...committedVector.receiptJson, rung: "committed",
    blockNumber: `0x${recorded.evidence.landingBlockHeight.toString(16)}`,
    blockHash: hex(recorded.evidence.blockHeader.blockId.hash) };
  const commitments = {
    async resolve(transactionHash, network) {
      assert.equal(transactionHash, committedTxHash);
      assert.equal(network, committedOffer.network);
      return recorded.resolver.resolve(transactionHash, network);
    },
    resolveValidators: (network, height) => recorded.resolver.resolveValidators(network, height),
  };
  let releases = 0;
  let required;
  let seller;
  let committedRequired;
  let committedSeller;
  let buyer;
  let resource;
  const settlements = {};
  const paidRequests = [];
  const reopen = () => {
    seller = new SellerMiddleware({ paymentRequired: required, receiptKeys: keys,
      authority: new ReceiptPayloadAuthority(keys), fulfillments: new DurableFulfillmentRepository(directory) });
    committedSeller = new SellerMiddleware({ paymentRequired: committedRequired, receiptKeys: committedKeys,
      authority: new ReceiptPayloadAuthority(committedKeys), commitments,
      fulfillments: new DurableFulfillmentRepository(directory) });
  };
  const peer = await loopback(context, async (body, request) => {
    if (request.method === "POST" && request.url === "/agent") {
      const verified = await verifyAgentPayment(body.proof, body.spend, keys);
      return { text: JSON.stringify({ txHash: verified.txHash, receiptDigest: hex(verified.receiptDigest),
        network: body.spend.commitment.network, amount: verified.payment.amount.toString(),
        asset: hex(verified.payment.assetId), payTo: hex(verified.payment.to) }) };
    }
    if (request.method === "POST" && request.url === "/capture") {
      const captured = await buyer.captureSettlement(body.settlement, { ...proof, txHash, offer: { accepted: offer } });
      return { text: JSON.stringify({ txHash: captured.txHash, receiptDigest: captured.receiptDigest, network: captured.response.network }) };
    }
    if (request.method === "POST" && request.url === "/reopen") { reopen(); return { text: JSON.stringify({ releases }) }; }
    if (request.method === "GET" && request.url === "/metrics") return { text: JSON.stringify({ releases, settlements, paidRequests }) };
    if (request.method === "GET" && request.url === "/fixture") return { text: JSON.stringify({ required, proof, txHash, agentSpend,
      header: encodePaymentPayloadHeader(payloadFor(resource)), result: exact.receiptJson,
      committed: { required: committedRequired, proof: committedProof, txHash: committedTxHash,
        header: encodePaymentPayloadHeader({ x402Version: 2, resource: committedRequired.resource,
          accepted: committedOffer, payload: committedProof }), result: committedResult } }) };
    if (request.method === "GET" && (request.url === "/paid" || request.url === "/committed")) {
      const selected = request.url === "/committed" ? committedSeller : seller;
      const decision = await selected.handle(request.headers["x-principal"] ?? "payer", request.headers["payment-signature"], async () => {
        releases += 1;
        return request.url === "/committed" ? "committed resource" : "resource";
      });
      paidRequests.push({ path: request.url, status: decision.status });
      if (decision.kind === "released") settlements[request.url] = decision.headers["PAYMENT-RESPONSE"];
      return { status: decision.status, headers: decision.headers,
        text: decision.kind === "released" ? decision.resource : JSON.stringify(decision.body ?? decision.settlement) };
    }
    assert.equal(request.method, "POST");
    assert.equal(request.url, "/");
    assert.equal(request.headers["content-type"], "application/json");
    assert.equal(body.jsonrpc, "2.0");
    assert.ok(Number.isSafeInteger(body.id));
    if (body.method === "eth_sendRawTransaction") {
      assert.ok([proof.rawTransaction, committedProof.rawTransaction].includes(body.params[0]));
      assert.equal(body.params.length, 1);
      return { result: body.params[0] === committedProof.rawTransaction ? committedTxHash : txHash };
    }
    assert.equal(body.method, "pax_sequencerReceipt");
    if (body.params[0] === committedTxHash) {
      assert.deepEqual(body.params, [committedTxHash]);
      return { result: committedResult };
    }
    assert.deepEqual(body.params, [txHash]);
    return { result: exact.receiptJson };
  });
  resource = { url: `${peer.endpoint}paid` };
  required = { x402Version: 2, resource, accepts: [offer] };
  committedRequired = { x402Version: 2, resource: { url: `${peer.endpoint}committed` }, accepts: [committedOffer] };
  reopen();
  buyer = new BuyerMiddleware({ client: new ProductionClient(new AgentHttpTransport({ endpoint: peer.endpoint })),
    source: "acct:buyer", supported: [{ scheme: "exact", network: offer.network }], receiptKeys: keys });
  const close = async () => { await peer.close(); await rm(directory, { recursive: true, force: true }); };
  // The listener belongs to loopback's hook; only storage cleanup is added here.
  context?.after(() => rm(directory, { recursive: true, force: true }));
  return { ...peer, close, required, resource, header: encodePaymentPayloadHeader(payloadFor(resource)),
    reopen, get releases() { return releases; } };
}

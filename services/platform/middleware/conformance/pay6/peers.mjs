import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DurableFulfillmentRepository, ReceiptPayloadAuthority, SellerMiddleware,
  encodePaymentPayloadHeader } from "../../seller/dist/index.js";
import { AgentHttpTransport, ProductionClient } from "@sidiora/layerx-sdk";
import { verifyAgentPayment } from "@sidiora/layerx-agent-middleware";
import { BuyerMiddleware } from "@sidiora/layerx-buyer-middleware";
import { agentSpend, exact, hex, keys, offer, payloadFor, proof, txHash } from "./node-fixtures.mjs";

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
  let releases = 0;
  let required;
  let seller;
  let buyer;
  let resource;
  const reopen = () => {
    seller = new SellerMiddleware({ paymentRequired: required, receiptKeys: keys,
      authority: new ReceiptPayloadAuthority(keys), fulfillments: new DurableFulfillmentRepository(directory) });
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
    if (request.method === "GET" && request.url === "/metrics") return { text: JSON.stringify({ releases }) };
    if (request.method === "GET" && request.url === "/fixture") return { text: JSON.stringify({ required, proof, txHash, agentSpend,
      header: encodePaymentPayloadHeader(payloadFor(resource)), result: exact.receiptJson }) };
    if (request.method === "GET" && request.url === "/paid") {
      const decision = await seller.handle(request.headers["x-principal"] ?? "payer", request.headers["payment-signature"], async () => {
        releases += 1;
        return "resource";
      });
      return { status: decision.status, headers: decision.headers,
        text: decision.kind === "released" ? decision.resource : JSON.stringify(decision.body ?? decision.settlement) };
    }
    assert.equal(request.method, "POST");
    assert.equal(request.url, "/");
    assert.equal(request.headers["content-type"], "application/json");
    assert.equal(body.jsonrpc, "2.0");
    assert.ok(Number.isSafeInteger(body.id));
    if (body.method === "eth_sendRawTransaction") {
      assert.deepEqual(body.params, [proof.rawTransaction]);
      return { result: txHash };
    }
    assert.equal(body.method, "pax_sequencerReceipt");
    assert.deepEqual(body.params, [txHash]);
    return { result: exact.receiptJson };
  });
  resource = { url: `${peer.endpoint}paid` };
  required = { x402Version: 2, resource, accepts: [offer] };
  reopen();
  buyer = new BuyerMiddleware({ client: new ProductionClient(new AgentHttpTransport({ endpoint: peer.endpoint })),
    source: "acct:buyer", supported: [{ scheme: "exact", network: offer.network }], receiptKeys: keys });
  const close = async () => { await peer.close(); await rm(directory, { recursive: true, force: true }); };
  // The listener belongs to loopback's hook; only storage cleanup is added here.
  context?.after(() => rm(directory, { recursive: true, force: true }));
  return { ...peer, close, required, resource, header: encodePaymentPayloadHeader(payloadFor(resource)),
    reopen, get releases() { return releases; } };
}

import assert from "node:assert/strict";
import { once } from "node:events";
import { readFile } from "node:fs/promises";
import { createServer as createHttpServer } from "node:http";
import { createServer as createHttpsServer } from "node:https";
import { Agent, getGlobalDispatcher, setGlobalDispatcher } from "undici";
import {
  decodePaymentExactLog,
  decodePaymentPayloadHeader,
  PAYMENT_SIGNATURE_HEADER,
  ReceiptPayloadAuthority,
  SellerMiddleware,
} from "@sidiora/layerx-seller-middleware";
import { keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { createPaidApiServer, FileFulfillmentRepository } from "../../paid-api/index.mjs";
import { SequencerReceiptClient } from "../../support/runtime.mjs";

const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const json = async (url) => JSON.parse(await readFile(url, "utf8"));

export async function buyerPaymentFixture(context, directory) {
  const document = await json(new URL("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url));
  assert.equal(document.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
  const vector = document.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
  assert.ok(vector);
  const pin = {
    keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
    first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
  };
  await verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), [pin]);
  const log = vector.receiptJson.logs[0];
  const paid = decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
  const transcript = await json(new URL("./fixtures/human-transcript.json", import.meta.url));
  const exchanges = [];
  const failures = [];
  const payments = [];
  const token = "committed-buyer-fixture-bearer";
  const source = "acct:buyer-fixture";
  const paymentKey = "buyer-human-endpoint-22-31-347";
  const account = "agent:did:layerx:payee:main";
  const body = { weather: "sunny", access: "receipt-verified" };

  const readBody = async (request) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    return chunks.length === 0 ? undefined : JSON.parse(Buffer.concat(chunks).toString("utf8"));
  };
  const listen = async (server, protocol) => {
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    context.after(async () => {
      server.closeAllConnections();
      await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    });
    return `${protocol}://127.0.0.1:${server.address().port}`;
  };
  const record = (origin, request, response) => {
    const call = {
      origin, path: request.url, method: request.method,
      authorization: request.headers.authorization ?? null,
      idempotencyKey: request.headers["idempotency-key"] ?? null,
    };
    exchanges.push(call);
    response.on("finish", () => { call.status = response.statusCode; });
  };
  const guard = (handler) => async (request, response) => {
    try {
      await handler(request, response);
    } catch (error) {
      failures.push(error);
      response.writeHead(500).end();
    }
  };

  let rpcUrl;
  const node = createHttpServer(guard(async (request, response) => {
    record(rpcUrl, request, response);
    assert.equal(request.headers.authorization, undefined);
    assert.equal(request.method, "POST");
    assert.equal(request.url, "/");
    const call = await readBody(request);
    assert.deepEqual(call, {
      jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt",
      params: [hex(keccak256(bytes(vector.rawTransaction)))],
    });
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({ jsonrpc: "2.0", id: call.id, result: vector.receiptJson }));
  }));
  rpcUrl = await listen(node, "http");
  const resolver = new SequencerReceiptClient(rpcUrl, [pin]);

  const certificate = await readFile(new URL("./fixtures/localhost-cert.pem", import.meta.url));
  const key = await readFile(new URL("./fixtures/localhost-key.pem", import.meta.url));
  const dispatcher = new Agent({ connect: { ca: certificate } });
  const previousDispatcher = getGlobalDispatcher();
  setGlobalDispatcher(dispatcher);
  context.after(async () => {
    setGlobalDispatcher(previousDispatcher);
    await dispatcher.close();
  });
  let humanUrl;
  const human = createHttpsServer({ cert: certificate, key }, guard(async (request, response) => {
    record(humanUrl, request, response);
    assert.equal(request.headers.authorization, `Bearer ${token}`);
    const received = await readBody(request);
    let exchange;
    if (request.method === "POST" && request.url === "/v1/moves/quote") {
      assert.deepEqual(received, { source, destination: account, money: { amount: paid.amount.toString(), currency: "LXP" } });
      exchange = structuredClone(transcript["move.quote"]);
    } else if (request.method === "POST" && request.url === "/v1/moves") {
      assert.deepEqual(received, { quote_id: transcript["move.quote"].body.result.quote_id });
      assert.equal(request.headers["idempotency-key"], paymentKey);
      exchange = structuredClone(transcript["move.commit"]);
    } else {
      assert.equal(request.method, "GET");
      assert.equal(request.url, `/v1/evidence/${transcript["evidence.get"].body.result.evidence_id}`);
      assert.equal(received, undefined);
      exchange = structuredClone(transcript["evidence.get"]);
      exchange.body.result.bytes_base64 = Buffer.from(JSON.stringify(await resolver.resolve(vector.rawTransaction))).toString("base64");
    }
    response.writeHead(exchange.status, { "content-type": "application/json" });
    response.end(JSON.stringify(exchange.body));
  }));
  humanUrl = await listen(human, "https");

  // Bind the loopback port before constructing the seller's exact resource
  // offer. The adapter forwards every call to the real SellerMiddleware.
  const server = createPaidApiServer({
    async handle(...arguments_) { return seller.handle(...arguments_); },
  }, body);
  const resourceOrigin = await listen(server, "http");
  const resourceUrl = `${resourceOrigin}/paid`;
  const seller = new SellerMiddleware({
    receiptKeys: [pin],
    paymentRequired: {
      x402Version: 2, resource: { url: resourceUrl },
      accepts: [{ scheme: "exact", network: "eip155:125", amount: paid.amount.toString(),
        asset: hex(paid.assetId), payTo: hex(paid.to), maxTimeoutSeconds: 120,
        extra: { layerx: { commitment: "receipted", account, currency: "LXP",
          payer: hex(paid.from).slice(2), purposeHash: hex(paid.paymentReference).slice(2) } },
      }],
    },
    authority: new ReceiptPayloadAuthority([pin]),
    fulfillments: new FileFulfillmentRepository(directory),
  });
  server.prependListener("request", (request, response) => {
    record(resourceOrigin, request, response);
    if (request.headers.authorization !== undefined) failures.push(new Error("Human bearer reached seller"));
    const header = request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()];
    if (header !== undefined) payments.push(decodePaymentPayloadHeader(header));
  });
  return { humanUrl, rpcUrl, resourceUrl, token, source, paymentKey, pins: [pin],
    pinInputs: [vector.pinnedKey], exchanges, failures, payments, vector, body };
}

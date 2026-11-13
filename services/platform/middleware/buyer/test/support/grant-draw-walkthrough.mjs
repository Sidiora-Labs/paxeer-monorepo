import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { ProductionClient, SecretBytes, decodeRawEip1559, keccak256, recoverEip1559Sender } from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS, MiddlewareError, PAYMENT_REQUIRED_HEADER, PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER, PaymentRpc, decodePaymentPayloadHeader, decodeSettlementHeader,
  encodeLedgerDraw, encodePaymentRequiredHeader, encodeSettlementHeader, verifyPaymentReceipt,
} from "@sidiora/layerx-seller-middleware";
import * as buyer from "@sidiora/layerx-buyer-middleware";

const transcript = JSON.parse(readFileSync(new URL("./grant-draw-transcript.json", import.meta.url), "utf8"));
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Uint8Array.from(Buffer.from(value.replace(/^0x/u, ""), "hex"));
const word = (value) => new Uint8Array(32).fill(value);
const rawTransaction = transcript.proof.rawTransaction;
const txHash = hex(keccak256(bytes(rawTransaction)));
const payer = "11".repeat(32);
const recipient = `0x${"66".repeat(32)}`;
const asset = `0x${"88".repeat(32)}`;
const key = transcript.proof.receiptKey;
const receiptKeys = [{ keyId: bytes(key.keyId), publicKey: bytes(key.publicKey),
  first_height: BigInt(key.firstHeight), last_height: BigInt(key.lastHeight) }];
const verificationFailure = (error) => error instanceof MiddlewareError && error.code === "verification-failure";

function offer(scheme) {
  return { x402Version: 2, resource: { url: "https://seller.example/paid" }, accepts: [{
    scheme, network: "eip155:125", amount: "25", asset, payTo: recipient, maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", payer, purposeHash: "22".repeat(32), contextHash: "55".repeat(32),
      grant: { grantId: "33".repeat(32), asset },
      ...(scheme === "subscription" ? { windowSeconds: "3600" } : {}),
    } },
  }], extensions: { grant: { info: { version: 1 }, schema: {} } } };
}

async function listen(server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${server.address().port}/`;
}

async function close(server) {
  const closed = new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  server.closeAllConnections();
  await closed;
}

// These peers replay the committed conformance wire transcript. They never
// implement a ledger, sign receipts, or claim to execute the native precompile.
// See repair-22-33-1477.kvx for the owning ledger domain's missing transcript.
async function openPeers(state, required) {
  const node = createServer(async (request, response) => {
    try {
      assert.equal(request.url, "/");
      assert.equal(request.method, "POST");
      assert.equal(request.headers["content-type"], "application/json");
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      assert.deepEqual(Object.keys(body).sort(), ["id", "jsonrpc", "method", "params"]);
      assert.equal(body.jsonrpc, "2.0");
      assert.ok(Number.isSafeInteger(body.id) && body.id > 0);
      state.nodeRequests.push(body);
      let result;
      if (body.method === "eth_sendRawTransaction") {
        assert.deepEqual(body.params, [rawTransaction]);
        assert.equal(state.submissions, 0, "the walkthrough submits once, before observing outcomes");
        state.submissions += 1;
        state.accepted.set(txHash, rawTransaction);
        result = txHash;
      } else {
        assert.equal(body.method, "pax_sequencerReceipt");
        assert.deepEqual(body.params, [txHash]);
        assert.equal(state.accepted.get(txHash), rawTransaction);
        result = state.outcome === "unknown" ? null : state.outcome === "submitted"
          ? { rung: "submitted", txHash }
          : { rung: "receipted", txHash, receipt: transcript[state.variant].signedReceipt, keyId: key.keyId, height: "0x1" };
      }
      state.nodeResponses.push(result);
      response.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result }));
    } catch (error) {
      state.errors.push(error);
      response.writeHead(500).end("invalid transcript request");
    }
  });
  const nodeUrl = await listen(node);
  const rpc = new PaymentRpc(nodeUrl);
  const merchant = createServer(async (request, response) => {
    try {
      assert.equal(request.url, "/paid");
      assert.equal(request.method, "GET");
      const header = request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()];
      if (header === undefined) {
        state.challenges += 1;
        response.writeHead(402, { [PAYMENT_REQUIRED_HEADER]: encodePaymentRequiredHeader(required) }).end();
        return;
      }
      const payload = decodePaymentPayloadHeader(header);
      assert.deepEqual(payload, { x402Version: 2, resource: required.resource, accepted: required.accepts[0],
        extensions: required.extensions, payload: { rawTransaction, idempotencyKey: "44".repeat(32) } });
      state.headers.push(header);
      if (state.outcome === "refused") {
        const refusal = encodeSettlementHeader({
          success: false, errorReason: "grant_unavailable", network: "eip155:125", transaction: "", amount: "25", payer,
        });
        state.refusals.push(refusal);
        response.writeHead(402, { [PAYMENT_RESPONSE_HEADER]: refusal }).end();
        return;
      }
      const observed = await rpc.receipt(txHash);
      if (observed === null || observed.rung === "submitted") {
        response.writeHead(202, { "retry-after": "1" }).end();
        return;
      }
      assert.deepEqual(observed, { rung: "receipted", txHash, receipt: transcript[state.variant].signedReceipt,
        keyId: key.keyId, height: "0x1" });
      const proof = { ...transcript[state.variant], rawTransaction: payload.payload.rawTransaction,
        signedReceipt: observed.receipt };
      if (state.variant === "proof") {
        const verification = await verifyPaymentReceipt(proof, payload.accepted, receiptKeys);
        assert.equal(verification.txHash, txHash);
        assert.equal(hex(verification.payment.from), `0x${payer}`);
        assert.equal(hex(verification.payment.to), recipient);
      }
      const settlement = encodeSettlementHeader({ success: true, network: "eip155:125", transaction: txHash,
        payer, amount: "25", extensions: { layerx: proof } });
      state.settlements.push(settlement);
      response.writeHead(200, { [PAYMENT_RESPONSE_HEADER]: settlement }).end("paid");
    } catch (error) {
      state.errors.push(error);
      response.writeHead(500).end("invalid merchant request");
    }
  });
  const merchantUrl = await listen(merchant);
  return { rpc, nodeUrl, resourceUrl: `${merchantUrl}paid`, async close() { await Promise.all([close(merchant), close(node)]); } };
}

function makeBuyer(api, nodeUrl, t) {
  const token = new SecretBytes(new TextEncoder().encode("local-transcript-only"));
  t.after(() => token.destroy());
  return new api.BuyerMiddleware({
    client: new ProductionClient(new api.LayerXPaymentHttpTransport({ baseUrl: nodeUrl, bearerToken: token })),
    source: "acct:grant-payer", supported: ["metered", "subscription"].map((scheme) => ({ scheme, network: "eip155:125" })),
    receiptKeys,
  });
}

function assertPaid(result, header) {
  assert.equal(result.response.status, 200);
  assert.ok(result.settlement);
  assert.equal(result.settlement.txHash, txHash);
  assert.equal(result.settlement.rawTransaction, rawTransaction);
  assert.equal(result.settlement.signedReceipt, transcript.proof.signedReceipt);
  assert.equal(result.settlement.receiptDigest, transcript.proof.receiptDigest);
  assert.deepEqual(result.settlement.response, decodeSettlementHeader(header));
  assert.equal(result.settlement.verification.rung, "receipted");
  assert.equal(hex(result.settlement.verification.receipt.from),
    `0x${recoverEip1559Sender(bytes(rawTransaction)).slice(2).toLowerCase().padStart(64, "0")}`);
  assert.equal(hex(result.settlement.verification.receipt.to), `0x${"00".repeat(12)}${LAYERX_LEDGER_ADDRESS.slice(2)}`);
  assert.notEqual(hex(result.settlement.verification.receipt.from), `0x${payer}`);
  assert.deepEqual(result.settlement.verification.payment, {
    grantId: word(0x33), from: word(0x11), to: word(0x66), assetId: word(0x88), amount: 25n,
    idempotencyKey: word(0x44), contextHash: word(0x55),
  });
}

async function grantDrawWalkthrough(t, api, scheme) {
  const required = offer(scheme);
  const transaction = decodeRawEip1559(bytes(rawTransaction));
  assert.equal(transaction.chainId, 125n);
  assert.equal(hex(transaction.to), LAYERX_LEDGER_ADDRESS);
  assert.equal(transaction.value, 0n);
  assert.deepEqual(transaction.data, encodeLedgerDraw({ recipient: bytes(recipient), purposeHash: word(0x22),
    amount: 25n, grantId: word(0x33), sequence: 1n, idempotencyKey: word(0x44), contextHash: word(0x55) }));
  assert.equal(hex(transaction.data.subarray(36, 68)), recipient);
  assert.equal(hex(transaction.data.subarray(164, 196)), `0x${"22".repeat(32)}`);
  assert.notEqual(recipient, `0x${payer}`);
  assert.notEqual(hex(transaction.data.subarray(164, 196)), asset);
  const state = { outcome: "unknown", variant: "proof", accepted: new Map(), submissions: 0, challenges: 0,
    nodeRequests: [], nodeResponses: [], headers: [], settlements: [], refusals: [], errors: [] };
  let peers = await openPeers(state, required);
  t.after(async () => { await peers.close(); assert.deepEqual(state.errors, []); });
  let middleware = makeBuyer(api, peers.nodeUrl, t);
  const header = api.grantPaymentHeader(required, required.accepts[0], rawTransaction);
  assert.equal(header, middleware.grantHeader(encodePaymentRequiredHeader(required), rawTransaction));
  assert.throws(() => api.grantPaymentHeader(required, required.accepts[0], transcript.payerAsRecipientTransaction),
    (error) => error instanceof MiddlewareError && error.code === "requirements-mismatch");
  assert.equal(await peers.rpc.send(rawTransaction), txHash);
  const unknown = await middleware.fetchGrant(peers.resourceUrl, {}, rawTransaction, txHash);
  assert.equal(unknown.response.status, 202);
  assert.equal(unknown.settlement, undefined);
  assert.equal(unknown.response.headers.get(PAYMENT_RESPONSE_HEADER), null);
  assert.equal(await unknown.response.text(), "");
  assert.equal(state.nodeResponses.at(-1), null);

  // Stop both peers and discard the consumer. Retain only the peer's in-memory
  // accepted transaction and the caller's original raw transaction/hash.
  await peers.close();
  peers = await openPeers(state, required);
  middleware = makeBuyer(api, peers.nodeUrl, t);
  assert.equal(middleware.grantHeader(encodePaymentRequiredHeader(required), rawTransaction), header);
  const reopened = await middleware.fetchGrant(peers.resourceUrl, {}, rawTransaction, txHash);
  assert.equal(reopened.response.status, 202);
  assert.equal(reopened.settlement, undefined);
  assert.equal(await reopened.response.text(), "");
  assert.equal(state.submissions, 1);
  assert.equal(state.settlements.length, 0);

  state.outcome = "submitted";
  const pending = await middleware.fetchGrant(peers.resourceUrl, {}, rawTransaction, txHash);
  assert.equal(pending.response.status, 202);
  assert.equal(pending.settlement, undefined);
  assert.equal(await pending.response.text(), "");
  assert.deepEqual(state.nodeResponses.at(-1), { rung: "submitted", txHash });

  state.outcome = "receipted";
  for (let round = 0; round < 2; round += 1) {
    const paid = await middleware.fetchGrant(peers.resourceUrl, {}, rawTransaction, txHash);
    assertPaid(paid, state.settlements.at(-1));
    assert.equal(await paid.response.text(), "paid");
    assert.equal(state.submissions, 1, "recovery and repeated observation do not resubmit");
    middleware = makeBuyer(api, peers.nodeUrl, t);
  }
  const savedHeader = state.settlements.at(-1);
  const recovered = await makeBuyer(api, peers.nodeUrl, t).captureGrantSettlement(savedHeader, header, txHash);
  assert.equal(recovered.receiptDigest, transcript.proof.receiptDigest);
  assert.equal(hex(recovered.verification.payment.to), recipient);
  assert.equal(hex(recovered.verification.payment.from), `0x${payer}`);
  state.outcome = "refused";
  const observations = state.nodeRequests.length;
  await assert.rejects(middleware.fetchGrant(peers.resourceUrl, {}, rawTransaction, txHash), verificationFailure);
  assert.equal(state.nodeRequests.length, observations, "a refusal supplies no successful receipt");
  assert.equal(state.refusals.length, 1);
  assert.deepEqual(decodeSettlementHeader(state.refusals[0]), {
    success: false, errorReason: "grant_unavailable", network: "eip155:125", transaction: "", amount: "25", payer,
  });
  state.outcome = "receipted";
  for (const variant of ["wrongRecipient", "wrongPayer"]) {
    state.variant = variant;
    await assert.rejects(middleware.fetchGrant(peers.resourceUrl, {}, rawTransaction, txHash), verificationFailure);
  }
  assert.equal(state.submissions, 1);
  assert.equal(state.challenges, 8);
  assert.equal(state.headers.length, 8);
  assert.ok(state.headers.every((observed) => observed === header));
  assert.deepEqual(state.nodeRequests.map(({ method, params }) => ({ method, params })), [
    { method: "eth_sendRawTransaction", params: [rawTransaction] },
    ...Array.from({ length: 7 }, () => ({ method: "pax_sequencerReceipt", params: [txHash] })),
  ]);
}

export async function buyerDrawFixture(t) {
  for (const scheme of ["metered", "subscription"]) {
    await t.test(scheme, async (context) => grantDrawWalkthrough(context, buyer, scheme));
  }
}

export async function buyerExportDrawFixture(t) {
  for (const name of ["express", "next"]) {
    const api = await import(`@sidiora/layerx-${name}`);
    assert.equal(api.BuyerMiddleware, buyer.BuyerMiddleware);
    assert.equal(api.grantPaymentHeader, buyer.grantPaymentHeader);
    for (const scheme of ["metered", "subscription"]) {
      await t.test(`${name} ${scheme}`, async (context) => grantDrawWalkthrough(context, api, scheme));
    }
  }
}

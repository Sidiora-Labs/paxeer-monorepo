import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import test from "node:test";
import { ProductionClient, SecretBytes, decodeRawEip1559, keccak256 } from "@sidiora/layerx-sdk";
import {
  MiddlewareError, PAYMENT_REQUIRED_HEADER, PAYMENT_RESPONSE_HEADER, PAYMENT_SIGNATURE_HEADER,
  PaymentRpc, decodePaymentPayloadHeader, encodeLedgerDraw, encodePaymentPayloadHeader,
  encodePaymentRequiredHeader, encodeSettlementHeader,
} from "@sidiora/layerx-seller-middleware";
import { BuyerMiddleware, LayerXPaymentHttpTransport, grantPaymentHeader } from "../dist/index.js";

// Task 22.31.436: buyer-draw-recipient is proved by buyerDrawRecipientThroughNode,
// buyerDrawRecipientBindsCalldata and buyerDrawRecipientAuthenticatesGrantPayer.
// The baseline gate was not run: the bounded gate order requires implementation first.
// No out-of-path implementation is required by this finding's proposal.
// Committed signed wire fixtures use the existing grant-draw-header receipt layout,
// with distinct grant payer (11), recipient (66), asset (88) and purpose (22).
const fixture = {
  "proof": {
    "format": "layerx-receipt-proof-v2",
    "verificationLevel": "receipted",
    "rawTransaction": "0x02f901487d80010283030d4094000000000000000000000000000000000000101a80b8e42e89490f3333333333333333333333333333333333333333333333333333333333333333666666666666666666666666666666666666666666666666666666666666666600000000000000000000000000000000000000000000000000000000000000194444444444444444444444444444444444444444444444444444444444444444000000000000000000000000000000000000000000000000000000000000000122222222222222222222222222222222222222222222222222222222222222225555555555555555555555555555555555555555555555555555555555555555c001a0d23f98cd579ec6ce86ab5bd32c6e879ef80d21417fff8557d58f520a37b6b230a0414c16ea9fd91dee4cdda9645ec72b018caa9fc6ed98e850e633ac4fc469b082",
    "signedReceipt": "0x000252010002000000205ec2704041e4849d8a2bfb73636afe1cd5e2b7c689012c71af259a70e221f62800000000000000010000002000000000000000000000000000000000000000000000000000000000000000000000002071b91e53e57dbc6c821ce8856ca9ec75b1579b7b12a492b7862472bc5eabe5b400000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550000000000000002000c00000000010000000020000000000000000000000000000000000000000000000000000000000000000000000071000000000000c350000000000000c35000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85500000000000000010475687078000c000100000300000000200000000000000000000000000000000000000000000000000000000000000000000000b5000000000000000000000000000000000000101a0413473d103a2d52172016c872eeae0153e254af3c21a1d987492ab737f1a34fe1333333333333333333333333333333333333333333333333333333333333333311111111111111111111111111111111111111111111111111111111111111116666666666666666666666666666666666666666666666666666666666666666b134bb71d1abe9f3013e19ba2916198362138e8e254b2e3e8d197ca82b3b5417000000000000000000000000000186a0000000200000000000000000000000000000000000000000000000000000000000000007000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f7300000000000000000000000000000000000000200000000000000000000000001a642f0e3c3af545e7acbd38b07251b3990914f1000000000000000000000000000f4240000000000000000000000000000dbba0000000000000000000000020000000000000000000000000000000000000000000000000000000000000101a00000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000002077777777777777777777777777777777777777777777777777777777777777770000000000000001010000004078d33aec2efd7065db296fb5a7f302493fc6095eec3c8a390d962e79323d113c461a0f0c2dfbb539e9dcbb5e0703ba86d7dacc47a87d3c7dfed7f56eaa8c9e07",
    "receiptDigest": "0x6bffde8de5088b8ad893cfbf0ec86c75a5aaeeac2cb4952a5795a9e1e8e46830",
    "receiptKey": {
      "keyId": "0x7777777777777777777777777777777777777777777777777777777777777777",
      "publicKey": "0x1ae3db73e76bb9c3acf08dd132abde09425dc3b696bf48f89e5ec1e66826b413",
      "firstHeight": "0x1",
      "lastHeight": "0x3e8"
    }
  },
  "wrongRecipient": {
    "format": "layerx-receipt-proof-v2",
    "verificationLevel": "receipted",
    "rawTransaction": "0x02f901487d80010283030d4094000000000000000000000000000000000000101a80b8e42e89490f3333333333333333333333333333333333333333333333333333333333333333666666666666666666666666666666666666666666666666666666666666666600000000000000000000000000000000000000000000000000000000000000194444444444444444444444444444444444444444444444444444444444444444000000000000000000000000000000000000000000000000000000000000000122222222222222222222222222222222222222222222222222222222222222225555555555555555555555555555555555555555555555555555555555555555c001a0d23f98cd579ec6ce86ab5bd32c6e879ef80d21417fff8557d58f520a37b6b230a0414c16ea9fd91dee4cdda9645ec72b018caa9fc6ed98e850e633ac4fc469b082",
    "signedReceipt": "0x000252010002000000205ec2704041e4849d8a2bfb73636afe1cd5e2b7c689012c71af259a70e221f62800000000000000010000002000000000000000000000000000000000000000000000000000000000000000000000002071b91e53e57dbc6c821ce8856ca9ec75b1579b7b12a492b7862472bc5eabe5b400000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550000000000000002000c00000000010000000020000000000000000000000000000000000000000000000000000000000000000000000071000000000000c350000000000000c35000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85500000000000000010475687078000c000100000300000000200000000000000000000000000000000000000000000000000000000000000000000000b5000000000000000000000000000000000000101a0413473d103a2d52172016c872eeae0153e254af3c21a1d987492ab737f1a34fe1333333333333333333333333333333333333333333333333333333333333333311111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111b134bb71d1abe9f3013e19ba2916198362138e8e254b2e3e8d197ca82b3b5417000000000000000000000000000186a0000000200000000000000000000000000000000000000000000000000000000000000007000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f7300000000000000000000000000000000000000200000000000000000000000001a642f0e3c3af545e7acbd38b07251b3990914f1000000000000000000000000000f4240000000000000000000000000000dbba0000000000000000000000020000000000000000000000000000000000000000000000000000000000000101a00000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000002077777777777777777777777777777777777777777777777777777777777777770000000000000001010000004061591345eb8050f32d2a2f758903b00c3ecfd4b58eee671946dbceb3f16f4a63e80d66e91f0a69b820a5fdb7e95f75716c41ba9c3032b8725549aed1f89b030d",
    "receiptDigest": "0x36cc28ba205b81833cdb3ba21ea2828a89fb65db63dccc2ca72467ad6c807661",
    "receiptKey": {
      "keyId": "0x7777777777777777777777777777777777777777777777777777777777777777",
      "publicKey": "0x1ae3db73e76bb9c3acf08dd132abde09425dc3b696bf48f89e5ec1e66826b413",
      "firstHeight": "0x1",
      "lastHeight": "0x3e8"
    }
  },
  "wrongPayer": {
    "format": "layerx-receipt-proof-v2",
    "verificationLevel": "receipted",
    "rawTransaction": "0x02f901487d80010283030d4094000000000000000000000000000000000000101a80b8e42e89490f3333333333333333333333333333333333333333333333333333333333333333666666666666666666666666666666666666666666666666666666666666666600000000000000000000000000000000000000000000000000000000000000194444444444444444444444444444444444444444444444444444444444444444000000000000000000000000000000000000000000000000000000000000000122222222222222222222222222222222222222222222222222222222222222225555555555555555555555555555555555555555555555555555555555555555c001a0d23f98cd579ec6ce86ab5bd32c6e879ef80d21417fff8557d58f520a37b6b230a0414c16ea9fd91dee4cdda9645ec72b018caa9fc6ed98e850e633ac4fc469b082",
    "signedReceipt": "0x000252010002000000205ec2704041e4849d8a2bfb73636afe1cd5e2b7c689012c71af259a70e221f62800000000000000010000002000000000000000000000000000000000000000000000000000000000000000000000002071b91e53e57dbc6c821ce8856ca9ec75b1579b7b12a492b7862472bc5eabe5b400000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550000000000000002000c00000000010000000020000000000000000000000000000000000000000000000000000000000000000000000071000000000000c350000000000000c35000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85500000000000000010475687078000c000100000300000000200000000000000000000000000000000000000000000000000000000000000000000000b5000000000000000000000000000000000000101a0413473d103a2d52172016c872eeae0153e254af3c21a1d987492ab737f1a34fe1333333333333333333333333333333333333333333333333333333333333333366666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666666b134bb71d1abe9f3013e19ba2916198362138e8e254b2e3e8d197ca82b3b5417000000000000000000000000000186a0000000200000000000000000000000000000000000000000000000000000000000000007000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f7300000000000000000000000000000000000000200000000000000000000000001a642f0e3c3af545e7acbd38b07251b3990914f1000000000000000000000000000f4240000000000000000000000000000dbba0000000000000000000000020000000000000000000000000000000000000000000000000000000000000101a0000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000207777777777777777777777777777777777777777777777777777777777777777000000000000000101000000407b99da52c270cf8fb484a8bc5f55c2d94552811576abc56a6dfd3ffede1053c7ad4e553e1af0332c73d100a7bd810682c4155bcec1e5181a49e1f04d48619f0a",
    "receiptDigest": "0x4e37ed5153b275650891601f6703e5f5b353fadf202be5effefec61fa22563",
    "receiptKey": {
      "keyId": "0x7777777777777777777777777777777777777777777777777777777777777777",
      "publicKey": "0x1ae3db73e76bb9c3acf08dd132abde09425dc3b696bf48f89e5ec1e66826b413",
      "firstHeight": "0x1",
      "lastHeight": "0x3e8"
    }
  },
  "payerAsRecipientTransaction": "0x02f901487d80010283030d4094000000000000000000000000000000000000101a80b8e42e89490f3333333333333333333333333333333333333333333333333333333333333333111111111111111111111111111111111111111111111111111111111111111100000000000000000000000000000000000000000000000000000000000000194444444444444444444444444444444444444444444444444444444444444444000000000000000000000000000000000000000000000000000000000000000122222222222222222222222222222222222222222222222222222222222222225555555555555555555555555555555555555555555555555555555555555555c001a0212e68158f5de8d1f67af0623fa552f00be50c64aaa1b16cb49f1808af79bf3ca011cf1398266f4504793600dde092022c84de71514b23a6896736fdad496411a6"
};

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const word = (value) => new Uint8Array(32).fill(value);
const payer = "11".repeat(32);
const recipient = `0x${"66".repeat(32)}`;
const asset = `0x${"88".repeat(32)}`;
const rawTransaction = fixture.proof.rawTransaction;
const txHash = hex(keccak256(bytes(rawTransaction)));

function offer(scheme = "metered") {
  return {
    x402Version: 2,
    resource: { url: "https://seller.example/paid" },
    accepts: [{
      scheme, network: "eip155:125", amount: "25", asset, payTo: recipient, maxTimeoutSeconds: 120,
      extra: { layerx: {
        commitment: "receipted", payer, purposeHash: "22".repeat(32), contextHash: "55".repeat(32),
        grant: { grantId: "33".repeat(32), asset },
        ...(scheme === "subscription" ? { windowSeconds: "3600" } : {}),
      } },
    }],
  };
}

function buyer(baseUrl = "http://127.0.0.1:1/") {
  const key = fixture.proof.receiptKey;
  return new BuyerMiddleware({
    client: new ProductionClient(new LayerXPaymentHttpTransport({
      baseUrl, bearerToken: new SecretBytes(new TextEncoder().encode("local-fixture-token")),
    })),
    source: "acct:grant-payer",
    supported: ["metered", "subscription"].map((scheme) => ({ scheme, network: "eip155:125" })),
    receiptKeys: [{
      keyId: bytes(key.keyId), publicKey: bytes(key.publicKey),
      first_height: BigInt(key.firstHeight), last_height: BigInt(key.lastHeight),
    }],
  });
}

function settlement(proof = fixture.proof, expectedPayer = payer) {
  return encodeSettlementHeader({
    success: true, network: "eip155:125", amount: "25", payer: expectedPayer, transaction: txHash,
    extensions: { layerx: proof },
  });
}

function refuses(code) {
  return (error) => error instanceof MiddlewareError && error.code === code;
}

function buyerDrawRecipientBindsCalldata() {
  const encoded = encodeLedgerDraw({
    grantId: word(0x33), recipient: bytes(recipient), amount: 25n, idempotencyKey: word(0x44),
    sequence: 1n, purposeHash: word(0x22), contextHash: word(0x55),
  });
  const transaction = decodeRawEip1559(bytes(rawTransaction));
  assert.deepEqual(transaction.data, encoded);
  assert.equal(hex(transaction.data.subarray(36, 68)), recipient);
  assert.notEqual(recipient, `0x${payer}`);
  assert.notEqual(hex(transaction.data.subarray(164, 196)), asset);
  for (const scheme of ["metered", "subscription"]) {
    const required = offer(scheme);
    const accepted = required.accepts[0];
    const header = grantPaymentHeader(required, accepted, rawTransaction);
    assert.deepEqual(decodePaymentPayloadHeader(header), {
      x402Version: 2, resource: required.resource, accepted, extensions: {},
      payload: { rawTransaction, idempotencyKey: "44".repeat(32) },
    });
    assert.throws(() => grantPaymentHeader(required, accepted, fixture.payerAsRecipientTransaction),
      refuses("requirements-mismatch"));
    for (const changes of [{ payTo: `0x${payer}` }, { amount: "24" }, { asset: `0x${"99".repeat(32)}` },
      { extra: { layerx: { ...accepted.extra.layerx, purposeHash: "99".repeat(32) } } },
      { extra: { layerx: { ...accepted.extra.layerx, contextHash: "99".repeat(32) } } },
      { extra: { layerx: { ...accepted.extra.layerx, grant: { ...accepted.extra.layerx.grant, grantId: "99".repeat(32) } } } },
      { extra: { layerx: { ...accepted.extra.layerx, payer: undefined } } }]) {
      const changed = { ...accepted, ...changes };
      assert.throws(() => grantPaymentHeader({ ...required, accepts: [changed] }, changed, rawTransaction),
        refuses("requirements-mismatch"));
    }
  }
}

async function buyerDrawRecipientAuthenticatesGrantPayer() {
  const middleware = buyer();
  const required = offer();
  const header = grantPaymentHeader(required, required.accepts[0], rawTransaction);
  const captured = await middleware.captureGrantSettlement(settlement(), header, txHash);
  assert.equal(hex(captured.verification.payment.from), `0x${payer}`);
  assert.equal(hex(captured.verification.payment.to), recipient);
  for (const proof of [fixture.wrongRecipient, fixture.wrongPayer]) {
    await assert.rejects(middleware.captureGrantSettlement(settlement(proof), header, txHash),
      refuses("verification-failure"));
  }
  // A coherent offer/response payer cannot override the grant's signed debit account.
  const changed = structuredClone(required);
  changed.accepts[0].extra.layerx.payer = recipient.slice(2);
  const changedHeader = grantPaymentHeader(changed, changed.accepts[0], rawTransaction);
  await assert.rejects(middleware.captureGrantSettlement(settlement(fixture.proof, recipient.slice(2)), changedHeader, txHash),
    refuses("verification-failure"));
  await assert.rejects(middleware.captureGrantSettlement(settlement(fixture.proof, recipient.slice(2)), header, txHash),
    refuses("verification-failure"));
  const payload = decodePaymentPayloadHeader(header);
  const wrongKey = encodePaymentPayloadHeader({ ...payload,
    payload: { ...payload.payload, idempotencyKey: "99".repeat(32) } });
  await assert.rejects(middleware.captureGrantSettlement(settlement(), wrongKey, txHash),
    refuses("invalid-payment-payload"));
  const tampered = bytes(fixture.proof.signedReceipt);
  tampered[tampered.length - 1] ^= 1;
  await assert.rejects(middleware.captureGrantSettlement(settlement({ ...fixture.proof,
    signedReceipt: hex(tampered), receiptDigest: hex(keccak256(tampered)),
  }), header, txHash), refuses("verification-failure"));
}

async function buyerDrawRecipientThroughNode(context) {
  const requests = [];
  const paidRequests = [];
  const errors = [];
  let rpc;
  const server = createServer(async (request, response) => {
    try {
      if (request.url === "/") {
        const chunks = [];
        for await (const chunk of request) chunks.push(chunk);
        const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
        requests.push(body);
        assert.equal(request.method, "POST");
        assert.equal(body.jsonrpc, "2.0");
        let result;
        if (body.method === "eth_sendRawTransaction") {
          assert.deepEqual(body.params, [rawTransaction]);
          result = txHash;
        } else {
          assert.equal(body.method, "pax_sequencerReceipt");
          assert.deepEqual(body.params, [txHash]);
          result = { rung: "receipted", txHash, receipt: fixture.proof.signedReceipt,
            keyId: fixture.proof.receiptKey.keyId, height: "0x1" };
        }
        response.writeHead(200, { "content-type": "application/json" });
        response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result }));
        return;
      }
      const scheme = request.url.slice(1);
      assert.ok(["metered", "subscription"].includes(scheme));
      const required = offer(scheme);
      const header = request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()];
      if (header === undefined) {
        response.writeHead(402, { [PAYMENT_REQUIRED_HEADER]: encodePaymentRequiredHeader(required) }).end();
        return;
      }
      const payload = decodePaymentPayloadHeader(header);
      paidRequests.push(payload);
      assert.deepEqual(payload.accepted, required.accepts[0]);
      assert.equal(payload.payload.rawTransaction, rawTransaction);
      assert.equal(payload.payload.idempotencyKey, "44".repeat(32));
      assert.equal(hex(decodeRawEip1559(bytes(payload.payload.rawTransaction)).data.subarray(36, 68)), recipient);
      const sent = await rpc.send(payload.payload.rawTransaction);
      const receipt = await rpc.receipt(sent);
      assert.equal(receipt.rung, "receipted");
      const proof = { ...fixture.proof, rawTransaction: payload.payload.rawTransaction, signedReceipt: receipt.receipt };
      response.writeHead(200, { [PAYMENT_RESPONSE_HEADER]: settlement(proof) }).end("paid");
    } catch (error) {
      errors.push(error);
      response.writeHead(500).end("fixture request failed");
    }
  });
  context.after(async () => {
    const closed = new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const baseUrl = `http://127.0.0.1:${server.address().port}/`;
  rpc = new PaymentRpc(baseUrl);
  const middleware = buyer(baseUrl);
  for (const scheme of ["metered", "subscription"]) {
    const result = await middleware.fetchGrant(`${baseUrl}${scheme}`, {}, rawTransaction, txHash);
    assert.equal(await result.response.text(), "paid");
    assert.equal(result.settlement.txHash, txHash);
    assert.equal(result.settlement.verification.rung, "receipted");
    const payment = result.settlement.verification.payment;
    assert.deepEqual(payment, {
      grantId: word(0x33), from: bytes(`0x${payer}`), to: bytes(recipient), assetId: bytes(asset),
      amount: 25n, idempotencyKey: word(0x44), contextHash: word(0x55),
    });
  }
  assert.deepEqual(errors, []);
  assert.equal(paidRequests.length, 2);
  assert.deepEqual(requests.map(({ method, params }) => ({ method, params })), [
    { method: "eth_sendRawTransaction", params: [rawTransaction] },
    { method: "pax_sequencerReceipt", params: [txHash] },
    { method: "eth_sendRawTransaction", params: [rawTransaction] },
    { method: "pax_sequencerReceipt", params: [txHash] },
  ]);
}

test(buyerDrawRecipientBindsCalldata);
test(buyerDrawRecipientAuthenticatesGrantPayer);
test(buyerDrawRecipientThroughNode);

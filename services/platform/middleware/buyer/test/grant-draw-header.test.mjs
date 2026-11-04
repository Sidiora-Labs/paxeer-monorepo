import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test from "node:test";
import { buyerDrawFixture } from "./support/grant-draw-walkthrough.mjs";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { ProductionClient, SecretBytes, decodeRawEip1559, encodeUnsignedEip1559, keccak256 } from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS, MiddlewareError, PAYMENT_REQUIRED_HEADER, PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER, decodePaymentPayloadHeader, encodeLedgerDraw,
  encodePaymentPayloadHeader, encodePaymentRequiredHeader, encodeSettlementHeader,
} from "@sidiora/layerx-seller-middleware";
import { BuyerMiddleware, LayerXPaymentHttpTransport, grantPaymentHeader } from "../dist/index.js";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (value) => createHash("sha256").update(value).digest();
const word = (value) => Buffer.alloc(32, value);
const draw = {
  from: word(0x66), assetId: word(0x22), amount: 25n, grantId: word(0x33), receiverSequence: 1n,
  idempotencyKey: word(0x44), contextHash: word(0x55),
};
const asset = word(0x88);
const payee = word(0x66);
const payer = word(0x11);
const keyId = word(0x77);
const pair = generateKeyPairSync("ed25519");
const pin = { keyId, publicKey: new Uint8Array(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32)),
  first_height: 1n, last_height: 1000n };
const drawnTopic = keccak256(new TextEncoder().encode("PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)"));

function offer(scheme = "metered") {
  const accepted = {
    scheme, network: "eip155:125", amount: "25", asset: hex(asset), payTo: hex(payee), maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", payer: payer.toString("hex"),
      purposeHash: draw.assetId.toString("hex"), contextHash: draw.contextHash.toString("hex"),
      grant: { grantId: draw.grantId.toString("hex"), asset: hex(asset) },
      ...(scheme === "subscription" ? { windowSeconds: "3600" } : {}) } },
  };
  return { x402Version: 2, resource: { url: "https://seller.example/paid" }, accepts: [accepted],
    extensions: { grant: { info: { version: 1 }, schema: {} } } };
}

function buyer(receiptKeys = [pin]) {
  return new BuyerMiddleware({
    client: new ProductionClient(new LayerXPaymentHttpTransport({
      baseUrl: "http://127.0.0.1:1/",
      bearerToken: new SecretBytes(new TextEncoder().encode("buyer-grant-test-token")),
    })),
    source: "acct:buyer-grant-test", receiptKeys,
    supported: ["metered", "subscription"].map((scheme) => ({ scheme, network: "eip155:125" })),
  });
}

function uint(value, width) {
  assert.ok(value >= 0n && value < 1n << BigInt(width * 8));
  return Buffer.from(value.toString(16).padStart(width * 2, "0"), "hex");
}

function integer(value) {
  if (value === 0n) return Buffer.alloc(0);
  const digits = value.toString(16);
  return Buffer.from(digits.padStart(digits.length + digits.length % 2, "0"), "hex");
}

function rlp(value) {
  const list = Array.isArray(value);
  const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0] < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.of(prefix + payload.length), payload]);
  const length = integer(BigInt(payload.length));
  return Buffer.concat([Buffer.of(prefix + 55 + length.length), length, payload]);
}

function transaction({ data = encodeLedgerDraw(draw), to = bytes(LAYERX_LEDGER_ADDRESS), chainId = 125n, value = 0n } = {}) {
  const input = { chainId, nonce: 0n, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n, gasLimit: 200000n,
    to, value, data, accessList: [] };
  const signature = secp256k1.sign(keccak256(encodeUnsignedEip1559(input)), new Uint8Array(32).fill(1),
    { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  return hex(Buffer.concat([Buffer.of(2), rlp([
    integer(chainId), integer(input.nonce), integer(input.maxPriorityFeePerGas), integer(input.maxFeePerGas),
    integer(input.gasLimit), to, integer(value), data, [], integer(BigInt(signature[0])), integer(parsed.r), integer(parsed.s),
  ])]));
}

function bounded(value) {
  return Buffer.concat([uint(BigInt(value.length), 4), value]);
}

function effect(ordinal, kind, body) {
  return Buffer.concat([uint(12n, 2), uint(BigInt(ordinal), 2), uint(0n, 2), Buffer.of(kind, 0), bounded(Buffer.alloc(32)), bounded(body)]);
}

function signedProof(rawTransaction, { grantId = draw.grantId, count = 1, address = LAYERX_LEDGER_ADDRESS,
  topic = drawnTopic, amount = draw.amount, resultCode = 0n, recipient = payee, contextHash = draw.contextHash,
  idempotencyKey = draw.idempotencyKey, from = payer, paymentAsset = asset,
  receiptFrom = Buffer.from("0000000000000000000000001a642f0e3c3af545e7acbd38b07251b3990914f1", "hex"),
  receiptTo = Buffer.concat([Buffer.alloc(12), bytes(LAYERX_LEDGER_ADDRESS)]) } = {}) {
  const txHash = keccak256(bytes(rawTransaction));
  const logData = Buffer.concat([paymentAsset, uint(amount, 32), idempotencyKey, contextHash]);
  const logBody = Buffer.concat([bytes(address), Buffer.of(4), topic, grantId, from, recipient, sha256(logData)]);
  const summary = Buffer.concat([uint(50000n, 8), uint(50000n, 8), uint(2n, 8), uint(0n, 4), Buffer.alloc(40),
    sha256(Buffer.alloc(0)), uint(0n, 4), uint(BigInt(count), 4), Buffer.of(4), Buffer.from("uhpx")]);
  const effects = [effect(0, 1, summary), ...Array.from({ length: count }, (_, i) => effect(i + 1, 3, logBody))];
  const prefix = Buffer.concat([
    uint(2n, 2), uint(0x5201n, 2), uint(2n, 2), bounded(txHash), uint(1n, 8),
    bounded(Buffer.alloc(32)), bounded(sha256(Buffer.concat([uint(1n, 8), txHash]))), bounded(sha256(Buffer.alloc(0))),
    uint(resultCode, 4), uint(BigInt(effects.length), 4), ...effects,
    uint(100000n, 16), bounded(uint(7n, 32)), uint(12n, 2), uint(1n, 4), uint(0n, 4), Buffer.of(2),
    bounded(sha256(Buffer.from("uhpx"))), uint(0n, 16), bounded(receiptFrom), uint(1000000n, 16), uint(900000n, 16),
    uint(0n, 8), bounded(receiptTo), uint(0n, 16), uint(0n, 16), bounded(Buffer.alloc(32)), bounded(Buffer.alloc(32)),
    bounded(keyId), uint(1n, 8),
  ]);
  const digest = sha256(Buffer.concat([Buffer.from("LXP/v1/receipt\0"), prefix, Buffer.of(0)]));
  const signedReceipt = Buffer.concat([prefix, Buffer.of(1), bounded(sign(null, digest, pair.privateKey))]);
  return { format: "layerx-receipt-proof-v2", verificationLevel: "receipted", rawTransaction,
    signedReceipt: hex(signedReceipt), receiptDigest: hex(keccak256(signedReceipt)),
    receiptKey: { keyId: hex(pin.keyId), publicKey: hex(pin.publicKey), firstHeight: "0x1", lastHeight: "0x3e8" } };
}

function settlement(proof, changes = {}) {
  return encodeSettlementHeader({ success: true, network: "eip155:125", amount: "25", payer: payer.toString("hex"),
    transaction: hex(keccak256(bytes(proof.rawTransaction))), extensions: { layerx: proof }, ...changes });
}

function refuses(code) {
  return (error) => error instanceof MiddlewareError && error.code === code;
}

async function grantHeaderCarriesRawDrawTransaction() {
  const rawTransaction = transaction();
  const decoded = decodeRawEip1559(bytes(rawTransaction));
  assert.equal(hex(decoded.to), LAYERX_LEDGER_ADDRESS);
  assert.deepEqual(new Uint8Array(decoded.data), encodeLedgerDraw(draw));
  assert.notDeepEqual(payer, payee);
  assert.deepEqual(Buffer.from(decoded.data.subarray(36, 68)), payee);
  const middleware = buyer();
  for (const scheme of ["metered", "subscription"]) {
    const required = offer(scheme);
    const header = middleware.grantHeader(encodePaymentRequiredHeader(required), rawTransaction);
    assert.equal(header, grantPaymentHeader(required, required.accepts[0], rawTransaction));
    const payload = decodePaymentPayloadHeader(header);
    assert.deepEqual(payload.accepted, required.accepts[0]);
    assert.deepEqual(payload.resource, required.resource);
    assert.deepEqual(payload.extensions, required.extensions);
    assert.deepEqual(payload.payload, { rawTransaction, idempotencyKey: draw.idempotencyKey.toString("hex") });
  }
  const required = offer();
  const received = [];
  const server = createServer((request, response) => {
    const header = request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()];
    if (header === undefined) {
      response.writeHead(402, { [PAYMENT_REQUIRED_HEADER]: encodePaymentRequiredHeader(required) }).end();
      return;
    }
    const payload = decodePaymentPayloadHeader(header);
    received.push(payload);
    response.writeHead(200, { [PAYMENT_RESPONSE_HEADER]: settlement(signedProof(payload.payload.rawTransaction)) }).end("paid");
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  try {
    const result = await middleware.fetchGrant(`http://127.0.0.1:${server.address().port}/paid`, {}, rawTransaction,
      hex(keccak256(bytes(rawTransaction))));
    assert.equal(await result.response.text(), "paid");
    assert.equal(result.settlement.txHash, hex(keccak256(bytes(rawTransaction))));
    assert.equal(result.settlement.verification.payment.amount, 25n);
    assert.equal(hex(result.settlement.verification.payment.assetId), hex(asset));
    assert.deepEqual(received.map((payload) => payload.payload),
      [{ rawTransaction, idempotencyKey: draw.idempotencyKey.toString("hex") }]);
  } finally {
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

function grantHeaderRefusesAmountMismatch() {
  const required = offer();
  const accepted = required.accepts[0];
  const rawTransaction = transaction({ data: encodeLedgerDraw({ ...draw, amount: 24n }) });
  assert.throws(() => grantPaymentHeader(required, accepted, rawTransaction), refuses("requirements-mismatch"));
  assert.throws(() => buyer().grantHeader(encodePaymentRequiredHeader(required), rawTransaction), refuses("requirements-mismatch"));
  const matching = { ...accepted, amount: "24" };
  assert.equal(decodePaymentPayloadHeader(grantPaymentHeader({ ...required, accepts: [matching] }, matching, rawTransaction))
    .payload.rawTransaction, rawTransaction);
  const badSelector = encodeLedgerDraw(draw);
  badSelector[0] ^= 1;
  const overflow = encodeLedgerDraw(draw);
  overflow[132] = 1;
  const amountOverflow = encodeLedgerDraw(draw);
  amountOverflow[68] = 1;
  for (const invalid of [
    transaction({ to: bytes("0x000000000000000000000000000000000000101b") }),
    transaction({ chainId: 1n }), transaction({ value: 1n }), transaction({ data: badSelector }),
    transaction({ data: overflow }), transaction({ data: amountOverflow }),
    transaction({ data: encodeLedgerDraw(draw).slice(0, -1) }),
    ...[{ from: word(1) }, { assetId: word(1) }, { contextHash: word(1) }]
      .map((changes) => transaction({ data: encodeLedgerDraw({ ...draw, ...changes }) })),
    "ab".repeat(733), "0x02", rawTransaction.slice(2), `${rawTransaction}f`,
  ]) assert.throws(() => grantPaymentHeader(required, accepted, invalid), refuses("requirements-mismatch"));
  assert.throws(() => grantPaymentHeader({ ...required, accepts: [] }, accepted, transaction()), refuses("requirements-mismatch"));
}

async function captureGrantSettlementRequiresDrawHash() {
  const middleware = buyer();
  const rawTransaction = transaction();
  const paymentHeader = middleware.grantHeader(encodePaymentRequiredHeader(offer()), rawTransaction);
  const txHash = hex(keccak256(bytes(rawTransaction)));
  const proof = signedProof(rawTransaction);
  const captured = await middleware.captureGrantSettlement(settlement(proof), paymentHeader, txHash);
  assert.equal(captured.txHash, txHash);
  assert.deepEqual(Buffer.from(captured.verification.payment.from), payer);
  assert.deepEqual(Buffer.from(captured.verification.payment.to), payee);
  assert.equal(captured.rawTransaction, rawTransaction);
  assert.deepEqual(Buffer.from(captured.verification.payment.grantId), draw.grantId);
  assert.deepEqual(Buffer.from(captured.verification.payment.contextHash), draw.contextHash);
  for (const transaction of [hex(word(0xaa)), `lxp:${txHash.slice(2)}`]) {
    await assert.rejects(middleware.captureGrantSettlement(settlement(proof, { transaction }), paymentHeader, txHash),
      refuses("verification-failure"));
  }
  await assert.rejects(middleware.captureGrantSettlement(settlement(proof), paymentHeader, hex(word(0xaa))),
    refuses("verification-failure"));
  for (const invalid of [txHash.slice(2), "0x12", `0x${"gg".repeat(32)}`, `0x${"AA".repeat(32)}`]) {
    await assert.rejects(middleware.captureGrantSettlement(settlement(proof), paymentHeader, invalid), refuses("invalid-payment-payload"));
    await assert.rejects(middleware.fetchGrant("http://127.0.0.1:1/", {}, rawTransaction, invalid), refuses("invalid-payment-payload"));
  }
  for (const changes of [{ grantId: word(0xaa) }, { count: 0 }, { count: 2 }, { amount: 24n },
    { address: "0x000000000000000000000000000000000000101b" }, { topic: word(0xbb) },
    { resultCode: 1n }, { recipient: word(0xcc) }, { contextHash: word(0xcc) },
    { idempotencyKey: word(0xcc) }, { from: word(0xcc) }, { paymentAsset: draw.assetId },
    { receiptFrom: payer }, { receiptFrom: payee }, { receiptTo: payee },
    { receiptTo: Buffer.concat([Buffer.alloc(12), bytes("0x000000000000000000000000000000000000101b")]) }]) {
    await assert.rejects(middleware.captureGrantSettlement(settlement(signedProof(rawTransaction, changes)), paymentHeader, txHash),
      refuses("verification-failure"));
  }
  const changedReceipt = bytes(proof.signedReceipt);
  changedReceipt[changedReceipt.length - 1] ^= 1;
  for (const changedProof of [
    { ...proof, receiptDigest: hex(word(0)) },
    { ...proof, signedReceipt: hex(changedReceipt), receiptDigest: hex(keccak256(changedReceipt)) },
    { ...proof, receiptKey: { ...proof.receiptKey, firstHeight: "0x0" } },
    { ...proof, rawTransaction: transaction({ data: encodeLedgerDraw({ ...draw, receiverSequence: 2n }) }) },
  ]) {
    await assert.rejects(middleware.captureGrantSettlement(settlement(changedProof), paymentHeader, txHash), refuses("verification-failure"));
  }
  await assert.rejects(buyer([{ ...pin, publicKey: word(0) }]).captureGrantSettlement(settlement(proof), paymentHeader, txHash),
    refuses("verification-failure"));
  const payload = decodePaymentPayloadHeader(paymentHeader);
  await assert.rejects(middleware.captureGrantSettlement(settlement(proof), encodePaymentPayloadHeader({ ...payload,
    payload: { rawTransaction, idempotencyKey: word(0xaa).toString("hex") } }), txHash), refuses("invalid-payment-payload"));
}

test(grantHeaderCarriesRawDrawTransaction);
test(grantHeaderRefusesAmountMismatch);
test(captureGrantSettlementRequiresDrawHash);

function grantHeaderUsesPrecompileWordOrder() {
  // precompiles/layerxledger/abi.json: draw(grantId, recipient, amount,
  // idempotencyKey, sequence, purposeHash, contextHash).
  const abi = JSON.parse(readFileSync(new URL("../../../../../precompiles/layerxledger/abi.json", import.meta.url), "utf8"));
  const definition = abi.find((entry) => entry.type === "function" && entry.name === "draw");
  assert.deepEqual(definition.inputs.map((input) => input.name),
    ["grantId", "recipient", "amount", "idempotencyKey", "sequence", "purposeHash", "contextHash"]);
  const values = { grantId: draw.grantId, recipient: draw.from, amount: uint(draw.amount, 32),
    idempotencyKey: draw.idempotencyKey, sequence: uint(draw.receiverSequence, 32),
    purposeHash: draw.assetId, contextHash: draw.contextHash };
  const signature = `draw(${definition.inputs.map((input) => input.type).join(",")})`;
  const selector = keccak256(new TextEncoder().encode(signature)).slice(0, 4);
  const words = definition.inputs.map((input) => values[input.name]);
  const data = Buffer.concat([selector, ...words]);
  assert.deepEqual(encodeLedgerDraw(draw), new Uint8Array(data));
  const required = offer();
  const accepted = required.accepts[0];
  const header = grantPaymentHeader(required, accepted, transaction({ data }));
  assert.equal(decodePaymentPayloadHeader(header).payload.idempotencyKey, draw.idempotencyKey.toString("hex"));
  for (let left = 0; left < words.length; left += 1) {
    for (let right = left + 1; right < words.length; right += 1) {
      const permuted = words.slice();
      [permuted[left], permuted[right]] = [permuted[right], permuted[left]];
      const invalid = transaction({ data: Buffer.concat([selector, ...permuted]) });
      assert.throws(() => grantPaymentHeader(required, accepted, invalid), refuses("requirements-mismatch"),
        `swapped ${definition.inputs[left].name} and ${definition.inputs[right].name}`);
    }
  }
  for (const grant of [undefined, { ...accepted.extra.layerx.grant, grantId: word(0xaa).toString("hex") },
    { ...accepted.extra.layerx.grant, asset: hex(word(0xaa)) }]) {
    const changed = { ...accepted, extra: { layerx: { ...accepted.extra.layerx, grant } } };
    assert.throws(() => grantPaymentHeader({ ...required, accepts: [changed] }, changed, transaction()),
      refuses("requirements-mismatch"));
  }
}

test(grantHeaderUsesPrecompileWordOrder);

test(buyerDrawFixture);

import "./source-loader.mjs";
import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import test from "node:test";
import { readFileSync } from "node:fs";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { decodeRawEip1559, keccak256, recoverEip1559Sender, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { MiddlewareError, ReceiptPayloadAuthority, encodeLedgerDraw, LAYERX_LEDGER_ADDRESS } from "../dist/index.js";
import { GrantPaymentAuthority } from "../dist/grant.js";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (value) => createHash("sha256").update(value).digest();
const word = (value) => Buffer.alloc(32, value);
const draw = {
  recipient: word(0x66), purposeHash: word(0x22), amount: 25n, grantId: word(0x33), sequence: 1n,
  idempotencyKey: word(0x44), contextHash: word(0x55),
};
const payee = word(0x66);
const payer = word(0x11);
const asset = word(0x88);
const wire = JSON.parse(readFileSync(new URL("./grant-draw-wire.json", import.meta.url), "utf8"));
const keyId = word(0x77);
const pair = generateKeyPairSync("ed25519");
const adversarialPin = { keyId, publicKey: new Uint8Array(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32)),
  first_height: 1n, last_height: 1000n };
const pin = { keyId, publicKey: new Uint8Array(bytes(wire.proof.receiptKey.publicKey)), first_height: 1n, last_height: 1000n };
const requirements = { scheme: "metered", network: "eip155:125", amount: "25", asset: hex(asset),
  payTo: hex(payee), maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", payer: hex(payer).slice(2), purposeHash: hex(draw.purposeHash).slice(2) } } };
const drawnTopic = keccak256(new TextEncoder().encode("PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)"));

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

function transaction({ to = LAYERX_LEDGER_ADDRESS, data = encodeLedgerDraw(draw), type = 2,
  chainId = 125n, value = 0n, signed = true } = {}) {
  const fields = [integer(chainId), integer(0n), integer(1n), integer(2n), integer(200000n),
    bytes(to), integer(value), data, []];
  const unsigned = Buffer.concat([Buffer.of(type), rlp(fields)]);
  if (!signed) return hex(unsigned);
  const signature = secp256k1.sign(keccak256(unsigned), new Uint8Array(32).fill(1), { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  return hex(Buffer.concat([Buffer.of(type), rlp([...fields, integer(BigInt(signature[0])), integer(parsed.r), integer(parsed.s)])]));
}

function bounded(value) {
  return Buffer.concat([uint(BigInt(value.length), 4), value]);
}

function effect(ordinal, kind, body) {
  return Buffer.concat([uint(12n, 2), uint(BigInt(ordinal), 2), uint(0n, 2), Buffer.of(kind, 0), bounded(Buffer.alloc(32)), bounded(body)]);
}

function signedProof(rawTransaction, { grantId = draw.grantId, count = 1, address = LAYERX_LEDGER_ADDRESS,
  topic = drawnTopic, amount = draw.amount, resultCode = 0n, recipient = payee } = {}) {
  const txHash = keccak256(bytes(rawTransaction));
  const logData = Buffer.concat([asset, uint(amount, 32), draw.idempotencyKey, draw.contextHash]);
  const logBody = Buffer.concat([bytes(address), Buffer.of(4), topic, grantId, payer, recipient, sha256(logData)]);
  const summary = Buffer.concat([uint(50000n, 8), uint(50000n, 8), uint(2n, 8), uint(0n, 4), Buffer.alloc(40),
    sha256(Buffer.alloc(0)), uint(0n, 4), uint(BigInt(count), 4), Buffer.of(4), Buffer.from("uhpx")]);
  const effects = [effect(0, 1, summary), ...Array.from({ length: count }, (_, i) => effect(i + 1, 3, logBody))];
  const from = Buffer.from("0000000000000000000000001a642f0e3c3af545e7acbd38b07251b3990914f1", "hex");
  const to = Buffer.concat([Buffer.alloc(12), bytes(LAYERX_LEDGER_ADDRESS)]);
  const prefix = Buffer.concat([
    uint(2n, 2), uint(0x5201n, 2), uint(2n, 2), bounded(txHash), uint(1n, 8),
    bounded(Buffer.alloc(32)), bounded(sha256(Buffer.concat([uint(1n, 8), txHash]))), bounded(sha256(Buffer.alloc(0))),
    uint(resultCode, 4), uint(BigInt(effects.length), 4), ...effects,
    uint(100000n, 16), bounded(uint(7n, 32)), uint(12n, 2), uint(1n, 4), uint(0n, 4), Buffer.of(2),
    bounded(sha256(Buffer.from("uhpx"))), uint(0n, 16), bounded(from), uint(1000000n, 16), uint(900000n, 16),
    uint(0n, 8), bounded(to), uint(0n, 16), uint(0n, 16), bounded(Buffer.alloc(32)), bounded(Buffer.alloc(32)),
    bounded(keyId), uint(1n, 8),
  ]);
  const digest = sha256(Buffer.concat([Buffer.from("LXP/v1/receipt\0"), prefix, Buffer.of(0)]));
  const signedReceipt = Buffer.concat([prefix, Buffer.of(1), bounded(sign(null, digest, pair.privateKey))]);
  return { kind: "settled", format: "layerx-receipt-proof-v2", verificationLevel: "receipted", rawTransaction,
    signedReceipt: hex(signedReceipt), receiptDigest: hex(digest),
    receiptKey: { keyId: hex(adversarialPin.keyId), publicKey: hex(adversarialPin.publicKey),
      firstHeight: `0x${pin.first_height.toString(16)}`, lastHeight: `0x${pin.last_height.toString(16)}` } };
}

class GrantDrawExecution {
  inputs = [];
  grants = new Map([[hex(draw.grantId), { source: hex(payer), asset: hex(asset), recipient: hex(payee),
    executor: recoverEip1559Sender(bytes(wire.proof.rawTransaction)), active: true }]]);
  constructor(outcome) { this.outcome = outcome; }
  async resolveGrant(id, network) {
    assert.equal(network, "eip155:125");
    return structuredClone(this.grants.get(id));
  }
  async execute(input) {
    this.inputs.push(input);
    return this.outcome;
  }
}

function request(payload, required = requirements) {
  return { principal: "merchant", requestDigest: "88".repeat(32), idempotencyKey: "99".repeat(32),
    requirements: required, payload: { x402Version: 2, accepted: required, payload } };
}

function payload(rawTransaction = transaction()) {
  return { rawTransaction, idempotencyKey: draw.idempotencyKey.toString("hex") };
}

function refuses(code) {
  return (error) => error instanceof MiddlewareError && error.code === code;
}

async function grantAuthorityForwardsLedgerDrawTransaction() {
  const rawTransaction = transaction();
  const decoded = decodeRawEip1559(new Uint8Array(bytes(rawTransaction)));
  assert.equal(hex(decoded.to), LAYERX_LEDGER_ADDRESS);
  assert.deepEqual(decoded.data, encodeLedgerDraw(draw));
  const outcome = { kind: "settled", ...wire.proof };
  assert.equal(rawTransaction, outcome.rawTransaction);
  const verified = await verifySequencerReceipt(bytes(outcome.signedReceipt), bytes(rawTransaction), [pin]);
  assert.equal(verified.receipt.resultCode, 0);
  assert.deepEqual(Buffer.from(verified.receipt.effects[1].body.subarray(53, 85)), draw.grantId);
  const draws = new GrantDrawExecution(outcome);
  const exact = new GrantDrawExecution(outcome);
  const authority = new GrantPaymentAuthority(draws, { settle: (input) => exact.execute(input) }, [pin]);
  for (const scheme of ["metered", "subscription"]) {
    const input = request(payload(rawTransaction), { ...requirements, scheme });
    assert.equal(await authority.settle(input), outcome);
    assert.notDeepEqual(payer, payee);
    assert.deepEqual(Buffer.from(decoded.data.subarray(36, 68)), payee);
    assert.deepEqual(Buffer.from(decoded.data.subarray(68, 100)), uint(draw.amount, 32));
    assert.deepEqual(Buffer.from(decoded.data.subarray(132, 164)), uint(draw.sequence, 32));
    assert.deepEqual(Buffer.from(decoded.data.subarray(164, 196)), draw.purposeHash);
    assert.deepEqual(draws.inputs.at(-1), { principal: input.principal, requestDigest: input.requestDigest,
      rawTransaction, idempotencyKey: input.payload.payload.idempotencyKey, requirements: input.requirements });
    assert.equal("receive" in draws.inputs.at(-1), false);
  }
  assert.equal(draws.inputs.length, 2);
  assert.equal(exact.inputs.length, 0);
  const exactRequest = request({ receive: "00".repeat(733), idempotencyKey: "44".repeat(32) }, { ...requirements, scheme: "exact" });
  assert.equal(await authority.settle(exactRequest), outcome);
  assert.deepEqual(exact.inputs, [exactRequest]);
  assert.equal(draws.inputs.length, 2);

  for (const changes of [{ grantId: word(0xaa) }, { count: 0 }, { count: 2 }, { amount: 24n },
    { address: "0x000000000000000000000000000000000000101b" }, { topic: word(0xbb) },
    { resultCode: 1n }, { recipient: word(0xcc) }]) {
    const executor = new GrantDrawExecution(signedProof(rawTransaction, changes));
    await assert.rejects(new GrantPaymentAuthority(executor, authority, [pin, adversarialPin]).settle(request(payload(rawTransaction))), refuses("verification-failure"));
    assert.equal(executor.inputs.length, 1);
  }
  const alteredReceipt = bytes(outcome.signedReceipt);
  alteredReceipt[alteredReceipt.length - 1] ^= 1;
  for (const changes of [{ receiptDigest: hex(word(0)) }, { rawTransaction: transaction({ value: 1n }) },
    { signedReceipt: hex(alteredReceipt), receiptDigest: hex(keccak256(alteredReceipt)) }]) {
    await assert.rejects(new GrantPaymentAuthority(new GrantDrawExecution({ ...outcome, ...changes }), authority, [pin])
      .settle(request(payload(rawTransaction))), refuses("verification-failure"));
  }
  for (const result of [{ kind: "pending" }, { kind: "refused", reason: "grant-exhausted" }]) {
    const executor = new GrantDrawExecution(result);
    assert.equal(await new GrantPaymentAuthority(executor, authority, [pin, adversarialPin]).settle(request(payload(rawTransaction))), result);
    assert.equal(executor.inputs.length, 1);
  }
  for (const invalid of [wire.wrongRecipient, wire.wrongPayer]) {
    await assert.rejects(new GrantPaymentAuthority(new GrantDrawExecution({ kind: "settled", ...invalid }), authority, [pin])
      .settle(request(payload(rawTransaction))), refuses("verification-failure"));
  }
}

async function grantAuthorityRefusesDrawToAnotherAddress() {
  const draws = new GrantDrawExecution({ kind: "settled", ...wire.proof });
  const authority = new GrantPaymentAuthority(draws, { async settle() { assert.fail("exact authority must not execute"); } }, [pin]);
  for (const to of ["0x000000000000000000000000000000000000101b", "0x0000000000000000000000000000000000001012", "0x"]) {
    await assert.rejects(authority.settle(request(payload(transaction({ to })))), refuses("invalid-payment-payload"));
  }
  assert.equal(draws.inputs.length, 0);
}

async function grantAuthorityRefusesKernelReceivePayload() {
  const draws = new GrantDrawExecution({ kind: "settled", ...wire.proof });
  const authority = new GrantPaymentAuthority(draws, { async settle() { assert.fail("exact authority must not execute"); } }, [pin]);
  const oldPayload = { receive: "ab".repeat(733), idempotencyKey: draw.idempotencyKey.toString("hex") };
  for (const scheme of ["metered", "subscription"]) {
    await assert.rejects(authority.settle(request(oldPayload, { ...requirements, scheme })), refuses("invalid-payment-payload"));
  }
  const badSelector = encodeLedgerDraw(draw);
  badSelector[0] ^= 1;
  const amountOverflow = encodeLedgerDraw(draw);
  amountOverflow[68] = 1;
  const sequenceOverflow = encodeLedgerDraw(draw);
  sequenceOverflow[132] = 1;
  for (const invalid of [
    { ...payload(), receive: oldPayload.receive },
    payload(transaction({ data: encodeLedgerDraw({ ...draw, amount: 24n }) })),
    { ...payload(), idempotencyKey: "aa".repeat(32) },
    { ...payload(), idempotencyKey: "44".repeat(31) },
    { ...payload(), idempotencyKey: 44 },
    payload(transaction({ data: badSelector })), payload(transaction({ data: amountOverflow })),
    payload(transaction({ data: sequenceOverflow })), payload(transaction({ data: encodeLedgerDraw(draw).slice(0, -1) })),
    payload(transaction({ data: Buffer.concat([encodeLedgerDraw(draw), word(0)]) })),
    payload(transaction({ signed: false })), payload(transaction({ type: 1 })),
    payload(transaction({ value: 1n })), payload(transaction({ chainId: 1n })),
    payload("0x02"), payload(transaction().slice(2)), payload(`${transaction()}f`), payload("0xgg"),
  ]) {
    await assert.rejects(authority.settle(request(invalid)), refuses("invalid-payment-payload"));
  }
  assert.equal(draws.inputs.length, 0);
}

test(grantAuthorityForwardsLedgerDrawTransaction);
test(grantAuthorityRefusesDrawToAnotherAddress);
test(grantAuthorityRefusesKernelReceivePayload);

async function grantWireConsumerRefusesAndRecoversDurableClaims(t) {
  const { mkdtemp, rm } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const { once } = await import("node:events");
  const { SellerMiddleware, encodePaymentPayloadHeader, decodeSettlementHeader, verifyPaymentReceipt } = await import("../dist/index.js");
  const { FileFulfillmentRepository, createPaidApiServer } = await import("../../../examples/paid-api/index.mjs");
  const directory = await mkdtemp(join(tmpdir(), "seller-grant-wire-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const accepted = requirements;
  const paymentRequired = { x402Version: 2, resource: { url: "https://merchant.example/paid" }, accepts: [accepted] };
  const paymentHeader = encodePaymentPayloadHeader({ x402Version: 2, accepted, resource: paymentRequired.resource, payload: payload() });
  const settled = { kind: "settled", ...wire.proof };
  const draws = new GrantDrawExecution(settled);
  const make = (state) => new SellerMiddleware({ paymentRequired, receiptKeys: [pin],
    authority: new GrantPaymentAuthority(draws, new ReceiptPayloadAuthority([pin]), [pin]),
    fulfillments: new FileFulfillmentRepository(state) });
  let server;
  const stop = async () => {
    const closing = new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    server.closeAllConnections(); await closing;
  };
  t.after(async () => { if (server?.listening) await stop(); });
  const serve = async (middleware) => {
    server = createPaidApiServer(middleware, { report: "grant-paid" });
    server.listen(0, "127.0.0.1"); await once(server, "listening");
    return `http://127.0.0.1:${server.address().port}/paid`;
  };
  let endpoint = await serve(make(join(directory, "settled")));
  const call = () => fetch(endpoint, { headers: { "PAYMENT-SIGNATURE": paymentHeader } });
  let response = await call();
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { report: "grant-paid" });
  const settlement = decodeSettlementHeader(response.headers.get("PAYMENT-RESPONSE"));
  const payment = await verifyPaymentReceipt(settlement.extensions.layerx, accepted, [pin]);
  assert.deepEqual(Buffer.from(payment.payment.from), payer);
  assert.deepEqual(Buffer.from(payment.payment.to), payee);
  assert.notDeepEqual(payment.payment.from, payment.payment.to);
  assert.deepEqual(Buffer.from(payment.payment.assetId), asset);
  assert.deepEqual(Buffer.from(decodeRawEip1559(bytes(wire.proof.rawTransaction)).data.subarray(164, 196)), draw.purposeHash);
  await stop(); endpoint = await serve(make(join(directory, "settled")));
  response = await call();
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { report: "grant-paid" });
  assert.deepEqual(decodeSettlementHeader(response.headers.get("PAYMENT-RESPONSE")), settlement);
  await stop();

  // An interrupted fulfillment remains uncertain across a fresh server and repository.
  const uncertain = join(directory, "uncertain");
  let releases = 0;
  await assert.rejects(make(uncertain).handle("public-paid-api", paymentHeader, async () => {
    releases++; throw new Error("delivery acknowledgement lost");
  }), /delivery acknowledgement lost/u);
  endpoint = await serve(make(uncertain));
  response = await call();
  assert.equal(response.status, 503);
  assert.deepEqual(await response.json(), { state: "unknown", error: "fulfillment-outcome-unknown" });
  assert.equal(response.headers.get("retry-after"), "1");
  assert.equal(releases, 1);
  await stop();
  endpoint = await serve(make(join(directory, "unavailable")));
  draws.grants.clear();
  response = await call();
  assert.equal(response.status, 402);
  assert.deepEqual(await response.json(), { state: "refused" });
  assert.equal(decodeSettlementHeader(response.headers.get("PAYMENT-RESPONSE")).errorReason, "grant_unavailable");
}

test(grantWireConsumerRefusesAndRecoversDurableClaims);

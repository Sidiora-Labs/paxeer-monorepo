import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { decodeRawEip1559, keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS,
  MiddlewareError,
  ReceiptPayloadAuthority,
  verifyPaymentReceipt,
} from "../dist/index.js";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (value) => createHash("sha256").update(value).digest();
const word = (value) => Buffer.alloc(32, value);
const asset = word(0x11);
const payee = word(0x22);
const payer = word(0x33);
const reference = word(0x44);
const keyId = word(0x55);
const pair = generateKeyPairSync("ed25519");
const publicKey = new Uint8Array(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32));
const pin = { keyId: new Uint8Array(keyId), publicKey, first_height: 1n, last_height: 1000n };
const requirements = {
  scheme: "exact", network: "eip155:125", amount: "25", asset: hex(asset), payTo: hex(payee),
  maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", payer: payer.toString("hex"), purposeHash: reference.toString("hex") } },
};
const exactTopic = keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"));

function uint(value, width) {
  assert.ok(value >= 0n && value < 1n << BigInt(width * 8));
  return Buffer.from(value.toString(16).padStart(width * 2, "0"), "hex");
}

function bounded(value) {
  return Buffer.concat([uint(BigInt(value.length), 4), value]);
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

function transaction(amount) {
  const selector = keccak256(new TextEncoder().encode("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")).slice(0, 4);
  const data = Buffer.concat([selector, payer, payee, asset, uint(amount, 32), uint(1n, 32), reference]);
  const fields = [integer(125n), integer(0n), integer(1n), integer(2n), integer(200000n),
    bytes(LAYERX_LEDGER_ADDRESS), integer(0n), data, []];
  const unsigned = Buffer.concat([Buffer.of(2), rlp(fields)]);
  const signature = secp256k1.sign(keccak256(unsigned), new Uint8Array(32).fill(1), { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  const raw = Buffer.concat([Buffer.of(2), rlp([...fields, integer(BigInt(signature[0])), integer(parsed.r), integer(parsed.s)])]);
  const decoded = decodeRawEip1559(raw);
  assert.equal(decoded.chainId, 125n);
  assert.deepEqual(Buffer.from(decoded.data), data);
  return raw;
}

function effect(ordinal, kind, body) {
  return Buffer.concat([uint(12n, 2), uint(BigInt(ordinal), 2), uint(0n, 2), Buffer.of(kind, 0), bounded(Buffer.alloc(32)), bounded(body)]);
}

function signedProof({ amount = 25n, height = 7n, signingPin = pin, paymentReference = reference,
  paymentTo = payee, logAddress = LAYERX_LEDGER_ADDRESS, count = 1, resultCode = 0n } = {}) {
  const rawTransaction = transaction(amount);
  const txHash = keccak256(rawTransaction);
  const logData = Buffer.concat([asset, uint(amount, 32)]);
  const logBody = Buffer.concat([bytes(logAddress), Buffer.of(4), exactTopic, paymentReference, payer, paymentTo, sha256(logData)]);
  const summary = Buffer.concat([
    uint(50000n, 8), uint(50000n, 8), uint(2n, 8), uint(0n, 4), Buffer.alloc(40), sha256(Buffer.alloc(0)),
    uint(0n, 4), uint(BigInt(count), 4), Buffer.of(4), Buffer.from("uhpx"),
  ]);
  const effects = [effect(0, 1, summary), ...Array.from({ length: count }, (_, i) => effect(i + 1, 3, logBody))];
  const from = Buffer.from("0000000000000000000000001a642f0e3c3af545e7acbd38b07251b3990914f1", "hex");
  const to = Buffer.concat([Buffer.alloc(12), bytes(LAYERX_LEDGER_ADDRESS)]);
  const prefix = Buffer.concat([
    uint(2n, 2), uint(0x5201n, 2), uint(2n, 2), bounded(txHash), uint(1n, 8),
    bounded(Buffer.alloc(32)), bounded(sha256(Buffer.concat([uint(1n, 8), txHash]))), bounded(sha256(Buffer.alloc(0))),
    uint(resultCode, 4), uint(BigInt(effects.length), 4), ...effects,
    uint(100000n, 16), bounded(uint(height, 32)), uint(12n, 2), uint(1n, 4), uint(0n, 4), Buffer.of(2),
    bounded(sha256(Buffer.from("uhpx"))), uint(0n, 16), bounded(from), uint(1000000n, 16), uint(900000n, 16),
    uint(0n, 8), bounded(to), uint(0n, 16), uint(0n, 16), bounded(Buffer.alloc(32)), bounded(Buffer.alloc(32)),
    bounded(keyId), uint(1n, 8),
  ]);
  const unsignedReceipt = Buffer.concat([prefix, Buffer.of(0)]);
  const signingDigest = sha256(Buffer.concat([Buffer.from("LXP/v1/receipt\0"), unsignedReceipt]));
  const signedReceipt = Buffer.concat([prefix, Buffer.of(1), bounded(sign(null, signingDigest, pair.privateKey))]);
  return {
    format: "layerx-receipt-proof-v2", verificationLevel: "receipted", rawTransaction: hex(rawTransaction),
    signedReceipt: hex(signedReceipt), receiptDigest: hex(keccak256(signedReceipt)),
    receiptKey: { keyId: hex(signingPin.keyId), publicKey: hex(signingPin.publicKey),
      firstHeight: `0x${signingPin.first_height.toString(16)}`, lastHeight: `0x${signingPin.last_height.toString(16)}` },
  };
}

function refuses(code) {
  return (error) => error instanceof MiddlewareError && error.code === code;
}

function request(proof) {
  return { principal: "merchant", payload: { x402Version: 2, accepted: requirements, payload: proof },
    requirements, idempotencyKey: "test-payment", requestDigest: "test-request" };
}

async function verifyPaymentReceiptAcceptsSignedSequencerReceipt() {
  const proof = signedProof();
  const settled = await new ReceiptPayloadAuthority([pin]).settle(request(proof));
  assert.equal(settled.kind, "settled");
  assert.equal(settled.rawTransaction, proof.rawTransaction);
  assert.equal(settled.signedReceipt, proof.signedReceipt);
  assert.deepEqual(settled.receiptKey, proof.receiptKey);
  const verified = await verifyPaymentReceipt(proof, requirements, [pin]);
  assert.equal(verified.rung, "receipted");
  assert.equal(verified.height, 7n);
  assert.equal(verified.txHash, hex(keccak256(bytes(proof.rawTransaction))));
  assert.equal(verified.payment.amount, 25n);
  assert.deepEqual(Buffer.from(verified.payment.assetId), asset);
  assert.deepEqual(Buffer.from(verified.payment.to), payee);
  assert.deepEqual(Buffer.from(verified.payment.paymentReference), reference);
  for (const height of [1n, 1000n]) {
    assert.equal((await verifyPaymentReceipt(signedProof({ height }), requirements, [pin])).height, height);
  }
  for (const changes of [{ count: 0 }, { count: 2 }, { paymentReference: word(0x66) },
    { paymentTo: word(0x77) }, { logAddress: "0x000000000000000000000000000000000000101b" }, { resultCode: 1n }]) {
    await assert.rejects(verifyPaymentReceipt(signedProof(changes), requirements, [pin]), refuses("verification-failure"));
  }
  for (const malformed of [{ ...proof, format: "layerx-receipt-proof-v1" }, { ...proof, verificationLevel: "sequencer-signed" },
    { ...proof, extra: true }, { ...proof, receiptKey: { ...proof.receiptKey, extra: true } }]) {
    await assert.rejects(new ReceiptPayloadAuthority([pin]).settle(request(malformed)), refuses("invalid-payment-payload"));
  }
  await assert.rejects(verifyPaymentReceipt(proof, requirements, [{ ...pin, publicKey: word(0x88) }]), refuses("verification-failure"));
  await assert.rejects(verifyPaymentReceipt(proof, { ...requirements, asset: hex(word(0x99)) }, [pin]),
    refuses("verification-failure"));
  const changedTransaction = bytes(proof.rawTransaction);
  changedTransaction[changedTransaction.length - 1] ^= 1;
  await assert.rejects(verifyPaymentReceipt({ ...proof, rawTransaction: hex(changedTransaction) }, requirements, [pin]), refuses("verification-failure"));
  const changedReceipt = bytes(proof.signedReceipt);
  changedReceipt[changedReceipt.length - 1] ^= 1;
  await assert.rejects(verifyPaymentReceipt({ ...proof, signedReceipt: hex(changedReceipt), receiptDigest: hex(keccak256(changedReceipt)) },
    requirements, [pin]), refuses("verification-failure"));
}

async function verifyPaymentReceiptRefusesKeyOutsideHeightRange() {
  for (const outside of [{ ...pin, first_height: 8n }, { ...pin, last_height: 6n },
    { ...pin, first_height: 100n, last_height: 200n }]) {
    const proof = signedProof({ signingPin: outside });
    await assert.rejects(verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [outside]),
      (error) => error.code === "verification-failure" && error.check === "key_out_of_range");
    await assert.rejects(verifyPaymentReceipt(proof, requirements, [outside]), refuses("verification-failure"));
    const widened = { ...proof, receiptKey: { ...proof.receiptKey, firstHeight: "0x1", lastHeight: "0x3e8" } };
    await assert.rejects(verifyPaymentReceipt(widened, requirements, [outside]), refuses("verification-failure"));
  }
}

async function verifyPaymentReceiptRefusesAmountMismatch() {
  const proof = signedProof({ amount: 24n });
  const valid = await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [pin]);
  assert.equal(valid.height, 7n);
  assert.equal((await verifyPaymentReceipt(proof, { ...requirements, amount: "24" }, [pin])).payment.amount, 24n);
  await assert.rejects(verifyPaymentReceipt(proof, requirements, [pin]), refuses("verification-failure"));
}

async function receiptPayloadAuthorityRefusesDigestMismatch() {
  const proof = signedProof();
  const authority = new ReceiptPayloadAuthority([pin]);
  assert.equal((await authority.settle(request(proof))).kind, "settled");
  for (const index of [0, 31]) {
    const badDigest = bytes(proof.receiptDigest);
    badDigest[index] ^= 1;
    await assert.rejects(authority.settle(request({ ...proof, receiptDigest: hex(badDigest) })), refuses("verification-failure"));
  }
  await assert.rejects(authority.settle(request({ ...proof, receiptDigest: hex(sha256(bytes(proof.signedReceipt))) })),
    refuses("verification-failure"));
}

test(verifyPaymentReceiptAcceptsSignedSequencerReceipt);
test(verifyPaymentReceiptRefusesKeyOutsideHeightRange);
test(verifyPaymentReceiptRefusesAmountMismatch);
test(receiptPayloadAuthorityRefusesDigestMismatch);

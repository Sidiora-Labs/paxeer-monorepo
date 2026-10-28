import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import test from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { decodeRawEip1559, keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS,
  MiddlewareError,
  PAYMENT_RESPONSE_HEADER,
  SellerMiddleware,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
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

const secrets = [1, 2, 3, 4].map((value) => new Uint8Array(32).fill(value));
const power = 1n << 54n;

function committedFixture(verified, height = verified.height, validatorCount = 4, signerCount = 3) {
  const validatorSet = secrets.slice(0, validatorCount).map((secret) => {
    const publicKey = ed25519.getPublicKey(secret);
    return {
      publicKey,
      address: Uint8Array.from(createHash("sha256").update(publicKey).digest().subarray(0, 20)),
      votingPower: power,
    };
  });
  const hash = new Uint8Array(32).fill(0x11);
  const partsHash = new Uint8Array(32).fill(0x22);
  const blockId = { hash, partSetHeader: { total: 1, hash: partsHash } };
  const heightBytes = Buffer.alloc(8);
  heightBytes.writeBigInt64LE(height);
  const signBytes = Buffer.concat([
    Buffer.from("65080211", "hex"), heightBytes,
    Buffer.from("22480a20", "hex"), hash,
    Buffer.from("122408011220", "hex"), partsHash,
    Buffer.from("2a020801320a7061786565722d313235", "hex"),
  ]);
  const signatures = validatorSet.slice(0, signerCount).map((validator, index) => {
    const signature = ed25519.sign(signBytes, secrets[index]);
    assert.equal(ed25519.verify(signature, signBytes, validator.publicKey, { zip215: false }), true);
    return {
      blockIdFlag: 2,
      validatorAddress: validator.address,
      timestamp: { seconds: 1n, nanos: 0 },
      signature,
    };
  });
  return {
    landingBlockHeight: height,
    blockHeader: { chainId: "paxeer-125", blockId },
    receiptInclusion: {
      transactionHash: verified.receipt.activityId.slice(),
      receiptDigest: verified.receiptDigest.slice(),
      sequence: verified.receipt.globalSequence,
    },
    commit: { height, round: 0, blockId, signatures },
    validatorSet,
  };
}

class CountingReceiptPayloadAuthority extends ReceiptPayloadAuthority {
  settleCount = 0;

  async settle(request) {
    this.settleCount += 1;
    return super.settle(request);
  }
}

class InMemoryFulfillmentRepository {
  records = new Map();
  inFlight = new Map();

  async get(idempotencyKey) {
    return structuredClone(this.records.get(idempotencyKey));
  }

  async fulfill(proposed, release) {
    const current = this.records.get(proposed.idempotencyKey);
    if (current !== undefined) {
      if (current.requestDigest !== proposed.requestDigest) throw new MiddlewareError("fulfillment-conflict");
      return structuredClone(current);
    }
    const pending = this.inFlight.get(proposed.idempotencyKey);
    if (pending !== undefined) {
      await pending;
      return this.fulfill(proposed, release);
    }
    const fulfillment = (async () => {
      const resource = await release();
      const stored = structuredClone({ ...proposed, resource });
      this.records.set(proposed.idempotencyKey, stored);
      return structuredClone(stored);
    })();
    this.inFlight.set(proposed.idempotencyKey, fulfillment);
    try {
      return await fulfillment;
    } finally {
      this.inFlight.delete(proposed.idempotencyKey);
    }
  }
}

class CommitmentEvidenceRepository {
  records = new Map();

  async resolve(transactionHash, network) {
    const evidence = this.records.get(`${network}/${transactionHash}`);
    if (evidence === undefined) throw new MiddlewareError("verification-failure");
    return structuredClone(evidence);
  }
}

function sellerFixture(proof = signedProof(), terms = requirements, commitments) {
  const authority = new CountingReceiptPayloadAuthority([pin]);
  const fulfillments = new InMemoryFulfillmentRepository();
  const paymentRequired = {
    x402Version: 2,
    resource: { url: "https://merchant.example/paid" },
    accepts: [terms],
  };
  const header = encodePaymentPayloadHeader({
    x402Version: 2, accepted: terms, payload: proof, resource: paymentRequired.resource,
  });
  let releaseCount = 0;
  const releaseKeys = [];
  const config = { paymentRequired, receiptKeys: [pin], authority, fulfillments,
    ...(commitments === undefined ? {} : { commitments }) };
  return {
    proof, authority, fulfillments, header, config,
    seller: new SellerMiddleware(config),
    releaseCount: () => releaseCount,
    releaseKeys,
    release: async (idempotencyKey) => {
      releaseCount += 1;
      releaseKeys.push(idempotencyKey);
      return { access: "paid-resource", idempotencyKey };
    },
  };
}

function response(decision) {
  assert.equal(decision.kind, "released");
  assert.equal(decision.status, 200);
  const settlement = decodeSettlementHeader(decision.headers[PAYMENT_RESPONSE_HEADER]);
  assert.deepEqual(settlement, decision.settlement);
  return settlement;
}

async function settlementResponseNamesTransactionHash() {
  const fixture = sellerFixture();
  const decision = await fixture.seller.handle("merchant", fixture.header, fixture.release);
  const settlement = response(decision);
  const txHash = hex(keccak256(bytes(fixture.proof.rawTransaction)));
  assert.equal(settlement.success, true);
  assert.match(settlement.transaction, /^0x[0-9a-f]{64}$/u);
  assert.equal(settlement.transaction, txHash);
  assert.equal(settlement.network, "eip155:125");
  assert.equal(settlement.amount, "25");
  assert.equal(settlement.payer, hex(decision.verification.receipt.from).slice(2));
  assert.equal(decision.verification.txHash, txHash);
  assert.equal(fixture.authority.settleCount, 1);
  assert.equal(fixture.releaseCount(), 1);
  assert.equal(decision.resource.access, "paid-resource");
  const stored = await fixture.fulfillments.get(fixture.releaseKeys[0]);
  assert.equal(stored.txHash, txHash);
  assert.equal(stored.rawTransaction, fixture.proof.rawTransaction);
  assert.equal(stored.signedReceipt, fixture.proof.signedReceipt);
  assert.deepEqual(stored.receiptKey, fixture.proof.receiptKey);
}

async function settlementResponseCarriesPortableReceipt() {
  const fixture = sellerFixture();
  const decision = await fixture.seller.handle("merchant", fixture.header, fixture.release);
  const portable = response(decision).extensions.layerx;
  assert.equal(portable.format, "layerx-receipt-proof-v2");
  assert.equal(portable.verificationLevel, "receipted");
  assert.equal(portable.rawTransaction, fixture.proof.rawTransaction);
  assert.deepEqual(portable, fixture.proof);
  const verified = await verifyPaymentReceipt(portable, requirements, [pin]);
  assert.equal(verified.rung, "receipted");
  assert.equal(verified.txHash, decision.settlement.transaction);

  const proof = { ...signedProof(), verificationLevel: "committed" };
  const terms = { ...requirements, extra: { layerx: { ...requirements.extra.layerx, commitment: "committed" } } };
  const signed = await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [pin]);
  const evidence = committedFixture(signed);
  const commitments = new CommitmentEvidenceRepository();
  const txHash = hex(keccak256(bytes(proof.rawTransaction)));
  commitments.records.set(`eip155:125/${txHash}`, evidence);
  const committed = sellerFixture(proof, terms, commitments);
  const committedDecision = await committed.seller.handle("merchant", committed.header, committed.release);
  const committedPortable = response(committedDecision).extensions.layerx;
  assert.equal(committedPortable.verificationLevel, "committed");
  assert.equal(committedDecision.verification.rung, "committed");
  assert.equal(committedPortable.rawTransaction, proof.rawTransaction);
  assert.equal(committedPortable.signedReceipt, proof.signedReceipt);
  const wireEvidence = JSON.parse(JSON.stringify(evidence, (_key, value) => {
    if (value instanceof Uint8Array) return hex(value);
    if (typeof value === "bigint") return `0x${value.toString(16)}`;
    return value;
  }));
  for (const field of ["landingBlockHeight", "blockHeader", "receiptInclusion", "commit", "validatorSet"]) {
    assert.deepEqual(committedPortable[field], wireEvidence[field]);
  }
  assert.equal(committedPortable.receiptInclusion.transactionHash, txHash);
  assert.equal(committedPortable.receiptInclusion.receiptDigest, hex(signed.receiptDigest));
  assert.equal(committedPortable.receiptDigest, proof.receiptDigest);
  assert.equal(committedPortable.receiptDigest, hex(keccak256(bytes(committedPortable.signedReceipt))));
  assert.equal(committedPortable.commit.signatures.length, 3);
  assert.equal(committedPortable.validatorSet.length, 4);

  commitments.records.clear();
  const replay = await new SellerMiddleware(committed.config).handle("merchant", committed.header, committed.release);
  assert.deepEqual(response(replay), committedDecision.settlement);
  assert.equal(committed.authority.settleCount, 1);
  assert.equal(committed.releaseCount(), 1);

  const unproven = sellerFixture(proof, terms, commitments);
  await assert.rejects(unproven.seller.handle("merchant", unproven.header, unproven.release),
    (error) => error instanceof MiddlewareError && error.code === "verification-failure");
  assert.equal(unproven.releaseCount(), 0);
  const downgraded = sellerFixture({ ...proof, verificationLevel: "receipted" }, terms, commitments);
  await assert.rejects(downgraded.seller.handle("merchant", downgraded.header, downgraded.release),
    (error) => error instanceof MiddlewareError && error.code === "verification-failure");
  assert.equal(downgraded.releaseCount(), 0);
}

async function replayedSettlementAnswersStoredTransaction() {
  const fixture = sellerFixture();
  const first = await fixture.seller.handle("merchant", fixture.header, fixture.release);
  const second = await fixture.seller.handle("merchant", fixture.header, fixture.release);
  const restarted = new SellerMiddleware(fixture.config);
  const third = await restarted.handle("merchant", fixture.header, fixture.release);
  const txHash = hex(keccak256(bytes(fixture.proof.rawTransaction)));
  assert.equal(response(first).transaction, txHash);
  assert.equal(response(second).transaction, txHash);
  assert.equal(response(third).transaction, txHash);
  assert.deepEqual(second.settlement, first.settlement);
  assert.deepEqual(third.headers, first.headers);
  assert.deepEqual(third.resource, first.resource);
  assert.equal(fixture.authority.settleCount, 1);
  assert.equal(fixture.releaseCount(), 1);
  assert.equal(fixture.fulfillments.records.size, 1);

  const key = fixture.releaseKeys[0];
  const stored = await fixture.fulfillments.get(key);
  assert.equal(stored.txHash, txHash);
  assert.equal(stored.resource.idempotencyKey, key);
  fixture.fulfillments.records.set(key, { ...stored, txHash: hex(word(0xff)) });
  await assert.rejects(restarted.handle("merchant", fixture.header, fixture.release),
    (error) => error instanceof MiddlewareError && error.code === "fulfillment-conflict");
  fixture.fulfillments.records.set(key, { ...stored, requestDigest: "wrong-request" });
  await assert.rejects(restarted.handle("merchant", fixture.header, fixture.release),
    (error) => error instanceof MiddlewareError && error.code === "fulfillment-conflict");
  assert.equal(fixture.authority.settleCount, 1);
  assert.equal(fixture.releaseCount(), 1);
}

test(settlementResponseNamesTransactionHash);
test(settlementResponseCarriesPortableReceipt);
test(replayedSettlementAnswersStoredTransaction);

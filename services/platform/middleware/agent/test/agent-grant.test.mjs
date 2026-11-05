import "./production-sources.mjs";
import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import test from "node:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { BudgetStore, temporaryDirectory, loopback } from "./repair-support.mjs";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { decodeRawEip1559, keccak256 } from "@sidiora/layerx-sdk";
import { decodePaymentDrawnLog, encodeLedgerDraw, LAYERX_LEDGER_ADDRESS, ReceiptPayloadAuthority, PaymentRpc, PaymentRpcError } from "@sidiora/layerx-seller-middleware";
const { AgentGrantMiddleware, AgentMiddlewareError } = await import("../src/index.ts");

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (value) => createHash("sha256").update(value).digest();
const word = (value) => Buffer.alloc(32, value);
const draw = {
  recipient: word(0x66), purposeHash: word(0x99), amount: 40n, grantId: word(0x33), sequence: 1n,
  idempotencyKey: word(0x44), contextHash: word(0x55),
};
const payee = word(0x66);
const payer = word(0x11);
const grantAsset = word(0x22);
const keyId = word(0x77);
const pair = generateKeyPairSync("ed25519");
const pin = { keyId, publicKey: new Uint8Array(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32)),
  first_height: 1n, last_height: 1000n };
const requirements = { scheme: "metered", network: "eip155:125", amount: "40", asset: hex(grantAsset),
  payTo: hex(payee), maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", payer: payer.toString("hex"), purposeHash: draw.purposeHash.toString("hex") } } };
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
  topic = drawnTopic, amount = draw.amount, resultCode = 0n, recipient = payee,
  idempotencyKey = draw.idempotencyKey, contextHash = draw.contextHash, assetId = grantAsset, source = payer } = {}) {
  const txHash = keccak256(bytes(rawTransaction));
  const logData = Buffer.concat([assetId, uint(amount, 32), idempotencyKey, contextHash]);
  const logBody = Buffer.concat([bytes(address), Buffer.of(4), topic, grantId, source, recipient, sha256(logData)]);
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
    signedReceipt: hex(signedReceipt), receiptDigest: hex(keccak256(signedReceipt)),
    receiptKey: { keyId: hex(pin.keyId), publicKey: hex(pin.publicKey),
      firstHeight: `0x${pin.first_height.toString(16)}`, lastHeight: `0x${pin.last_height.toString(16)}` } };
}

class AgentBudgetLedger {
  reservations = new Map();
  transitions = [];

  constructor(available = 100n) { this.available = available; }

  async reserve(request) {
    const key = `${request.tenant}:${request.idempotencyKey}`;
    const previous = this.reservations.get(key);
    if (previous !== undefined) {
      if (previous.requestDigest !== request.requestDigest || previous.amount !== request.amount || previous.asset !== request.asset) {
        return { kind: "conflict" };
      }
      return { kind: "reserved", reservation: previous };
    }
    if (BigInt(request.amount) > this.available) return { kind: "exhausted", available: this.available.toString() };
    this.available -= BigInt(request.amount);
    const reservation = Object.freeze({ state: "reserved", reservationId: key, requestDigest: request.requestDigest,
      amount: request.amount, asset: request.asset });
    this.reservations.set(key, reservation);
    return { kind: "reserved", reservation };
  }

  transition(transition, state) {
    const previous = this.reservations.get(transition.reservationId);
    assert.ok(previous, "transition requires a reservation");
    assert.equal(previous.requestDigest, transition.requestDigest);
    assert.equal(previous.amount, transition.amount);
    assert.equal(previous.asset, transition.asset);
    assert.ok(previous.state === "reserved" || previous.state === "held");
    const reservation = Object.freeze({ ...transition, state });
    this.reservations.set(transition.reservationId, reservation);
    this.transitions.push(reservation);
    return reservation;
  }

  async hold(transition) { return this.transition(transition, "held"); }
  async commit(transition) { return this.transition(transition, "committed"); }
  async release(transition) {
    const reservation = this.transition(transition, "released");
    this.available += BigInt(reservation.amount);
    return reservation;
  }
}

class AgentReceiptRepository {
  records = [];

  constructor(proof) { this.proofs = new Map([[proof.receiptDigest.slice(2), proof]]); }

  async resolve(digest) {
    const proof = this.proofs.get(digest);
    assert.ok(proof, "receipt digest must be retained");
    return structuredClone(proof);
  }

  async retainVerified(record) {
    assert.ok(this.proofs.has(record.receiptDigest));
    this.records.push(structuredClone(record));
  }
}

class ReceiptDrawExecution {
  inputs = [];

  constructor(proof) {
    const { kind, ...evidence } = proof;
    assert.equal(kind, "settled");
    this.proof = evidence;
    this.authority = new ReceiptPayloadAuthority([pin]);
  }

  async execute(request) {
    this.inputs.push(structuredClone(request));
    try {
      return await this.authority.settle({ principal: request.principal, requestDigest: request.requestDigest,
        idempotencyKey: request.idempotencyKey, requirements: request.requirements,
        payload: { x402Version: 2, accepted: request.requirements, payload: this.proof } });
    } catch (error) {
      assert.equal(error.code, "verification-failure");
      throw new AgentMiddlewareError("verification-failure");
    }
  }
}

function request(rawTransaction = transaction(), scheme = "metered") {
  return { principal: "merchant", requestDigest: "88".repeat(32), idempotencyKey: draw.idempotencyKey.toString("hex"),
    rawTransaction, requirements: { ...requirements, scheme } };
}

function setup(proof = signedProof(transaction()), receiptKeys = [pin], available = 100n) {
  const budgets = new AgentBudgetLedger(available);
  const receipts = new AgentReceiptRepository(proof);
  const draws = new ReceiptDrawExecution(proof);
  const middleware = new AgentGrantMiddleware({ tenant: "agent-grant", budgets, receipts, draws, receiptKeys,
    grants: { async resolve({ network, grantId }) {
      return { network, grantId, payer: payer.toString("hex"), asset: grantAsset.toString("hex") };
    } } });
  return { middleware, budgets, receipts, draws };
}

function refuses(code) {
  return (error) => error instanceof AgentMiddlewareError && error.code === code;
}

async function agentGrantSettlesDrawWithPaymentDrawnLog() {
  const input = request();
  const proof = signedProof(input.rawTransaction);
  const event = decodePaymentDrawnLog({ address: LAYERX_LEDGER_ADDRESS,
    topics: [drawnTopic, draw.grantId, payer, payee],
    data: Buffer.concat([grantAsset, uint(40n, 32), draw.idempotencyKey, draw.contextHash]) });
  assert.equal(event.amount, 40n);
  assert.deepEqual(Buffer.from(event.from), payer);
  assert.deepEqual(Buffer.from(event.to), payee);
  assert.notDeepEqual(payer, payee);
  assert.notDeepEqual(grantAsset, draw.purposeHash);
  assert.equal(Buffer.from(event.idempotencyKey).toString("hex"), input.idempotencyKey);
  for (const scheme of ["metered", "subscription"]) {
    const { middleware, budgets, receipts, draws } = setup(proof);
    const grantRequest = { ...input, requirements: { ...input.requirements, scheme } };
    const outcome = await middleware.execute(grantRequest);
    assert.equal(outcome.kind, "settled");
    assert.deepEqual(outcome, proof);
    assert.deepEqual(draws.inputs, [grantRequest]);
    assert.equal(budgets.available, 60n);
    assert.equal(budgets.transitions.length, 1);
    assert.equal(budgets.transitions[0].state, "committed");
    assert.equal(budgets.transitions[0].amount, "40");
    assert.equal(budgets.transitions[0].asset, grantAsset.toString("hex"));
    assert.equal(budgets.transitions[0].receiptDigest, proof.receiptDigest.slice(2));
    assert.deepEqual(receipts.records, [{ idempotencyKey: input.idempotencyKey,
      activityId: hex(keccak256(bytes(input.rawTransaction))), receiptDigest: proof.receiptDigest.slice(2) }]);
    assert.deepEqual(await middleware.execute(grantRequest), outcome);
    assert.equal(draws.inputs.length, 1, "committed replay must not execute another draw");
    assert.equal(budgets.available, 60n);
    assert.equal(budgets.transitions.length, 1);
    assert.equal(receipts.records.length, 1);

    const badReplay = signedProof(input.rawTransaction, { idempotencyKey: word(0xaa) });
    receipts.proofs.set(proof.receiptDigest.slice(2), badReplay);
    await assert.rejects(middleware.execute(grantRequest), refuses("verification-failure"));
    assert.equal(draws.inputs.length, 1);
  }

  for (const changes of [
    { count: 0 }, { count: 2 }, { amount: 41n }, { grantId: word(0xaa) },
    { source: word(0xaa) }, { source: payee }, { recipient: payer }, { recipient: word(0xaa) }, { assetId: word(0xaa) }, { idempotencyKey: word(0xaa) },
    { contextHash: word(0xaa) }, { address: "0x000000000000000000000000000000000000101b" },
    { topic: word(0xaa) }, { resultCode: 1n },
  ]) {
    const { middleware, budgets, receipts, draws } = setup(signedProof(input.rawTransaction, changes));
    await assert.rejects(middleware.execute(input), refuses("verification-failure"));
    assert.equal(draws.inputs.length, 1);
    assert.equal(budgets.transitions.length, 0);
    assert.equal(receipts.records.length, 0);
  }

  const wrongPin = { ...pin, publicKey: new Uint8Array(32).fill(0xaa) };
  const untrusted = setup(proof, [wrongPin]);
  await assert.rejects(untrusted.middleware.execute(input), refuses("verification-failure"));
  assert.equal(untrusted.budgets.transitions.length, 0);
  assert.equal(untrusted.receipts.records.length, 0);
}

async function agentGrantRefusesDrawAboveReservation() {
  const oversized = transaction({ data: encodeLedgerDraw({ ...draw, amount: 41n }) });
  const { middleware, budgets, receipts, draws } = setup();
  await assert.rejects(middleware.execute(request(oversized)), refuses("budget-conflict"));
  assert.equal(draws.inputs.length, 0);
  assert.equal(budgets.available, 60n);
  assert.equal(budgets.reservations.size, 1);
  assert.equal([...budgets.reservations.values()][0].state, "reserved");
  assert.equal([...budgets.reservations.values()][0].amount, "40");
  assert.equal(budgets.transitions.length, 0);
  assert.equal(receipts.records.length, 0);

  for (const raw of [
    transaction({ to: "0x000000000000000000000000000000000000101b" }),
    transaction({ value: 1n }), transaction({ chainId: 1n }),
    transaction({ data: encodeLedgerDraw({ ...draw, amount: 39n }) }),
    transaction({ data: encodeLedgerDraw({ ...draw, purposeHash: word(0xaa) }) }),
    transaction({ data: encodeLedgerDraw({ ...draw, idempotencyKey: word(0xaa) }) }),
    transaction({ data: Buffer.alloc(228) }),
  ]) {
    await assert.rejects(middleware.execute(request(raw)), refuses("budget-conflict"));
    assert.equal(draws.inputs.length, 0);
    assert.equal(budgets.transitions.length, 0);
    assert.equal(receipts.records.length, 0);
  }
  for (const raw of [undefined, "0x02", transaction({ signed: false })]) {
    await assert.rejects(middleware.execute({ ...request(), rawTransaction: raw }), refuses("invalid-request"));
    assert.equal(draws.inputs.length, 0);
  }
  const exhausted = setup(undefined, [pin], 39n);
  assert.deepEqual(await exhausted.middleware.execute(request()), { kind: "refused", reason: "budget_refused" });
  assert.equal(exhausted.draws.inputs.length, 0);
  assert.equal(exhausted.budgets.reservations.size, 0);
}

export async function agentDrawProducerRefusalSurvivesReopen(t) {
  const recording = JSON.parse(readFileSync(new URL(
    "../../../../../services/interop/crates/x-websearch/tests/fixtures/gateway/metered.json", import.meta.url,
  ), "utf8"));
  const refused = recording.endpoints.gateway["*"].find((entry) =>
    entry.error?.message === "execution reverted: LXP_NO_PAYER_GRANT");
  assert.ok(refused, "the committed producer refusal must be present");
  assert.equal(refused.error.code, 3);
  const calls = [];
  let disconnected = true;
  const endpoint = await loopback(t, (body, _request, response) => {
    assert.equal(body.jsonrpc, "2.0");
    assert.equal(body.method, refused.method);
    assert.deepEqual(body.params, refused.params);
    calls.push(body);
    if (disconnected) { response.destroy(); return; }
    return { jsonrpc: "2.0", id: body.id, error: refused.error };
  });
  const path = join(temporaryDirectory(t), "draw-budget.sqlite");
  const input = request(transaction({ data: encodeLedgerDraw({ ...draw, grantId: word(0xee) }) }));
  const proof = signedProof(input.rawTransaction, { grantId: word(0xee) });
  const draws = new ReceiptDrawExecution(proof);
  const receipts = new AgentReceiptRepository(proof);
  const grants = { async resolve({ network, grantId }) {
    assert.equal(network, input.requirements.network);
    assert.equal(grantId, refused.params[0].data.slice(10));
    // Query the actual producer-owned getPayerGrant refusal before any draw.
    // The current ledger has no successful grant state to supply to the consumer.
    return new PaymentRpc(endpoint).call(refused.method, refused.params);
  } };
  const open = () => {
    const budgets = new BudgetStore(path, 100n);
    return { budgets, middleware: new AgentGrantMiddleware({ tenant: "agent-grant", budgets,
      receipts, draws, grants, receiptKeys: [pin] }) };
  };
  let component = open();
  const calldata = decodeRawEip1559(bytes(input.rawTransaction)).data;
  assert.equal(hex(calldata.subarray(36, 68)), hex(payee));
  assert.equal(hex(calldata.subarray(164, 196)), hex(draw.purposeHash));
  assert.notEqual(hex(payee), hex(payer));
  assert.notEqual(hex(grantAsset), hex(draw.purposeHash));
  await assert.rejects(component.middleware.execute(input));
  assert.equal(component.budgets.available, 60n);
  assert.equal(component.budgets.transitions.length, 0);
  component.budgets.close();
  disconnected = false;
  for (let restart = 0; restart < 2; restart += 1) {
    component = open();
    await assert.rejects(component.middleware.execute(input), (error) =>
      error instanceof PaymentRpcError && error.code === refused.error.code);
    assert.equal(component.budgets.available, 60n);
    assert.equal(component.budgets.transitions.length, 0);
    assert.equal([...component.budgets.reservations.values()][0].state, "reserved");
    assert.deepEqual(draws.inputs, []);
    assert.deepEqual(receipts.records, []);
    component.budgets.close();
  }
  assert.equal(calls.length, 3);
}

test(agentGrantSettlesDrawWithPaymentDrawnLog);
test(agentGrantRefusesDrawAboveReservation);
test(agentDrawProducerRefusalSurvivesReopen);

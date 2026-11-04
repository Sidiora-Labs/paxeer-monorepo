import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import test from "node:test";
import { ed25519 } from "@noble/curves/ed25519.js";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { AgentEnvelopeTransport, AgentSessionCredential, SecretBytes, ProductionClient, JsonRpcClient, PreparedGrantDraws, decodeRawEip1559, eip1559SigningHash, keccak256, verifySequencerReceipt, PlatformSdkError } from "@sidiora/layerx-sdk";
import { decodePaymentDrawnLog, encodeLedgerDraw, LAYERX_LEDGER_ADDRESS, ReceiptPayloadAuthority } from "@sidiora/layerx-seller-middleware";
import { AgentGrantMiddleware, AgentMiddleware, AgentMiddlewareError, decodeDaemonSubmission, decodeDaemonSubmitResult, resolveDaemonReceipt, verifyAgentPayment } from "../dist/index.js";

import { once } from "node:events";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";

export const REPAIR_TASK_RECORD = Object.freeze({
  task: "22.31.435",
  findings: {
    "daemon-submission-shape": ["daemonSubmissionShapeUsesDispatchTranscripts", "daemonSubmissionRecoversAfterLostSubmit"],
    "agent-draw-recipient": ["agentDrawRecipientBindsAuthenticatedGrant", "agentDrawUnknownReservationSurvivesReopen"],
  },
  baseline: "Not run: the task's ordered gate rules prohibit verification before full implementation; no baseline failure is claimed.",
  wiring: [
    "sdk/integrations/agents/src/daemon-providers.ts — agent integration domain: consume canonical observation receipts with resolveDaemonReceipt and configured receipt pins.",
    "services/platform/middleware/conformance/service.mjs — middleware conformance domain: supply AgentGrantMiddleware.grants from authenticated grant state.",
  ],
});

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (...values) => createHash("sha256").update(Buffer.concat(values)).digest();
const word = (value) => Buffer.alloc(32, value);
const draw = {
  from: word(0x66), assetId: word(0x99), amount: 40n, grantId: word(0x33), receiverSequence: 1n,
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
  extra: { layerx: { commitment: "receipted", purposeHash: draw.assetId.toString("hex") } } };
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
  topic = drawnTopic, amount = draw.amount, resultCode = 0n, recipient = payee, source = payer,
  idempotencyKey = draw.idempotencyKey, contextHash = draw.contextHash, assetId = grantAsset } = {}) {
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


class BudgetStore extends AgentBudgetLedger {
  constructor(path, available = 100000n) {
    super(available);
    this.db = new DatabaseSync(path);
    this.db.exec("CREATE TABLE IF NOT EXISTS budget (id INTEGER PRIMARY KEY, snapshot TEXT NOT NULL)");
    const row = this.db.prepare("SELECT snapshot FROM budget WHERE id = 1").get();
    if (row) {
      const saved = JSON.parse(row.snapshot);
      this.available = BigInt(saved.available);
      this.reservations = new Map(saved.reservations);
      this.transitions = saved.transitions;
    }
  }
  save() {
    this.db.prepare("INSERT INTO budget VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET snapshot = excluded.snapshot")
      .run(JSON.stringify({ available: this.available.toString(), reservations: [...this.reservations], transitions: this.transitions }));
  }
  async reserve(request) { const result = await super.reserve(request); this.save(); return result; }
  transition(request, state) { const result = super.transition(request, state); this.save(); return result; }
  async release(request) { const result = await super.release(request); this.save(); return result; }
  close() { this.save(); this.db.close(); }
}

function temporaryDirectory(t) {
  const path = mkdtempSync(join(tmpdir(), "repair-22-31-435-"));
  t.after(() => rmSync(path, { recursive: true, force: true }));
  return path;
}

async function loopback(t, handle) {
  const errors = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      const result = await handle(body, request, response);
      if (response.destroyed) return;
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify(result));
    } catch (error) {
      errors.push(error);
      response.writeHead(500, { "content-type": "application/json" });
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); });
    assert.deepEqual(errors, []);
  });
  return `http://127.0.0.1:${server.address().port}`;
}

function dispatchTranscript() {
  const source = readFileSync(new URL("../../../../agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs", import.meta.url), "utf8");
  const match = source.match(/const MIDDLEWARE_SUBMISSION_TRANSCRIPT: &str = r#"([\s\S]*?)"#;/u);
  assert.ok(match, "shared dispatch producer transcript must exist");
  return JSON.parse(match[1]);
}

function daemonFixture() {
  const transcript = dispatchTranscript();
  const raw = Buffer.from(transcript.raw_transaction, "hex");
  const tx = decodeRawEip1559(raw);
  const payload = "01";
  const input = {
    tenant: "repair-agent", networkId: "125", authorityHex: "01",
    signerPublicKey: Buffer.from(secp256k1.getPublicKey(new Uint8Array(32).fill(1))).toString("hex"),
    submitIdempotencyKey: "88".repeat(32), approvalCurrentSequence: "1",
    asset: "11".repeat(32), amount: "25000", recipient: "22".repeat(32),
    commitment: { network: "eip155:125", level: "receipted" },
    preparation: { activity_type: "65537", actor: "agent:repair", authority: "repair-authority",
      account_sequence: "1", not_before: "0", not_after: "1000", fee_limit: "1",
      idempotency_key: "44".repeat(32), payload,
      payload_hash: createHash("sha256").update("LXP/v1/payload-hash\0").update(Buffer.from(payload, "hex")).digest("hex") },
  };
  const prepared = { ...input.preparation, preparation_ref: transcript.submit.preparation_ref,
    chain_id: tx.chainId.toString(), nonce: tx.nonce.toString(), max_priority_fee_per_gas: tx.maxPriorityFeePerGas.toString(),
    max_fee_per_gas: tx.maxFeePerGas.toString(), gas_limit: tx.gasLimit.toString(), to: Buffer.from(tx.to).toString("hex"),
    data: Buffer.from(tx.data).toString("hex"), signing_hash: Buffer.from(eip1559SigningHash(tx)).toString("hex") };
  const receiptKeys = [{ keyId: bytes(transcript.receipt_key.keyId), publicKey: bytes(transcript.receipt_key.publicKey),
    first_height: BigInt(transcript.receipt_key.firstHeight), last_height: BigInt(transcript.receipt_key.lastHeight) }];
  const signer = { async sign(value) {
    const recovered = secp256k1.sign(Buffer.from(value.signing_hash, "hex"), new Uint8Array(32).fill(1), { prehash: false, format: "recovered" });
    return Buffer.concat([recovered.subarray(1), recovered.subarray(0, 1)]).toString("hex");
  } };
  return { transcript, raw, input, prepared, receiptKeys, signer };
}

function envelopeClient(endpoint) {
  const session = new AgentSessionCredential("repair-agent", "11".repeat(32), new SecretBytes(new Uint8Array(32).fill(1)), "0");
  const transport = new AgentEnvelopeTransport({ endpoint, session });
  // ProductionClient consumes values; AgentEnvelopeTransport authenticates and decodes the envelope.
  return new ProductionClient({ async call(request) { return (await transport.call(request)).value; } });
}

function receiptsForDaemon() {
  const values = new Map();
  return {
    values,
    async resolve(digest) { assert.ok(values.has(digest)); return structuredClone(values.get(digest)); },
    async retainVerified(record) { this.record = structuredClone(record); },
  };
}

async function daemonSubmissionShapeUsesDispatchTranscripts(t) {
  const { transcript, raw, input, prepared, receiptKeys, signer } = daemonFixture();
  const calls = [];
  const endpoint = await loopback(t, (body, request) => {
    assert.equal(request.url, "/v1/agent/rpc");
    calls.push(body.operation);
    const value = body.operation === "prepare" ? prepared : transcript.submit;
    if (body.operation === "submit") assert.equal(body.request.preparation_ref, prepared.preparation_ref);
    return { request_id: body.request_id, value, verification_status: { state: "achieved", rung: "receipted" } };
  });
  const client = envelopeClient(endpoint);
  const response = await client.agent("submit", { preparation_ref: prepared.preparation_ref }, { idempotencyKey: input.submitIdempotencyKey });
  const decoded = decodeDaemonSubmitResult(response, transcript.submit.tx_hash, prepared.preparation_ref);
  const resolved = await resolveDaemonReceipt(decoded, raw, receiptKeys);
  assert.equal(resolved.state, "Receipted");
  assert.equal(resolved.preparation_ref, prepared.preparation_ref);
  assert.equal(resolved.signed_receipt, transcript.submit.signed_receipt);
  assert.deepEqual(resolved.portableReceipt.receiptKey, transcript.receipt_key);
  assert.equal((await verifyAgentPayment(resolved.portableReceipt, input, receiptKeys)).payment.amount, 25000n);
  await assert.rejects(resolveDaemonReceipt(decoded, raw, [{ ...receiptKeys[0], publicKey: new Uint8Array(32).fill(9) }]),
    (error) => error instanceof PlatformSdkError && error.code === "verification-failure");
  await assert.rejects(resolveDaemonReceipt(decoded, raw, [{ ...receiptKeys[0], first_height: 8n }]),
    (error) => error instanceof PlatformSdkError && error.code === "verification-failure");
  assert.throws(() => decodeDaemonSubmitResult(response, "99".repeat(32), prepared.preparation_ref));
  assert.throws(() => decodeDaemonSubmitResult(response, transcript.submit.tx_hash, "wrong-reference"));

  const budgets = new AgentBudgetLedger(100000n);
  const receipts = receiptsForDaemon();
  const middleware = new AgentMiddleware({ client, budgets, receipts, receiptKeys, signer, maximumTrackPolls: 0 });
  const result = await middleware.spend(input);
  assert.equal(result.kind, "verified");
  assert.equal(result.reservation.state, "committed");
  assert.equal(budgets.available, 75000n);
  assert.deepEqual(calls, ["submit", "prepare", "submit"]);
  receipts.values.set(result.reservation.receiptDigest, resolved.portableReceipt);
  const reopened = new AgentMiddleware({ client, budgets, receipts, receiptKeys, signer, maximumTrackPolls: 0 });
  assert.equal((await reopened.spend(input)).kind, "verified");
  assert.equal(calls.length, 3, "committed replay must not submit again");

  const pending = decodeDaemonSubmission(transcript.pending, transcript.submit.tx_hash, transcript.submit.tx_hash);
  assert.equal(pending.rung, "submitted");
  assert.deepEqual(pending.transitions, transcript.pending.submission.transitions);
  const committed = await resolveDaemonReceipt(decodeDaemonSubmission(transcript.committed), raw, receiptKeys);
  assert.equal(committed.state, "Committed");
  assert.equal(committed.portableReceipt.verificationLevel, "committed");
  assert.deepEqual(committed.observation, transcript.committed);
  for (const changes of [
    (x) => { x.activity_id = "99".repeat(32); },
    (x) => { x.submission.tx_hash = "99".repeat(32); },
    (x) => { x.receipt.tx_hash = "99".repeat(32); },
    (x) => { x.receipt.rung = "receipted"; },
    (x) => { x.submission.rung = "Committed"; },
    (x) => { x.receipt = null; },
  ]) {
    const value = structuredClone(transcript.committed); changes(value);
    assert.throws(() => decodeDaemonSubmission(value));
  }
  for (const state of ["Failed", "Expired"]) {
    const value = structuredClone(transcript.pending);
    value.submission.state = state;
    if (state === "Failed") value.submission.result_code = "-42";
    const terminal = decodeDaemonSubmission(value);
    assert.equal(terminal.state, state);
    if (state === "Failed") assert.equal(terminal.result_code, "-42");
  }
}

const chainId = "paxeer-125";
function varint(value) {
  const result = [];
  do {
    const low = Number(value & 127n);
    value >>= 7n;
    result.push(low | (value === 0n ? 0 : 128));
  } while (value !== 0n);
  return Buffer.from(result);
}

function protoBytes(field, value) {
  return Buffer.concat([varint(BigInt(field * 8 + 2)), varint(BigInt(value.length)), value]);
}

function protoInteger(field, value) {
  return value === 0n ? Buffer.alloc(0) : Buffer.concat([varint(BigInt(field * 8)), varint(value)]);
}

function merkleRoot(leaves) {
  if (leaves.length === 0) return sha256(Buffer.alloc(0));
  if (leaves.length === 1) return sha256(Buffer.of(0), leaves[0]);
  let split = 1;
  while (split * 2 < leaves.length) split *= 2;
  return sha256(Buffer.of(1), merkleRoot(leaves.slice(0, split)), merkleRoot(leaves.slice(split)));
}

function validatorSetHash(validators) {
  return merkleRoot(validators.map((validator) => Buffer.concat([
    protoBytes(1, protoBytes(1, validator.publicKey)),
    protoInteger(2, validator.votingPower),
  ])));
}

function validator(secret) {
  const publicKey = ed25519.getPublicKey(secret);
  return { publicKey, address: Uint8Array.from(sha256(publicKey).subarray(0, 20)), votingPower: 1n };
}

function blockHeader(verified, rawTransaction, validators) {
  const height = verified.height;
  const validatorsHash = validatorSetHash(validators);
  const receiptRoot = merkleRoot([verified.receiptDigest]);
  const dataHash = merkleRoot([rawTransaction]);
  const timestamp = protoInteger(1, 1n);
  const empty = Buffer.alloc(0);
  const fields = [
    Buffer.concat([protoInteger(1, 11n), protoInteger(2, 1n)]),
    protoBytes(1, Buffer.from(chainId)), protoInteger(1, height), timestamp,
    protoBytes(2, empty), empty, protoBytes(1, dataHash),
    protoBytes(1, validatorsHash), protoBytes(1, validatorsHash),
    protoBytes(1, sha256(Buffer.from("conformance-consensus-parameters"))),
    protoBytes(1, verified.receipt.resultingStateRoot), protoBytes(1, receiptRoot),
    protoBytes(1, merkleRoot([])), protoBytes(1, validators[0].address),
  ];
  const blockId = {
    hash: Uint8Array.from(merkleRoot(fields)),
    partSetHeader: { total: 1, hash: Uint8Array.from(merkleRoot([Buffer.concat(fields)])) },
  };
  return { chainId, height, validatorsHash: Uint8Array.from(validatorsHash), receiptRoot, dataHash, blockId };
}

function commitSignBytes(header) {
  const height = Buffer.alloc(9);
  height[0] = 17;
  height.writeBigInt64LE(header.height, 1);
  const partSet = Buffer.concat([
    protoInteger(1, BigInt(header.blockId.partSetHeader.total)),
    protoBytes(2, header.blockId.partSetHeader.hash),
  ]);
  const block = Buffer.concat([protoBytes(1, header.blockId.hash), protoBytes(2, partSet)]);
  const vote = Buffer.concat([
    protoInteger(1, 2n), height, protoBytes(4, block),
    protoBytes(5, protoInteger(1, 1n)), protoBytes(6, Buffer.from(header.chainId)),
  ]);
  return Buffer.concat([varint(BigInt(vote.length)), vote]);
}

async function signedCommitmentResolver(transcript, receiptKeys) {
  const raw = Buffer.from(transcript.raw_transaction, "hex");
  const verified = await verifySequencerReceipt(Buffer.from(transcript.submit.signed_receipt, "hex"), raw, receiptKeys);
  const keys = [1, 2, 3, 4].map((seed) => {
    const secret = word(seed);
    return { secret, validator: validator(secret) };
  }).sort((a, b) => Buffer.compare(a.validator.address, b.validator.address));
  const validatorSet = keys.map((key) => key.validator);
  const header = blockHeader(verified, raw, validatorSet);
  const signBytes = commitSignBytes(header);
  const evidence = {
    landingBlockHeight: header.height, blockHeader: header,
    receiptInclusion: { transactionHash: verified.receipt.activityId.slice(), receiptDigest: verified.receiptDigest.slice(),
      sequence: verified.receipt.globalSequence, leafIndex: 0, leafCount: 1, siblings: [] },
    commit: { height: header.height, round: 0, blockId: header.blockId,
      signatures: keys.slice(0, 3).map(({ secret, validator }) => ({ blockIdFlag: 2,
        validatorAddress: validator.address, timestamp: { seconds: 1n, nanos: 0 }, signature: ed25519.sign(signBytes, secret) })) },
    validatorSet,
  };
  return { async resolve(hash, network) {
    assert.equal(hash, `0x${transcript.submit.tx_hash}`);
    assert.equal(network, "eip155:125");
    return structuredClone(evidence);
  } };
}

async function daemonSubmissionRecoversAfterLostSubmit(t) {
  const fixture = daemonFixture();
  const { transcript, raw, input, prepared, receiptKeys, signer } = fixture;
  const commitments = await signedCommitmentResolver(transcript, receiptKeys);
  let tracked = transcript.pending;
  let submits = 0;
  let polls = 0;
  const endpoint = await loopback(t, (body, request, response) => {
    let value;
    if (body.operation === "prepare") value = prepared;
    else if (body.operation === "submit") { submits += 1; response.destroy(); return; }
    else {
      assert.equal(body.operation, "track");
      assert.equal(body.request.submission_ref, transcript.submit.tx_hash);
      polls += 1; value = tracked;
    }
    return { request_id: body.request_id, value, verification_status: { state: "achieved", rung: value.submission?.rung ?? "submitted" } };
  });
  const path = join(temporaryDirectory(t), "budget.sqlite");
  const receipts = receiptsForDaemon();
  const open = () => {
    const budgets = new BudgetStore(path);
    return { budgets, middleware: new AgentMiddleware({ client: envelopeClient(endpoint), budgets, receipts, receiptKeys, signer, commitments, maximumTrackPolls: 0 }) };
  };
  let component = open();
  assert.equal((await component.middleware.spend(input)).kind, "unknown");
  assert.equal(component.budgets.available, 75000n);
  component.budgets.close(); component = open();
  assert.equal((await component.middleware.resume(input, hex(raw), transcript.submit.tx_hash)).kind, "unknown");
  assert.equal([...component.budgets.reservations.values()][0].state, "reserved");
  assert.equal(component.budgets.transitions.length, 0);
  component.budgets.close(); tracked = transcript.committed; component = open();
  const result = await component.middleware.resume(input, hex(raw), transcript.submit.tx_hash);
  assert.equal(result.kind, "verified");
  assert.equal(result.submission.state, "Committed");
  assert.equal(result.submission.rung, "committed");
  assert.equal(component.budgets.available, 75000n);
  assert.equal(component.budgets.transitions.length, 1);
  assert.equal(submits, 1);
  assert.equal(polls, 2);
  component.budgets.close();
}
function drawRequest(rawTransaction = transaction()) {
  return { principal: "merchant", requestDigest: "88".repeat(32), idempotencyKey: draw.idempotencyKey.toString("hex"),
    rawTransaction, requirements: structuredClone(requirements) };
}

class AuthenticatedGrantSnapshot {
  calls = [];
  constructor(state = {}) {
    this.state = Object.freeze({ network: "eip155:125", grantId: draw.grantId.toString("hex"),
      payer: payer.toString("hex"), asset: grantAsset.toString("hex"), ...state });
  }
  async resolve(request) {
    this.calls.push(structuredClone(request));
    assert.equal(request.network, this.state.network);
    assert.equal(request.grantId, this.state.grantId);
    return this.state;
  }
}

class RegisteredDrawExecution {
  constructor(path, rpc, proof, input) {
    this.proof = proof;
    this.authority = new ReceiptPayloadAuthority([pin]);
    this.prepared = new PreparedGrantDraws(path, payee.toString("hex"), 125n, {
      sendRawTransaction: (raw) => rpc.sendRawTransaction(bytes(raw)),
      sequencerReceipt: (hash) => rpc.sequencerReceipt(hash),
    }, [pin]);
    try { this.prepared.register(input.principal, input.requestDigest, bytes(input.rawTransaction), {
      grantId: draw.grantId.toString("hex"), recipient: payee.toString("hex"), amount: draw.amount,
      idempotencyKey: input.idempotencyKey, sequence: draw.receiverSequence,
      purposeHash: draw.assetId.toString("hex"), contextHash: draw.contextHash.toString("hex"), chainId: 125n,
    }); } catch (error) { this.prepared.close(); throw error; }
  }
  async execute(input) {
    const tx = decodeRawEip1559(bytes(input.rawTransaction));
    const result = await this.prepared.execute({ principal: input.principal, requestDigest: input.requestDigest,
      idempotencyKey: input.idempotencyKey, draw: Buffer.from(tx.data).toString("hex"),
      requirements: { scheme: input.requirements.scheme, amount: input.requirements.amount,
        asset: input.requirements.asset.slice(2), payTo: input.requirements.payTo.slice(2),
        extra: { layerx: { payer: payer.toString("hex"), grantId: draw.grantId.toString("hex"),
          receiverSequence: "1", purposeHash: draw.assetId.toString("hex"), contextHash: draw.contextHash.toString("hex"),
          ...(input.requirements.scheme === "subscription" ? { windowSeconds: "60" } : {}) } } } });
    if (result.kind === "pending") return result;
    const { kind, ...proof } = this.proof;
    const evidence = { ...proof, signedReceipt: hex(result.canonicalReceipt),
      receiptDigest: hex(keccak256(result.canonicalReceipt)) };
    return this.authority.settle({ ...input,
      payload: { x402Version: 2, accepted: input.requirements, payload: evidence } });
  }
  close() { this.prepared.close(); }
}

async function drawFixture(t, { proof = signedProof(transaction()), state = {}, loseAdmission = false } = {}) {
  const input = drawRequest(proof.rawTransaction);
  const hash = hex(keccak256(bytes(input.rawTransaction)));
  const calls = [];
  let recovered = !loseAdmission;
  const endpoint = await loopback(t, (body, _request, response) => {
    calls.push(body.method);
    assert.equal(body.jsonrpc, "2.0");
    let result;
    if (body.method === "eth_sendRawTransaction") {
      assert.equal(body.params[0], input.rawTransaction);
      if (!recovered) { response.destroy(); return; }
      result = hash;
    } else {
      assert.equal(body.method, "pax_sequencerReceipt");
      assert.equal(body.params[0], hash);
      result = { txHash: hash, rung: "receipted", status: "0x1", receipt: proof.signedReceipt };
    }
    return { jsonrpc: "2.0", id: body.id, result };
  });
  const root = temporaryDirectory(t);
  const grants = new AuthenticatedGrantSnapshot(state);
  const receipts = new AgentReceiptRepository(proof);
  const open = () => {
    const budgets = new BudgetStore(join(root, "budget.sqlite"), 100n);
    let draws;
    try { draws = new RegisteredDrawExecution(join(root, "draw.sqlite"), new JsonRpcClient(endpoint), proof, input); }
    catch (error) { budgets.close(); throw error; }
    return { budgets, draws, middleware: new AgentGrantMiddleware({ tenant: "agent-grant", receiptKeys: [pin], grants, budgets, draws, receipts }),
      close() { draws.close(); budgets.close(); } };
  };
  return { open, input, calls, grants, receipts, recover() { recovered = true; } };
}

async function agentDrawRecipientBindsAuthenticatedGrant(t) {
  const fixture = await drawFixture(t);
  const component = fixture.open();
  const outcome = await component.middleware.execute(fixture.input);
  assert.equal(outcome.kind, "settled");
  assert.equal(component.budgets.available, 60n);
  assert.equal(component.budgets.transitions.length, 1);
  assert.equal(component.budgets.transitions[0].state, "committed");
  assert.equal(component.budgets.transitions[0].asset, grantAsset.toString("hex"));
  assert.deepEqual(fixture.calls, ["eth_sendRawTransaction", "pax_sequencerReceipt"]);
  assert.deepEqual(fixture.grants.calls[0], { network: "eip155:125", grantId: draw.grantId.toString("hex") });
  assert.equal(fixture.receipts.records.length, 1);
  component.close();
  const reopened = fixture.open();
  assert.deepEqual(await reopened.middleware.execute(fixture.input), outcome);
  assert.equal(reopened.budgets.transitions.length, 1);
  assert.equal(fixture.calls.length, 2, "committed replay verifies retained evidence without drawing again");
  reopened.close();

  // Correct recipient with an authenticated payer different from the calldata recipient.
  // The old check rejected this valid draw and accepted the reversed fixture below.
  for (const changes of [
    { source: payee }, { source: word(0xaa) }, { recipient: payer },
    { assetId: word(0xaa) }, { assetId: draw.assetId }, { amount: 41n },
    { grantId: word(0xaa) }, { idempotencyKey: word(0xaa) }, { contextHash: word(0xaa) },
    { count: 0 }, { count: 2 },
  ]) {
    const invalid = await drawFixture(t, { proof: signedProof(transaction(), changes) });
    const candidate = invalid.open();
    await assert.rejects(candidate.middleware.execute(invalid.input),
      (error) => error instanceof AgentMiddlewareError && error.code === "verification-failure");
    assert.equal(candidate.budgets.transitions.length, 0);
    assert.equal(invalid.receipts.records.length, 0);
    assert.equal([...candidate.budgets.reservations.values()][0].state, "reserved");
    candidate.close();
  }

  const failedReceipt = await drawFixture(t, { proof: signedProof(transaction(), { resultCode: 1n }) });
  const failed = failedReceipt.open();
  await assert.rejects(failed.middleware.execute(failedReceipt.input), /draw-transaction-failed/u);
  assert.equal(failed.budgets.transitions.length, 0);
  assert.equal(failedReceipt.receipts.records.length, 0);
  failed.close();

  const wrongPayer = await drawFixture(t, { state: { payer: "aa".repeat(32) } });
  const payerMismatch = wrongPayer.open();
  await assert.rejects(payerMismatch.middleware.execute(wrongPayer.input),
    (error) => error instanceof AgentMiddlewareError && error.code === "verification-failure");
  assert.equal(payerMismatch.budgets.transitions.length, 0);
  payerMismatch.close();

  const reversed = transaction({ data: encodeLedgerDraw({ ...draw, from: payer }) });
  const reversedProof = signedProof(reversed, { source: payer, recipient: payee });
  const refused = await drawFixture(t, { proof: reversedProof });
  // The real registration binder refuses this reversed recipient before it reaches execution.
  assert.throws(() => refused.open(), /draw-transaction-mismatch/u);
  // Exercise the middleware's independent recipient check with the correctly registered executor.
  const original = fixture.open();
  await assert.rejects(original.middleware.execute({ ...fixture.input, rawTransaction: reversed }),
    (error) => error instanceof AgentMiddlewareError && error.code === "budget-conflict");
  assert.equal(original.budgets.transitions.length, 1);
  original.close();

  const wrongAsset = await drawFixture(t, { state: { asset: "aa".repeat(32) } });
  const wrong = wrongAsset.open();
  await assert.rejects(wrong.middleware.execute(wrongAsset.input),
    (error) => error instanceof AgentMiddlewareError && error.code === "budget-conflict");
  assert.deepEqual(wrongAsset.calls, []);
  assert.equal(wrong.budgets.transitions.length, 0);
  wrong.close();
}

async function agentDrawUnknownReservationSurvivesReopen(t) {
  const fixture = await drawFixture(t, { loseAdmission: true });
  let component = fixture.open();
  await assert.rejects(component.middleware.execute(fixture.input));
  assert.equal(component.budgets.available, 60n);
  assert.equal(component.budgets.transitions.length, 0);
  assert.equal([...component.budgets.reservations.values()][0].state, "reserved");
  component.close(); fixture.recover(); component = fixture.open();
  const outcome = await component.middleware.execute(fixture.input);
  assert.equal(outcome.kind, "settled");
  assert.equal(component.budgets.available, 60n);
  assert.equal(component.budgets.transitions.length, 1);
  assert.deepEqual(fixture.calls, ["eth_sendRawTransaction", "pax_sequencerReceipt"]);
  assert.equal(fixture.receipts.records.length, 1);
  component.close();
}

test(daemonSubmissionShapeUsesDispatchTranscripts);
test(daemonSubmissionRecoversAfterLostSubmit);
test(agentDrawRecipientBindsAuthenticatedGrant);
test(agentDrawUnknownReservationSurvivesReopen);

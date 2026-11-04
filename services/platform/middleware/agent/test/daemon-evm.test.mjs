import "./production-sources.mjs";
import assert from "node:assert/strict";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import { decodeRawEip1559, eip1559SigningHash, recoverEip1559Sender, PlatformSdkError } from "@sidiora/layerx-sdk";
import { daemonFixture, loopback, envelopeClient, BudgetStore, temporaryDirectory, receiptsForDaemon } from "./repair-support.mjs";
import { join } from "node:path";
const { AgentMiddleware, decodeDaemonPrepared, signedActivityId, decodeDaemonSubmission,
  decodeDaemonSubmitResult, resolveDaemonReceipt, verifyAgentPayment } = await import("../src/index.ts");

const hex = (value) => Buffer.from(value).toString("hex");
const bytes = (value) => Buffer.from(value, "hex");
const secret = bytes("00".repeat(31) + "01");
const otherSecret = bytes("00".repeat(31) + "02");
const publicKey = hex(secp256k1.getPublicKey(secret));
const unsigned = "02ec7d07843b9aca0084b2d05e00830186a09410101010101010101010101010101010101010108084deadbeefc0";
const answer = JSON.parse(`{
  "preparation_ref": "preparation-7",
  "chain_id": "125", "nonce": "7",
  "max_priority_fee_per_gas": "1000000000", "max_fee_per_gas": "3000000000",
  "gas_limit": "100000", "to": "1010101010101010101010101010101010101010", "data": "deadbeef",
  "signing_hash": "${hex(keccak_256(bytes(unsigned)))}",
  "activity_type": "65541", "actor": "did:layerx:agent", "authority": "authority",
  "account_sequence": "7", "not_before": "10", "not_after": "20", "fee_limit": "300000000000000",
  "payload": "7061796c6f6164", "payload_hash": "5c2dd48a6979fc0ba082791cd6bbfac899e725b178933de26d7085854c8d0712", "idempotency_key": "${"22".repeat(32)}"
}`);
const request = {
  tenant: "daemon-evm", networkId: "125", signerPublicKey: publicKey, authorityHex: "01",
  submitIdempotencyKey: "33".repeat(32), approvalCurrentSequence: "7",
  asset: "44".repeat(32), amount: "1", recipient: "55".repeat(32),
  preparation: Object.fromEntries([
    "activity_type", "actor", "authority", "account_sequence", "not_before", "not_after",
    "fee_limit", "payload", "payload_hash", "idempotency_key",
  ].map((name) => [name, answer[name]])),
};
const transaction = (value) => ({
  chainId: BigInt(value.chain_id), nonce: BigInt(value.nonce),
  maxPriorityFeePerGas: BigInt(value.max_priority_fee_per_gas), maxFeePerGas: BigInt(value.max_fee_per_gas),
  gasLimit: BigInt(value.gas_limit), to: bytes(value.to), data: bytes(value.data), value: 0n, accessList: [],
});
const decodeFailure = (error) => error instanceof PlatformSdkError && error.code === "decode-failure";
const verificationFailure = (error) => error instanceof PlatformSdkError && error.code === "verification-failure";

function integer(value) {
  if (value === 0n) return Buffer.alloc(0);
  const encoded = value.toString(16);
  return bytes(encoded.padStart(encoded.length + encoded.length % 2, "0"));
}

function rlp(value) {
  const list = Array.isArray(value);
  const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0] < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.from([prefix + payload.length]), payload]);
  const length = integer(BigInt(payload.length));
  return Buffer.concat([Buffer.from([prefix + 55 + length.length]), length, payload]);
}

function sign(value, key = secret) {
  const recovered = secp256k1.sign(bytes(value.signing_hash), key, { prehash: false, format: "recovered" });
  const signature = Buffer.concat([recovered.subarray(1), recovered.subarray(0, 1)]);
  const tx = transaction(value);
  const raw = Buffer.concat([Buffer.from([2]), rlp([
    integer(tx.chainId), integer(tx.nonce), integer(tx.maxPriorityFeePerGas), integer(tx.maxFeePerGas),
    integer(tx.gasLimit), tx.to, integer(tx.value), tx.data, [], integer(BigInt(signature[64])),
    integer(BigInt(`0x${hex(signature.subarray(0, 32))}`)), integer(BigInt(`0x${hex(signature.subarray(32, 64))}`)),
  ])]);
  return { signature: hex(signature), raw };
}

function decodeDaemonPreparedRecomputesSigningHash() {
  assert.equal(answer.signing_hash, hex(eip1559SigningHash(transaction(answer))));
  const prepared = decodeDaemonPrepared(answer, request, 2);
  for (const name of ["chain_id", "nonce", "max_priority_fee_per_gas", "max_fee_per_gas", "gas_limit", "to", "data", "signing_hash"]) {
    assert.equal(prepared[name], answer[name]);
  }
  assert.equal(prepared.sender, "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf");
  assert(Object.isFrozen(prepared));
  assert(Object.isFrozen(prepared.disclosure));
  assert.equal(prepared.unsigned_canonical_bytes, undefined);
  assert.equal(prepared.signing_preimage, undefined);
  assert.throws(() => decodeDaemonPrepared({ ...answer, nonce: "8" }, request, 2), decodeFailure);
  assert.throws(() => decodeDaemonPrepared({ ...answer, signing_hash: "00".repeat(32) }, request, 2), decodeFailure);
  assert.throws(() => decodeDaemonPrepared(answer, { ...request, networkId: "1" }, 2), decodeFailure);
  assert.throws(() => decodeDaemonPrepared({ ...answer, actor: "another-actor" }, request, 2), decodeFailure);
  assert.throws(() => decodeDaemonPrepared({ ...answer, nonce: "07" }, request, 2), decodeFailure);
  assert.throws(() => decodeDaemonPrepared({ ...answer, unsigned_canonical_bytes: "01" }, request, 2), decodeFailure);
  for (const change of [{ data: "", nonce: "0" }, { data: "ab".repeat(512) }, { nonce: "18446744073709551615", max_fee_per_gas: "340282366920938463463374607431768211455" }]) {
    const value = { ...answer, ...change };
    value.signing_hash = hex(eip1559SigningHash(transaction(value)));
    assert.equal(decodeDaemonPrepared(value, request, 2).signing_hash, value.signing_hash);
  }
  for (const change of [{ chain_id: "0" }, { gas_limit: "0" }, { nonce: "18446744073709551616" }, { max_priority_fee_per_gas: "3000000001" }]) {
    const value = { ...answer, ...change };
    value.signing_hash = hex(eip1559SigningHash(transaction(value)));
    assert.throws(() => decodeDaemonPrepared(value, request, 2), decodeFailure);
  }
}

function signedTransactionHashRecoversPreparedSender() {
  for (const data of ["deadbeef", "", "ab".repeat(512)]) {
    const value = { ...answer, data };
    value.signing_hash = hex(eip1559SigningHash(transaction(value)));
    const prepared = decodeDaemonPrepared(value, request, 2);
    const signed = sign(value);
    const { yParity, r, s, ...decoded } = decodeRawEip1559(signed.raw);
    assert.deepEqual(decoded, transaction(value));
    assert.equal(recoverEip1559Sender(signed.raw).toLowerCase(), prepared.sender);
    assert.equal(signedActivityId(prepared, signed.signature, publicKey), hex(keccak_256(signed.raw)));
    assert.throws(() => signedActivityId(prepared, sign(value, otherSecret).signature, publicKey), verificationFailure);
    assert.throws(() => signedActivityId(prepared, signed.signature, hex(secp256k1.getPublicKey(otherSecret))), verificationFailure);
    assert.throws(() => signedActivityId(prepared, signed.signature.slice(0, -2), publicKey), decodeFailure);
    assert.throws(() => signedActivityId(prepared, signed.signature.slice(0, -2) + "02", publicKey), decodeFailure);
    assert.throws(() => signedActivityId({ ...prepared, nonce: "8" }, signed.signature, publicKey), decodeFailure);
    const order = 0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141n;
    const highS = r.toString(16).padStart(64, "0") + (order - s).toString(16).padStart(64, "0") + (yParity ^ 1).toString(16).padStart(2, "0");
    assert.throws(() => signedActivityId(prepared, highS, publicKey), verificationFailure);
  }
}

async function decodeDaemonSubmissionAcceptsOnlyRungLevels(t) {
  const { transcript, raw, input, receiptKeys } = daemonFixture();
  const endpoint = await loopback(t, (body) => {
    assert.equal(body.operation, "track");
    assert.deepEqual(body.request, { submission_ref: transcript.submit.tx_hash });
    return { request_id: body.request_id, value: transcript.committed,
      verification_status: { state: "achieved", rung: "committed" } };
  });
  const observation = await envelopeClient(endpoint).agent("track", { submission_ref: transcript.submit.tx_hash });
  assert.deepEqual(observation, transcript.committed);
  const decoded = await resolveDaemonReceipt(decodeDaemonSubmission(observation,
    observation.activity_id, observation.submission.submission_ref), raw, receiptKeys);
  assert.equal(decoded.verification_level, "Committed");
  assert.equal(decoded.state, "Committed");
  assert.equal(decoded.rung, "committed");
  assert.equal(decoded.tx_hash, observation.activity_id);
  assert.equal(decoded.activity_id, observation.activity_id);
  assert.deepEqual(decoded.portableReceipt, {
    format: "layerx-receipt-proof-v2", verificationLevel: "committed",
    rawTransaction: `0x${transcript.raw_transaction}`, signedReceipt: `0x${observation.receipt.canonical_bytes}`,
    receiptDigest: `0x${hex(keccak_256(bytes(observation.receipt.canonical_bytes)))}`,
    receiptKey: transcript.receipt_key,
  });
  assert.equal(decodeDaemonSubmission(transcript.pending).verification_level, "Submitted");
  assert.equal(decodeDaemonSubmission(transcript.pending).state, "Unknown");
  for (const level of ["Unverified", "SequencerSigned", "BatchIncluded", "StateProven", "CheckpointFinalised", "SettlementAnchored", "Receipted", "Committed"]) {
    const value = structuredClone(observation);
    value.submission.rung = level;
    assert.throws(() => decodeDaemonSubmission(value), decodeFailure, level);
    value.submission.rung = "committed";
    value.receipt.rung = level;
    assert.throws(() => decodeDaemonSubmission(value), decodeFailure, `receipt ${level}`);
  }
  const missing = structuredClone(observation);
  delete missing.receipt.canonical_bytes;
  assert.throws(() => decodeDaemonSubmission(missing), decodeFailure);
  await assert.rejects(resolveDaemonReceipt(decoded, sign(answer).raw, receiptKeys), decodeFailure);
  await assert.rejects(resolveDaemonReceipt(decoded, raw, [{ ...receiptKeys[0], last_height: 0n }]), verificationFailure);
  assert.throws(() => decodeDaemonSubmission(observation, "00".repeat(32)), decodeFailure);
  assert.throws(() => decodeDaemonSubmission(observation, undefined, "another-submission"), decodeFailure);
  for (const mutate of [
    (value) => { value.submission.tx_hash = "99".repeat(32); },
    (value) => { value.receipt.tx_hash = "99".repeat(32); },
    (value) => { value.receipt.rung = "receipted"; },
    (value) => { value.receipt = null; },
  ]) {
    const value = structuredClone(observation); mutate(value);
    assert.throws(() => decodeDaemonSubmission(value), decodeFailure);
  }
  const submitted = decodeDaemonSubmitResult(transcript.submit, decoded.tx_hash, transcript.submit.preparation_ref);
  assert.equal(submitted.state, "Receipted");
  assert.equal((await verifyAgentPayment((await resolveDaemonReceipt(submitted, raw, receiptKeys)).portableReceipt,
    input, receiptKeys)).payment.amount, 25000n);
}

export async function daemonSerializerPendingTrackingSurvivesReopen(t) {
  const { transcript, raw, input, prepared, receiptKeys, signer } = daemonFixture();
  const calls = [];
  let observation = transcript.pending;
  const endpoint = await loopback(t, (body, request, response) => {
    assert.equal(request.url, "/v1/agent/rpc");
    calls.push(body);
    let value;
    if (body.operation === "prepare") {
      assert.deepEqual(body.request, input.preparation);
      value = prepared;
    } else if (body.operation === "submit") {
      assert.deepEqual(body.request, { preparation_ref: prepared.preparation_ref,
        signature: sign(prepared, new Uint8Array(32).fill(1)).signature,
        signer_public_key: input.signerPublicKey, approval_release_ref: null });
      response.destroy();
      return;
    } else {
      assert.equal(body.operation, "track");
      assert.deepEqual(body.request, { submission_ref: transcript.submit.tx_hash });
      value = observation;
    }
    return { request_id: body.request_id, value,
      verification_status: { state: "achieved", rung: value.submission?.rung ?? "submitted" } };
  });
  const path = join(temporaryDirectory(t), "pending.sqlite");
  const receipts = receiptsForDaemon();
  const open = () => {
    const budgets = new BudgetStore(path);
    const middleware = new AgentMiddleware({ client: envelopeClient(endpoint), budgets, receipts, receiptKeys, signer, maximumTrackPolls: 0 });
    return { budgets, middleware };
  };
  let component = open();
  assert.equal((await component.middleware.spend(input)).kind, "unknown");
  assert.equal(component.budgets.available, 75000n);
  assert.equal(component.budgets.transitions.length, 0);
  component.budgets.close();
  component = open();
  const pending = await component.middleware.resume(input, `0x${hex(raw)}`, transcript.submit.tx_hash);
  assert.equal(pending.kind, "unknown");
  assert.equal(pending.submission.state, "Unknown");
  assert.equal(pending.submission.rung, "submitted");
  assert.deepEqual(pending.submission.transitions, transcript.pending.submission.transitions);
  assert.equal(component.budgets.available, 75000n);
  assert.equal(component.budgets.transitions.length, 0);
  component.budgets.close();
  observation = transcript.committed;
  component = open();
  // A committed label alone cannot authorize a debit without consensus evidence.
  const committed = await component.middleware.resume(input, `0x${hex(raw)}`, transcript.submit.tx_hash);
  assert.equal(committed.kind, "unknown");
  assert.equal(committed.submission.state, "Committed");
  assert.equal(committed.submission.rung, "committed");
  assert.deepEqual(committed.submission.observation, transcript.committed);
  assert.equal(committed.submission.portableReceipt.signedReceipt, `0x${transcript.submit.signed_receipt}`);
  assert.equal(component.budgets.available, 75000n);
  assert.equal(component.budgets.transitions.length, 0);
  assert.equal([...component.budgets.reservations.values()][0].state, "reserved");
  assert.equal(calls.filter((call) => call.operation === "submit").length, 1);
  assert.equal(calls.filter((call) => call.operation === "track").length, 2);
  component.budgets.close();
}

test(decodeDaemonPreparedRecomputesSigningHash);
test(signedTransactionHashRecoversPreparedSender);
test(decodeDaemonSubmissionAcceptsOnlyRungLevels);
test(daemonSerializerPendingTrackingSurvivesReopen);

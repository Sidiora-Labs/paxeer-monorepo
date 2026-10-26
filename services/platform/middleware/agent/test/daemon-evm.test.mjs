import assert from "node:assert/strict";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";
import { decodeRawEip1559, eip1559SigningHash, recoverEip1559Sender, PlatformSdkError } from "@sidiora/layerx-sdk";
import { decodeDaemonPrepared, signedActivityId, decodeDaemonSubmission } from "../dist/index.js";

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

const recordedSubmission = `{
  "tx_hash": "a52f2455a87ccb3b84a90ae4e3c9153e7122b66cd69cb080ae9ecd9732619277",
  "submission": {
    "submission_ref": "submission-7",
    "state": "Executed",
    "verification_level": "Receipted",
    "evidence": [],
    "transitions": [],
    "receipt_ref": "receipt-7"
  },
  "receipt": {
    "signed_receipt": "00025201000200000020a52f2455a87ccb3b84a90ae4e3c9153e7122b66cd69cb080ae9ecd9732619277000000000000000100000020000000000000000000000000000000000000000000000000000000000000000000000020a48065818165fb64611dff9b6de4056284bde354442b57d953ecf798a046330d00000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550000000000000001000c000000000100000000200000000000000000000000000000000000000000000000000000000000000000000000710000000000005208000000000000520800000000773594000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85500000000000000000475687078000000000000000000002632e314a0000000002000000000000000000000000000000000000000000000000000000000000003e8000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f7300000000000000000de0b6b3a764000000000020000000000000000000000000fe3b557e8fb62b89f4916b721be55ceb828dbd7300000000000000056bc75e2d6310000000000000000000055de68146d89760000000000000000000000000200000000000000000000000008ba1f109551bd432803012645ac136ddd64dba720000000000000000000000000000000000000000000000000de0b6b3a7640000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000206b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b000001977426f6800100000040bfff5ed541997e8ce3eb29f7492d71d29689b76f85b41b422184810697d08244dcb3c8db9cfdbabd7b8fd3fd22e707f834fac3ebcb52d09be6435487f7cd190c",
    "raw_transaction": "02f8727d80843b9aca0084b2d05e00825208948ba1f109551bd432803012645ac136ddd64dba72880de0b6b3a764000080c080a0c10d1d3ded8a2ac5bf5652235628abf4963e5618e1f0b374794de957bc2401cfa079c223a5baf08e5d5b17ab51b244e2d1d528df9d692adc690653eaa3ddfb0981",
    "receipt_key": {
      "key_id": "6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b",
      "public_key": "8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e",
      "first_height": "1000",
      "last_height": "1999"
    },
    "verification_level": "Receipted"
  }
}`;

function decodeDaemonSubmissionAcceptsOnlyRungLevels() {
  const observation = JSON.parse(recordedSubmission);
  const decoded = decodeDaemonSubmission(observation, observation.tx_hash, observation.submission.submission_ref);
  assert.equal(decoded.verification_level, "Receipted");
  assert.equal(decoded.tx_hash, observation.tx_hash);
  assert.equal(decoded.activity_id, undefined);
  assert.deepEqual(decoded.portableReceipt, {
    format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
    rawTransaction: `0x${observation.receipt.raw_transaction}`, signedReceipt: `0x${observation.receipt.signed_receipt}`,
    receiptDigest: `0x${hex(keccak_256(bytes(observation.receipt.signed_receipt)))}`,
    receiptKey: {
      keyId: `0x${observation.receipt.receipt_key.key_id}`, publicKey: `0x${observation.receipt.receipt_key.public_key}`,
      firstHeight: "1000", lastHeight: "1999",
    },
  });
  const committed = structuredClone(observation);
  committed.submission.verification_level = "Committed";
  committed.receipt.verification_level = "Committed";
  assert.equal(decodeDaemonSubmission(committed).portableReceipt.verificationLevel, "committed");
  const submitted = { tx_hash: observation.tx_hash, submission: {
    submission_ref: "submission-7", state: "Submitted", verification_level: "Submitted", evidence: [], transitions: [],
  }, receipt: null };
  assert.equal(decodeDaemonSubmission(submitted).verification_level, "Submitted");
  for (const level of ["Unverified", "SequencerSigned", "BatchIncluded", "StateProven", "CheckpointFinalised", "SettlementAnchored"]) {
    const value = structuredClone(observation);
    value.submission.verification_level = level;
    assert.throws(() => decodeDaemonSubmission(value), decodeFailure, level);
    value.submission.verification_level = "Receipted";
    value.receipt.verification_level = level;
    assert.throws(() => decodeDaemonSubmission(value), decodeFailure, `receipt ${level}`);
  }
  const missing = structuredClone(observation);
  delete missing.receipt.signed_receipt;
  assert.throws(() => decodeDaemonSubmission(missing), decodeFailure);
  const wrongRaw = structuredClone(observation);
  wrongRaw.receipt.raw_transaction = hex(sign(answer).raw);
  assert.throws(() => decodeDaemonSubmission(wrongRaw), decodeFailure);
  const wrongRange = structuredClone(observation);
  wrongRange.receipt.receipt_key.last_height = "999";
  assert.throws(() => decodeDaemonSubmission(wrongRange), decodeFailure);
  assert.throws(() => decodeDaemonSubmission(observation, "00".repeat(32)), decodeFailure);
  assert.throws(() => decodeDaemonSubmission(observation, undefined, "another-submission"), decodeFailure);
}

test(decodeDaemonPreparedRecomputesSigningHash);
test(signedTransactionHashRecoversPreparedSender);
test(decodeDaemonSubmissionAcceptsOnlyRungLevels);

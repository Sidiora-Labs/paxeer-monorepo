import "./source-loader.mjs";
import assert from "node:assert/strict";
import { createHash, createPrivateKey, createPublicKey, sign } from "node:crypto";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import {
  decodeRawEip1559,
  eip1559SigningHash,
  encodeSequencerReceipt,
  encodeUnsignedEip1559,
  keccak256,
  recoverEip1559Sender,
  verifySequencerReceipt,
} from "@sidiora/layerx-sdk";
import { bytes, hex, nodeReceipt } from "./node-receipts.mjs";

const sha256 = (value) => createHash("sha256").update(value).digest();

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

export async function merchantReceipt(offer) {
  const vector = nodeReceipt("V1-layerxledger-pay-exact");
  const original = await verifySequencerReceipt(vector.receipt, vector.raw, [vector.key]);
  const transaction = decodeRawEip1559(vector.raw);
  const purpose = keccak256(new TextEncoder().encode(offer.resource.url));
  assert.equal(hex(purpose).slice(2), offer.accepts[0].extra.layerx.purposeHash);
  assert.equal(transaction.data.length, 196);
  transaction.data.set(purpose, 164);
  assert.deepEqual(transaction.accessList, []);
  const fields = [
    ...[transaction.chainId, transaction.nonce, transaction.maxPriorityFeePerGas,
      transaction.maxFeePerGas, transaction.gasLimit].map(integer),
    transaction.to, integer(transaction.value), transaction.data, [],
  ];
  assert.deepEqual(Uint8Array.from(Buffer.concat([Buffer.of(2), rlp(fields)])), encodeUnsignedEip1559(transaction));
  // These public test keys are the keys of rpc/sequencer_receipt_vectors_test.go.
  const signature = secp256k1.sign(eip1559SigningHash(transaction),
    bytes("0x8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63"),
    { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  const raw = Uint8Array.from(Buffer.concat([Buffer.of(2), rlp([
    ...fields, integer(BigInt(signature[0])), integer(parsed.r), integer(parsed.s),
  ])]));
  assert.equal(recoverEip1559Sender(raw).toLowerCase(), vector.receiptJson.from);
  const activityId = keccak256(raw);
  const receiptJson = structuredClone(vector.receiptJson);
  receiptJson.txHash = hex(activityId);
  receiptJson.logs = [{
    address: hex(transaction.to),
    topics: [
      hex(keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"))),
      hex(transaction.data.subarray(164, 196)), hex(transaction.data.subarray(4, 36)),
      hex(transaction.data.subarray(36, 68)),
    ],
    data: hex(transaction.data.subarray(68, 132)), logIndex: "0x0",
  }];
  const effects = [original.receipt.effects[0], ...receiptJson.logs.map((log, index) => ({
    moduleId: original.receipt.moduleId, ordinal: index + 1, eventType: 0,
    kind: 3, monetary: false, transferSetRoot: new Uint8Array(32),
    body: Uint8Array.from(Buffer.concat([
      bytes(log.address), Buffer.of(log.topics.length), ...log.topics.map(bytes), sha256(bytes(log.data)),
    ])),
  }))];
  const sequence = Buffer.alloc(8);
  sequence.writeBigUInt64BE(original.receipt.globalSequence);
  const receipt = {
    ...original.receipt, activityId, effects,
    resultingStateRoot: sha256(Buffer.concat([
      Buffer.from("PAXEER/v1/sequence\0"), original.receipt.previousStateRoot, sequence, activityId,
    ])),
  };
  const receiptDigest = sha256(Buffer.concat([
    Buffer.from("LXP/v1/receipt\0"), encodeSequencerReceipt(receipt, false),
  ]));
  const privateKey = createPrivateKey({
    key: Buffer.concat([Buffer.from("302e020100300506032b657004220420", "hex"), Buffer.alloc(32, 0x5e)]),
    format: "der", type: "pkcs8",
  });
  assert.equal(hex(createPublicKey(privateKey).export({ format: "der", type: "spki" }).subarray(-32)), vector.pinnedKey.publicKey);
  const signedReceipt = encodeSequencerReceipt({ ...receipt, sequencerSignature: sign(null, receiptDigest, privateKey) });
  receiptJson.receipt = hex(signedReceipt);
  const verified = await verifySequencerReceipt(signedReceipt, raw, [vector.key]);
  assert.equal(hex(verified.receiptDigest), hex(receiptDigest));
  return {
    ...vector, raw, receipt: signedReceipt, txHash: receiptJson.txHash, receiptJson,
    rawTransaction: hex(raw), signedReceipt: hex(signedReceipt), receiptDigest: hex(receiptDigest),
  };
}

import assert from "node:assert/strict";
import test from "node:test";

import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";

import * as sdk from "@sidiora/layerx-sdk";
import {
  decodeRawEip1559,
  eip1559SigningHash,
  encodeUnsignedEip1559,
  keccak256,
  recoverEip1559Sender,
  type Eip1559Transaction,
} from "../src/evm-transaction.js";

const hex = (bytes: Uint8Array): string => Buffer.from(bytes).toString("hex");
const bytes = (text: string): Uint8Array => Uint8Array.from(Buffer.from(text, "hex"));

type RlpValue = Uint8Array | readonly RlpValue[];

function rlp(value: RlpValue): Uint8Array {
  const list = !(value instanceof Uint8Array);
  const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0]! < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.of(prefix + payload.length), payload]);
  const length = integer(BigInt(payload.length));
  return Buffer.concat([Buffer.of(prefix + 55 + length.length), length, payload]);
}

function integer(value: bigint): Uint8Array {
  if (value === 0n) return new Uint8Array();
  const text = value.toString(16);
  return bytes(text.padStart(text.length + text.length % 2, "0"));
}

function fields(transaction: Eip1559Transaction): RlpValue[] {
  return [
    integer(transaction.chainId), integer(transaction.nonce), integer(transaction.maxPriorityFeePerGas),
    integer(transaction.maxFeePerGas), integer(transaction.gasLimit), transaction.to, integer(transaction.value),
    transaction.data, transaction.accessList.map((entry) => [entry.address, entry.storageKeys]),
  ];
}

function envelope(fields: readonly RlpValue[]): Uint8Array {
  return Uint8Array.from(Buffer.concat([Buffer.of(2), rlp(fields)]));
}

function evm_transaction_roundtrip_recovers_sender(): void {
  const secret = new Uint8Array(32).fill(1);
  const transaction: Eip1559Transaction = {
    chainId: 1n,
    nonce: 7n,
    maxPriorityFeePerGas: 2n,
    maxFeePerGas: 3n,
    gasLimit: 30_000n,
    to: new Uint8Array(20).fill(0x11),
    value: 0n,
    data: bytes("deadbeef"),
    accessList: [],
  };
  const unsigned = `02e30107020382753094${"11".repeat(20)}8084deadbeefc0`;
  assert.equal(hex(encodeUnsignedEip1559(transaction)), unsigned);
  assert.deepEqual(eip1559SigningHash(transaction), keccak_256(bytes(unsigned)));

  for (const input of [
    transaction,
    { ...transaction, nonce: 0n, to: new Uint8Array(), data: new Uint8Array(256).fill(0xab), value: 1n << 200n },
    { ...transaction, chainId: 125n, accessList: [
      { address: new Uint8Array(20).fill(0x22), storageKeys: [new Uint8Array(32), new Uint8Array(32).fill(0xff)] },
      { address: new Uint8Array(20).fill(0x33), storageKeys: [] },
    ] },
  ]) {
    const unsignedFields = fields(input);
    assert.deepEqual(encodeUnsignedEip1559(input), envelope(unsignedFields));
    const digest = eip1559SigningHash(input);
    assert.deepEqual(digest, keccak_256(envelope(unsignedFields)));
    const signature = secp256k1.sign(digest, secret, { prehash: false, format: "recovered" });
    const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
    const signedFields = [...unsignedFields, integer(BigInt(signature[0]!)), integer(parsed.r), integer(parsed.s)];
    const raw = envelope(signedFields);
    const decoded = decodeRawEip1559(raw);
    const { yParity, r, s, ...decodedInput } = decoded;
    assert.deepEqual(decodedInput, input);
    assert.equal(yParity, signature[0]);
    assert.equal(r, parsed.r);
    assert.equal(s, parsed.s);
    assert.deepEqual(encodeUnsignedEip1559(decoded), encodeUnsignedEip1559(input));
    assert.equal(recoverEip1559Sender(raw).toLowerCase(), "0x1a642f0e3c3af545e7acbd38b07251b3990914f1");
    const publicKey = secp256k1.getPublicKey(secret, false);
    assert.equal(recoverEip1559Sender(raw).slice(2).toLowerCase(), hex(keccak_256(publicKey.slice(1)).slice(12)));
    assert(secp256k1.verify(signature.slice(1), digest, publicKey, { prehash: false }));

    const order = 0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141n;
    for (const [index, replacement] of [
      [0, bytes("0001")], [0, new Uint8Array(33).fill(1)], [5, new Uint8Array(19)],
      [8, [[new Uint8Array(19), []]]], [8, [[new Uint8Array(20), [new Uint8Array(31)]]]],
      [8, [[new Uint8Array(20), [], new Uint8Array()]]],
      [9, integer(2n)], [10, integer(0n)], [10, integer(order)], [11, integer(order - s)],
    ] as const) {
      const malformed = signedFields.slice();
      malformed[index] = replacement;
      assert.throws(() => decodeRawEip1559(envelope(malformed)));
      assert.throws(() => recoverEip1559Sender(envelope(malformed)));
    }
    assert.throws(() => decodeRawEip1559(raw.slice(0, -1)));
    assert.throws(() => decodeRawEip1559(Uint8Array.from([...raw, 0])));
    assert.throws(() => decodeRawEip1559(envelope(signedFields.slice(0, -1))));
    assert.throws(() => decodeRawEip1559(envelope([...signedFields, new Uint8Array()])));
    assert.throws(() => decodeRawEip1559(encodeUnsignedEip1559(input)));
    assert.throws(() => decodeRawEip1559(Uint8Array.from([1, ...raw.slice(1)])));
  }

  assert.throws(() => decodeRawEip1559(new Uint8Array()));
  for (const malformed of ["0280", "02c181", "02f801c0", "02f90038", "02ffffffffffffffffff"]) {
    assert.throws(() => decodeRawEip1559(bytes(malformed)));
  }
  for (const value of [-1n, 1n << 256n]) {
    assert.throws(() => encodeUnsignedEip1559({ ...transaction, value }), RangeError);
  }
  assert.throws(() => encodeUnsignedEip1559({ ...transaction, to: new Uint8Array(21) }));
  assert.throws(() => encodeUnsignedEip1559({ ...transaction, accessList: [
    { address: new Uint8Array(20), storageKeys: [new Uint8Array(31)] },
  ] }));
}

function evm_transaction_exports_from_package_root(): void {
  assert.equal(hex(sdk.keccak256(new Uint8Array())), "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470");
  assert.equal(sdk.keccak256, keccak256);
  for (const [name, helper] of [
    ["encodeUnsignedEip1559", encodeUnsignedEip1559],
    ["decodeRawEip1559", decodeRawEip1559],
    ["eip1559SigningHash", eip1559SigningHash],
    ["recoverEip1559Sender", recoverEip1559Sender],
  ] as const) {
    assert.equal(typeof sdk[name], "function");
    assert.equal(sdk[name], helper);
  }
}

test(evm_transaction_roundtrip_recovers_sender);
test(evm_transaction_exports_from_package_root);

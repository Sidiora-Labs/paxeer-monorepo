import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { test } from "node:test";

import { ed25519 } from "@noble/curves/ed25519.js";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { keccak_256 } from "@noble/hashes/sha3.js";

import { eip1559SigningHash, type Eip1559Transaction } from "../src/evm-transaction.js";
import { bindDrawTransaction, type ExpectedDrawTransaction } from "../src/x402/activity.js";
import {
  encodeAccountOpen,
  encodeAssetRegister,
  encodeGrant,
  grantAuthorizationMessage,
  issueGrantCall,
  openAccountCall,
  registerAssetCall,
  type AssetRegistration,
  type PayerGrant,
} from "../src/x402/receive.js";

const LEDGER = "0x000000000000000000000000000000000000101a";
const hex = (bytes: Uint8Array): string => Buffer.from(bytes).toString("hex");
const bytes = (value: string): Uint8Array => Uint8Array.from(Buffer.from(value, "hex"));
const id = (value: number): string => value.toString(16).padStart(2, "0").repeat(32);
const word = (value: bigint | number): string => BigInt(value).toString(16).padStart(64, "0");

function dynamicBytes(value: Uint8Array): string {
  return word(value.length) + hex(value).padEnd(Math.ceil(value.length / 32) * 64, "0");
}

function payerGrant(): PayerGrant {
  const secret = new Uint8Array(32).fill(7);
  const grant: PayerGrant = {
    grant_id: id(1), from: id(2), recipient: id(3), asset: id(4),
    per_draw_maximum: "40", allowance: "400", recurring: true,
    window_length: "3600", expiration: "2000000000", purpose_hash: id(5),
    has_reference: false, reference_hash: id(0), revocation_sequence: "2",
    public_key: hex(ed25519.getPublicKey(secret)), signature: "00".repeat(64),
  };
  return { ...grant, signature: hex(ed25519.sign(grantAuthorizationMessage(grant), secret)) };
}

function grant_encodes_issueGrant_calldata_over_the_payer_grant(): void {
  const grant = payerGrant();
  const payer = encodeGrant(grant);
  const call = issueGrantCall(grant);
  assert.equal(payer.length, 346);
  assert.equal(call.to, LEDGER);
  assert.equal(call.value, 0n);
  assert.equal(call.data, `0xd0d53480${word(32)}${word(346)}${hex(payer)}${"00".repeat(6)}`);
  const calldata = bytes(call.data.slice(2));
  assert.equal(calldata.length, 420);
  const wrapped = calldata.subarray(68, 68 + 346);
  assert.deepEqual(wrapped, payer);
  assert.deepEqual(wrapped.subarray(250, 282), bytes(grant.public_key));
  assert.deepEqual(wrapped.subarray(282), bytes(grant.signature));
  assert(ed25519.verify(wrapped.subarray(282), grantAuthorizationMessage(grant), wrapped.subarray(250, 282)));
  assert(!ed25519.verify(bytes(grant.signature), grantAuthorizationMessage({ ...grant, allowance: "401" }), bytes(grant.public_key)));
  assert.throws(() => issueGrantCall({ ...grant, signature: "00".repeat(63) }));
  assert.throws(() => issueGrantCall({ ...grant, per_draw_maximum: (1n << 128n).toString() }));
}

function account_open_and_asset_register_encode_layerxledger_calldata(): void {
  const asset = id(8);
  const opened = openAccountCall(asset);
  assert.deepEqual(opened, { to: LEDGER, value: 0n, data: `0xbdf52150${asset}` });
  assert.equal(hex(encodeAccountOpen(asset)), `0001${asset}`);
  assert.throws(() => openAccountCall("ab".repeat(31)));
  assert.throws(() => openAccountCall(`0x${asset}`));

  const issuer = id(9);
  const native: AssetRegistration = {
    salt: id(10), symbol: "LXT", name: "LayerX token", decimals: 18,
    supply_cap: "1000000000000000000000000", issuer_kind: 1, custody_ref: new Uint8Array(),
  };
  const derived = createHash("sha256").update("LX:ASSET:v1", "ascii")
    .update(bytes(issuer)).update(bytes(native.salt)).digest("hex");
  const registrations: readonly AssetRegistration[] = [
    native,
    { ...native, asset_id: derived },
    { ...native, asset_id: asset, issuer_kind: 2, symbol: "S".repeat(16),
      name: "\u00e9".repeat(16), decimals: 38, supply_cap: ((1n << 128n) - 1n).toString(),
      custody_ref: new Uint8Array(128).fill(0xab) },
    { ...native, asset_id: asset, issuer_kind: 2, custody_ref: Uint8Array.of(1, 2, 3) },
  ];
  for (const registration of registrations) {
    const call = registerAssetCall(issuer, registration);
    const symbol = dynamicBytes(new TextEncoder().encode(registration.symbol));
    const name = dynamicBytes(new TextEncoder().encode(registration.name));
    const custody = dynamicBytes(registration.custody_ref);
    const expected = `0xfd03012a${registration.asset_id ?? derived}${registration.salt}`
      + word(256) + word(256 + symbol.length / 2) + word(registration.decimals)
      + word(BigInt(registration.supply_cap)) + word(registration.issuer_kind)
      + word(256 + symbol.length / 2 + name.length / 2) + symbol + name + custody;
    assert.deepEqual(call, { to: LEDGER, data: expected, value: 0n });
    assert.equal(hex(encodeAssetRegister(issuer, registration).subarray(2, 34)), registration.asset_id ?? derived);
  }
  for (const invalid of [
    { ...native, asset_id: asset },
    { ...native, issuer_kind: 2 as const },
    { ...native, custody_ref: Uint8Array.of(1) },
    { ...native, decimals: 39 },
    { ...native, supply_cap: (1n << 128n).toString() },
    { ...native, symbol: "S".repeat(17) },
    { ...native, name: "\u00e9".repeat(17) },
    { ...native, asset_id: asset, issuer_kind: 2 as const, custody_ref: new Uint8Array(129) },
  ]) assert.throws(() => registerAssetCall(issuer, invalid));
}

type RlpValue = Uint8Array | readonly RlpValue[];

function integer(value: bigint): Uint8Array {
  if (value === 0n) return new Uint8Array();
  const text = value.toString(16);
  return bytes(text.padStart(text.length + text.length % 2, "0"));
}

function rlp(value: RlpValue): Uint8Array {
  const list = !(value instanceof Uint8Array);
  const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0]! < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.of(prefix + payload.length), payload]);
  const length = integer(BigInt(payload.length));
  return Buffer.concat([Buffer.of(prefix + 55 + length.length), length, payload]);
}

function signTransaction(transaction: Eip1559Transaction): Uint8Array {
  const signature = secp256k1.sign(eip1559SigningHash(transaction), new Uint8Array(32).fill(1),
    { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  return Uint8Array.from(Buffer.concat([Buffer.of(2), rlp([
    integer(transaction.chainId), integer(transaction.nonce), integer(transaction.maxPriorityFeePerGas),
    integer(transaction.maxFeePerGas), integer(transaction.gasLimit), transaction.to,
    integer(transaction.value), transaction.data,
    transaction.accessList.map((entry) => [entry.address, entry.storageKeys]),
    integer(BigInt(signature[0]!)), integer(parsed.r), integer(parsed.s),
  ])]));
}

function bindDrawTransaction_binds_a_raw_draw_transaction_and_returns_its_hash(): void {
  const expected: ExpectedDrawTransaction = {
    grantId: id(1), recipient: id(3), amount: 40n, idempotencyKey: id(11),
    sequence: 7n, purposeHash: id(5), contextHash: id(12), chainId: 125n,
  };
  const calldata = "2e89490f" + expected.grantId + expected.recipient + word(expected.amount)
    + expected.idempotencyKey + word(expected.sequence) + expected.purposeHash + expected.contextHash;
  const transaction: Eip1559Transaction = {
    chainId: expected.chainId, nonce: 4n, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n,
    gasLimit: 100000n, to: bytes(LEDGER.slice(2)), value: 0n, data: bytes(calldata), accessList: [],
  };
  const raw = signTransaction(transaction);
  const payment = bindDrawTransaction(raw, expected);
  assert.equal(payment, `0x${hex(keccak_256(raw))}`);
  assert.match(payment, /^0x[0-9a-f]{64}$/u);
  const changedAmount = "2e89490f" + expected.grantId + expected.recipient + word(41)
    + expected.idempotencyKey + word(expected.sequence) + expected.purposeHash + expected.contextHash;
  assert.throws(() => bindDrawTransaction(signTransaction({ ...transaction, data: bytes(changedAmount) }), expected), /draw-transaction-mismatch/u);
  for (const mismatch of [
    { grantId: id(20) }, { recipient: id(20) }, { amount: 41n }, { idempotencyKey: id(20) },
    { sequence: 8n }, { purposeHash: id(20) }, { contextHash: id(20) }, { chainId: 1n },
  ]) assert.throws(() => bindDrawTransaction(raw, { ...expected, ...mismatch }), /draw-transaction-mismatch/u);
  for (const mismatch of [
    { to: bytes("00".repeat(19) + "01") }, { to: new Uint8Array() },
    { chainId: 1n }, { value: 1n }, { data: bytes("d0d53480" + calldata.slice(8)) },
    { data: bytes(calldata + "00") }, { data: bytes(calldata.slice(0, -2)) },
    { data: bytes(calldata.slice(0, 8 + 64 * 2) + word(1n << 128n) + calldata.slice(8 + 64 * 3)) },
    { data: bytes(calldata.slice(0, 8 + 64 * 4) + word(1n << 64n) + calldata.slice(8 + 64 * 5)) },
  ]) assert.throws(() => bindDrawTransaction(signTransaction({ ...transaction, ...mismatch }), expected), /draw-transaction-mismatch/u);
  assert.throws(() => bindDrawTransaction(raw.slice(0, -1), expected));
  assert.throws(() => bindDrawTransaction(Uint8Array.from([...raw, 0]), expected));
  assert.throws(() => bindDrawTransaction(raw, { ...expected, amount: 1n << 128n }));
  assert.throws(() => bindDrawTransaction(raw, { ...expected, sequence: 1n << 64n }));
  assert.throws(() => bindDrawTransaction(raw, { ...expected, grantId: `0x${expected.grantId}` }));
  const anotherRaw = signTransaction({ ...transaction, nonce: 5n });
  assert.equal(bindDrawTransaction(anotherRaw, expected), `0x${hex(keccak_256(anotherRaw))}`);
  assert.notEqual(bindDrawTransaction(anotherRaw, expected), payment);
}

test(grant_encodes_issueGrant_calldata_over_the_payer_grant);
test(account_open_and_asset_register_encode_layerxledger_calldata);
test(bindDrawTransaction_binds_a_raw_draw_transaction_and_returns_its_hash);

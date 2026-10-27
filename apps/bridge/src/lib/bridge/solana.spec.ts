import { ed25519 } from '@noble/curves/ed25519.js';
import { createHash } from 'node:crypto';
import { hexToBytes, type Address } from 'viem';
import { expect, test } from 'vitest';

import { base58Decode, base58Encode, parseRecipientRecord, recipientRecordAddress } from './solana';

const HANDLE: Address = '0x0123456789abcdef0123456789abcdef01234567';
const KEY = Uint8Array.from({ length: 32 }, (_, index) => index);

function recipientRecordAddressIsOffCurve() {
  expect(base58Encode(new Uint8Array())).toBe('');
  expect(base58Encode(new Uint8Array(32))).toBe('11111111111111111111111111111111');
  expect(base58Encode(new TextEncoder().encode('Hello World'))).toBe('JxF12TrwUP45BMd');
  expect(base58Decode(base58Encode(new Uint8Array([ 0, 0, 1, 255 ])))).toEqual(new Uint8Array([ 0, 0, 1, 255 ]));
  let rejectedOnCurveCandidate = false;
  const addresses = new Set<string>();
  for (let index = 0; index < 8; index += 1) {
    const program = Uint8Array.from({ length: 32 }, (_, offset) => offset + index);
    const result = recipientRecordAddress(base58Encode(program), HANDLE);
    expect(result).toBeDefined();
    if (!result) throw new Error('A valid program and handle must derive a PDA');
    expect(result.bump).toBeGreaterThanOrEqual(0);
    expect(result.bump).toBeLessThanOrEqual(255);
    const expected = createHash('sha256')
      .update('recipient')
      .update(hexToBytes(HANDLE))
      .update(new Uint8Array([ result.bump ]))
      .update(program)
      .update('ProgramDerivedAddress')
      .digest();
    expect(base58Decode(result.address)).toEqual(new Uint8Array(expected));
    expect(() => ed25519.Point.fromBytes(new Uint8Array(expected), true)).toThrow();
    for (let bump = 255; bump > result.bump; bump -= 1) {
      const skipped = createHash('sha256')
        .update('recipient')
        .update(hexToBytes(HANDLE))
        .update(new Uint8Array([ bump ]))
        .update(program)
        .update('ProgramDerivedAddress')
        .digest();
      expect(() => ed25519.Point.fromBytes(new Uint8Array(skipped), true)).not.toThrow();
      rejectedOnCurveCandidate = true;
    }
    addresses.add(result.address);
  }
  expect(addresses.size).toBe(8);
  expect(rejectedOnCurveCandidate).toBe(true);
  expect(recipientRecordAddress('invalid', HANDLE)).toBeUndefined();
  expect(recipientRecordAddress('1', HANDLE)).toBeUndefined();
  expect(recipientRecordAddress(base58Encode(KEY), '0x01')).toBeUndefined();
}

function recipientRecordBytes(): Uint8Array {
  const bytes = new Uint8Array(62);
  bytes.set(new TextEncoder().encode('PXBRREC0'));
  new DataView(bytes.buffer).setUint16(8, 1, false);
  bytes.set(hexToBytes(HANDLE), 10);
  bytes.set(KEY, 30);
  return bytes;
}

function parseRecipientRecordReadsTheKey() {
  const bytes = recipientRecordBytes();
  expect(parseRecipientRecord(bytes, HANDLE)).toEqual(KEY);
  expect(parseRecipientRecord(bytes, '0x0123456789ABCDEF0123456789ABCDEF01234567')).toEqual(KEY);
  const backing = new Uint8Array(70);
  backing.set(bytes, 4);
  expect(parseRecipientRecord(backing.subarray(4, 66), HANDLE)).toEqual(KEY);
}

function parseRecipientRecordRefusesAnotherHandle() {
  const bytes = recipientRecordBytes();
  expect(parseRecipientRecord(bytes, '0x0123456789abcdef0123456789abcdef01234568')).toBeUndefined();
  const otherHandle = bytes.slice();
  otherHandle[10] ^= 1;
  expect(parseRecipientRecord(otherHandle, HANDLE)).toBeUndefined();
  const otherMagic = bytes.slice();
  otherMagic[0] ^= 1;
  expect(parseRecipientRecord(otherMagic, HANDLE)).toBeUndefined();
  for (const version of [ 0, 2, 256, 257 ]) {
    const otherVersion = bytes.slice();
    new DataView(otherVersion.buffer).setUint16(8, version, false);
    expect(parseRecipientRecord(otherVersion, HANDLE)).toBeUndefined();
  }
  for (let length = 0; length < 62; length += 1) {
    expect(parseRecipientRecord(bytes.slice(0, length), HANDLE)).toBeUndefined();
  }
  const tooLong = new Uint8Array(63);
  tooLong.set(bytes);
  expect(parseRecipientRecord(tooLong, HANDLE)).toBeUndefined();
  expect(parseRecipientRecord(bytes, '0x01')).toBeUndefined();
}

test(recipientRecordAddressIsOffCurve.name, recipientRecordAddressIsOffCurve);
test(parseRecipientRecordReadsTheKey.name, parseRecipientRecordReadsTheKey);
test(parseRecipientRecordRefusesAnotherHandle.name, parseRecipientRecordRefusesAnotherHandle);

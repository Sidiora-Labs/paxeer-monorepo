import { ed25519 } from '@noble/curves/ed25519.js';
import { createHash } from 'node:crypto';
import { hexToBytes, type Address } from 'viem';
import { expect, test } from 'vitest';

import {
  base58Decode, base58Encode, parseRecipientRecord, recipientRecordAddress,
  recipientRecordFromResponse, recipientRecordRequest, solanaRecipientHandle,
} from './solana';

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

function recipientRecordRequestUsesGetAccountInfo() {
  const programId = base58Encode(KEY);
  const destination = base58Encode(new Uint8Array(32).fill(7));
  const handle = solanaRecipientHandle(destination);
  if (!handle) throw new Error('A valid Solana destination must have a handle');
  const record = recipientRecordAddress(programId, handle);
  if (!record) throw new Error('A valid program and destination must derive a PDA');
  expect(recipientRecordRequest(programId, destination)).toEqual({
    jsonrpc: '2.0',
    id: 1,
    method: 'getAccountInfo',
    params: [ record.address, { encoding: 'base64' } ],
  });
  expect(record.address).not.toBe(destination);
  expect(recipientRecordRequest(programId, base58Encode(KEY))).not.toEqual(recipientRecordRequest(programId, destination));
  expect(recipientRecordRequest('invalid', destination)).toBeUndefined();
  expect(recipientRecordRequest(programId, 'invalid')).toBeUndefined();
}

function recipientRecordFromResponseReportsAMissingRecord() {
  expect(recipientRecordFromResponse({ jsonrpc: '2.0', id: 1, result: { context: { slot: 1 }, value: null } }, HANDLE))
    .toEqual({ registered: false });
  const data = Buffer.from(recipientRecordBytes()).toString('base64');
  const response = { jsonrpc: '2.0', id: 1, result: { value: { data: [ data, 'base64' ] } } };
  expect(recipientRecordFromResponse(response, HANDLE)).toEqual({ registered: true, key: KEY });
  expect(() => recipientRecordFromResponse(response, '0x0123456789abcdef0123456789abcdef01234568')).toThrow();
  for (const invalid of [
    null,
    {},
    { error: { code: -32603, message: 'Internal error' } },
    { result: {} },
    { result: { value: {} } },
    { result: { value: { data: [ data, 'base58' ] } } },
    { result: { value: { data: [ '!', 'base64' ] } } },
    { result: { value: { data: [ '', 'base64' ] } } },
  ]) {
    expect(() => recipientRecordFromResponse(invalid, HANDLE)).toThrow();
  }
}

test(recipientRecordRequestUsesGetAccountInfo.name, recipientRecordRequestUsesGetAccountInfo);
test(recipientRecordFromResponseReportsAMissingRecord.name, recipientRecordFromResponseReportsAMissingRecord);
test(recipientRecordAddressIsOffCurve.name, recipientRecordAddressIsOffCurve);
test(parseRecipientRecordReadsTheKey.name, parseRecipientRecordReadsTheKey);
test(parseRecipientRecordRefusesAnotherHandle.name, parseRecipientRecordRefusesAnotherHandle);

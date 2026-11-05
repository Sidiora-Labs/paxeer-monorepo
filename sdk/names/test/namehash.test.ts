import { keccak_256 } from '@noble/hashes/sha3.js';
import { bytesToHex, concatBytes, hexToBytes, utf8ToBytes } from '@noble/hashes/utils.js';
import { expect, test } from 'vitest';
import { labelhash, namehash, normalize } from '../src/index.js';

function namehash_matches_the_ens_vectors_and_pax_names() {
  expect(namehash('')).toBe(`0x${'00'.repeat(32)}`);
  expect(namehash('eth')).toBe(
    '0x93cdeb708b7545dc668eb9280176169d1c33cfd8ed6f04690a0bcc88a93fc4ae'
  );
  expect(namehash('foo.eth')).toBe(
    '0xde9b09fd7c5f901e23a3f19fecc54828e9c848539801e86591bd9801b019f84f'
  );
  expect(labelhash('eth')).toBe(
    '0x4f5b812789fc606be1b3b16908db13fc7a9adf7ca72641f84d75b47069d3d7f0'
  );
  expect(labelhash('example')).toBe(`0x${bytesToHex(keccak_256(utf8ToBytes('example')))}`);
  const node = keccak_256(
    concatBytes(hexToBytes(namehash('pax').slice(2)), hexToBytes(labelhash('example').slice(2)))
  );
  expect(namehash('example.pax')).toBe(`0x${bytesToHex(node)}`);
  expect(normalize('ExAmPlE.PAX')).toBe('example.pax');
  expect(namehash('EXAMPLE.PAX')).toBe(namehash('example.pax'));
  expect(normalize('addr.reverse')).toBe('addr.reverse');
  expect(normalize(`${'ab'.repeat(20)}.addr.reverse`)).toBe(`${'ab'.repeat(20)}.addr.reverse`);
  expect(normalize('abc-def.pax')).toBe('abc-def.pax');
  expect(normalize(`${'a'.repeat(63)}.pax`)).toBe(`${'a'.repeat(63)}.pax`);
  expect(normalize('')).toBe('');
  for (const invalid of [
    'Ex_ample.pax',
    '-abc.pax',
    'abc-.pax',
    'ab.pax',
    `${'a'.repeat(64)}.pax`,
    '.pax',
    'abc..pax',
    'abc.pax.',
    'école.pax',
    'e\u0301cole.pax',
    'abc pax',
  ]) {
    expect(() => normalize(invalid), invalid).toThrow(TypeError);
  }
}

test(
  namehash_matches_the_ens_vectors_and_pax_names,
  namehash_matches_the_ens_vectors_and_pax_names
);

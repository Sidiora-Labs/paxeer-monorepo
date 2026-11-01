import { hexToBytes } from '@noble/hashes/utils.js';
import { expect, test } from 'vitest';
import {
  PAXEER_CONTENTHASH_TAG,
  canonicalManifest,
  decodeContenthash,
  encodeContenthash,
  manifestDigest,
  parseManifest,
} from '../src/index.js';

function contenthash_round_trips_the_paxeer_tag() {
  const manifest = { version: 1 as const, name: 'example.pax', entries: {} };
  const digest = manifestDigest(manifest);
  const encoded = encodeContenthash(digest);
  expect(PAXEER_CONTENTHASH_TAG).toBe(0x70617831);
  expect(encoded).toBe(`0x70617831${digest.slice(2)}`);
  expect(hexToBytes(encoded.slice(2))).toHaveLength(36);
  expect(decodeContenthash(encoded)).toEqual({ digest });
  expect(decodeContenthash(hexToBytes(encoded.slice(2)))).toEqual({ digest });
  expect(decodeContenthash(encoded.toUpperCase().replace('0X', '0x'))).toEqual({ digest });
  expect(encodeContenthash(`0x${digest.slice(2).toUpperCase()}`)).toBe(encoded);
  const decoded = decodeContenthash(encoded)!;
  expect(manifestDigest(parseManifest(canonicalManifest(manifest)))).toBe(decoded.digest);
  for (const invalid of [encoded.slice(0, -2), `0x70617832${digest.slice(2)}`, `${encoded}00`]) {
    expect(decodeContenthash(invalid)).toBeNull();
    expect(decodeContenthash(hexToBytes(invalid.slice(2)))).toBeNull();
  }
  for (const invalid of ['', '0x', encoded.slice(2), `0x70617831${'gg'.repeat(32)}`]) {
    expect(decodeContenthash(invalid)).toBeNull();
  }
  for (const invalid of [
    digest.slice(0, -2),
    `${digest}00`,
    digest.slice(2),
    `0x${'gg'.repeat(32)}`,
  ]) {
    expect(() => encodeContenthash(invalid)).toThrow(TypeError);
  }
}

test(contenthash_round_trips_the_paxeer_tag, contenthash_round_trips_the_paxeer_tag);

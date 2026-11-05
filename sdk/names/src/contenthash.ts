import { bytesToHex } from '@noble/hashes/utils.js';

export const PAXEER_CONTENTHASH_TAG = 0x70617831;

export function encodeContenthash(digest: string): `0x${string}` {
  if (!/^0x[0-9a-fA-F]{64}$/.test(digest)) {
    throw new TypeError('Contenthash digest must be a 32-byte 0x-hex string');
  }
  return `0x${PAXEER_CONTENTHASH_TAG.toString(16)}${digest.slice(2).toLowerCase()}`;
}

export function decodeContenthash(bytes: Uint8Array | string): { digest: `0x${string}` } | null {
  const hex = typeof bytes === 'string' ? bytes : `0x${bytesToHex(bytes)}`;
  if (!/^0x[0-9a-fA-F]{72}$/.test(hex)) return null;
  if (hex.slice(2, 10).toLowerCase() !== PAXEER_CONTENTHASH_TAG.toString(16)) return null;
  return { digest: `0x${hex.slice(10).toLowerCase()}` };
}

import { keccak_256 } from '@noble/hashes/sha3.js';
import { bytesToHex, concatBytes, hexToBytes, utf8ToBytes } from '@noble/hashes/utils.js';

export function normalize(name: string): string {
  const normalized = name.toLowerCase().normalize('NFC');
  if (normalized === '') return normalized;
  for (const label of normalized.split('.')) {
    if (!/^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$/.test(label)) {
      throw new TypeError(`Invalid name label: ${label}`);
    }
  }
  return normalized;
}

export function labelhash(label: string): `0x${string}` {
  return `0x${bytesToHex(keccak_256(utf8ToBytes(label)))}`;
}

export function namehash(name: string): `0x${string}` {
  const normalized = normalize(name);
  let node: Uint8Array = new Uint8Array(32);
  if (normalized !== '') {
    for (const label of normalized.split('.').reverse()) {
      node = keccak_256(concatBytes(node, hexToBytes(labelhash(label).slice(2))));
    }
  }
  return `0x${bytesToHex(node)}`;
}

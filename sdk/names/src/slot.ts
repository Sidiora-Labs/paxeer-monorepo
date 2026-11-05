import { keccak_256 } from '@noble/hashes/sha3.js';
import { bytesToHex, concatBytes, hexToBytes } from '@noble/hashes/utils.js';

// PaxPublicResolver.manifestDigest mapping slot in resolver_layout_v1.json.
export const MANIFEST_DIGEST_SLOT = 0;

export function contenthashSlot(node: string): `0x${string}` {
  if (!/^0x[0-9a-fA-F]{64}$/.test(node)) {
    throw new TypeError('Name node must be a 32-byte 0x-hex string');
  }
  const slot = hexToBytes(MANIFEST_DIGEST_SLOT.toString(16).padStart(64, '0'));
  return `0x${bytesToHex(keccak_256(concatBytes(hexToBytes(node.slice(2)), slot)))}`;
}

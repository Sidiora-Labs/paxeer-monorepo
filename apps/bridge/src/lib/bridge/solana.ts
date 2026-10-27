/**
 * Solana-side address handling, per contracts/bridge/ATTESTATION-SOLANA.md.
 *
 * A bridge-out to Solana names the recipient as a 20-byte *handle*:
 * keccak256(full 32-byte pubkey)[12:32]. The burn on Paxeer carries only
 * that handle (ILayerXBridge.bridgeOut takes `address`), so the full
 * base58 address the user typed is handed to the bridge backend, which
 * resubmits it to the Solana custody program for the release. The UI shows
 * the user that this is final: once burned, the payout can only ever go to
 * the pubkey behind the handle.
 */
import { ed25519 } from '@noble/curves/ed25519.js';
import { concat, hexToBytes, keccak256, sha256, type Address, type Hex } from 'viem';

const BASE58_ALPHABET = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
const BASE58_INDEX = new Map([ ...BASE58_ALPHABET ].map((ch, i) => [ ch, i ]));
const RECIPIENT_BYTES = 62;
const RECIPIENT_MAGIC = new TextEncoder().encode('PXBRREC0');
const LAYOUT_VERSION = 1;
const RECIPIENT_SEED = new TextEncoder().encode('recipient');
const PDA_MARKER = new TextEncoder().encode('ProgramDerivedAddress');

/** Decode a base58 string to bytes. Returns undefined on any invalid input. */
export function base58Decode(input: string): Uint8Array | undefined {
  const text = input.trim();
  if (!text) return undefined;
  let zeros = 0;
  while (zeros < text.length && text[zeros] === '1') zeros += 1;
  const bytes: number[] = [];
  for (const ch of text) {
    const digit = BASE58_INDEX.get(ch);
    if (digit === undefined) return undefined;
    let carry = digit;
    for (let i = 0; i < bytes.length; i += 1) {
      carry += bytes[i] * 58;
      bytes[i] = carry & 0xff;
      carry >>= 8;
    }
    while (carry > 0) {
      bytes.push(carry & 0xff);
      carry >>= 8;
    }
  }
  const out = new Uint8Array(zeros + bytes.length);
  for (let i = 0; i < bytes.length; i += 1) {
    out[zeros + i] = bytes[bytes.length - 1 - i];
  }
  return out;
}

export function base58Encode(bytes: Uint8Array): string {
  let zeros = 0;
  while (zeros < bytes.length && bytes[zeros] === 0) zeros += 1;
  const digits: number[] = [];
  for (const byte of bytes) {
    let carry = byte;
    for (let i = 0; i < digits.length; i += 1) {
      carry += digits[i] * 256;
      digits[i] = carry % 58;
      carry = Math.floor(carry / 58);
    }
    while (carry > 0) {
      digits.push(carry % 58);
      carry = Math.floor(carry / 58);
    }
  }
  return '1'.repeat(zeros) + digits.reverse().map((digit) => BASE58_ALPHABET[digit]).join('');
}

export function bytesToHex(bytes: Uint8Array): Hex {
  return `0x${ [ ...bytes ].map((b) => b.toString(16).padStart(2, '0')).join('') }` as Hex;
}

/** A Solana address is a base58 string that decodes to exactly 32 bytes. */
export function isSolanaAddress(input: string): boolean {
  const decoded = base58Decode(input);
  return decoded !== undefined && decoded.length === 32;
}

/** keccak256(pubkey)[12:32] — the handle bridgeOut records for a Solana release. */
export function solanaRecipientHandle(base58Address: string): Address | undefined {
  const decoded = base58Decode(base58Address);
  if (!decoded || decoded.length !== 32) return undefined;
  const hash = keccak256(bytesToHex(decoded));
  return `0x${ hash.slice(26) }` as Address;
}

export function recipientRecordAddress(programId: string, handle: Address): { address: string; bump: number } | undefined {
  const program = base58Decode(programId);
  if (!program || program.length !== 32 || !/^0x[0-9a-fA-F]{40}$/.test(handle)) return undefined;
  const handleBytes = hexToBytes(handle);
  for (let bump = 255; bump >= 0; bump -= 1) {
    const candidate = sha256(concat([ RECIPIENT_SEED, handleBytes, new Uint8Array([ bump ]), program, PDA_MARKER ]), 'bytes');
    try {
      ed25519.Point.fromBytes(candidate, true);
    } catch {
      return { address: base58Encode(candidate), bump };
    }
  }
  throw new Error('No off-curve recipient record address exists for these seeds');
}

export function parseRecipientRecord(bytes: Uint8Array, handle: Address): Uint8Array | undefined {
  if (
    bytes.length !== RECIPIENT_BYTES ||
    !RECIPIENT_MAGIC.every((byte, index) => bytes[index] === byte) ||
    bytes[8] * 256 + bytes[9] !== LAYOUT_VERSION ||
    !/^0x[0-9a-fA-F]{40}$/.test(handle) ||
    bytesToHex(bytes.subarray(10, 30)) !== handle.toLowerCase()
  ) return undefined;
  return bytes.slice(30, 62);
}

export function recipientRecordRequest(programId: string, destination: string) {
  const handle = solanaRecipientHandle(destination);
  const record = handle && recipientRecordAddress(programId, handle);
  if (!record) return undefined;
  return {
    jsonrpc: '2.0',
    id: 1,
    method: 'getAccountInfo',
    params: [ record.address, { encoding: 'base64' } ],
  } as const;
}

export function recipientRecordFromResponse(
  response: unknown,
  handle: Address,
): { registered: false } | { registered: true; key: Uint8Array } {
  if (typeof response !== 'object' || response === null || 'error' in response || !('result' in response)) {
    throw new Error('The Solana RPC returned an invalid recipient record response.');
  }
  const result = response.result;
  if (typeof result !== 'object' || result === null || !('value' in result)) {
    throw new Error('The Solana RPC returned an invalid account result.');
  }
  if (result.value === null) return { registered: false };
  const account = result.value;
  if (typeof account !== 'object' || account === null || !('data' in account)) {
    throw new Error('The Solana RPC returned invalid account data.');
  }
  const data = account.data;
  if (!Array.isArray(data) || data.length !== 2 || typeof data[0] !== 'string' || data[1] !== 'base64') {
    throw new Error('The Solana RPC did not return a base64 recipient record.');
  }
  const bytes = Uint8Array.from(atob(data[0]), (char) => char.charCodeAt(0));
  const key = parseRecipientRecord(bytes, handle);
  if (!key) throw new Error('The Solana recipient record does not match the expected layout and handle.');
  return { registered: true, key };
}

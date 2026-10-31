import { bytesToHex, concatBytes, hexToBytes } from '@noble/hashes/utils.js';
import { namehash } from './namehash.js';
import { ProofError, verifyStorageProof, type ProofOp } from './proof.js';
import { contenthashSlot } from './slot.js';

export type HeaderSource = {
  appHashAt(height: bigint): Promise<Uint8Array>;
};

function record(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new TypeError('Expected an RPC object');
  }
  return value as Record<string, unknown>;
}

function heightOf(value: unknown): bigint {
  if (
    !(
      (typeof value === 'number' && Number.isSafeInteger(value)) ||
      (typeof value === 'string' && /^(?:[1-9][0-9]*|0x[0-9a-fA-F]+)$/.test(value))
    )
  ) {
    throw new TypeError('Proof response must include an exact proven height');
  }
  const height = BigInt(value);
  if (height <= 0n) throw new TypeError('Proven height must be positive');
  return height;
}

async function rpcResponse(response: Response): Promise<Record<string, unknown>> {
  if (!response.ok) throw new Error(`RPC HTTP status ${response.status}`);
  const envelope = record(await response.json());
  if (envelope.error != null) throw new Error(`RPC error: ${JSON.stringify(envelope.error)}`);
  return record(envelope.result);
}

function proofOps(value: unknown): ProofOp[] {
  const ops = record(value).ops;
  if (!Array.isArray(ops)) throw new ProofError('proofOps', 'Missing proof ops');
  return ops.map((value) => {
    const op = record(value);
    if (typeof op.type !== 'string' || typeof op.key !== 'string' || typeof op.data !== 'string') {
      throw new ProofError('proofOps', 'Invalid encoded proof op');
    }
    return { type: op.type, key: op.key, data: op.data };
  });
}

export async function resolveContent(
  name: string,
  { rpcUrl, resolver, headers }: { rpcUrl: string; resolver: string; headers: HeaderSource }
): Promise<{ digest: `0x${string}`; height: bigint; verified: true } | null> {
  if (!/^0x[0-9a-fA-F]{40}$/.test(resolver)) throw new TypeError('Invalid resolver address');
  const slot = contenthashSlot(namehash(name));
  const result = await rpcResponse(
    await fetch(rpcUrl, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: 1,
        method: 'eth_getProof',
        params: [resolver, [slot], 'latest'],
      }),
    })
  );
  const height = heightOf(result.height);
  if (
    typeof result.address !== 'string' ||
    result.address.toLowerCase() !== resolver.toLowerCase()
  ) {
    throw new ProofError('ics23:iavl', 'Proof response address differs from the resolver');
  }
  if (
    !Array.isArray(result.hexValues) ||
    result.hexValues.length !== 1 ||
    !Array.isArray(result.storageProof) ||
    result.storageProof.length !== 1
  ) {
    throw new ProofError('ics23:iavl', 'Expected exactly one storage value and proof');
  }
  const encodedValue: unknown = result.hexValues[0];
  if (typeof encodedValue !== 'string' || !/^(?:0x)?[0-9a-fA-F]{64}$/.test(encodedValue)) {
    throw new ProofError('ics23:iavl', 'Manifest digest must be a 32-byte storage word');
  }
  const value = hexToBytes(encodedValue.replace(/^0x/, ''));
  const key = concatBytes(
    Uint8Array.of(3),
    hexToBytes(resolver.slice(2)),
    hexToBytes(slot.slice(2))
  );
  const ops = proofOps(result.storageProof[0]);
  // Header h + 1 commits the application state produced by block h.
  const appHash = await headers.appHashAt(height + 1n);
  verifyStorageProof({ proofOps: ops, appHash, store: 'evm', key, value });
  if (value.every((byte) => byte === 0)) return null;
  return { digest: `0x${bytesToHex(value)}`, height, verified: true };
}

/** Reads consensus RPC headers; callers must trust the RPC until signature verification is added. */
export function tendermintHeaders(url: string): HeaderSource {
  return {
    async appHashAt(height: bigint): Promise<Uint8Array> {
      if (height <= 0n) throw new RangeError('Header height must be positive');
      const endpoint = new URL(`${url.replace(/\/$/, '')}/commit`);
      endpoint.searchParams.set('height', height.toString());
      const result = await rpcResponse(await fetch(endpoint));
      const header = record(record(result.signed_header).header);
      if (heightOf(header.height) !== height)
        throw new Error('Consensus RPC returned the wrong header height');
      if (typeof header.app_hash !== 'string') throw new TypeError('Missing header app_hash');
      const appHash = Uint8Array.from(atob(header.app_hash), (character) =>
        character.charCodeAt(0)
      );
      if (appHash.length !== 32) throw new TypeError('Header app_hash must be 32 bytes');
      return appHash;
    },
  };
}

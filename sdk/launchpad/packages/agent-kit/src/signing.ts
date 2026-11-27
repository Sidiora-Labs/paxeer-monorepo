import { createHash, createPrivateKey, createPublicKey, randomBytes, sign } from 'node:crypto';
import { requireThat } from './errors.js';

export const AGENT_REQUEST_DOMAIN = 'PXW:AGENT-REQUEST:v1';
export const AGENT_REQUEST_DEFAULT_TTL_SECONDS = 60;
const U64_MAX = (1n << 64n) - 1n;

function u32be(value: number): Buffer {
  const bytes = Buffer.alloc(4);
  bytes.writeUInt32BE(value);
  return bytes;
}

function u64be(value: bigint): Buffer {
  requireThat(value >= 0n && value <= U64_MAX, 'invalid_expiry');
  const bytes = Buffer.alloc(8);
  bytes.writeBigUInt64BE(value);
  return bytes;
}

function sha256(value: Uint8Array | string): Buffer {
  return createHash('sha256').update(value).digest();
}

function privateKey(seed: Uint8Array) {
  requireThat(seed instanceof Uint8Array && seed.length === 32, 'invalid_agent_seed');
  return createPrivateKey({
    key: Buffer.concat([Buffer.from('302e020100300506032b657004220420', 'hex'), seed]),
    format: 'der',
    type: 'pkcs8',
  });
}

export function agentPublicKey(seed: Uint8Array): Buffer {
  return createPublicKey(privateKey(seed)).export({ format: 'der', type: 'spki' }).subarray(-32);
}

export function agentDid(seed: Uint8Array, label = 'launchpad'): string {
  requireThat(/^[A-Za-z0-9_-]{1,128}$/.test(label), 'invalid_agent_label');
  return `did:matrix:${label}:${agentPublicKey(seed).toString('hex').slice(0, 16)}`;
}

export function signAgentBytes(seed: Uint8Array, message: Uint8Array): string {
  return sign(null, message, privateKey(seed)).toString('hex');
}

export function agentRequestMethod(method: string, url: string): string {
  const path = /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(url)
    ? new URL(url).pathname
    : (url.split(/[?#]/, 1)[0] ?? url);
  return `${method.toUpperCase()} ${path}`;
}

export interface AgentRequestDigestInput {
  method: string;
  keyId: string;
  nonce: Uint8Array;
  expiry: bigint;
  body: string | Uint8Array;
}

export function agentRequestDigest(input: AgentRequestDigestInput): Buffer {
  const method = Buffer.from(input.method, 'utf8');
  const keyId = Buffer.from(input.keyId, 'utf8');
  requireThat(method.length <= 0xffffffff && keyId.length <= 0xffffffff, 'agent_request_too_large');
  requireThat(input.nonce.length === 16, 'invalid_agent_nonce');
  return sha256(
    Buffer.concat([
      Buffer.from(AGENT_REQUEST_DOMAIN),
      u32be(method.length),
      method,
      u32be(keyId.length),
      keyId,
      input.nonce,
      u64be(input.expiry),
      sha256(input.body),
    ]),
  );
}

export function signAgentRequest(input: {
  privateKey: Uint8Array;
  keyId: string;
  method: string;
  url: string;
  body?: string | Uint8Array;
  nonce?: Uint8Array;
  expiresAt?: bigint | number;
  nowSeconds?: number;
}): Record<string, string> {
  requireThat(input.keyId.length > 0, 'invalid_agent_key_id');
  const now = input.nowSeconds ?? Math.floor(Date.now() / 1000);
  requireThat(Number.isSafeInteger(now) && now >= 0, 'invalid_time');
  if (typeof input.expiresAt === 'number')
    requireThat(Number.isSafeInteger(input.expiresAt), 'invalid_expiry');
  const expiry =
    input.expiresAt === undefined
      ? BigInt(now + AGENT_REQUEST_DEFAULT_TTL_SECONDS)
      : BigInt(input.expiresAt);
  const nonce = input.nonce ?? randomBytes(16);
  const digest = agentRequestDigest({
    method: agentRequestMethod(input.method, input.url),
    keyId: input.keyId,
    nonce,
    expiry,
    body: input.body ?? '',
  });
  return {
    'x-agent-key': input.keyId,
    'x-agent-nonce': Buffer.from(nonce).toString('hex'),
    'x-agent-expires': expiry.toString(),
    'x-agent-signature': signAgentBytes(input.privateKey, digest),
  };
}

export function buildClaim(
  seed: Uint8Array,
  did: string,
  ownerUserId: string,
  expiry: bigint,
): { expires: string; signature: string } {
  requireThat(did.length > 0 && ownerUserId.length > 0, 'invalid_claim');
  const fields = [did, ownerUserId].flatMap((value) => {
    const bytes = Buffer.from(value);
    return [u32be(bytes.length), bytes];
  });
  const digest = sha256(
    Buffer.concat([Buffer.from('PXW:AGENT-CLAIM:v1'), ...fields, u64be(expiry)]),
  );
  return { expires: expiry.toString(), signature: signAgentBytes(seed, digest) };
}

export function bindingMessage(chainId: number, address: string, nonce: bigint): Buffer {
  requireThat(
    Number.isSafeInteger(chainId) && chainId > 0 && /^0x[0-9a-fA-F]{40}$/.test(address),
    'invalid_binding',
  );
  const chain = Buffer.alloc(32);
  chain.writeBigUInt64BE(BigInt(chainId), 24);
  return Buffer.concat([
    Buffer.from('LX:PAXEER-BIND:v1'),
    chain,
    Buffer.from(address.slice(2), 'hex'),
    u64be(nonce),
  ]);
}

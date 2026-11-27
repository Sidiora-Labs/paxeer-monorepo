import { bytesToHex, hexToBytes, sha256 } from 'viem';
import type { PaxeerWallet, SigningSession } from './index';
import type { AgentClaim, AgentRequestHeaders } from './agents';

const encoder = new TextEncoder();
const didPattern = /^did:matrix:[A-Za-z0-9_-]{1,128}:[0-9a-f]{16}$/;

function join(...parts: Uint8Array[]): Uint8Array<ArrayBuffer> {
  const bytes = new Uint8Array(parts.reduce((total, part) => total + part.length, 0));
  let offset = 0;
  for (const part of parts) { bytes.set(part, offset); offset += part.length; }
  return bytes;
}
function u64(value: bigint): Uint8Array<ArrayBuffer> {
  if (value < 0n || value > (1n << 64n) - 1n) throw new Error('The expiry is out of range.');
  const bytes = new Uint8Array(8);
  new DataView(bytes.buffer).setBigUint64(0, value);
  return bytes;
}
function prefixed(text: string): Uint8Array<ArrayBuffer> {
  const bytes = encoder.encode(text);
  const length = new Uint8Array(4);
  new DataView(length.buffer).setUint32(0, bytes.length);
  return join(length, bytes);
}

export function agentClaimDigest(did: string, owner: string, expires: string): `0x${string}` {
  if (!didPattern.test(did) || !owner || !/^\d{1,20}$/.test(expires)) throw new Error('Enter a valid agent DID and expiry.');
  return sha256(join(encoder.encode('PXW:AGENT-CLAIM:v1'), prefixed(did), prefixed(owner), u64(BigInt(expires))));
}

export interface ClaimInstructions {
  did: string;
  owner_user_id: string;
  expires: string;
  digest: `0x${string}`;
}
export function prepareAgentClaim(did: string, owner: string): ClaimInstructions {
  const expires = String(Math.floor(Date.now() / 1000) + 300);
  return { did, owner_user_id: owner, expires, digest: agentClaimDigest(did, owner, expires) };
}
export function parseAgentClaim(text: string, instructions: ClaimInstructions): AgentClaim {
  let value: unknown;
  try { value = JSON.parse(text); } catch { throw new Error('Paste the claim as JSON with expires and signature.'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Paste a claim object.');
  const record = value as Record<string, unknown>;
  if (Object.keys(record).some(key => key !== 'expires' && key !== 'signature') || record.expires !== instructions.expires ||
    typeof record.signature !== 'string' || !/^[0-9a-fA-F]{128}$/.test(record.signature))
    throw new Error('The claim must contain this expiry and a 128-character hex signature without 0x.');
  if (BigInt(instructions.expires) <= BigInt(Math.floor(Date.now() / 1000))) throw new Error('This claim has expired. Prepare a new signing request.');
  return { expires: instructions.expires, signature: record.signature };
}

export async function supportsAgentKeyCreation(): Promise<boolean> {
  try {
    if (!globalThis.crypto?.subtle) return false;
    await crypto.subtle.generateKey('Ed25519', false, ['sign', 'verify']);
    return true;
  } catch { return false; }
}

export interface CreatedAgentKey { did: string; publicKey: string; privateKey: string }

async function sign(key: CryptoKey, bytes: Uint8Array): Promise<string> {
  return bytesToHex(new Uint8Array(await crypto.subtle.sign('Ed25519', key, new Uint8Array(bytes)))).slice(2);
}

export async function signAgentProvisionRequest(did: string, key: CryptoKey, body: { bind_signature?: string }): Promise<AgentRequestHeaders> {
  const nonce = crypto.getRandomValues(new Uint8Array(16));
  const expires = String(Math.floor(Date.now() / 1000) + 60);
  const digest = sha256(join(
    encoder.encode('PXW:AGENT-REQUEST:v1'), prefixed('POST /v1/agent/provision'), prefixed(did),
    nonce, u64(BigInt(expires)), hexToBytes(sha256(encoder.encode(JSON.stringify(body)))),
  ));
  return {
    'x-agent-key': did, 'x-agent-nonce': bytesToHex(nonce).slice(2), 'x-agent-expires': expires,
    'x-agent-signature': await sign(key, hexToBytes(digest)),
  };
}

export async function createOwnedAgent(
  wallet: PaxeerWallet,
  expected: SigningSession,
  label: string,
  showKey: (key: CreatedAgentKey) => void,
): Promise<string> {
  if (!/^[A-Za-z0-9_-]{1,128}$/.test(label)) throw new Error('Use 1–128 letters, numbers, underscores or hyphens for the agent name.');
  expected.assertCurrent();
  const keys = await crypto.subtle.generateKey('Ed25519', true, ['sign', 'verify']) as CryptoKeyPair;
  const publicKey = bytesToHex(new Uint8Array(await crypto.subtle.exportKey('raw', keys.publicKey))).slice(2);
  const did = `did:matrix:${label}:${publicKey.slice(0, 16)}`;
  const exported = new Uint8Array(await crypto.subtle.exportKey('pkcs8', keys.privateKey));
  try {
    expected.assertCurrent();
    // RFC 8410: the inner 32-byte private-key octet string ends the Ed25519 PKCS#8 document.
    if (exported.length !== 48 || bytesToHex(exported.slice(0, 16)) !== '0x302e020100300506032b657004220420')
      throw new Error('This browser cannot export an Ed25519 seed. Claim an existing agent instead.');
    showKey({ did, publicKey, privateKey: bytesToHex(exported.slice(16)).slice(2) });
    const challenge = await wallet.agentChallenge(did);
    expected.assertCurrent();
    if (challenge.did !== did || challenge.message !== `paxeer-agent-auth:${did}:${challenge.nonce}`)
      throw new Error('The agent challenge did not match the new identity.');
    await wallet.verifyAgent({ did, public_key: publicKey, nonce: challenge.nonce, signature: await sign(keys.privateKey, encoder.encode(challenge.message)) });
    expected.assertCurrent();
    let body: { bind_signature?: string } = {};
    let provision = await wallet.provisionAgent(await signAgentProvisionRequest(did, keys.privateKey, body), body);
    expected.assertCurrent();
    if (provision.provision.awaiting === 'agent_signature' && provision.provision.bindMessage) {
      const message = provision.provision.bindMessage;
      const prefix = encoder.encode('LX:PAXEER-BIND:v1');
      const bytes = hexToBytes(message);
      if (bytes.length !== prefix.length + 32 + 20 + 8 ||
        bytesToHex(bytes.slice(0, prefix.length)) !== bytesToHex(prefix) ||
        BigInt(bytesToHex(bytes.slice(prefix.length, prefix.length + 32))) !== 125n ||
        bytesToHex(bytes.slice(prefix.length + 32, prefix.length + 52)).toLowerCase() !== provision.wallet.address.toLowerCase())
        throw new Error('The wallet binding did not match this agent wallet on Paxeer.');
      body = { bind_signature: await sign(keys.privateKey, bytes) };
      provision = await wallet.provisionAgent(await signAgentProvisionRequest(did, keys.privateKey, body), body);
      expected.assertCurrent();
    }
    if (provision.provision.state !== 'active') throw new Error('The agent wallet is not active yet. Keep the key and finish provisioning with your agent.');
    const instructions = prepareAgentClaim(did, expected.userId);
    const claimed = await wallet.claimAgent(did, { expires: instructions.expires, signature: await sign(keys.privateKey, hexToBytes(instructions.digest)) }, expected);
    if (claimed.did !== did || claimed.owner_user_id !== expected.userId) throw new Error('The claim did not match this owner.');
    await wallet.updateAgentPolicy(did, { mode: 'read_only' }, expected);
    return did;
  } finally { exported.fill(0); }
}

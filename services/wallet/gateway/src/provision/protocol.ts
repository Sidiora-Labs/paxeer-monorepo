/** Binding wire formats and cryptography, independent of runtime configuration. */
import { createHash, createPublicKey, verify as edVerify } from 'node:crypto';
import { encodeFunctionData, getAddress, parseAbi, type Hex } from 'viem';

export const ADDR_PRECOMPILE = '0x0000000000000000000000000000000000001004' as const;
export const BIND_DOMAIN = 'LX:PAXEER-BIND:v1';
export const DID_PREFIX = 'did:layerx:';

export const addrAbi = parseAbi([
  'function bindLayerX(bytes32 didPublicKey, bytes signature)',
  'function layerXBindNonce(address evm) view returns (uint64)',
  'function getUnifiedAccount(address evm) view returns (address evm, string paxAddr, bytes32 didPublicKey, bytes32 layerxMainAccountId)',
  'event LayerXBound(address indexed evm, bytes32 indexed didPublicKey, uint64 nonce)',
]);

function strip(hex: string): string {
  return hex.startsWith('0x') || hex.startsWith('0X') ? hex.slice(2) : hex;
}

export function normalisePublicKey(hex: string): string {
  const raw = strip(hex).toLowerCase();
  if (!/^[0-9a-f]{64}$/.test(raw)) throw new Error('an Ed25519 public key is 32 bytes of hex');
  return raw;
}

export function didFromPublicKey(publicKeyHex: string): string {
  return `${DID_PREFIX}${normalisePublicKey(publicKeyHex)}`;
}

export function publicKeyFromDid(did: string): string {
  if (!did.startsWith(DID_PREFIX)) throw new Error('not a did:layerx identifier');
  return normalisePublicKey(did.slice(DID_PREFIX.length));
}

export function mainAccountName(did: string): string {
  return `agent:${did}:main`;
}

export function mainAccountId(did: string): string {
  const name = Buffer.from(mainAccountName(did), 'utf8');
  const len = Buffer.alloc(4);
  len.writeUInt32BE(name.length, 0);
  return createHash('sha256')
    .update(Buffer.from('LX:ACCOUNT:v1', 'utf8'))
    .update(len)
    .update(name)
    .digest('hex');
}

export function bindMessage(
  chainId: bigint | number,
  address: string,
  nonce: bigint | number
): Buffer {
  const chain = Buffer.alloc(32);
  const c = BigInt(chainId);
  if (c <= 0n || c >= 1n << 256n) throw new Error('chain id out of range');
  chain.write(c.toString(16).padStart(64, '0'), 'hex');
  const addr = Buffer.from(strip(getAddress(address)), 'hex');
  const n = Buffer.alloc(8);
  n.writeBigUInt64BE(BigInt(nonce), 0);
  return Buffer.concat([Buffer.from(BIND_DOMAIN, 'utf8'), chain, addr, n]);
}

const ED25519_SPKI_PREFIX = Buffer.from('302a300506032b6570032100', 'hex');

export function verifyEd25519(
  publicKeyHex: string,
  message: Buffer,
  signatureHex: string
): boolean {
  const sig = Buffer.from(strip(signatureHex), 'hex');
  if (sig.length !== 64) return false;
  const key = createPublicKey({
    key: Buffer.concat([ED25519_SPKI_PREFIX, Buffer.from(normalisePublicKey(publicKeyHex), 'hex')]),
    format: 'der',
    type: 'spki',
  });
  return edVerify(null, message, key, sig);
}

export function bindLayerXCalldata(publicKeyHex: string, signatureHex: string): Hex {
  const sig = strip(signatureHex);
  if (!/^[0-9a-fA-F]{128}$/.test(sig)) throw new Error('a binding signature is 64 bytes of hex');
  return encodeFunctionData({
    abi: addrAbi,
    functionName: 'bindLayerX',
    args: [`0x${normalisePublicKey(publicKeyHex)}`, `0x${sig}`],
  });
}

export const SIDIORA_TOKEN = '0x21f7b20a555199fa73A238B1a91FD0f549068fEe' as const;
export const sponsoredBindAbi = parseAbi([
  'function nonce() view returns (uint256)',
  'function executeSponsored((address to,uint256 value,bytes data)[] calls,(address sponsor,address token,uint256 maxTokenAmount,uint256 tokenAmount,uint256 deadline,uint256 quoteNonce,uint256 gasCost) quote,bytes accountSignature,bytes relayerSignature)',
]);

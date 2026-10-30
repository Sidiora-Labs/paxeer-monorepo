import { createHash, createPublicKey, verify as edVerify } from 'node:crypto';
import { readFileSync } from 'node:fs';
import {
  bytesToHex,
  decodeFunctionResult,
  encodeFunctionData,
  getAddress,
  hexToBytes,
  parseAbi,
  recoverTransactionAddress,
  recoverAddress,
  parseSignature,
  serializeSignature,
  serializeTransaction,
  type Hex,
  type TransactionSerializableEIP1559,
} from 'viem';
import { publicKeyToAddress } from 'viem/accounts';
import { GasStationClient, StationAdapterError } from '../sponsor/station.js';
import { z } from 'zod';
import { AttestorClient, constructionDigest, sponsorQuoteDigest, type Authorisation, type SignPayload, type ConstructionWire } from '../attestor/client.js';
import { RpcPool, RpcResponseError } from '../rpc/pool.js';
import { transactionHash } from '../chain.js';

export { transactionHash } from '../chain.js';

export const ADDR_PRECOMPILE = '0x0000000000000000000000000000000000001004' as const;
export const BIND_DOMAIN = 'LX:PAXEER-BIND:v1';
export const DID_PREFIX = 'did:layerx:';

export const addrAbi = parseAbi([
  'function bindLayerX(bytes32 didPublicKey, bytes signature)',
  'function layerXBindNonce(address evm) view returns (uint64)',
  'function getUnifiedAccount(address evm) view returns (address evm, string paxAddr, bytes32 didPublicKey, bytes32 layerxMainAccountId)',
  'event LayerXBound(address indexed evm, bytes32 indexed didPublicKey, uint64 nonce)',
]);

const ZERO32 = `0x${'00'.repeat(32)}` as Hex;

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
  return createHash('sha256').update(Buffer.from('LX:ACCOUNT:v1', 'utf8')).update(len).update(name).digest('hex');
}

export function bindMessage(chainId: bigint | number, address: string, nonce: bigint | number): Buffer {
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

export function verifyEd25519(publicKeyHex: string, message: Buffer, signatureHex: string): boolean {
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

export interface UnifiedAccount {
  evm: `0x${string}`;
  paxAddr: string;
  didPublicKey: string | null;
  mainAccountId: string | null;
}

export async function readUnifiedAccount(rpc: RpcPool, address: `0x${string}`): Promise<UnifiedAccount> {
  const data = encodeFunctionData({ abi: addrAbi, functionName: 'getUnifiedAccount', args: [address] });
  const out = await rpc.request<Hex>('eth_call', [{ to: ADDR_PRECOMPILE, data }, 'latest']);
  const [evm, paxAddr, did, main] = decodeFunctionResult({ abi: addrAbi, functionName: 'getUnifiedAccount', data: out });
  return {
    evm,
    paxAddr,
    didPublicKey: did === ZERO32 ? null : strip(did).toLowerCase(),
    mainAccountId: main === ZERO32 ? null : strip(main).toLowerCase(),
  };
}

export async function readBindNonce(rpc: RpcPool, address: `0x${string}`): Promise<bigint> {
  const data = encodeFunctionData({ abi: addrAbi, functionName: 'layerXBindNonce', args: [address] });
  const out = await rpc.request<Hex>('eth_call', [{ to: ADDR_PRECOMPILE, data }, 'latest']);
  return decodeFunctionResult({ abi: addrAbi, functionName: 'layerXBindNonce', data: out });
}

export type KernelAvailability =
  | { state: 'available' }
  | { state: 'unavailable'; reason: string };

export async function readKernelAvailability(rpc: RpcPool): Promise<KernelAvailability> {
  try {
    await readBindNonce(rpc, '0x0000000000000000000000000000000000000000');
    return { state: 'available' };
  } catch (err) {
    return { state: 'unavailable', reason: `kernel bind nonce could not be read: ${(err as Error).message}` };
  }
}

export class AttestorRefusal extends Error {
  readonly status: number;
  readonly category: string;
  readonly code: string;
  readonly nodeUrl: string;
  constructor(nodeUrl: string, status: number, category: string, code: string, message: string) {
    super(`${category}: ${code}: ${message}`);
    this.name = 'AttestorRefusal';
    this.nodeUrl = nodeUrl;
    this.status = status;
    this.category = category;
    this.code = code;
  }
}

export class AttestorUnavailable extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AttestorUnavailable';
  }
}

export interface GeneratedKey {
  keyId: string;
  curve: 'secp256k1' | 'ed25519';
  publicKey: string;
  address: `0x${string}` | null;
  did: string | null;
}

export interface SignedPayload {
  signature: string;
  recoveryId: number | null;
  signers: string[];
}

export type SignBody = Extract<SignPayload, { kind: 'evm_tx' | 'lx_bind' | 'eth_sign_digest' | 'eip7702_authorization' }>;

export interface AttestorDaemonOptions {
  endpoints: string[];
  cert: string | Buffer;
  key: string | Buffer;
  ca: string | Buffer;
  quorum: number;
  timeoutMs: number;
}

export class AttestorDaemonClient {
  private readonly client: AttestorClient;
  constructor(opts: AttestorDaemonOptions) {
    this.client = new AttestorClient({ endpoints: opts.endpoints,
      tls: { cert: opts.cert, key: opts.key, ca: opts.ca }, quorum: opts.quorum,
      timeoutMs: opts.timeoutMs, healthIntervalMs: 5_000 });
  }
  close(): void { this.client.stop(); }
  async health(): Promise<Array<{ url: string; nodeId: string | null; ready: boolean }>> {
    return (await this.client.refreshHealth()).map((node) => ({ url: node.endpoint, nodeId: node.nodeId, ready: node.healthy }));
  }
  async generate(keyId: string, curve: 'secp256k1' | 'ed25519', owner: string, account?: string): Promise<GeneratedKey> {
    const generated = await this.client.generateKey({ keyId, curve, owner, ...(account ? { account } : {}) });
    const pub = strip(generated.publicKey).toLowerCase();
    if (curve === 'secp256k1') {
      if (pub.length !== 130 || !pub.startsWith('04')) throw new AttestorUnavailable('secp256k1 public key is not uncompressed');
      const address = publicKeyToAddress(`0x${pub}`);
      if (generated.address && getAddress(generated.address) !== address) throw new AttestorUnavailable('attestor address does not match the public key');
      return { keyId, curve, publicKey: pub, address, did: null };
    }
    const did = didFromPublicKey(pub);
    if (generated.did && generated.did !== did) throw new AttestorUnavailable('attestor DID does not match the public key');
    return { keyId, curve, publicKey: pub, address: null, did };
  }
  async sign(keyId: string, body: SignBody, bearerToken: string): Promise<SignedPayload> {
    return this.signAuthorized(keyId, body, { scheme: 'supabase_jwt', token: bearerToken });
  }
  async signAuthorized(keyId: string, body: SignBody, authorisation: Authorisation): Promise<SignedPayload> {
    const result = await this.client.sign({ keyId, payload: body, authorisation });
    return { signature: strip(result.signature), recoveryId: result.recoveryId, signers: result.participants };
  }
}

export function attestorDaemonFromConfig(cfg: {
  ATTESTOR_ENDPOINTS: string[];
  ATTESTOR_CLIENT_CERT_FILE?: string;
  ATTESTOR_CLIENT_KEY_FILE?: string;
  ATTESTOR_CA_FILE?: string;
  ATTESTOR_QUORUM: number;
  ATTESTOR_TIMEOUT_MS: number;
}): AttestorDaemonClient | null {
  if (cfg.ATTESTOR_ENDPOINTS.length === 0) return null;
  return new AttestorDaemonClient({
    endpoints: cfg.ATTESTOR_ENDPOINTS,
    cert: readFileSync(cfg.ATTESTOR_CLIENT_CERT_FILE!),
    key: readFileSync(cfg.ATTESTOR_CLIENT_KEY_FILE!),
    ca: readFileSync(cfg.ATTESTOR_CA_FILE!),
    quorum: cfg.ATTESTOR_QUORUM,
    timeoutMs: cfg.ATTESTOR_TIMEOUT_MS,
  });
}

export async function signBindWithAttestors(
  client: AttestorDaemonClient,
  keyId: string,
  publicKeyHex: string,
  message: Buffer,
  bearerToken: string,
): Promise<string> {
  const out = await client.sign(keyId, { kind: 'lx_bind', message: bytesToHex(message) }, bearerToken);
  if (!verifyEd25519(publicKeyHex, message, out.signature)) {
    throw new AttestorUnavailable('attestor binding signature does not verify under the identity key');
  }
  return out.signature;
}

export async function signTransactionWithAttestors(
  client: AttestorDaemonClient,
  keyId: string,
  address: `0x${string}`,
  tx: TransactionSerializableEIP1559,
  bearerToken: string,
): Promise<Hex> {
  const envelope = serializeTransaction(tx, { r: '0x0', s: '0x0', yParity: 0 });
  const out = await client.sign(keyId, { kind: 'evm_tx', transaction: envelope }, bearerToken);
  const raw = hexToBytes(`0x${out.signature}`);
  if (raw.length !== 65 || out.recoveryId === null) throw new AttestorUnavailable('attestor transaction signature is malformed');
  const signed = serializeTransaction(tx, {
    r: bytesToHex(raw.slice(0, 32)),
    s: bytesToHex(raw.slice(32, 64)),
    yParity: out.recoveryId & 1,
  });
  const recovered = await recoverTransactionAddress({ serializedTransaction: signed as `0x02${string}` });
  if (getAddress(recovered) !== getAddress(address)) {
    throw new AttestorUnavailable('attestor transaction signature recovers another address');
  }
  return signed;
}

export interface ChainReceipt {
  status: 'success' | 'reverted';
  gasUsed: bigint;
  blockNumber: bigint;
}

export async function readReceipt(rpc: RpcPool, hash: Hex): Promise<ChainReceipt | null> {
  const r = await rpc.sequencerReceipt(hash);
  if (!r || r.rung === 'submitted') return null;
  return {
    status: BigInt(r.status) === 1n ? 'success' : 'reverted',
    gasUsed: BigInt(r.gasUsed),
    blockNumber: BigInt(r.rung === 'committed' ? r.blockNumber : r.height),
  };
}

export async function broadcastOnce(rpc: RpcPool, raw: Hex): Promise<Hex> {
  const hash = transactionHash(raw);
  if (await readReceipt(rpc, hash)) return hash;
  try {
    await rpc.sendRawTransaction(raw);
  } catch (err) {
    if (!(err instanceof RpcResponseError)) throw err;
    const known = await rpc.request<unknown>('eth_getTransactionByHash', [hash]).catch(() => null);
    if (!known) throw err;
  }
  return hash;
}

export async function waitForReceipt(rpc: RpcPool, hash: Hex, timeoutMs: number, pollMs: number): Promise<ChainReceipt> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const r = await readReceipt(rpc, hash);
    if (r) return r;
    if (Date.now() >= deadline) throw new Error(`transaction ${hash} has no receipt after ${timeoutMs} ms`);
    await new Promise((resolve) => setTimeout(resolve, Math.min(pollMs, Math.max(0, deadline - Date.now()))));
  }
}

export interface BindTransactionInput {
  chainId: number;
  address: `0x${string}`;
  publicKey: string;
  signature: string;
  nonce: number;
  gas: bigint;
  maxFeePerGas: bigint;
  maxPriorityFeePerGas: bigint;
}

export function bindLayerX(input: BindTransactionInput): TransactionSerializableEIP1559 {
  return {
    type: 'eip1559',
    chainId: input.chainId,
    nonce: input.nonce,
    to: ADDR_PRECOMPILE,
    value: 0n,
    data: bindLayerXCalldata(input.publicKey, input.signature),
    gas: input.gas,
    maxFeePerGas: input.maxFeePerGas,
    maxPriorityFeePerGas: input.maxPriorityFeePerGas,
  };
}


export const SIDIORA_TOKEN = '0x21f7b20a555199fa73A238B1a91FD0f549068fEe' as const;
export const sponsoredBindAbi = parseAbi([
  'function nonce() view returns (uint256)',
  'function executeSponsored((address to,uint256 value,bytes data)[] calls,(address sponsor,address token,uint256 maxTokenAmount,uint256 tokenAmount,uint256 deadline,uint256 quoteNonce,uint256 gasCost) quote,bytes accountSignature,bytes relayerSignature)',
]);
const stationUint = z.string().max(78).regex(/^(0|[1-9][0-9]*)$/).refine(value => BigInt(value) < 1n << 256n);
const stationAddress = z.string().regex(/^0x[0-9a-fA-F]{40}$/);
const stationQuote = z.object({
  quote: z.object({
    sponsor: stationAddress, token: stationAddress,
    maxTokenAmount: stationUint, tokenAmount: stationUint, deadline: stationUint,
    quoteNonce: stationUint, gasCost: stationUint, decimals: z.literal(6),
  }).strict(),
  relayerSignature: z.string().regex(/^0x[0-9a-fA-F]{130}$/),
}).strict();

export function provisioningStationConfig(): { paymaster: Hex; gasLimit: bigint; maxTokenAmount: bigint } {
  const paymaster = stationAddress.safeParse(process.env.PAXEER_GAS_STATION_PAYMASTER);
  const gasLimit = stationUint.safeParse(process.env.PAXEER_GAS_STATION_GAS_LIMIT);
  const maximum = stationUint.safeParse(process.env.PAXEER_GAS_STATION_MAX_TOKEN_AMOUNT);
  if (!paymaster.success || !gasLimit.success || !maximum.success
    || BigInt(gasLimit.data) === 0n || BigInt(maximum.data) === 0n
    || BigInt(paymaster.data) === 0n || paymaster.data.toLowerCase() === SIDIORA_TOKEN.toLowerCase()) {
    throw new StationAdapterError(503, 'gas_station_configuration');
  }
  return { paymaster: getAddress(paymaster.data), gasLimit: BigInt(gasLimit.data), maxTokenAmount: BigInt(maximum.data) };
}

export interface SponsoredBindConstruction {
  batch: {
    chainId: string; account: Hex; nonce: string;
    calls: Array<{ to: typeof ADDR_PRECOMPILE; value: string; data: Hex }>;
    quote: z.infer<typeof stationQuote>['quote'];
  };
  authorization: { chainId: string; address: Hex; nonce: string };
  relayerSignature: Hex;
  accountSignature?: Hex;
}

export async function sponsoredBindRequest(input: {
  station: GasStationClient; rpc: RpcPool; chainId: number; address: Hex;
  publicKey: string; signature: string; gasCost: bigint;
  paymaster: Hex; maxTokenAmount: bigint;
  sign: (payload: SignBody) => Promise<SignedPayload>;
  retained?: SponsoredBindConstruction;
  persist: (construction: SponsoredBindConstruction) => Promise<void>;
}) {
  const { rpc, address, paymaster } = input;
  let retained = input.retained;
  if (!retained) {
    const code = await rpc.request<Hex>('eth_getCode', [address, 'pending']);
    const data = encodeFunctionData({ abi: sponsoredBindAbi, functionName: 'nonce' });
    let encoded: Hex;
    if (code === '0x') {
      const delegateCode = await rpc.request<Hex>('eth_getCode', [paymaster, 'pending']);
      if (!/^0x(?:[0-9a-fA-F]{2})+$/.test(delegateCode) || delegateCode.startsWith('0xef0100'))
        throw new StationAdapterError(503, 'gas_station_delegate_unavailable');
      encoded = await rpc.request<Hex>('eth_call', [{ to: address, data }, 'pending', { [address]: { code: delegateCode } }]);
    } else {
      if (code.toLowerCase() !== `0xef0100${paymaster.slice(2).toLowerCase()}`)
        throw new StationAdapterError(409, 'gas_station_incompatible_delegation');
      encoded = await rpc.request<Hex>('eth_call', [{ to: address, data }, 'pending']);
    }
    if (!/^0x[0-9a-fA-F]{64}$/.test(encoded)) throw new StationAdapterError(502, 'gas_station_invalid_nonce');
    const nonce = decodeFunctionResult({ abi: sponsoredBindAbi, functionName: 'nonce', data: encoded }).toString();
    const calls: SponsoredBindConstruction['batch']['calls'] = [{ to: ADDR_PRECOMPILE, value: '0', data: bindLayerXCalldata(input.publicKey, input.signature) }];
    const chainId = input.chainId.toString();
    const quoted = stationQuote.safeParse(await input.station.quote({
      account: address, nonce, calls, maxTokenAmount: input.maxTokenAmount.toString(),
      gasCost: input.gasCost.toString(), chainId, token: SIDIORA_TOKEN, decimals: 6,
    }));
    if (!quoted.success) throw new StationAdapterError(502, 'gas_station_invalid_response');
    const { quote, relayerSignature } = quoted.data;
    if (quote.token.toLowerCase() !== SIDIORA_TOKEN.toLowerCase()
      || quote.maxTokenAmount !== input.maxTokenAmount.toString() || quote.gasCost !== input.gasCost.toString()
      || BigInt(quote.tokenAmount) > input.maxTokenAmount || BigInt(quote.deadline) <= BigInt(Math.floor(Date.now() / 1000))
      || quote.sponsor.toLowerCase() === address.toLowerCase()) throw new StationAdapterError(502, 'gas_station_invalid_quote');
    const construction: ConstructionWire = { kind: 'sponsored_batch', chainId, account: address, nonce, calls,
      quote: { sponsor: quote.sponsor, token: quote.token, maxTokenAmount: quote.maxTokenAmount,
        tokenAmount: quote.tokenAmount, deadline: quote.deadline, quoteNonce: quote.quoteNonce, gasCost: quote.gasCost } };
    if (getAddress(await recoverAddress({ hash: sponsorQuoteDigest(construction), signature: relayerSignature as Hex })) !== getAddress(quote.sponsor))
      throw new StationAdapterError(502, 'gas_station_quote_signature');
    const accountNonce = BigInt(await rpc.request<Hex>('eth_getTransactionCount', [address, 'pending']));
    if (accountNonce >= (1n << 64n) - 1n) throw new StationAdapterError(409, 'gas_station_account_nonce');
    retained = { batch: { chainId, account: address, nonce, calls, quote }, relayerSignature: relayerSignature as Hex,
      authorization: { chainId, address: paymaster, nonce: accountNonce.toString() } };
    await input.persist(retained);
  }
  const { batch, relayerSignature } = retained;
  const { chainId, nonce, calls, quote } = batch;
  const construction: ConstructionWire = { kind: 'sponsored_batch', chainId, account: batch.account, nonce,
    calls: calls.map(call => ({ to: call.to, value: call.value, data: call.data })),
    quote: { sponsor: quote.sponsor, token: quote.token, maxTokenAmount: quote.maxTokenAmount,
      tokenAmount: quote.tokenAmount, deadline: quote.deadline, quoteNonce: quote.quoteNonce, gasCost: quote.gasCost } };
  const authConstruction: ConstructionWire = { kind: 'eip7702_authorization', chainId: retained.authorization.chainId,
    address: retained.authorization.address, nonce: retained.authorization.nonce };
  const signDigest = async (payload: SignBody, digest: Hex): Promise<Hex> => {
    const signed = await input.sign(payload);
    const raw = hexToBytes(`0x${strip(signed.signature)}`);
    if (raw.length !== 65 || signed.recoveryId === null) throw new AttestorUnavailable('attestor account signature is malformed');
    const signature = serializeSignature({ r: bytesToHex(raw.slice(0, 32)), s: bytesToHex(raw.slice(32, 64)), yParity: signed.recoveryId & 1 });
    if (getAddress(await recoverAddress({ hash: digest, signature })) !== getAddress(address))
      throw new AttestorUnavailable('attestor signature recovers another account');
    return signature;
  };
  const digest = constructionDigest(construction);
  const accountSignature = retained.accountSignature ?? await signDigest({ kind: 'eth_sign_digest', digest, construction }, digest);
  if (!retained.accountSignature) {
    retained = { ...retained, accountSignature };
    await input.persist(retained);
  }
  const authSignature = await signDigest({ kind: 'eip7702_authorization', construction: authConstruction }, constructionDigest(authConstruction));
  const auth = parseSignature(authSignature);
  const authorization = { ...retained.authorization, yParity: auth.yParity!, r: auth.r, s: auth.s };
  const call = { to: address, value: '0', data: encodeFunctionData({
    abi: sponsoredBindAbi, functionName: 'executeSponsored', args: [
      calls.map(call => ({ to: call.to, value: BigInt(call.value), data: call.data })),
      { sponsor: getAddress(quote.sponsor), token: getAddress(quote.token), maxTokenAmount: BigInt(quote.maxTokenAmount),
        tokenAmount: BigInt(quote.tokenAmount), deadline: BigInt(quote.deadline), quoteNonce: BigInt(quote.quoteNonce), gasCost: BigInt(quote.gasCost) },
      accountSignature, relayerSignature as Hex,
    ],
  }) };
  return { call, authorization, batch: { chainId, account: address, nonce, calls, quote }, accountSignature, relayerSignature };
}

export type SponsoredBindRequest = Awaited<ReturnType<typeof sponsoredBindRequest>>;

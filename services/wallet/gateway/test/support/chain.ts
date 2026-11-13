import {
  createHash,
  createPrivateKey,
  createPublicKey,
  sign as edSign,
  verify as edVerify
} from 'node:crypto';
import { createServer as createHttpServer, type Server } from 'node:http';
import { readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  decodeFunctionData,
  encodeAbiParameters,
  encodeFunctionResult,
  getAddress,
  keccak256,
  parseTransaction,
  recoverTransactionAddress,
  recoverAddress,
  serializeSignature,
  toFunctionSelector,
  toHex,
  type Abi,
  type Hex
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { sponsoredBindAbi } from '../../src/provision/protocol.js';
import type { SequencerReceiptResult } from '../../src/rpc/pool.js';
import { encodeSequencerReceipt } from '../../../../agent/sdk/typescript/src/sequencer-receipt-codec.js';
import { encodeSequencerLogEffect } from '../../../../agent/sdk/typescript/src/sequencer-log.js';
import {
  verifySequencerReceipt,
  type SequencerReceipt
} from '../../../../agent/sdk/typescript/src/verifier.js';

const here = dirname(fileURLToPath(import.meta.url));
export const repoRoot = resolve(here, '../../../../..');
export const addrAbiJson = JSON.parse(
  readFileSync(join(repoRoot, 'precompiles/addr/abi.json'), 'utf8')
) as Abi;
export const anchorAbiJson = JSON.parse(
  readFileSync(join(repoRoot, 'precompiles/layerxanchor/abi.json'), 'utf8')
) as Abi;

export const ADDR = '0x0000000000000000000000000000000000001004';
export const ANCHOR = '0x0000000000000000000000000000000000001014';
export const BIND_SELECTOR = toFunctionSelector('bindLayerX(bytes32,bytes)');
const ED_SPKI = Buffer.from('302a300506032b6570032100', 'hex');
const PRECOMPILE_RANGE = /^0x0{36}1[0-9a-f]{3}$/;
const ZERO_ADDRESS = '0x0000000000000000000000000000000000000000';

export type SignedChainReceipt = Exclude<SequencerReceiptResult, { rung: 'submitted' }> & {
  signingKey: Hex;
  paymentDenom: string;
};

export interface ChainReceiptKeyPin {
  keyId: Hex;
  publicKey: Hex;
  firstHeight: number | string;
  lastHeight: number | string;
}

export interface NodeReceiptVector {
  id: string;
  vector: string;
  rawTransaction: Hex;
  receiptJson: SignedChainReceipt;
  signedReceipt: Hex;
  prevSeqHash: Hex;
  pinnedKey: ChainReceiptKeyPin;
}

export const nodeReceiptVectors = JSON.parse(
  readFileSync(
    join(repoRoot, 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json'),
    'utf8'
  )
) as { provenance: { generator: string }; cases: NodeReceiptVector[] };

function receiptBytes(value: unknown): Buffer {
  if (
    typeof value !== 'string' ||
    !/^0x(?:[0-9a-fA-F]{2})*$/.test(value) ||
    value.length > 2_097_154
  ) {
    throw new TypeError('malformed receipt bytes');
  }
  return Buffer.from(value.slice(2), 'hex');
}

function receiptQuantity(value: unknown): bigint {
  if (typeof value !== 'string' || !/^0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)$/.test(value)) {
    throw new TypeError('malformed receipt quantity');
  }
  return BigInt(value);
}

function receiptUint(value: bigint, width: number): Buffer {
  return Buffer.from(toHex(value, { size: width }).slice(2), 'hex');
}

function receiptHash(...parts: (string | Uint8Array)[]): Buffer {
  const hash = createHash('sha256');
  for (const part of parts) hash.update(part);
  return hash.digest();
}

function receiptAddress(value: Hex): Buffer {
  const bytes = receiptBytes(value);
  if (bytes.length !== 20) throw new TypeError('malformed receipt address');
  return Buffer.concat([Buffer.alloc(12), bytes]);
}

function receiptSummary(view: SignedChainReceipt): Buffer {
  if (
    typeof view.feeDenom !== 'string' ||
    Buffer.byteLength(view.feeDenom) < 1 ||
    Buffer.byteLength(view.feeDenom) > 64
  ) {
    throw new TypeError('malformed receipt feeDenom');
  }
  if (typeof view.vmError !== 'string') throw new TypeError('malformed receipt vmError');
  return Buffer.concat([
    ...[view.gasUsed, view.cumulativeGasUsed, view.effectiveGasPrice].map((v) =>
      receiptUint(receiptQuantity(v), 8)
    ),
    receiptUint(receiptQuantity(view.index), 4),
    receiptAddress(view.contractAddress).subarray(12),
    receiptAddress(view.sponsor).subarray(12),
    receiptHash(view.vmError),
    receiptUint(BigInt(receiptBytes(view.returnData).length), 4),
    receiptUint(BigInt(view.logs.length), 4),
    Buffer.from([Buffer.byteLength(view.feeDenom)]),
    Buffer.from(view.feeDenom)
  ]);
}

export async function verifyChainSequencerReceipt(
  view: SignedChainReceipt,
  raw: Hex,
  pins: readonly ChainReceiptKeyPin[]
): Promise<SignedChainReceipt> {
  const canonical = receiptBytes(view.receipt);
  const verified = await verifySequencerReceipt(
    canonical,
    receiptBytes(raw),
    pins.map((pin) => ({
      keyId: receiptBytes(pin.keyId),
      publicKey: receiptBytes(pin.publicKey),
      first_height: BigInt(pin.firstHeight),
      last_height: BigInt(pin.lastHeight)
    }))
  );
  const r = verified.receipt;
  const tx = parseTransaction(raw);
  const from = await recoverTransactionAddress({
    serializedTransaction: raw as `0x02${string}`
  });
  if (view.rung !== 'receipted' && view.rung !== 'committed')
    throw new TypeError('malformed receipt rung');
  if (view.rung === 'receipted' && ('blockNumber' in view || 'blockHash' in view))
    throw new TypeError('malformed receipted block');
  if (
    view.rung === 'committed' &&
    (receiptQuantity(view.blockNumber) < verified.height ||
      receiptBytes(view.blockHash).length !== 32 ||
      BigInt(view.blockHash) === 0n)
  ) {
    throw new TypeError('malformed committed block');
  }
  if (
    typeof view.paymentDenom !== 'string' ||
    Buffer.byteLength(view.paymentDenom) < 1 ||
    Buffer.byteLength(view.paymentDenom) > 64
  ) {
    throw new TypeError('malformed receipt paymentDenom');
  }
  const seqHash = receiptHash(
    'PAXEER/v1/sequence\0',
    r.previousStateRoot,
    receiptUint(r.globalSequence, 8),
    r.activityId
  );
  if (
    verified.height === 0n ||
    r.timestamp === 0n ||
    r.moduleId <= 11 ||
    !seqHash.equals(r.resultingStateRoot) ||
    tx.nonce === undefined ||
    r.fromSequence !== BigInt(tx.nonce) ||
    r.amount !== (tx.value ?? 0n) ||
    !receiptAddress(from).equals(r.from) ||
    !receiptAddress(tx.to ?? ZERO_ADDRESS).equals(r.to) ||
    r.operation !==
      (Number.parseInt(raw.slice(2, 4), 16) < 128
        ? Number.parseInt(raw.slice(2, 4), 16)
        : 0) ||
    toHex(verified.receiptKey.publicKey) !== view.signingKey ||
    (view.status !== '0x0' && view.status !== '0x1')
  ) {
    throw new TypeError('receipt transaction, key or sequence mismatch');
  }
  const zero = new Uint8Array(32);
  const projected: SequencerReceipt = {
    ...r,
    activityId: receiptBytes(view.txHash),
    globalSequence: receiptQuantity(view.seq),
    batchId: receiptUint(receiptQuantity(view.height), 32),
    timestamp: receiptQuantity(view.timestamp),
    fromSequence: receiptQuantity(view.nonce),
    from: receiptAddress(view.from),
    to: receiptAddress(view.to),
    asset: receiptHash(view.paymentDenom),
    activityRoot: receiptHash(receiptBytes(view.returnData)),
    resultCode: view.status === '0x1' ? 0 : 1,
    feeCharged: receiptQuantity(view.feeAmount),
    contextHash: receiptBytes(view.keyId),
    effects: [
      {
        moduleId: r.moduleId,
        ordinal: 0,
        eventType: 0,
        kind: 1,
        monetary: false,
        transferSetRoot: zero,
        body: receiptSummary(view)
      },
      ...view.logs.map((log, index) => {
        if (receiptQuantity(log.logIndex) !== BigInt(index))
          throw new TypeError('receipt logIndex mismatch');
        return {
          moduleId: r.moduleId,
          ordinal: index + 1,
          eventType: 0,
          kind: 3 as const,
          monetary: false,
          transferSetRoot: zero,
          body: encodeSequencerLogEffect(
            receiptBytes(log.address),
            log.topics.map(receiptBytes),
            receiptBytes(log.data)
          )
        };
      })
    ]
  };
  if (!canonical.equals(encodeSequencerReceipt(projected)))
    throw new TypeError('signed receipt view mismatch');
  return view;
}

export function chainMainAccountId(pubHex: string): string {
  const name = Buffer.from(`agent:did:layerx:${pubHex}:main`, 'utf8');
  const len = Buffer.alloc(4);
  len.writeUInt32BE(name.length, 0);
  return createHash('sha256').update('LX:ACCOUNT:v1').update(len).update(name).digest('hex');
}

export interface Receipt {
  transactionHash: Hex;
  status: Hex;
  gasUsed: Hex;
  blockNumber: Hex;
  logs: SignedChainReceipt['logs'];
}

export interface Fault {
  method: string;
  mode: 'error' | 'apply-then-error';
  remaining: number;
}

export interface SentTx {
  hash: Hex;
  from: string;
  to: string;
  value: bigint;
  data: Hex | undefined;
  nonce: number;
  effectiveGasPrice: bigint;
  seq: bigint;
  timestamp: bigint;
}

export class ChainRpcError extends Error {
  readonly code: number;
  constructor(code: number, message: string) {
    super(message);
    this.name = 'ChainRpcError';
    this.code = code;
  }
}

export class TestChain {
  readonly chainId: number;
  // The committed node vector generator uses this public test seed and governed pin.
  private readonly sequencerKey = createPrivateKey({
    key: Buffer.concat([
      Buffer.from('302e020100300506032b657004220420', 'hex'),
      Buffer.alloc(32, 0x5e)
    ]),
    type: 'pkcs8',
    format: 'der'
  });
  readonly sequencerPublicKey = createPublicKey(this.sequencerKey);
  readonly receiptKeyPins: readonly ChainReceiptKeyPin[] = Object.freeze([
    Object.freeze({ ...nodeReceiptVectors.cases[0]!.pinnedKey })
  ]);
  readonly sequencerReceipts = new Map<string, SignedChainReceipt>();
  readonly rawTransactions = new Map<string, Hex>();
  private sequenceHash = Buffer.alloc(32);
  readonly balances = new Map<string, bigint>();
  readonly nonces = new Map<string, number>();
  readonly bindNonces = new Map<string, bigint>();
  readonly bindings = new Map<string, string>();
  readonly bindingsByDid = new Map<string, string>();
  readonly receipts = new Map<string, Receipt>();
  readonly sent: SentTx[] = [];
  readonly faults: Fault[] = [];
  baseFee = 1_000_000_000n;
  tip = 2_000_000n;
  block = BigInt(this.receiptKeyPins[0]!.firstHeight) - 1n;
  private server: Server | null = null;
  url = '';

  constructor(
    chainId: number,
    readonly mode: 'paxeer' | 'evm' = 'paxeer'
  ) {
    this.chainId = chainId;
    if (mode === 'evm') {
      this.block = 0n;
      this.balances.set(this.faucet.address.toLowerCase(), 10n ** 24n);
    }
  }

  // Public conformance key, used only to fund EVM route tests on loopback.
  private readonly faucet = privateKeyToAccount(`0x${'23'.repeat(32)}`);

  async recordSequencerReceipt(recording: NodeReceiptVector): Promise<void> {
    const view = structuredClone(recording.receiptJson);
    const pin = recording.pinnedKey;
    const verified = await verifySequencerReceipt(
      receiptBytes(view.receipt),
      receiptBytes(recording.rawTransaction),
      [
        {
          keyId: receiptBytes(pin.keyId),
          publicKey: receiptBytes(pin.publicKey),
          first_height: BigInt(pin.firstHeight),
          last_height: BigInt(pin.lastHeight)
        }
      ]
    );
    if (!receiptBytes(recording.prevSeqHash).equals(verified.receipt.previousStateRoot)) {
      throw new TypeError('node recording prevSeqHash mismatch');
    }
    await verifyChainSequencerReceipt(view, recording.rawTransaction, [pin]);
    if (view.receipt !== recording.signedReceipt)
      throw new TypeError('node recording receipt mismatch');
    this.sequencerReceipts.set(view.txHash, view);
    this.rawTransactions.set(view.txHash, recording.rawTransaction);
    this.receipts.set(view.txHash, {
      transactionHash: view.txHash,
      status: view.status,
      gasUsed: view.gasUsed,
      blockNumber: view.height,
      logs: structuredClone(view.logs)
    });
  }

  private async signSequencerReceipt(
    raw: Hex,
    tx: SentTx,
    receipt: Receipt,
    fromBefore: bigint,
    toBefore: bigint
  ): Promise<void> {
    const pin = this.receiptKeyPins[0]!;
    const view: SignedChainReceipt = {
      rung: 'receipted',
      txHash: tx.hash,
      nonce: toHex(tx.nonce),
      seq: toHex(tx.seq),
      height: receipt.blockNumber,
      index: '0x0',
      timestamp: toHex(tx.timestamp),
      status: receipt.status,
      returnData: '0x',
      logs: structuredClone(receipt.logs),
      gasUsed: receipt.gasUsed,
      cumulativeGasUsed: receipt.gasUsed,
      effectiveGasPrice: toHex(tx.effectiveGasPrice),
      paymentDenom: 'uhpx',
      feeDenom: 'uhpx',
      feeAmount: toHex(BigInt(receipt.gasUsed) * tx.effectiveGasPrice),
      sponsor: parseTransaction(raw).type === 'eip7702' ? (tx.from as Hex) : ZERO_ADDRESS,
      from: tx.from as Hex,
      to: (tx.to || ZERO_ADDRESS) as Hex,
      contractAddress: ZERO_ADDRESS,
      vmError: '',
      keyId: pin.keyId,
      signingKey: toHex(
        this.sequencerPublicKey
          .export({ type: 'spki', format: 'der' })
          .subarray(ED_SPKI.length)
      ),
      receipt: '0x'
    };
    const resultingStateRoot = receiptHash(
      'PAXEER/v1/sequence\0',
      this.sequenceHash,
      receiptUint(tx.seq, 8),
      receiptBytes(tx.hash)
    );
    const zero = new Uint8Array(32);
    const canonical: SequencerReceipt = {
      protocolVersion: 2,
      activityId: receiptBytes(tx.hash),
      globalSequence: tx.seq,
      previousStateRoot: this.sequenceHash,
      resultingStateRoot,
      activityRoot: receiptHash(receiptBytes(view.returnData)),
      resultCode: receipt.status === '0x1' ? 0 : 1,
      effects: [
        {
          moduleId: 12,
          ordinal: 0,
          eventType: 0,
          kind: 1,
          monetary: false,
          transferSetRoot: zero,
          body: receiptSummary(view)
        },
        ...view.logs.map((log, index) => ({
          moduleId: 12,
          ordinal: index + 1,
          eventType: 0,
          kind: 3 as const,
          monetary: false,
          transferSetRoot: zero,
          body: encodeSequencerLogEffect(
            receiptBytes(log.address),
            log.topics.map(receiptBytes),
            receiptBytes(log.data)
          )
        }))
      ],
      feeCharged: BigInt(view.feeAmount),
      batchId: receiptUint(BigInt(receipt.blockNumber), 32),
      moduleId: 12,
      moduleVersion: 1,
      parameterVersion: 0,
      operation:
        parseTransaction(raw).type === 'eip7702'
          ? 4
          : parseTransaction(raw).type === 'eip1559'
            ? 2
            : 0,
      asset: receiptHash(view.paymentDenom),
      amount: tx.value,
      from: receiptAddress(view.from),
      fromBalanceBefore: fromBefore,
      fromBalanceAfter: this.bal(tx.from),
      fromSequence: BigInt(tx.nonce),
      to: receiptAddress(view.to),
      toBalanceBefore: toBefore,
      toBalanceAfter: this.bal(tx.to),
      transferSetRoot: zero,
      authorizationHash: zero,
      contextHash: receiptBytes(pin.keyId),
      timestamp: tx.timestamp,
      sequencerSignature: new Uint8Array()
    };
    const digest = receiptHash('LXP/v1/receipt\0', encodeSequencerReceipt(canonical, false));
    view.receipt = toHex(
      encodeSequencerReceipt({
        ...canonical,
        sequencerSignature: edSign(null, digest, this.sequencerKey)
      })
    );
    await verifyChainSequencerReceipt(view, raw, this.receiptKeyPins);
    this.sequencerReceipts.set(tx.hash, view);
    this.rawTransactions.set(tx.hash, raw);
    this.sequenceHash = resultingStateRoot;
  }

  fail(method: string, mode: Fault['mode'] = 'error', times = 1): void {
    this.faults.push({ method, mode, remaining: times });
  }

  private takeFault(method: string): Fault | null {
    const f = this.faults.find((x) => x.method === method && x.remaining > 0);
    if (!f) return null;
    f.remaining -= 1;
    return f;
  }

  static gasFor(to: string | undefined, data: Hex | undefined): bigint {
    let gas = 21_000n;
    const bytes = data ? Buffer.from(data.slice(2), 'hex') : Buffer.alloc(0);
    for (const b of bytes) gas += b === 0 ? 4n : 16n;
    if (to && to.toLowerCase() === ADDR && data && data.startsWith(BIND_SELECTOR))
      gas += 4_000n + 44_100n;
    return gas;
  }

  private bal(a: string): bigint {
    return this.balances.get(a.toLowerCase()) ?? 0n;
  }

  private call(
    c: { from?: string; to: string; data?: Hex; value?: Hex },
    method: string
  ): Hex {
    const to = c.to.toLowerCase();
    const data = c.data ?? '0x';
    const value = c.value ? BigInt(c.value) : 0n;
    if (
      value > 0n &&
      this.bal(c.from ?? '0x0000000000000000000000000000000000000000') < value
    ) {
      throw new Error('insufficient funds for transfer');
    }
    if (this.mode === 'evm') return '0x';
    if (to === ADDR) {
      const decoded = decodeFunctionData({ abi: addrAbiJson, data });
      if (decoded.functionName === 'getUnifiedAccount') {
        if (this.takeFault('getUnifiedAccount'))
          throw new Error('injected getUnifiedAccount failure');
        const evm = (decoded.args![0] as string).toLowerCase();
        const did = this.bindings.get(evm);
        return encodeFunctionResult({
          abi: addrAbiJson,
          functionName: 'getUnifiedAccount',
          result: [
            getAddress(evm),
            '',
            did ? `0x${did}` : `0x${'00'.repeat(32)}`,
            did ? `0x${chainMainAccountId(did)}` : `0x${'00'.repeat(32)}`
          ]
        });
      }
      if (decoded.functionName === 'layerXBindNonce') {
        const evm = (decoded.args![0] as string).toLowerCase();
        return encodeFunctionResult({
          abi: addrAbiJson,
          functionName: 'layerXBindNonce',
          result: this.bindNonces.get(evm) ?? 0n
        });
      }
      throw new Error(`addr method ${decoded.functionName} is not served`);
    }
    if (to === ANCHOR) {
      const decoded = decodeFunctionData({ abi: anchorAbiJson, data });
      if (decoded.functionName === 'latestFinalized')
        return encodeAbiParameters([{ type: 'uint64' }, { type: 'bool' }], [0n, false]);
      throw new Error(`anchor method ${decoded.functionName} is not served`);
    }
    if (PRECOMPILE_RANGE.test(to))
      throw new Error(`${method}: precompile ${to} is not served`);
    return '0x';
  }

  private applyBind(from: string, data: Hex): boolean {
    const decoded = decodeFunctionData({ abi: addrAbiJson, data });
    if (decoded.functionName !== 'bindLayerX') return false;
    const [didKey, signature] = decoded.args as [Hex, Hex];
    const pub = didKey.slice(2).toLowerCase();
    const sig = Buffer.from(signature.slice(2), 'hex');
    if (sig.length !== 64 || /^0+$/.test(pub)) return false;
    if (this.bindings.has(from) || this.bindingsByDid.has(pub)) return false;
    const nonce = this.bindNonces.get(from) ?? 0n;
    const chain = Buffer.alloc(32);
    chain.write(BigInt(this.chainId).toString(16).padStart(64, '0'), 'hex');
    const n = Buffer.alloc(8);
    n.writeBigUInt64BE(nonce, 0);
    const message = Buffer.concat([
      Buffer.from('LX:PAXEER-BIND:v1'),
      chain,
      Buffer.from(from.slice(2), 'hex'),
      n
    ]);
    let ok = false;
    try {
      const key = createPublicKey({
        key: Buffer.concat([ED_SPKI, Buffer.from(pub, 'hex')]),
        format: 'der',
        type: 'spki'
      });
      ok = edVerify(null, message, key, sig);
    } catch {
      ok = false;
    }
    if (!ok) return false;
    this.bindings.set(from, pub);
    this.bindingsByDid.set(pub, from);
    this.bindNonces.set(from, nonce + 1n);
    return true;
  }

  private async sendRaw(raw: Hex): Promise<Hex> {
    const hash = keccak256(raw);
    if (this.receipts.has(hash)) throw new Error('already known');
    const tx = parseTransaction(raw);
    const from = (
      await recoverTransactionAddress({ serializedTransaction: raw as `0x02${string}` })
    ).toLowerCase();
    if (tx.chainId !== this.chainId) throw new Error('wrong chain id');
    const expected = this.nonces.get(from) ?? 0;
    if (tx.nonce !== expected)
      throw new Error(`nonce ${tx.nonce} but account is at ${expected}`);
    const gas = tx.gas!;
    const maxFee = tx.maxFeePerGas!;
    const value = tx.value ?? 0n;
    if (maxFee < this.baseFee) throw new Error('max fee below base fee');
    if (this.bal(from) < value + gas * maxFee)
      throw new Error('insufficient funds for gas * price + value');
    const price =
      maxFee < this.baseFee + (tx.maxPriorityFeePerGas ?? 0n)
        ? maxFee
        : this.baseFee + (tx.maxPriorityFeePerGas ?? 0n);
    const to = (tx.to ?? '').toLowerCase();
    const fromBefore = this.bal(from);
    const toBefore = this.bal(to);
    const needed = TestChain.gasFor(to, tx.data);
    this.nonces.set(from, expected + 1);
    this.block += 1n;
    let status = 1n;
    let used = needed;
    if (needed > gas) {
      status = 0n;
      used = gas;
    } else if (to === ADDR && this.mode === 'paxeer') {
      if (value !== 0n || !tx.data || !this.applyBind(from, tx.data)) status = 0n;
    } else if (tx.type === 'eip7702' && this.mode === 'paxeer') {
      const { constructionDigest, sponsorQuoteDigest } =
        await import('../../src/attestor/client.js');
      if (value !== 0n || !tx.data || tx.authorizationList.length !== 1)
        throw new Error('invalid sponsored binding envelope');
      const auth = tx.authorizationList[0]!;
      const owner = await recoverAddress({
        hash: constructionDigest({
          kind: 'eip7702_authorization',
          chainId: String(auth.chainId),
          address: auth.address,
          nonce: String(auth.nonce)
        }),
        signature: serializeSignature(auth)
      });
      if (
        owner.toLowerCase() !== to ||
        auth.chainId !== this.chainId ||
        auth.nonce !== (this.nonces.get(to) ?? 0)
      )
        throw new Error('invalid sponsored binding authority');
      const execution = decodeFunctionData({ abi: sponsoredBindAbi, data: tx.data });
      if (execution.functionName !== 'executeSponsored')
        throw new Error('unsupported delegated execution');
      const [calls, quote, accountSignature, relayerSignature] = execution.args;
      const construction = {
        kind: 'sponsored_batch' as const,
        chainId: String(this.chainId),
        account: owner,
        nonce: '0',
        calls: calls.map((call) => ({ ...call, value: String(call.value) })),
        quote: {
          ...quote,
          maxTokenAmount: String(quote.maxTokenAmount),
          tokenAmount: String(quote.tokenAmount),
          deadline: String(quote.deadline),
          quoteNonce: String(quote.quoteNonce),
          gasCost: String(quote.gasCost),
          decimals: 6
        }
      };
      if (
        calls.length !== 1 ||
        calls[0]!.to.toLowerCase() !== ADDR ||
        calls[0]!.value !== 0n ||
        quote.sponsor.toLowerCase() !== from ||
        quote.tokenAmount > quote.maxTokenAmount ||
        quote.deadline <= BigInt(Math.floor(Date.now() / 1000))
      )
        throw new Error('invalid sponsored bind contract');
      if (
        (
          await recoverAddress({
            hash: constructionDigest(construction),
            signature: accountSignature
          })
        ).toLowerCase() !== to ||
        (
          await recoverAddress({
            hash: sponsorQuoteDigest(construction),
            signature: relayerSignature
          })
        ).toLowerCase() !== from
      )
        throw new Error('invalid sponsored bind signatures');
      if (!this.applyBind(to, calls[0]!.data)) throw new Error('invalid sponsored bind proof');
      this.nonces.set(to, auth.nonce + 1);
      this.balances.set(to, this.bal(to));
    } else {
      this.balances.set(to, this.bal(to) + value);
      this.balances.set(from, this.bal(from) - value);
    }
    this.balances.set(from, this.bal(from) - used * price);
    this.rawTransactions.set(hash, raw);
    this.sent.push({
      hash,
      from,
      to,
      value,
      data: tx.data,
      nonce: expected,
      effectiveGasPrice: price,
      seq: BigInt(this.sent.length + 1),
      timestamp: BigInt(Date.now())
    });
    const receipt: Receipt = {
      transactionHash: hash,
      status: `0x${status.toString(16)}`,
      gasUsed: `0x${used.toString(16)}`,
      blockNumber: `0x${this.block.toString(16)}`,
      logs:
        tx.type === 'eip7702' && this.mode === 'paxeer'
          ? [
              {
                address: ADDR,
                topics: [
                  keccak256(Buffer.from('LayerXBound(address,bytes32,uint64)')),
                  `0x${to.slice(2).padStart(64, '0')}`,
                  `0x${this.bindings.get(to)!}`
                ],
                data: toHex((this.bindNonces.get(to) ?? 1n) - 1n, { size: 32 }),
                logIndex: '0x0'
              }
            ]
          : []
    };
    if (this.mode === 'paxeer')
      await this.signSequencerReceipt(
        raw,
        this.sent[this.sent.length - 1]!,
        receipt,
        fromBefore,
        toBefore
      );
    this.receipts.set(hash, receipt);
    return hash;
  }

  private async handle(method: string, params: unknown[]): Promise<unknown> {
    const fault = this.takeFault(method);
    if (fault && fault.mode === 'error') throw new Error(`injected ${method} failure`);
    switch (method) {
      case 'eth_accounts':
        if (this.mode !== 'evm') throw new ChainRpcError(-32601, 'eth_accounts not served');
        return [this.faucet.address];
      case 'eth_sendTransaction': {
        if (this.mode !== 'evm')
          throw new ChainRpcError(-32601, 'eth_sendTransaction not served');
        const tx = params[0] as { from: Hex; to: Hex; value: Hex };
        if (tx.from.toLowerCase() !== this.faucet.address.toLowerCase())
          throw new Error('unlocked account required');
        return this.sendRaw(
          await this.faucet.signTransaction({
            type: 'eip1559',
            chainId: this.chainId,
            to: tx.to,
            value: BigInt(tx.value),
            nonce: this.nonces.get(tx.from.toLowerCase()) ?? 0,
            gas: 21_000n,
            maxFeePerGas: 2_000_000_000n,
            maxPriorityFeePerGas: 1_000_000_000n
          })
        );
      }
      case 'eth_getCode':
        if (this.mode !== 'evm') throw new ChainRpcError(-32601, 'eth_getCode not served');
        return '0x';
      case 'eth_chainId':
        return `0x${this.chainId.toString(16)}`;
      case 'eth_blockNumber':
        return `0x${this.block.toString(16)}`;
      case 'eth_getBlockByNumber':
        return {
          number: `0x${this.block.toString(16)}`,
          baseFeePerGas: `0x${this.baseFee.toString(16)}`,
          transactions: []
        };
      case 'eth_maxPriorityFeePerGas':
        return `0x${this.tip.toString(16)}`;
      case 'eth_gasPrice':
        return `0x${(this.baseFee + this.tip).toString(16)}`;
      case 'eth_getBalance':
        return `0x${this.bal(params[0] as string).toString(16)}`;
      case 'eth_getTransactionCount':
        return `0x${(this.nonces.get((params[0] as string).toLowerCase()) ?? 0).toString(16)}`;
      case 'eth_estimateGas': {
        const c = params[0] as { to?: string; data?: Hex };
        return `0x${TestChain.gasFor(c.to, c.data).toString(16)}`;
      }
      case 'eth_call':
        return this.call(
          params[0] as { from?: string; to: string; data?: Hex; value?: Hex },
          method
        );
      case 'eth_sendRawTransaction': {
        const hash = await this.sendRaw(params[0] as Hex);
        if (fault && fault.mode === 'apply-then-error')
          throw new Error('injected broadcast failure after the transaction applied');
        return hash;
      }
      case 'eth_getTransactionByHash': {
        const r = this.receipts.get(params[0] as string);
        if (!r) return null;
        if (this.mode === 'paxeer')
          return { hash: r.transactionHash, blockNumber: r.blockNumber };
        const raw = this.rawTransactions.get(params[0] as string)!;
        const tx = parseTransaction(raw);
        const sent = this.sent.find((entry) => entry.hash === r.transactionHash)!;
        return {
          hash: r.transactionHash,
          blockNumber: r.blockNumber,
          blockHash: keccak256(r.blockNumber),
          transactionIndex: '0x0',
          from: sent.from,
          to: sent.to,
          value: toHex(sent.value),
          nonce: toHex(sent.nonce),
          input: tx.data ?? '0x',
          gas: toHex(tx.gas!),
          type: '0x2',
          chainId: toHex(this.chainId),
          maxFeePerGas: toHex(tx.maxFeePerGas!),
          maxPriorityFeePerGas: toHex(tx.maxPriorityFeePerGas!),
          v: tx.v === undefined ? '0x0' : toHex(tx.v),
          r: tx.r,
          s: tx.s
        };
      }
      case 'eth_getTransactionReceipt':
        return this.receipts.get(params[0] as string) ?? null;
      case 'eth_getRawTransactionByHash':
        return this.rawTransactions.get(params[0] as string) ?? null;
      case 'px_getReceiptKeys':
        return { keys: this.receiptKeyPins };
      case 'pax_sequencerReceipt': {
        if (this.mode === 'evm')
          throw new ChainRpcError(
            -32601,
            'the method pax_sequencerReceipt does not exist/is not available'
          );
        const hash = params[0] as Hex;
        const receipt = this.sequencerReceipts.get(hash);
        if (!receipt) {
          if (this.receipts.has(hash))
            throw new Error(`applied transaction ${hash} has no signed receipt`);
          return null;
        }
        const raw = this.rawTransactions.get(hash);
        if (!raw || receipt.txHash !== hash)
          throw new TypeError('receipt transaction mismatch');
        return verifyChainSequencerReceipt(receipt, raw, this.receiptKeyPins);
      }
      default:
        throw new ChainRpcError(
          -32601,
          `the method ${method} does not exist/is not available`
        );
    }
  }

  async start(): Promise<void> {
    this.server = createHttpServer((req, res) => {
      const chunks: Buffer[] = [];
      req.on('data', (c: Buffer) => chunks.push(c));
      req.on('end', () => {
        const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
          id: number;
          method: string;
          params: unknown[];
        };
        this.handle(body.method, body.params ?? []).then(
          (result) => {
            res.writeHead(200, { 'content-type': 'application/json' });
            res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
          },
          (err: Error) => {
            const code = err instanceof ChainRpcError ? err.code : -32000;
            res.writeHead(200, { 'content-type': 'application/json' });
            res.end(
              JSON.stringify({
                jsonrpc: '2.0',
                id: body.id,
                error: { code, message: err.message }
              })
            );
          }
        );
      });
    });
    await new Promise<void>((r) => this.server!.listen(0, '127.0.0.1', r));
    const addr = this.server.address();
    if (!addr || typeof addr === 'string') throw new Error('chain server did not bind');
    this.url = `http://127.0.0.1:${addr.port}`;
  }

  async stop(): Promise<void> {
    if (this.server) await new Promise<void>((r) => this.server!.close(() => r()));
  }
}

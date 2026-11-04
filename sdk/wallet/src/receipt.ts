import { ed25519 } from '@noble/curves/ed25519.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, hexToBytes, concatBytes } from 'viem';
import { LADDER_RUNGS, type TransferRpc } from './ladder.js';
import { isRecord } from './rpc.js';
import type {
  Hex,
  ReceiptKeyPin,
  SequencerReceiptAnswer,
  SequencerReceiptView,
  SequencerRung,
} from './types.js';

const HASH = /^0x[0-9a-fA-F]{64}$/;
const ADDRESS = /^0x[0-9a-fA-F]{40}$/;
const QUANTITY = /^0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)$/;
const BYTES = /^0x(?:[0-9a-fA-F]{2})*$/;
const U64_MAX = (1n << 64n) - 1n;
const encoder = new TextEncoder();

function invalid(field: string): never {
  throw new TypeError(`malformed or unauthenticated sequencer receipt ${field}`);
}
function decodeHex(value: unknown, pattern: RegExp, field: string): Hex {
  if (typeof value !== 'string' || !pattern.test(value)) return invalid(field);
  return value as Hex;
}
function equalBytes(a: Uint8Array, b: Uint8Array): boolean {
  return a.length === b.length && a.every((v, i) => v === b[i]);
}
function pinHeight(value: unknown): bigint {
  if (typeof value === 'string' && /^(0|[1-9][0-9]*)$/.test(value)) {
    const height = BigInt(value);
    if (height >= 1n && height <= U64_MAX) return height;
  }
  if (typeof value === 'number' && Number.isSafeInteger(value) && value >= 1) return BigInt(value);
  return invalid('key height');
}
export async function readReceiptKeyPins(rpc: TransferRpc): Promise<readonly ReceiptKeyPin[]> {
  const value = await rpc('px_getReceiptKeys', []);
  if (!isRecord(value) || !Array.isArray(value.keys)) return invalid('governed keys');
  const seen = new Set<string>();
  return value.keys.map((entry: unknown) => {
    if (!isRecord(entry)) return invalid('governed key');
    const keyId = decodeHex(entry.keyId, HASH, 'keyId');
    const publicKey = decodeHex(entry.publicKey, HASH, 'publicKey');
    if (BigInt(keyId) === 0n) return invalid('zero keyId');
    const firstHeight = pinHeight(entry.firstHeight);
    const lastHeight = pinHeight(entry.lastHeight);
    if (firstHeight > lastHeight || lastHeight > U64_MAX || seen.has(keyId.toLowerCase()))
      return invalid('key range');
    seen.add(keyId.toLowerCase());
    return Object.freeze({ keyId, publicKey, firstHeight, lastHeight });
  });
}

class ReceiptReader {
  offset = 0;
  constructor(readonly bytes: Uint8Array) {}
  take(length: number): Uint8Array {
    if (!Number.isSafeInteger(length) || length < 0 || this.offset + length > this.bytes.length)
      return invalid('truncated bytes');
    const out = this.bytes.subarray(this.offset, this.offset + length);
    this.offset += length;
    return out;
  }
  uint(length: number): bigint {
    return this.take(length).reduce((n, byte) => (n << 8n) | BigInt(byte), 0n);
  }
  word(): Uint8Array {
    if (this.uint(4) !== 32n) return invalid('word length');
    return this.take(32);
  }
}

function verifyView(
  view: SequencerReceiptView,
  txHash: Hex,
  nonce: Hex,
  pins: readonly ReceiptKeyPin[]
): void {
  const bytes = hexToBytes(view.receipt);
  if (bytes.length > 1_048_576) return invalid('length');
  const r = new ReceiptReader(bytes);
  if (r.uint(2) !== 2n || r.uint(2) !== 0x5201n || r.uint(2) !== 2n) return invalid('layout');
  const check = (field: string, actual: string, expected: string): void => {
    if (actual.toLowerCase() !== expected.toLowerCase()) invalid(field);
  };
  const number = (field: string, actual: Hex, expected: bigint): void => {
    if (BigInt(actual) !== expected) invalid(field);
  };
  check('txHash', txHash, bytesToHex(r.word()));
  if (BigInt(txHash) === 0n) return invalid('zero transaction');
  const seq = r.uint(8);
  number('seq', view.seq, seq);
  r.word();
  if (r.word().every((b) => b === 0)) return invalid('resulting root');
  const outputDigest = r.word();
  const result = r.uint(4);
  if (result > 1n) return invalid('result code');
  number('status', view.status, result === 0n ? 1n : 0n);
  const count = Number(r.uint(4));
  if (count < 1 || count > Math.floor(bytes.length / 48)) return invalid('effect count');
  const effects: { module: bigint; body: Uint8Array }[] = [];
  for (let i = 0; i < count; i += 1) {
    const module = r.uint(2);
    if (module === 0n || r.uint(2) !== BigInt(i)) return invalid('effect ordinal');
    r.uint(2);
    if (r.uint(1) !== (i === 0 ? 1n : 3n) || r.uint(1) !== 0n) return invalid('effect kind');
    r.word();
    const size = Number(r.uint(4));
    if (size > 256) return invalid('effect body');
    effects.push({ module, body: r.take(size) });
  }
  number('feeAmount', view.feeAmount, r.uint(16));
  const batch = r.word();
  if (batch.subarray(0, 24).some((b) => b !== 0)) return invalid('batch height');
  const height = new ReceiptReader(batch.subarray(24)).uint(8);
  number('height', view.height, height);
  const module = r.uint(2);
  if (
    module === 0n ||
    effects.some((e) => e.module !== module) ||
    r.uint(4) !== 1n ||
    r.uint(4) !== 0n
  )
    return invalid('module layout');
  if (count > 512 && module <= 11n) return invalid('effect bound');
  r.uint(1);
  if (!equalBytes(r.word(), sha256(encoder.encode(view.paymentDenom))))
    return invalid('paymentDenom');
  r.uint(16);
  const address = (field: string, actual: Hex): void => {
    const word = r.word();
    if (word.subarray(0, 12).some((b) => b !== 0)) invalid(field);
    check(field, actual, bytesToHex(word.subarray(12)));
  };
  address('from', view.from);
  r.uint(16);
  r.uint(16);
  number('nonce', nonce, r.uint(8));
  address('to', view.to);
  r.uint(16);
  r.uint(16);
  r.word();
  r.word();
  check('keyId', view.keyId, bytesToHex(r.word()));
  const timestamp = r.uint(8);
  number('timestamp', view.timestamp, timestamp);
  if (seq === 0n || timestamp === 0n) return invalid('zero sequence or timestamp');
  const signedEnd = r.offset;
  if (r.uint(1) !== 1n || r.uint(4) !== 64n) return invalid('signature presence');
  const signature = r.take(64);
  if (r.offset !== bytes.length) return invalid('trailing bytes');
  const matches = pins.filter((pin) => pin.keyId.toLowerCase() === view.keyId.toLowerCase());
  if (matches.length !== 1) return invalid('governed key pin');
  const pin = matches[0]!;
  if (
    pin.firstHeight < 1n ||
    pin.lastHeight > U64_MAX ||
    height < pin.firstHeight ||
    height > pin.lastHeight
  )
    return invalid('key height range');
  check('signingKey', view.signingKey, pin.publicKey);
  const digest = sha256(
    concatBytes([
      encoder.encode('LXP/v1/receipt\0'),
      bytes.subarray(0, signedEnd),
      new Uint8Array([0]),
    ])
  );
  if (!ed25519.verify(signature, digest, hexToBytes(view.signingKey), { zip215: false }))
    return invalid('signature');
  const summary = new ReceiptReader(effects[0]!.body);
  number('gasUsed', view.gasUsed, summary.uint(8));
  number('cumulativeGasUsed', view.cumulativeGasUsed, summary.uint(8));
  number('effectiveGasPrice', view.effectiveGasPrice, summary.uint(8));
  number('index', view.index, summary.uint(4));
  check('contractAddress', view.contractAddress, bytesToHex(summary.take(20)));
  check('sponsor', view.sponsor, bytesToHex(summary.take(20)));
  if (!equalBytes(summary.take(32), sha256(encoder.encode(view.vmError))))
    return invalid('vmError digest');
  if (
    summary.uint(4) !== BigInt(hexToBytes(view.returnData).length) ||
    !equalBytes(outputDigest, sha256(hexToBytes(view.returnData)))
  )
    return invalid('returnData digest');
  if (summary.uint(4) !== BigInt(count - 1) || view.logs.length !== count - 1)
    return invalid('log count');
  const denomLength = Number(summary.uint(1));
  if (
    denomLength < 1 ||
    denomLength > 64 ||
    !equalBytes(summary.take(denomLength), encoder.encode(view.feeDenom)) ||
    summary.offset !== summary.bytes.length
  )
    return invalid('feeDenom');
  view.logs.forEach((log, i) => {
    const body = new ReceiptReader(effects[i + 1]!.body);
    check('log address', log.address, bytesToHex(body.take(20)));
    const topics = Number(body.uint(1));
    if (topics > 4 || topics !== log.topics.length) invalid('log topics');
    log.topics.forEach((topic) => check('log topic', topic, bytesToHex(body.take(32))));
    if (
      !equalBytes(body.take(32), sha256(hexToBytes(log.data))) ||
      body.offset !== body.bytes.length
    )
      invalid('log data digest');
    number('logIndex', log.logIndex, BigInt(i));
  });
}

export function decodeSequencerReceipt(
  value: unknown,
  expectedHash?: string,
  pins: readonly ReceiptKeyPin[] = []
): SequencerReceiptAnswer {
  if (!isRecord(value)) return invalid('answer');
  const { rung } = value;
  if (rung !== 'submitted' && rung !== 'receipted' && rung !== 'committed') return invalid('rung');
  const txHash = decodeHex(value.txHash, HASH, 'txHash');
  if (
    expectedHash !== undefined &&
    txHash.toLowerCase() !== decodeHex(expectedHash, HASH, 'expected hash').toLowerCase()
  )
    throw new TypeError('sequencer receipt txHash differs from the asked hash');
  const nonce = decodeHex(value.nonce, QUANTITY, 'nonce');
  if (BigInt(nonce) > U64_MAX) return invalid('nonce range');
  if (rung === 'submitted') {
    if (Object.keys(value).some((key) => !['rung', 'txHash', 'nonce'].includes(key)))
      return invalid('submitted fields');
    return Object.freeze({ rung, txHash, nonce });
  }
  const view = {} as SequencerReceiptView;
  for (const field of [
    'seq',
    'height',
    'index',
    'timestamp',
    'status',
    'gasUsed',
    'cumulativeGasUsed',
    'effectiveGasPrice',
    'feeAmount',
  ] as const)
    view[field] = decodeHex(value[field], QUANTITY, field);
  for (const field of ['returnData', 'receipt'] as const)
    view[field] = decodeHex(value[field], BYTES, field);
  for (const field of ['sponsor', 'from', 'to', 'contractAddress'] as const)
    view[field] = decodeHex(value[field], ADDRESS, field);
  for (const field of ['keyId', 'signingKey'] as const)
    view[field] = decodeHex(value[field], HASH, field);
  for (const field of ['paymentDenom', 'feeDenom', 'vmError'] as const) {
    if (typeof value[field] !== 'string') return invalid(field);
    view[field] = value[field];
  }
  if (encoder.encode(view.paymentDenom).length < 1 || encoder.encode(view.paymentDenom).length > 64)
    return invalid('paymentDenom');
  if (!Array.isArray(value.logs)) return invalid('logs');
  view.logs = value.logs.map((log: unknown) => {
    if (!isRecord(log) || !Array.isArray(log.topics)) return invalid('log');
    return {
      address: decodeHex(log.address, ADDRESS, 'log address'),
      topics: log.topics.map((topic: unknown) => decodeHex(topic, HASH, 'log topic')),
      data: decodeHex(log.data, BYTES, 'log data'),
      logIndex: decodeHex(log.logIndex, QUANTITY, 'logIndex'),
    };
  });
  verifyView(view, txHash, nonce, pins);
  for (const log of view.logs) {
    Object.freeze(log.topics);
    Object.freeze(log);
  }
  Object.freeze(view.logs);
  if (rung === 'receipted') {
    if (value.blockNumber !== undefined || value.blockHash !== undefined)
      return invalid('receipted block');
    return Object.freeze({ ...view, rung, txHash, nonce });
  }
  const blockNumber = decodeHex(value.blockNumber, QUANTITY, 'blockNumber');
  const blockHash = decodeHex(value.blockHash, HASH, 'blockHash');
  if (BigInt(blockNumber) > U64_MAX || BigInt(blockNumber) < BigInt(view.height))
    return invalid('committed height');
  return Object.freeze({ ...view, rung, txHash, nonce, blockNumber, blockHash });
}

export async function authenticateSequencerReceipt(
  rpc: TransferRpc,
  value: unknown,
  expectedHash?: string
): Promise<SequencerReceiptAnswer> {
  const pins = isRecord(value) && value.rung === 'submitted' ? [] : await readReceiptKeyPins(rpc);
  return decodeSequencerReceipt(value, expectedHash, pins);
}

export async function readSequencerReceipt(
  rpc: TransferRpc,
  hash: string
): Promise<SequencerReceiptAnswer | null> {
  decodeHex(hash, HASH, 'transaction hash');
  const value = await rpc('pax_sequencerReceipt', [hash]);
  return value === null ? null : authenticateSequencerReceipt(rpc, value, hash);
}

export class ReceiptTimeoutError extends Error {
  constructor(readonly lastAnswer: SequencerReceiptAnswer | null) {
    super('sequencer receipt did not reach the requested rung within the allotted attempts');
    this.name = 'ReceiptTimeoutError';
  }
}

export interface WaitForSequencerReceiptOptions {
  rung: SequencerRung;
  attempts?: number;
  intervalMs?: number;
  signal?: AbortSignal;
}

async function abortableReceiptWait<T>(
  operation: () => Promise<T>,
  signal?: AbortSignal
): Promise<T> {
  if (signal?.aborted) throw new DOMException('sequencer receipt wait aborted', 'AbortError');
  if (!signal) return operation();
  let onAbort: (() => void) | undefined;
  try {
    return await new Promise<T>((resolve, reject) => {
      onAbort = () => reject(new DOMException('sequencer receipt wait aborted', 'AbortError'));
      signal.addEventListener('abort', onAbort, { once: true });
      operation().then(resolve, reject);
    });
  } finally {
    if (onAbort) signal.removeEventListener('abort', onAbort);
  }
}

export async function waitForSequencerReceipt(
  rpc: TransferRpc,
  hash: string,
  { rung, attempts = 30, intervalMs = 1000, signal }: WaitForSequencerReceiptOptions
): Promise<SequencerReceiptAnswer> {
  if (signal?.aborted) throw new DOMException('sequencer receipt wait aborted', 'AbortError');
  const targetRank = LADDER_RUNGS.indexOf(rung);
  if (targetRank < 0) throw new RangeError('unknown sequencer receipt rung');
  if (!Number.isSafeInteger(attempts) || attempts < 0) {
    throw new RangeError('attempts must be a non-negative safe integer');
  }
  if (!Number.isFinite(intervalMs) || intervalMs < 0 || intervalMs > 2147483647) {
    throw new RangeError('intervalMs must be between 0 and 2147483647');
  }
  decodeHex(hash, HASH, 'transaction hash');
  let lastAnswer: SequencerReceiptAnswer | null = null;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    lastAnswer = await abortableReceiptWait(() => readSequencerReceipt(rpc, hash), signal);
    if (signal?.aborted) throw new DOMException('sequencer receipt wait aborted', 'AbortError');
    if (lastAnswer !== null && LADDER_RUNGS.indexOf(lastAnswer.rung) >= targetRank)
      return lastAnswer;
    if (attempt + 1 < attempts) {
      let timer: ReturnType<typeof setTimeout> | undefined;
      try {
        await abortableReceiptWait(
          () =>
            new Promise<void>((resolve) => {
              timer = setTimeout(resolve, intervalMs);
            }),
          signal
        );
      } finally {
        clearTimeout(timer);
      }
    }
  }
  throw new ReceiptTimeoutError(lastAnswer);
}

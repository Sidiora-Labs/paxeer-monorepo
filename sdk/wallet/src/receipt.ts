import { ed25519 } from '@noble/curves/ed25519.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { verifySequencerReceipt, type SequencerReceipt } from '@sidiora/layerx-sdk/browser';
import {
  bytesToHex,
  hexToBytes,
  concatBytes,
  parseTransaction,
  recoverTransactionAddress,
  type TransactionSerialized,
} from 'viem';
import { LADDER_RUNGS, type TransferRpc } from './ladder.js';
import { isRecord, type JsonRpcRequestOptions } from './rpc.js';
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
export async function readReceiptKeyPins(
  rpc: TransferRpc,
  options: JsonRpcRequestOptions = {}
): Promise<readonly ReceiptKeyPin[]> {
  options.signal?.throwIfAborted();
  const value = await rpc('px_getReceiptKeys', [], options);
  options.signal?.throwIfAborted();
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

export function encodeKernelReceipt(receipt: SequencerReceipt, signed = true): Uint8Array {
  const parts: Uint8Array[] = [];
  const uint = (value: bigint | number, width: number): void => {
    let remaining = BigInt(value);
    if (remaining < 0n || remaining >= 1n << BigInt(width * 8)) invalid('integer range');
    const bytes = new Uint8Array(width);
    for (let index = width - 1; index >= 0; index -= 1) {
      bytes[index] = Number(remaining & 255n);
      remaining >>= 8n;
    }
    parts.push(bytes);
  };
  const bytes = (value: Uint8Array, length?: number, maximum = 1_048_576): void => {
    if ((length !== undefined && value.length !== length) || value.length > maximum)
      invalid('byte length');
    uint(value.length, 4);
    parts.push(value);
  };
  if (receipt.protocolVersion !== 2 || receipt.effects.length > 512) invalid('layout');
  uint(2, 2);
  uint(receipt.totalUnits === undefined ? 0x5201 : 0x5202, 2);
  uint(receipt.protocolVersion, 2);
  bytes(receipt.activityId, 32);
  uint(receipt.globalSequence, 8);
  bytes(receipt.previousStateRoot, 32);
  bytes(receipt.resultingStateRoot, 32);
  bytes(receipt.activityRoot, 32);
  if (
    !Number.isInteger(receipt.resultCode) ||
    receipt.resultCode < -2147483648 ||
    receipt.resultCode > 2147483647
  )
    invalid('result code');
  uint(BigInt.asUintN(32, BigInt(receipt.resultCode)), 4);
  uint(receipt.effects.length, 4);
  for (const effect of receipt.effects) {
    if (![1, 2, 3].includes(effect.kind) || (effect.monetary && effect.kind !== 2))
      invalid('effect kind');
    uint(effect.moduleId, 2);
    uint(effect.ordinal, 2);
    uint(effect.eventType, 2);
    uint(effect.kind, 1);
    uint(effect.monetary ? 1 : 0, 1);
    bytes(effect.transferSetRoot, 32);
    bytes(effect.body, undefined, 256);
  }
  uint(receipt.feeCharged, 16);
  bytes(receipt.batchId, 32);
  uint(receipt.moduleId, 2);
  uint(receipt.moduleVersion, 4);
  uint(receipt.parameterVersion, 4);
  uint(receipt.operation, 1);
  bytes(receipt.asset, 32);
  uint(receipt.amount, 16);
  bytes(receipt.from, 32);
  uint(receipt.fromBalanceBefore, 16);
  uint(receipt.fromBalanceAfter, 16);
  uint(receipt.fromSequence, 8);
  bytes(receipt.to, 32);
  uint(receipt.toBalanceBefore, 16);
  uint(receipt.toBalanceAfter, 16);
  bytes(receipt.transferSetRoot, 32);
  bytes(receipt.authorizationHash, 32);
  bytes(receipt.contextHash, 32);
  uint(receipt.timestamp, 8);
  if (receipt.totalUnits !== undefined) {
    uint(receipt.totalUnits[0], 16);
    uint(receipt.totalUnits[1], 16);
  }
  uint(signed ? 1 : 0, 1);
  if (signed) bytes(receipt.sequencerSignature, 64);
  const encoded = concatBytes(parts);
  if (encoded.length > 1_048_576) invalid('length');
  return encoded;
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

export type AuthenticatedSequencerReceiptAnswer = Exclude<
  SequencerReceiptAnswer,
  { rung: 'submitted' }
> & {
  readonly authentication: Readonly<{
    level: 'sequencer-signed';
    keyId: Hex;
    signingKey: Hex;
    height: bigint;
    receiptDigest: Hex;
    rawTransaction: Hex;
  }>;
};

const authenticatedReceipts = new WeakSet<object>();

export function requireAuthenticatedReceipt(
  receipt: unknown
): asserts receipt is AuthenticatedSequencerReceiptAnswer {
  if (!isRecord(receipt) || !authenticatedReceipts.has(receipt)) {
    throw new TypeError('sequencer receipt authentication required');
  }
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

function receiptMismatch(field: string): never {
  throw new TypeError(`sequencer receipt authentication mismatch: ${field}`);
}

function receiptBytes(value: unknown, field: string): Uint8Array {
  return hexToBytes(decodeHex(value, BYTES, field));
}

function equalHex(value: unknown, expected: Uint8Array, field: string): void {
  if (decodeHex(value, BYTES, field).toLowerCase() !== bytesToHex(expected)) receiptMismatch(field);
}

function equalQuantity(value: unknown, expected: bigint, field: string): void {
  if (BigInt(decodeHex(value, QUANTITY, field)) !== expected) receiptMismatch(field);
}

export async function authenticateSequencerReceipt(
  rpc: TransferRpc,
  value: unknown,
  expectedHash?: string,
  options: JsonRpcRequestOptions = {}
): Promise<SequencerReceiptAnswer> {
  options.signal?.throwIfAborted();
  // Snapshot untrusted input before consulting governed keys over the transport.
  const snapshot: unknown = structuredClone(value);
  const pins = await readReceiptKeyPins(rpc, options);
  const answer = decodeSequencerReceipt(snapshot, expectedHash, pins);
  if (answer.rung === 'submitted') return receiptMismatch('receipt rung');
  const canonical = receiptBytes(answer.receipt, 'receipt');
  const keys = pins.map((pin) => ({
    keyId: receiptBytes(pin.keyId, 'keyId'),
    publicKey: receiptBytes(pin.publicKey, 'publicKey'),
    first_height: pin.firstHeight,
    last_height: pin.lastHeight,
  }));
  options.signal?.throwIfAborted();
  const raw = await rpc('eth_getRawTransactionByHash', [answer.txHash], options);
  options.signal?.throwIfAborted();
  const rawTransaction = decodeHex(raw, BYTES, 'raw transaction');
  const verified = await verifySequencerReceipt(
    canonical,
    receiptBytes(rawTransaction, 'raw transaction'),
    keys
  );
  const { receipt, height } = verified;
  if (!equalBytes(encodeKernelReceipt(receipt), canonical))
    return receiptMismatch('canonical bytes');
  const transaction = parseTransaction(rawTransaction);
  const sender = await recoverTransactionAddress({
    serializedTransaction: rawTransaction as TransactionSerialized,
  });
  const envelopeTag = Number.parseInt(rawTransaction.slice(2, 4), 16);
  const operation = envelopeTag < 0x80 ? envelopeTag : 0;
  if (
    transaction.chainId !== 125 ||
    transaction.nonce === undefined ||
    !transaction.to ||
    receipt.fromSequence !== BigInt(transaction.nonce) ||
    receipt.moduleId <= 11 ||
    receipt.effects.length === 0 ||
    receipt.operation !== operation ||
    receipt.transferSetRoot.some((byte) => byte !== 0) ||
    receipt.resultCode < 0 ||
    receipt.resultCode > 1 ||
    height === 0n
  ) {
    return receiptMismatch('transaction or receipt layout');
  }
  const pin = keys.find(
    (key) =>
      bytesToHex(key.keyId) === bytesToHex(receipt.contextHash) &&
      key.first_height <= height &&
      height <= key.last_height
  );
  if (!pin) return receiptMismatch('ReceiptKeys');
  equalHex(answer.signingKey, pin.publicKey, 'signingKey');
  equalHex(answer.keyId, receipt.contextHash, 'keyId');
  equalHex(answer.txHash, receipt.activityId, 'txHash');
  const fromWord = receipt.from;
  const toWord = receipt.to;
  if (
    fromWord.subarray(0, 12).some((byte) => byte !== 0) ||
    toWord.subarray(0, 12).some((byte) => byte !== 0) ||
    bytesToHex(fromWord.subarray(12)) !== sender.toLowerCase() ||
    bytesToHex(toWord.subarray(12)) !== transaction.to.toLowerCase() ||
    receipt.amount !== (transaction.value ?? 0n)
  )
    return receiptMismatch('transaction fields');
  equalHex(answer.from, fromWord.subarray(12), 'from');
  equalHex(answer.to, toWord.subarray(12), 'to');
  equalQuantity(answer.nonce, receipt.fromSequence, 'nonce');
  const sequence = new Uint8Array(8);
  new DataView(sequence.buffer).setBigUint64(0, receipt.globalSequence, false);
  const sequenceInput = new Uint8Array([
    ...new TextEncoder().encode('PAXEER/v1/sequence\0'),
    ...receipt.previousStateRoot,
    ...sequence,
    ...receipt.activityId,
  ]);
  if (bytesToHex(sha256(sequenceInput)) !== bytesToHex(receipt.resultingStateRoot)) {
    return receiptMismatch('sequence hash');
  }
  for (const [index, effect] of receipt.effects.entries()) {
    if (
      effect.moduleId !== receipt.moduleId ||
      effect.ordinal !== index ||
      effect.eventType !== 0 ||
      effect.kind !== (index === 0 ? 1 : 3) ||
      effect.monetary ||
      effect.transferSetRoot.some((byte) => byte !== 0)
    )
      return receiptMismatch('effect layout');
  }
  const body = receipt.effects[0]!.body;
  if (body.length < 109) return receiptMismatch('summary');
  const summary = new DataView(body.buffer, body.byteOffset, body.byteLength);
  const denomLength = body[108]!;
  if (denomLength === 0 || denomLength > 64 || body.length !== 109 + denomLength) {
    return receiptMismatch('feeDenom length');
  }
  const feeDenom = new TextDecoder('utf-8', { fatal: true }).decode(body.subarray(109));
  if (answer.feeDenom !== feeDenom) return receiptMismatch('feeDenom');
  if (
    typeof answer.paymentDenom !== 'string' ||
    answer.paymentDenom.length === 0 ||
    new TextEncoder().encode(answer.paymentDenom).length > 64
  )
    return receiptMismatch('paymentDenom');
  equalHex(
    bytesToHex(sha256(new TextEncoder().encode(answer.paymentDenom))),
    receipt.asset,
    'payment asset'
  );
  const quantities = {
    seq: receipt.globalSequence,
    height,
    index: BigInt(summary.getUint32(24, false)),
    timestamp: receipt.timestamp,
    status: receipt.resultCode === 0 ? 1n : 0n,
    gasUsed: summary.getBigUint64(0, false),
    cumulativeGasUsed: summary.getBigUint64(8, false),
    effectiveGasPrice: summary.getBigUint64(16, false),
    feeAmount: receipt.feeCharged,
  };
  for (const [field, expected] of Object.entries(quantities)) {
    equalQuantity(answer[field as keyof typeof quantities], expected, field);
  }
  equalHex(answer.contractAddress, body.subarray(28, 48), 'contractAddress');
  equalHex(answer.sponsor, body.subarray(48, 68), 'sponsor');
  if (typeof answer.vmError !== 'string') return receiptMismatch('vmError');
  equalHex(
    bytesToHex(sha256(new TextEncoder().encode(answer.vmError))),
    body.subarray(68, 100),
    'vmError digest'
  );
  const returnData = receiptBytes(answer.returnData, 'returnData');
  if (returnData.length !== summary.getUint32(100, false))
    return receiptMismatch('returnData length');
  equalHex(bytesToHex(sha256(returnData)), receipt.activityRoot, 'returnData digest');
  const logCount = summary.getUint32(104, false);
  if (!answer.logs || answer.logs.length !== logCount || receipt.effects.length !== logCount + 1) {
    return receiptMismatch('log count');
  }
  for (const [index, log] of answer.logs.entries()) {
    const logBody = receipt.effects[index + 1]!.body;
    const topicCount = logBody[20];
    if (
      topicCount === undefined ||
      topicCount > 4 ||
      logBody.length !== 53 + topicCount * 32 ||
      log.topics.length !== topicCount
    )
      return receiptMismatch('log layout');
    equalHex(log.address, logBody.subarray(0, 20), 'log address');
    equalQuantity(log.logIndex, BigInt(index), 'log index');
    for (const [topic, value] of log.topics.entries()) {
      equalHex(value, logBody.subarray(21 + topic * 32, 53 + topic * 32), 'log topic');
    }
    equalHex(
      bytesToHex(sha256(receiptBytes(log.data, 'log data'))),
      logBody.subarray(logBody.length - 32),
      'log data digest'
    );
  }
  if (answer.rung === 'committed') {
    if (BigInt(answer.blockNumber!) < height) return receiptMismatch('committed height');
  } else if (answer.blockNumber !== undefined || answer.blockHash !== undefined) {
    return receiptMismatch('receipted block');
  }
  for (const log of answer.logs) {
    Object.freeze(log.topics);
    Object.freeze(log);
  }
  Object.freeze(answer.logs);
  const authenticated = Object.freeze({
    ...answer,
    authentication: Object.freeze({
      level: 'sequencer-signed' as const,
      keyId: answer.keyId!,
      signingKey: answer.signingKey!,
      height,
      receiptDigest: bytesToHex(verified.receiptDigest),
      rawTransaction,
    }),
  });
  authenticatedReceipts.add(authenticated);
  return authenticated;
}

export function matchReceiptTransaction(
  receipt: SequencerReceiptAnswer,
  from: string,
  tx: { readonly to: string; readonly data: string; readonly value: bigint }
): void {
  requireAuthenticatedReceipt(receipt);
  const transaction = parseTransaction(receipt.authentication.rawTransaction);
  if (
    receipt.from?.toLowerCase() !== from.toLowerCase() ||
    transaction.to?.toLowerCase() !== tx.to.toLowerCase() ||
    (transaction.data ?? '0x').toLowerCase() !== tx.data.toLowerCase() ||
    (transaction.value ?? 0n) !== tx.value
  )
    return receiptMismatch('requested transaction');
}

export async function readSequencerReceipt(
  rpc: TransferRpc,
  hash: string,
  options: JsonRpcRequestOptions = {}
): Promise<SequencerReceiptAnswer | null> {
  options.signal?.throwIfAborted();
  decodeHex(hash, HASH, 'transaction hash');
  const value = await rpc('pax_sequencerReceipt', [hash], options);
  options.signal?.throwIfAborted();
  if (value === null) return null;
  if (isRecord(value) && value.rung === 'submitted') return decodeSequencerReceipt(value, hash);
  return authenticateSequencerReceipt(rpc, value, hash, options);
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
    lastAnswer = await abortableReceiptWait(
      () => readSequencerReceipt(rpc, hash, { signal }),
      signal
    );
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

import { LADDER_RUNGS, type TransferRpc } from './ladder.js';
import { isRecord } from './rpc.js';
import type { Hex, SequencerReceiptAnswer, SequencerRung } from './types.js';

const HASH = /^0x[0-9a-fA-F]{64}$/;
const ADDRESS = /^0x[0-9a-fA-F]{40}$/;
const QUANTITY = /^0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)$/;
const BYTES = /^0x(?:[0-9a-fA-F]{2})*$/;

function decodeHex(value: unknown, pattern: RegExp, field: string): Hex {
  if (typeof value !== 'string' || !pattern.test(value)) {
    throw new TypeError(`malformed sequencer receipt ${field}`);
  }
  return value as Hex;
}

export function decodeSequencerReceipt(
  value: unknown,
  expectedHash?: string
): SequencerReceiptAnswer {
  if (!isRecord(value)) throw new TypeError('malformed sequencer receipt');
  const { rung } = value;
  if (rung !== 'submitted' && rung !== 'receipted' && rung !== 'committed') {
    throw new TypeError('malformed sequencer receipt rung');
  }
  const txHash = decodeHex(value.txHash, HASH, 'txHash');
  if (
    expectedHash !== undefined &&
    txHash.toLowerCase() !== decodeHex(expectedHash, HASH, 'expected hash').toLowerCase()
  ) {
    throw new TypeError('sequencer receipt txHash differs from the asked hash');
  }
  const answer: SequencerReceiptAnswer = {
    rung,
    txHash,
    nonce: decodeHex(value.nonce, QUANTITY, 'nonce'),
  };
  if (rung === 'committed' || value.blockNumber !== undefined) {
    answer.blockNumber = decodeHex(value.blockNumber, QUANTITY, 'blockNumber');
  }
  if (rung === 'committed' || value.blockHash !== undefined) {
    answer.blockHash = decodeHex(value.blockHash, HASH, 'blockHash');
  }
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
  ] as const) {
    if (value[field] !== undefined) answer[field] = decodeHex(value[field], QUANTITY, field);
  }
  for (const field of ['returnData', 'receipt'] as const) {
    if (value[field] !== undefined) answer[field] = decodeHex(value[field], BYTES, field);
  }
  for (const field of ['sponsor', 'from', 'to', 'contractAddress'] as const) {
    if (value[field] !== undefined) answer[field] = decodeHex(value[field], ADDRESS, field);
  }
  if (value.keyId !== undefined) answer.keyId = decodeHex(value.keyId, HASH, 'keyId');
  for (const field of ['feeDenom', 'vmError'] as const) {
    if (value[field] !== undefined) {
      if (typeof value[field] !== 'string')
        throw new TypeError(`malformed sequencer receipt ${field}`);
      answer[field] = value[field];
    }
  }
  if (value.logs !== undefined) {
    if (!Array.isArray(value.logs)) throw new TypeError('malformed sequencer receipt logs');
    answer.logs = value.logs.map((log: unknown) => {
      if (!isRecord(log) || !Array.isArray(log.topics))
        throw new TypeError('malformed sequencer receipt log');
      return {
        address: decodeHex(log.address, ADDRESS, 'log address'),
        topics: log.topics.map((topic: unknown) => decodeHex(topic, HASH, 'log topic')),
        data: decodeHex(log.data, BYTES, 'log data'),
        logIndex: decodeHex(log.logIndex, QUANTITY, 'logIndex'),
      };
    });
  }
  return answer;
}

export async function readSequencerReceipt(
  rpc: TransferRpc,
  hash: string
): Promise<SequencerReceiptAnswer | null> {
  decodeHex(hash, HASH, 'transaction hash');
  const value = await rpc('pax_sequencerReceipt', [hash]);
  return value === null ? null : decodeSequencerReceipt(value, hash);
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

import type { TransferRpc } from './ladder.js';
import { isRecord } from './rpc.js';
import type { Hex, SequencerReceiptAnswer } from './types.js';

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

export function decodeSequencerReceipt(value: unknown, expectedHash?: string): SequencerReceiptAnswer {
  if (!isRecord(value)) throw new TypeError('malformed sequencer receipt');
  const { rung } = value;
  if (rung !== 'submitted' && rung !== 'receipted' && rung !== 'committed') {
    throw new TypeError('malformed sequencer receipt rung');
  }
  const txHash = decodeHex(value.txHash, HASH, 'txHash');
  if (expectedHash !== undefined && txHash.toLowerCase() !== decodeHex(expectedHash, HASH, 'expected hash').toLowerCase()) {
    throw new TypeError('sequencer receipt txHash differs from the asked hash');
  }
  const answer: SequencerReceiptAnswer = {
    rung,
    txHash,
    nonce: decodeHex(value.nonce, QUANTITY, 'nonce'),
  };
  if (rung !== 'submitted' || value.blockNumber !== undefined) {
    answer.blockNumber = decodeHex(value.blockNumber, QUANTITY, 'blockNumber');
  }
  if (rung !== 'submitted' || value.blockHash !== undefined) {
    answer.blockHash = decodeHex(value.blockHash, HASH, 'blockHash');
  }
  for (const field of [
    'seq', 'height', 'index', 'timestamp', 'status', 'gasUsed', 'cumulativeGasUsed',
    'effectiveGasPrice', 'feeAmount',
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
      if (typeof value[field] !== 'string') throw new TypeError(`malformed sequencer receipt ${field}`);
      answer[field] = value[field];
    }
  }
  if (value.logs !== undefined) {
    if (!Array.isArray(value.logs)) throw new TypeError('malformed sequencer receipt logs');
    answer.logs = value.logs.map((log: unknown) => {
      if (!isRecord(log) || !Array.isArray(log.topics)) throw new TypeError('malformed sequencer receipt log');
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

export async function readSequencerReceipt(rpc: TransferRpc, hash: string): Promise<SequencerReceiptAnswer | null> {
  decodeHex(hash, HASH, 'transaction hash');
  const value = await rpc('pax_sequencerReceipt', [hash]);
  return value === null ? null : decodeSequencerReceipt(value, hash);
}

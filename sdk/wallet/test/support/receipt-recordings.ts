import { readFileSync } from 'node:fs';
import type { Hex, ReceiptKeyPin, SequencerReceiptAnswer } from '../../src/types.js';

export function recording<T>(path: string): T {
  return JSON.parse(readFileSync(new URL(path, import.meta.url), 'utf8')) as T;
}

export interface ReceiptRecording {
  id: string;
  vector: string;
  rawTransaction: Hex;
  receiptDigest: Hex;
  receiptJson: SequencerReceiptAnswer;
  pinnedKey: {
    keyId: Hex;
    publicKey: Hex;
    firstHeight: string | number;
    lastHeight: string | number;
  };
}

export const nodeVectors = recording<{
  provenance: { generator: string };
  cases: ReceiptRecording[];
}>('../../../platform/conformance/fixtures/sequencer-receipt-v1.json');
export const committedVector = recording<{ positive: ReceiptRecording }>(
  '../../../platform/conformance/fixtures/vector-v4-committed.json'
).positive;
export const committedReceipt = recording<{
  settlement: { receipt: SequencerReceiptAnswer };
}>('../../../../daemon/committed-fixture/vectors/vector-v4-committed.json').settlement.receipt;

export function nodeVector(id: string): ReceiptRecording {
  const entry = nodeVectors.cases.find((entry) => entry.id === id);
  if (!entry) throw new Error(`missing node recording ${id}`);
  return entry;
}

export function receiptRecording(answer: Pick<SequencerReceiptAnswer, 'txHash'>): ReceiptRecording {
  const entry = [...nodeVectors.cases, committedVector].find(
    (entry) => entry.receiptJson.txHash === answer.txHash
  );
  if (!entry) throw new Error(`missing governed key recording ${answer.txHash}`);
  return entry;
}

export function authenticatedReceipt(answer: SequencerReceiptAnswer) {
  if (answer.rung === 'submitted') return answer;
  const entry = receiptRecording(answer);
  return {
    ...answer,
    authentication: {
      level: 'sequencer-signed' as const,
      keyId: entry.pinnedKey.keyId,
      signingKey: entry.pinnedKey.publicKey,
      height: BigInt(answer.height!),
      receiptDigest: entry.receiptDigest,
      rawTransaction: entry.rawTransaction,
    },
  };
}

export function receiptPins(answer: SequencerReceiptAnswer): readonly ReceiptKeyPin[] {
  const entry = receiptRecording(answer);
  return [
    {
      ...entry.pinnedKey,
      firstHeight: BigInt(entry.pinnedKey.firstHeight),
      lastHeight: BigInt(entry.pinnedKey.lastHeight),
    },
  ];
}

export function receiptWireKeys(answer: SequencerReceiptAnswer) {
  return {
    keys: receiptPins(answer).map((pin) => ({
      ...pin,
      firstHeight: pin.firstHeight.toString(),
      lastHeight: pin.lastHeight.toString(),
    })),
  };
}

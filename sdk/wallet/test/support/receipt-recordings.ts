import { readFileSync } from 'node:fs';
import type { Hex, ReceiptKeyPin, SequencerReceiptAnswer } from '../../src/types.js';

export function recording<T>(path: string): T {
  return JSON.parse(readFileSync(new URL(path, import.meta.url), 'utf8')) as T;
}

export interface ReceiptRecording {
  id: string;
  vector: string;
  rawTransaction: Hex;
  receiptJson: SequencerReceiptAnswer;
  pinnedKey: { keyId: Hex; publicKey: Hex; firstHeight: string; lastHeight: string };
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

export function receiptPins(answer: SequencerReceiptAnswer): readonly ReceiptKeyPin[] {
  const entry = [...nodeVectors.cases, committedVector].find(
    (entry) => entry.receiptJson.txHash === answer.txHash
  );
  if (!entry) throw new Error(`missing governed key recording ${answer.txHash}`);
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

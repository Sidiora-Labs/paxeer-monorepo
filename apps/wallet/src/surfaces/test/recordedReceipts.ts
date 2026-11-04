import { readFileSync } from 'node:fs';
import path from 'node:path';
import { webcrypto } from 'node:crypto';
import { ethers } from 'ethers';
import { parseTransaction, type Hex } from 'viem';
import type { SequencerReceiptAnswer } from '@paxeer/wallet';
import { SurfaceChain, type SentCall } from './chain';

Object.defineProperty(globalThis, 'crypto', { value: webcrypto, configurable: true });

interface RecordedReceipt {
  readonly id: string;
  readonly rawTransaction: Hex;
  readonly receiptJson: SequencerReceiptAnswer;
  readonly pinnedKey: { keyId: string; publicKey: string; firstHeight: number; lastHeight: number };
}

const corpus = JSON.parse(
  readFileSync(
    path.resolve(
      __dirname,
      '../../../../../sdk/platform/conformance/fixtures/kernel-receipt-v1.json'
    ),
    'utf8'
  )
) as { cases: RecordedReceipt[] };

export function recordedReceipt(id: string): RecordedReceipt {
  if (id === 'V1-pax-transfer') {
    const source = JSON.parse(
      readFileSync(
        path.resolve(
          __dirname,
          '../../../../../sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json'
        ),
        'utf8'
      )
    ) as { cases: RecordedReceipt[] };
    const entry = source.cases.find((candidate) => candidate.id === id);
    if (!entry) throw new Error(`Missing producer recording ${id}`);
    return entry;
  }
  if (id === 'committed-payExact') {
    const source = JSON.parse(
      readFileSync(
        path.resolve(
          __dirname,
          '../../../../../daemon/committed-fixture/vectors/vector-v4-committed.json'
        ),
        'utf8'
      )
    ) as {
      settlement: { raw_transaction: Hex; receipt: SequencerReceiptAnswer };
    };
    const conformance = JSON.parse(
      readFileSync(
        path.resolve(
          __dirname,
          '../../../../../sdk/platform/conformance/fixtures/vector-v4-committed.json'
        ),
        'utf8'
      )
    ) as { positive: RecordedReceipt };
    if (source.settlement.receipt.txHash !== conformance.positive.receiptJson.txHash)
      throw new Error('Committed recording does not match the conformance receipt');
    return {
      id,
      rawTransaction: source.settlement.raw_transaction,
      receiptJson: source.settlement.receipt,
      pinnedKey: conformance.positive.pinnedKey,
    };
  }
  const entry = corpus.cases.find((candidate) => candidate.id === id);
  if (!entry) throw new Error(`Missing producer recording ${id}`);
  return entry;
}

export class RecordedBridgeChain extends SurfaceChain {
  // Public conformance account used by rpc/sequencer_receipt_vectors_test.go.
  override readonly wallet = new ethers.Wallet(
    '0x8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63'
  );
  override readonly address = this.wallet.address.toLowerCase();
  readonly recording: RecordedReceipt;

  constructor(entryId = 'V1-layerxbridge-bridge-out') {
    super();
    this.recording = recordedReceipt(entryId);
  }

  override async request(args: {
    method: string;
    params?: readonly unknown[] | object;
  }): Promise<unknown> {
    const params = Array.isArray(args.params) ? args.params : [];
    if (args.method === 'px_getReceiptKeys') {
      this.methods.push(args.method);
      return { keys: [this.recording.pinnedKey] };
    }
    if (args.method === 'eth_getRawTransactionByHash') {
      this.methods.push(args.method);
      if (params[0] !== this.recording.receiptJson.txHash)
        throw new Error('Unrecorded transaction');
      return this.recording.rawTransaction;
    }
    if (args.method === 'pax_sequencerReceipt') {
      this.methods.push(args.method);
      if (params[0] !== this.recording.receiptJson.txHash)
        throw new Error('Unrecorded transaction');
      return structuredClone(this.recording.receiptJson);
    }
    if (args.method === 'paxeer_sendTransactionReceipted') {
      this.methods.push(args.method);
      const call = params[0] as SentCall;
      const raw = parseTransaction(this.recording.rawTransaction);
      if (
        call.from.toLowerCase() !== this.address ||
        call.to.toLowerCase() !== raw.to?.toLowerCase() ||
        call.data.toLowerCase() !== raw.data ||
        BigInt(call.value) !== (raw.value ?? 0n)
      )
        throw new Error('Call differs from the recorded producer transaction');
      this.sent.push({
        from: call.from.toLowerCase(),
        to: call.to.toLowerCase(),
        data: call.data.toLowerCase(),
        value: call.value,
      });
      return structuredClone(this.recording.receiptJson);
    }
    return super.request(args);
  }
}

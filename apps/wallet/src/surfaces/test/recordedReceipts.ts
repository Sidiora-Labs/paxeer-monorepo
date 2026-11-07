import { readFileSync } from 'node:fs';
import path from 'node:path';
import { webcrypto } from 'node:crypto';
import { ethers } from 'ethers';
import { keccak256, parseTransaction, type Hex } from 'viem';
import { FEE_TOKEN_PRECOMPILE, decodeAbiString, type SequencerReceiptAnswer } from '@paxeer/wallet';
import { abiSelector } from '@sidiora/layerx-sdk/browser';
import {
  CHAIN_KEY,
  SurfaceChain,
  nodeReceiptCall,
  type NodeReceiptVector,
  type SentCall,
} from './chain';
import ledgerProof from './fixtures/ledger-proof-v1.json';

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
    if (
      source.settlement.receipt.txHash !== conformance.positive.receiptJson.txHash ||
      source.settlement.raw_transaction !== conformance.positive.rawTransaction ||
      keccak256(source.settlement.raw_transaction) !== source.settlement.receipt.txHash ||
      parseTransaction(source.settlement.raw_transaction).chainId !== 125 ||
      parseTransaction(source.settlement.raw_transaction).type !== 'eip1559'
    )
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

const surfaceCorpus: {
  provenance: { kind: string; receiptModel: string; ledgerModel: string };
  cases: NodeReceiptVector[];
} = JSON.parse(readFileSync(path.resolve(__dirname, 'fixtures/surface-receipts-v1.json'), 'utf8'));

export function surfaceReceiptVectors(): NodeReceiptVector[] {
  return structuredClone(surfaceCorpus.cases);
}

export function surfaceReceiptProvenance() {
  return structuredClone(surfaceCorpus.provenance);
}

export function recordedLedgerProof() {
  return structuredClone(ledgerProof);
}

export const EXCHANGE_RECORDING_KEY = ethers.id('paxeer-wallet-sdk-fixture');

/** Replays only complete, signed call recordings; never manufactures a send answer. */
export class RecordedSurfaceChain extends SurfaceChain {
  override readonly wallet: ethers.Wallet;
  override readonly address: string;
  private readonly recordings: NodeReceiptVector[];
  private readonly replayed = new Map<string, NodeReceiptVector>();

  constructor(options: { recordingIds?: readonly string[]; privateKey?: string } = {}) {
    super();
    this.wallet = new ethers.Wallet(options.privateKey ?? CHAIN_KEY);
    this.address = this.wallet.address.toLowerCase();
    this.recordings = surfaceReceiptVectors().filter((entry) =>
      options.recordingIds
        ? options.recordingIds.includes(entry.id)
        : entry.receiptJson.status === '0x1'
    );
    if (options.recordingIds && this.recordings.length !== options.recordingIds.length)
      throw new Error('Missing surface recording');
  }

  override async request(args: Parameters<SurfaceChain['request']>[0]): Promise<unknown> {
    const params = Array.isArray(args.params) ? args.params : [];
    const hash = String(params[0]);
    if (args.method === 'px_getReceiptKeys') {
      this.methods.push(args.method);
      return {
        keys: structuredClone([
          ...new Map(
            this.recordings.map((entry) => [entry.pinnedKey.keyId, entry.pinnedKey])
          ).values(),
        ]),
      };
    }
    if (
      args.method === 'eth_getRawTransactionByHash' ||
      args.method === 'pax_sequencerReceipt' ||
      args.method === 'eth_getTransactionReceipt'
    ) {
      this.methods.push(args.method);
      const entry = this.replayed.get(hash);
      if (!entry) return null;
      if (args.method === 'eth_getRawTransactionByHash') return entry.rawTransaction;
      if (args.method === 'eth_getTransactionReceipt')
        return { transactionHash: hash, ...structuredClone(entry.receiptJson) };
      return structuredClone(entry.receiptJson);
    }
    if (args.method !== 'eth_sendTransaction' && args.method !== 'paxeer_sendTransactionReceipted')
      return super.request(args);

    this.methods.push(args.method);
    const requested = params[0] as SentCall;
    if (requested.from.toLowerCase() !== this.address)
      throw Object.assign(new Error('unknown account'), { code: 4100 });
    const call: SentCall = {
      from: requested.from.toLowerCase(),
      to: requested.to.toLowerCase(),
      data: requested.data.toLowerCase(),
      value: requested.value,
    };
    const entry = this.recordings.find((candidate) => {
      const recorded = nodeReceiptCall(candidate);
      return (
        !this.replayed.has(candidate.receiptJson.txHash) &&
        recorded.from === call.from &&
        recorded.to === call.to &&
        recorded.data === call.data &&
        BigInt(recorded.value) === BigInt(call.value)
      );
    });
    if (!entry)
      throw Object.assign(new Error('no node receipt vector for this call'), { code: -32000 });
    const requestedLogs = this.logsFor(call);
    if (
      requestedLogs.length &&
      JSON.stringify(requestedLogs) !==
        JSON.stringify(
          entry.receiptJson.logs.map(({ address, topics, data }) => ({ address, topics, data }))
        )
    )
      throw new Error('surface logs differ from the signed node receipt');

    this.sent.push(call);
    this.replayed.set(entry.receiptJson.txHash, entry);
    if (entry.receiptJson.status === '0x1') {
      if (call.data.startsWith(abiSelector('approve(address,uint256)')))
        this.allowances.set(
          `${call.to}:${this.address.slice(2).padStart(64, '0')}${call.data.slice(10, 74)}`,
          BigInt(`0x${call.data.slice(74)}`)
        );
      if (call.to === FEE_TOKEN_PRECOMPILE) {
        if (call.data.startsWith(abiSelector('setFeeDenom(string)')))
          this.feeDenom = decodeAbiString(`0x${call.data.slice(10)}`);
        if (call.data === abiSelector('clearFeeDenom()')) this.feeDenom = '';
      }
    }
    return args.method === 'eth_sendTransaction'
      ? entry.receiptJson.txHash
      : structuredClone(entry.receiptJson);
  }
}

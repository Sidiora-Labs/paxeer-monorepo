// @vitest-environment jsdom

import { readFileSync } from 'node:fs';
import { EventEmitter } from 'node:events';
import { webcrypto } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { resolve } from 'node:path';
import { createElement } from 'react';
import { JsonRpcProvider, Transaction, Wallet, toQuantity } from 'ethers';
import { expect, test } from 'vitest';
import {
  JsonRpcClient,
  WalletInterface,
  type Eip1193Provider,
  type RequestArguments,
  type SequencerReceiptAnswer,
  type Hex,
  decodeSequencerReceipt,
  recoverTransferObservation,
  submittedTransfer,
  type TransferObservation,
  type ReceiptKeyPin,
} from '@paxeer/wallet';
import { PAXEER_CONFIG } from '../lib/constants';
import { WalletSigner } from '../wallet/signer';
import { TransferSuccessResult } from '../components/TransferSuccess';
import { mount } from '../account/test/render';
import {
  putTransferRecord,
  transferObservationsRepository,
  type JsonValue,
} from '../platform/storage/repositories';
import { observeConfiguredTransfer, waitForIndexedCommit } from '../../e2e/explorer-proof';

Object.defineProperty(globalThis, 'crypto', { value: webcrypto, configurable: true });

const fixtureRoot = resolve(__dirname, '../../../../sdk/platform/conformance/fixtures');
interface ProducerVector {
  id: string;
  rawTransaction: string;
  signedReceipt: string;
  receiptJson: Exclude<SequencerReceiptAnswer, { rung: 'submitted' }>;
  pinnedKey: { keyId: Hex; publicKey: Hex; firstHeight: number; lastHeight: number };
}

const producer: { cases: ProducerVector[] } = JSON.parse(
  readFileSync(resolve(fixtureRoot, 'sequencer-receipt-v1.json'), 'utf8')
);
function producerCase(id: string) {
  const entry = producer.cases.find((entry) => entry.id === id);
  if (!entry) throw new Error(`The production RPC vector ${id} is required`);
  return entry;
}
const vector = producerCase('V1-layerxledger-pay-exact');
// Keep the producer's signed receipt unchanged. Only unsigned block/indexer
// metadata belongs to the in-memory chain peer, as in the interop transport fixtures.
const blockNumber = vector.receiptJson.height!;
const blockHash = JSON.parse(
  readFileSync(
    resolve(
      __dirname,
      '../../../../services/interop/crates/layerx-gas-station/tests/fixtures/sequencer_receipts.json'
    ),
    'utf8'
  )
).committed.blockHash as string;
const sequencerAnswer = {
  ...vector.receiptJson,
  rung: 'committed',
  blockNumber,
  blockHash,
};
const explorerAnswers = JSON.parse(
  readFileSync(
    resolve(__dirname, '../../../../sdk/wallet/test/fixtures/explorer-status.json'),
    'utf8'
  )
);
explorerAnswers.committed = {
  ...explorerAnswers.committed,
  block_number: Number(BigInt(blockNumber)),
  committed_height: Number(BigInt(blockNumber)),
};
const receiptKeyRecords = [vector.pinnedKey];
const receiptKeys: readonly ReceiptKeyPin[] = receiptKeyRecords.map((key) => ({
  ...key,
  firstHeight: BigInt(key.firstHeight),
  lastHeight: BigInt(key.lastHeight),
}));
// Public transaction key from rpc/sequencer_receipt_vectors_test.go.
const signingWallet = new Wallet(
  '0x8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63'
);

class RecordedSigningPeer extends EventEmitter implements Eip1193Provider {
  constructor(readonly rpc: JsonRpcClient) {
    super();
  }

  async request({ method, params }: RequestArguments): Promise<unknown> {
    const positional = params ?? [];
    if (!Array.isArray(positional)) throw new Error('Positional wallet parameters are required');
    if (method !== 'eth_sendTransaction') return this.rpc.call(method, positional);
    const [tx] = positional as Record<string, string>[];
    expect(Number(BigInt(tx.chainId))).toBe(PAXEER_CONFIG.chainId);
    expect(tx.from.toLowerCase()).toBe(signingWallet.address.toLowerCase());
    const raw = await signingWallet.signTransaction({
      type: 2,
      to: tx.to,
      value: BigInt(tx.value),
      data: tx.data,
      gasLimit: BigInt(tx.gas),
      maxFeePerGas: BigInt(tx.maxFeePerGas),
      maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas),
      nonce: Number(BigInt(tx.nonce)),
      chainId: PAXEER_CONFIG.chainId,
    });
    expect(raw).toBe(vector.rawTransaction);
    return this.rpc.call('eth_sendRawTransaction', [raw]);
  }
}

async function listen(server: Server): Promise<string> {
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}

async function close(server: Server): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
}

function cacheableObservation(observation: TransferObservation): JsonValue {
  // Authentication is an in-memory capability. Persist only the canonical wire
  // receipt; recovery must decode the saved evidence and authenticate it afresh.
  const wire = { ...observation };
  if (wire.sequencer) {
    wire.sequencer = decodeSequencerReceipt(wire.sequencer, wire.identity.hash, receiptKeys);
  }
  if (wire.lastVerified?.sequencer) {
    wire.lastVerified = {
      ...wire.lastVerified,
      sequencer: decodeSequencerReceipt(
        wire.lastVerified.sequencer,
        wire.identity.hash,
        receiptKeys
      ),
    };
  }
  return JSON.parse(JSON.stringify(wire)) as JsonValue;
}

async function assertWallet(observation: TransferObservation, completed: boolean) {
  const dismissed: boolean[] = [];
  const view = await mount(
    createElement(TransferSuccessResult, {
      fromLabel: 'Sender',
      fromAmount: '1',
      fromSymbol: 'PAX',
      toLabel: 'Recipient',
      transfer: observation.identity,
      observation,
      error: observation.warning ?? '',
      onDone: (terminal: boolean) => dismissed.push(terminal),
    })
  );
  try {
    expect(view.text('h2')).toBe(completed ? 'Transfer Completed' : 'Receipted');
    const rung = view.query('[data-rung="committed"]');
    expect(rung?.getAttribute('data-reached')).toBe(String(completed));
    expect(rung?.getAttribute('data-source')).toBe(completed ? 'explorer' : null);
    expect(rung?.getAttribute('data-evidence')).toBe(
      completed ? `committed:${sequencerAnswer.blockHash}` : null
    );
    expect(view.query('[role="alert"]')?.textContent ?? '').toBe(observation.warning ?? '');
    await view.click('button');
    expect(dismissed).toEqual([completed]);
  } finally {
    await view.unmount();
  }
}

async function send_explorer_proof_native_transfer_fixture_identity() {
  const native = producerCase('V1-pax-transfer');
  const signed = Transaction.from(native.rawTransaction);
  const receipt = native.receiptJson;
  const reproduced = await signingWallet.signTransaction({
    type: 2,
    to: signed.to,
    value: signed.value,
    data: signed.data,
    gasLimit: signed.gasLimit,
    maxFeePerGas: signed.maxFeePerGas,
    maxPriorityFeePerGas: signed.maxPriorityFeePerGas,
    nonce: signed.nonce,
    chainId: PAXEER_CONFIG.chainId,
  });
  expect(reproduced).toBe(native.rawTransaction);
  expect(signed.hash).toBe(receipt.txHash);
  expect(signed.from?.toLowerCase()).toBe(receipt.from);
  expect(signed.to?.toLowerCase()).toBe(receipt.to);
  expect(signed.chainId).toBe(125n);
  expect(signed.value).toBe(1_000_000_000_000_000_000n);
  expect(decodeSequencerReceipt(receipt, signed.hash!, receiptKeys)).toMatchObject({
    rung: 'receipted',
    txHash: signed.hash,
    height: '0x3e8',
  });
}

test(
  send_explorer_proof_native_transfer_fixture_identity.name,
  send_explorer_proof_native_transfer_fixture_identity
);

async function send_explorer_proof_transport_commit_reload_and_unavailable() {
  localStorage.clear();
  expect(PAXEER_CONFIG.chainId).toBe(125);
  const signed = Transaction.from(vector.rawTransaction);
  expect(signed.hash).toBe(sequencerAnswer.txHash);
  expect(signed.from?.toLowerCase()).toBe(sequencerAnswer.from);
  expect(signed.to?.toLowerCase()).toBe(sequencerAnswer.to);
  expect(signed.chainId).toBe(125n);
  expect(signed.value).toBe(0n);
  expect(signed.type).toBe(2);
  expect(signed.data).toMatch(/^0x78ea11db/);
  expect(sequencerAnswer.receipt).toBe(vector.signedReceipt);
  expect(sequencerAnswer).toMatchObject({ ...vector.receiptJson, rung: 'committed' });
  const identity = {
    hash: sequencerAnswer.txHash as string,
    chainId: PAXEER_CONFIG.chainId,
  };
  let available = false;
  let submitted = false;
  const indexedRungs = ['submitted', 'receipted', 'committed'];
  const requests: { method: string; params: unknown[] }[] = [];
  const explorerRequests: { path: string; status: number; rung?: string }[] = [];
  const failures: unknown[] = [];
  const node = createServer((req, res) => {
    void (async () => {
      expect(req.method).toBe('POST');
      expect(req.url).toBe('/');
      let text = '';
      for await (const chunk of req) text += chunk;
      const request = JSON.parse(text);
      requests.push({ method: request.method, params: request.params });
      let result: unknown;
      switch (request.method) {
        case 'eth_requestAccounts':
        case 'eth_accounts':
          expect(request.params).toEqual([]);
          result = [signingWallet.address];
          break;
        case 'eth_sendRawTransaction':
          expect(request.params).toEqual([vector.rawTransaction]);
          submitted = true;
          result = signed.hash;
          break;
        case 'eth_chainId':
          expect(request.params).toEqual([]);
          result = toQuantity(PAXEER_CONFIG.chainId);
          break;
        case 'eth_getTransactionReceipt':
          expect(submitted).toBe(true);
          expect(request.params).toEqual([identity.hash]);
          result = {
            transactionHash: sequencerAnswer.txHash,
            blockNumber: sequencerAnswer.blockNumber,
            blockHash: sequencerAnswer.blockHash,
            status: sequencerAnswer.status,
            gasUsed: sequencerAnswer.gasUsed,
            logs: sequencerAnswer.logs,
          };
          break;
        case 'eth_getBlockByNumber':
          expect(request.params).toEqual([sequencerAnswer.blockNumber, false]);
          result = {
            number: sequencerAnswer.blockNumber,
            hash: sequencerAnswer.blockHash,
            transactions: [identity.hash],
          };
          break;
        case 'eth_getTransactionByHash':
          expect(request.params).toEqual([identity.hash]);
          result = {
            hash: signed.hash,
            from: signed.from,
            to: signed.to,
            value: `0x${signed.value.toString(16)}`,
            nonce: `0x${signed.nonce.toString(16)}`,
            input: signed.data,
            type: toQuantity(signed.type!),
            chainId: toQuantity(signed.chainId),
            gas: toQuantity(signed.gasLimit),
            gasPrice: sequencerAnswer.effectiveGasPrice,
            maxFeePerGas: toQuantity(signed.maxFeePerGas!),
            maxPriorityFeePerGas: toQuantity(signed.maxPriorityFeePerGas!),
            accessList: signed.accessList,
            r: signed.signature!.r,
            s: signed.signature!.s,
            v: toQuantity(signed.signature!.yParity),
            blockNumber,
            blockHash,
            transactionIndex: sequencerAnswer.index,
          };
          break;
        case 'px_getReceiptKeys':
          expect(request.params).toEqual([]);
          result = { keys: receiptKeyRecords };
          break;
        case 'eth_getRawTransactionByHash':
          expect(request.params).toEqual([identity.hash]);
          result = vector.rawTransaction;
          break;
        case 'pax_sequencerReceipt':
          expect(request.params).toEqual([identity.hash]);
          result = sequencerAnswer;
          break;
        default:
          throw new Error(`Unexpected RPC method ${request.method}`);
      }
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, result }));
    })().catch((error) => {
      failures.push(error);
      res.writeHead(500).end();
    });
  });
  const explorer = createServer((req, res) => {
    try {
      expect(submitted).toBe(true);
      expect(req.method).toBe('GET');
      expect(req.url).toBe(`/api/v2/transactions/${identity.hash}/status`);
      expect(req.headers.accept).toBe('application/json');
      res.setHeader('content-type', 'application/json');
      res.setHeader('cache-control', 'no-store');
      if (!available) {
        explorerRequests.push({ path: req.url!, status: 503 });
        res.writeHead(503).end(JSON.stringify({ message: 'Indexer unavailable' }));
        return;
      }
      const rung = indexedRungs.shift() ?? 'committed';
      explorerRequests.push({ path: req.url!, status: 200, rung });
      res.end(JSON.stringify(explorerAnswers[rung as keyof typeof explorerAnswers]));
    } catch (error) {
      failures.push(error);
      res.writeHead(500).end();
    }
  });
  const target = { rpcUrl: await listen(node), explorerUrl: await listen(explorer) };
  const client = new JsonRpcClient({ url: target.rpcUrl });
  const wallet = new WalletInterface(new RecordedSigningPeer(client));
  const readProvider = new JsonRpcProvider(target.rpcUrl, PAXEER_CONFIG.chainId, {
    staticNetwork: true,
    batchMaxCount: 1,
  });
  try {
    const signer = new WalletSigner(wallet, readProvider);
    const sent = await signer.sendTransaction({
      from: signingWallet.address,
      to: signed.to,
      value: signed.value,
      data: signed.data,
      gasLimit: signed.gasLimit,
      maxFeePerGas: signed.maxFeePerGas,
      maxPriorityFeePerGas: signed.maxPriorityFeePerGas,
      nonce: signed.nonce,
      chainId: PAXEER_CONFIG.chainId,
    });
    expect(sent.hash).toBe(identity.hash);
    expect(sent.chainId).toBe(125n);
    const unavailable = await observeConfiguredTransfer(target, submittedTransfer(identity));
    expect(unavailable.warning).toBe('explorer status answered HTTP 503');
    expect(unavailable.steps).toEqual([
      { source: 'receipt', rung: 'receipted', state: `receipted:${sequencerAnswer.blockHash}` },
    ]);
    await assertWallet(unavailable, false);

    available = true;
    const committed = await waitForIndexedCommit(target, identity);
    expect(committed.blockNumber).toBe(1001);
    expect(committed.blockHash).toBe(sequencerAnswer.blockHash);
    expect(committed.warning).toBeUndefined();
    expect(committed.steps).toEqual([
      { source: 'receipt', rung: 'committed', state: `committed:${sequencerAnswer.blockHash}` },
      { source: 'explorer', rung: 'committed', state: `committed:${sequencerAnswer.blockHash}` },
    ]);
    await assertWallet(committed, true);

    const key = `${PAXEER_CONFIG.chainId}:${identity.hash}`;
    transferObservationsRepository.update((records) =>
      putTransferRecord(records, key, cacheableObservation(committed))
    );
    const recovered = recoverTransferObservation(
      identity,
      transferObservationsRepository.read()[key],
      receiptKeys
    );
    expect(recovered.truth).toBe('submitted');
    expect(recovered.steps).toEqual([]);
    const savedEvidence = {
      ...committed.lastVerified,
      sequencer: decodeSequencerReceipt(committed.sequencer, identity.hash, receiptKeys),
    };
    expect(recovered.lastVerified).toEqual(savedEvidence);
    const reloaded = await observeConfiguredTransfer(target, recovered);
    await assertWallet(reloaded, true);

    available = false;
    const refused = await observeConfiguredTransfer(target, reloaded);
    expect(refused.warning).toBe('explorer status answered HTTP 503');
    expect(refused.lastVerified).toEqual(committed.lastVerified);
    await assertWallet(refused, false);
    const reopened = recoverTransferObservation(
      identity,
      cacheableObservation(refused),
      receiptKeys
    );
    expect(reopened.steps).toEqual([]);
    expect(reopened.lastVerified).toEqual(savedEvidence);
    await assertWallet(await observeConfiguredTransfer(target, reopened), false);
    expect(explorerRequests.map(({ status, rung }) => [status, rung])).toEqual([
      [503, undefined],
      [200, 'submitted'],
      [200, 'receipted'],
      [200, 'committed'],
      [200, 'committed'],
      [503, undefined],
      [503, undefined],
    ]);
    expect(requests.filter(({ method }) => method === 'eth_sendRawTransaction')).toHaveLength(1);
    expect(requests.filter(({ method }) => method === 'eth_getRawTransactionByHash')).toHaveLength(
      7
    );
    expect(requests.filter(({ method }) => method === 'px_getReceiptKeys')).toHaveLength(7);
    expect(failures).toEqual([]);
  } finally {
    wallet.dispose();
    readProvider.destroy();
    await close(explorer);
    await close(node);
    localStorage.clear();
  }
}

test(
  send_explorer_proof_transport_commit_reload_and_unavailable.name,
  send_explorer_proof_transport_commit_reload_and_unavailable
);

function send_explorer_proof_preexecution_refusal_fixture_identity() {
  const native = producerCase('V2-pax-transfer-insufficient-funds');
  const signed = Transaction.from(native.rawTransaction);
  const receipt = native.receiptJson;
  expect(signed.hash).toBe(receipt.txHash);
  expect(signed.from?.toLowerCase()).toBe(receipt.from);
  expect(signed.to?.toLowerCase()).toBe(receipt.to);
  expect(signed.value).toBe(101_000_000_000_000_000_000n);
  expect(signed.data).toBe('0x');
  expect(decodeSequencerReceipt(receipt, signed.hash!, receiptKeys)).toMatchObject({
    rung: 'receipted',
    status: '0x0',
    returnData: '0x',
    logs: [],
    vmError: 'insufficient balance for transfer',
  });
  expect(receipt).not.toHaveProperty('ledgerResultCode');
  expect(native).not.toHaveProperty('lxpCode');
}

test(
  send_explorer_proof_preexecution_refusal_fixture_identity.name,
  send_explorer_proof_preexecution_refusal_fixture_identity
);

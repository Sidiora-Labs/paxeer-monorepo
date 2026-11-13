// @vitest-environment jsdom
import { createServer } from 'node:http';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { webcrypto } from 'node:crypto';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { ed25519 } from '@noble/curves/ed25519.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, concatBytes, hexToBytes } from 'viem';
import { verifySequencerReceipt } from '@sidiora/layerx-sdk/browser';
import {
  encodeKernelReceipt,
  readReceiptKeyPins,
  readSequencerReceipt,
  recoverTransferObservation,
  requireAuthenticatedReceipt,
  statusLadder,
  submittedTransfer,
  type TransferObservation,
} from '@paxeer/wallet';
import { mount } from '@/account/test/render';
import { transferObservationsRepository } from '@/platform/storage/repositories';
import type { NodeReceiptVector } from '@/surfaces/test/chain';
import { createTransferObservationClient, TransferSuccessResult } from './TransferSuccess';
import { saveTransferObservation, transferStorageKey } from './transferOrigin';

async function transfer_observation_with_an_authenticated_receipt_is_saved() {
  const vectors = JSON.parse(
    readFileSync(
      resolve(__dirname, '../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json'),
      'utf8'
    )
  ) as { cases: NodeReceiptVector[] };
  const entry = vectors.cases.find(({ id }) => id === 'V1-pax-transfer');
  if (!entry) throw new Error('The node-produced PAX transfer receipt is missing');
  const cryptoDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'crypto');
  const previousRecords = transferObservationsRepository.read();
  Object.defineProperty(globalThis, 'crypto', { value: webcrypto, configurable: true });
  const server = createServer();
  try {
    const original = await verifySequencerReceipt(
      hexToBytes(entry.signedReceipt),
      hexToBytes(entry.rawTransaction),
      [
        {
          keyId: hexToBytes(entry.pinnedKey.keyId),
          publicKey: hexToBytes(entry.pinnedKey.publicKey),
          first_height: BigInt(entry.pinnedKey.firstHeight),
          last_height: BigInt(entry.pinnedKey.lastHeight),
        },
      ]
    );
    const height = 9223372036854775807n;
    const batchId = new Uint8Array(32);
    new DataView(batchId.buffer).setBigUint64(24, height, false);
    const receipt = { ...original.receipt, batchId };
    const domain = new TextEncoder().encode('LXP/v1/receipt\0');
    const encodedReceipt = encodeKernelReceipt(receipt);
    const signedEnd = encodedReceipt.length - (1 + 4 + 64);
    const signingBytes = concatBytes([domain, encodeKernelReceipt(receipt, false)]);
    const verificationBytes = concatBytes([
      domain,
      encodedReceipt.subarray(0, signedEnd),
      new Uint8Array([0]),
    ]);
    expect(signingBytes).toEqual(verificationBytes);
    const signingKey = new Uint8Array(32).fill(0x5e);
    expect(bytesToHex(ed25519.getPublicKey(signingKey))).toBe(entry.pinnedKey.publicKey);
    const digest = sha256(signingBytes);
    const signedReceipt = encodeKernelReceipt({
      ...receipt,
      sequencerSignature: ed25519.sign(digest, signingKey),
    });
    const answer = {
      ...entry.receiptJson,
      height: `0x${height.toString(16)}`,
      receipt: bytesToHex(signedReceipt),
    };
    const keys = [{ ...entry.pinnedKey, lastHeight: height.toString(10) }];
    server.on('request', async (request, response) => {
      let body = '';
      for await (const chunk of request) body += chunk;
      const payload = JSON.parse(body) as { id: number; method: string };
      const results: Record<string, unknown> = {
        pax_sequencerReceipt: answer,
        px_getReceiptKeys: { keys },
        eth_getRawTransactionByHash: entry.rawTransaction,
      };
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(
        JSON.stringify({ jsonrpc: '2.0', id: payload.id, result: results[payload.method] })
      );
    });
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    const { port } = server.address() as AddressInfo;
    const rpc = createTransferObservationClient(`http://127.0.0.1:${port}`);
    const sequencer = await readSequencerReceipt(
      (method, params) => rpc.call(method, params),
      entry.receiptJson.txHash
    );
    requireAuthenticatedReceipt(sequencer);
    expect(sequencer.authentication.height).toBe(height);
    const identity = { hash: sequencer.txHash, chainId: 125 };
    const blockHash = `0x${'ab'.repeat(32)}`;
    const steps = [
      { rung: 'receipted', source: 'receipt', state: `receipted:${blockHash}` },
    ] as const;
    const evidence = { blockNumber: 42, blockHash, steps, sequencer };
    const observation: TransferObservation = {
      ...submittedTransfer(identity),
      ...evidence,
      truth: 'included',
      lastVerified: evidence,
    };
    const origin = {
      chainId: 125,
      rpcUrl: `http://127.0.0.1:${port}`,
      explorerUrl: 'https://explorer.example',
      statusUrl: 'https://explorer.example/api/status',
    };
    saveTransferObservation(observation, origin);
    const stored = transferObservationsRepository.read()[transferStorageKey(identity)];
    const storedSequencer = {
      ...sequencer,
      authentication: { ...sequencer.authentication, height: '9223372036854775807' },
    };
    expect(stored).toEqual({
      ...observation,
      sequencer: storedSequencer,
      lastVerified: { ...evidence, sequencer: storedSequencer },
      origin,
    });
    expect(sequencer.authentication.height).toBe(height);
    expect(recoverTransferObservation(identity, stored)).toEqual(submittedTransfer(identity));
  } finally {
    await new Promise<void>((resolve, reject) => {
      if (!server.listening) return resolve();
      server.close((error) => (error ? reject(error) : resolve()));
    });
    transferObservationsRepository.write(previousRecords);
    if (cryptoDescriptor) Object.defineProperty(globalThis, 'crypto', cryptoDescriptor);
  }
}

test(
  transfer_observation_with_an_authenticated_receipt_is_saved.name,
  transfer_observation_with_an_authenticated_receipt_is_saved
);

async function transfer_observation_keeps_the_full_range_heights_of_the_receipt_keys() {
  const keyId = `0x${'ab'.repeat(32)}`;
  const publicKey = `0x${'cd'.repeat(32)}`;
  const requests: unknown[] = [];
  const server = createServer(async (request, response) => {
    let body = '';
    for await (const chunk of request) body += chunk;
    const payload = JSON.parse(body) as { id: number };
    requests.push(JSON.parse(body));
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(
      `{"jsonrpc":"2.0","id":${payload.id},"result":{"keys":[{"keyId":"${keyId}","publicKey":"${publicKey}","firstHeight":7,"lastHeight":9223372036854775807}]}}`
    );
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const { port } = server.address() as AddressInfo;
    const rpc = createTransferObservationClient(`http://127.0.0.1:${port}`);
    expect(rpc.timeoutMs).toBe(10_000);
    const answer = await rpc.call('px_getReceiptKeys', []);
    expect(answer).toEqual({
      keys: [{ keyId, publicKey, firstHeight: 7, lastHeight: '9223372036854775807' }],
    });
    const pins = await readReceiptKeyPins((method, params) => rpc.call(method, params));
    expect(pins).toEqual([{ keyId, publicKey, firstHeight: 7n, lastHeight: 9223372036854775807n }]);
    expect(requests).toEqual([
      { jsonrpc: '2.0', id: 1, method: 'px_getReceiptKeys', params: [] },
      { jsonrpc: '2.0', id: 2, method: 'px_getReceiptKeys', params: [] },
    ]);
  } finally {
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
  }
}

test(
  transfer_observation_keeps_the_full_range_heights_of_the_receipt_keys.name,
  transfer_observation_keeps_the_full_range_heights_of_the_receipt_keys
);

async function transfer_success_completes_only_at_the_committed_rung() {
  const identity = { hash: `0x${'ab'.repeat(32)}`, chainId: 125 };
  const submitted = submittedTransfer(identity);
  const receipted = statusLadder.fromExplorer('receipted');
  const committed = statusLadder.fromExplorer('committed');
  const included: TransferObservation = {
    ...submitted,
    truth: 'included',
    blockNumber: 42,
    blockHash: `0x${'cd'.repeat(32)}`,
  };
  const cases: ReadonlyArray<{
    observation: TransferObservation;
    title: string;
    completed: boolean;
    terminal: boolean;
  }> = [
    { observation: submitted, title: 'Transfer Submitted', completed: false, terminal: false },
    {
      observation: { ...submitted, truth: 'pending' },
      title: 'Transfer Pending',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...submitted, truth: 'pending', steps: [receipted] },
      title: 'Transfer Pending',
      completed: false,
      terminal: false,
    },
    { observation: included, title: 'Transfer Included', completed: false, terminal: false },
    {
      observation: { ...included, steps: [statusLadder.fromExplorer('submitted')] },
      title: 'Transfer Included',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...included, steps: [receipted] },
      title: 'Receipted',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...included, steps: [receipted, statusLadder.fromExplorer('submitted')] },
      title: 'Receipted',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...included, steps: [committed] },
      title: 'Transfer Completed',
      completed: true,
      terminal: true,
    },
    {
      observation: { ...included, steps: [committed, receipted] },
      title: 'Transfer Completed',
      completed: true,
      terminal: true,
    },
    {
      observation: { ...included, truth: 'unknown', steps: [committed] },
      title: 'Transfer Status Unknown',
      completed: false,
      terminal: false,
    },
    {
      observation: {
        ...submitted,
        truth: 'unknown',
        lastVerified: { blockNumber: 42, blockHash: `0x${'cd'.repeat(32)}`, steps: [committed] },
      },
      title: 'Transfer Status Unknown',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...included, truth: 'reverted', steps: [] },
      title: 'Transfer Reverted',
      completed: false,
      terminal: true,
    },
    {
      observation: { ...included, truth: 'replaced', steps: [] },
      title: 'Transfer Replaced',
      completed: false,
      terminal: true,
    },
  ];

  for (const { observation, title, completed, terminal } of cases) {
    const dismissed: boolean[] = [];
    const view = await mount(
      <TransferSuccessResult
        fromLabel="Sender"
        fromAmount="1"
        fromSymbol="PAX"
        toLabel="Recipient"
        transfer={identity}
        observation={observation}
        error=""
        onDone={(value) => dismissed.push(value)}
      />
    );
    try {
      expect(view.text('h2')).toBe(title);
      expect(view.text('button')).toBe(completed ? 'Done' : 'Close');
      expect(view.all('svg title').some((element) => element.textContent === 'Success')).toBe(
        completed
      );
      await view.click('button');
      expect(dismissed).toEqual([terminal]);
    } finally {
      await view.unmount();
    }
  }
}

test(
  transfer_success_completes_only_at_the_committed_rung.name,
  transfer_success_completes_only_at_the_committed_rung
);

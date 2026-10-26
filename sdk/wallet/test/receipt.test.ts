import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, expect, test } from 'vitest';
import {
  decodeSequencerReceipt,
  JsonRpcClient,
  JsonRpcError,
  readSequencerReceipt,
  type TransferRpc,
} from '../src/index.js';

const recorded = JSON.parse(
  readFileSync(new URL('./fixtures/sequencer-receipt.json', import.meta.url), 'utf8'),
) as Record<'submitted' | 'receipted' | 'committed', Record<string, unknown>>;
const transactionHash = String(recorded.submitted.txHash);
const foreignHash = `0x${'ff'.repeat(32)}`;
const answers: Record<string, unknown> = {
  submitted: recorded.submitted,
  receipted: {
    ...recorded.receipted,
    blockNumber: recorded.committed.blockNumber,
    blockHash: recorded.committed.blockHash,
  },
  committed: recorded.committed,
  missing: null,
  foreign: { ...recorded.submitted, txHash: foreignHash },
  unknown: { ...recorded.submitted, rung: 'final' },
};
const requests: { method: string; params: unknown[] }[] = [];
let base: string;
const server = createServer(async (req, res) => {
  let body = '';
  for await (const chunk of req) body += chunk;
  const request = JSON.parse(body);
  requests.push({ method: request.method, params: request.params });
  const name = req.url?.slice(1) ?? '';
  res.writeHead(200, { 'content-type': 'application/json' });
  if (request.method !== 'pax_sequencerReceipt' || request.params?.[0] !== transactionHash ||
      request.params.length !== 1 || !(name in answers)) {
    res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, error: { code: -32602, message: 'invalid receipt request' } }));
    return;
  }
  res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, result: answers[name] }));
});

beforeAll(async () => {
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise<void>((resolve, reject) => server.close((err) => err ? reject(err) : resolve()));
});

function receiptRpc(name: string): TransferRpc {
  const client = new JsonRpcClient({ url: `${base}/${name}` });
  return (method, params) => client.call(method, params);
}

async function read_sequencer_receipt_calls_pax_sequencer_receipt_and_decodes_each_rung() {
  const start = requests.length;
  for (const rung of ['submitted', 'receipted', 'committed'] as const) {
    const answer = await readSequencerReceipt(receiptRpc(rung), transactionHash);
    expect(answer).toEqual(answers[rung]);
    expect(answer?.rung).toBe(rung);
    expect(decodeSequencerReceipt(answers[rung])).toEqual(answer);
  }
  expect(await readSequencerReceipt(receiptRpc('missing'), transactionHash)).toBeNull();
  expect(requests.slice(start)).toEqual(Array.from({ length: 4 }, () => ({
    method: 'pax_sequencerReceipt', params: [transactionHash],
  })));
  const beforeInvalidHash = requests.length;
  await expect(readSequencerReceipt(receiptRpc('submitted'), 'invalid')).rejects.toThrow('transaction hash');
  expect(requests).toHaveLength(beforeInvalidHash);
  await expect(readSequencerReceipt(receiptRpc('unavailable'), transactionHash)).rejects.toBeInstanceOf(JsonRpcError);
  const largeNonce = { ...recorded.submitted, nonce: '0xffffffffffffffff' };
  expect(decodeSequencerReceipt(largeNonce).nonce).toBe(largeNonce.nonce);
}

async function decode_sequencer_receipt_refuses_an_unknown_rung_or_a_foreign_hash() {
  for (const rung of ['final', 'instant', '', null, 1, undefined]) {
    expect(() => decodeSequencerReceipt({ ...recorded.submitted, rung }, transactionHash)).toThrow('rung');
  }
  expect(() => decodeSequencerReceipt(answers.foreign, transactionHash)).toThrow('asked hash');
  await expect(readSequencerReceipt(receiptRpc('foreign'), transactionHash)).rejects.toThrow('asked hash');
  await expect(readSequencerReceipt(receiptRpc('unknown'), transactionHash)).rejects.toThrow('rung');
  for (const rung of ['receipted', 'committed'] as const) {
    for (const field of ['blockNumber', 'blockHash'] as const) {
      for (const invalid of [undefined, null, '', 1]) {
        expect(() => decodeSequencerReceipt({ ...recorded.committed, rung, [field]: invalid })).toThrow(field);
      }
    }
  }
  for (const invalid of [null, undefined, [], true, 'submitted']) {
    expect(() => decodeSequencerReceipt(invalid)).toThrow(TypeError);
  }
  for (const txHash of [undefined, null, '0x12', foreignHash.slice(2)]) {
    expect(() => decodeSequencerReceipt({ ...recorded.submitted, txHash })).toThrow('txHash');
  }
  for (const nonce of [undefined, null, 0, '0x00', '0x', '1']) {
    expect(() => decodeSequencerReceipt({ ...recorded.submitted, nonce })).toThrow('nonce');
  }
  expect(() => decodeSequencerReceipt(recorded.receipted)).toThrow('blockNumber');
  expect(() => decodeSequencerReceipt({ ...recorded.committed, logs: [null] })).toThrow('log');
  expect(() => decodeSequencerReceipt({ ...recorded.committed, receipt: '0x1' })).toThrow('receipt');
}

test(read_sequencer_receipt_calls_pax_sequencer_receipt_and_decodes_each_rung);
test(decode_sequencer_receipt_refuses_an_unknown_rung_or_a_foreign_hash);

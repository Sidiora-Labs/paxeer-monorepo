import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, expectTypeOf, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import { KernelAvailability, KernelClient } from '../src/kernel.js';
import { JsonRpcError, JsonRpcTransportError } from '../src/rpc.js';
import type {
  Balances,
  ChainAccount,
  JsonRpcErrorObject,
  KernelRead,
  SequencerReceiptAnswer,
} from '../src/types.js';

async function kernel_client_reads_account_and_balances_through_px_methods(): Promise<void> {
  const address = `0x${'ab'.repeat(20)}`;
  const account: ChainAccount = {
    account: 'pax1account',
    paxeer: { address, balance: '90071992547409931234567890', nonce: '7', height: '42' },
  };
  const balances: Balances = {
    account: address,
    height: '42',
    balances: [{ denom: 'apax', amount: '90071992547409931234567890' }],
  };
  const requests: { method: string; params: unknown[] }[] = [];
  let error: JsonRpcErrorObject | null = null;
  let malformed = false;
  const server = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    requests.push({ method: body.method, params: body.params });
    const result = body.method === 'px_getAccount' ? account : balances;
    const response = error !== null
      ? { error }
      : body.method === 'px_getAccount' || body.method === 'px_getBalances'
        ? { result: malformed ? {} : result }
        : { error: { code: -32601, message: 'Method not found' } };
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, ...response }));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const endpoint = new EndpointClient({ url: `http://127.0.0.1:${(server.address() as AddressInfo).port}` });
    const availability = new KernelAvailability(endpoint, { ttlMs: 60_000 });
    availability.record({ available: true, reason: 'available' });
    const kernel = new KernelClient(endpoint, availability);
    expectTypeOf(kernel.getAccount).returns.toEqualTypeOf<Promise<KernelRead<ChainAccount>>>();
    expectTypeOf(kernel.getBalances).returns.toEqualTypeOf<Promise<KernelRead<Balances>>>();
    await expect(kernel.getAccount(address)).resolves.toEqual({ available: true, result: account });
    await expect(kernel.getBalances(address)).resolves.toEqual({ available: true, result: balances });
    expect(requests).toEqual([
      { method: 'px_getAccount', params: [address] },
      { method: 'px_getBalances', params: [address] },
    ]);

    malformed = true;
    await expect(kernel.getAccount(address)).rejects.toThrow(JsonRpcTransportError);
    await expect(kernel.getBalances(address)).rejects.toThrow(TypeError);
    malformed = false;

    error = { code: -32001, message: 'Read refused' };
    await expect(kernel.getAccount(address)).rejects.toBeInstanceOf(JsonRpcError);
    await expect(kernel.getBalances(address)).rejects.toMatchObject(error);

    for (const read of [() => kernel.getAccount(address), () => kernel.getBalances(address)]) {
      availability.record({ available: true, reason: 'available' });
      error = {
        code: -32010,
        message: 'Kernel unavailable',
        data: { code: 'kernel_unavailable', reason: 'unreachable', backend: 'public_core' },
      };
      const unavailable = { available: false, reason: 'unreachable', backend: 'public_core' };
      await expect(read()).resolves.toEqual(unavailable);
      const count = requests.length;
      await expect(availability.current()).resolves.toEqual(unavailable);
      await expect(kernel.getAccount(address)).resolves.toEqual(unavailable);
      await expect(kernel.getBalances(address)).resolves.toEqual(unavailable);
      expect(requests).toHaveLength(count);
    }
    expect(requests).toEqual(Array.from({ length: 4 }, () => [
      { method: 'px_getAccount', params: [address] },
      { method: 'px_getBalances', params: [address] },
    ]).flat());
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

test(kernel_client_reads_account_and_balances_through_px_methods, kernel_client_reads_account_and_balances_through_px_methods);

async function kernel_client_reads_the_pending_nonce_and_sequencer_receipt(): Promise<void> {
  const address = `0x${'ab'.repeat(20)}`;
  const receipts = JSON.parse(
    readFileSync(new URL('./fixtures/sequencer-receipt.json', import.meta.url), 'utf8')
  ) as Record<'submitted' | 'receipted' | 'committed', SequencerReceiptAnswer>;
  const hash = receipts.submitted.txHash;
  const requests: { method: string; params: unknown[] }[] = [];
  let nonce: unknown = '0x0';
  let receipt: unknown = null;
  let error: JsonRpcErrorObject | null = null;
  const server = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    requests.push({ method: body.method, params: body.params });
    const response = error !== null
      ? { error }
      : body.method === 'eth_getTransactionCount'
        ? { result: nonce }
        : body.method === 'pax_sequencerReceipt'
          ? { result: receipt }
          : { error: { code: -32601, message: 'Method not found' } };
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, ...response }));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const endpoint = new EndpointClient({ url: `http://127.0.0.1:${(server.address() as AddressInfo).port}` });
    const availability = new KernelAvailability(endpoint, { ttlMs: 60_000 });
    availability.record({ available: true, reason: 'available' });
    const kernel = new KernelClient(endpoint, availability);
    expectTypeOf(kernel.getSequence).returns.toEqualTypeOf<Promise<KernelRead<string>>>();
    expectTypeOf(kernel.getReceipt).returns.toEqualTypeOf<
      Promise<KernelRead<SequencerReceiptAnswer | null>>
    >();
    expectTypeOf<KernelClient>().not.toHaveProperty('getActivityStatus');
    expect('getActivityStatus' in kernel).toBe(false);

    for (const [quantity, decimal] of [
      ['0x0', '0'],
      ['0x2a', '42'],
      ['0xffffffffffffffff', '18446744073709551615'],
    ]) {
      nonce = quantity;
      await expect(kernel.getSequence(address)).resolves.toEqual({ available: true, result: decimal });
    }
    for (const result of [
      receipts.submitted,
      {
        ...receipts.receipted,
        blockNumber: receipts.committed.blockNumber,
        blockHash: receipts.committed.blockHash,
      },
      receipts.committed,
      null,
    ]) {
      receipt = result;
      await expect(kernel.getReceipt(hash)).resolves.toEqual({ available: true, result });
    }
    expect(requests).toEqual([
      ...Array.from({ length: 3 }, () => ({
        method: 'eth_getTransactionCount',
        params: [address, 'pending'],
      })),
      ...Array.from({ length: 4 }, () => ({ method: 'pax_sequencerReceipt', params: [hash] })),
    ]);

    const beforeInvalid = requests.length;
    for (const invalid of ['', 'ab'.repeat(32), `0x${'ab'.repeat(32)}`]) {
      expect(() => kernel.getSequence(invalid)).toThrow(RangeError);
    }
    await expect(kernel.getReceipt('invalid')).rejects.toThrow('transaction hash');
    expect(requests).toHaveLength(beforeInvalid);
    for (const invalid of [null, {}, 1, '42', '0x', '0x00', '-0x1', '0xgg']) {
      nonce = invalid;
      await expect(kernel.getSequence(address)).rejects.toThrow(TypeError);
    }
    for (const invalid of [
      {},
      { ...receipts.submitted, rung: 'final' },
      { ...receipts.submitted, txHash: `0x${'ff'.repeat(32)}` },
    ]) {
      receipt = invalid;
      await expect(kernel.getReceipt(hash)).rejects.toThrow(TypeError);
    }

    error = { code: -32001, message: 'Read refused' };
    await expect(kernel.getSequence(address)).rejects.toMatchObject(error);
    await expect(kernel.getReceipt(hash)).rejects.toBeInstanceOf(JsonRpcError);
    for (const read of [() => kernel.getSequence(address), () => kernel.getReceipt(hash)]) {
      availability.record({ available: true, reason: 'available' });
      error = {
        code: -32010,
        message: 'Kernel unavailable',
        data: { code: 'kernel_unavailable', reason: 'unreachable', backend: 'public_core' },
      };
      const unavailable = { available: false, reason: 'unreachable', backend: 'public_core' };
      await expect(read()).resolves.toEqual(unavailable);
      const count = requests.length;
      await expect(availability.current()).resolves.toEqual(unavailable);
      await expect(kernel.getSequence(address)).resolves.toEqual(unavailable);
      await expect(kernel.getReceipt(hash)).resolves.toEqual(unavailable);
      expect(requests).toHaveLength(count);
    }
    expect(requests.every(({ method }) =>
      method === 'eth_getTransactionCount' || method === 'pax_sequencerReceipt'
    )).toBe(true);
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

test(
  kernel_client_reads_the_pending_nonce_and_sequencer_receipt,
  kernel_client_reads_the_pending_nonce_and_sequencer_receipt
);

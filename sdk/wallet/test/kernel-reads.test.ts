import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, expectTypeOf, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import { KernelAvailability, KernelClient } from '../src/kernel.js';
import { JsonRpcError, JsonRpcTransportError } from '../src/rpc.js';
import type { Balances, ChainAccount, JsonRpcErrorObject, KernelRead } from '../src/types.js';

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

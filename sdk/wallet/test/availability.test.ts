import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import {
  DEFAULT_KERNEL_AVAILABILITY_TTL_MS,
  KERNEL_UNAVAILABLE_CODE,
  KernelAvailability,
  KernelClient,
  kernelUnavailableFrom,
} from '../src/kernel.js';
import { JsonRpcError, JsonRpcTransportError } from '../src/rpc.js';
import type { ConsensusHead, JsonRpcErrorObject } from '../src/types.js';

async function kernel_availability_follows_the_consensus_head(): Promise<void> {
  const head: ConsensusHead = {
    latest_height: '9007199254740993',
    latest_block_hash: 'a1'.repeat(32),
    latest_app_hash: 'b2'.repeat(32),
    catching_up: false,
  };
  let consensus: ConsensusHead | null = head;
  let error: JsonRpcErrorObject | null = null;
  let malformed = false;
  let now = 0;
  const requests: { method: string; params: unknown[] }[] = [];
  const server = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    requests.push({ method: body.method, params: body.params });
    const response = error !== null
      ? { error }
      : {
          result: malformed ? {} : {
            network_id: 'paxeer',
            paxeer: { chain_id: '0x7d', latest_block: '0x123' },
            consensus,
            ...(consensus === null ? { reason: 'consensus_unavailable' } : {}),
          },
        };
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, ...response }));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const endpoint = new EndpointClient({
      url: `http://127.0.0.1:${(server.address() as AddressInfo).port}`,
    });
    const availability = new KernelAvailability(endpoint, { now: () => now });
    const available = { available: true, reason: 'available' } as const;
    const unavailable = {
      available: false, reason: 'consensus_unavailable', backend: null,
    } as const;
    expect(DEFAULT_KERNEL_AVAILABILITY_TTL_MS).toBe(5_000);
    expect(KERNEL_UNAVAILABLE_CODE).toBe(-32010);

    await expect(Promise.all([availability.current(), availability.current()]))
      .resolves.toEqual([available, available]);
    expect(requests).toEqual([{ method: 'px_getNetwork', params: [] }]);

    consensus = { ...head, catching_up: true };
    now = DEFAULT_KERNEL_AVAILABILITY_TTL_MS - 1;
    await expect(availability.current()).resolves.toEqual(available);
    expect(requests).toHaveLength(1);
    now += 1;
    await expect(availability.current()).resolves.toEqual(unavailable);
    expect(requests).toHaveLength(2);

    const kernel = new KernelClient(endpoint, availability);
    await expect(kernel.getSequence(`0x${'12'.repeat(20)}`)).resolves.toEqual(unavailable);
    expect(requests).toHaveLength(2);
    consensus = head;
    now += DEFAULT_KERNEL_AVAILABILITY_TTL_MS - 1;
    await expect(availability.current()).resolves.toEqual(unavailable);
    expect(requests).toHaveLength(2);
    now += 1;
    await expect(availability.current()).resolves.toEqual(available);
    expect(requests).toHaveLength(3);

    for (const next of [null, { ...head, catching_up: true }, head]) {
      consensus = next;
      availability.invalidate();
      const expected = next !== null && !next.catching_up ? available : unavailable;
      await expect(availability.current()).resolves.toEqual(expected);
      const count = requests.length;
      await expect(availability.current()).resolves.toEqual(expected);
      expect(requests).toHaveLength(count);
    }

    for (const latest_height of ['0', '18446744073709551615']) {
      consensus = { ...head, latest_height };
      availability.invalidate();
      await expect(availability.current()).resolves.toEqual(available);
    }

    for (const reason of ['not_configured', 'unreachable', 'consensus_unavailable'] as const) {
      error = {
        code: KERNEL_UNAVAILABLE_CODE,
        message: 'Kernel unavailable',
        data: { code: 'kernel_unavailable', reason, backend: 'public_core' },
      };
      const recorded = { available: false, reason, backend: 'public_core' } as const;
      availability.record(available);
      await expect(kernel.getSequence(`0x${'12'.repeat(20)}`)).resolves.toEqual(recorded);
      const count = requests.length;
      now += DEFAULT_KERNEL_AVAILABILITY_TTL_MS - 1;
      await expect(availability.current()).resolves.toEqual(recorded);
      expect(requests).toHaveLength(count);
      error = null;
      now += 1;
      await expect(availability.current()).resolves.toEqual(available);
      expect(requests).toHaveLength(count + 1);
    }

    const rpcError = new JsonRpcError({
      code: KERNEL_UNAVAILABLE_CODE,
      message: 'Kernel unavailable',
      data: { code: 'kernel_unavailable', reason: 'consensus_unavailable', backend: 'unknown' },
    });
    expect(kernelUnavailableFrom(rpcError)).toEqual(unavailable);
    for (const invalid of [
      new Error('Kernel unavailable'),
      { code: KERNEL_UNAVAILABLE_CODE, data: rpcError.data },
      new JsonRpcError({ code: -32001, message: 'Refused', data: rpcError.data }),
      new JsonRpcError({ code: KERNEL_UNAVAILABLE_CODE, message: 'Refused' }),
      new JsonRpcError({
        code: KERNEL_UNAVAILABLE_CODE, message: 'Refused',
        data: { code: 'other', reason: 'consensus_unavailable' },
      }),
      new JsonRpcError({
        code: KERNEL_UNAVAILABLE_CODE, message: 'Refused',
        data: { code: 'kernel_unavailable', reason: 'available' },
      }),
    ]) expect(kernelUnavailableFrom(invalid)).toBeNull();

    availability.invalidate();
    malformed = true;
    await expect(availability.current()).rejects.toBeInstanceOf(JsonRpcTransportError);
    malformed = false;
    await expect(availability.current()).resolves.toEqual(available);
    availability.invalidate();
    error = { code: -32001, message: 'Read refused' };
    await expect(availability.current()).rejects.toMatchObject(error);
    error = null;
    await expect(availability.current()).resolves.toEqual(available);
    expect(requests.filter(({ method }) => method === 'eth_getTransactionCount')).toEqual(
      Array.from({ length: 3 }, () => ({
        method: 'eth_getTransactionCount', params: [`0x${'12'.repeat(20)}`, 'pending'],
      }))
    );
    expect(requests.every(({ method }) =>
      method === 'px_getNetwork' || method === 'eth_getTransactionCount'
    )).toBe(true);
  } finally {
    await new Promise<void>((resolve, reject) =>
      server.close((error) => error ? reject(error) : resolve())
    );
  }
}

test(kernel_availability_follows_the_consensus_head, kernel_availability_follows_the_consensus_head);

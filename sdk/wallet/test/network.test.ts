import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, expectTypeOf, test } from 'vitest';
import { decodeKernelStatus, EndpointClient } from '../src/endpoint.js';
import { KernelAvailability } from '../src/kernel.js';
import { JsonRpcTransportError } from '../src/rpc.js';
import type {
  Balances,
  ChainAccount,
  ConsensusHead,
  KernelDocument,
  KernelRead,
  KernelReason,
  NetworkHead,
  SequencerReceiptAnswer,
} from '../src/types.js';

async function endpoint_network_decodes_the_consensus_head_or_its_absence(): Promise<void> {
  const consensus: ConsensusHead = {
    latest_height: '9007199254740993',
    latest_block_hash: 'A1'.repeat(32),
    latest_app_hash: 'b2'.repeat(32),
    catching_up: false,
  };
  const answer = {
    network_id: 'paxeer-degraded',
    paxeer: { chain_id: '0x7d', latest_block: '0x123' },
    consensus,
  } satisfies Omit<NetworkHead, 'consensus_reason'>;
  const expected: NetworkHead = { ...answer, consensus_reason: null };
  let result: unknown = answer;
  const requests: unknown[] = [];
  const server = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    requests.push({ method: req.method, body });
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const client = new EndpointClient({ url: `http://127.0.0.1:${(server.address() as AddressInfo).port}` });
    expectTypeOf(client.getNetwork).returns.toEqualTypeOf<Promise<NetworkHead>>();
    expectTypeOf<NetworkHead['consensus']>().toEqualTypeOf<ConsensusHead | null>();
    expectTypeOf<NetworkHead['consensus_reason']>().toEqualTypeOf<'consensus_unavailable' | null>();
    expectTypeOf<Extract<keyof NetworkHead, 'layerx' | 'anchor' | 'kernel'>>().toEqualTypeOf<never>();
    expectTypeOf<KernelReason>().toEqualTypeOf<'available' | 'not_configured' | 'unreachable' | 'consensus_unavailable'>();
    expectTypeOf<KernelDocument>().toEqualTypeOf<ChainAccount | Balances | SequencerReceiptAnswer | null>();
    expectTypeOf<Extract<KernelRead<ChainAccount>, { available: true }>['result']>().toEqualTypeOf<ChainAccount>();
    await expect(client.getNetwork()).resolves.toEqual(expected);
    expect(requests).toEqual([{
      method: 'POST',
      body: { jsonrpc: '2.0', id: 1, method: 'px_getNetwork', params: [] },
    }]);
    const availability = new KernelAvailability(client);
    await expect(availability.current()).resolves.toEqual({ available: true, reason: 'available' });
    const availableRequestCount = requests.length;
    await expect(availability.current()).resolves.toEqual({ available: true, reason: 'available' });
    expect(requests).toHaveLength(availableRequestCount);

    for (const latest_height of ['0', '34742403', '9007199254740993', '18446744073709551615']) {
      result = { ...answer, consensus: { ...consensus, latest_height, catching_up: true } };
      await expect(client.getNetwork()).resolves.toEqual({
        ...expected, consensus: { ...consensus, latest_height, catching_up: true },
      });
    }
    result = { ...answer, consensus: { ...consensus, latest_block_hash: '', latest_app_hash: '' } };
    await expect(client.getNetwork()).resolves.toEqual({
      ...expected, consensus: { ...consensus, latest_block_hash: '', latest_app_hash: '' },
    });

    result = { ...answer, consensus: null, reason: 'consensus_unavailable' };
    await expect(client.getNetwork()).resolves.toEqual({
      ...expected, consensus: null, consensus_reason: 'consensus_unavailable',
    });
    availability.invalidate();
    await expect(availability.current()).resolves.toEqual({
      available: false, reason: 'consensus_unavailable', backend: null,
    });
    const unavailableRequestCount = requests.length;
    await expect(availability.current()).resolves.toEqual({
      available: false, reason: 'consensus_unavailable', backend: null,
    });
    expect(requests).toHaveLength(unavailableRequestCount);
    result = answer;
    availability.invalidate();
    await expect(availability.current()).resolves.toEqual({ available: true, reason: 'available' });

    for (const invalid of [
      null,
      [],
      { ...answer, network_id: null },
      { ...answer, paxeer: null },
      { ...answer, paxeer: { ...answer.paxeer, chain_id: '125' } },
      { ...answer, paxeer: { ...answer.paxeer, latest_block: 291 } },
      { network_id: answer.network_id, paxeer: answer.paxeer },
      { ...answer, consensus: [] },
      { ...answer, consensus: {} },
      { ...answer, consensus: null },
      { ...answer, consensus: null, reason: null },
      { ...answer, consensus: null, reason: 'unreachable' },
      { ...answer, consensus: null, consensus_reason: 'consensus_unavailable' },
      { ...answer, reason: 'consensus_unavailable' },
      { ...answer, reason: null },
      { ...answer, consensus: null, layerx: { node_info: null }, anchor: null, kernel: { available: true, reason: 'available' } },
    ]) {
      result = invalid;
      await expect(client.getNetwork()).rejects.toThrow(JsonRpcTransportError);
    }
    for (const latest_height of [
      undefined, null, 42, Number.MAX_SAFE_INTEGER + 1, '', '-1', '1.5', '0x10', '1e3', '01', ' 1', '1\n',
      '18446744073709551616',
    ]) {
      result = { ...answer, consensus: { ...consensus, latest_height } };
      await expect(client.getNetwork()).rejects.toThrow('malformed consensus latest_height');
    }
    for (const key of ['latest_block_hash', 'latest_app_hash'] as const) {
      for (const hash of [undefined, null, 42, '', '0a', '0x12', 'abc', 'gg', 'aa\n']) {
        if (hash === '' || hash === '0a') {
          result = { ...answer, consensus: { ...consensus, [key]: hash } };
          await expect(client.getNetwork()).resolves.toEqual({ ...expected, consensus: { ...consensus, [key]: hash } });
        } else {
          result = { ...answer, consensus: { ...consensus, [key]: hash } };
          await expect(client.getNetwork()).rejects.toThrow(`malformed consensus ${key}`);
        }
      }
    }
    for (const catching_up of [undefined, null, 0, 'false']) {
      result = { ...answer, consensus: { ...consensus, catching_up } };
      await expect(client.getNetwork()).rejects.toThrow('malformed consensus catching_up');
    }
    for (const [index, request] of requests.entries()) {
      expect(request).toEqual({
        method: 'POST',
        body: { jsonrpc: '2.0', id: index + 1, method: 'px_getNetwork', params: [] },
      });
    }

    for (const reason of ['available', 'not_configured', 'unreachable', 'consensus_unavailable'] as const) {
      const status = { available: reason === 'available', reason };
      expect(decodeKernelStatus(status)).toEqual(status);
      expect(() => decodeKernelStatus({ ...status, available: !status.available })).toThrow(JsonRpcTransportError);
    }
    expect(() => decodeKernelStatus({ available: false, reason: 'no_finalised_checkpoint' })).toThrow(JsonRpcTransportError);
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

test(endpoint_network_decodes_the_consensus_head_or_its_absence, endpoint_network_decodes_the_consensus_head_or_its_absence);

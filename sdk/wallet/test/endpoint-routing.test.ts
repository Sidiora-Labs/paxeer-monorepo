import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import type { ChainAccount, JsonRpcCall, UnifiedAccountDocument } from '../src/types.js';

const address = '0x1111111111111111111111111111111111111111';
const account: UnifiedAccountDocument = {
  evm_address: address,
  pax_address: null,
  layerx_did: null,
  layerx_account: null,
  bound: false,
};
const chainAccount: ChainAccount = {
  account,
  paxeer: { address, balance: '0xde0b6b3a7640000', nonce: '0x3', height: '1715004' },
};

type Request = JsonRpcCall & { jsonrpc: '2.0'; id: number };
type Exchange = { method: string | undefined; path: string | undefined; body: Request | Request[] };

async function rpcEndpoint(source: 'node' | 'hosted') {
  const exchanges: Exchange[] = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body: Request | Request[] = JSON.parse(Buffer.concat(chunks).toString('utf8'));
      exchanges.push({ method: request.method, path: request.url, body });
      if (request.method !== 'POST' || request.url !== '/') {
        response.writeHead(404).end();
        return;
      }
      const answer = (call: Request) => {
        let result: unknown;
        if (Array.isArray(body)) result = { source, method: call.method, params: call.params };
        else if (call.method === 'px_resolveAccount') result = account;
        else if (call.method === 'px_getAccount') result = chainAccount;
        else throw new Error(`unexpected method ${call.method}`);
        return { jsonrpc: '2.0', id: call.id, result };
      };
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(JSON.stringify(Array.isArray(body) ? body.map(answer).reverse() : answer(body)));
    } catch (error) {
      response.writeHead(500).end(String(error));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  return {
    server,
    url: `http://127.0.0.1:${(server.address() as AddressInfo).port}/`,
    exchanges,
  };
}

async function closeEndpoint(server: Server): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
}

async function walletAccountReadsUseNodeWithAndWithoutHosted(): Promise<void> {
  const node = await rpcEndpoint('node');
  try {
    const hosted = await rpcEndpoint('hosted');
    try {
      for (const hostedUrl of [hosted.url, undefined]) {
        const client = new EndpointClient({ nodeUrl: node.url, hostedUrl });
        await expect(client.resolveAccount(address)).resolves.toEqual(account);
        await expect(client.getChainAccount(address)).resolves.toEqual(chainAccount);
      }
      expect(hosted.exchanges).toEqual([]);
      expect(node.exchanges).toEqual(
        [1, 2].flatMap(() => [
          {
            method: 'POST', path: '/',
            body: { jsonrpc: '2.0', id: 1, method: 'px_resolveAccount', params: [address] },
          },
          {
            method: 'POST', path: '/',
            body: { jsonrpc: '2.0', id: 2, method: 'px_getAccount', params: [address] },
          },
        ])
      );
    } finally {
      await closeEndpoint(hosted.server);
    }
  } finally {
    await closeEndpoint(node.server);
  }
}

async function walletHostedOperationsUseHostedTransport(): Promise<void> {
  const node = await rpcEndpoint('node');
  try {
    const hosted = await rpcEndpoint('hosted');
    try {
      const client = new EndpointClient({ nodeUrl: node.url, hostedUrl: hosted.url });
      const calls: JsonRpcCall[] = [
        { method: 'px_register', params: [{ address }] },
        { method: 'px_requestFunds', params: [address] },
      ];
      for (const call of calls) {
        await expect(client.batch([call])).resolves.toEqual([
          { ok: true, result: { source: 'hosted', ...call } },
        ]);
      }
      expect(hosted.exchanges).toEqual(calls.map((call, index) => ({
        method: 'POST', path: '/',
        body: [{ jsonrpc: '2.0', id: index + 1, ...call }],
      })));
      const withoutHosted = new EndpointClient({ nodeUrl: node.url });
      for (const call of calls) {
        await expect(withoutHosted.batch([call])).rejects.toThrow('hostedUrl required');
      }
      expect(node.exchanges).toEqual([]);
      expect(hosted.exchanges).toHaveLength(2);
    } finally {
      await closeEndpoint(hosted.server);
    }
  } finally {
    await closeEndpoint(node.server);
  }
}

async function walletMixedBatchSplitsTransportsAndPreservesOrder(): Promise<void> {
  const node = await rpcEndpoint('node');
  try {
    const hosted = await rpcEndpoint('hosted');
    try {
      const client = new EndpointClient({ nodeUrl: node.url, hostedUrl: hosted.url });
      const calls: JsonRpcCall[] = [
        { method: 'px_getAccount', params: [address] },
        { method: 'px_register', params: [{ address }] },
        { method: 'px_resolveAccount', params: [address] },
        { method: 'px_getReceiptKeys', params: [] },
        { method: 'px_requestFunds', params: [address] },
        { method: 'px_listAssets', params: [] },
        { method: 'px_getRouteCatalogue', params: [] },
        { method: 'px_getCapabilities', params: [] },
        { method: 'px_getNetwork', params: [] },
        { method: 'px_getBalances', params: [address] },
        { method: 'px_getUnifiedHistory', params: [address, null, 2, null] },
        { method: 'px_registerAccount', params: [address] },
        { method: 'px_requestFundsStatus', params: [address] },
        { method: 'eth_chainId', params: [] },
      ];
      const hostedIndexes = new Set([1, 4]);
      await expect(client.batch(calls)).resolves.toEqual(calls.map((call, index) => ({
        ok: true,
        result: { source: hostedIndexes.has(index) ? 'hosted' : 'node', ...call },
      })));
      for (const [endpoint, isHosted] of [[node, false], [hosted, true]] as const) {
        expect(endpoint.exchanges).toEqual([{
          method: 'POST', path: '/',
          body: calls.filter((_, index) => hostedIndexes.has(index) === isHosted)
            .map((call, index) => ({ jsonrpc: '2.0', id: index + 1, ...call })),
        }]);
      }
      await expect(client.batch([])).resolves.toEqual([]);
      expect(node.exchanges).toHaveLength(1);
      expect(hosted.exchanges).toHaveLength(1);
    } finally {
      await closeEndpoint(hosted.server);
    }
  } finally {
    await closeEndpoint(node.server);
  }
}

test(walletAccountReadsUseNodeWithAndWithoutHosted, walletAccountReadsUseNodeWithAndWithoutHosted);
test(walletHostedOperationsUseHostedTransport, walletHostedOperationsUseHostedTransport);
test(walletMixedBatchSplitsTransportsAndPreservesOrder, walletMixedBatchSplitsTransportsAndPreservesOrder);

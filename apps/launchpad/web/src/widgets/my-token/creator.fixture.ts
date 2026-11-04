import { createServer } from 'node:http';
import { strict as assert } from 'node:assert';
import type { Address } from 'viem';
import {
  RecordedProvider,
  recordedCall,
  type RpcRequest,
} from '../swap/swap-chain.fixture';
import recording from './creator-chain.recording.json';
import metadata from './creator-metadata.recording.json';

export async function creatorFixture() {
  const coins = recording.coins.map((coin) => ({
    ...coin,
    Pool: coin.Pool as Address,
    Token: coin.Token as Address,
    Creator: coin.Creator as Address,
  }));
  const provider = new RecordedProvider(({ method, params = [] }) => {
    switch (method) {
      case 'eth_chainId':
        return recording.chainId;
      case 'eth_accounts':
      case 'eth_requestAccounts':
        return [coins[0].Creator];
      case 'eth_call':
        return recordedCall(recording.calls, params);
      default:
        throw new Error(`Unrecorded creator RPC method: ${method}`);
    }
  });
  const requests: string[] = [];
  const errors: unknown[] = [];
  const server = createServer(async (request, response) => {
    try {
      const url = new URL(request.url!, 'http://localhost');
      if (url.pathname === '/rpc') {
        assert.equal(request.method, 'POST');
        let body = '';
        for await (const chunk of request) body += String(chunk);
        const rpc = JSON.parse(body) as RpcRequest & { id: number };
        const result = await provider.request(
          rpc as Parameters<typeof provider.request>[0],
        );
        response.setHeader('content-type', 'application/json');
        response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, result }));
        return;
      }
      assert.equal(request.method, 'GET');
      requests.push(url.pathname.toLowerCase() + url.search.toLowerCase());
      const record = metadata.find(
        (row) => row.path === url.pathname.toLowerCase(),
      );
      assert.ok(record, `Unrecorded HTTP request: ${url.pathname}`);
      response.writeHead(record.status, { 'content-type': 'application/json' });
      response.end(JSON.stringify(record.body));
    } catch (error) {
      errors.push(error);
      response.writeHead(500, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  const origin = `http://127.0.0.1:${address.port}`;
  return {
    origin,
    provider,
    coins,
    requests,
    errors,
    async close() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
      await provider.disconnect();
    },
  };
}

export type CreatorFixture = Awaited<ReturnType<typeof creatorFixture>>;

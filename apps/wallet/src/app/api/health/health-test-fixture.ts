import { once } from 'node:events';
import { createServer } from 'node:http';
import { expect } from 'vitest';

export async function assertHealthRpcChainReadiness(
  cases: ReadonlyArray<{ chainId: unknown; status: 'ok' | 'degraded' }>
) {
  let chainId: unknown;
  const requests: Array<{ method: string | undefined; path: string | undefined; body: string }> =
    [];
  const server = createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    requests.push({
      method: request.method,
      path: request.url,
      body: Buffer.concat(chunks).toString('utf-8'),
    });
    response.setHeader('Content-Type', 'application/json');
    if (request.method === 'POST' && request.url === '/rpc') {
      response.end(JSON.stringify({ jsonrpc: '2.0', id: 1, result: chainId }));
    } else if (request.method === 'GET' && request.url === '/healthz') {
      response.end(
        JSON.stringify({ ok: true, service: 'paxeer-wallet-api', version: '0.1.0', chain_id: 125 })
      );
    } else if (
      request.method === 'GET' &&
      (request.url === '/indexer/health' || request.url === '/identity/auth/v1/health')
    ) {
      response.end(JSON.stringify({ ok: true }));
    } else {
      response.writeHead(404);
      response.end(JSON.stringify({ error: 'not_found' }));
    }
  });
  const envNames = [
    'NEXT_PUBLIC_PAXEER_WALLET_API',
    'NEXT_PUBLIC_PAXEER_RPC_URL',
    'NEXT_PUBLIC_SUPABASE_URL',
    'NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY',
    'NEXT_PUBLIC_AUTH_REDIRECT_URL',
    'NEXT_PUBLIC_VAPID_PUBLIC_KEY',
    'VAPID_PRIVATE_KEY',
    'HEALTH_CAPTURE_DEGRADED',
  ];
  const originalEnv = envNames.map((name) => [name, process.env[name]] as const);
  let restoreConfig: (() => void) | undefined;

  try {
    server.listen(0, '127.0.0.1');
    await once(server, 'listening');
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('Expected a TCP listener');
    const origin = `http://127.0.0.1:${address.port}`;
    process.env.NEXT_PUBLIC_PAXEER_WALLET_API = origin;
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL = `${origin}/rpc`;
    process.env.NEXT_PUBLIC_SUPABASE_URL = `${origin}/identity`;
    process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY = 'health-route-fixture';
    delete process.env.NEXT_PUBLIC_AUTH_REDIRECT_URL;
    process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = 'health-route-fixture-public';
    process.env.VAPID_PRIVATE_KEY = 'health-route-fixture-private';
    process.env.HEALTH_CAPTURE_DEGRADED = 'false';

    const { PAXEER_CONFIG } = await import('@/lib/constants');
    const originalConfig = {
      rpcUrl: PAXEER_CONFIG.rpcUrl,
      portfolioApiBase: PAXEER_CONFIG.portfolioApiBase,
    };
    restoreConfig = () => Object.assign(PAXEER_CONFIG, originalConfig);
    Object.assign(PAXEER_CONFIG, {
      rpcUrl: `${origin}/rpc`,
      portfolioApiBase: `${origin}/indexer`,
    });
    const { GET } = await import('./route');

    for (const scenario of cases) {
      chainId = scenario.chainId;
      requests.length = 0;
      const response = await GET(new Request(`${origin}/api/health?mode=readiness`));
      const body = await response.json();

      expect(requests).toEqual([
        {
          method: 'POST',
          path: '/rpc',
          body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'eth_chainId', params: [] }),
        },
        { method: 'GET', path: '/indexer/health', body: '' },
        { method: 'GET', path: '/identity/auth/v1/health', body: '' },
        { method: 'GET', path: '/healthz', body: '' },
      ]);
      expect(body.checks.rpc).toEqual({ status: scenario.status, latencyMs: expect.any(Number) });
      for (const name of ['indexer', 'supabase', 'embeddedWallet', 'push']) {
        expect(body.checks[name]).toEqual({ status: 'ok', latencyMs: expect.any(Number) });
      }
      expect(body.status).toBe(scenario.status);
      expect(body.check).toBe('readiness');
      expect(response.status).toBe(scenario.status === 'ok' ? 200 : 503);
      expect(response.headers.get('cache-control')).toBe('no-store');
    }
  } finally {
    restoreConfig?.();
    for (const [name, value] of originalEnv) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}

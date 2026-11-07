import { createServer } from 'node:http';
import { once } from 'node:events';
import { expect, test } from 'vitest';
import { assertHealthRpcChainReadiness } from './health-test-fixture';

async function health_route_checks_the_gateway_healthz() {
  let gatewayStatus = 200;
  let gatewayVersion: unknown = '0.1.0';
  let gatewayChainId = 125;
  let gatewayService = 'paxeer-wallet-api';
  const requests: Array<{ method: string | undefined; path: string | undefined }> = [];
  const server = createServer((request, response) => {
    requests.push({ method: request.method, path: request.url });
    response.setHeader('Content-Type', 'application/json');
    if (request.method === 'GET' && request.url === '/healthz') {
      response.writeHead(gatewayStatus);
      response.end(
        JSON.stringify({
          ok: true,
          service: gatewayService,
          version: gatewayVersion,
          chain_id: gatewayChainId,
        })
      );
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
    'HEALTH_CAPTURE_DEGRADED',
  ];
  const originalEnv = envNames.map((name) => [name, process.env[name]] as const);

  try {
    server.listen(0, '127.0.0.1');
    await once(server, 'listening');
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('Expected a TCP listener');
    const origin = `http://127.0.0.1:${address.port}`;
    process.env.NEXT_PUBLIC_PAXEER_WALLET_API = origin;
    process.env.NEXT_PUBLIC_PAXEER_RPC_URL = `${origin}/rpc`;
    process.env.NEXT_PUBLIC_SUPABASE_URL = `${origin}/identity`;
    process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY = 'health-route-test';
    delete process.env.NEXT_PUBLIC_AUTH_REDIRECT_URL;
    process.env.HEALTH_CAPTURE_DEGRADED = 'false';

    const { GET } = await import('./route');
    for (const status of [200, 404, 503, 200]) {
      gatewayStatus = status;
      requests.length = 0;
      const response = await GET(new Request(`${origin}/api/health`));
      const body = await response.json();

      expect(requests).toEqual([
        { method: 'POST', path: '/rpc' },
        { method: 'GET', path: '/identity/auth/v1/health' },
        { method: 'GET', path: '/healthz' },
      ]);
      expect(body.checks.embeddedWallet).toEqual({
        status: status === 200 ? 'ok' : 'degraded',
        latencyMs: expect.any(Number),
      });
      expect(body.checks.embeddedWallet.latencyMs).toBeGreaterThanOrEqual(0);
      expect(body.apiVersion).toBe(status === 200 ? '0.1.0' : undefined);
      expect(Object.keys(body).sort()).toEqual(
        status === 200
          ? ['apiVersion', 'check', 'checks', 'status', 'timestamp']
          : ['check', 'checks', 'status', 'timestamp']
      );
      expect(Object.keys(body.checks).sort()).toEqual([
        'embeddedWallet',
        'indexer',
        'push',
        'rpc',
        'supabase',
      ]);
      expect(body.check).toBe('readiness');
      expect(body.status).toBe('degraded');
      expect(response.status).toBe(503);
      expect(new Date(body.timestamp).toISOString()).toBe(body.timestamp);
      expect(response.headers.get('cache-control')).toBe('no-store');
    }

    for (const version of ['1.4.2', '2.0.0-rc.1+build.42']) {
      gatewayVersion = version;
      const response = await GET(new Request(`${origin}/api/health`));
      const body = await response.json();

      expect(body.checks.embeddedWallet).toEqual({
        status: 'ok',
        latencyMs: expect.any(Number),
      });
      expect(body.apiVersion).toBe(version);
    }

    for (const version of ['', ' ', '1.4', 'v1.4.2', '01.4.2', '1.4.2-01', '1.4.2\n', 142, null]) {
      gatewayVersion = version;
      const response = await GET(new Request(`${origin}/api/health`));
      const body = await response.json();

      expect(body.checks.embeddedWallet).toEqual({
        status: 'degraded',
        latencyMs: expect.any(Number),
      });
      expect(body.apiVersion).toBeUndefined();
      expect(response.status).toBe(503);
    }

    gatewayVersion = '1.4.2';
    gatewayChainId = 124;
    const foreignChainResponse = await GET(new Request(`${origin}/api/health`));
    const foreignChainBody = await foreignChainResponse.json();
    expect(foreignChainBody.checks.embeddedWallet).toEqual({
      status: 'degraded',
      latencyMs: expect.any(Number),
    });
    expect(foreignChainBody.apiVersion).toBeUndefined();
    expect(foreignChainResponse.status).toBe(503);

    gatewayChainId = 125;
    gatewayService = 'another-service';
    const wrongServiceResponse = await GET(new Request(`${origin}/api/health`));
    const wrongServiceBody = await wrongServiceResponse.json();
    expect(wrongServiceBody.checks.embeddedWallet).toEqual({
      status: 'degraded',
      latencyMs: expect.any(Number),
    });
    expect(wrongServiceBody.apiVersion).toBeUndefined();
    expect(wrongServiceResponse.status).toBe(503);
  } finally {
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

test(health_route_checks_the_gateway_healthz, health_route_checks_the_gateway_healthz);

async function health_route_rejects_a_healthy_foreign_chain() {
  await assertHealthRpcChainReadiness([{ chainId: '0x1', status: 'degraded' }]);
}

test(health_route_rejects_a_healthy_foreign_chain, health_route_rejects_a_healthy_foreign_chain);

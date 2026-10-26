export const TRANSPILE_PACKAGES = Object.freeze([
  'candles-sdk',
  'indexer-sdk',
  'metadata-sdk',
  'ranking-algo-sdk',
  'stats-sdk',
  'users-sdk',
  'sidiora-launchpad-v3',
] as const);

export const NEXT_DEV_ALLOWED_ORIGINS = 'NEXT_DEV_ALLOWED_ORIGINS';

export function allowedDevOrigins(env: Readonly<Record<string, string | undefined>>): string[] {
  const origins = ['localhost', '127.0.0.1'];
  for (const entry of (env[NEXT_DEV_ALLOWED_ORIGINS] ?? '').split(',')) {
    const hostname = entry.trim();
    if (!hostname) continue;
    if (
      hostname.length > 253 ||
      !/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*\.?$/i.test(hostname)
    ) {
      throw new TypeError(`${NEXT_DEV_ALLOWED_ORIGINS} entries must be hostnames without a scheme, port, path or wildcard`);
    }
    origins.push(hostname);
  }
  return origins;
}

function upstream(value: string): string {
  return value.replace(/\/+$/, '');
}

/**
 * Same-origin backend paths. The wallet identity session requires the identity
 * origin to equal the page origin (core/wallet-sdk/auth.ts), and the network
 * authority / preflight / private snapshot reads are same-origin by contract,
 * so the web server proxies these paths to the private Railway services.
 *
 * Upstreams are server-side only (never NEXT_PUBLIC_) and are read when the
 * build writes routes-manifest.json, so they must be set as build args.
 * Next's external-rewrite proxy sets Host to the upstream, adds
 * X-Forwarded-Host = the public web host, and passes the edge's
 * X-Forwarded-Proto / X-Forwarded-For / Origin through unchanged.
 *
 * WebSocket upgrades are proxied by the same rewrite in the standalone server
 * (router-server upgradeHandler -> proxyRequest(ws: true)), but the public
 * market stream is opened directly on NEXT_PUBLIC_STREAM_ORIGIN; only the
 * private stream uses NEXT_PUBLIC_PRIVATE_STREAM_ORIGIN.
 *
 * Path ownership on the web origin follows the production topology
 * (deploy/production/topology/topology.py): `/v2/auth/*` and
 * `/v2/stream/tickets` -> media/gateway; `/v2/snapshots` -> core/api REST
 * (API_ROLE=rest); `/v2/stream` and `/v2/stream/sse` -> core/api sockets
 * (API_ROLE=sockets). The gateway's own `/v2/stream` (the social chat socket)
 * is never addressed through the web origin: the chat client opens it on the
 * absolute gateway host (NEXT_PUBLIC_GATEWAY_URL + NEXT_PUBLIC_SOCIAL_WS_PATH),
 * so `/v2/stream` here is unambiguously the market/private data stream.
 */
export async function backendRewrites(env: Readonly<Record<string, string | undefined>>) {
  const gatewayValue = env.WEB_GATEWAY_UPSTREAM;
  const apiValue = env.WEB_API_UPSTREAM;
  const socketsValue = env.WEB_SOCKETS_UPSTREAM;
  const chartsValue = env.WEB_CHARTS_UPSTREAM;
  if (env.NODE_ENV === 'production') {
    for (const name of [
      'WEB_GATEWAY_UPSTREAM',
      'WEB_API_UPSTREAM',
      'WEB_SOCKETS_UPSTREAM',
      'WEB_CHARTS_UPSTREAM',
    ] as const) {
      if (!env[name]) throw new Error(`missing production upstream: ${name}`);
    }
  }
  if (!gatewayValue || !apiValue || !socketsValue || !chartsValue) {
    return { beforeFiles: [], afterFiles: [], fallback: [] };
  }
  const gateway = upstream(gatewayValue);
  const api = upstream(apiValue);
  const sockets = upstream(socketsValue);
  const charts = upstream(chartsValue);
  return {
    beforeFiles: [
      // Identity (media/gateway): challenges, sessions, `sessions:refresh`, and stream tickets.
      { source: '/v2/auth/:path*', destination: `${gateway}/v2/auth/:path*` },
      { source: '/v2/stream/tickets', destination: `${gateway}/v2/stream/tickets` },
      // Control plane (media/gateway connected runtime, security/control): public GET
      // /v2/control/caps/{capability} read by security/control/cap-admission.ts on the
      // web origin before token creation, plus the authenticated POST operations.
      { source: '/v2/control/:path*', destination: `${gateway}/v2/control/:path*` },
      // core/api REST (API_ROLE=rest): runtime authority, preflight and topic snapshots.
      { source: '/v2/network/authority', destination: `${api}/v2/network/authority` },
      { source: '/v2/preflight/:path*', destination: `${api}/v2/preflight/:path*` },
      { source: '/v2/snapshots', destination: `${api}/v2/snapshots` },
      // core/api sockets (API_ROLE=sockets): the only role that registers GET /v2/stream and /v2/stream/sse.
      { source: '/v2/stream', destination: `${sockets}/v2/stream` },
      { source: '/v2/stream/sse', destination: `${sockets}/v2/stream/sse` },
      // core/trading-charts (no public charts domain): the versioned UDF documents
      // (RegisterProductionUDF). Live candles are core/api `/v2/stream` topics.
      { source: '/v2/udf/config', destination: `${charts}/v2/udf/config` },
      { source: '/v2/udf/symbols', destination: `${charts}/v2/udf/symbols` },
      { source: '/v2/udf/history', destination: `${charts}/v2/udf/history` },
      { source: '/v2/udf/time', destination: `${charts}/v2/udf/time` },
    ],
    afterFiles: [],
    fallback: [],
  };
}

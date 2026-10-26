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

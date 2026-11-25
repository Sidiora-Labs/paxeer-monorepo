export const origin = 'https://sidiora.test:4173';

export function applicationEnvironment() {
  return {
    ...process.env,
    NEXT_TELEMETRY_DISABLED: '1',
    NEXT_PUBLIC_COLOR_MODE_DEFAULT: 'device',
    NEXT_PUBLIC_CANONICAL_ORIGIN: origin,
    NEXT_PUBLIC_DATA_API_URL: origin,
    NEXT_PUBLIC_GATEWAY_URL: origin,
    NEXT_PUBLIC_METADATA_API_URL: origin,
    NEXT_PUBLIC_STATS_API_URL: origin,
    NEXT_PUBLIC_PNL_API_URL: origin,
    NEXT_PUBLIC_LIVESTREAM_API_URL: origin,
    NEXT_PUBLIC_STREAM_ORIGIN: origin.replace('https:', 'wss:'),
    NEXT_PUBLIC_RPC_URL: origin,
    NEXT_PUBLIC_EXPLORER_API: `${origin}/explorer/api/v2`,
    WEB_GATEWAY_UPSTREAM: origin,
    WEB_API_UPSTREAM: origin,
    WEB_SOCKETS_UPSTREAM: origin,
    WEB_CHARTS_UPSTREAM: origin,
  };
}

import type { readConfig } from './config';

export function contentSecurityPolicy(config: ReturnType<typeof readConfig>): string {
  const connectSources = new Set([ '\'self\'' ]);
  const endpoints = [
    config.paxeerRpcUrl,
    config.attestationApiUrl,
    ...Object.values(config.foreignRpcUrls),
  ];

  for (const endpoint of endpoints) {
    if (endpoint !== undefined) {
      connectSources.add(new URL(endpoint).origin);
    }
  }

  return [
    'default-src \'self\'',
    'script-src \'self\' \'unsafe-inline\'',
    'style-src \'self\' \'unsafe-inline\'',
    'img-src \'self\' data: blob:',
    `connect-src ${ Array.from(connectSources).join(' ') }`,
    'frame-ancestors \'none\'',
  ].join('; ');
}

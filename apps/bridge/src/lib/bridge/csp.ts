import type { readConfig } from './config';
import { buildBridgeChains, walletBridgeChains } from './chains';
import { effectiveForeignRpcUrl } from './rpc';

export type BridgeConfig = ReturnType<typeof readConfig>;

export function contentSecurityPolicy(config: BridgeConfig): string {
  const connectSources = new Set([ '\'self\'' ]);
  const endpoints = [
    config.paxeerRpcUrl,
    config.attestationApiUrl,
    ...walletBridgeChains(buildBridgeChains(config)).map((chain) => effectiveForeignRpcUrl(chain, config)),
    config.foreignRpcUrls.solana,
  ];

  for (const endpoint of endpoints) {
    if (endpoint !== undefined) {
      const url = new URL(endpoint);
      if ((url.protocol !== 'https:' && url.protocol !== 'http:') ||
        url.username || url.password || /[;\s']/.test(url.origin)) {
        throw new TypeError('CSP endpoints must be HTTP(S) URLs with an origin safe for a source list and no credentials');
      }
      connectSources.add(url.origin);
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

import type CspDev from 'csp-dev';

import config from 'configs/app';
import * as essentialDappsChains from 'configs/essential-dapps-chains/config.edge';

const feature = config.features.marketplace;

function analyticsOrigins(host: string | undefined): Array<string> {
  if (!host) {
    return [];
  }

  try {
    const url = new URL(host);
    if ((url.protocol !== 'https:' && url.protocol !== 'http:') || /[\s;,"'*<>]/.test(url.origin)) {
      return [];
    }

    let cloudRegion: 'us' | 'eu' | undefined;
    if (url.protocol === 'https:') {
      if (/^(?:app|us|us-assets)(?:\.i)?\.posthog\.com$/.test(url.hostname)) {
        cloudRegion = 'us';
      } else if (/^(?:eu|eu-assets)(?:\.i)?\.posthog\.com$/.test(url.hostname)) {
        cloudRegion = 'eu';
      }
    }

    return cloudRegion ? Array.from(new Set([
      url.origin,
      `https://${ cloudRegion }.i.posthog.com`,
      `https://${ cloudRegion }-assets.i.posthog.com`,
    ])) : [ url.origin ];
  } catch (error) {
    return [];
  }
}

export function marketplace(): CspDev.DirectiveDescriptor {
  if (!feature.isEnabled) {
    return {};
  }

  const chainsConfig = feature.essentialDapps && essentialDappsChains.getValue();
  const externalApiEndpoints = chainsConfig?.chains.map((chain) => chain.app_config?.apis?.general?.endpoint).filter(Boolean);
  const defaultRpcUrls = chainsConfig?.chains.map((chain) => chain.app_config?.chain?.rpcUrls).flat().filter(Boolean);

  const liFiHost = feature.essentialDapps?.swap ? 'li.quest' : '';
  const multisenderHost = feature.essentialDapps?.multisend ? '*.multisender.app' : '';
  const posthogHosts = analyticsOrigins(feature.essentialDapps?.multisend?.posthogHost);

  return {
    'connect-src': [
      'api' in feature ? feature.api.endpoint : '',
      ...(feature.essentialDapps ? [
        liFiHost,
        multisenderHost,
        ...posthogHosts,
        ...(externalApiEndpoints ?? []),
        ...(defaultRpcUrls ?? []),
      ] : []),
    ],

    'frame-src': [
      '*',
    ],

    'script-src': [
      ...posthogHosts,
    ],
  };
}

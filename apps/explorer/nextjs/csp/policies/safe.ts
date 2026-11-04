import type CspDev from 'csp-dev';

import config from 'configs/app';

export function safe(): CspDev.DirectiveDescriptor {
  const feature = config.features.safe;
  if (!feature.isEnabled) {
    return {};
  }

  return {
    'connect-src': [
      new URL(feature.apiUrl).origin,
    ],
  };
}

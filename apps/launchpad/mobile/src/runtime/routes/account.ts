import { type SdkClient } from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';

export function portfolio(sdk: SdkClient, signal: AbortSignal) {
  return sdk.request('/v1/me/portfolio?limit=50', 'PortfolioResponse', {
    signal,
    authenticated: true,
    idempotentRead: true,
  });
}

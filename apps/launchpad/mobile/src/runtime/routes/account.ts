import {
  Operations,
  type OperationResponse,
  type SdkClient,
} from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';

export const IF_PORTFOLIO = 'IF_PORTFOLIO';
export type PortfolioResponse = OperationResponse<typeof IF_PORTFOLIO>;

export function portfolio(sdk: SdkClient, signal: AbortSignal) {
  return new Operations(sdk).call(IF_PORTFOLIO, {}, { signal });
}

export const accountRoutes = { portfolio };

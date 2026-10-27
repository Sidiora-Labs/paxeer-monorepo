import {
  decodeContract,
  Operations,
  type SdkClient,
} from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import type { QuoteBody } from '../../../../../../sdk/launchpad/packages/sdk/src/types.gen.ts';

export function streams(sdk: SdkClient, signal: AbortSignal) {
  return new Operations(sdk).call(
    'IF_STREAM_LIST',
    { query: { status: 'live', limit: 50 } },
    { signal },
  );
}

export function quote(sdk: SdkClient, body: QuoteBody, signal: AbortSignal) {
  return sdk.request('/v1/quotes', 'QuoteResponse', {
    signal,
    method: 'POST',
    body: decodeContract('QuoteBody', body),
  });
}

export const IF_TX_STATUS = 'IF_TX_STATUS';

export function receipt(sdk: SdkClient, hash: string, signal: AbortSignal) {
  if (!/^0x[0-9a-fA-F]{64}$/.test(hash))
    throw new Error('Invalid transaction hash');
  return new Operations(sdk).call(
    IF_TX_STATUS,
    { path: { transactionHash: hash } },
    { signal },
  );
}

export const activityRoutes = { streams, quote, receipt };

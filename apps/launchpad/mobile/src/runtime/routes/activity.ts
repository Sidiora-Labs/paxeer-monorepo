import {
  decodeContract,
  type SdkClient,
} from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import type { QuoteBody } from '../../../../../../sdk/launchpad/packages/sdk/src/types.gen.ts';

export function streams(sdk: SdkClient, signal: AbortSignal) {
  return sdk.request('/v1/streams?status=live&limit=50', 'StreamListResponse', {
    signal,
    idempotentRead: true,
  });
}

export function quote(sdk: SdkClient, body: QuoteBody, signal: AbortSignal) {
  return sdk.request('/v1/quotes', 'QuoteResponse', {
    signal,
    method: 'POST',
    body: decodeContract('QuoteBody', body),
  });
}

export function receipt(sdk: SdkClient, hash: string, signal: AbortSignal) {
  if (!/^0x[0-9a-fA-F]{64}$/.test(hash))
    throw new Error('Invalid transaction hash');
  return sdk.request(`/v1/transactions/${hash}`, 'TxStatusResponse', {
    signal,
    idempotentRead: true,
  });
}

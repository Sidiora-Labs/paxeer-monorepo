import BigNumber from 'bignumber.js';

import type { TokenInfo } from 'types/api/token';
import type { AggregatedTokenInfo } from 'types/client/multichainAggregator';

export function getTokenChange(token: TokenInfo | AggregatedTokenInfo): BigNumber | undefined {
  if (!('price_change_percentage_24h' in token)) {
    return undefined;
  }

  const value = token.price_change_percentage_24h;
  if (typeof value !== 'number' && (typeof value !== 'string' || value.trim() === '')) {
    return undefined;
  }

  const change = new BigNumber(value);
  return change.isFinite() && Number.isFinite(change.toNumber()) ? change : undefined;
}

export function hasTokenChanges(tokens: Array<TokenInfo | AggregatedTokenInfo>): boolean {
  return tokens.some((token) => getTokenChange(token) !== undefined);
}

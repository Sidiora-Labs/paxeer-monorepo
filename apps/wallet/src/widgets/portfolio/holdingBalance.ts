import { formatBalance } from '@/lib/format';
import { readTokenPrecision } from '@/widgets/send/useSendableTokens';
import type { HoldingsGridHolding } from './HoldingsGrid';

export function holdingBalance(holding: HoldingsGridHolding): string {
  const decimals = readTokenPrecision(holding.decimals ?? null);
  if (decimals === null) return 'Precision unavailable';
  const raw = holding.balance_raw ?? '0';
  return decimals === 0 ? raw : formatBalance(raw, decimals, 4);
}

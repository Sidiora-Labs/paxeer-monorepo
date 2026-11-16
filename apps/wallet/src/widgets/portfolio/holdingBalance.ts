import { formatBalance } from '@/lib/format';
import { readTokenPrecision } from '@/widgets/send/useSendableTokens';
import type { HoldingsGridHolding } from './HoldingsGrid';

export function holdingBalance(holding: HoldingsGridHolding): string {
  const decimals = readTokenPrecision(holding.decimals ?? null);
  if (decimals === null) return 'Precision unavailable';
  const raw = holding.balance_raw ?? '0';
  if (decimals === 0) return raw;
  const formatted = formatBalance(raw, decimals, 4);
  return /^0*[1-9]\d*$/u.test(raw) && /^0\.0+$/u.test(formatted) ? '<0.0001' : formatted;
}

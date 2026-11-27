'use client';

import { cn } from '@/lib/utils';
import { MoneyValue, PercentPill } from '@/ui/primitives';
import { displayCoinText } from '@/utils/coin-text';
import { CoinIdentity } from '@/widgets/explore/coin-identity';

export interface SearchCoinRowProps {
  id: string;
  active: boolean;
  name: string;
  symbol: string;
  logo: string | null;
  mcapRaw: string | null;
  change24h?: number | null;
  address?: string;
  tokenAddress?: string | null;
  kind: 'pool' | 'token';
  disabled?: boolean;
  onSelect: () => void;
}

export function SearchCoinRow({ id, active, name, symbol, logo, mcapRaw, change24h = null, address, tokenAddress, disabled, onSelect }: SearchCoinRowProps) {
  name = displayCoinText(name, tokenAddress ?? address);
  symbol = displayCoinText(symbol, tokenAddress ?? address);
  return (
    <button id={id} type="button" role="option" aria-selected={active} tabIndex={-1}
      disabled={disabled} onClick={onSelect}
      className={cn('flex min-h-12 w-full items-center gap-3 border-b border-surface-3 px-3 py-2 text-left transition-colors duration-fast focus-visible:outline-2 focus-visible:outline-focus', active ? 'bg-surface-3' : 'hover:bg-surface-1', change24h !== null && change24h > 0 && 'sl-row-wash-up', change24h !== null && change24h < 0 && 'sl-row-wash-down', disabled && 'cursor-not-allowed')}>
      <CoinIdentity image={logo} name={name} symbol={symbol} />
      <span className="flex max-w-1/2 flex-col items-end">
        <span className="flex max-w-full flex-wrap items-center justify-end gap-x-2">
          <MoneyValue kind="market" raw={mcapRaw} compact size="sm" className="text-right font-bold tabular-nums" />
          {change24h !== null ? <PercentPill value={change24h} arrow /> : null}
        </span>
        {disabled ? <span className="text-2xs text-text-muted">Not trading yet</span> : null}
      </span>
    </button>
  );
}

export function SearchCoinWaiting() {
  return (
    <div aria-busy="true">
      <span className="sr-only">Searching</span>
      {Array.from({ length: 6 }, (_, index) => (
        <div key={index} aria-hidden="true" className="flex min-h-12 items-center gap-3 border-b border-surface-3 px-3 py-2">
          <span className="sl-skeleton size-avatar-md shrink-0 rounded-coin" />
          <span className="flex min-w-0 flex-1 flex-col gap-1.5">
            <span className="sl-skeleton h-3 w-2/3" />
            <span className="sl-skeleton h-2.5 w-1/3" />
          </span>
          <span className="sl-skeleton h-3 w-12" />
        </div>
      ))}
    </div>
  );
}

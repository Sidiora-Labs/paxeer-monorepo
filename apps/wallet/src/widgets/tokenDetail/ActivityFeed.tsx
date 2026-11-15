'use client';

/**
 * Token-scoped activity feed.
 *
 * Filters the wallet's full tx history down to rows that touch this token —
 * native transactions for PAX, token-transfer rows matching the contract
 * address otherwise. Renders skeleton placeholders while loading.
 */

import { formatBalance, shortenAddress } from '@/lib/format';
import { SvgIcon } from '@/components/ui/SvgIcon';
import { SectionLabel } from './Atoms';
import type { TokenDetailHistoryRow } from './detailQueries';

export interface ActivityFeedProps {
  rows: TokenDetailHistoryRow[];
  walletAddress: string;
  loading: boolean;
  unavailable?: boolean;
  onTxDetail?: (hash: string) => void;
}

export function ActivityFeed({
  rows,
  walletAddress,
  loading,
  unavailable,
  onTxDetail,
}: ActivityFeedProps) {
  const addr = walletAddress.toLowerCase();

  return (
    <>
      <div className="col-span-2 px-1 pt-1">
        <SectionLabel text="Activity" />
      </div>
      {unavailable ? (
        <p className="col-span-2 text-xs text-pax-muted">Activity unavailable for this network</p>
      ) : loading ? (
        <div className="col-span-2 space-y-2">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-14 shimmer rounded-xl" />
          ))}
        </div>
      ) : rows.length === 0 ? (
        <div className="col-span-2 bg-pax-surface rounded-xl py-10 text-center">
          <p className="text-xs text-pax-muted">No activity yet</p>
        </div>
      ) : (
        <div className="col-span-2 bg-pax-surface rounded-xl  divide-white/[0.04] overflow-hidden">
          {rows.map((row, i) => {
            const isSent = row.fromAddress.toLowerCase() === addr;
            return (
              <button
                type="button"
                key={row.hash + i}
                onClick={() => onTxDetail?.(row.hash)}
                className="w-full flex items-center gap-3 px-4 py-3 press-scale text-left transition-colors"
              >
                <div
                  className={`w-9 h-9 rounded-full flex items-center justify-center shrink-0 ${
                    isSent ? 'bg-pax-error/10' : 'bg-pax-success/10'
                  }`}
                >
                  <SvgIcon
                    name={isSent ? 'arrow-up-right' : 'arrow-down-left'}
                    className="w-4 h-4"
                  />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-semibold">{isSent ? 'Sent' : 'Received'}</p>
                  <p className="text-xs text-pax-muted truncate">
                    {isSent
                      ? `To ${shortenAddress(row.toAddress)}`
                      : `From ${shortenAddress(row.fromAddress)}`}
                  </p>
                </div>
                <p
                  className={`text-xs font-semibold shrink-0 ${
                    isSent ? 'text-pax-error' : 'text-pax-success'
                  }`}
                >
                  {isSent ? '-' : '+'}
                  {row.decimals === null
                    ? 'Unknown amount'
                    : formatBalance(row.amountRaw, row.decimals, 4)}{' '}
                  {row.symbol}
                </p>
              </button>
            );
          })}
        </div>
      )}
    </>
  );
}

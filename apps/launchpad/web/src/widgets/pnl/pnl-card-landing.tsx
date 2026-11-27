import { formatCompactUsd } from '@/utils/format';
import { cn } from '@/lib/utils';
import type { PublicShare } from './share-transport';

export const shareFieldLabel: Record<string, string> = {
  knownRealizedSubtotalUsdlRaw: 'Realized (known trades)',
  realizedPnlRaw: 'Realized PnL',
  completeValueUsdlRaw: 'Position value',
  historicalReturn: 'Return',
};

export function PublicShareFigures({ share }: { share: PublicShare }) {
  return (
    <section
      data-account-view="public-share"
      data-share-id={share.id}
      data-accounting-revision={share.watermark.projectionRevision}
      aria-label="Your PnL card"
      className="min-w-0 space-y-3 break-words rounded-lg bg-surface-1 p-4"
    >
      <p className="text-sm">{share.wallet ? `Wallet: ${share.wallet}` : 'Wallet hidden'}</p>
      <p className="text-xs tabular-nums text-text-muted">
        <time dateTime={share.rangeStart}>{share.rangeStart}</time> to{' '}
        <time dateTime={share.rangeEnd}>{share.rangeEnd}</time> UTC
      </p>
      <dl className="flex flex-col">
        {share.fields.map((field) => (
          <div key={field} className="sl-hairline flex min-w-0 flex-wrap items-baseline justify-between gap-x-3 gap-y-1 py-2">
            <dt className="text-sm font-medium text-text-muted">{shareFieldLabel[field] ?? field}</dt>
            <dd className={cn('min-w-0 break-all text-right text-base font-bold tabular-nums', field !== 'completeValueUsdlRaw' && field !== 'historicalReturn' && share.values[field] !== null && share.values[field] !== undefined ? BigInt(share.values[field]) > 0n ? 'text-percent-up' : BigInt(share.values[field]) < 0n ? 'text-percent-down' : 'text-text' : 'text-text')}>{field === 'historicalReturn' ? share.values.historicalReturn?.return ? `${share.values.historicalReturn.return.numerator}/${share.values.historicalReturn.return.denominator}` : 'Not available yet' : formatCompactUsd(share.values[field] ?? null, { raw: true, decimals: share.decimals, fractionDigits: share.decimals, group: false })}</dd>
          </div>
        ))}
      </dl>
      {(share.labels ?? []).map((label) => (
        <p key={label} className="text-xs text-text-muted">{label}</p>
      ))}
      <p className="text-xs tabular-nums text-text-muted">
        Expires <time dateTime={share.expiresAt}>{share.expiresAt}</time>. Copies others save
        can&apos;t be taken back.
      </p>
    </section>
  );
}

export default function PnlCardLanding({ share }: { share: PublicShare }) {
  return (
    <main className="mx-auto w-162.5 max-w-full space-y-4 p-6 text-text">
      <h1 className="text-md font-semibold">PnL card</h1>
      <PublicShareFigures share={share} />
    </main>
  );
}

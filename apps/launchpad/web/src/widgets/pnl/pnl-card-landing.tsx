import { exactAmount } from '../profile/account-disclosure';
import type { PublicShare } from './share-transport';

export const shareFieldLabel: Record<string, string> = {
  knownRealizedSubtotalUsdlRaw: 'Realized (known trades)',
  realizedPnlRaw: 'Realized PnL',
  completeValueUsdlRaw: 'Position value',
  historicalReturn: 'Return',
};

export function PublicShareFigures({ share }: { share: PublicShare }) {
  return (
    <section data-account-view="public-share" data-share-id={share.id} data-accounting-revision={share.watermark.projectionRevision} aria-label="Your PnL card" className="min-w-0 space-y-4 break-words">
      <p>{share.wallet ? `Wallet: ${share.wallet}` : 'Wallet hidden'}</p>
      <p className="text-sm text-text-muted">{share.rangeStart} to {share.rangeEnd} UTC</p>
      <dl className="space-y-3">
        {share.fields.map(field => (
          <div key={field} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] items-baseline gap-4">
            <dt className="text-text-muted">{shareFieldLabel[field] ?? field}</dt>
            <dd className="text-right tabular-nums">{field === 'historicalReturn' ? share.values.historicalReturn?.reason ?? share.values.historicalReturn?.method ?? '—' : exactAmount(share.values[field] ?? null, share.decimals, 'USDL')}</dd>
          </div>
        ))}
      </dl>
      {(share.labels ?? []).map(label => <p key={label}>{label}</p>)}
      <p className="text-sm text-text-muted">Expires {share.expiresAt}. Copies others save can&apos;t be taken back.</p>
    </section>
  );
}

export default function PnlCardLanding({ share }: { share: PublicShare }) {
  return <main className="mx-auto w-full max-w-2xl space-y-6 p-6 text-text"><h1 className="text-xl font-bold">PnL card</h1><PublicShareFigures share={share} /></main>;
}

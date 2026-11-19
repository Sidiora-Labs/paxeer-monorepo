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
    <section
      data-account-view="public-share"
      data-share-id={share.id}
      data-accounting-revision={share.watermark.projectionRevision}
      aria-label="Your PnL card"
      className="min-w-0 space-y-3 break-words"
    >
      <p>{share.wallet ? `Wallet: ${share.wallet}` : 'Wallet hidden'}</p>
      <p className="text-text-muted">
        <time dateTime={share.rangeStart}>{share.rangeStart}</time> to{' '}
        <time dateTime={share.rangeEnd}>{share.rangeEnd}</time> UTC
      </p>
      <dl>
        {share.fields.map((field) => (
          <div key={field}>
            <dt className="text-text-muted">{shareFieldLabel[field] ?? field}</dt>
            <dd className="tabular-nums">{field === 'historicalReturn' ? share.values.historicalReturn?.return ? `${share.values.historicalReturn.return.numerator}/${share.values.historicalReturn.return.denominator}` : 'Not available yet' : exactAmount(share.values[field] ?? null, share.decimals, 'USDL')}</dd>
          </div>
        ))}
      </dl>
      {(share.labels ?? []).map((label) => (
        <p key={label}>{label}</p>
      ))}
      <p className="text-text-muted">
        Expires <time dateTime={share.expiresAt}>{share.expiresAt}</time>. Copies others save
        can&apos;t be taken back.
      </p>
    </section>
  );
}

export default function PnlCardLanding({ share }: { share: PublicShare }) {
  return (
    <main className="mx-auto w-162.5 max-w-full space-y-4 p-6 text-text">
      <h1 className="text-md font-bold">PnL card</h1>
      <PublicShareFigures share={share} />
    </main>
  );
}

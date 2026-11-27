'use client';
import { formatCompactUsd } from '@/utils/format';

import { useId, useRef, useState } from 'react';
import { Calculator } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button, DisclosureRow, FormField, KeyValueRow, SegmentedControl, Skeleton, humanError } from '@/ui/primitives';
import { exactAmount } from './account-disclosure';
import { downloadExport, readExport, readInterval, readPortfolio, requestExport, type ExportJob, type TrackedPortfolio, type UtcInterval } from './account-transport';
import { MaskedAmount } from './balance-visibility';
import NetWorthChart from './net-worth-chart';
import { usePrivateAccountQuery } from './use-private-account-query';
import ShareCardButton from '../pnl/share-pnl-button';
import { actionFailureMessage } from '@/core/market/unsupported';

function initialInterval(): UtcInterval {
  const end = new Date();
  end.setUTCHours(0, 0, 0, 0);
  const start = new Date(end);
  start.setUTCDate(start.getUTCDate() - 7);
  return { from: start.toISOString(), to: end.toISOString(), interval: '1d' };
}

const granularity = [
  { value: '1d' as const, label: 'Daily' },
  { value: '1h' as const, label: 'Hourly' },
];

export default function PositionsTab({ walletAddress, canShare, defaultOpen = false }: { walletAddress: string; canShare: boolean; defaultOpen?: boolean }) {
  const id = useId();
  const [open, setOpen] = useState(defaultOpen);
  const exportMutation = useRef<{ snapshot: string; id: string } | null>(null);
  const [interval, setInterval] = useState<UtcInterval>(initialInterval);
  const [cursor, setCursor] = useState<string>();
  const [job, setJob] = useState<{ scope: string; job: ExportJob } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const portfolio = usePrivateAccountQuery('portfolio', () => readPortfolio(cursor), [cursor ?? ''], walletAddress);
  const history = usePrivateAccountQuery('history', () => readInterval(interval), [interval.from, interval.to, interval.interval], walletAddress);
  const scope = portfolio.session?.session.sessionId;
  const exportJob = job && job.scope === scope ? job.job : null;

  async function exportSelected() {
    if (!history.data || !scope) return;
    try {
      const snapshot = history.data.snapshotId;
      if (exportMutation.current?.snapshot !== snapshot) exportMutation.current = { snapshot, id: crypto.randomUUID() };
      setJob({ scope, job: await requestExport(snapshot, exportMutation.current.id) });
      setError(null);
    } catch (failure) {
      setError(actionFailureMessage('Portfolio export', failure));
    }
  }

  async function download() {
    if (!exportJob) return;
    try {
      const blob = await downloadExport(exportJob.id);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = 'portfolio.csv';
      anchor.click();
      URL.revokeObjectURL(url);
    } catch (failure) {
      setError(actionFailureMessage('Portfolio export', failure));
    }
  }

  async function refreshExport() {
    if (!exportJob || !scope) return;
    try {
      setJob({ scope, job: await readExport(exportJob.id) });
    } catch (failure) {
      setError(actionFailureMessage('Portfolio export', failure));
    }
  }

  if (!portfolio.owner) return <p className="px-1 py-2 text-sm text-text-muted">Sign in with this wallet to see your PnL details.</p>;
  const value = portfolio.data;
  const summary = value?.data.summary;

  return (
    <DisclosureRow id={`${id}-accounting`} label="PnL details" icon={Calculator} open={open} onToggle={setOpen}>
      <section
        data-account-view="portfolio"
        data-account-wallet={walletAddress.toLowerCase()}
        data-snapshot-id={summary?.snapshotId}
        data-accounting-revision={summary?.watermark.projectionRevision}
        className="flex flex-col gap-4"
      >
        <div className="flex flex-col gap-2">
          <h3 className="text-sm font-bold text-text">Your trades</h3>
          <p className="text-xs text-text-muted">Market values are in dollars. Network fees are paid in PAX.</p>
          {portfolio.isPending ? <Skeleton variant="rows" rows={3} label="Loading PnL" /> : null}
          {portfolio.error ? <><p role="alert" className="text-sm text-danger">{humanError(portfolio.error)}</p><Button variant="secondary" size="sm" onClick={() => { void portfolio.refetch(); }}>Retry positions</Button></> : null}
          {value && summary ? (
            <dl className="flex flex-col gap-0.5">
              <KeyValueRow label="Method" value={`${value.algorithm} · history ${summary.basisQuality} · prices ${summary.pricingQuality}`} />
              <KeyValueRow label="Total value" value={<MaskedAmount>{formatCompactUsd(summary.completeValueUsdlRaw, { raw: true, fractionDigits: 6, group: false })}</MaskedAmount>} />
              <KeyValueRow label="Priced value" value={<MaskedAmount>{formatCompactUsd(summary.pricedSubtotalUsdlRaw, { raw: true, fractionDigits: 6, group: false })}</MaskedAmount>} />
              <KeyValueRow label="Realized" value={<MaskedAmount>{formatCompactUsd(summary.realizedPnlRaw, { raw: true, fractionDigits: 6, group: false })}</MaskedAmount>} />
              <KeyValueRow label="Known realized" value={<MaskedAmount>{formatCompactUsd(summary.knownRealizedSubtotalUsdlRaw, { raw: true, fractionDigits: 6, group: false })}</MaskedAmount>} />
              <KeyValueRow label="Missing prices" value={`${summary.unpricedAssetCount} ${summary.unpricedAssetCount === 1 ? 'coin' : 'coins'}`} />
              <KeyValueRow label="Last updated" value={summary.watermark.observedAt} />
              <KeyValueRow label="Refreshed" value={`${Math.max(0, Math.floor((Date.now() - portfolio.dataUpdatedAt) / 1000))}s ago${portfolio.isRefetchError ? ", couldn't refresh" : ''}`} />
            </dl>
          ) : null}
        </div>

        {value?.data.positions.length ? (
          <ul aria-label="PnL by coin" className="flex flex-col gap-1">
            {value.data.positions.map(row => <li key={row.positionId}><PositionFigures row={row} errors={value.positionErrors[row.positionId]} /></li>)}
          </ul>
        ) : value ? (
          <p className="rounded-md bg-surface-3 p-3 text-sm text-text-faint">No trades yet. Your PnL by coin shows up here after your first trade.</p>
        ) : null}
        {value?.data.next || cursor ? (
          <div className="flex flex-wrap gap-2">
            {value?.data.next ? <Button variant="secondary" size="sm" onClick={() => setCursor(value.data.next)}>Next page</Button> : null}
            {cursor ? <Button variant="ghost" size="sm" onClick={() => setCursor(undefined)}>First page</Button> : null}
          </div>
        ) : null}

        <section aria-labelledby={`${id}-utc`} className="flex flex-col gap-3 rounded-md bg-surface-3 p-3">
          <h3 id={`${id}-utc`} className="text-sm font-bold text-text">Date range (UTC)</h3>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <FormField id={`${id}-from`} label="From (UTC)" surface="surface-2" inputProps={{ type: 'date', value: interval.from.slice(0, 10), onChange: event => setInterval({ ...interval, from: `${event.target.value}T00:00:00.000Z` }) }} />
            <FormField id={`${id}-to`} label="To (UTC)" surface="surface-2" inputProps={{ type: 'date', value: interval.to.slice(0, 10), onChange: event => setInterval({ ...interval, to: `${event.target.value}T00:00:00.000Z` }) }} />
          </div>
          <SegmentedControl label="Group by" options={granularity} value={interval.interval} onChange={next => setInterval({ ...interval, interval: next })} />
          {history.isPending ? <Skeleton variant="rows" rows={3} label="PnL history" /> : null}
          {history.error ? <><p role="alert" className="text-sm text-danger">{humanError(history.error)}</p><Button variant="secondary" size="sm" onClick={() => { void history.refetch(); }}>Retry history</Button></> : null}
          {history.data ? <NetWorthChart series={history.data} /> : null}
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="secondary" size="sm" disabled={!history.data} onClick={() => void exportSelected()}>Export CSV</Button>
            {exportJob ? (
              <ExportStatus job={exportJob} onRefresh={refreshExport} onDownload={download} />
            ) : null}
            {canShare && value && summary ? <ShareCardButton ownerAddress={walletAddress} poolAddress={value.data.positions[0]?.tokenAddress ?? walletAddress} snapshotId={summary.snapshotId} /> : null}
          </div>
          {error ? <p role="alert" className="text-sm text-danger">{error}</p> : null}
        </section>
      </section>
    </DisclosureRow>
  );
}

export function ExportStatus({ job, onRefresh, onDownload }: { job: ExportJob; onRefresh: () => Promise<void>; onDownload: () => Promise<void> }) {
  return (
    <>
      <span className="text-xs text-text-muted" role="status">Export: {job.state}</span>
      <Button variant="ghost" size="sm" onClick={() => void onRefresh()}>Refresh</Button>
      {job.state === 'complete' ? <Button variant="primary" size="sm" onClick={() => void onDownload()}>Download CSV</Button> : null}
    </>
  );
}

function PositionFigures({ row, errors, className }: { row: TrackedPortfolio['data']['positions'][number]; errors?: readonly string[] | null; className?: string }) {
  const decimals = row.markPrice?.baseAsset.decimals;
  return (
    <article
      data-position-id={row.positionId}
      data-token-address={row.tokenAddress.toLowerCase()}
      data-remaining-basis-raw={row.remainingCostUsdlRaw ?? 'unavailable'}
      className={cn('flex flex-col gap-1 rounded-md bg-surface-3 p-3 text-xs text-text-muted', className)}
    >
      <h4 className="break-all font-mono-sl text-xs font-bold text-text">{row.tokenAddress}</h4>
      <p>Holding: <MaskedAmount>{decimals !== null && decimals !== undefined && decimals <= 36 ? exactAmount(row.balanceRaw, decimals, 'tokens') : row.balanceRaw}</MaskedAmount></p>
      <p>Known cost: <MaskedAmount>{formatCompactUsd(row.knownRemainingCostUsdlRaw, { raw: true, fractionDigits: 6, group: false })}</MaskedAmount>, total cost: <MaskedAmount>{formatCompactUsd(row.remainingCostUsdlRaw, { raw: true, fractionDigits: 6, group: false })}</MaskedAmount></p>
      <p>Realized: <MaskedAmount>{formatCompactUsd(row.realizedPnlRaw, { raw: true, fractionDigits: 6, group: false })}</MaskedAmount>, unrealized: <MaskedAmount>{formatCompactUsd(row.unrealizedPnlRaw, { raw: true, fractionDigits: 6, group: false })}</MaskedAmount></p>
      <p>History {row.basisQuality} · prices {row.pricingQuality} · price time {row.markPrice?.source.observedAt ?? 'not available yet'}</p>
      {errors?.length ? <p role="alert" className="text-danger">Some figures for this coin couldn't load</p> : null}
    </article>
  );
}

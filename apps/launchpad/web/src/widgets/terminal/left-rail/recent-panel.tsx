'use client';
import { useEffect, useId, useMemo, useState } from 'react';
import Link from 'next/link';
import { ChevronLeft, ChevronRight, MoreHorizontal, Trash2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { technicalDetail } from '@/core/user-facing-error';
import { Avatar, DropdownMenu, EmptyState, IconButton, Skeleton } from '@/ui/primitives';
import { formatCompactUsd } from '@/utils/format';
import { StatDash } from '@/ui/shared/stat-value';
import { clearRecentPools, recentPoolsLimit, removeRecentPool, useRecentPools } from '../recent-pools';
import { formatRailPrice, type RecentRow } from './rows';
import { useRecentRows } from './use-recent-rows';

export const recentPageSize = 5;

export function pageCount(total: number, size = recentPageSize): number {
  return Math.max(1, Math.ceil(total / size));
}

function RecentPairRow({ row, active, loadingPrice }: { row: RecentRow; active: boolean; loadingPrice: boolean }) {
  return (
    <li data-pool={row.poolAddress} className={cn('relative grid min-h-control-height-sm grid-cols-[var(--sl-recent-columns)] items-center gap-1 px-2 transition-colors duration-fast', active ? 'bg-surface-3' : 'bg-surface-1 hover:bg-surface-hover')}>
      <div className="col-span-2 flex min-w-0 items-center gap-1.5">
        <Avatar src={row.image} name={row.name} size="xs" decorative />
        <Link prefetch={false}
          href={row.href}
          aria-current={active ? 'page' : undefined}
          title={row.name}
          className="min-w-0 flex-1 truncate text-xs font-bold text-text after:absolute after:inset-0 after:content-[''] focus-visible:outline-2 focus-visible:outline-focus"
        >
          {row.symbol}
          <span className="sr-only">{`, ${row.name}`}</span>
        </Link>
      </div>
      {row.price === null && loadingPrice
        ? <span className="col-start-1 row-start-2 flex justify-end"><Skeleton label="price" className="w-8" /></span>
        : row.price === null
        ? <span className="col-start-1 row-start-2 truncate text-right"><StatDash reason={row.priceReason ?? 'The price is not available for this coin yet.'} subject="Price" /></span>
        : (
          <span data-numeric className="col-start-1 row-start-2 truncate text-right text-xs tabular-nums text-text">
            <span className="sr-only">Price </span>{formatRailPrice(row.price)}
          </span>
        )}
      <span data-numeric className={cn('col-start-2 row-start-2 truncate text-right text-xs tabular-nums', row.mcap === null ? 'text-text-faint' : 'text-text')}>
        <span className="sr-only">Market cap </span>{formatCompactUsd(row.mcap)}
      </span>
      <IconButton label={`Remove ${row.symbol} from recently viewed`} icon={<Trash2 />} size="sm" className="relative z-10 col-start-3 row-start-1" onClick={() => removeRecentPool(row.poolAddress)} />
    </li>
  );
}

export interface RecentPanelProps {
  activePool: string | null;
  className?: string;
}

export function RecentPanel({ activePool, className }: RecentPanelProps) {
  const headingId = useId();
  const storedPools = useRecentPools();
  const active = activePool?.toLowerCase() ?? null;
  const [publishedActive, setPublishedActive] = useState<string | null>(null);
  const pools = useMemo(() => active && publishedActive !== active ? [active, ...storedPools.filter(pool => pool !== active)].slice(0, recentPoolsLimit) : storedPools, [active, publishedActive, storedPools]);
  useEffect(() => { setPublishedActive(active); }, [active]);
  const { rows, prices, enrichmentPending } = useRecentRows(pools);
  const [page, setPage] = useState(0);
  const pages = pageCount(rows.length);

  useEffect(() => {
    if (page > pages - 1) setPage(pages - 1);
  }, [page, pages]);

  const current = Math.min(page, pages - 1);
  const shown = rows.slice(current * recentPageSize, current * recentPageSize + recentPageSize);

  return (
    <section aria-labelledby={headingId} className={cn('flex flex-col bg-surface-1', className)}>
      <header className="flex min-h-control-height-sm shrink-0 flex-wrap items-center gap-1 px-2">
        <h2 id={headingId} className="mr-auto truncate text-sm font-bold text-text">Recently viewed</h2>
        {pages > 1
          ? (
            <div className="flex items-center gap-0.5">
              <IconButton label="Previous recently viewed page" icon={<ChevronLeft />} size="sm" disabled={current === 0} onClick={() => setPage(current - 1)} />
              <span className="text-2xs tabular-nums text-text-faint" aria-live="polite">{`${current + 1}/${pages}`}</span>
              <IconButton label="Next recently viewed page" icon={<ChevronRight />} size="sm" disabled={current >= pages - 1} onClick={() => setPage(current + 1)} />
            </div>
          )
          : null}
        <DropdownMenu
          label="Recently viewed actions"
          align="end"
          size="sm"
          trigger={<MoreHorizontal aria-hidden="true" className="size-icon-sm" />}
          triggerClassName="w-control-height-sm px-0"
          items={[{ id: 'clear', label: 'Clear all', icon: <Trash2 aria-hidden="true" className="size-icon-sm" />, destructive: true, disabled: pools.length === 0, onSelect: clearRecentPools }]}
        />
      </header>
      {pools.length === 0
        ? <EmptyState variant="inline" title="Nothing viewed yet" body="Coins you open show up here." className="px-2 pb-2 text-xs" />
        : (
          <>
            <div aria-hidden="true" className="sr-only">
              <span>Coin</span>
              <span className="text-right">Price</span>
              <span className="text-right">MCap</span>
              <span />
            </div>
            <ul aria-labelledby={headingId}>
              {shown.map(row => <RecentPairRow key={row.poolAddress} row={row} active={row.poolAddress === active} loadingPrice={enrichmentPending} />)}
            </ul>
            {prices.state === 'unsupported'
              ? <p role="status" title={technicalDetail(prices.operation, prices.status) ?? undefined} data-operation={technicalDetail(prices.operation, prices.status) ?? undefined} className="px-2 py-1 text-2xs text-text-faint">Prices are not available yet</p>
              : prices.state === 'error'
                ? <p role="alert" className="px-2 py-1 text-2xs text-danger">Prices can&apos;t load right now</p>
                : null}
          </>
        )}
    </section>
  );
}

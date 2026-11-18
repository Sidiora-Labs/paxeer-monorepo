'use client';

import Link from 'next/link';
import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/core/query-keys';
import { useOperations } from '@/core/market/operations';
import { fetchPlatformMetricsV2, type PlatformMetricsV2 } from '@/core/market/platform-metrics';
import { toUnsupported } from '@/core/market/unsupported';
import { isTransientError, retryTransientOnly } from '@/core/transient-retry';
import { technicalDetail } from '@/core/user-facing-error';
import { useRefetchOnAnyEvent } from '@/hooks/market/use-stream-refetch';
import { UnavailableState } from '@/ui/primitives';
import { formatCount, formatUsdl } from '@/utils/format';

const OPERATION = 'IF_PLATFORM';

/** One footer figure; `value` null renders the unavailable treatment, with `operation` naming the v2 field the envelope lacked. */
export interface FooterMetric {
  label: string;
  value: string | null;
  operation: string | null;
}

const pending = (field: string) => `${OPERATION} (${field})`;

/**
 * The six footer figures from IF_PLATFORM (interface 2.2.0): volume, total
 * market cap (totalMarketCapUsdlRaw, the sum of pool total-supply
 * valuations), trades, traders (activeWallets), coins launched (pools) and
 * new today (newPools). A field an older producer does not send stays
 * unavailable, never a guessed number.
 */
export function platformFooterItems(data: PlatformMetricsV2): FooterMetric[] {
  const optional = (
    value: string | null | undefined,
    field: string,
    format: (raw: string) => string
  ): Pick<FooterMetric, 'value' | 'operation'> =>
    value === undefined || value === null
      ? { value: null, operation: pending(field) }
      : { value: format(value), operation: null };
  const usdl = (raw: string) => formatUsdl(raw, { compact: true });
  return [
    { label: '24h volume', value: usdl(data.volumeUsdlRaw), operation: null },
    { label: 'Market cap', ...optional(data.totalMarketCapUsdlRaw, 'totalMarketCapUsdlRaw', usdl) },
    { label: '24h trades', ...optional(data.trades, 'trades', formatCount) },
    { label: '24h traders', value: formatCount(data.activeWallets), operation: null },
    { label: 'Coins launched', value: formatCount(data.pools), operation: null },
    { label: 'New today', ...optional(data.newPools, 'newPools', formatCount) },
  ];
}

/**
 * Platform activity figures use the same labelled grid while loading and ready,
 * or one quiet status line when unavailable.
 * A figure the envelope does not carry shows a faint em dash that says "not
 * served" to assistive tech, with the operation in a developer-only attribute
 * (the SourceNotice / rail convention).
 */
export default function PlatformMetricsFooter() {
  const operations = useOperations();
  const query = useQuery({
    queryKey: queryKeys.platformStats(),
    queryFn: ({ signal }) => fetchPlatformMetricsV2(operations, signal),
    staleTime: 30_000,
    ...retryTransientOnly,
  });
  useRefetchOnAnyEvent({ queryKeys: [queryKeys.platformStats()], throttleMs: 30_000 });

  // A refetch whose transient retries ran out keeps the last-good figures.
  const data = query.data;
  let failure: ReactNode = null;
  if (query.error && !(data && isTransientError(query.error))) {
    const marker = toUnsupported(OPERATION, query.error);
    if (marker)
      failure = (
        <UnavailableState
          variant="inline"
          title="Platform activity"
          operation={marker.operation}
          status={marker.status}
          className="text-2xs"
        />
      );
    else failure = (
      <p role="status" className="truncate text-2xs text-text-faint">
        Couldn't load stats.{' '}
        <button
          type="button"
          onClick={() => {
            void query.refetch();
          }}
          className="font-bold text-text-muted hover:text-text focus-visible:-outline-offset-2 focus-visible:underline"
        >
          Try again
        </button>
      </p>
    );
  }

  if (!data && !query.isPending && !failure) return null;
  const items: FooterMetric[] = data ? platformFooterItems(data) : [
    '24h volume', 'Market cap', '24h trades', '24h traders', 'Coins launched', 'New today',
  ].map(label => ({ label, value: null, operation: null }));
  return (
    <div className="grid min-w-0">
      <dl aria-label="Platform activity" aria-busy={query.isPending || undefined} aria-hidden={failure ? true : undefined} inert={failure ? true : undefined} className={`col-start-1 row-start-1 grid min-w-0 grid-cols-2 gap-x-4 gap-y-2 md:grid-cols-3 xl:grid-cols-6${failure ? ' invisible' : ''}`}>
        {items.map((item) => {
          const detail = item.value === null ? technicalDetail(item.operation, null) : null;
          return (
            <div key={item.label} className="min-w-0 text-2xs">
              <dt className="text-text-faint">{item.label}</dt>
              {query.isPending ? (
                <dd className="min-h-[2lh]"><span className="sr-only">Loading platform activity</span></dd>
              ) : item.value === null ? (
                <dd
                  data-unavailable=""
                  title={detail ?? undefined}
                  data-operation={detail ?? undefined}
                  className="min-h-[2lh] text-right text-text-faint"
                >
                  <span aria-hidden="true">—</span>
                  <span className="sr-only">not available yet</span>
                </dd>
              ) : (
                <dd data-numeric className="min-h-[2lh] text-right font-bold tabular-nums text-text-muted">
                  {item.value}
                </dd>
              )}
            </div>
          );
        })}
      </dl>
      {failure ? <div className="col-start-1 row-start-1 min-w-0 self-start">{failure}</div> : null}
    </div>
  );
}

const LEGAL_LINKS = [
  { label: 'Terms', href: '/docs/terms' },
  { label: 'Privacy', href: '/docs/privacy' },
  { label: 'Trademarks', href: '/docs/trademark-guidelines' },
  { label: 'DMCA', href: '/docs/dmca' },
  { label: 'Do not sell my data', href: '/docs/do-not-sell' },
];

/**
 * Copyright and legal links share a canvas row above the activity grid.
 * Page content fills a viewport before the footer on short pages.
 */
export function ShellFooter() {
  return (
    <footer className="flex flex-wrap items-center gap-x-4 gap-y-1 bg-canvas px-3 py-3 text-2xs text-text-faint sm:px-4 lg:px-6">
      <p suppressHydrationWarning>© {new Date().getFullYear()} Sidiora</p>
      <nav aria-label="Legal" className="flex flex-wrap items-center gap-x-3 gap-y-1">
        {LEGAL_LINKS.map(({ label, href }) => (
          <Link prefetch={false}
            key={href}
            href={href}
            className="inline-flex min-h-touch-target-min items-center transition-colors duration-fast hover:text-text"
          >
            {label}
          </Link>
        ))}
      </nav>
      <div className="min-h-[calc(6lh+var(--sl-space-4))] min-w-0 basis-full md:min-h-[calc(4lh+var(--sl-space-2))] xl:min-h-[2lh]">
        <PlatformMetricsFooter />
      </div>
    </footer>
  );
}

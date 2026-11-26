import type { ReactNode } from 'react';
import { ArrowDown } from 'lucide-react';

export function RankingTable({ label, children, state }: { label: string; children?: ReactNode; state?: ReactNode }) {
  return (
    <div className="relative min-w-0 overflow-x-auto">
      <table className="w-full min-w-[var(--sl-rewards-table-min-width)] table-fixed text-sm">
        <caption className="sr-only">{label}</caption>
        <thead className="bg-canvas text-2xs font-semibold uppercase tracking-[var(--sl-tracking-label)] text-text-muted">
          <tr>
            <th scope="col" className="h-table-header-height w-8 text-left" aria-label="Rank">#</th>
            <th scope="col" className="h-table-header-height text-left">Trader</th>
            <th scope="col" aria-sort="descending" className="h-table-header-height w-24 px-2 text-right sm:w-36"><span className="inline-flex items-center gap-1">Profit<ArrowDown aria-hidden="true" className="size-3" /></span></th>
            <th scope="col" className="w-touch-target-min"><span className="sr-only">Follow</span></th>
          </tr>
        </thead>
        <tbody>{state ? <tr><td colSpan={4}>{state}</td></tr> : children}</tbody>
      </table>
    </div>
  );
}

import { UnavailableState } from './states';

/** A paired bar reserves its labels and track while its values are awaited. */
export function PairedBarPlaceholder({ leftLabel, rightLabel, title, operation, loading = false }: { leftLabel: string; rightLabel: string; title: string; operation: string; loading?: boolean }) {
  return (
    <div className="flex flex-col gap-1" data-split-frame={leftLabel}>
      <div aria-hidden="true" className="flex items-center justify-between text-xs">
        <span className="text-text-faint">{loading ? <span className="sl-skeleton inline-block h-3 w-8 align-middle" /> : <span className="font-bold text-text-faint">—</span>} {leftLabel}</span>
        <span className="text-text-faint">{loading ? <span className="sl-skeleton inline-block h-3 w-8 align-middle" /> : <span className="font-bold text-text-faint">—</span>} {rightLabel}</span>
      </div>
      <div aria-hidden="true" className="h-1 w-full rounded-pill bg-track" />
      {loading ? <span className="sr-only">Loading {title.toLowerCase()}</span> : <UnavailableState variant="inline" title={title} plural="auto" operation={operation} status={null} className="sr-only" />}
    </div>
  );
}


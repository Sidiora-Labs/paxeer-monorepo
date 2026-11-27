'use client';
import { useEffect, useId, useState } from 'react';
import { Button, FormField } from '@/ui/primitives';
import { activeFilterCount, emptyFilters, parseFilterNumber, type ExploreFilters } from './sort';

type FilterField = keyof ExploreFilters;

const fields: ReadonlyArray<{ id: FilterField; label: string; helper: string; unit: string }> = Object.freeze([
  { id: 'maxAgeHours', label: 'Max age', helper: 'Hours since launch', unit: 'h' },
  { id: 'minMcap', label: 'Min market cap', helper: 'Dollars', unit: '$' },
  { id: 'maxMcap', label: 'Max market cap', helper: 'Dollars', unit: '$' },
  { id: 'minVolume24h', label: 'Min 24h volume', helper: 'Dollars traded in the last 24 hours', unit: '$' },
  { id: 'minHolders', label: 'Min holders', helper: 'Number of holders', unit: 'holders' },
]);

function textOf(filters: ExploreFilters): Record<FilterField, string> {
  return {
    maxAgeHours: filters.maxAgeHours === null ? '' : String(filters.maxAgeHours),
    minMcap: filters.minMcap === null ? '' : String(filters.minMcap),
    maxMcap: filters.maxMcap === null ? '' : String(filters.maxMcap),
    minVolume24h: filters.minVolume24h === null ? '' : String(filters.minVolume24h),
    minHolders: filters.minHolders === null ? '' : String(filters.minHolders),
  };
}

export interface FilterPanelProps {
  id: string;
  filters: ExploreFilters;
  onChange: (filters: ExploreFilters) => void;
  /** Rows loaded from the backend and rows left after filtering, for the honest "within loaded rows" count. */
  loadedCount: number;
  matchCount: number;
  onClose: () => void;
}

export function FilterPanel({ id, filters, onChange, loadedCount, matchCount, onClose }: FilterPanelProps) {
  const headingId = useId();
  const [text, setText] = useState(() => textOf(filters));
  useEffect(() => {
    if (activeFilterCount(filters) === 0) setText(textOf(emptyFilters));
  }, [filters]);

  function update(field: FilterField, value: string) {
    setText(current => ({ ...current, [field]: value }));
    onChange({ ...filters, [field]: parseFilterNumber(value) });
  }

  const active = activeFilterCount(filters);
  return (
    <section id={id} aria-labelledby={headingId} className="flex flex-col gap-4 rounded-md bg-surface-2 p-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1 basis-40">
          <h2 id={headingId} className="text-md font-semibold text-text">Filter</h2>
          <p role="status" className="text-xs text-text-faint">
            {active === 0
              ? `Filters apply to the ${loadedCount} coins shown`
              : `${matchCount} of ${loadedCount} coins match`}
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={onClose}>Done</Button>
      </div>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,var(--sl-form-field-min-width)),1fr))] gap-3">
        {fields.map(field => (
          <FormField
            key={field.id}
            id={`${id}-${field.id}`}
            label={field.label}
            helper={field.helper}
            surface="surface-2"
            inputProps={{
              inputMode: 'decimal',
              autoComplete: 'off',
              placeholder: 'Any',
              value: text[field.id],
              onChange: event => update(field.id, event.target.value),
            }}
            trailing={<span className="text-xs text-text-faint">{field.unit}</span>}
          />
        ))}
      </div>
      <div className="flex items-center justify-end">
        <Button variant="secondary" size="sm" disabled={active === 0} onClick={() => { setText(textOf(emptyFilters)); onChange({ ...emptyFilters }); }}>Clear all</Button>
      </div>
    </section>
  );
}

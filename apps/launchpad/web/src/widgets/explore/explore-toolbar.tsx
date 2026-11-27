'use client';
import type { ReactNode } from 'react';
import { LayoutGrid, ListFilter, Rows3, Settings2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { ViewMode } from './column-settings';

export interface ExploreToolbarProps {
  /** Left slot: preset tabs on /explore, a heading or count on /watchlist. */
  leading?: ReactNode;
  filterOpen: boolean;
  filterCount: number;
  filterPanelId: string;
  onToggleFilter: () => void;
  view: ViewMode;
  onViewChange: (view: ViewMode) => void;
  settingsOpen: boolean;
  settingsPanelId: string;
  onToggleSettings: () => void;
}

const toolClass = 'relative inline-flex border size-control-height-md shrink-0 items-center justify-center rounded-sm text-text-muted transition-colors duration-fast hover:bg-surface-hover hover:text-text focus-visible:outline-2 focus-visible:outline-focus [&_svg]:size-icon-md';
const toolPressed = 'border-control-edge bg-surface-3 text-text';
const toolIdle = 'border-transparent hover:border-control-edge';

/** Presets scroll in one row beside the quiet filter, layout and column controls. */
export function ExploreToolbar({ leading, filterOpen, filterCount, filterPanelId, onToggleFilter, view, onViewChange, settingsOpen, settingsPanelId, onToggleSettings }: ExploreToolbarProps) {
  return (
    <div className="flex min-w-0 flex-nowrap items-center gap-2">
      <div className="min-w-0 flex-1">{leading}</div>
      <div className="flex shrink-0 items-center gap-2">
        <button
          type="button"
          aria-label={filterCount > 0 ? `Filter, ${filterCount} active` : 'Filter'}
          title="Filter"
          aria-expanded={filterOpen}
          aria-controls={filterPanelId}
          aria-pressed={filterCount > 0 || undefined}
          onClick={onToggleFilter}
          className={cn(toolClass, filterOpen || filterCount > 0 ? toolPressed : toolIdle)}
        >
          <ListFilter aria-hidden="true" />
          {filterCount > 0 ? (
            <span aria-hidden="true" data-numeric className="absolute -right-1 -top-1 inline-flex h-4 min-w-4 items-center justify-center rounded-pill bg-accent px-1 text-2xs font-bold tabular-nums text-on-accent">{filterCount}</span>
          ) : null}
        </button>
        <div role="radiogroup" aria-label="Layout" className="hidden items-center gap-0.5 md:inline-flex">
          {([{ value: 'grid', label: 'Grid', Icon: LayoutGrid }, { value: 'table', label: 'Table', Icon: Rows3 }] as const).map(({ value, label, Icon }) => (
            <button key={value} type="button" role="radio" aria-label={label} title={label}
              aria-checked={view === value} tabIndex={view === value ? 0 : -1}
              onClick={() => onViewChange(value)}
              onKeyDown={event => {
                if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) {
                  event.preventDefault();
                  const next = event.key === 'Home' ? 'grid' : event.key === 'End' ? 'table' : view === 'grid' ? 'table' : 'grid';
                  onViewChange(next);
                  event.currentTarget.parentElement?.querySelector<HTMLButtonElement>(`[aria-label="${next === 'grid' ? 'Grid' : 'Table'}"]`)?.focus();
                }
              }}
              className={cn(toolClass, view === value ? toolPressed : 'border-transparent')}>
              <Icon aria-hidden="true" />
            </button>
          ))}
        </div>
        <button
          type="button"
          aria-label="Column settings"
          title="Column settings"
          aria-pressed={settingsOpen}
          aria-expanded={settingsOpen}
          aria-controls={settingsPanelId}
          onClick={onToggleSettings}
          className={cn(toolClass, settingsOpen ? toolPressed : toolIdle)}
        >
          <Settings2 aria-hidden="true" />
        </button>
      </div>
    </div>
  );
}

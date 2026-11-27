'use client';
import './coin-market.css';
import { Info, SlidersHorizontal } from 'lucide-react';
import { AvatarStack, Button, DropdownMenu, humanErrorText, SegmentedControl, Tooltip } from '@/ui/primitives';
import type { ChartFreshness, ChartMode } from '@/widgets/trading/udf-datafeed';
import { cn } from '@/lib/utils';
import { CHART_INTERVALS, type ChartInterval } from './use-chart-settings';

export interface ChartMarkerActor { wallet: string; name: string; avatarUrl: string | null }

export interface ChartToolbarProps {
  interval: ChartInterval;
  onIntervalChange: (interval: ChartInterval) => void;
  mode: ChartMode;
  onModeChange: (mode: ChartMode) => void;
  /** Set when market cap cannot be charted; the toggle is disabled and the reason is shown. */
  mcapDisabledReason: string | null;
  markers: boolean;
  onMarkersChange: (enabled: boolean) => void;
  markerActors: readonly ChartMarkerActor[];
  freshness: ChartFreshness;
  noTrades?: boolean;
  waitingForFirstTrade?: boolean;
  onRetry: () => void;
  className?: string;
}

export function freshnessLabel(status: ChartFreshness['connectionStatus']): string {
  switch (status) {
    case 'live': return 'Live';
    case 'offline': return 'Updates paused';
    case 'connecting':
    case 'snapshotting':
    case 'replaying': return 'Syncing...';
    case 'closed': return 'Chart closed';
    default: return 'May be out of date';
  }
}

export function ChartToolbar({ interval, onIntervalChange, mode, onModeChange, mcapDisabledReason, markers, onMarkersChange, markerActors, freshness, noTrades, waitingForFirstTrade, onRetry, className }: ChartToolbarProps) {
  const primary = CHART_INTERVALS.filter(option => option.primary);
  const secondary = CHART_INTERVALS.filter(option => !option.primary);
  const activeSecondary = secondary.find(option => option.value === interval);
  const segments = [...primary, ...(activeSecondary ? [activeSecondary] : [])].map(option => ({ value: option.value, label: option.label, ariaLabel: option.description }));
  const mcapDisabled = mcapDisabledReason !== null;
  const live = !waitingForFirstTrade && freshness.connectionStatus === 'live';
  const waitingForTrades = waitingForFirstTrade || (noTrades && (freshness.connectionStatus === 'connecting' || freshness.connectionStatus === 'snapshotting' || freshness.connectionStatus === 'replaying'));
  const canRetry = !waitingForTrades && !live && freshness.connectionStatus !== 'closed';

  return (
    <div data-chart-toolbar className={cn('coin-chart-toolbar relative flex min-h-11 min-w-0 flex-nowrap items-center gap-3 overflow-x-auto whitespace-nowrap bg-canvas px-3 py-2', className)}>
      <div className="flex shrink-0 flex-nowrap items-center gap-1">
        <SegmentedControl label="Candle interval" options={segments} value={interval} onChange={onIntervalChange} shape="sm" />
      </div>

      <div className="flex shrink-0 flex-nowrap items-center gap-1">
        <SegmentedControl<ChartMode>
          label="Chart value"
          options={[{ value: 'price', label: 'Price' }, { value: 'mcap', label: 'MCap', disabled: mcapDisabled }]}
          value={mode}
          onChange={onModeChange}
          shape="sm"
        />
        {mcapDisabled ? (
          <Tooltip content={mcapDisabledReason}>
            <span tabIndex={0} role="img" aria-label={`Market cap not available yet: ${mcapDisabledReason}`} className="inline-flex size-control-height-sm items-center justify-center rounded-sm text-text-faint hover:text-text">
              <Info aria-hidden="true" className="size-icon-sm" />
            </span>
          </Tooltip>
        ) : null}
      </div>

      <DropdownMenu
        label="Chart options"
        trigger={<SlidersHorizontal aria-hidden="true" className="size-icon-md" />}
        size="sm"
        align="end"
        className="shrink-0"
        items={[
          ...secondary.map(option => ({ id: option.value, group: 'More intervals', label: `${option.label} (${option.description})`, selected: option.value === interval, onSelect: () => onIntervalChange(option.value) })),
          { id: 'markers', group: 'Trade markers', label: markers ? 'Hide trade markers' : 'Show trade markers', selected: markers, onSelect: () => onMarkersChange(!markers) },
          { id: 'currency', group: 'Currency', label: 'USDL', selected: true },
        ]}
      />
      {markers && markerActors.length > 0 ? <AvatarStack items={markerActors.map(actor => ({ src: actor.avatarUrl, name: actor.name }))} size="xs" max={5} surface={1} label={`${markerActors.length} ${markerActors.length === 1 ? 'trader' : 'traders'} marked on the chart`} /> : null}

      <div role="status" aria-live="polite" className="ml-auto flex shrink-0 items-center gap-2 text-xs text-text-faint">
        <span className={cn('inline-flex items-center gap-1.5', live && 'text-success')}>
          <span aria-hidden="true" className={cn('size-1.5 rounded-pill', live ? 'bg-success' : !waitingForTrades && freshness.connectionStatus === 'offline' ? 'bg-danger' : 'bg-text-faint')} />
          {waitingForTrades ? 'Waiting for trades' : freshnessLabel(freshness.connectionStatus)}
        </span>
        {freshness.lastSourceHeight !== null ? <span data-numeric className="hidden tabular-nums sm:inline">Last update at block {freshness.lastSourceHeight}</span> : null}
        {!waitingForTrades && freshness.lagReason ? <span className="sr-only">{humanErrorText(freshness.lagReason, 'Live updates are paused')}</span> : null}
        <Button variant="ghost" size="sm" onClick={onRetry} inert={!canRetry} aria-hidden={!canRetry} className={cn(!canRetry && 'invisible')}>Try again</Button>
      </div>
    </div>
  );
}

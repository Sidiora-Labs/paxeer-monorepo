'use client';
import { launchpadMedia } from '@/app/presentation-tokens';
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type CSSProperties,
  type RefObject,
} from 'react';
import { ChevronLeft, ChevronRight, Plus } from 'lucide-react';
import { cn } from '@/lib/utils';
import { DropdownMenu, IconButton, type MenuItem } from '@/ui/primitives';
import { maxColumns, useDashboardLayout, type WidgetType } from './layout-store';
import { widgetOrder, widgetRegistry } from './widget-registry';
import { Column } from './column';

/** Below 1024 the home presents coin and callout tabs; wider views keep the board. */
export const compactHomeQuery = launchpadMedia.compact;

function subscribeMedia(onChange: () => void) {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function')
    return () => undefined;
  const media = window.matchMedia(compactHomeQuery);
  media.addEventListener('change', onChange);
  return () => media.removeEventListener('change', onChange);
}

function readCompact() {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia(compactHomeQuery).matches
  );
}

const desktop = () => false;

/** True below lg, where home renders discovery tabs. */
export function useCompactHome(): boolean {
  return useSyncExternalStore(subscribeMedia, readCompact, desktop);
}

/**
 * The board fits 3 columns from lg, 4 from 1800 and 5 on very wide screens.
 * The slot width divides the list by min(that count, the number of columns), so a board with fewer columns than fit fills the width
 * instead of leaving an empty tail, and a board with more scrolls behind the rail. The 288 px slot
 * minimum keeps every tab row on one line; the column never gets narrower than that.
 */
export const boardVisibleClass =
  '[--board-visible:2] lg:[--board-visible:3] board-wide:[--board-visible:4] board-ultrawide:[--board-visible:5]';
export const columnSlotClass = 'min-w-72';
/** Flush canvas columns with a hairline between adjacent slots. */
export const boardSeamClass = '[--sl-dashboard-column-gap:var(--sl-edge-width)]';
const shown = 'min(var(--board-visible), var(--board-count))';
export const columnSlotStyle = {
  width: `calc((100% - (${shown} - 1) * var(--sl-dashboard-column-gap)) / ${shown})`,
} as const;

/** Visible scrollbar on the board: a surface-3 thumb on a surface-1 track, never hidden. */
export const boardScrollbarClass =
  '[scrollbar-width:auto] [scrollbar-color:var(--sl-surface-3)_var(--sl-surface-1)]';

export interface BoardOverflow {
  overflow: boolean;
  atStart: boolean;
  atEnd: boolean;
  hiddenAfter: number;
}

/** Tracks whether the board scrolls horizontally and where it is, for the scroll rail. */
export function useBoardOverflow(
  ref: RefObject<HTMLElement | null>,
  itemCount: number
): BoardOverflow {
  const [state, setState] = useState<BoardOverflow>({
    overflow: false,
    atStart: true,
    atEnd: true,
    hiddenAfter: 0,
  });
  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const measure = () => {
      const max = element.scrollWidth - element.clientWidth;
      const overflow = max > 1;
      const items = Array.from(element.children) as HTMLElement[];
      const right = element.scrollLeft + element.clientWidth;
      const hiddenAfter = overflow
        ? items.filter((item) => item.offsetLeft + item.offsetWidth / 2 > right).length
        : 0;
      const next = {
        overflow,
        atStart: element.scrollLeft <= 1,
        atEnd: element.scrollLeft >= max - 1,
        hiddenAfter,
      };
      setState((previous) =>
        previous.overflow === next.overflow &&
        previous.atStart === next.atStart &&
        previous.atEnd === next.atEnd &&
        previous.hiddenAfter === next.hiddenAfter
          ? previous
          : next
      );
    };
    measure();
    element.addEventListener('scroll', measure, { passive: true });
    const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(measure) : null;
    observer?.observe(element);
    window.addEventListener('resize', measure);
    return () => {
      element.removeEventListener('scroll', measure);
      observer?.disconnect();
      window.removeEventListener('resize', measure);
    };
  }, [ref, itemCount]);
  return state;
}

export interface DashboardBoardProps {
  className?: string;
}

/**
 * Home dashboard board (layout.md S03, GAP_AC WP-09): a row of widget columns from the persisted
 * layout that fills the width (Trending, Hot callouts, Leaderboard by default). Each column header
 * adds a column to its right. When more columns exist than fit, the row scrolls horizontally with
 * a visible scrollbar and a surface-2 rail on the right (previous / next, hidden count).
 */
export function DashboardBoard({ className }: DashboardBoardProps) {
  const layout = useDashboardLayout();
  const [dragging, setDragging] = useState<string | null>(null);
  const columns = layout.columns;

  const endDrag = useCallback(() => setDragging(null), []);
  const drop = useCallback(
    (id: string, toIndex: number) => {
      layout.move(id, toIndex);
      setDragging(null);
    },
    [layout]
  );
  const add = useCallback(
    (type: WidgetType, atIndex?: number) => {
      layout.add(type, atIndex);
    },
    [layout]
  );

  const addItems: MenuItem[] = widgetOrder.map((type) => {
    const TypeIcon = widgetRegistry[type].icon;
    return {
      id: type,
      label: widgetRegistry[type].title,
      icon: <TypeIcon aria-hidden="true" className="size-icon-sm" />,
      onSelect: () => add(type),
    };
  });
  const full = columns.length >= maxColumns;
  const listRef = useRef<HTMLDivElement | null>(null);
  const overflow = useBoardOverflow(listRef, columns.length);
  const scrollColumns = useCallback((direction: 1 | -1) => {
    const element = listRef.current;
    if (!element) return;
    // One column: the distance between two neighbouring slots, which includes the gap.
    const [first, second] = Array.from(element.children) as HTMLElement[];
    const pitch = first && second ? second.offsetLeft - first.offsetLeft : 0;
    element.scrollBy({ left: direction * (pitch > 0 ? pitch : element.clientWidth) });
  }, []);

  return (
    <div
      className={cn('flex min-h-0 flex-1 gap-dashboard-column-gap', boardSeamClass, className)}
      style={{ height: 'calc(100dvh - var(--shell-top))' }}
    >
      {columns.length === 0 ? (
        <div
          data-board-empty=""
          className="flex flex-1 flex-col items-center gap-3 bg-canvas p-8 text-text-muted"
        >
          <p className="text-md font-semibold text-text">Your board has no columns</p>
          <p className="max-w-70 text-center text-sm text-text-muted">
            Add Trending, callouts, the leaderboard or any other widget as a column.
          </p>
          <DropdownMenu
            label="Add column"
            menuLabel="Add column"
            showLabel
            items={addItems}
            size="md"
            trigger={<Plus aria-hidden="true" className="size-icon-md" />}
            triggerClassName="rounded-md bg-surface-3 px-4 text-text hover:bg-surface-hover"
          />
          <button
            type="button"
            onClick={layout.reset}
            className="text-xs font-bold text-text-muted hover:text-text"
          >
            Restore the default board
          </button>
        </div>
      ) : (
        <div
          ref={listRef}
          role="list"
          aria-label="Dashboard columns"
          data-home-board="true"
          data-board-overflow={overflow.overflow ? 'true' : 'false'}
          data-board-full={full ? 'true' : 'false'}
          className={cn(
            'relative flex min-h-0 min-w-0 flex-1 snap-x snap-mandatory scroll-smooth motion-reduce:scroll-auto focus-within:snap-none gap-dashboard-column-gap overflow-x-auto overflow-y-hidden scroll-px-2 bg-surface-3',
            overflow.overflow && 'pb-1',
            boardVisibleClass,
            boardScrollbarClass
          )}
          style={{ '--board-count': String(columns.length) } as CSSProperties}
        >
          {columns.map((column, index) => (
            <div
              key={column.id}
              role="listitem"
              className={cn('flex h-full shrink-0 snap-start', columnSlotClass)}
              style={columnSlotStyle}
            >
              <Column
                column={column}
                index={index}
                count={columns.length}
                dragging={dragging}
                fluid
                onDragStart={setDragging}
                onDragEnd={endDrag}
                onDrop={drop}
                onMove={layout.move}
                onRemove={layout.remove}
                onChangeType={layout.changeType}
                onPatch={layout.update}
                onAdd={add}
              />
            </div>
          ))}
        </div>
      )}
      {columns.length > 0 ? (
        <div
          data-board-scroll-controls=""
          data-board-rail={overflow.overflow ? '' : undefined}
          hidden={!overflow.overflow}
          className="flex w-8 shrink-0 flex-col items-center justify-center gap-2 bg-surface-2 py-2"
        >
          <IconButton
            size="sm"
            className="text-text"
            label="Scroll columns left"
            icon={<ChevronLeft />}
            disabled={overflow.atStart}
            onClick={() => scrollColumns(-1)}
          />
          <span
            aria-hidden="true"
            hidden={overflow.hiddenAfter === 0}
            className="text-xs font-bold text-text-muted"
          >
            +{overflow.hiddenAfter}
          </span>
          <IconButton
            size="sm"
            className="text-text"
            label={
              overflow.hiddenAfter > 0
                ? `Scroll columns right, ${overflow.hiddenAfter} more`
                : 'Scroll columns right'
            }
            icon={<ChevronRight />}
            disabled={overflow.atEnd}
            onClick={() => scrollColumns(1)}
          />
        </div>
      ) : null}
    </div>
  );
}

export default DashboardBoard;

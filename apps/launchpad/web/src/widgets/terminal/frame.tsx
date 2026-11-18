'use client';
import { launchpadMedia } from '@/app/presentation-tokens';
import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type RefObject,
  type FormEvent,
  type ReactNode,
} from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { ChevronLeft, ChevronRight, ClipboardPaste, Search, Settings } from 'lucide-react';
import { toast } from 'sonner';
import { Button, DropdownMenu, IconButton, ResizeHandle } from '@/ui/primitives';
import { useDismiss } from '@/ui/primitives/use-popover';
import { openSearchPalette } from '@/shell/global-search';
import { cn } from '@/lib/utils';
import { useMediaQuery } from '@/widgets/explore/use-visible';
import TerminalSubNav, { terminalPoolPath, terminalViewForPath } from './sub-nav';
import {
  columnLimits,
  layoutColumns,
  resetTerminalColumns,
  useResetColumnsEvent,
  useTerminalColumns,
  type SideLayout,
} from './column-store';

const ADDRESS_IN_TEXT = /(?:^|[^0-9a-zA-Z])(0x[0-9a-fA-F]{40})(?![0-9a-zA-Z])/;

/** The single 0x40hex address in pasted text (bare, or inside an explorer / coin URL), lowercased; null otherwise. */
export function pastedPoolAddress(text: string): string | null {
  const match = ADDRESS_IN_TEXT.exec(text.trim());
  return match ? match[1].toLowerCase() : null;
}

function isEditable(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    target.tagName === 'INPUT' ||
    target.tagName === 'TEXTAREA' ||
    target.tagName === 'SELECT'
  );
}

/** Height of the fixed shell top stack, read from the inline padding app-shell puts on <main>. */
function useShellTop(): number | null {
  const [top, setTop] = useState<number | null>(null);
  useEffect(() => {
    const main = document.getElementById('main-content');
    if (!main) return;
    const read = () => {
      const value = Number.parseFloat(main.style.paddingTop);
      setTop(Number.isFinite(value) && value > 0 ? value : null);
    };
    read();
    const observer = new MutationObserver(read);
    observer.observe(main, { attributes: true, attributeFilter: ['style'] });
    return () => observer.disconnect();
  }, []);
  return top;
}

function useElementWidth<T extends HTMLElement>(): [RefObject<T | null>, number | null, number] {
  const ref = useRef<T | null>(null);
  const [width, setWidth] = useState<number | null>(null);
  const [textScale, setTextScale] = useState(1);
  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const update = () => {
      setWidth(element.clientWidth);
      setTextScale(Number.parseFloat(getComputedStyle(document.documentElement).fontSize) / 16);
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(element);
    const rootObserver = new MutationObserver(update);
    rootObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['style', 'class'],
    });
    return () => {
      observer.disconnect();
      rootObserver.disconnect();
    };
  }, []);
  return [ref, width, textScale];
}

function PasteCaButton() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [manual, setManual] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [reading, setReading] = useState(false);
  const anchorRef = useRef<HTMLDivElement | null>(null);
  const inputId = useId();
  useDismiss(open, () => setOpen(false), [anchorRef]);

  const go = (address: string) => {
    router.push(terminalPoolPath(address));
    setOpen(false);
    setManual('');
    setError(null);
  };

  const paste = async () => {
    if (reading) return;
    setReading(true);
    try {
      const text = await navigator.clipboard.readText();
      const address = pastedPoolAddress(text);
      if (address) go(address);
      else toast('Copy a coin address first, then paste it here.');
    } catch {
      setOpen(true);
    } finally {
      setReading(false);
    }
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const address = pastedPoolAddress(manual);
    if (!address) {
      setError('Enter a valid coin address (0x...).');
      return;
    }
    go(address);
  };

  return (
    <div ref={anchorRef} className="relative shrink-0">
      <Button
        variant="secondary"
        size="sm"
        icon={<ClipboardPaste />}
        loading={reading}
        aria-expanded={open}
        onClick={() => {
          void paste();
        }}
      >
        Paste CA
      </Button>
      {open ? (
        <form
          onSubmit={submit}
          role="dialog"
          aria-label="Open a coin by address"
          className="absolute right-0 top-full z-[var(--sl-z-dropdown)] mt-1 flex w-full max-w-menu-min flex-col gap-2 rounded-md bg-surface-2 p-3 shadow-pop"
        >
          <label htmlFor={inputId} className="text-xs font-bold text-text-muted">
            Paste the coin address.
          </label>
          <input
            id={inputId}
            value={manual}
            onChange={(event) => {
              setManual(event.target.value);
              setError(null);
            }}
            autoFocus
            autoComplete="off"
            spellCheck={false}
            placeholder="0x..."
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? `${inputId}-error` : undefined}
            className="h-control-height-sm w-full rounded-sm bg-surface-3 px-3 font-mono text-sm text-text placeholder:text-text-faint"
          />
          {error ? (
            <p id={`${inputId}-error`} role="alert" className="text-xs text-danger">
              {error}
            </p>
          ) : null}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" size="sm" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" size="sm">
              Open coin
            </Button>
          </div>
        </form>
      ) : null}
    </div>
  );
}

interface ColumnHandleProps {
  side: 'left' | 'right';
  layout: SideLayout;
  onResize(width: number): void;
  onToggle(): void;
}

/** Column controls share a compact gutter for resizing and collapsing. */
function ColumnHandle({ side, layout, onResize, onToggle }: ColumnHandleProps) {
  const min = side === 'left' ? columnLimits.leftMin : columnLimits.rightMin;
  const fallback = side === 'left' ? columnLimits.leftDefault : columnLimits.rightDefault;
  const open = layout.mode === 'open';
  const label = side === 'left' ? 'left column' : 'right column';

  return (
    <div className="group relative hidden w-1 shrink-0 lg:block">
      {open ? (
        <ResizeHandle
          orientation="vertical"
          inverted={side === 'right'}
          value={layout.width}
          min={min}
          max={layout.max}
          defaultValue={fallback}
          onChange={onResize}
          label={`Resize ${label}`}
          className="absolute -inset-x-1 inset-y-0 z-10 h-auto w-auto min-w-0 rounded-sm"
        />
      ) : null}
      <button
        type="button"
        onClick={onToggle}
        aria-label={open ? `Collapse ${label}` : `Expand ${label}`}
        aria-expanded={open}
        title={open ? `Collapse ${label}` : `Expand ${label}`}
        className="absolute top-1/2 left-1/2 z-20 flex h-8 w-4 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-sm bg-surface-2 text-text-muted border border-control-edge transition-colors duration-fast hover:bg-surface-hover hover:text-text [&_svg]:size-icon-sm"
      >
        {(side === 'left') === open ? (
          <ChevronLeft aria-hidden="true" />
        ) : (
          <ChevronRight aria-hidden="true" />
        )}
      </button>
    </div>
  );
}

function CollapsedStrip({
  side,
  hidden,
  onExpand,
}: {
  side: 'left' | 'right';
  hidden: boolean;
  onExpand(): void;
}) {
  const label = side === 'left' ? 'Expand left column' : 'Expand right column';
  return (
    <div
      hidden={hidden}
      className="hidden w-6 shrink-0 flex-col items-center rounded-md bg-surface-1 py-1 lg:flex"
    >
      <IconButton
        label={label}
        icon={side === 'left' ? <ChevronRight /> : <ChevronLeft />}
        variant="ghost"
        size="sm"
        className="w-full"
        onClick={onExpand}
      />
    </div>
  );
}

export interface TerminalFrameProps {
  left?: ReactNode;
  center: ReactNode;
  right?: ReactNode;
  status?: ReactNode;
  /** Optional row between the header and the columns (route-level notices). */
  children?: ReactNode;
}

/**
 * The terminal workspace: a 40px header (sub-nav, Paste CA, search, column
 * settings), up to three columns (left / centre / right) that scroll
 * independently, and a status-bar slot pinned to the bottom. From lg up the
 * frame fills the viewport under the shell top stack; below lg the columns
 * stack and the page scrolls.
 */
export default function TerminalFrame({
  left,
  center,
  right,
  status,
  children,
}: TerminalFrameProps) {
  const pathname = usePathname();
  const active = terminalViewForPath(pathname) ?? 'trending';
  const top = useShellTop();
  const columns = useTerminalColumns();
  const [bodyRef, bodyWidth, textScale] = useElementWidth<HTMLDivElement>();
  const wide = useMediaQuery(launchpadMedia.large);
  const hasLeft = left !== undefined && left !== null;
  const hasRight = right !== undefined && right !== null;

  useResetColumnsEvent();

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (
        event.key !== '/' ||
        event.metaKey ||
        event.ctrlKey ||
        event.altKey ||
        event.defaultPrevented ||
        isEditable(event.target)
      )
        return;
      event.preventDefault();
      openSearchPalette();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);

  const layout = layoutColumns(
    wide && bodyWidth !== null ? bodyWidth : Number.POSITIVE_INFINITY,
    {
      present: hasLeft,
      width: columns.leftWidth,
      collapsed: columns.leftCollapsed,
      min: columnLimits.leftMin,
    },
    {
      present: hasRight,
      width: columns.rightWidth,
      collapsed: columns.rightCollapsed,
      min: columnLimits.rightMin,
    },
    textScale,
  );

  const toggle = (side: 'left' | 'right') => {
    const sideLayout = side === 'left' ? layout.left : layout.right;
    const stored = side === 'left' ? columns.leftCollapsed : columns.rightCollapsed;
    if (sideLayout.autoCollapsed && !stored) {
      toast(
        'There is not enough room to open this column. Collapse the other column or widen the window.',
      );
      return;
    }
    if (side === 'left') columns.toggleLeft();
    else columns.toggleRight();
  };

  const settingsItems = [
    { id: 'reset', label: 'Reset column layout', onSelect: resetTerminalColumns },
    ...(hasLeft && wide
      ? [
          {
            id: 'left',
            label: layout.left.mode === 'open' ? 'Collapse left column' : 'Expand left column',
            onSelect: () => toggle('left'),
          },
        ]
      : []),
    ...(hasRight && wide
      ? [
          {
            id: 'right',
            label: layout.right.mode === 'open' ? 'Collapse right column' : 'Expand right column',
            onSelect: () => toggle('right'),
          },
        ]
      : []),
  ];

  const style = {
    '--terminal-top': top === null ? 'var(--spacing-shell-top-total)' : `${top}px`,
    '--terminal-left': `${layout.left.width}px`,
    '--terminal-right': `${layout.right.width}px`,
  } as CSSProperties;

  return (
    <div
      style={style}
      data-terminal-frame
      className="flex min-w-0 flex-col bg-canvas lg:h-[calc(100dvh-var(--terminal-top))] lg:overflow-hidden"
    >
      <header className="flex min-h-10 shrink-0 flex-wrap items-center gap-2 px-2 py-1">
        <h1 className="shrink-0 px-1 text-sm font-extrabold text-text">Terminal</h1>
        <div className="order-last min-w-0 basis-full lg:order-none lg:flex-1 lg:basis-0">
          <TerminalSubNav active={active} />
        </div>
        <div className="flex max-w-full flex-wrap items-center gap-1">
          <PasteCaButton />
          <IconButton
            label="Search pairs (press /)"
            icon={<Search />}
            variant="raised"
            size="sm"
            onClick={openSearchPalette}
          />
          <DropdownMenu
            label="Terminal layout"
            align="end"
            size="sm"
            trigger={<Settings aria-hidden="true" className="size-icon-sm" />}
            items={settingsItems}
          />
        </div>
      </header>
      {children}
      <div
        ref={bodyRef}
        aria-busy={bodyWidth === null}
        className={cn(
          'flex min-h-0 flex-1 flex-col gap-1 px-1 pb-1 lg:flex-row lg:gap-0',
          bodyWidth === null && 'lg:invisible',
        )}
      >
        {hasLeft ? (
          <>
            <CollapsedStrip
              side="left"
              hidden={layout.left.mode === 'open'}
              onExpand={() => toggle('left')}
            />
            <aside
              aria-label="Pair discovery"
              className={cn(
                'min-w-0 overflow-y-auto rounded-md bg-surface-1 lg:w-[var(--terminal-left)] lg:shrink-0',
                layout.left.mode !== 'open' && 'lg:hidden',
              )}
            >
              {left}
            </aside>
            <ColumnHandle
              side="left"
              layout={layout.left}
              onResize={columns.setLeftWidth}
              onToggle={() => toggle('left')}
            />
          </>
        ) : null}
        <section
          aria-label="Workspace"
          className="@container/workspace flex min-w-0 flex-col gap-1 bg-canvas lg:min-h-0 lg:flex-1 lg:overflow-y-auto"
        >
          {center}
        </section>
        {hasRight ? (
          <>
            <ColumnHandle
              side="right"
              layout={layout.right}
              onResize={columns.setRightWidth}
              onToggle={() => toggle('right')}
            />
            <aside
              aria-label="Position and trading"
              className={cn(
                'min-w-0 overflow-y-auto rounded-md bg-surface-1 lg:w-[var(--terminal-right)] lg:shrink-0',
                layout.right.mode !== 'open' && 'lg:hidden',
              )}
            >
              {right}
            </aside>
            <CollapsedStrip
              side="right"
              hidden={layout.right.mode === 'open'}
              onExpand={() => toggle('right')}
            />
          </>
        ) : null}
      </div>
      {status ? <div className="shrink-0">{status}</div> : null}
    </div>
  );
}

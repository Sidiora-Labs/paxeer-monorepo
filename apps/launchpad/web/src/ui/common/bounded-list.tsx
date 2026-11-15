'use client';

import { useEffect, useRef, useState, type ReactNode, type Key } from 'react';
import { listWindow } from '@/core/app-foundation/window';
import { surfaceToken } from '@/core/app-foundation/surface-token';

interface VirtualListProps<T> {
  items: T[];
  estimateSize?: number;
  renderItem: (item: T, index: number) => ReactNode;
  maxHeight?: number;
  emptyMessage?: string;
  className?: string;
  getItemKey?: (item: T, index: number) => Key;
}

function defaultKey(item: unknown, index: number): Key {
  if (item && typeof item === 'object') {
    const row = item as Record<string, unknown>;
    for (const field of ['id', 'poolAddress', 'pool_address', 'address', 'tokenAddress']) {
      if (typeof row[field] === 'string' || typeof row[field] === 'number')
        return row[field] as Key;
    }
  }
  return index;
}

/** Bounded rows reserve the tallest measured content height, including enlarged text. */
export function VirtualList<T>({
  items,
  estimateSize,
  renderItem,
  maxHeight,
  emptyMessage = 'No items',
  className,
  getItemKey = defaultKey,
}: VirtualListProps<T>) {
  const ref = useRef<HTMLDivElement>(null);
  const [offset, setOffset] = useState(0);
  const [geometry, setGeometry] = useState<{ row: number; viewport: number } | null>(null);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const measure = () => {
      const minimum = estimateSize ?? surfaceToken('--sl-table-row-dense');
      const viewport = maxHeight ?? surfaceToken('--sl-list-max-height');
      const row = Math.ceil(
        Math.max(
          minimum,
          ...Array.from(element.querySelectorAll<HTMLElement>('[data-index] > div')).map(
            (item) => item.getBoundingClientRect().height
          )
        )
      );
      setGeometry((current) => {
        const measured = Math.max(current?.row ?? minimum, row);
        return current?.row === measured && current.viewport === viewport
          ? current
          : { row: measured, viewport };
      });
    };
    measure();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    observer.observe(document.documentElement);
    for (const row of element.querySelectorAll('[data-index] > div')) observer.observe(row);
    return () => observer.disconnect();
  }, [items, estimateSize, maxHeight, offset, geometry?.row]);

  if (!items.length)
    return (
      <div role="status" className="p-4 text-center text-xs text-text-faint">
        {emptyMessage}
      </div>
    );
  const visible = geometry
    ? listWindow(items.length, geometry.row, geometry.viewport, offset)
    : null;
  const start = visible?.start ?? 0;
  const end = visible?.end ?? 1;
  return (
    <div
      ref={ref}
      data-foundation-bounded-list
      role="list"
      aria-label="Results"
      tabIndex={0}
      className={`overflow-y-auto overscroll-contain ${className ?? ''}`}
      style={{
        height: geometry && visible ? Math.min(geometry.viewport, visible.totalHeight) : undefined,
        maxHeight: maxHeight ?? 'var(--sl-list-max-height)',
        overflowAnchor: 'none',
      }}
      onScroll={(event) => setOffset(event.currentTarget.scrollTop)}
    >
      <div style={{ height: visible?.totalHeight, position: 'relative', width: '100%' }}>
        {items.slice(start, end).map((item, localIndex) => {
          const index = start + localIndex;
          return (
            <div
              role="listitem"
              aria-posinset={index + 1}
              aria-setsize={items.length}
              key={getItemKey(item, index)}
              data-index={index}
              style={{
                position: visible ? 'absolute' : 'relative',
                top: geometry ? index * geometry.row : undefined,
                minHeight: geometry?.row,
                width: '100%',
              }}
            >
              <div className="flow-root">{renderItem(item, index)}</div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

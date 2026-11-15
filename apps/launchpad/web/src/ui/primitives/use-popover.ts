'use client';

import { surfaceToken } from '@/core/app-foundation/surface-token';
import { useCallback, useEffect, useLayoutEffect, useState, type RefObject } from 'react';

export interface PopoverPosition {
  top: number;
  left: number;
  minWidth: number;
}

const useIsoLayoutEffect = typeof window === 'undefined' ? useEffect : useLayoutEffect;

/** Fixed-position anchor placement for portalled layers. Flips above the anchor when it would overflow the viewport. */
export function usePopoverPosition(
  anchorRef: RefObject<HTMLElement | null>,
  layerRef: RefObject<HTMLElement | null>,
  open: boolean,
  align: 'start' | 'end' | 'center' = 'start',
  offset?: number,
  popover: 'manual' | 'hint' | 'auto' = 'manual'
): PopoverPosition | null {
  const [position, setPosition] = useState<PopoverPosition | null>(null);

  const measure = useCallback(() => {
    const anchor = anchorRef.current;
    if (!anchor) return;
    const rect = anchor.getBoundingClientRect();
    const layer = layerRef.current;
    if (layer && typeof layer.showPopover === 'function') {
      layer.classList.add('sl-popover');
      layer.setAttribute('popover', popover);
      if (!layer.matches(':popover-open')) layer.showPopover();
    }
    const gutter = surfaceToken('--sl-viewport-gutter');
    const gap = offset ?? surfaceToken('--sl-popover-gap');
    const layerWidth = layer?.offsetWidth ?? rect.width;
    const layerHeight = layer?.offsetHeight ?? 0;
    const viewportW = window.innerWidth;
    const viewportH = window.innerHeight;
    let left =
      align === 'end'
        ? rect.right - layerWidth
        : align === 'center'
          ? rect.left + rect.width / 2 - layerWidth / 2
          : rect.left;
    left = Math.max(gutter, Math.min(left, viewportW - layerWidth - gutter));
    let top = rect.bottom + gap;
    if (
      layerHeight &&
      top + layerHeight > viewportH - gutter &&
      rect.top - gap - layerHeight > gutter
    ) {
      top = rect.top - gap - layerHeight;
    }
    top = Math.max(gutter, Math.min(top, viewportH - layerHeight - gutter));
    setPosition((current) =>
      current?.top === top && current.left === left && current.minWidth === rect.width
        ? current
        : { top, left, minWidth: rect.width }
    );
  }, [anchorRef, layerRef, align, offset, popover]);

  useIsoLayoutEffect(() => {
    if (!open) return;
    measure();
    const layer = layerRef.current;
    if (!layer || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(() => measure());
    observer.observe(layer);
    return () => observer.disconnect();
  });

  useIsoLayoutEffect(() => {
    if (!open) {
      const layer = layerRef.current;
      if (layer && typeof layer.hidePopover === 'function' && layer.matches(':popover-open'))
        layer.hidePopover();
      setPosition(null);
      return;
    }
    measure();
    const onChange = () => measure();
    window.addEventListener('resize', onChange);
    window.addEventListener('scroll', onChange, true);
    return () => {
      window.removeEventListener('resize', onChange);
      window.removeEventListener('scroll', onChange, true);
    };
  }, [open, measure, layerRef]);

  return position;
}

export function useMounted(): boolean {
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);
  return mounted;
}

/** Closes when a pointer event lands outside both refs, or on Escape. */
export function useDismiss(
  open: boolean,
  onDismiss: () => void,
  refs: Array<RefObject<HTMLElement | null>>
) {
  useEffect(() => {
    if (!open) return;
    function onPointer(event: PointerEvent) {
      const target = event.target as Node | null;
      if (!target) return;
      if (refs.some((ref) => ref.current?.contains(target))) return;
      onDismiss();
    }
    function onKey(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopPropagation();
        event.stopImmediatePropagation();
        onDismiss();
        refs[0]?.current?.focus();
      }
    }
    document.addEventListener('pointerdown', onPointer, true);
    document.addEventListener('keydown', onKey, true);
    return () => {
      document.removeEventListener('pointerdown', onPointer, true);
      document.removeEventListener('keydown', onKey, true);
    };
  }, [open, onDismiss, refs]);
}

let idCounter = 0;
export function nextId(prefix: string): string {
  idCounter += 1;
  return `${prefix}-${idCounter}`;
}

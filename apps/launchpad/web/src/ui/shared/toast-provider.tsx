'use client';

import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { toast, Toaster, useSonner, type ToastT } from 'sonner';
import { useColorMode } from '@/core/app-foundation/use-color-mode';

export const PLAIN_NOTICE_DURATION_MS = 6000;

function noticeDuration(notice: ToastT) {
  if (
    notice.type === 'error' ||
    notice.type === 'warning' ||
    notice.type === 'loading' ||
    notice.action ||
    notice.cancel
  )
    return Infinity;
  return notice.duration ?? PLAIN_NOTICE_DURATION_MS;
}

function useNoticeExpiry(paused: boolean) {
  const { toasts } = useSonner();
  const lifetimes = useRef(new Map<ToastT['id'], { notice: ToastT; remaining: number }>());

  useEffect(() => {
    const active = new Set(toasts.map((notice) => notice.id));
    for (const id of lifetimes.current.keys()) {
      if (!active.has(id)) lifetimes.current.delete(id);
    }
    const running: Array<{
      lifetime: { remaining: number };
      timer: ReturnType<typeof setTimeout>;
    }> = [];
    const started = Date.now();
    for (const notice of toasts) {
      let lifetime = lifetimes.current.get(notice.id);
      if (!lifetime || lifetime.notice !== notice) {
        lifetime = { notice, remaining: noticeDuration(notice) };
        lifetimes.current.set(notice.id, lifetime);
      }
      if (paused || !Number.isFinite(lifetime.remaining)) continue;
      const timer = setTimeout(() => {
        lifetime.remaining = Infinity;
        notice.onAutoClose?.(notice);
        toast.dismiss(notice.id);
      }, lifetime.remaining);
      running.push({ lifetime, timer });
    }
    return () => {
      for (const { lifetime, timer } of running) {
        clearTimeout(timer);
        lifetime.remaining = Math.max(0, lifetime.remaining - (Date.now() - started));
      }
    };
  }, [toasts, paused]);
}

export default function ToastProvider() {
  const { scheme } = useColorMode();
  const layerRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const [suppressed, setSuppressed] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  useNoticeExpiry(hovered || focused);
  useEffect(() => {
    const layer = layerRef.current;
    if (!layer) return;
    if (typeof layer.showPopover === 'function') {
      layer.setAttribute('popover', 'manual');
      layer.showPopover();
    }
    const announced = new WeakMap<HTMLElement, string>();
    const sync = () => {
      if (!layer.querySelector('[data-sonner-toast]')) {
        setHovered(false);
        setFocused(false);
      }
      const modal = Boolean(
        document.querySelector('dialog[open], [role="dialog"][aria-modal="true"]')
      );
      setSuppressed(modal);
      if ('ariaNotify' in document && typeof document.ariaNotify === 'function') {
        layer.querySelector('section[aria-live]')?.setAttribute('aria-live', 'off');
        if (!modal) {
          for (const toast of layer.querySelectorAll<HTMLElement>(
            '[data-sonner-toast]:not([data-removed="true"])'
          )) {
            const text = ['[data-title]', '[data-description]']
              .map((selector) => toast.querySelector(selector)?.textContent?.trim())
              .filter(Boolean)
              .join('. ');
            const message = toast.querySelector('[data-button]')
              ? `${text}. Actions available`
              : text;
            if (text && announced.get(toast) !== message) {
              document.ariaNotify(message, { priority: 'normal' });
              announced.set(toast, message);
            }
          }
        }
      }
      if (
        layer.querySelector('[data-sonner-toast]') &&
        !previousFocus.current &&
        !layer.contains(document.activeElement) &&
        document.activeElement !== document.body &&
        document.activeElement instanceof HTMLElement
      )
        previousFocus.current = document.activeElement;
    };
    sync();
    const observer = new MutationObserver(sync);
    observer.observe(document.body, {
      childList: true,
      characterData: true,
      subtree: true,
      attributes: true,
      attributeFilter: ['open'],
    });
    return () => {
      observer.disconnect();
      if (typeof layer.hidePopover === 'function' && layer.matches(':popover-open'))
        layer.hidePopover();
    };
  }, []);
  return (
    <div
      ref={layerRef}
      className="sl-notifications"
      data-suppressed={suppressed}
      aria-hidden={suppressed || undefined}
      inert={suppressed}
      onPointerOverCapture={(event) => {
        if (event.target instanceof Element && event.target.closest('[data-sonner-toast]'))
          setHovered(true);
      }}
      onPointerOutCapture={(event) => {
        if (
          !(event.relatedTarget instanceof Element) ||
          !event.relatedTarget.closest('[data-sonner-toast]')
        )
          setHovered(false);
      }}
      onFocusCapture={(event) => {
        setFocused(true);
        if (
          event.relatedTarget instanceof HTMLElement &&
          event.relatedTarget !== document.body &&
          !event.currentTarget.contains(event.relatedTarget)
        )
          previousFocus.current = event.relatedTarget;
      }}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false);
      }}
      onClickCapture={(event) => {
        const close =
          event.target instanceof Element ? event.target.closest('[data-close-button]') : null;
        if (!close) return;
        const closing = close.closest('[data-sonner-toast]');
        requestAnimationFrame(() => {
          const next = Array.from(
            layerRef.current?.querySelectorAll<HTMLElement>(
              '[data-sonner-toast]:not([data-removed="true"]) [data-close-button]'
            ) ?? []
          ).find((button) => button.closest('[data-sonner-toast]') !== closing);
          if (next) next.focus();
          else if (previousFocus.current?.isConnected) previousFocus.current.focus();
        });
      }}
    >
      <Toaster
        position="bottom-center"
        theme={scheme}
        duration={Infinity}
        offset={{ bottom: 'var(--sl-notice-bottom-offset)' }}
        toastOptions={{
          style: {
            background: 'var(--sl-floating-layer-background)',
            color: 'var(--sl-text)',
            border: 'var(--sl-edge-width) solid var(--sl-surface-3)',
            backdropFilter: 'blur(var(--sl-floating-blur))',
            fontSize: 'var(--sl-text-sm)',
            fontFamily: 'var(--sl-font-sans)',
            borderRadius: 'var(--sl-radius-lg)',
            boxShadow: 'var(--sl-shadow-pop)',
            maxWidth: 'calc(100dvw - var(--sl-space-8))',
          },
          classNames: {
            success: '[&>div>svg]:text-accent',
            error: '[&>div>svg]:text-danger',
            warning: '[&>div>svg]:text-warning',
            info: '[&>div>svg]:text-info',
          },
        }}
        style={
          {
            '--gap': 'var(--sl-space-2)',
            '--width': 'min(var(--sl-toast-width), calc(100dvw - var(--sl-space-8)))',
            '--normal-bg': 'var(--sl-surface-2)',
            '--normal-text': 'var(--sl-text)',
            '--normal-border': 'var(--sl-control-edge)',
            '--normal-bg-hover': 'var(--sl-surface-hover)',
            '--normal-border-hover': 'var(--sl-control-edge)',
          } as CSSProperties
        }
        expand={false}
        richColors={false}
        closeButton
        mobileOffset={{
          bottom: 'var(--sl-notice-bottom-offset)',
          left: 'var(--sl-space-4)',
          right: 'var(--sl-space-4)',
          top: 'var(--sl-space-4)',
        }}
      />
    </div>
  );
}

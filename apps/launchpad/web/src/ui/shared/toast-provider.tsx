'use client';

import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { Toaster } from 'sonner';

export default function ToastProvider() {
  const layerRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const [suppressed, setSuppressed] = useState(false);
  useEffect(() => {
    const layer = layerRef.current;
    if (!layer) return;
    if (typeof layer.showPopover === 'function') {
      layer.setAttribute('popover', 'manual');
      layer.showPopover();
    }
    const announced = new WeakMap<HTMLElement, string>();
    const sync = () => {
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
      onFocusCapture={(event) => {
        if (
          event.relatedTarget instanceof HTMLElement &&
          event.relatedTarget !== document.body &&
          !event.currentTarget.contains(event.relatedTarget)
        )
          previousFocus.current = event.relatedTarget;
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
        theme="system"
        duration={Infinity}
        offset="var(--sl-space-6)"
        toastOptions={{
          style: {
            background: 'var(--sl-surface-3)',
            color: 'var(--sl-text)',
            borderWidth: 0,
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
            '--normal-bg': 'var(--sl-surface-3)',
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
          bottom:
            'calc(var(--sl-bottom-nav-height) + var(--sl-space-8) + env(safe-area-inset-bottom, 0rem))',
          left: 'var(--sl-space-4)',
          right: 'var(--sl-space-4)',
          top: 'var(--sl-space-4)',
        }}
      />
    </div>
  );
}

'use client';

import { type ReactNode, useState, useEffect, useLayoutEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { usePathname } from 'next/navigation';
import { clearPrivateFoundationCache } from '@/core/app-foundation/private-cache';
import { routeForPath, safeExternalDestination } from '@/core/app-foundation/navigation';
import '@/core/app-foundation/design-tokens.css';
import { installNavigationHistory } from '@/core/app-foundation/history-context';
import { Button } from '@/ui/primitives';
import { SquiCircleFilterStatic } from '@/new-components/SkiperSquiCircleFilterLayout';
import { useWalletToasts } from '@/hooks/ui/use-wallet-toasts';
import { cn } from '@/lib/utils';
import SidebarNav, { useRailState } from './sidebar-nav';
import TopBar from './top-bar';
import TickerStrip from './ticker-strip';
import MobileBottomNav from './mobile-bottom-nav';
import { SHELL_TOP_VARS } from './nav-config';
import { ShellFooter } from './platform-metrics';

export default function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [ready, setReady] = useState(false);
  useEffect(() => setReady(true), []);
  const queryClient = useQueryClient();
  const [privateEpoch, setPrivateEpoch] = useState(0);
  const [linkBlocked, setLinkBlocked] = useState(false);
  const previousPath = useRef(pathname);
  const { expanded: railExpanded } = useRailState();
  const shellRef = useRef<HTMLDivElement>(null);
  const topRef = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    const element = topRef.current;
    if (!element) return;
    const update = () => {
      const height = element.offsetHeight;
      shellRef.current?.style.setProperty('--shell-top', `${height}px`);
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (previousPath.current === pathname) return;
    document.getElementById('main-content')?.focus({ preventScroll: true });
    previousPath.current = pathname;
  }, [pathname]);

  useEffect(() => {
    const clear = () => setPrivateEpoch((epoch) => epoch + 1);
    const restored = (event: PageTransitionEvent) => {
      if (event.persisted) void clearPrivateFoundationCache(queryClient);
    };
    window.addEventListener('kindle:private-state-cleared', clear);
    window.addEventListener('pageshow', restored);
    return () => {
      window.removeEventListener('kindle:private-state-cleared', clear);
      window.removeEventListener('pageshow', restored);
    };
  }, [queryClient]);
  useEffect(() => {
    const guard = (event: MouseEvent) => {
      const anchor = event.target instanceof Element ? event.target.closest('a[href]') : null;
      if (!(anchor instanceof HTMLAnchorElement)) return;
      let target: URL;
      try {
        target = new URL(anchor.href);
      } catch {
        event.preventDefault();
        event.stopPropagation();
        setLinkBlocked(true);
        return;
      }
      if (
        target.origin === window.location.origin &&
        target.protocol === window.location.protocol &&
        !target.username &&
        !target.password
      )
        return;
      const approved = safeExternalDestination(anchor.href);
      if (!approved) {
        event.preventDefault();
        event.stopPropagation();
        setLinkBlocked(true);
        return;
      }
      anchor.href = approved;
      anchor.rel = 'noopener noreferrer';
      anchor.referrerPolicy = 'no-referrer';
    };
    document.addEventListener('click', guard, true);
    document.addEventListener('auxclick', guard, true);
    return () => {
      document.removeEventListener('click', guard, true);
      document.removeEventListener('auxclick', guard, true);
    };
  }, []);
  useEffect(() => installNavigationHistory(), []);
  useEffect(() => {
    window.dispatchEvent(new Event('kindle:navigation-complete'));
  }, [pathname]);
  useWalletToasts();

  const route = routeForPath(pathname);
  // --shell-top is the fixed header stack height: measured once mounted, and
  // before that the CSS default (top bar, plus the ticker from md up).

  return (
    <div
      ref={shellRef}
      onFocusCapture={(event) => {
        const control = event.target;
        if (
          !(control instanceof HTMLElement) ||
          !control.closest('[data-shell-main]') ||
          !control.matches(':focus-visible')
        )
          return;
        if (typeof control.scrollIntoView !== 'function') return;
        requestAnimationFrame(() => {
          control.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'instant' });
          const headerBottom = topRef.current?.getBoundingClientRect().bottom ?? 0;
          const rect = control.getBoundingClientRect();
          const navigation = document.querySelector('nav[aria-label="Mobile navigation"]');
          const navigationTop =
            navigation instanceof HTMLElement && navigation.getClientRects().length > 0
              ? navigation.getBoundingClientRect().top
              : window.innerHeight;
          const cookieBanner = document.querySelector('[data-cookie-banner]');
          let visibleBottom = Math.min(
            navigationTop,
            cookieBanner instanceof HTMLElement
              ? cookieBanner.getBoundingClientRect().top
              : window.innerHeight
          );
          const floatingAction = document.querySelector('[data-floating-action]');
          if (
            floatingAction instanceof HTMLElement &&
            floatingAction.getClientRects().length > 0 &&
            !floatingAction.contains(control)
          ) {
            const actionRect = floatingAction.getBoundingClientRect();
            if (rect.right > actionRect.left && rect.left < actionRect.right) {
              visibleBottom = Math.min(visibleBottom, actionRect.top);
            }
          }
          if (rect.top < headerBottom || rect.height > visibleBottom - headerBottom) {
            window.scrollBy({ top: rect.top - headerBottom, behavior: 'instant' });
          } else if (rect.bottom > visibleBottom) {
            window.scrollBy({ top: rect.bottom - visibleBottom, behavior: 'instant' });
          }
        });
      }}
      data-shell-ready={ready ? '' : undefined}
      data-shell-pathname={pathname}
      data-shell-route-id={route?.id || 'unknown'}
      className={cn('foundation-shell min-w-0 max-w-full bg-canvas text-text', SHELL_TOP_VARS)}
    >
      <a
        href="#main-content"
        className="fixed left-3 top-3 z-[var(--sl-z-tooltip)] min-h-touch-target-min -translate-y-[calc(100%+var(--sl-space-6))] rounded-sm bg-surface-3 p-3 text-sm text-text focus:translate-y-0"
      >
        Skip to content
      </a>
      <SquiCircleFilterStatic />
      <SidebarNav />
      <div
        className={cn(
          'flex min-h-screen flex-col md:ml-rail-width',
          railExpanded && 'xl:ml-rail-expanded-width'
        )}
      >
        <div
          ref={topRef}
          className={cn(
            'fixed left-0 right-0 top-0 z-[var(--sl-z-header)] flex flex-col bg-canvas md:left-rail-width',
            railExpanded && 'xl:left-rail-expanded-width'
          )}
        >
          <TopBar />
          <TickerStrip />
        </div>

        <main
          id="main-content"
          tabIndex={-1}
          data-shell-main=""
          className={cn(
            'flex min-h-dvh min-w-0 flex-1 flex-col pt-[var(--shell-top)] pb-[calc(var(--sl-bottom-nav-height)+env(safe-area-inset-bottom,0rem)+var(--sl-cookie-banner-height,0px))] md:pb-[var(--sl-cookie-banner-height,0px)]',
            route?.id === 'discovery' && 'h-dvh flex-none'
          )}
        >
          {linkBlocked ? (
            <p
              role="alert"
              className="flex items-center gap-2 bg-warning-soft px-4 py-2 text-sm text-text"
            >
              This link can't be opened
              <Button variant="ghost" size="sm" onClick={() => setLinkBlocked(false)}>
                Dismiss
              </Button>
            </p>
          ) : null}
          {route ? (
            <div
              key={privateEpoch}
              data-shell-content=""
              className={cn(
                'flex min-w-0 flex-1 flex-col',
                route.id === 'discovery' && 'min-h-0',
                route.id === 'chat' && pathname.startsWith('/chat/') &&
                  'min-h-[calc(100vh-var(--sl-space)*15)]',
                (route.id === 'coin' || route.id === 'create' || (route.id === 'token' && pathname !== '/token')) &&
                  'min-h-[calc(100dvh-var(--shell-top))]',
                (route.id === 'swap' || route.id === 'legacy-swap' || route.id === 'orders') &&
                  'min-h-[calc(100dvh-var(--sl-space-8)*2)]'
              )}
            >
              {children}
            </div>
          ) : (
            <section data-shell-content="" className="flex flex-1 flex-col items-start gap-3 p-6">
              <h1 className="text-xl font-extrabold text-text">Page not found</h1>
              <p className="text-sm text-text-muted">We couldn't find that page</p>
              <Button href="/" variant="secondary" size="md">
                Go home
              </Button>
            </section>
          )}
          {route?.id === 'terminal' || route?.id === 'holder-rewards' || route?.id === 'leaderboard' ? null : <ShellFooter />}
        </main>
      </div>
      <MobileBottomNav />
    </div>
  );
}

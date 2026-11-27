'use client';

import { useState, useEffect, useLayoutEffect, useRef, useSyncExternalStore } from 'react';
import Link from 'next/link';
import { BOTTOM_NAV_BREAKPOINT_PX } from '@/shell/mobile-bottom-nav';
import { Button } from '@/ui/primitives';
import { RULES_AGREED_EVENT, rulesAgreed } from './platform-rules-modal';

const COOKIE_KEY = 'sidiora_cookie_consent';

/**
 * Below the bottom-nav breakpoint the phone tab bar is pinned to the viewport
 * bottom, so the banner sits directly above it (bar height plus the safe-area
 * inset the bar pads with) and never covers its targets. From the breakpoint
 * up there is no bar and the notice floats at the lower left.
 */
export const COOKIE_BANNER_PHONE_BOTTOM_CLASS =
  'bottom-[calc(var(--sl-bottom-nav-height)+env(safe-area-inset-bottom,0rem))]';
export const COOKIE_BANNER_WIDE_BOTTOM_CLASS = 'bottom-3';

const PHONE_QUERY = `(max-width: ${BOTTOM_NAV_BREAKPOINT_PX - 0.02}px)`;

function subscribePhone(onChange: () => void) {
  if (typeof window.matchMedia !== 'function') return () => {};
  const query = window.matchMedia(PHONE_QUERY);
  query.addEventListener('change', onChange);
  return () => query.removeEventListener('change', onChange);
}
const phoneSnapshot = () =>
  typeof window.matchMedia === 'function' && window.matchMedia(PHONE_QUERY).matches;
const serverSnapshot = () => false;

export default function CookieBanner() {
  const [visible, setVisible] = useState(false);
  const bannerRef = useRef<HTMLDivElement>(null);
  const phone = useSyncExternalStore(subscribePhone, phoneSnapshot, serverSnapshot);

  useLayoutEffect(() => {
    const banner = bannerRef.current;
    if (!visible || !banner) return;
    const update = () => {
      const actions = document.querySelectorAll<HTMLElement>('[data-floating-action]');
      const actionTop = [...actions]
        .filter((action) => action.getClientRects().length > 0)
        .reduce((top, action) => Math.min(top, action.getBoundingClientRect().top), window.innerHeight);
      const navigation = document.querySelector<HTMLElement>('nav[aria-label="Mobile navigation"]');
      const navigationTop =
        navigation && navigation.getClientRects().length > 0
          ? navigation.getBoundingClientRect().top
          : window.innerHeight;
      banner.style.marginBottom = `${Math.max(0, navigationTop - actionTop)}px`;
      document.documentElement.style.setProperty(
        '--sl-cookie-banner-height',
        `${banner.offsetHeight + Math.max(0, navigationTop - actionTop)}px`
      );
    };
    update();
    const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(update) : null;
    observer?.observe(banner);
    window.addEventListener('resize', update);
    window.addEventListener('kindle:navigation-complete', update);
    return () => {
      observer?.disconnect();
      window.removeEventListener('resize', update);
      window.removeEventListener('kindle:navigation-complete', update);
      document.documentElement.style.removeProperty('--sl-cookie-banner-height');
    };
  }, [visible]);

  // The platform rules modal is the first-visit consent; the cookie banner
  // waits until it has been accepted so the two never stack on screen.
  useEffect(() => {
    let pending = false;
    try {
      pending = !window.localStorage.getItem(COOKIE_KEY);
    } catch {
      pending = true;
    }
    if (!pending) return;
    if (rulesAgreed()) {
      setVisible(true);
      return;
    }
    const show = () => setVisible(true);
    window.addEventListener(RULES_AGREED_EVENT, show);
    return () => window.removeEventListener(RULES_AGREED_EVENT, show);
  }, []);

  if (!visible) return null;

  const choose = (value: 'accepted' | 'declined') => {
    try {
      window.localStorage.setItem(COOKIE_KEY, value);
    } catch {
      /* storage blocked: hide for this page view */
    }
    setVisible(false);
  };
  const accept = () => choose('accepted');
  const decline = () => choose('declined');

  return (
    <div
      ref={bannerRef}
      data-cookie-banner=""
      data-placement={phone ? 'above-bottom-nav' : 'viewport-bottom'}
      role="region"
      aria-label="Cookie preferences"
      className={`fixed left-3 right-3 z-[var(--sl-z-toast)] max-w-[calc(var(--sl-space)*90)] rounded-md sl-floating sl-floating-opaque p-[calc(var(--sl-space-2)-var(--sl-edge-width))] md:right-auto md:p-3 ${phone ? COOKIE_BANNER_PHONE_BOTTOM_CLASS : COOKIE_BANNER_WIDE_BOTTOM_CLASS}`}
    >
      <div className="flex min-w-0 flex-col gap-1">
        <p className="whitespace-normal text-xs text-text-muted">
          We use cookies to improve your experience. See our{' '}
          <Link
            prefetch={false}
            href="/docs/privacy"
            className="text-text underline hover:text-text-muted"
          >
            privacy policy
          </Link>
        </p>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" onClick={decline} className="flex-1">
            Decline
          </Button>
          <Button variant="primary" size="sm" onClick={accept} className="flex-1">
            Accept
          </Button>
        </div>
      </div>
    </div>
  );
}

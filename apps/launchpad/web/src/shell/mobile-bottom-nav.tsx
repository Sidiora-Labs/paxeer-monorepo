'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAccount } from 'wagmi';
import { CircleEllipsis, CirclePlus, X, type LucideIcon } from 'lucide-react';
import { isRouteActive, routeForPath } from '@/core/app-foundation/navigation';
import { ModalDialog } from '@/core/app-foundation/modal-dialog';
import { useAccountSession } from '@/widgets/profile/use-account-session';
import { IconButton } from '@/ui/primitives';
import { cn } from '@/lib/utils';
import { createItem, isNavItemEligible, navItems, type NavItem } from './nav-config';

/**
 * Phone-only bottom navigation (< md, 768). From md up the icon rail is the
 * navigation. Five labelled targets share a translucent bar, with a neutral
 * current destination and an accent create disc. More opens the other destinations.
 */
export const BOTTOM_NAV_BREAKPOINT_PX = 768;

/** Bar height above the safe-area inset (`h-14`); fixed overlays below the breakpoint offset by this. */
export const BOTTOM_NAV_HEIGHT_PX = 56;

/** Bottom bar targets in order; ids refer to nav-config. */
export const BOTTOM_NAV_IDS = ['home', 'explore', 'create', 'bounties'] as const;

function configured(id: string): NavItem {
  const item = id === createItem.id ? createItem : navItems.find((entry) => entry.id === id);
  if (!item) throw new Error(`Navigation item ${id} is not configured`);
  return item;
}

export interface MoreSheetGroup {
  id: 'browse' | 'account' | 'resources';
  label: string;
  items: NavItem[];
}

/**
 * Destinations listed in the More sheet: every rail item that is not already a
 * bottom-bar target. Wallet and session requirements match the desktop rail.
 */
export function moreSheetGroups(walletReady: boolean, sessionReady: boolean): MoreSheetGroup[] {
  const pinned = new Set<string>(BOTTOM_NAV_IDS);
  const pick = (section: NavItem['section']) =>
    navItems.filter(
      (item) =>
        item.section === section &&
        !pinned.has(item.id) &&
        isNavItemEligible(item, walletReady, sessionReady)
    );
  const groups: MoreSheetGroup[] = [
    { id: 'browse', label: 'Browse', items: pick('main') },
    { id: 'account', label: 'Your account', items: pick('signed-in') },
    { id: 'resources', label: 'Resources', items: pick('bottom') },
  ];
  return groups.filter((group) => group.items.length > 0);
}

/** Create uses a circled plus inside its accent disc. */
const barIcon: Partial<Record<string, LucideIcon>> = { create: CirclePlus };

const targetClass = (active: boolean) =>
  cn(
    'flex min-h-touch-target-min min-w-0 w-full flex-col items-center justify-center gap-0.5 rounded-sm transition-colors duration-fast active:bg-surface-active',
    active ? 'text-text' : 'text-text-muted hover:text-text'
  );

export default function MobileBottomNav() {
  const pathname = usePathname() ?? '/';
  const { address, isConnected } = useAccount();
  const { session } = useAccountSession();
  const walletReady = isConnected && !!address;
  // The sheet records the path it was opened on, so any navigation closes it without an effect.
  const [openedOn, setOpenedOn] = useState<string | null>(null);
  const open = openedOn !== null && openedOn === pathname;
  const close = useCallback(() => setOpenedOn(null), []);

  useEffect(() => {
    if (!open) return;
    const wide = window.matchMedia(`(min-width: ${BOTTOM_NAV_BREAKPOINT_PX}px)`);
    const closeWhenWide = () => {
      if (wide.matches) close();
    };
    closeWhenWide();
    wide.addEventListener('change', closeWhenWide);
    return () => wide.removeEventListener('change', closeWhenWide);
  }, [open, close]);

  const groups = moreSheetGroups(walletReady, !!session);
  const moreActive = groups.some((group) =>
    group.items.some((item) => isRouteActive(pathname, item.href))
  );
  const targets = BOTTOM_NAV_IDS.map(configured);

  return (
    <>
      <nav
        aria-label="Mobile navigation"
        className="sl-mobile-destinations fixed inset-x-0 bottom-0 z-[var(--sl-z-rail)] border-t border-surface-3 md:hidden"
        style={{ paddingBottom: 'env(safe-area-inset-bottom, 0rem)' }}
      >
        <ul className="flex min-h-bottom-nav items-stretch px-2">
          {targets.map((item) => {
            const active = isRouteActive(pathname, item.href);
            const Icon = barIcon[item.id] ?? item.icon;
            return (
              <li key={item.id} className="flex min-w-0 flex-1 items-center">
                <Link prefetch={false}
                  href={item.href}
                  aria-label={item.label}
                  aria-current={active ? 'page' : undefined}
                  data-route-id={routeForPath(item.href)?.id}
                  className={targetClass(active)}
                >
                  <Icon aria-hidden="true" className={cn('size-6 shrink-0', item.id === 'create' && 'rounded-pill bg-accent text-on-accent')} strokeWidth={active ? 2.25 : 2} />
                  <span className="max-w-full truncate text-2xs font-medium">{item.label}</span>
                </Link>
              </li>
            );
          })}
          <li className="flex min-w-0 flex-1 items-center">
            <button
              type="button"
              aria-label="More"
              aria-haspopup="dialog"
              aria-expanded={open}
              onClick={() => setOpenedOn(open ? null : pathname)}
              className={targetClass(moreActive)}
            >
              <CircleEllipsis
                aria-hidden="true"
                className="size-6"
                strokeWidth={moreActive ? 2.25 : 2}
              />
              <span className="max-w-full truncate text-2xs font-medium">More</span>
            </button>
          </li>
        </ul>
      </nav>

      {open ? (
        <ModalDialog
          open={open}
          onClose={close}
          aria-label="More"
          className="sl-sheet sl-floating !inset-x-0 !bottom-0 !top-auto !m-0 !w-full !max-w-none flex-col rounded-t-[var(--sl-radius-sheet)] md:hidden"
          style={{ paddingBottom: 'env(safe-area-inset-bottom, 0rem)' }}
        >
          <div
            aria-hidden="true"
            className="mx-auto mt-2 h-1 w-8 shrink-0 rounded-pill bg-surface-3"
          />
          <div className="flex shrink-0 items-center justify-between px-4 pt-1">
            <h2 className="text-md font-semibold text-text">More</h2>
            <IconButton label="Close menu" icon={<X />} variant="ghost" size="md" onClick={close} />
          </div>
          <div className="flex min-h-0 flex-col gap-3 overflow-y-auto px-2 pb-3 pt-1">
            {groups.map((group) => (
              <section key={group.id} aria-label={group.label} className="flex flex-col">
                {group.id === 'browse' ? null : (
                  <h3 className="px-3 pb-1 text-2xs font-semibold uppercase text-text-faint">
                    {group.label}
                  </h3>
                )}
                <ul className="flex flex-col">
                  {group.items.map((item) => {
                    const active = isRouteActive(pathname, item.href);
                    const Icon = item.icon;
                    return (
                      <li key={item.id}>
                        <Link prefetch={false}
                          href={item.href}
                          aria-current={active ? 'page' : undefined}
                          data-route-id={routeForPath(item.href)?.id}
                          onClick={close}
                          className={cn(
                            'flex min-h-12 items-center py-2 gap-3 rounded-sm px-3 text-base font-semibold transition-colors duration-fast active:bg-surface-active',
                            active ? 'bg-surface-2 text-text' : 'text-text-muted hover:bg-surface-hover hover:text-text'
                          )}
                        >
                          <Icon aria-hidden="true" className="size-icon-lg shrink-0" />
                          <span>{item.label}</span>
                        </Link>
                      </li>
                    );
                  })}
                </ul>
              </section>
            ))}
          </div>
        </ModalDialog>
      ) : null}
    </>
  );
}

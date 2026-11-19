'use client';

import { useLayoutEffect, useRef } from 'react';
import { cn } from '@/lib/cn';
import { SvgIcon } from '@/components/ui/SvgIcon';
import type { AppRoute } from '@/widgets/shell/useAppRoute';

interface BottomNavProps {
  active: AppRoute;
  onNavigate: (route: AppRoute) => void;
}

const tabs: { id: AppRoute; label: string; icon: string }[] = [
  { id: 'portfolio', label: 'Wallet', icon: 'wallet' },
  { id: 'transactions', label: 'Activity', icon: 'clock' },
  { id: 'swap', label: 'Swap', icon: 'swap' },
  { id: 'discover', label: 'Discover', icon: 'compass' },
  { id: 'settings', label: 'Settings', icon: 'settings' },
];

function haptic() {
  if (typeof navigator !== 'undefined' && 'vibrate' in navigator) {
    navigator.vibrate(8);
  }
}

export function BottomNav({ active, onNavigate }: BottomNavProps) {
  const nav = useRef<HTMLElement>(null);
  useLayoutEffect(() => {
    const element = nav.current;
    const parent = element?.parentElement;
    if (!element || !parent) return;
    const reserve = () =>
      parent.style.setProperty(
        '--bottom-nav-height',
        `${element.getBoundingClientRect().height}px`
      );
    reserve();
    const observer = new ResizeObserver(reserve);
    observer.observe(element);
    return () => {
      observer.disconnect();
      parent.style.removeProperty('--bottom-nav-height');
    };
  }, []);
  return (
    <nav
      ref={nav}
      aria-label="Primary"
      className="pax-bottom-nav fixed bottom-0 inset-x-0 glass-nav safe-area-pb z-50"
    >
      <div className="pax-bottom-tabs grid grid-cols-5 items-center min-h-16 max-w-lg mx-auto px-2">
        {tabs.map(({ id, label, icon }) => {
          const isActive = active === id;
          return (
            <button
              key={id}
              type="button"
              aria-current={isActive ? 'page' : undefined}
              aria-label={label}
              onClick={() => {
                haptic();
                onNavigate(id);
              }}
              className={cn(
                'flex flex-col items-center justify-center gap-0.5 py-1 px-1 min-w-0 flex-1 transition-all press-scale',
                isActive ? 'text-pax-accent' : 'text-pax-muted'
              )}
            >
              <SvgIcon name={icon} className="w-5 h-5" />
              <span className="text-size-10 font-medium max-w-full">{label}</span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}

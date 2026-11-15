'use client';

/**
 * Shared atoms reused across token-detail cards.
 *
 * - {@link SectionLabel} — small uppercase header above each card
 * - {@link MetricCard}   — single-stat tile (Position, Market Cap, etc.)
 * - {@link InfoRow}      — label + value row used inside grouped cards
 * - {@link SocialPill}   — link pill for website / X / Telegram
 */

import type { ReactNode } from 'react';
import { openExternalUrl, validatedExternalUrl } from '@/lib/security/navigation';

export function SectionLabel({ text }: { text: string }) {
  return <p className="text-sm font-bold text-pax-muted uppercase tracking-[0.06em]">{text}</p>;
}

export function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-pax-surface rounded-xl px-4 py-3">
      <p className="text-xs text-pax-muted mb-1">{label}</p>
      <p className="text-base font-bold">{value}</p>
    </div>
  );
}

export function InfoRow({ label, value }: { label: ReactNode; value: ReactNode }) {
  return (
    <div className="flex items-center justify-between px-4 py-3">
      <span className="text-sm text-pax-muted">{label}</span>
      <span className="text-sm text-pax-light">{value}</span>
    </div>
  );
}

export function SocialPill({
  icon,
  label,
  href,
}: {
  icon: ReactNode;
  label: string;
  href: string;
}) {
  const destination = validatedExternalUrl(href);
  if (!destination) {
    return (
      <span
        aria-disabled="true"
        className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-pax-control text-xs font-medium text-pax-muted opacity-50"
      >
        {icon} {label}
      </span>
    );
  }
  return (
    <button
      type="button"
      onClick={() => openExternalUrl(destination.toString())}
      className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-pax-control text-xs font-medium text-pax-muted hover:text-pax-light transition-colors press-scale"
    >
      {icon} {label}
    </button>
  );
}

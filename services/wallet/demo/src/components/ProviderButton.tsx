'use client';

import Image from 'next/image';
import { Spinner } from './Spinner';

interface ProviderButtonProps {
  label: string;
  iconSrc: string;
  onClick: () => void;
  loading?: boolean;
  disabled?: boolean;
}

/**
 * One row in the OAuth provider stack. Monochrome surface, neutral border.
 * No accent color here — primary blue is reserved for the user's chosen action
 * (the "Send magic link" CTA, "Send Transaction" CTA, etc.).
 */
export function ProviderButton({
  label,
  iconSrc,
  onClick,
  loading,
  disabled,
}: ProviderButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={loading || disabled}
      aria-busy={loading || undefined}
      className="button provider"
    >
      <span className="provider-icon">
        <Image src={iconSrc} alt="" width={24} height={24} className="icon" />
      </span>
      <span className="provider-label">{label}</span>
      <span className="spinner-slot">{loading && <Spinner label="Signing in…" />}</span>
    </button>
  );
}

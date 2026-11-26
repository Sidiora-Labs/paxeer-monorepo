'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { ModalDialog } from '@/core/app-foundation/modal-dialog';

/** localStorage key recording that the visitor accepted the platform rules. */
export const RULES_AGREED_KEY = 'sidiora_rules_agreed_v1';
/** Window event fired once the visitor accepts; the cookie banner waits for it. */
export const RULES_AGREED_EVENT = 'sidiora:rules-agreed';

/** True once the visitor has accepted the platform rules on this device. */
export function rulesAgreed(): boolean {
  try {
    return Boolean(window.localStorage.getItem(RULES_AGREED_KEY));
  } catch {
    return false;
  }
}

const RULES: { text: React.ReactNode }[] = [
  {
    text: (
      <>
        User-generated tokens are <strong className="text-text">not endorsed</strong> by Sidiora.
        Always do your own research before buying, selling, or holding any Digital Asset.
      </>
    ),
  },
  {
    text: (
      <>
        Smart contracts carry inherent risk. You may experience{' '}
        <span className="text-danger">partial or total loss of funds</span>. Use only what you can
        afford to lose.
      </>
    ),
  },
  {
    text: (
      <>
        Market manipulation, pump-and-dump schemes, wash trading, and deceptive conduct are{' '}
        <strong className="text-text">strictly prohibited</strong>.
      </>
    ),
  },
  {
    text: (
      <>
        No harassment, hate speech, impersonation, or synthetic/AI-generated voice misuse in the
        Voice Chat Feature.
      </>
    ),
  },
  {
    text: (
      <>
        Access from sanctioned or prohibited jurisdictions (including Russia, Iran, North Korea,
        Cuba, and Syria) is not permitted.
      </>
    ),
  },
  {
    text: (
      <>
        By continuing, you confirm you have read and accept our{' '}
        <Link
          prefetch={false}
          href="/docs/terms"
          className="text-accent underline hover:opacity-80 transition"
        >
          Terms of Service
        </Link>{' '}
        and{' '}
        <Link
          prefetch={false}
          href="/docs/privacy"
          className="text-accent underline hover:opacity-80 transition"
        >
          Privacy Policy
        </Link>
        .
      </>
    ),
  },
];

export default function PlatformRulesModal() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (!rulesAgreed()) setVisible(true);
  }, []);

  if (!visible) return null;

  function handleAgree() {
    try {
      window.localStorage.setItem(RULES_AGREED_KEY, Date.now().toString());
    } catch {
      /* storage blocked: accept for this page view only */
    }
    setVisible(false);
    window.dispatchEvent(new Event(RULES_AGREED_EVENT));
  }

  return (
    <ModalDialog
      open={visible}
      closeOnScrim={false}
      aria-labelledby="platform-rules-title"
      aria-describedby="platform-rules-summary"
      // Acceptance is required, so Escape must not dismiss it.
      onKeyDown={(event) => {
        if (event.key === 'Escape') {
          event.preventDefault();
          event.stopPropagation();
        }
      }}
      className="max-w-notice-max flex-col overflow-hidden rounded-xl bg-surface-1"
    >
      <div data-slot="dialog-body" className="min-h-0 flex-1 overflow-y-auto overscroll-contain">
        <header data-slot="dialog-banner" className="bg-surface-2 px-6 pb-7 pt-8 text-center">
          <p className="text-sm font-bold text-text-muted">
            Launch, earn, <span className="text-accent">enjoy</span>
          </p>
          <p className="mt-1 text-2xl font-bold leading-[var(--sl-leading-none)] tracking-tight text-text">
            Sidiora
          </p>
        </header>

        <div className="px-6 pt-5">
          <div className="mb-5 flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-accent-soft flex items-center justify-center flex-shrink-0">
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                className="size-icon-md text-accent"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
              </svg>
            </div>
            <div>
              <h2 id="platform-rules-title" className="font-semibold text-md text-text leading-tight">
                Platform rules
              </h2>
              <p id="platform-rules-summary" className="text-2xs text-text-faint mt-0.5">
                Please review before continuing
              </p>
            </div>
          </div>

          <ul className="pb-5 space-y-3">
            {RULES.map((rule, i) => (
              <li key={i} className="flex items-start gap-3">
                <span
                  aria-hidden="true"
                  className="mt-1.5 block size-1.5 shrink-0 rounded-full bg-accent"
                />
                <p className="min-w-0 text-xs text-text-muted leading-[var(--sl-leading-prose)]">
                  {rule.text}
                </p>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <footer data-slot="dialog-footer" className="shrink-0 bg-surface-1 px-6 pb-5 pt-3">
        <button
          type="button"
          onClick={handleAgree}
          className="w-full rounded-xl bg-accent py-3 text-sm font-bold text-on-accent transition-colors hover:bg-accent-hover active:bg-accent-active"
        >
          I understand and agree
        </button>

        <p className="mt-3 text-center text-2xs text-text-faint">
          You'll only see this once. All policies are in the site footer
        </p>
      </footer>
    </ModalDialog>
  );
}

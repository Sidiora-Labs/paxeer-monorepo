import Link from 'next/link';
import type { ReactNode } from 'react';
import { DOC_LINKS } from './doc-links';
import './docs.css';

export default function DocsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="sl-documents bg-canvas">
      <div className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6">
        <div className="mb-7">
          <Link
            prefetch={false}
            href="/"
            className="inline-flex items-center gap-1.5 text-xs text-text-faint transition-colors duration-fast hover:text-text focus-visible:outline-2 focus-visible:outline-focus"
          >
            <svg aria-hidden="true" width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path
                d="M9 2L4 7l5 5"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
            Back to Sidiora
          </Link>
        </div>

        <div className="flex flex-col lg:flex-row gap-8">
          <aside className="lg:w-52 flex-shrink-0">
            <nav
              aria-label="Documentation"
              className="lg:sticky lg:top-[calc(var(--shell-top)+var(--sl-space-4))]"
            >
              <p className="text-2xs text-text-faint font-semibold uppercase tracking-wider mb-3 px-2">
                <Link
                  prefetch={false}
                  href="/docs"
                  className="inline-flex items-center hover:text-text"
                >
                  Documentation
                </Link>
              </p>
              <div className="space-y-0.5">
                {DOC_LINKS.map(({ label, href }) => (
                  <Link
                    prefetch={false}
                    key={href}
                    href={href}
                    className="flex items-center text-xs font-medium text-text-muted hover:text-text py-1.5 px-2 rounded-pill hover:bg-surface-2 transition-colors duration-fast"
                  >
                    {label}
                  </Link>
                ))}
              </div>
            </nav>
          </aside>

          <div className="@container/legal flex-1 min-w-0">{children}</div>
        </div>
      </div>
    </div>
  );
}

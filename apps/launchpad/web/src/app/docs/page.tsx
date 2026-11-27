import type { Metadata } from 'next';
import Link from 'next/link';
import { DOC_LINKS } from './doc-links';

export const metadata: Metadata = { title: 'Documentation — Sidiora' };

/** /docs index: every documentation page with a one-line summary. */
export default function DocsIndexPage() {
  return (
    <article className="prose-legal">
      <h1>Documentation</h1>
      <p className="updated">Policies and guidelines for using Sidiora</p>
      <ul className="flex flex-col">
        {DOC_LINKS.map(({ label, href, summary }) => (
          <li key={href} className="sl-hairline">
            <Link prefetch={false} href={href} className="block px-4 py-3 transition-colors duration-fast hover:bg-surface-1">
              <span className="block text-sm font-semibold text-text">{label}</span>
              <span className="mt-0.5 block text-xs text-text-muted">{summary}</span>
            </Link>
          </li>
        ))}
      </ul>
    </article>
  );
}

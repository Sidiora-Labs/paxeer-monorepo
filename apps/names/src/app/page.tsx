import { shellDependencies } from './_pns/[label]/[[...path]]/route.ts';
import { serve } from '../lib/serve.ts';
import ThemeToggle from '../components/theme-toggle.tsx';

export const dynamic = 'force-dynamic';

export default async function StatusPage({
  searchParams,
}: {
  searchParams: Promise<{ name?: string | string[] }>;
}) {
  const query = await searchParams;
  const name = typeof query.name === 'string' ? query.name.trim().toLowerCase() : '';
  let response: Response | undefined;
  let error = '';
  if (name) {
    if (!/^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]\.pax$/.test(name)) {
      error = 'Enter a valid .pax name.';
    } else {
      try {
        const domain = process.env.NAMES_SHELL_DOMAIN;
        if (!domain) throw new Error('Shell configuration unavailable');
        response = await serve(
          new Request(`https://${name}.${domain}/`, { method: 'HEAD' }),
          shellDependencies()
        );
        if (!response.ok) error = `Site verification failed (HTTP ${response.status}).`;
      } catch {
        error = 'Shell configuration unavailable.';
      }
    }
  }
  const verified =
    response?.ok === true &&
    response.headers.has('X-PNS-Manifest') &&
    response.headers.has('X-PNS-Height');
  return (
    <div className="shell">
      <a className="skip-link" href="#main-content">Skip to verification</a>
      <header className="shell-header">
        <a className="brand" href="/" aria-label="Paxeer Names home">
          <span className="brand-mark" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M6 5.5h7.2a4.3 4.3 0 1 1 0 8.6H6M6 5.5v13M6 5.5l5.7 5.7" />
            </svg>
          </span>
          <span>Paxeer Names shell</span>
        </a>
        <ThemeToggle />
      </header>

      <main className="shell-main" id="main-content">
        <section className="hero" aria-labelledby="page-title">
          <p className="eyebrow">Published sites</p>
          <h1 id="page-title">Verify a .pax site before you visit</h1>
          <p className="hero-copy">
            Check the name resolution and storage proof for a published site. The shell only serves content after both proofs are valid.
          </p>
        </section>

        <div className="surface-grid">
          <section className="card" aria-labelledby="verify-heading">
            <div className="card-heading">
              <h2 id="verify-heading">Site verification</h2>
              <p>Enter a published name to inspect its latest manifest and proven height.</p>
            </div>
            <form className="verification-form" method="get">
              <div className="field">
                <label htmlFor="name">.pax name</label>
                <input
                  id="name"
                  name="name"
                  type="text"
                  inputMode="url"
                  autoComplete="url"
                  spellCheck={false}
                  placeholder="example.pax"
                  defaultValue={name}
                  pattern="[a-z0-9][a-z0-9-]{1,61}[a-z0-9]\.pax"
                  aria-describedby="name-hint"
                  aria-invalid={Boolean(error)}
                  required
                />
                <span className="field-hint" id="name-hint">Use 3–63 lowercase letters, numbers or hyphens, followed by .pax.</span>
              </div>
              <button className="primary-action" type="submit">Verify site</button>
            </form>

            {name && (
              <section className="result" aria-label="Verification result">
                <h2 className="result-title">{name}</h2>
                <p className="status" data-state={verified ? 'verified' : error ? 'error' : 'pending'} role="status" aria-live="polite">
                  <svg className="status-icon" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    {verified ? <path d="m5 12 4 4L19 6" /> : <path d="M12 7v5m0 4h.01" />}
                  </svg>
                  <span>{verified ? 'Verified storage proof and site content' : error || 'This site is not verified.'}</span>
                </p>
                <dl className="facts">
                  <div className="fact">
                    <dt>Manifest digest</dt>
                    <dd data-monospace="true">{response?.headers.get('X-PNS-Manifest') ?? 'Unavailable'}</dd>
                  </div>
                  <div className="fact">
                    <dt>Proven height</dt>
                    <dd data-monospace="true">{response?.headers.get('X-PNS-Height') ?? 'Unavailable'}</dd>
                  </div>
                  <div className="fact">
                    <dt>Resolver address</dt>
                    <dd data-monospace="true">{process.env.NAMES_RESOLVER ?? 'Not configured'}</dd>
                  </div>
                </dl>
              </section>
            )}
          </section>

          <aside className="card" aria-labelledby="proof-heading">
            <div className="card-heading">
              <h2 id="proof-heading">What gets checked</h2>
              <p>Every response is tied to the published name and its proof.</p>
            </div>
            <ul className="principles">
              <li>The resolver returns a verified manifest digest and proven height.</li>
              <li>The manifest belongs to the requested .pax name.</li>
              <li>Each file is fetched by digest and checked before it is served.</li>
            </ul>
          </aside>
        </div>
      </main>

      <footer className="shell-footer">
        <span>Content is served only after storage-proof verification.</span>
        <span>© Paxeer Names</span>
      </footer>
    </div>
  );
}

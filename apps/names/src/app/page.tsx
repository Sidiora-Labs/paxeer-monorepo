import { shellDependencies } from './_pns/[label]/[[...path]]/route.ts';
import { serve } from '../lib/serve.ts';

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
    <main>
      <h1>Paxeer Names shell</h1>
      <p>Check a published site and its storage proof.</p>
      <form method="get">
        <label htmlFor="name">Name</label>{' '}
        <input id="name" name="name" placeholder="example.pax" defaultValue={name} required />{' '}
        <button type="submit">Verify site</button>
      </form>
      {name && (
        <section aria-label="Verification result">
          <h2>{name}</h2>
          <p role="status" style={{ color: verified ? '#166534' : '#374151' }}>
            {verified ? 'Verified storage proof and site content' : 'Not verified'}
          </p>
          {error && <p role="alert">{error}</p>}
          <dl>
            <dt>Manifest digest</dt>
            <dd>{response?.headers.get('X-PNS-Manifest') ?? 'Unavailable'}</dd>
            <dt>Proven height</dt>
            <dd>{response?.headers.get('X-PNS-Height') ?? 'Unavailable'}</dd>
            <dt>Resolver address</dt>
            <dd>{process.env.NAMES_RESOLVER ?? 'Not configured'}</dd>
          </dl>
        </section>
      )}
    </main>
  );
}

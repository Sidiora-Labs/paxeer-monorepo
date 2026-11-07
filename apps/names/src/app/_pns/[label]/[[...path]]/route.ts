import { resolveContent, tendermintHeaders } from '@paxeer/names';
import type { NextRequest } from 'next/server.js';
import { nameForHost, serve, type ServeDependencies } from '../../../../lib/serve.ts';

export const runtime = 'nodejs';
export const dynamic = 'force-dynamic';

function required(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing ${name}`);
  return value;
}

export function shellDependencies(): ServeDependencies {
  const rpcUrl = required('NAMES_RPC_URL');
  const resolver = required('NAMES_RESOLVER');
  const headers = tendermintHeaders(required('NAMES_CONSENSUS_RPC_URL'));
  return {
    resolve: (name) => resolveContent(name, { rpcUrl, resolver, headers }),
    bucketBaseUrl: required('NAMES_BUCKET_BASE_URL'),
    fetch: globalThis.fetch,
  };
}

export async function GET(
  request: NextRequest,
  context: { params: Promise<{ label: string; path?: string[] }> }
): Promise<Response> {
  const { label, path = [] } = await context.params;
  const domain = required('NAMES_SHELL_DOMAIN');
  const name = nameForHost(request.headers.get('host') ?? request.nextUrl.host, domain);
  if (name !== `${label}.pax`) return Response.json({ code: 'invalid_site_host' }, { status: 404 });
  const url = new URL(request.url);
  url.pathname = `/${path.map(encodeURIComponent).join('/')}`;
  return serve(
    new Request(url, { method: request.method, headers: request.headers }),
    shellDependencies()
  );
}

export const HEAD = GET;

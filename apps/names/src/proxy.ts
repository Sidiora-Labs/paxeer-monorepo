import { NextRequest, NextResponse } from 'next/server.js';
import { GET } from './app/_pns/[label]/[[...path]]/route.ts';
import { nameForHost } from './lib/serve.ts';

export async function proxy(request: NextRequest): Promise<Response> {
  const domain = process.env.NAMES_SHELL_DOMAIN;
  const name = domain ? nameForHost(request.headers.get('host') ?? '', domain) : null;
  if (!name) return NextResponse.next();
  const label = name.slice(0, -4);
  const url = request.nextUrl.clone();
  const path = url.pathname.slice(1).split('/').map(decodeURIComponent);
  url.pathname = `/_pns/${label}${url.pathname}`;
  // Next excludes underscore folders from route discovery; dispatch the rewritten URL here.
  return GET(
    new NextRequest(url.toString(), { method: request.method, headers: request.headers }),
    {
      params: Promise.resolve({ label, path }),
    }
  );
}

export const config = { matcher: ['/:path*'] };

import { NextRequest, NextResponse } from 'next/server.js';
import { GET, shellDependencies } from './app/_pns/[label]/[[...path]]/route.ts';
import { nameForHost, serve } from './lib/serve.ts';

export async function proxy(request: NextRequest): Promise<Response> {
  const domain = process.env.NAMES_SHELL_DOMAIN;
  const name = domain ? nameForHost(request.headers.get('host') ?? '', domain) : null;
  if (!name) return NextResponse.next();
  const label = name.slice(0, -4);
  const url = request.nextUrl.clone();
  let path: string[];
  try {
    path = url.pathname.slice(1).split('/').map(decodeURIComponent);
  } catch {
    return serve(
      new Request(request.url, { method: request.method, headers: request.headers }),
      shellDependencies()
    );
  }
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

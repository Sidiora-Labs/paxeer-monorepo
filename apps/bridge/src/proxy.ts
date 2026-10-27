import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

import { bridgeConfig } from './lib/bridge/config';
import { contentSecurityPolicy } from './lib/bridge/csp';

export function proxy(_request: NextRequest) {
  const response = NextResponse.next();
  response.headers.set('X-Content-Type-Options', 'nosniff');
  response.headers.set('X-Frame-Options', 'DENY');
  response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
  response.headers.set('Permissions-Policy', 'camera=(), microphone=(), geolocation=()');
  response.headers.set('Content-Security-Policy', contentSecurityPolicy(bridgeConfig));
  return response;
}

export const config = {
  matcher: [ '/((?!_next/static|_next/image|favicon.ico).*)' ],
};

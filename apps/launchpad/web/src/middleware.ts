import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';
import { PublicShareError, readPublicShare } from '@/widgets/pnl/share-transport';
import {
  BackendOriginError,
  browserConnectOrigins,
  browserImageOrigins,
  walletParentOrigins,
} from '@/core/backend-origins';
import { getPaxeerRpcUrls, RpcConfigurationError } from '@/core/rpc';

/**
 * Runtime CSP middleware.
 *
 * Content-Security-Policy is assembled here rather than in next.config.ts so
 * the frame-ancestors directive follows NEXT_PUBLIC_WALLET_IFRAME_ORIGIN at
 * request time. Every host list comes from core/backend-origins.ts, the same
 * resolver the SDK URL builders, stream clients, explorer client and embedded
 * wallet read, so connect-src is exactly the set of origins the bundle can
 * contact: no fallback host is listed that the bundle would not use, and no
 * host the bundle uses is missing.
 */
function buildCsp(): { policy: string; walletOrigins: string[] } {
  let connectSrc: string[] = [];
  let plainHttpImageHosts: string[] = [];
  let walletOrigins: string[] = [];
  try {
    walletOrigins = walletParentOrigins();
    getPaxeerRpcUrls();
    connectSrc = browserConnectOrigins();
    // `https:` already admits every https image/media host; a plain-http backend (local stack) is listed explicitly.
    plainHttpImageHosts = browserImageOrigins().filter(origin => origin.startsWith('http:'));
  } catch (error) {
    if (!(error instanceof BackendOriginError) && !(error instanceof RpcConfigurationError)) throw error;
    connectSrc = [];
    plainHttpImageHosts = [];
    walletOrigins = [];
  }
  const walletList = walletOrigins.length > 0 ? ' ' + walletOrigins.join(' ') : '';

  const directives = [
    "default-src 'self'",
    "script-src 'self' 'unsafe-inline' blob:",
    "style-src 'self' 'unsafe-inline'",
    // Token logos and banners are on the metadata host; remote IPFS / avatar hosts are https.
    ['img-src', "'self'", 'data:', 'blob:', 'https:', ...plainHttpImageHosts].join(' '),
    "font-src 'self' data:",
    ['connect-src', "'self'", ...connectSrc].join(' '),
    "worker-src 'self' blob:",
    `child-src 'self' blob:${walletList}`,
    `frame-src 'self' blob:${walletList}`,
    // Livestream playback and uploaded media are served over https.
    ['media-src', "'self'", 'blob:', 'https:', ...plainHttpImageHosts].join(' '),
    "object-src 'none'",
    "base-uri 'self'",
    `frame-ancestors 'self'${walletList}`,
  ];

  return { policy: directives.join('; '), walletOrigins };
}

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  let destination: string | undefined;
  if (pathname === '/new') destination = '/explore?preset=new';
  const token = /^\/token\/(0x[0-9a-fA-F]{40})$/.exec(pathname);
  if (token) destination = `/coin/${token[1].toLowerCase()}`;
  const referral = /^\/r\/([A-Za-z0-9_-]{1,128})$/.exec(pathname);
  if (referral) {
    const cards = request.nextUrl.searchParams.getAll('cardId');
    if (cards.length === 0) destination = `/?ref=${encodeURIComponent(referral[1])}`;
    else if (cards.length === 1 && /^[A-Za-z0-9_-]{1,128}$/.test(cards[0])) {
      destination = `/pnl/${encodeURIComponent(cards[0])}?ref=${encodeURIComponent(referral[1])}`;
    }
  }
  const publicCard = /^\/pnl\/([^/]+)$/.exec(pathname);
  let missingCard = false;
  if (publicCard) {
    try {
      await readPublicShare(publicCard[1]);
    } catch (error) {
      missingCard = error instanceof PublicShareError && error.status === 404;
    }
  }
  const response = destination
    ? NextResponse.redirect(new URL(destination, request.url))
    : NextResponse.next({ status: missingCard ? 404 : 200 });

  // Baseline security headers applied to EVERY route (incl. the charting
  // library branch below). HSTS forces TLS; nosniff blocks MIME-confusion
  // XSS; Referrer-Policy avoids leaking full URLs cross-origin.
  response.headers.set(
    'Strict-Transport-Security',
    'max-age=15552000; includeSubDomains; preload',
  );
  response.headers.set('X-Content-Type-Options', 'nosniff');
  response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');

  response.headers.set('Permissions-Policy', 'clipboard-write=(self), clipboard-read=(self), unload=(self)');

  // Charting library gets a permissive CSP (TradingView requirement)
  if (pathname.startsWith('/charting_library')) {
    response.headers.set(
      'Content-Security-Policy',
      "default-src 'self' 'unsafe-eval' 'unsafe-inline' blob: data:; worker-src 'self' blob:; child-src 'self' blob:",
    );
    return response;
  }

  const { policy, walletOrigins } = buildCsp();
  response.headers.set('Content-Security-Policy', policy);

  // Only set X-Frame-Options if no wallet origin is configured
  // (CSP frame-ancestors supersedes X-Frame-Options per spec)
  if (walletOrigins.length === 0) {
    response.headers.set('X-Frame-Options', 'SAMEORIGIN');
  }

  return response;
}

export const config = {
  matcher: [
    // Match all paths except static files and Next.js internals
    '/((?!_next/static|_next/image|favicon.ico|icon.svg).*)',
  ],
};

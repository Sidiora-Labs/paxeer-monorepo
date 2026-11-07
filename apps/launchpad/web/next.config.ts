import path from 'path';
import type { NextConfig } from 'next';
import { withSentryConfig } from '@sentry/nextjs';
import bundleAnalyzer from '@next/bundle-analyzer';
import { backendConfigurationProblems, browserImageOrigins } from './src/core/backend-origins';
import { allowedDevOrigins, backendRewrites, TRANSPILE_PACKAGES } from './src/core/next-build-config';

// Run `ANALYZE=true pnpm --filter @sidiora/web build` to emit treemap reports
// (.next/analyze/*.html) for per-route JS budgeting. No-op for normal builds.
const withBundleAnalyzer = bundleAnalyzer({ enabled: process.env.ANALYZE === 'true' });

// CSP headers are now set at RUNTIME via src/middleware.ts so that
// NEXT_PUBLIC_WALLET_IFRAME_ORIGIN is always read fresh (not baked at build time).

/**
 * Remote image hosts: the http(s) origins of core/backend-origins.ts
 * browserImageOrigins(), the same resolver the SDK URL builders and the CSP
 * use, so token logos, avatars and banners served by metadata/gateway are
 * optimisable exactly when the bundle requests them, with or without explicit
 * variables, and no host is listed that the bundle never uses.
 */
function backendImageHosts(): Array<{ protocol: 'https' | 'http'; hostname: string; port?: string }> {
  const problems = backendConfigurationProblems();
  if (problems.length && process.env.NODE_ENV === 'production') {
    // The deployed page renders the configuration error state; say why at build time too.
    console.error(`[sidiora] backend origins are not configured: ${problems.map(problem => problem.message).join('; ')}`);
  }
  const canonical = process.env.NEXT_PUBLIC_CANONICAL_ORIGIN;
  const origins = [...browserImageOrigins()];
  if (canonical) { try { origins.push(new URL(canonical).origin); } catch {} }
  const seen = new Set<string>();
  const out: Array<{ protocol: 'https' | 'http'; hostname: string; port?: string }> = [];
  for (const origin of origins) {
    if (seen.has(origin)) continue;
    seen.add(origin);
    const url = new URL(origin);
    out.push({ protocol: url.protocol === 'http:' ? 'http' : 'https', hostname: url.hostname, ...(url.port ? { port: url.port } : {}) });
  }
  return out;
}

const nextConfig: NextConfig = {
  rewrites: () => backendRewrites(process.env),
  // Builds can target a separate directory so `next build` never clobbers the running dev server's .next.
  distDir: process.env.NEXT_DIST_DIR ?? '.next',
  allowedDevOrigins: allowedDevOrigins(process.env),
  // `standalone` is for the Railway/Docker runtime (`node .next/standalone/
  // server.js`). On Vercel it is unnecessary and its file-tracing follows
  // pnpm's symlinked node_modules to absolute paths that break the deploy
  // (styled-jsx ENOENT), so disable it there (Vercel sets VERCEL=1 at build).
  output: process.env.VERCEL ? undefined : 'standalone',
  compress: true,

  // This app lives at apps/launchpad/web inside a pnpm workspace whose root is
  // apps/launchpad/. Next's NFT auto-detection picks the wrong tracing root and emits
  // symlinks that escape to an absolute /node_modules/.pnpm/... path, which
  // makes `vercel deploy --prebuilt` die with `ENOENT lstat .../styled-jsx`
  // (next.js#73648). Pinning the trace root to the workspace root keeps every
  // traced symlink relative and inside the uploaded artifact.
  //
  // `next dev --turbopack` resolves imports only inside that same root, and
  // Next forces turbopack.root to equal outputFileTracingRoot. The create flow
  // imports services/launchpad/security/control/cap-admission.ts, so in dev both
  // roots are the repository root; production builds (webpack) keep apps/launchpad/.
  outputFileTracingRoot: process.env.NODE_ENV === 'development' ? path.join(__dirname, '..', '..', '..') : path.join(__dirname, '..'),
  turbopack: { root: process.env.NODE_ENV === 'development' ? path.join(__dirname, '..', '..', '..') : path.join(__dirname, '..') },
  outputFileTracingIncludes: {
    '/*': [
      '../node_modules/.pnpm/sharp@*/node_modules/@img/sharp-libvips-*/versions.json',
      '../node_modules/.pnpm/sharp@*/node_modules/@img/sharp-libvips-*/package.json',
    ],
  },

  images: {
    remotePatterns: [
      ...backendImageHosts(),
      { protocol: 'https', hostname: 'api.dicebear.com' },
      { protocol: 'https', hostname: 'ipfs.io' },
      { protocol: 'https', hostname: 'gateway.pinata.cloud' },
      { protocol: 'https', hostname: 'cloudflare-ipfs.com' },
      { protocol: 'https', hostname: 'arweave.net' },
      { protocol: 'https', hostname: 'nftstorage.link' },
    ],
    // Remote SVG requires a validated raster derivative; local static icons bypass optimization.
    dangerouslyAllowSVG: false,
    maximumResponseBody: 8388608,
    contentDispositionType: 'attachment',
    contentSecurityPolicy: "default-src 'self'; script-src 'none'; style-src 'unsafe-inline'; sandbox;",
  },

  experimental: {
    serverActions: {},
    mdxRs: true,
    imgOptConcurrency: 2,
    imgOptMaxInputPixels: 16777216,
    imgOptTimeoutInSeconds: 7,
  },

  transpilePackages: [...TRANSPILE_PACKAGES],
};

// Wrap with Sentry. Source-map upload only runs when SENTRY_AUTH_TOKEN is set
// (CI/release builds); locally it is a transparent no-op. `tunnelRoute` proxies
// Sentry ingestion through our own origin so ad-blockers don't drop events.
export default withSentryConfig(withBundleAnalyzer(nextConfig), {
  org: process.env.SENTRY_ORG,
  project: process.env.SENTRY_PROJECT,
  authToken: process.env.SENTRY_AUTH_TOKEN,
  silent: !process.env.CI,
  widenClientFileUpload: true,
  tunnelRoute: '/monitoring',
  disableLogger: true,
  automaticVercelMonitors: false,
  sourcemaps: {
    disable: !process.env.SENTRY_AUTH_TOKEN,
  },
});

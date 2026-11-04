/* ══════════════════════════════════════════════════════════════════════
   Sentry — client config

   Reads `NEXT_PUBLIC_SENTRY_DSN` at runtime. If unset, Sentry stays
   inert and the app's local error UI is unaffected. We deliberately
   do NOT enable Replay or Profiling by default to keep the bundle
   light — turn them on per-environment if/when you want them.

   The `beforeSend` hook performs a final sanitisation pass:
     · Strips wallet addresses, private keys, and seed phrases.
     · Drops AbortError frames (benign, noisy).
   ══════════════════════════════════════════════════════════════════════ */

import * as Sentry from "@sentry/nextjs";
import { sanitizeSentryEvent } from "./src/core/errors/sentrySanitizer";

const DSN = process.env.NEXT_PUBLIC_SENTRY_DSN;
const ENV = process.env.NEXT_PUBLIC_SENTRY_ENV || process.env.NODE_ENV || "production";
const RELEASE = process.env.NEXT_PUBLIC_RELEASE;

if (DSN) {
  Sentry.init({
    dsn: DSN,
    environment: ENV,
    release: RELEASE,

    // Performance + replay are opt-in. Keep cheap defaults.
    tracesSampleRate: 0,
    replaysSessionSampleRate: 0,
    replaysOnErrorSampleRate: 0,

    // Lock down PII — we never want to ship wallet addresses or
    // private keys to a third-party. Our own `reportError` already
    // attaches what we want.
    sendDefaultPii: false,

    // Drop benign noise BEFORE it leaves the browser.
    ignoreErrors: [
      // ResizeObserver chrome warnings
      "ResizeObserver loop limit exceeded",
      "ResizeObserver loop completed with undelivered notifications.",
      // Cross-origin opaque errors — useless to triage
      "Script error.",
      // wagmi/viem expected user actions
      "User rejected the request",
      "UserRejectedRequestError",
    ],

    beforeSend: sanitizeSentryEvent,
  });
}

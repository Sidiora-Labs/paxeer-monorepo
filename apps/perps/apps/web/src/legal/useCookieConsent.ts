"use client";

import { useEffect, useLayoutEffect, useSyncExternalStore } from "react";
import * as Sentry from "@sentry/nextjs";

type BeforeSend = NonNullable<Parameters<typeof Sentry.init>[0]["beforeSend"]>;
type ErrorEvent = Parameters<BeforeSend>[0];
type EventHint = Parameters<BeforeSend>[1];

export const COOKIE_CONSENT_STORAGE_KEY = "hyperpax_cookie_consent";

export type CookieConsentValue = "accepted" | "declined" | null;

export interface CookieConsent {
  value: CookieConsentValue;
  loading: boolean;
  accept: () => void;
  decline: () => void;
}

interface CookieConsentStore extends CookieConsent {
  hydrate: () => void;
}

type ConsentListener = () => void;
const listeners = new Set<ConsentListener>();

function store(value: Exclude<CookieConsentValue, null>): void {
  try {
    localStorage.setItem(COOKIE_CONSENT_STORAGE_KEY, value);
  } catch {
    // The choice still applies for this visit when browser storage is unavailable.
  }
}

const initialState: CookieConsentStore = {
  value: null,
  loading: true,
  hydrate() {
    if (!state.loading) return;
    let value: CookieConsentValue = null;
    try {
      const stored = localStorage.getItem(COOKIE_CONSENT_STORAGE_KEY);
      if (stored === "accepted" || stored === "declined") value = stored;
    } catch {
      // An unavailable preference leaves the notice open for a choice.
    }
    cookieConsentStore.setState({ value, loading: false });
  },
  accept() {
    store("accepted");
    cookieConsentStore.setState({ value: "accepted", loading: false });
  },
  decline() {
    store("declined");
    cookieConsentStore.setState({ value: "declined", loading: false });
  },
};

let state = initialState;

export const cookieConsentStore = {
  getState(): CookieConsentStore {
    return state;
  },
  setState(next: Partial<CookieConsentStore>): void {
    state = { ...state, ...next };
    for (const listener of listeners) listener();
  },
  subscribe(listener: ConsentListener): () => void {
    listeners.add(listener);
    return () => listeners.delete(listener);
  },
};

export function useCookieConsent(): CookieConsent {
  const consent = useSyncExternalStore(
    cookieConsentStore.subscribe,
    cookieConsentStore.getState,
    () => initialState,
  );

  (typeof window === "undefined" ? useEffect : useLayoutEffect)(() => {
    cookieConsentStore.getState().hydrate();
  }, []);

  return consent;
}

export async function initializeErrorReporting(
  runtime: "browser" | "server",
  permitted: () => boolean = () => true,
) {
  const dsn =
    runtime === "browser"
      ? process.env.NEXT_PUBLIC_SENTRY_DSN
      : process.env.SENTRY_DSN || process.env.NEXT_PUBLIC_SENTRY_DSN;
  if (!dsn || !permitted()) return null;

  const options = {
    dsn,
    environment:
      (runtime === "browser"
        ? process.env.NEXT_PUBLIC_SENTRY_ENV
        : process.env.SENTRY_ENV) ||
      process.env.NODE_ENV ||
      "production",
    release: process.env.NEXT_PUBLIC_RELEASE,
    tracesSampleRate: 0,
    sendDefaultPii: false,
  };
  if (runtime === "browser") {
    Sentry.init({
      ...options,
      replaysSessionSampleRate: 0,
      replaysOnErrorSampleRate: 0,
      ignoreErrors: [
        "ResizeObserver loop limit exceeded",
        "ResizeObserver loop completed with undelivered notifications.",
        "Script error.",
        "User rejected the request",
        "UserRejectedRequestError",
      ],
      beforeSend: (event: ErrorEvent, hint: EventHint) =>
        permitted() ? sanitizeSentryEvent(event, hint) : null,
    });
  } else {
    Sentry.init(options);
  }
  return Sentry;
}

export function startConsentErrorReporting(): () => void {
  let active = true;
  let started = false;
  const permitted = () =>
    active && !state.loading && state.value === "accepted";
  const update = () => {
    if (started || !permitted()) return;
    started = true;
    void initializeErrorReporting("browser", permitted)
      .then((sentry) => {
        if (!sentry) started = false;
      })
      .catch((error: unknown) => {
        started = false;
        console.error("Error reporting could not be initialized.", error);
      });
  };
  const unsubscribe = cookieConsentStore.subscribe(update);
  update();
  return () => {
    active = false;
    unsubscribe();
  };
}

const REDACTED = "[redacted]";
const SENSITIVE_KEY =
  /address|wallet|privatekey|secret|seed|mnemonic|passphrase|password|signature|authorization|cookie|credential|token|session|email|userid|ipaddress/i;

function redactString(input: string): string {
  // Request URLs and breadcrumb text may contain percent-encoded secrets.
  let value = input;
  try {
    value = decodeURIComponent(value);
  } catch {
    // Malformed URL escapes do not prevent ordinary text redaction.
  }
  return value
    .replace(/\b(?:0x)?[0-9a-f]{64}\b/gi, REDACTED)
    .replace(/\b0x[0-9a-f]{40}\b/gi, REDACTED)
    .replace(/\bpax1[02-9ac-hj-np-z]{20,}\b/gi, REDACTED)
    .replace(/\b(?:Bearer|Basic)\s+[a-z0-9._~+\/-]+=*/gi, REDACTED)
    .replace(
      /(\b(?:private[_ -]?key|secret|seed|mnemonic|passphrase|password|signature|authorization|cookie|credential|token|session)\b["']?\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^&\s,;}]+)/gi,
      `$1${REDACTED}`,
    )
    .replace(/\b(?:[a-z]{1,8}\s+){11,23}[a-z]{1,8}\b/gi, REDACTED);
}

function redactStructured(input: unknown, ancestors: Set<object>): unknown {
  if (typeof input === "string") return redactString(input);
  if (input === null || typeof input !== "object") return input;
  if (ancestors.has(input)) return "[redacted-circular]";
  ancestors.add(input);
  try {
    if (Array.isArray(input)) {
      return input.map((value) => redactStructured(value, ancestors));
    }
    return Object.fromEntries(
      Object.entries(input).map(([key, value]) => [
        redactString(key),
        SENSITIVE_KEY.test(key.replace(/[^a-z0-9]/gi, ""))
          ? REDACTED
          : redactStructured(value, ancestors),
      ]),
    );
  } finally {
    ancestors.delete(input);
  }
}

/** Final privacy boundary shared by browser and edge error reporting. */
export function sanitizeSentryEvent(
  event: ErrorEvent,
  hint: EventHint,
): ErrorEvent | null {
  const original = hint.originalException;
  if (
    original !== null &&
    typeof original === "object" &&
    "name" in original &&
    original.name === "AbortError"
  )
    return null;

  return redactStructured(event, new Set()) as ErrorEvent;
}

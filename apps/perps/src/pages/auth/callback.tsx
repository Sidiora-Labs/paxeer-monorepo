/**
 * Supabase OAuth + magic-link landing page.
 *
 * The SDK runs the PKCE flow with `detectSessionInUrl: false`, so no route
 * reads a session from the URL. This page alone exchanges the `code` query
 * parameter for a session, then bounces the user back to wherever they came
 * from. A URL fragment carrying tokens is never read.
 */
"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/router";
import Head from "next/head";
import { getPaxeerWallet, type PaxeerWallet } from "@/core/wallet/embedded";
import { useWalletStore } from "@/hooks/wallet/useWalletStore";

const RETURN_TO_KEY = "paxeer:auth:returnTo";

type AuthCallbackOptions = {
  search: string;
  isCancelled: () => boolean;
  onError: (message: string) => void;
  onSuccess: () => Promise<unknown>;
};

export async function finishAuthCallback(wallet: PaxeerWallet, options: AuthCallbackOptions): Promise<void> {
  const { search, isCancelled, onError, onSuccess } = options;
  try {
    const code = new URLSearchParams(search).get("code");
    if (isCancelled()) return;
    if (!code) {
      onError("Sign-in link is missing its authorization code. Please sign in again.");
      return;
    }
    const { error: exchangeError } = await wallet.supabase.auth.exchangeCodeForSession(code);
    if (isCancelled()) return;
    const session = await wallet.getSession();
    if (isCancelled()) return;
    if (session) {
      await onSuccess();
      return;
    }
    onError(exchangeError?.message ?? "Sign-in did not complete. Please sign in again.");
  } catch (err) {
    if (isCancelled()) return;
    onError(err instanceof Error && err.message ? err.message : "Sign-in did not complete. Please sign in again.");
  }
}

export async function retryAuthCallback(navigateHome: () => Promise<boolean>): Promise<void> {
  if (await navigateHome()) useWalletStore.getState().openConnectModal();
}

export default function AuthCallback() {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const wallet = getPaxeerWallet();
    if (!wallet) {
      setError("Embedded wallet is not configured (NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY missing).");
      return;
    }

    function destination(): string {
      try {
        const stored = window.sessionStorage.getItem(RETURN_TO_KEY);
        if (stored) {
          window.sessionStorage.removeItem(RETURN_TO_KEY);
          return stored;
        }
      } catch {
        /* ignore quota / privacy errors */
      }
      return "/";
    }

    void finishAuthCallback(wallet, {
      search: window.location.search,
      isCancelled: () => cancelled,
      onError: setError,
      onSuccess: () => router.replace(destination()),
    });

    return () => {
      cancelled = true;
    };
  }, [router]);

  return (
    <AuthCallbackContent
      error={error}
      onRetry={() => {
        void retryAuthCallback(() => router.replace("/")).catch((err: unknown) => {
          setError(err instanceof Error ? err.message : "Unable to return home. Please try again.");
        });
      }}
    />
  );
}

export function AuthCallbackContent({ error, onRetry }: { error: string | null; onRetry: () => void }) {
  return (
    <>
      <Head>
        <title>Signing in… · HyperPax</title>
      </Head>
      <main className="flex min-h-dvh flex-col items-center justify-center gap-3 px-6 text-center bg-[#121317]">
        {error ? (
          <>
            <p className="text-[15px] text-foreground">Sign-in failed</p>
            <p className="text-[13px] text-red-400">{error}</p>
            <button
              type="button"
              onClick={onRetry}
              className="mt-2 rounded-xl border border-border bg-surface-1 px-4 py-2 text-[13px] text-foreground hover:bg-muted/30"
            >
              Sign in again
            </button>
            <a
              href="/"
              className="mt-2 rounded-xl border border-border bg-surface-1 px-4 py-2 text-[13px] text-foreground hover:bg-muted/30"
            >
              Back to home
            </a>
          </>
        ) : (
          <>
            <svg
              className="h-6 w-6 animate-spin text-muted-foreground"
              viewBox="0 0 24 24"
              fill="none"
              aria-hidden="true"
            >
              <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" opacity="0.25" />
              <path fill="currentColor" d="M4 12a8 8 0 018-8v3a5 5 0 00-5 5H4z" />
            </svg>
            <p className="text-[13px] text-muted-foreground">Finishing sign-in…</p>
          </>
        )}
      </main>
    </>
  );
}

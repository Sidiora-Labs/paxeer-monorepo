'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { paxeerWallet } from '@/lib/paxeer';
import { Spinner } from '@/components/Spinner';

/**
 * Supabase OAuth + magic-link landing page.
 *
 * The Supabase JS client (with `detectSessionInUrl: true`, default) will
 * automatically pick up the `code` / `access_token` from the URL on mount,
 * exchange it for a session, persist it to localStorage, and fire
 * `onAuthStateChange`. We just need to mount and then redirect.
 */
export default function AuthCallback() {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let unsubscribe: (() => void) | undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;

    void (async () => {
      try {
        const paxeer = paxeerWallet();
        const { data, error } = await paxeer.supabase.auth.getSession();
        if (cancelled) return;
        if (error) {
          setError(error.message);
          return;
        }
        if (data.session) {
          router.replace('/');
        } else {
          unsubscribe = paxeer.onAuthStateChange((_event, session) => {
            if (session && !cancelled) {
              unsubscribe?.();
              clearTimeout(timer);
              router.replace('/');
            }
          });
          timer = setTimeout(() => {
            if (!cancelled) {
              unsubscribe?.();
              setError('Sign-in did not complete. Please try again.');
            }
          }, 8000);
        }
      } catch (cause) {
        if (!cancelled)
          setError(
            cause instanceof Error ? cause.message : 'Sign-in did not complete. Please try again.'
          );
      }
    })();

    return () => {
      cancelled = true;
      unsubscribe?.();
      clearTimeout(timer);
    };
  }, [router]);

  return (
    <main className="callback">
      {error ? (
        <>
          <h1>Sign-in failed</h1>
          <p role="alert" className="error">
            {error}
          </p>
          <a href="/" className="button">
            Back to home
          </a>
        </>
      ) : (
        <>
          <Spinner label="Finishing sign-in…" />
          <h1 role="status">Finishing sign-in…</h1>
        </>
      )}
    </main>
  );
}

'use client';

import { useEffect, useRef, useState } from 'react';
import { LogIn, TriangleAlert } from 'lucide-react';
import { Button, EmptyState, Skeleton } from '@/ui/primitives';
import { useRouter } from 'next/navigation';
import { useAccount } from 'wagmi';
import { openAuthModal } from '@/shell/auth-modal-store';
import { readReturnPath, useEmbeddedAutoConnect } from '@/shell/embedded-sign-in';
import { PAXEER_CONNECTOR_ID, usePaxeerOptional } from '@/core/wallet-sdk/embedded_sdk/embedded';
import { authCallbackState } from '@/shell/auth-callback-state';

/**
 * Landing page for Paxeer Embedded Wallet redirects (magic link and OAuth).
 * Supabase consumes the URL fragment and installs the session; this page
 * waits for the wallet record, connects wagmi to the embedded connector,
 * returns the user to where they started and reopens the sign-in dialog at
 * the signature step.
 */
export default function AuthCallbackPage() {
  const router = useRouter();
  const paxeer = usePaxeerOptional();
  const { isConnected, connector, address } = useAccount();
  const auto = useEmbeddedAutoConnect(Boolean(paxeer?.wallet));
  const [timedOut, setTimedOut] = useState(false);
  const finished = useRef(false);
  const status = authCallbackState({ configured: Boolean(paxeer?.wallet), authenticated: Boolean(paxeer?.isAuthenticated), connected: isConnected, connectorId: connector?.id, address, walletAddress: paxeer?.publicWallet?.address, authError: paxeer?.authError ?? null, connectionError: auto.error, timedOut }, PAXEER_CONNECTOR_ID);

  useEffect(() => {
    const timer = window.setTimeout(() => setTimedOut(true), 20_000);
    return () => window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (finished.current || status !== 'ready') return;
    finished.current = true;
    const target = readReturnPath();
    router.replace(target);
    openAuthModal('Sign one message to finish signing in');
  }, [status, router]);

  const unavailable = !paxeer || !paxeer.wallet;
  const failed = status === 'failed';
  const message = unavailable
    ? 'Email sign-in is not available yet. Go back and use a browser wallet'
    : failed ? "Couldn't finish signing in. Request a new link and try again" : null;

  return (
    <div className="mx-auto w-full max-w-96 px-4 py-4" role="status" aria-live="polite">
      {unavailable || failed ? (
        <EmptyState
          icon={failed ? TriangleAlert : LogIn}
          headingLevel={1}
          title={unavailable ? 'Email sign-in' : "Couldn't sign you in"}
          body={message}
          action={<Button variant="secondary" href="/">Go home</Button>}
        />
      ) : (
        <div className="flex flex-col items-center gap-3 p-8 text-center" aria-busy="true">
          <Skeleton variant="block" className="size-6" />
          <h1 className="text-md font-semibold text-text">Finishing sign-in</h1>
          <p className="text-xs text-text-muted">
            {auto.connecting ? 'Connecting your wallet...' : 'Signing you in...'}
          </p>
        </div>
      )}
    </div>
  );
}

'use client';

import { useEffect, useRef, useState } from 'react';
import { getAddress } from 'viem';
import { useConnect } from 'wagmi';
import { useWalletRuntime } from '../../core/wallet-sdk/runtime';
import type { LoginConsent } from '../../core/wallet-sdk/auth';
import { WalletReconnect, walletRecoveryGuidance } from './reconnect';

export default function ReconnectPanel() {
  const runtime = useWalletRuntime();
  const { connectors } = useConnect();
  const [expected, setExpected] = useState('');
  const [state, setState] = useState('disconnected');
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);
  const [review, setReview] = useState<LoginConsent>();
  const resolveConsent = useRef<((value: boolean) => void) | undefined>(undefined);
  const inFlight = useRef(false);
  useEffect(() => {
    try { setExpected(localStorage.getItem('kindle:last-authenticated-wallet:v2') ?? runtime.session?.account ?? ''); }
    catch { setExpected(runtime.session?.account ?? ''); }
    return () => { resolveConsent.current?.(false); };
  }, []);
  useEffect(() => runtime.sessions.subscribe(() => { resolveConsent.current?.(false); resolveConsent.current = undefined; setReview(undefined); }), [runtime.sessions]);
  const connect = async () => {
    if (inFlight.current || !runtime.wallet || !runtime.session) return;
    inFlight.current = true; setPending(true); setError(undefined);
    const selected = runtime.session;
    try {
      const reconnect = new WalletReconnect(runtime.sessions);
      const result = await reconnect.connect(runtime.wallet, selected.connectorId, getAddress(expected.trim()), connectors.map(connector => connector.id), selected.providerKind);
      setState(result.matched ? 'awaiting_fresh_proof' : 'distinct_identity_requires_own_proof');
    } catch (failure) { setState('disconnected'); console.error('[wallet] reconnect failed', failure); setError("Couldn't reconnect, try again"); }
    finally { inFlight.current = false; setPending(false); }
  };
  const authenticate = async () => {
    if (inFlight.current) return;
    inFlight.current = true; setPending(true); setError(undefined);
    try {
      await runtime.login(consent => new Promise<boolean>(resolve => { resolveConsent.current = resolve; setReview(consent); }));
      const verified = runtime.identity?.current;
      if (!verified) throw new Error('Sign-in did not complete');
      setState(verified.wallet.toLowerCase() === expected.trim().toLowerCase() ? 'authenticated_same_wallet' : 'authenticated_distinct_wallet');
    } catch (failure) { console.error('[wallet] sign-in failed', failure); setError("Sign-in didn't finish, try again"); }
    finally { inFlight.current = false; setPending(false); resolveConsent.current = undefined; setReview(undefined); }
  };
  return <section aria-label="Reconnect wallet" className="space-y-3 text-xs text-text">
    <h3 className="font-semibold">Reconnect your wallet</h3>
    <p>{walletRecoveryGuidance}</p>
    <label className="block">Your wallet<input name="wallet" autoComplete="off" inputMode="text" disabled={pending} className="sl-field mt-1 w-full rounded-sm bg-surface-2 p-2" value={expected} onChange={event => { setExpected(event.target.value); setState('disconnected'); }} /></label>
    {!runtime.wallet && <p role="status">Connect a wallet first</p>}
    <button type="button" disabled={pending || !runtime.wallet} className="sl-field min-h-touch-target-min rounded-sm bg-surface-2 px-3 py-2 disabled:opacity-50" onClick={() => { void connect(); }}>Reconnect wallet</button>
    {runtime.session && <p className="break-all">Connected: {runtime.session.account}</p>}
    {state === 'distinct_identity_requires_own_proof' && <p role="alert">This is a different wallet. It has its own balance and won't be linked to your old one.</p>}
    {(state === 'awaiting_fresh_proof' || state === 'distinct_identity_requires_own_proof') && <button type="button" disabled={pending || runtime.session?.chainId !== 125} className="min-h-touch-target-min rounded-sm bg-surface-inverse px-3 py-2 font-bold text-canvas disabled:opacity-50" onClick={() => { void authenticate(); }}>Sign in with this wallet</button>}
    {review && <div role="dialog" aria-modal="true" aria-label="Sign-in approval" className="space-y-2 rounded-sm bg-surface-2 p-3"><p>Sign in to {review.domain}</p><p className="break-all">Wallet: {review.account}</p><p>{review.fee}</p><pre className="max-h-52 overflow-auto whitespace-pre-wrap break-all">{review.message}</pre><button type="button" className="min-h-touch-target-min rounded-sm bg-surface-inverse px-3 py-2 font-bold text-canvas" onClick={() => { resolveConsent.current?.(true); resolveConsent.current = undefined; }}>Sign in</button><button type="button" className="ml-2 sl-field min-h-touch-target-min rounded-sm bg-surface-2 px-3 py-2" onClick={() => { resolveConsent.current?.(false); resolveConsent.current = undefined; }}>Cancel</button></div>}
    {state === 'authenticated_same_wallet' && <p role="status">You're signed in again. Your balance is right where you left it.</p>}
    {state === 'authenticated_distinct_wallet' && <p role="status">You're signed in with a different wallet. Nothing was merged with your old one.</p>}
    {pending && <p role="status">Check your wallet to continue...</p>}{error && <p role="alert">{error}</p>}
  </section>;
}

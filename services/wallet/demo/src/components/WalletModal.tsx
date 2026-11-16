'use client';

import { useEffect, useId, useRef, useState } from 'react';
import { discoverProviders, type Eip6963ProviderDetail, type PaxeerWallet } from '@paxeer/wallet';
import { useSession, useStandardAccount } from '@paxeer/wallet/react';
import { getAuthRedirectUrl } from '@/lib/paxeer';
import { ProviderButton } from './ProviderButton';
import { FormField } from './FormField';
import { WalletForm } from './WalletForm';
import { SupportedWalletPanel } from './FundedAccountPanel';

type View = 'chooser' | 'auth' | 'embedded' | 'injected';

interface WalletModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  paxeer: PaxeerWallet;
  origin: string;
}

export function WalletModal({ open, onOpenChange, paxeer, origin }: WalletModalProps) {
  const { user, loading } = useSession(paxeer);
  const standard = useStandardAccount(paxeer);
  const [view, setView] = useState<View>('chooser');
  const [providers, setProviders] = useState<Eip6963ProviderDetail[]>([]);
  const [injected, setInjected] = useState<Eip6963ProviderDetail | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) element.showModal();
    if (!open && element.open) element.close();
  }, [open]);

  useEffect(() => {
    if (open) {
      dialog.current?.scrollTo({ top: 0 });
      heading.current?.focus();
    }
  }, [open, view]);

  useEffect(() => {
    const element = dialog.current;
    if (!element || 'closedBy' in HTMLDialogElement.prototype) return;
    let pressedOutside = false;
    function outside(event: PointerEvent) {
      const rect = element!.getBoundingClientRect();
      return (
        event.clientX < rect.left ||
        event.clientX > rect.right ||
        event.clientY < rect.top ||
        event.clientY > rect.bottom
      );
    }
    function press(event: PointerEvent) {
      pressedOutside = outside(event);
    }
    function release(event: PointerEvent) {
      if (pressedOutside && outside(event)) element!.close();
      pressedOutside = false;
    }
    element.addEventListener('pointerdown', press);
    element.addEventListener('pointerup', release);
    return () => {
      element.removeEventListener('pointerdown', press);
      element.removeEventListener('pointerup', release);
    };
  }, []);

  useEffect(() => {
    if (!open) {
      setView('chooser');
      setInjected(null);
      return;
    }
    const discovery = discoverProviders((detail) =>
      setProviders((prior) =>
        prior.some((p) => p.info.uuid === detail.info.uuid) ? prior : [...prior, detail]
      )
    );
    setProviders([...discovery.providers]);
    return () => discovery.stop();
  }, [open]);

  useEffect(() => {
    if (view === 'auth' && user) setView('embedded');
    if (view === 'embedded' && !loading && !user) setView('auth');
  }, [user, loading, view]);

  async function provision() {
    setPending(true);
    setError(null);
    try {
      await paxeer.provisionStandardWallet();
      await standard.refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Provisioning unavailable');
    } finally {
      setPending(false);
    }
  }

  return (
    <dialog
      ref={dialog}
      {...{ closedby: 'any' }}
      aria-labelledby={titleId}
      aria-describedby={descriptionId}
      onClose={() => onOpenChange(false)}
      className="wallet-dialog"
    >
      <header className="dialog-header">
        <h2 id={titleId} ref={heading} tabIndex={-1}>
          Supported wallets
        </h2>
        <form method="dialog">
          <button type="submit" aria-label="Close wallet" className="button quiet icon-button">
            ×
          </button>
        </form>
        <p id={descriptionId}>
          Embedded threshold custody or your injected wallet. Every signing action requires
          approval.
        </p>
      </header>
      {view !== 'chooser' && (
        <button
          type="button"
          onClick={() => setView('chooser')}
          className="button quiet dialog-back"
        >
          ← Wallet choices
        </button>
      )}
      {view === 'chooser' && (
        <div className="stack">
          <button
            type="button"
            data-wallet-embedded
            onClick={() => setView(user ? 'embedded' : 'auth')}
            className="button choice"
          >
            Embedded wallet
            <span>Sign in and explicitly provision threshold custody.</span>
          </button>
          {providers.map((detail) => (
            <button
              type="button"
              key={detail.info.uuid}
              data-wallet-injected
              onClick={() => {
                setInjected(detail);
                setView('injected');
              }}
              className="button choice"
            >
              {detail.info.name}
              <span>Injected wallet · signing controlled by your wallet.</span>
            </button>
          ))}
          {!providers.length && (
            <p role="status" className="small-copy muted">
              No injected wallet announced. Enable an EIP-6963 wallet to connect.
            </p>
          )}
        </div>
      )}
      {view === 'auth' && <AuthView paxeer={paxeer} />}
      {view === 'embedded' && user && (
        <div className="stack">
          {standard.loading ? (
            <p role="status">Loading embedded wallet…</p>
          ) : standard.error ? (
            <p role="alert" className="error notice">
              {standard.error.message}
            </p>
          ) : !standard.data ? (
            <button
              type="button"
              data-wallet-provision
              disabled={pending}
              aria-busy={pending}
              onClick={provision}
              className="button primary"
            >
              {pending ? 'Creating…' : 'Create embedded threshold-custody wallet'}
            </button>
          ) : (
            <SupportedWalletPanel paxeer={paxeer} origin={origin} />
          )}
          {error && (
            <p role="alert" className="error notice">
              {error}
            </p>
          )}
          <button
            type="button"
            onClick={async () => {
              try {
                await paxeer.signOut();
                setView('chooser');
              } catch (cause) {
                setError(cause instanceof Error ? cause.message : 'Sign-out unavailable');
              }
            }}
            className="button quiet"
          >
            Sign out
          </button>
        </div>
      )}
      {view === 'injected' && injected && (
        <SupportedWalletPanel paxeer={paxeer} origin={origin} injected={injected} />
      )}
    </dialog>
  );
}

function AuthView({ paxeer }: { paxeer: PaxeerWallet }) {
  const [email, setEmail] = useState('');
  const [emailStatus, setEmailStatus] = useState<
    { kind: 'idle' } | { kind: 'sending' } | { kind: 'sent' } | { kind: 'error'; message: string }
  >({ kind: 'idle' });
  const [oauthLoading, setOauthLoading] = useState<string | null>(null);
  const resultHeading = useRef<HTMLHeadingElement>(null);
  const errorMessage = useRef<HTMLParagraphElement>(null);

  useEffect(() => {
    if (emailStatus.kind === 'sent') resultHeading.current?.focus();
    if (emailStatus.kind === 'error') errorMessage.current?.scrollIntoView({ block: 'nearest' });
  }, [emailStatus.kind]);

  async function clickProvider(provider: 'google' | 'github' | 'twitter') {
    setOauthLoading(provider);
    try {
      const redirectTo = getAuthRedirectUrl();
      await paxeer.signInWithOAuth(provider, redirectTo);
    } catch (err) {
      setOauthLoading(null);
      setEmailStatus({
        kind: 'error',
        message: err instanceof Error ? err.message : 'sign-in failed',
      });
    }
  }

  async function submitEmail(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (emailStatus.kind === 'sending' || !e.currentTarget.reportValidity()) return;
    setEmailStatus({ kind: 'sending' });
    try {
      const redirectTo = getAuthRedirectUrl();
      const { error } = await paxeer.signInWithEmail(email, redirectTo);
      setEmailStatus(error ? { kind: 'error', message: error.message } : { kind: 'sent' });
    } catch (cause) {
      setEmailStatus({
        kind: 'error',
        message: cause instanceof Error ? cause.message : 'Sign-in unavailable',
      });
    }
  }

  if (emailStatus.kind === 'sent') {
    return (
      <div className="outcome">
        <div className="outcome-icon">
          <svg
            className="icon"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            aria-hidden="true"
          >
            <path
              d="M3 7l9 6 9-6M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </div>
        <h3 ref={resultHeading} tabIndex={-1}>
          Check your email
        </h3>
        <p>
          We sent a magic link to <span>{email}</span>.<br />
          Click it to finish signing in.
        </p>
      </div>
    );
  }

  return (
    <div className="stack auth-content">
      <p className="small-copy muted">
        Sign in to explicitly create an embedded threshold-custody wallet.
      </p>
      <div className="stack">
        <ProviderButton
          label="Continue with Google"
          iconSrc="/icons/google.svg"
          onClick={() => clickProvider('google')}
          loading={oauthLoading === 'google'}
          disabled={oauthLoading !== null}
        />
        <ProviderButton
          label="Continue with X"
          iconSrc="/icons/x.svg"
          onClick={() => clickProvider('twitter')}
          loading={oauthLoading === 'twitter'}
          disabled={oauthLoading !== null}
        />
        <ProviderButton
          label="Continue with GitHub"
          iconSrc="/icons/github.svg"
          onClick={() => clickProvider('github')}
          loading={oauthLoading === 'github'}
          disabled={oauthLoading !== null}
        />
      </div>
      <hr className="divider" />
      <WalletForm
        onSubmit={submitEmail}
        className="form"
        aria-label="Email sign-in"
        aria-busy={emailStatus.kind === 'sending'}
      >
        <FormField
          label="Email"
          name="email"
          type="email"
          inputMode="email"
          autoComplete="email"
          placeholder="you@example.com"
          value={email}
          onValueChange={setEmail}
          disabled={emailStatus.kind === 'sending'}
          errorMessage="Enter an email address in the format you@example.com."
        />
        <button type="submit" disabled={emailStatus.kind === 'sending'} className="button primary">
          {emailStatus.kind === 'sending' ? 'Sending…' : 'Send magic link'}
        </button>
        {emailStatus.kind === 'error' && (
          <p ref={errorMessage} role="alert" className="error notice">
            {emailStatus.message}
          </p>
        )}
      </WalletForm>
      <p className="fine-print">By continuing you agree to Paxeer&apos;s terms.</p>
    </div>
  );
}

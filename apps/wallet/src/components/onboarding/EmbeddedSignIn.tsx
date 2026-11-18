'use client';

import { useState } from 'react';
import { ArrowLeft, ArrowRight, Loader2 } from 'lucide-react';
import { BsGoogle, BsDiscord, BsGithub, BsApple, BsTwitterX } from 'react-icons/bs';
import { useWallet } from '@/wallet/WalletProvider';
import type { IdentityProvider } from '@/wallet/identity';

const SOCIAL_PROVIDERS: Array<{
  id: IdentityProvider;
  label: string;
  Icon: React.ComponentType<{ className?: string }>;
}> = [
  { id: 'google', label: 'Google', Icon: BsGoogle },
  { id: 'discord', label: 'Discord', Icon: BsDiscord },
  { id: 'github', label: 'GitHub', Icon: BsGithub },
  { id: 'apple', label: 'Apple', Icon: BsApple },
  { id: 'twitter', label: 'X', Icon: BsTwitterX },
];

interface EmbeddedSignInProps {
  onBack: () => void;
}

function failure(err: unknown): string {
  return err instanceof Error && err.message ? err.message : 'Sign-in failed';
}

function showFocusRing(event: React.FocusEvent<HTMLButtonElement>) {
  event.currentTarget.style.boxShadow = '0 0 0 2px var(--color-action-primary)';
  event.currentTarget.style.transform = 'scale(1.08)';
}

function hideFocusRing(event: React.FocusEvent<HTMLButtonElement>) {
  event.currentTarget.style.boxShadow = '';
  event.currentTarget.style.transform = '';
}

export function EmbeddedSignIn({ onBack }: EmbeddedSignInProps) {
  const wallet = useWallet();
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [codeSent, setCodeSent] = useState(false);
  const [localError, setLocalError] = useState('');

  const busy = wallet.busy;
  const anyError = localError || wallet.error;
  const emailValid = /^\S+@\S+\.\S+$/.test(email.trim());
  const codeValid = /^\d{6,10}$/.test(code.trim());

  const handleSendCode = async () => {
    setLocalError('');
    if (!emailValid) {
      setLocalError('Enter a valid email address.');
      return;
    }
    try {
      await wallet.sendEmailCode(email.trim());
      setCodeSent(true);
    } catch (err) {
      setLocalError(failure(err));
    }
  };

  const handleVerify = async () => {
    setLocalError('');
    if (!codeValid) {
      setLocalError('Enter the code from the email.');
      return;
    }
    try {
      await wallet.verifyEmailCode(email.trim(), code.trim());
    } catch (err) {
      setLocalError(failure(err));
    }
  };

  const handleOAuth = async (id: IdentityProvider) => {
    setLocalError('');
    try {
      await wallet.signInWithProvider(id);
    } catch (err) {
      setLocalError(failure(err));
    }
  };

  if (codeSent) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center px-6 py-10">
        <div className="pax-signin flex flex-col items-center gap-6 max-w-sm w-full text-center">
          <div className="flex size-16 items-center justify-center rounded-2xl bg-pax-accent-muted">
            <ArrowRight className="size-8 text-pax-accent" />
          </div>
          <div>
            <h1 className="text-xl font-bold tracking-tight">Check Your Inbox</h1>
            <p className="text-sm text-pax-muted mt-2 text-pretty max-w-size-260">
              Enter the code we emailed you, or open the sign-in link on this device.
            </p>
          </div>
          {anyError && (
            <p id="signin-error" role="alert" className="text-pax-error text-xs text-center">
              {anyError}
            </p>
          )}
          <label htmlFor="signin-code" className="w-full text-left text-sm font-semibold">
            Email code
          </label>
          <div className="pax-signin-field bg-pax-surface flex min-h-12 w-full items-center gap-3 rounded-2xl pl-4 pr-1">
            <input
              id="signin-code"
              aria-label="Email code"
              aria-invalid={Boolean(anyError)}
              aria-describedby={anyError ? 'signin-error' : undefined}
              name="code"
              autoCapitalize="none"
              spellCheck={false}
              inputMode="numeric"
              autoComplete="one-time-code"
              autoFocus
              placeholder="000000"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.nativeEvent.isComposing && e.keyCode !== 229)
                  void handleVerify();
              }}
              disabled={busy}
              className="min-w-0 min-h-12 flex-1 bg-transparent text-base outline-none placeholder:text-pax-mid disabled:opacity-50"
            />
            <button
              type="button"
              aria-label="Verify code"
              onClick={() => void handleVerify()}
              disabled={!codeValid || busy}
              className={`flex h-10 w-12 shrink-0 items-center justify-center rounded-xl transition-all active:scale-95 focus-visible:ring-2 focus-visible:ring-pax-accent ${codeValid ? 'bg-pax-accent text-pax-on-accent' : 'bg-pax-mid text-pax-bg cursor-not-allowed'}`}
            >
              {busy ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <ArrowRight className="h-5 w-5" />
              )}
            </button>
          </div>
          <button
            onClick={() => {
              setCodeSent(false);
              setEmail('');
              setCode('');
            }}
            onFocus={showFocusRing}
            onBlur={hideFocusRing}
            className="text-sm text-pax-muted hover:text-pax-light focus-visible:bg-pax-control focus-visible:text-pax-light focus-visible:opacity-70"
          >
            Use a different email
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex flex-col items-center justify-center px-5 py-10">
      <div className="pax-signin flex flex-col gap-5 w-full max-w-sm">
        <button
          onClick={onBack}
          onFocus={showFocusRing}
          onBlur={hideFocusRing}
          className="flex items-center gap-1.5 rounded-lg text-xs text-pax-muted self-start focus-visible:bg-pax-control focus-visible:text-pax-light focus-visible:opacity-70"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          Back
        </button>

        <div className="text-center">
          <h1 className="text-xl font-bold tracking-tight">Sign in to Paxeer</h1>
          <p className="text-xs text-pax-muted mt-1.5 leading-relaxed">
            One wallet across every Paxeer app — no seed phrase.
          </p>
        </div>

        {anyError && (
          <p id="signin-error" role="alert" className="text-pax-error text-xs text-center">
            {anyError}
          </p>
        )}

        <div className="flex w-full flex-wrap items-center justify-center gap-2">
          {SOCIAL_PROVIDERS.map(({ id, label, Icon }) => (
            <button
              key={id}
              aria-label={label}
              disabled={busy}
              onClick={() => void handleOAuth(id)}
              onFocus={showFocusRing}
              onBlur={hideFocusRing}
              className="pax-signin-field bg-pax-mid text-pax-bg flex min-h-12 flex-1 items-center justify-center rounded-xl hover:bg-pax-overlay focus-visible:bg-pax-overlay focus-visible:ring-2 focus-visible:ring-pax-accent focus-visible:opacity-70 active:scale-95 disabled:opacity-40"
            >
              {busy ? <Loader2 className="size-4 animate-spin" /> : <Icon className="size-4" />}
            </button>
          ))}
        </div>

        <label htmlFor="signin-email" className="text-sm font-semibold">
          Email
        </label>
        <div className="pax-signin-field bg-pax-control flex min-h-12 w-full items-center justify-start gap-3 rounded-2xl pl-4 pr-1">
          <input
            type="email"
            id="signin-email"
            aria-label="Email"
            aria-invalid={Boolean(anyError)}
            aria-describedby={anyError ? 'signin-error' : undefined}
            name="email"
            autoCapitalize="none"
            spellCheck={false}
            autoComplete="email"
            inputMode="email"
            autoFocus
            placeholder="you@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.nativeEvent.isComposing && e.keyCode !== 229)
                void handleSendCode();
            }}
            disabled={busy}
            className="min-w-0 min-h-12 flex-1 bg-transparent text-base outline-none placeholder:text-pax-mid disabled:opacity-50"
          />
          <button
            type="button"
            aria-label="Send code"
            onClick={() => void handleSendCode()}
            disabled={!emailValid || busy}
            className={`flex h-10 w-12 shrink-0 items-center justify-center rounded-xl transition-all active:scale-95 focus-visible:ring-2 focus-visible:ring-pax-accent ${emailValid ? 'bg-pax-accent text-pax-on-accent' : 'bg-pax-mid text-pax-bg cursor-not-allowed'}`}
          >
            {busy ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <ArrowRight className="h-5 w-5" />
            )}
          </button>
        </div>
      </div>
    </div>
  );
}

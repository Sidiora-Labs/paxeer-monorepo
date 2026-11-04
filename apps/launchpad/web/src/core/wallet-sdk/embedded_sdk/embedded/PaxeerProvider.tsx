/**
 * PaxeerProvider — React context for the Paxeer Embedded Wallet.
 *
 * Wraps the new `@paxeer/wallet` SDK in a context so any component in the
 * tree can pull session state, kick off auth flows, and access the singleton
 * client. Backed by Supabase auth — all signing happens server-side via the
 * REST API (see `./sdk/index.ts`).
 *
 * The actual EIP-1193 provider used by wagmi is created in
 * `paxeerConnector.ts` from the same singleton, so the connector and this
 * provider stay in lockstep without prop drilling.
 */
"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import type { Session, User } from "@supabase/supabase-js";
import { PaxeerWallet, type OAuthProvider, type PublicWallet } from "./sdk";
import {
  getAuthRedirectUrl,
  getPaxeerWallet,
  resolvePaxeerConfig,
} from "./client";
import { reportUserFacingError } from "@/core/user-facing-error";

const SIGN_IN_FAILED = "Sign-in did not work, try again";
const EMAIL_NOT_READY = "Email sign-in is not available yet";

// ─── Types ────────────────────────────────────────────────────────────────────

export type SignInProvider = OAuthProvider;

export interface EmbeddedSignInResult {
  pending: boolean; // true when an OAuth redirect is in progress
  emailSent: boolean; // true when a magic link was just dispatched
}

interface PaxeerContextValue {
  /** The SDK singleton, or null when env vars are missing. */
  wallet: PaxeerWallet | null;
  session: Session | null;
  user: User | null;
  publicWallet: PublicWallet | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  /** Set while an auth call is in flight. */
  authBusy: boolean;
  authError: string | null;
  signInWithEmail: (email: string) => Promise<{ ok: boolean; error?: string }>;
  signInWithOAuth: (provider: SignInProvider) => Promise<void>;
  signOut: () => Promise<void>;
  /** Force-refresh the cached wallet record (after a tx, etc.). */
  refresh: () => void;
}

const PaxeerContext = createContext<PaxeerContextValue | null>(null);

// ─── Provider ─────────────────────────────────────────────────────────────────

interface PaxeerProviderProps {
  children: ReactNode;
}

export type { PaxeerContextValue };

export function PaxeerProvider({ children }: PaxeerProviderProps) {
  const wallet = useMemo<PaxeerWallet | null>(() => getPaxeerWallet(), []);

  const generation = useRef(0);
  const [identity, setIdentity] = useState<{
    session: Session | null;
    generation: number;
  }>({ session: null, generation: 0 });
  const session = identity.session;
  const user: User | null = session?.user ?? null;
  const [walletRecord, setWalletRecord] = useState<{
    wallet: PublicWallet;
    userId: string;
    generation: number;
  } | null>(null);
  const publicWallet =
    walletRecord?.userId === user?.id &&
    walletRecord?.generation === identity.generation &&
    identity.generation === generation.current
      ? (walletRecord?.wallet ?? null)
      : null;
  const [isLoading, setIsLoading] = useState(true);
  const [authBusy, setAuthBusy] = useState(false);
  const [authError, setAuthError] = useState<string | null>(null);
  const [refreshTick, setRefreshTick] = useState(0);

  const replaceSession = useCallback((next: Session | null) => {
    generation.current += 1;
    setWalletRecord(null);
    setAuthError(null);
    setIsLoading(Boolean(next));
    setIdentity({ session: next, generation: generation.current });
  }, []);

  // Hydrate session + subscribe to auth state changes.
  useEffect(() => {
    if (!wallet) {
      setIsLoading(false);
      return;
    }
    let alive = true;
    const initialGeneration = generation.current;
    void (async () => {
      const s = await wallet.getSession();
      if (!alive || generation.current !== initialGeneration) return;
      replaceSession(s);
    })();
    const unsub = wallet.onAuthStateChange((_event, s) => {
      if (alive) replaceSession(s);
    });
    return () => {
      alive = false;
      unsub();
    };
  }, [wallet, replaceSession]);

  // Provision/fetch the wallet record whenever we have an auth session.
  useEffect(() => {
    if (!wallet) {
      setIsLoading(false);
      return;
    }
    let alive = true;
    const isCurrent = () => alive && generation.current === identity.generation;
    setIsLoading(true);
    void (async () => {
      try {
        if (!session) {
          if (isCurrent()) {
            setWalletRecord(null);
            setIsLoading(false);
          }
          return;
        }
        const r = await wallet.getWallet({
          userId: session.user.id,
          accessToken: session.access_token,
          assertCurrent() {
            if (!isCurrent()) throw new Error("the wallet session changed");
          },
        });
        if (!isCurrent()) return;
        setWalletRecord({
          wallet: r.wallet,
          userId: session.user.id,
          generation: identity.generation,
        });
        setAuthError(null);
      } catch (err) {
        if (isCurrent()) {
          setWalletRecord(null);
          setAuthError(
            reportUserFacingError(err, SIGN_IN_FAILED, "embedded-session"),
          );
        }
      } finally {
        if (isCurrent()) setIsLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, [wallet, session, identity.generation, refreshTick]);

  const signInWithEmail = useCallback(
    async (email: string): Promise<{ ok: boolean; error?: string }> => {
      if (!wallet) return { ok: false, error: EMAIL_NOT_READY };
      setAuthBusy(true);
      setAuthError(null);
      try {
        const { error } = await wallet.signInWithEmail(
          email,
          getAuthRedirectUrl(),
        );
        if (error) {
          const message = reportUserFacingError(
            error,
            "Could not send the sign-in link, check your email and try again",
            "email-sign-in",
          );
          setAuthError(message);
          return { ok: false, error: message };
        }
        return { ok: true };
      } finally {
        setAuthBusy(false);
      }
    },
    [wallet],
  );

  const signInWithOAuth = useCallback(
    async (provider: SignInProvider): Promise<void> => {
      if (!wallet) {
        setAuthError("This sign-in option is not available yet");
        return;
      }
      setAuthBusy(true);
      setAuthError(null);
      try {
        await wallet.signInWithOAuth(provider, getAuthRedirectUrl());
        // Browser redirects away — control never returns here on success.
      } catch (err) {
        setAuthError(
          reportUserFacingError(err, SIGN_IN_FAILED, "oauth-sign-in"),
        );
        setAuthBusy(false);
        throw err;
      }
    },
    [wallet],
  );

  const signOut = useCallback(async () => {
    if (!wallet) return;
    setAuthBusy(true);
    replaceSession(null);
    try {
      await wallet.signOut();
    } finally {
      setAuthBusy(false);
    }
  }, [wallet, replaceSession]);

  const refresh = useCallback(() => setRefreshTick((t) => t + 1), []);

  const value = useMemo<PaxeerContextValue>(
    () => ({
      wallet,
      session,
      user,
      publicWallet,
      isLoading,
      isAuthenticated: Boolean(session && publicWallet),
      authBusy,
      authError,
      signInWithEmail,
      signInWithOAuth,
      signOut,
      refresh,
    }),
    [
      wallet,
      session,
      user,
      publicWallet,
      isLoading,
      authBusy,
      authError,
      signInWithEmail,
      signInWithOAuth,
      signOut,
      refresh,
    ],
  );

  return (
    <PaxeerContext.Provider value={value}>{children}</PaxeerContext.Provider>
  );
}

// ─── Hooks ────────────────────────────────────────────────────────────────────

export function usePaxeer(): PaxeerContextValue {
  const ctx = useContext(PaxeerContext);
  if (!ctx) {
    throw new Error("usePaxeer must be used within <PaxeerProvider>");
  }
  return ctx;
}

/** Like usePaxeer, but returns null where no PaxeerProvider is mounted. */
export function usePaxeerOptional(): PaxeerContextValue | null {
  return useContext(PaxeerContext);
}

/** Returns true when the embedded-wallet feature is configured (Supabase key set). */
export function useEmbeddedWalletAvailable(): boolean {
  return useMemo(() => resolvePaxeerConfig() !== null, []);
}

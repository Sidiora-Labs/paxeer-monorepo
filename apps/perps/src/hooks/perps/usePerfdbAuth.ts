"use client";

import { useEffect, useSyncExternalStore } from "react";
import { perfdbRest } from "@/core/perfdb/client";
import { useWalletStore } from "@/hooks/wallet/useWalletStore";

interface AuthState {
  address: string | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  expiresAt: number | null;
}

export interface UsePerfdbAuthReturn extends AuthState {
  authenticate: (address: string, signMessage: (message: string) => Promise<string>) => Promise<void>;
  refresh: () => Promise<void>;
  logout: () => void;
}

const JWT_KEY = "perfdb_jwt";
const JWT_EXPIRES_KEY = "perfdb_jwt_expires";
const JWT_ADDRESS_KEY = "perfdb_jwt_address";
const empty: AuthState = { address: null, token: null, isAuthenticated: false, isLoading: false, expiresAt: null };
let state = empty;
let generation = 0;
let refreshTimer: ReturnType<typeof setTimeout> | undefined;
let owners = 0;
const listeners = new Set<() => void>();
const selectedAddress = () => useWalletStore.getState().address?.toLowerCase() ?? null;

function publish(next: AuthState) {
  state = next;
  listeners.forEach(listener => listener());
}

function clearTimer() {
  if (refreshTimer !== undefined) clearTimeout(refreshTimer);
  refreshTimer = undefined;
}

function clearStorage() {
  if (typeof window === "undefined") return;
  for (const key of [JWT_KEY, JWT_EXPIRES_KEY, JWT_ADDRESS_KEY]) window.localStorage.removeItem(key);
}

function logout() {
  generation++;
  clearTimer();
  clearStorage();
  publish(empty);
}

function current(request: number, address: string) {
  return owners > 0 && request === generation && selectedAddress() === address;
}

function scheduleRefresh(expiresAt: number, request: number, address: string) {
  if (!current(request, address)) return;
  clearTimer();
  refreshTimer = setTimeout(() => {
    if (current(request, address)) void refresh();
  }, Math.max(10_000, expiresAt - Date.now() - 60_000));
}

function applyToken(token: string, expiresIn: number, address: string, request: number) {
  if (!current(request, address)) return;
  if (!token || !Number.isFinite(expiresIn) || expiresIn <= 0) throw new Error("Invalid authentication response.");
  const expiresAt = Date.now() + expiresIn * 1000;
  // Persist the binding before the token is available to REST consumers.
  window.localStorage.setItem(JWT_ADDRESS_KEY, address);
  window.localStorage.setItem(JWT_EXPIRES_KEY, String(expiresAt));
  window.localStorage.setItem(JWT_KEY, token);
  publish({ address, token, expiresAt, isAuthenticated: true, isLoading: false });
  scheduleRefresh(expiresAt, request, address);
}

async function authenticate(address: string, signMessage: (message: string) => Promise<string>) {
  const identity = address.toLowerCase();
  if (!identity || identity !== selectedAddress()) throw new Error("Authentication must match the selected wallet.");
  logout();
  const request = generation;
  publish({ ...empty, address: identity, isLoading: true });
  try {
    const challenge = await perfdbRest<{ nonce: string; message: string; }>("/v2/auth/challenge", {
      method: "POST", body: JSON.stringify({ address: identity }),
    });
    if (!current(request, identity)) return;
    const signature = await signMessage(challenge.message);
    if (!current(request, identity)) return;
    const verify = await perfdbRest<{ token: string; expires_in: number; }>("/v2/auth/verify", {
      method: "POST", body: JSON.stringify({ address: identity, signature, message: challenge.message }),
    });
    applyToken(verify.token, verify.expires_in, identity, request);
  } catch (cause) {
    if (current(request, identity)) logout();
    throw cause;
  }
}

async function refresh() {
  const { token, address } = state;
  if (!token || !address || address !== selectedAddress()) { logout(); return; }
  const request = ++generation;
  clearTimer();
  try {
    const data = await perfdbRest<{ token: string; expires_in: number; }>("/v2/auth/refresh", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ token }),
    });
    applyToken(data.token, data.expires_in, address, request);
  } catch {
    if (current(request, address)) logout();
  }
}

/** Mutations must match both the current wallet and the verified stored session. */
export function requirePerfdbIdentity(address: string | null | undefined): string {
  const identity = address?.toLowerCase();
  if (!identity || identity !== selectedAddress() || identity !== state.address || !state.token || !state.expiresAt || state.expiresAt <= Date.now() || typeof window === "undefined" || window.localStorage.getItem(JWT_ADDRESS_KEY) !== identity || window.localStorage.getItem(JWT_KEY) !== state.token) {
    throw new Error("Not authenticated for the selected wallet. Sign in first.");
  }
  return state.token;
}

function hydrate() {
  const address = selectedAddress();
  if (typeof window === "undefined") return;
  try {
    const storedAddress = window.localStorage.getItem(JWT_ADDRESS_KEY);
    const token = window.localStorage.getItem(JWT_KEY);
    const expiresAt = Number(window.localStorage.getItem(JWT_EXPIRES_KEY));
    if (!address || address !== storedAddress || !token || !Number.isFinite(expiresAt) || expiresAt <= Date.now()) { logout(); return; }
    publish({ address, token, expiresAt, isAuthenticated: true, isLoading: false });
    if (expiresAt - Date.now() <= 60_000) void refresh();
    else scheduleRefresh(expiresAt, generation, address);
  } catch { logout(); }
}

const subscribe = (listener: () => void) => { listeners.add(listener); return () => { listeners.delete(listener); }; };
const snapshot = () => state;
const serverSnapshot = () => empty;

/** Shared SIWE session; signing remains delegated to the caller's wallet. */
export function usePerfdbAuth(): UsePerfdbAuthReturn {
  const auth = useSyncExternalStore(subscribe, snapshot, serverSnapshot);
  const address = useWalletStore(s => s.address?.toLowerCase() ?? null);
  useEffect(() => {
    owners++;
    if (owners === 1) hydrate();
    const unsubscribe = useWalletStore.subscribe((next, previous) => {
      if (next.address?.toLowerCase() !== previous.address?.toLowerCase()) logout();
    });
    return () => {
      unsubscribe();
      owners--;
      generation++;
      clearTimer();
      if (state.isLoading) publish(empty);
      if (owners > 0 && state.address && state.expiresAt) scheduleRefresh(state.expiresAt, generation, state.address);
    };
  }, []);
  return { ...(auth.address === address ? auth : empty), authenticate, refresh, logout };
}

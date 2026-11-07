import type { Journey } from "../../api/index.ts";
import { ACTIVE_ACCOUNT_STORAGE_KEY } from "../../auth/session.ts";
import { accountActive } from "./model.ts";

export type OnboardingStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export interface PendingOnboarding {
  readonly version: 1;
  readonly idempotencyKey: string;
  readonly email: string;
  readonly displayName: string;
  readonly accountId?: string;
  readonly passkeyAdded: boolean;
}

export const PENDING_ONBOARDING_KEY = "layerx.onboarding.pending.v1";

export function pendingOnboarding(storage?: OnboardingStorage): PendingOnboarding | undefined {
  let decoded: unknown;
  try {
    const stored = (storage ?? localStorage).getItem(PENDING_ONBOARDING_KEY);
    if (stored === null) {
      return undefined;
    }
    decoded = JSON.parse(stored);
  } catch {
    return undefined;
  }
  if (typeof decoded !== "object" || decoded === null || Array.isArray(decoded)) {
    return undefined;
  }
  const candidate = decoded as Partial<PendingOnboarding>;
  if (
    candidate.version !== 1
    || typeof candidate.idempotencyKey !== "string"
    || typeof candidate.email !== "string"
    || typeof candidate.displayName !== "string"
    || typeof candidate.passkeyAdded !== "boolean"
    || (candidate.accountId !== undefined && typeof candidate.accountId !== "string")
  ) {
    return undefined;
  }
  return candidate as PendingOnboarding;
}

export function rememberPending(pending: PendingOnboarding, storage?: OnboardingStorage): void {
  try {
    (storage ?? localStorage).setItem(PENDING_ONBOARDING_KEY, JSON.stringify(pending));
  } catch {
    return;
  }
}

export function forgetPending(storage?: OnboardingStorage): void {
  try {
    (storage ?? localStorage).removeItem(PENDING_ONBOARDING_KEY);
  } catch {
    return;
  }
}

export function rememberActivatedAccount(journey: Journey, storage?: OnboardingStorage): void {
  if (!accountActive(journey)) {
    return;
  }
  const pending = pendingOnboarding(storage);
  if (pending?.accountId === undefined) {
    return;
  }
  try {
    (storage ?? localStorage).setItem(ACTIVE_ACCOUNT_STORAGE_KEY, pending.accountId);
  } catch {
    return;
  }
  forgetPending(storage);
}


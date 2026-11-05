// ── Native Push Notification Helpers ─────────────────────────────────────────
// Manages Web Push subscriptions without any third-party service.
// To send push from your backend, use the subscription object with web-push lib.

import { pushUnsubscribeMessage } from '@/app/api/push/subscribe/unsubscribe-intent';
import { reportBackgroundFailure } from '@/platform/status/background-failures';
import { pushDeletionRepository, retainPushDeletion } from './push-deletions';

const VAPID_PUBLIC_KEY = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY || '';
const PUSH_API_BASE = '/wallet/api/push';

// ── Helpers ─────────────────────────────────────────────────────────────────

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

// ── Feature Detection ───────────────────────────────────────────────────────

export function isPushSupported(): boolean {
  return (
    typeof window !== 'undefined' &&
    'serviceWorker' in navigator &&
    'PushManager' in window &&
    'Notification' in window
  );
}

export function isNotificationSupported(): boolean {
  return typeof window !== 'undefined' && 'Notification' in window;
}

export function getNotificationPermission(): NotificationPermission | 'unsupported' {
  if (!isNotificationSupported()) return 'unsupported';
  return Notification.permission;
}

// ── Service Worker Registration ─────────────────────────────────────────────

export async function getServiceWorkerRegistration(): Promise<ServiceWorkerRegistration | null> {
  if (!('serviceWorker' in navigator)) return null;
  try {
    const existing = await navigator.serviceWorker.getRegistration('/wallet/');
    const registration =
      existing || (await navigator.serviceWorker.register('/wallet/sw.js', { scope: '/wallet/' }));
    if (registration.active) return registration;
    return await navigator.serviceWorker.ready;
  } catch {
    return null;
  }
}

// ── Push Subscription ───────────────────────────────────────────────────────

export async function subscribeToPush(): Promise<PushSubscription | null> {
  return serializePushDeletion(async () => {
    if (!isPushSupported()) return null;
    const recoveringDeletion = pushDeletionRepository.read().length > 0;
    if (!(await reconcilePushDeletions()) || recoveringDeletion) return null;

    const registration = await getServiceWorkerRegistration();
    if (!registration) return null;

    // Check existing subscription
    const existing = await registration.pushManager.getSubscription();
    if (existing) return confirmPushRegistrationNow(existing);

    if (!VAPID_PUBLIC_KEY) {
      console.warn('[Push] No VAPID public key configured');
      return null;
    }

    try {
      const subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(VAPID_PUBLIC_KEY) as BufferSource,
      });

      // Send subscription to backend
      return await confirmPushRegistrationNow(subscription);
    } catch (err) {
      if (err instanceof PushRegistrationError) throw err;
      const failure: PushRegistrationResult = {
        ok: false,
        code: 'BROWSER_UNAVAILABLE',
        retryable: true,
      };
      reportPushRegistrationFailure(failure);
      throw new PushRegistrationError(failure);
    }
  });
}

export async function unsubscribeFromPush(): Promise<boolean> {
  return serializePushDeletion(async () => {
    try {
      const registration = await getServiceWorkerRegistration();
      if (!registration) return false;
      const subscription = await registration.pushManager.getSubscription();
      if (subscription) {
        const wallet = pushWallet;
        if (!wallet) throw new Error('Push deletion requires the owning wallet');
        retainPushDeletion({ endpoint: subscription.endpoint, walletAddress: wallet.address });
        acknowledgedEndpoint = null;
        acknowledgedWallet = null;
      }
      return await reconcilePushDeletions();
    } catch (error) {
      reportPushDeletionFailure(error);
      return false;
    }
  });
}

export async function getCurrentSubscription(): Promise<PushSubscription | null> {
  await retryPendingPushDeletions();
  const registration = await getServiceWorkerRegistration();
  if (!registration) return null;
  const subscription = await registration.pushManager.getSubscription();
  return subscription && isPushSubscriptionEnabled(subscription.endpoint) ? subscription : null;
}

// ── Notification Permission ─────────────────────────────────────────────────

export async function requestNotificationPermission(): Promise<NotificationPermission> {
  if (!isNotificationSupported()) return 'denied';
  const result = await Notification.requestPermission();
  return result;
}

// ── Local Notifications (no push server needed) ─────────────────────────────

export async function showLocalNotification(
  title: string,
  options?: NotificationOptions
): Promise<void> {
  const registration = await getServiceWorkerRegistration();
  if (!registration) return;

  await registration.showNotification(title, {
    icon: '/wallet/icons/android/launchericon-192x192.png',
    badge: '/wallet/icons/android/launchericon-96x96.png',
    ...options,
  } as NotificationOptions & Record<string, unknown>);
}

// ── Backend Communication ───────────────────────────────────────────────────

let pushWallet: { address: string; signMessage: (text: string) => Promise<string> } | null = null;

export function pushOwnershipMessage(
  endpoint: string,
  walletAddress: string,
  issuedAt: number
): string {
  return JSON.stringify({
    action: 'paxport:push-subscribe',
    endpoint,
    walletAddress: walletAddress.toLowerCase(),
    issuedAt,
  });
}

/** Set the active wallet address for push subscription association. */
export function setWalletAddressForPush(
  address: string,
  signMessage: (text: string) => Promise<string>
): void {
  if (!/^0x[0-9a-fA-F]{40}$/.test(address)) throw new TypeError('Push wallet address is invalid');
  acknowledgedEndpoint = null;
  acknowledgedWallet = null;
  pushWallet = { address, signMessage };
  if (typeof window !== 'undefined') window.addEventListener('online', recoverPushDeletions);
  recoverPushDeletions();
}

export function clearWalletAddressForPush(): void {
  pushWallet = null;
  acknowledgedEndpoint = null;
  acknowledgedWallet = null;
  if (typeof window !== 'undefined') window.removeEventListener('online', recoverPushDeletions);
}

export interface PushSubscriptionRecord {
  endpoint: string;
  keys: { p256dh: string; auth: string };
}

export type PushRegistrationResult =
  | { ok: true; endpoint: string; walletAddress: string }
  | {
      ok: false;
      code:
        | 'WALLET_UNAVAILABLE'
        | 'WALLET_CHANGED'
        | 'SIGNING_FAILED'
        | 'SERVER_REFUSED'
        | 'ACK_INVALID'
        | 'NETWORK_UNAVAILABLE'
        | 'BROWSER_UNAVAILABLE';
      retryable: true;
      status?: number;
    };

let acknowledgedEndpoint: string | null = null;
let acknowledgedWallet: string | null = null;

export function isPushSubscriptionEnabled(endpoint: string): boolean {
  return (
    acknowledgedEndpoint === endpoint &&
    acknowledgedWallet !== null &&
    acknowledgedWallet === pushWallet?.address.toLowerCase()
  );
}

export class PushRegistrationError extends Error {
  constructor(readonly result: Extract<PushRegistrationResult, { ok: false }>) {
    super('Notifications could not be enabled. Please retry.');
    this.name = 'PushRegistrationError';
  }
}

function reportPushRegistrationFailure(
  result: Extract<PushRegistrationResult, { ok: false }>
): void {
  acknowledgedEndpoint = null;
  acknowledgedWallet = null;
  reportBackgroundFailure({
    domain: 'platform',
    kind: 'unavailable',
    code: 'PUSH_SUBSCRIBE_FAILED',
    message: `Notifications could not be enabled (${result.code}).`,
    retryable: result.retryable,
  });
}

function subscriptionRecord(subscription: PushSubscription): PushSubscriptionRecord {
  const json = subscription.toJSON();
  if (!json.keys?.p256dh || !json.keys.auth)
    throw new TypeError('Push subscription keys are missing');
  return {
    endpoint: subscription.endpoint,
    keys: { p256dh: json.keys.p256dh, auth: json.keys.auth },
  };
}

export async function sendSubscriptionToServer(
  subscription: PushSubscriptionRecord
): Promise<PushRegistrationResult> {
  return serializePushDeletion(async () => {
    const recoveringDeletion = pushDeletionRepository.read().length > 0;
    if (!(await reconcilePushDeletions()) || recoveringDeletion) {
      return pushRegistrationUnavailable();
    }
    return sendPushRegistration(subscription);
  });
}

async function sendPushRegistration(
  subscription: PushSubscriptionRecord
): Promise<PushRegistrationResult> {
  const wallet = pushWallet;
  const fail = (
    code: Extract<PushRegistrationResult, { ok: false }>['code'],
    status?: number
  ): PushRegistrationResult => {
    const result = {
      ok: false,
      code,
      retryable: true,
      ...(status === undefined ? {} : { status }),
    } as const;
    reportPushRegistrationFailure(result);
    return result;
  };
  if (!wallet) return fail('WALLET_UNAVAILABLE');
  const issuedAt = Math.floor(Date.now() / 1_000);
  let signature: string;
  try {
    signature = await wallet.signMessage(
      pushOwnershipMessage(subscription.endpoint, wallet.address, issuedAt)
    );
  } catch {
    return fail('SIGNING_FAILED');
  }
  if (pushWallet !== wallet) return fail('WALLET_CHANGED');
  try {
    const url = new URL(`${PUSH_API_BASE}/subscribe`, window.location.origin);
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...subscription, walletAddress: wallet.address, signature, issuedAt }),
      signal: AbortSignal.timeout(10_000),
      cache: 'no-store',
    });
    if (!response.ok) return fail('SERVER_REFUSED', response.status);
    const body: unknown = await response.json();
    if (!body || typeof body !== 'object' || !('ok' in body) || body.ok !== true)
      return fail('ACK_INVALID');
    if (pushWallet !== wallet) return fail('WALLET_CHANGED');
    acknowledgedEndpoint = subscription.endpoint;
    acknowledgedWallet = wallet.address.toLowerCase();
    return { ok: true, endpoint: subscription.endpoint, walletAddress: acknowledgedWallet };
  } catch {
    return fail('NETWORK_UNAVAILABLE');
  }
}

function normalizePushSubscription(
  subscription: PushSubscriptionRecord | PushSubscription
): PushSubscriptionRecord {
  if ('keys' in subscription) return subscription;
  return subscriptionRecord(subscription);
}

export async function confirmPushRegistration<T extends PushSubscriptionRecord | PushSubscription>(
  subscription: T
): Promise<T> {
  const record = normalizePushSubscription(subscription);
  const result = await sendSubscriptionToServer(record);
  if (!result.ok) throw new PushRegistrationError(result);
  return subscription;
}

async function confirmPushRegistrationNow<T extends PushSubscriptionRecord | PushSubscription>(
  subscription: T
): Promise<T> {
  const result = await sendPushRegistration(normalizePushSubscription(subscription));
  if (!result.ok) throw new PushRegistrationError(result);
  return subscription;
}

function pushRegistrationUnavailable(): PushRegistrationResult {
  const result = { ok: false, code: 'BROWSER_UNAVAILABLE', retryable: true } as const;
  reportPushRegistrationFailure(result);
  return result;
}

function reportPushDeletionFailure(error: unknown): void {
  console.warn('[Push] Pending subscription deletion:', error);
  reportBackgroundFailure({
    domain: 'platform',
    kind: 'unavailable',
    code: 'PUSH_DELETION_PENDING',
    message: 'Notification removal is pending. Reconnect the owning wallet to retry.',
    retryable: true,
  });
}

let deletionWork: Promise<unknown> = Promise.resolve();

function serializePushDeletion<T>(operation: () => Promise<T>): Promise<T> {
  const work = deletionWork.then(operation);
  deletionWork = work.catch(reportPushDeletionFailure);
  return work;
}

function recoverPushDeletions(): void {
  void retryPendingPushDeletions().catch(reportPushDeletionFailure);
}

export function retryPendingPushDeletions(): Promise<boolean> {
  return serializePushDeletion(reconcilePushDeletions);
}

async function reconcilePushDeletions(): Promise<boolean> {
  const pending = pushDeletionRepository.read();
  for (const intent of pending) {
    const wallet = pushWallet;
    if (!wallet || wallet.address.toLowerCase() !== intent.walletAddress) continue;
    try {
      const registration = await getServiceWorkerRegistration();
      if (!registration) throw new Error('Push service worker unavailable');
      const subscription = await registration.pushManager.getSubscription();
      if (pushWallet !== wallet) continue;
      if (subscription?.endpoint === intent.endpoint && !(await subscription.unsubscribe())) {
        throw new Error('Browser refused push subscription removal');
      }
      if (pushWallet !== wallet) continue;
      await removeSubscriptionFromServer(intent.endpoint, wallet);
      if (pushWallet !== wallet) continue;
      pushDeletionRepository.update((current) =>
        current.filter((item) => item.endpoint !== intent.endpoint)
      );
    } catch (error) {
      reportPushDeletionFailure(error);
    }
  }
  return pushDeletionRepository.read().length === 0;
}

async function removeSubscriptionFromServer(
  endpoint: string,
  wallet: NonNullable<typeof pushWallet>
): Promise<void> {
  const issuedAt = Math.floor(Date.now() / 1_000);
  const signature = await wallet.signMessage(
    pushUnsubscribeMessage(endpoint, wallet.address, issuedAt)
  );
  if (pushWallet !== wallet) throw new Error('Push deletion wallet changed while signing');
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 10_000);
  try {
    const response = await fetch(`${PUSH_API_BASE}/subscribe`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint, walletAddress: wallet.address, issuedAt, signature }),
      signal: controller.signal,
    });
    if (!response.ok) throw new Error(`Push deletion refused (${response.status})`);
    const acknowledgement: unknown = await response.json();
    if (
      typeof acknowledgement !== 'object' ||
      acknowledgement === null ||
      !('ok' in acknowledgement) ||
      acknowledgement.ok !== true
    ) {
      throw new Error('Push deletion was not acknowledged');
    }
  } finally {
    clearTimeout(timer);
  }
}

/** Update the server that this subscription is still active (heartbeat). */
export async function heartbeat(
  subscription?: PushSubscriptionRecord
): Promise<PushRegistrationResult> {
  return serializePushDeletion(async () => {
    const recoveringDeletion = pushDeletionRepository.read().length > 0;
    if (!(await reconcilePushDeletions()) || recoveringDeletion) {
      return pushRegistrationUnavailable();
    }
    if (subscription) return sendPushRegistration(subscription);
    const registration = await getServiceWorkerRegistration();
    const current = await registration?.pushManager.getSubscription();
    if (!current) return pushRegistrationUnavailable();
    return sendPushRegistration(subscriptionRecord(current));
  });
}

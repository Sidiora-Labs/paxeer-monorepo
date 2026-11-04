import { reportBackgroundFailure } from '@/platform/status/background-failures';

// ── Native Push Notification Helpers ─────────────────────────────────────────
// Manages Web Push subscriptions without any third-party service.
// To send push from your backend, use the subscription object with web-push lib.

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
  if (!isPushSupported()) return null;

  const registration = await getServiceWorkerRegistration();
  if (!registration) return null;

  // Check existing subscription
  const existing = await registration.pushManager.getSubscription();
  if (existing) return confirmPushRegistration(existing);

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
    return await confirmPushRegistration(subscription);
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
}

export async function unsubscribeFromPush(): Promise<boolean> {
  const registration = await getServiceWorkerRegistration();
  if (!registration) return false;

  const subscription = await registration.pushManager.getSubscription();
  if (!subscription) return true;

  try {
    const success = await subscription.unsubscribe();
    if (success) {
      await removeSubscriptionFromServer(subscription);
    }
    return success;
  } catch (err) {
    console.error('[Push] Unsubscribe failed:', err);
    return false;
  }
}

export async function getCurrentSubscription(): Promise<PushSubscription | null> {
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
}

export function clearWalletAddressForPush(): void {
  pushWallet = null;
  acknowledgedEndpoint = null;
  acknowledgedWallet = null;
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

async function removeSubscriptionFromServer(subscription: PushSubscription): Promise<void> {
  try {
    await fetch(`${PUSH_API_BASE}/subscribe`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint: subscription.endpoint }),
    });
  } catch (err) {
    console.warn('[Push] Failed to remove subscription from server:', err);
  }
}

/** Update the server that this subscription is still active (heartbeat). */
export async function heartbeat(
  subscription?: PushSubscriptionRecord
): Promise<PushRegistrationResult> {
  if (subscription) return sendSubscriptionToServer(subscription);
  const registration = await getServiceWorkerRegistration();
  const current = await registration?.pushManager.getSubscription();
  if (!current) {
    const result = { ok: false, code: 'BROWSER_UNAVAILABLE', retryable: true } as const;
    reportPushRegistrationFailure(result);
    return result;
  }
  return sendSubscriptionToServer(subscriptionRecord(current));
}

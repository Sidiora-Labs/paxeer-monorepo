// ── Automatic Notification Triggers ──────────────────────────────────────────
// Client-side polling for tx received, inactivity nudge, and feature announcements.
// Runs inside the app (not the SW) — uses showLocalNotification from push.ts.

import {
  showLocalNotification,
  isPushSupported,
  getNotificationPermission,
  setWalletAddressForPush,
  clearWalletAddressForPush,
  subscribeToPush,
  heartbeat,
} from './push';
import { notificationStateRepository } from '@/platform/storage/repositories';
import { reportBackgroundFailure } from '@/platform/status/background-failures';

import { NotificationDelivery } from './notification-delivery';
import { selectedWalletChainId } from './wallet-read-contract';

let activeWallet: string | null = null;
let delivery: NotificationDelivery | null = null;

const INACTIVITY_THRESHOLD_MS = 3 * 24 * 60 * 60 * 1000; // 3 days

let incomingTimer: ReturnType<typeof setInterval> | null = null;
let inactivityTimer: ReturnType<typeof setInterval> | null = null;

function canNotify(): boolean {
  return isPushSupported() && getNotificationPermission() === 'granted';
}

// The count is only a wake-up signal. Direction and identity come from committed indexer activity.
export async function handleTxCount(_currentCount: number): Promise<void> {
  if (!canNotify() || !activeWallet || !delivery) return;
  try {
    await delivery.checkIncoming(activeWallet, selectedWalletChainId());
  } catch {
    reportBackgroundFailure({
      domain: 'platform',
      kind: 'unavailable',
      code: 'NOTIFICATION_HISTORY_UNAVAILABLE',
      message: 'Incoming transfer notifications could not be refreshed.',
      retryable: true,
    });
  }
}

// ── Inactivity nudge ────────────────────────────────────────────────────────

export function recordActivity(): void {
  notificationStateRepository.update((current) => ({
    ...current,
    lastActive: Date.now(),
  }));
}

async function checkInactivity(): Promise<void> {
  if (!canNotify()) return;

  const lastActive = notificationStateRepository.read().lastActive;
  if (lastActive === null) {
    recordActivity();
    return;
  }

  const elapsed = Date.now() - lastActive;

  if (elapsed >= INACTIVITY_THRESHOLD_MS) {
    await showLocalNotification('We miss you!', {
      body: 'Check your portfolio — your tokens may have moved.',
      tag: 'inactivity',
      data: { url: '/' },
    } as NotificationOptions);
    // Reset so we don't spam — next nudge after another 3 days
    recordActivity();
  }
}

// ── Feature announcements ───────────────────────────────────────────────────

async function checkFeatureAnnouncement(): Promise<void> {
  if (!canNotify() || !delivery) return;
  try {
    await delivery.checkCampaign();
  } catch {
    reportBackgroundFailure({
      domain: 'platform',
      kind: 'unavailable',
      code: 'CAMPAIGN_ANNOUNCEMENT_UNAVAILABLE',
      message: 'The campaign announcement could not be loaded.',
      retryable: true,
    });
  }
}

// ── Lifecycle ───────────────────────────────────────────────────────────────

export async function startNotificationTriggers(
  walletAddress: string,
  signMessage: (text: string) => Promise<string>
): Promise<void> {
  stopNotificationTriggers();

  activeWallet = walletAddress;
  delivery = new NotificationDelivery(showLocalNotification);
  const currentDelivery = delivery;

  // Associate wallet address with push subscription on the server
  setWalletAddressForPush(walletAddress, signMessage);

  // Ensure the browser is subscribed to push (registers with our backend)
  try {
    await subscribeToPush();
  } catch {
    reportBackgroundFailure({
      domain: 'platform',
      kind: 'unavailable',
      code: 'PUSH_SUBSCRIBE_FAILED',
      message: 'Notifications could not be enabled.',
      retryable: true,
    });
  }

  if (delivery !== currentDelivery || !canNotify()) return;

  await handleTxCount(0);
  if (delivery !== currentDelivery) return;
  incomingTimer = setInterval(() => {
    void handleTxCount(0);
  }, 60_000);

  // Record activity on start
  recordActivity();

  // Check feature announcements once on start (local fallback)
  await checkFeatureAnnouncement();
  if (delivery !== currentDelivery) return;

  // New-tx detection is driven by useTxCountQuery in useNotificationLifecycle.
  // Heartbeat every 10 minutes to keep server-side subscription active
  inactivityTimer = setInterval(
    () => {
      void heartbeat().catch(() =>
        reportBackgroundFailure({
          domain: 'platform',
          kind: 'unavailable',
          code: 'PUSH_SUBSCRIBE_FAILED',
          message: 'Notification registration could not be refreshed.',
          retryable: true,
        })
      );
    },
    10 * 60 * 1000
  );
}

export function stopNotificationTriggers(): void {
  activeWallet = null;
  delivery?.stop();
  delivery = null;
  clearWalletAddressForPush();
  if (incomingTimer) {
    clearInterval(incomingTimer);
    incomingTimer = null;
  }
  if (inactivityTimer) {
    clearInterval(inactivityTimer);
    inactivityTimer = null;
  }
}

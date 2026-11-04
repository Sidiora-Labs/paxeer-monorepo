import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { ECDH } from 'node:crypto';
import { AtomicJsonStore } from './atomic-json-store';

const DATA_DIR = process.env.PUSH_DATA_DIR ?? '/data/paxport/push';

export interface StoredSubscription {
  endpoint: string;
  keys: { p256dh: string; auth: string };
  walletAddress: string;
  userAgent?: string;
  createdAt: number;
  lastActiveAt: number;
  lastNotifiedAt?: number;
  tags?: string[];
}

export type RecipientStatus =
  | 'pending'
  | 'in_flight'
  | 'sent'
  | 'removed'
  | 'retry'
  | 'uncertain'
  | 'exhausted';

export interface CampaignRecipient {
  endpoint: string;
  status: RecipientStatus;
  attempts: number;
  retryAt?: number;
}

export interface CampaignClaim {
  token: string;
  expiresAt: number;
}

export const CAMPAIGN_CLAIM_MS = 60_000;
export const CAMPAIGN_RETRY_MS = 60_000;
export const CAMPAIGN_MAX_ATTEMPTS = 5;
export const INACTIVITY_COOLDOWN_MS = 3 * 24 * 60 * 60 * 1000;

export interface Campaign {
  id: string;
  title: string;
  body: string;
  url?: string;
  icon?: string;
  image?: string;
  tag?: string;
  targetTags?: string[];
  targetAddresses?: string[];
  scheduledAt?: number;
  sentAt?: number;
  sentCount?: number;
  failedCount?: number;
  recipients?: CampaignRecipient[];
  claim?: CampaignClaim;
}

interface StoreEnvelope<T> {
  version: 1;
  updatedAt: number;
  records: T[];
}

function isRecord(input: unknown): input is Record<string, unknown> {
  return typeof input === 'object' && input !== null && !Array.isArray(input);
}

function boundedString(input: unknown, name: string, minimum: number, maximum: number): string {
  if (typeof input !== 'string' || input.length < minimum || input.length > maximum) {
    throw new TypeError(`${name} is invalid`);
  }
  return input;
}

function parseStringArray(
  input: unknown,
  name: string,
  maximumItems: number,
  maximumLength: number
): string[] | undefined {
  if (input === undefined) return undefined;
  if (!Array.isArray(input) || input.length > maximumItems) {
    throw new TypeError(`${name} is invalid`);
  }
  return input.map((value) => boundedString(value, name, 1, maximumLength));
}

function parseTimestamp(input: unknown, name: string): number {
  if (typeof input !== 'number' || !Number.isSafeInteger(input) || input < 0) {
    throw new TypeError(`${name} is invalid`);
  }
  return input;
}

// Exact provider origins keep subscriber-controlled hosts and DNS out of egress.
const PUSH_ORIGINS = new Set([
  'https://fcm.googleapis.com',
  'https://updates.push.services.mozilla.com',
  'https://web.push.apple.com',
]);

function parsePushKey(input: unknown, name: string, bytes: number): string {
  const value = boundedString(input, name, 1, 256);
  const decoded = Buffer.from(value, 'base64url');
  if (
    !/^[A-Za-z0-9_-]+$/.test(value) ||
    decoded.length !== bytes ||
    decoded.toString('base64url') !== value
  ) {
    throw new TypeError(`${name} is invalid`);
  }
  if (bytes === 65) {
    if (decoded[0] !== 4) throw new TypeError(`${name} is invalid`);
    try {
      ECDH.convertKey(decoded, 'prime256v1', undefined, undefined, 'uncompressed');
    } catch {
      throw new TypeError(`${name} is invalid`);
    }
  }
  return value;
}

export function parseStoredSubscription(input: unknown): StoredSubscription {
  if (!isRecord(input) || !isRecord(input.keys)) {
    throw new TypeError('Subscription is invalid');
  }
  const endpoint = boundedString(input.endpoint, 'endpoint', 12, 2048);
  const parsedEndpoint = new URL(endpoint);
  if (
    !PUSH_ORIGINS.has(parsedEndpoint.origin) ||
    parsedEndpoint.username !== '' ||
    parsedEndpoint.password !== '' ||
    parsedEndpoint.hash !== '' ||
    endpoint !== parsedEndpoint.href ||
    /[\\\s]/.test(endpoint)
  ) {
    throw new TypeError('endpoint is invalid');
  }
  const walletAddress = boundedString(input.walletAddress, 'walletAddress', 42, 42).toLowerCase();
  if (!/^0x[0-9a-f]{40}$/.test(walletAddress)) {
    throw new TypeError('walletAddress is invalid');
  }
  return {
    endpoint,
    keys: {
      p256dh: parsePushKey(input.keys.p256dh, 'keys.p256dh', 65),
      auth: parsePushKey(input.keys.auth, 'keys.auth', 16),
    },
    walletAddress,
    userAgent:
      input.userAgent === undefined
        ? undefined
        : boundedString(input.userAgent, 'userAgent', 0, 512),
    createdAt: parseTimestamp(input.createdAt, 'createdAt'),
    lastActiveAt: parseTimestamp(input.lastActiveAt, 'lastActiveAt'),
    lastNotifiedAt:
      input.lastNotifiedAt === undefined
        ? undefined
        : parseTimestamp(input.lastNotifiedAt, 'lastNotifiedAt'),
    tags: parseStringArray(input.tags, 'tags', 32, 64),
  };
}

function parseRecipient(input: unknown): CampaignRecipient {
  if (
    !isRecord(input) ||
    !['pending', 'in_flight', 'sent', 'removed', 'retry', 'uncertain', 'exhausted'].includes(
      String(input.status)
    )
  ) {
    throw new TypeError('Campaign recipient is invalid');
  }
  const attempts = parseTimestamp(input.attempts, 'attempts');
  if (attempts > CAMPAIGN_MAX_ATTEMPTS) throw new TypeError('Too many campaign attempts');
  return {
    endpoint: boundedString(input.endpoint, 'endpoint', 12, 2048),
    status: input.status as RecipientStatus,
    attempts,
    retryAt: input.retryAt === undefined ? undefined : parseTimestamp(input.retryAt, 'retryAt'),
  };
}

export function parseCampaign(input: unknown): Campaign {
  if (!isRecord(input)) throw new TypeError('Campaign is invalid');
  const optionalString = (value: unknown, name: string, maximum: number): string | undefined =>
    value === undefined ? undefined : boundedString(value, name, 1, maximum);
  let claim: CampaignClaim | undefined;
  if (input.claim !== undefined) {
    if (!isRecord(input.claim)) throw new TypeError('Campaign claim is invalid');
    claim = {
      token: boundedString(input.claim.token, 'token', 1, 128),
      expiresAt: parseTimestamp(input.claim.expiresAt, 'expiresAt'),
    };
  }
  if (input.recipients !== undefined && !Array.isArray(input.recipients))
    throw new TypeError('Campaign recipients are invalid');
  return {
    recipients: input.recipients?.map(parseRecipient),
    claim,
    id: boundedString(input.id, 'id', 1, 128),
    title: boundedString(input.title, 'title', 1, 120),
    body: boundedString(input.body, 'body', 1, 500),
    url: optionalString(input.url, 'url', 512),
    icon: optionalString(input.icon, 'icon', 512),
    image: optionalString(input.image, 'image', 512),
    tag: optionalString(input.tag, 'tag', 128),
    targetTags: parseStringArray(input.targetTags, 'targetTags', 32, 64),
    targetAddresses: parseStringArray(input.targetAddresses, 'targetAddresses', 500, 42),
    scheduledAt:
      input.scheduledAt === undefined
        ? undefined
        : parseTimestamp(input.scheduledAt, 'scheduledAt'),
    sentAt: input.sentAt === undefined ? undefined : parseTimestamp(input.sentAt, 'sentAt'),
    sentCount:
      input.sentCount === undefined ? undefined : parseTimestamp(input.sentCount, 'sentCount'),
    failedCount:
      input.failedCount === undefined
        ? undefined
        : parseTimestamp(input.failedCount, 'failedCount'),
  };
}

function envelopeParser<T>(
  itemParser: (input: unknown) => T
): (input: unknown) => StoreEnvelope<T> {
  return (input) => {
    if (!isRecord(input) || input.version !== 1 || !Array.isArray(input.records)) {
      throw new TypeError('Store envelope is invalid');
    }
    return {
      version: 1,
      updatedAt: parseTimestamp(input.updatedAt, 'updatedAt'),
      records: input.records.map(itemParser),
    };
  };
}

function legacyMigration<T>(
  itemParser: (input: unknown) => T
): (input: unknown) => StoreEnvelope<T> {
  return (input) => {
    if (!Array.isArray(input)) throw new TypeError('Legacy store is invalid');
    return {
      version: 1,
      updatedAt: Date.now(),
      records: input.map(itemParser),
    };
  };
}

export function createPushStore(dataDirectory: string) {
  const subscriptions = new AtomicJsonStore<StoreEnvelope<StoredSubscription>>({
    filePath: path.join(dataDirectory, 'subscriptions.json'),
    empty: () => ({ version: 1, updatedAt: Date.now(), records: [] }),
    parse: envelopeParser(parseStoredSubscription),
    migrate: legacyMigration(parseStoredSubscription),
  });

  const campaigns = new AtomicJsonStore<StoreEnvelope<Campaign>>({
    filePath: path.join(dataDirectory, 'campaigns.json'),
    empty: () => ({ version: 1, updatedAt: Date.now(), records: [] }),
    parse: envelopeParser(parseCampaign),
    migrate: legacyMigration(parseCampaign),
  });

  async function upsertSubscription(input: StoredSubscription): Promise<void> {
    const subscription = parseStoredSubscription(input);
    await subscriptions.update((store) => {
      const existing = store.records.find(
        (candidate) => candidate.endpoint === subscription.endpoint
      );
      if (existing && existing.walletAddress !== subscription.walletAddress) {
        throw new TypeError('Subscription owner cannot change');
      }
      const now = Date.now();
      return {
        version: 1,
        updatedAt: Date.now(),
        records: [
          ...store.records.filter((candidate) => candidate.endpoint !== subscription.endpoint),
          {
            ...(existing ?? {
              endpoint: subscription.endpoint,
              walletAddress: subscription.walletAddress,
              createdAt: now,
            }),
            keys: subscription.keys,
            userAgent: subscription.userAgent,
            lastActiveAt: now,
          },
        ],
      };
    });
  }

  async function removeSubscription(endpoint: string): Promise<boolean> {
    let removed = false;
    await subscriptions.update((store) => {
      const records = store.records.filter((item) => item.endpoint !== endpoint);
      removed = records.length !== store.records.length;
      return { version: 1, updatedAt: Date.now(), records };
    });
    return removed;
  }

  async function getSubscription(endpoint: string): Promise<StoredSubscription | null> {
    const store = await subscriptions.read();
    return store.records.find((item) => item.endpoint === endpoint) ?? null;
  }

  async function getSubscriptionsByWallet(walletAddress: string): Promise<StoredSubscription[]> {
    const store = await subscriptions.read();
    const normalized = walletAddress.toLowerCase();
    return store.records.filter((item) => item.walletAddress === normalized);
  }

  async function getSubscriptionsByTag(tag: string): Promise<StoredSubscription[]> {
    const store = await subscriptions.read();
    return store.records.filter((item) => item.tags?.includes(tag));
  }

  async function getAllSubscriptions(): Promise<StoredSubscription[]> {
    return (await subscriptions.read()).records;
  }

  async function getSubscriptionCount(): Promise<number> {
    return (await subscriptions.read()).records.length;
  }

  async function updateLastNotified(endpoints: string[]): Promise<void> {
    const targets = new Set(endpoints);
    await subscriptions.update((store) => ({
      version: 1,
      updatedAt: Date.now(),
      records: store.records.map((item) =>
        targets.has(item.endpoint) ? { ...item, lastNotifiedAt: Date.now() } : item
      ),
    }));
  }

  async function addTagToWallet(walletAddress: string, tag: string): Promise<number> {
    let count = 0;
    const normalized = walletAddress.toLowerCase();
    await subscriptions.update((store) => ({
      version: 1,
      updatedAt: Date.now(),
      records: store.records.map((item) => {
        if (item.walletAddress !== normalized || item.tags?.includes(tag)) return item;
        count += 1;
        return { ...item, tags: [...(item.tags ?? []), tag] };
      }),
    }));
    return count;
  }

  async function getInactiveSubscriptions(thresholdMs: number): Promise<StoredSubscription[]> {
    const cutoff = Date.now() - thresholdMs;
    return (await subscriptions.read()).records.filter(
      (item) =>
        item.lastActiveAt < cutoff &&
        (item.lastNotifiedAt === undefined ||
          item.lastNotifiedAt <= Date.now() - INACTIVITY_COOLDOWN_MS)
    );
  }

  async function reserveInactiveSubscriptions(
    thresholdMs: number,
    now = Date.now()
  ): Promise<StoredSubscription[]> {
    const selected: StoredSubscription[] = [];
    await subscriptions.update((store) => ({
      ...store,
      updatedAt: now,
      records: store.records.map((item) => {
        if (
          item.lastActiveAt >= now - thresholdMs ||
          (item.lastNotifiedAt !== undefined && item.lastNotifiedAt > now - INACTIVITY_COOLDOWN_MS)
        )
          return item;
        const reserved = { ...item, lastNotifiedAt: now };
        selected.push(reserved);
        return reserved;
      }),
    }));
    return selected;
  }

  async function addCampaign(input: Campaign): Promise<void> {
    const campaign = parseCampaign(input);
    await campaigns.update((store) => {
      if (store.records.some((item) => item.id === campaign.id)) {
        throw new Error('Campaign already exists');
      }
      return {
        version: 1,
        updatedAt: Date.now(),
        records: [...store.records, campaign],
      };
    });
  }

  async function getCampaign(id: string): Promise<Campaign | null> {
    return (await campaigns.read()).records.find((item) => item.id === id) ?? null;
  }

  async function getPendingCampaigns(): Promise<Campaign[]> {
    const now = Date.now();
    return (await campaigns.read()).records.filter(
      (item) => !item.sentAt && (!item.scheduledAt || item.scheduledAt <= now)
    );
  }

  async function markCampaignSent(
    id: string,
    sentCount: number,
    failedCount: number
  ): Promise<void> {
    await campaigns.update((store) => ({
      version: 1,
      updatedAt: Date.now(),
      records: store.records.map((item) =>
        item.id === id ? completeCampaign(item, sentCount, failedCount) : item
      ),
    }));
  }

  async function getAllCampaigns(): Promise<Campaign[]> {
    return (await campaigns.read()).records;
  }

  async function claimCampaign(
    id: string,
    candidates: StoredSubscription[],
    now = Date.now()
  ): Promise<Campaign | null> {
    let claimed: Campaign | null = null;
    await campaigns.update((store) => ({
      ...store,
      updatedAt: now,
      records: store.records.map((item) => {
        if (
          item.id !== id ||
          item.sentAt !== undefined ||
          (item.scheduledAt !== undefined && item.scheduledAt > now) ||
          (item.claim && item.claim.expiresAt > now)
        )
          return item;
        const recipients = (
          item.recipients ??
          candidates.map(
            (sub): CampaignRecipient => ({ endpoint: sub.endpoint, status: 'pending', attempts: 0 })
          )
        ).map(
          (recipient): CampaignRecipient =>
            recipient.status === 'in_flight' ? { ...recipient, status: 'uncertain' } : recipient
        );
        claimed = {
          ...item,
          recipients,
          claim: { token: randomUUID(), expiresAt: now + CAMPAIGN_CLAIM_MS },
        };
        return claimed;
      }),
    }));
    return claimed;
  }

  async function mutateClaim(
    id: string,
    token: string,
    mutate: (item: Campaign) => Campaign,
    now: number
  ): Promise<void> {
    await campaigns.update((store) => ({
      ...store,
      updatedAt: now,
      records: store.records.map((item) => {
        if (item.id !== id) return item;
        if (item.claim?.token !== token || item.claim.expiresAt <= now)
          throw new Error('Campaign claim lost');
        return mutate(item);
      }),
    }));
  }

  async function beginCampaignAttempt(
    id: string,
    token: string,
    endpoint: string,
    now = Date.now()
  ): Promise<void> {
    await mutateClaim(
      id,
      token,
      (item) => ({
        ...item,
        recipients: item.recipients?.map((recipient) => {
          if (recipient.endpoint !== endpoint) return recipient;
          if (
            !['pending', 'retry'].includes(recipient.status) ||
            (recipient.retryAt !== undefined && recipient.retryAt > now) ||
            recipient.attempts >= CAMPAIGN_MAX_ATTEMPTS
          )
            throw new Error('Campaign recipient is not eligible');
          return {
            ...recipient,
            status: 'in_flight',
            attempts: recipient.attempts + 1,
            retryAt: undefined,
          };
        }),
      }),
      now
    );
  }

  async function recordCampaignAttempt(
    id: string,
    token: string,
    endpoint: string,
    outcome: 'sent' | 'removed' | 'retry' | 'uncertain',
    now = Date.now()
  ): Promise<void> {
    await mutateClaim(
      id,
      token,
      (item) => ({
        ...item,
        recipients: item.recipients?.map((recipient) => {
          if (recipient.endpoint !== endpoint) return recipient;
          if (recipient.status !== 'in_flight')
            throw new Error('Campaign recipient has no active attempt');
          const status =
            outcome === 'retry' && recipient.attempts >= CAMPAIGN_MAX_ATTEMPTS
              ? 'exhausted'
              : outcome;
          return {
            ...recipient,
            status,
            retryAt:
              status === 'retry'
                ? now + Math.min(3_600_000, CAMPAIGN_RETRY_MS * 2 ** (recipient.attempts - 1))
                : undefined,
          };
        }),
      }),
      now
    );
  }

  async function finishCampaignClaim(id: string, token: string, now = Date.now()): Promise<void> {
    await mutateClaim(
      id,
      token,
      (item) => {
        const recipients = item.recipients ?? [];
        const sentCount = recipients.filter((recipient) => recipient.status === 'sent').length;
        const failedCount = recipients.filter(
          (recipient) => recipient.status === 'removed' || recipient.status === 'exhausted'
        ).length;
        return {
          ...item,
          claim: undefined,
          sentCount,
          failedCount,
          sentAt: recipients.every(isTerminalRecipient) ? now : undefined,
        };
      },
      now
    );
  }

  async function reconcileCampaignRecipient(
    id: string,
    endpoint: string,
    outcome: 'sent' | 'retry',
    now = Date.now()
  ): Promise<void> {
    await campaigns.update((store) => ({
      ...store,
      updatedAt: now,
      records: store.records.map((item) => {
        if (item.id !== id) return item;
        if (item.claim && item.claim.expiresAt > now) throw new Error('Campaign is still claimed');
        return {
          ...item,
          recipients: item.recipients?.map((recipient) => {
            if (recipient.endpoint !== endpoint) return recipient;
            if (recipient.status !== 'uncertain' && recipient.status !== 'in_flight')
              throw new Error('Recipient is not uncertain');
            const status =
              outcome === 'retry' && recipient.attempts >= CAMPAIGN_MAX_ATTEMPTS
                ? 'exhausted'
                : outcome;
            return { ...recipient, status, retryAt: status === 'retry' ? now : undefined };
          }),
        };
      }),
    }));
  }

  return {
    upsertSubscription,
    removeSubscription,
    getSubscription,
    getSubscriptionsByWallet,
    getSubscriptionsByTag,
    getAllSubscriptions,
    getSubscriptionCount,
    updateLastNotified,
    addTagToWallet,
    getInactiveSubscriptions,
    reserveInactiveSubscriptions,
    addCampaign,
    getCampaign,
    getPendingCampaigns,
    markCampaignSent,
    getAllCampaigns,
    claimCampaign,
    beginCampaignAttempt,
    recordCampaignAttempt,
    finishCampaignClaim,
    reconcileCampaignRecipient,
  };
}

function isTerminalRecipient(recipient: CampaignRecipient): boolean {
  return ['sent', 'removed', 'exhausted'].includes(recipient.status);
}

function completeCampaign(item: Campaign, sentCount: number, failedCount: number): Campaign {
  if (
    !item.recipients ||
    item.claim ||
    item.recipients.some((recipient) => !isTerminalRecipient(recipient))
  )
    throw new Error('Campaign has unsettled recipients');
  return { ...item, sentAt: Date.now(), sentCount, failedCount };
}

export type PushStore = ReturnType<typeof createPushStore>;
export const pushStore = createPushStore(DATA_DIR);
export const {
  upsertSubscription,
  removeSubscription,
  getSubscription,
  getSubscriptionsByWallet,
  getSubscriptionsByTag,
  getAllSubscriptions,
  getSubscriptionCount,
  updateLastNotified,
  addTagToWallet,
  getInactiveSubscriptions,
  reserveInactiveSubscriptions,
  addCampaign,
  getCampaign,
  getPendingCampaigns,
  markCampaignSent,
  getAllCampaigns,
  claimCampaign,
  beginCampaignAttempt,
  recordCampaignAttempt,
  finishCampaignClaim,
  reconcileCampaignRecipient,
} = pushStore;

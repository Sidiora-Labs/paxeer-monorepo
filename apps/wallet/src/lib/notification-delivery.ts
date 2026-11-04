import { defineStorageRepository } from '@/platform/storage/registry';
import { getAddressTransactions, getAddressTokenTransfers } from './data/api/addresses';
import type { Transaction, TokenTransfer, PaginatedResponse } from './data';
import { boundedHistoryParams } from './wallet-history';
import { validateWalletChainId } from './wallet-read-contract';

interface StreamCursor {
  block: number;
  events: string[];
}
interface AccountCursor {
  native: StreamCursor;
  token: StreamCursor;
}
interface DeliveryState {
  accounts: Record<string, AccountCursor>;
  campaigns: string[];
}

function parseCursor(input: unknown): StreamCursor {
  if (
    !input ||
    typeof input !== 'object' ||
    !('block' in input) ||
    !('events' in input) ||
    !Number.isSafeInteger(input.block) ||
    Number(input.block) < 0 ||
    !Array.isArray(input.events) ||
    input.events.length > 1024 ||
    input.events.some((id) => typeof id !== 'string' || id.length > 160)
  )
    throw new TypeError('Notification cursor is invalid');
  return { block: Number(input.block), events: input.events as string[] };
}

const deliveryRepository = defineStorageRepository<DeliveryState>({
  id: 'notification-delivery',
  key: 'paxport:v1:notification-delivery',
  owner: 'notifications',
  schema: 'ChainWalletEventCursorsAndCampaignIds',
  version: 1,
  area: 'local',
  sensitivity: 'private-metadata',
  retention: 'Until logout, account removal, reset or uninstall',
  quotaBytes: 262144,
  migration:
    'Global transaction counts and relative announcement versions are not event identities',
  resetOn: ['logout', 'account-removal', 'reset', 'uninstall'],
  corruption: 'fail-closed',
  prohibitedData: ['private key', 'mnemonic', 'push secret'],
  fallback: () => ({ accounts: {}, campaigns: [] }),
  parse(input): DeliveryState {
    if (
      !input ||
      typeof input !== 'object' ||
      !('accounts' in input) ||
      !input.accounts ||
      typeof input.accounts !== 'object' ||
      Array.isArray(input.accounts) ||
      Object.keys(input.accounts).length > 32 ||
      !('campaigns' in input) ||
      !Array.isArray(input.campaigns) ||
      input.campaigns.length > 256 ||
      input.campaigns.some((id) => typeof id !== 'string' || !/^[a-zA-Z0-9_-]{1,80}$/.test(id))
    )
      throw new TypeError('Notification delivery state is invalid');
    const accounts = Object.fromEntries(
      Object.entries(input.accounts).map(([key, value]) => {
        if (
          !/^\d+:0x[0-9a-f]{40}$/.test(key) ||
          !value ||
          typeof value !== 'object' ||
          !('native' in value) ||
          !('token' in value)
        )
          throw new TypeError('Notification account cursor is invalid');
        validateWalletChainId(Number(key.split(':')[0]));
        return [key, { native: parseCursor(value.native), token: parseCursor(value.token) }];
      })
    );
    return { accounts, campaigns: input.campaigns as string[] };
  },
});

export interface CampaignAnnouncement {
  id: string;
  title: string;
  startsAt: number;
  endsAt: number;
  eligibility: { text: string; sourceUrl: string; verifiedAt: number };
}

export function parseCampaignAnnouncement(input: unknown): CampaignAnnouncement {
  if (!input || typeof input !== 'object') throw new TypeError('Campaign announcement is invalid');
  const campaign = input as CampaignAnnouncement;
  if (
    typeof campaign.id !== 'string' ||
    !/^[A-Za-z0-9_-]{1,80}$/.test(campaign.id) ||
    typeof campaign.title !== 'string' ||
    campaign.title.length < 1 ||
    campaign.title.length > 120 ||
    !Number.isSafeInteger(campaign.startsAt) ||
    !Number.isSafeInteger(campaign.endsAt) ||
    campaign.startsAt < 0 ||
    campaign.endsAt <= campaign.startsAt ||
    !campaign.eligibility ||
    typeof campaign.eligibility.text !== 'string' ||
    campaign.eligibility.text.length < 1 ||
    campaign.eligibility.text.length > 500 ||
    !Number.isSafeInteger(campaign.eligibility.verifiedAt) ||
    campaign.eligibility.verifiedAt < 0 ||
    campaign.eligibility.verifiedAt > campaign.startsAt
  )
    throw new TypeError('Campaign announcement is invalid');
  const source = new URL(campaign.eligibility.sourceUrl);
  if (source.protocol !== 'https:' || source.username || source.password)
    throw new TypeError('Campaign eligibility source is invalid');
  return campaign;
}

export type NotificationSink = (title: string, options: NotificationOptions) => Promise<void>;
interface Activity {
  block: number;
  id: string;
  incoming: boolean;
}

function committedBlock(block: number | null): block is number {
  return block !== null && Number.isSafeInteger(block) && block > 0;
}
function positiveAmount(value: string): boolean {
  return /^\d+$/.test(value) && BigInt(value) > 0n;
}
function nativeActivity(tx: Transaction, address: string): Activity | null {
  if (!committedBlock(tx.block_number) || tx.status !== 'ok') return null;
  return {
    block: tx.block_number,
    id: `native:${tx.hash.toLowerCase()}`,
    incoming:
      tx.to?.hash.toLowerCase() === address &&
      tx.from?.hash.toLowerCase() !== address &&
      positiveAmount(tx.value),
  };
}
function tokenActivity(transfer: TokenTransfer, address: string): Activity | null {
  if (!committedBlock(transfer.block_number) || !/^0x[0-9a-f]{64}$/i.test(transfer.block_hash))
    return null;
  const amount = transfer.total && 'value' in transfer.total ? transfer.total.value : null;
  return {
    block: transfer.block_number,
    id: `token:${transfer.transaction_hash.toLowerCase()}:${transfer.log_index}`,
    incoming:
      transfer.to?.hash.toLowerCase() === address &&
      transfer.from?.hash.toLowerCase() !== address &&
      typeof amount === 'string' &&
      positiveAmount(amount),
  };
}

async function collectStream<T>(
  load: (cursor?: Record<string, unknown> | null) => Promise<PaginatedResponse<T>>,
  map: (item: T) => Activity | null,
  previous?: StreamCursor
): Promise<{ cursor: StreamCursor; incoming: Activity[] }> {
  let next: Record<string, unknown> | null | undefined;
  const activities = new Map<string, Activity>();
  const visited = new Set<string>();
  for (let page = 0; page < 20; page += 1) {
    const response = await load(next);
    const batch = response.items.map(map).filter((item): item is Activity => item !== null);
    for (const activity of batch) activities.set(activity.id, activity);
    if (
      !previous ||
      response.next_page_params === null ||
      batch.some((item) => item.block < previous.block)
    )
      break;
    const identity = JSON.stringify(response.next_page_params);
    if (visited.has(identity) || page === 19)
      throw new Error('Notification history exceeded the bounded catch-up window');
    visited.add(identity);
    next = response.next_page_params;
  }
  const entries = [...activities.values()];
  const block = Math.max(previous?.block ?? 0, ...entries.map((item) => item.block));
  const ids = new Set(block === previous?.block ? previous.events : []);
  for (const item of entries) if (item.block === block) ids.add(item.id);
  if (ids.size > 1024) throw new Error('Notification block contains too many events');
  return {
    cursor: { block, events: [...ids] },
    incoming: previous
      ? entries.filter(
          (item) =>
            item.incoming &&
            (item.block > previous.block ||
              (item.block === previous.block && !previous.events.includes(item.id)))
        )
      : [],
  };
}

const pending = new Map<string, Promise<void>>();

export class NotificationDelivery {
  private stopped = false;
  constructor(private readonly notify: NotificationSink) {}
  stop(): void {
    this.stopped = true;
  }

  async checkIncoming(walletAddress: string, chainId: number): Promise<void> {
    validateWalletChainId(chainId);
    if (!/^0x[0-9a-f]{40}$/i.test(walletAddress))
      throw new TypeError('Notification wallet is invalid');
    const address = walletAddress.toLowerCase();
    const key = `${chainId}:${address}`;
    const existing = pending.get(key);
    if (existing) {
      await existing;
      return;
    }
    const run = this.readIncoming(address, chainId, key);
    pending.set(key, run);
    try {
      await run;
    } finally {
      if (pending.get(key) === run) pending.delete(key);
    }
  }

  private async readIncoming(address: string, chainId: number, key: string): Promise<void> {
    const previous = deliveryRepository.read().accounts[key];
    const options = { chainId, fresh: true };
    const [native, token] = await Promise.all([
      collectStream(
        (cursor) => getAddressTransactions(address, boundedHistoryParams(50, cursor), options),
        (tx) => nativeActivity(tx, address),
        previous?.native
      ),
      collectStream(
        (cursor) => getAddressTokenTransfers(address, boundedHistoryParams(50, cursor), options),
        (transfer) => tokenActivity(transfer, address),
        previous?.token
      ),
    ]);
    if (this.stopped) return;
    // Persist before delivery, and use the event identity as the OS notification tag.
    // An interrupted process cannot redeliver an already accepted event.
    deliveryRepository.update((current) => ({
      ...current,
      accounts: { ...current.accounts, [key]: { native: native.cursor, token: token.cursor } },
    }));
    for (const activity of [...native.incoming, ...token.incoming]) {
      if (this.stopped) return;
      await this.notify('Transaction Received', {
        body: 'You received a committed transfer.',
        tag: `${key}:${activity.id}`,
        data: { url: '/', chainId, walletAddress: address, eventId: activity.id },
      });
    }
  }

  async checkCampaign(now = Date.now()): Promise<void> {
    const url = new URL('/wallet/api/notifications/campaign', window.location.origin);
    const response = await fetch(url, { cache: 'no-store', signal: AbortSignal.timeout(10_000) });
    if (response.status === 204) return;
    if (!response.ok) throw new Error(`Campaign announcement unavailable (${response.status})`);
    const campaign = parseCampaignAnnouncement(await response.json());
    if (
      this.stopped ||
      now < campaign.startsAt ||
      now >= campaign.endsAt ||
      deliveryRepository.read().campaigns.includes(campaign.id)
    )
      return;
    deliveryRepository.update((current) => ({
      ...current,
      campaigns: [...current.campaigns, campaign.id],
    }));
    await this.notify(campaign.title, {
      body: `${new Date(campaign.startsAt).toISOString()} – ${new Date(campaign.endsAt).toISOString()}. ${campaign.eligibility.text} Eligibility: ${campaign.eligibility.sourceUrl}`,
      tag: `campaign:${campaign.id}`,
      data: { url: campaign.eligibility.sourceUrl, campaignId: campaign.id },
    });
  }
}

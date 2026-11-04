import path from 'node:path';
import { verifyMessage } from 'ethers';
import { AtomicJsonStore } from '@/server/atomic-json-store';
import { HttpBoundaryError } from '@/server/http';
import { parseStoredSubscription, type StoredSubscription } from '@/server/push-store';
import { pushUnsubscribeMessage } from './unsubscribe-intent';

interface SubscriptionEnvelope {
  version: 1;
  updatedAt: number;
  records: StoredSubscription[];
}

function parseEnvelope(input: unknown): SubscriptionEnvelope {
  if (
    typeof input !== 'object' ||
    input === null ||
    !('version' in input) ||
    input.version !== 1 ||
    !('updatedAt' in input) ||
    typeof input.updatedAt !== 'number' ||
    !Number.isSafeInteger(input.updatedAt) ||
    input.updatedAt < 0 ||
    !('records' in input) ||
    !Array.isArray(input.records)
  ) {
    throw new TypeError('Store envelope is invalid');
  }
  return {
    version: 1,
    updatedAt: input.updatedAt,
    records: input.records.map(parseStoredSubscription),
  };
}

const subscriptions = new AtomicJsonStore<SubscriptionEnvelope>({
  filePath: path.join(process.env.PUSH_DATA_DIR ?? '/data/paxport/push', 'subscriptions.json'),
  empty: () => ({ version: 1, updatedAt: Date.now(), records: [] }),
  parse: parseEnvelope,
  migrate: (input) => {
    if (!Array.isArray(input)) throw new TypeError('Legacy store is invalid');
    return { version: 1, updatedAt: Date.now(), records: input.map(parseStoredSubscription) };
  },
});

export async function removeOwnedSubscription(input: {
  endpoint: string;
  walletAddress: string;
  issuedAt: number;
  signature: string;
}): Promise<boolean> {
  const unproven = () =>
    new HttpBoundaryError(
      401,
      'SUBSCRIPTION_UNPROVEN',
      'Push subscription ownership could not be verified'
    );
  await subscriptions.update((store) => {
    const now = Math.floor(Date.now() / 1_000);
    const subscription = store.records.find((item) => item.endpoint === input.endpoint);
    if (
      !subscription ||
      input.walletAddress.toLowerCase() !== subscription.walletAddress ||
      !Number.isSafeInteger(input.issuedAt) ||
      input.issuedAt < now - 300 ||
      input.issuedAt > now + 60
    ) {
      throw unproven();
    }
    let signer: string;
    try {
      signer = verifyMessage(
        pushUnsubscribeMessage(subscription.endpoint, subscription.walletAddress, input.issuedAt),
        input.signature
      );
    } catch {
      throw unproven();
    }
    if (signer.toLowerCase() !== subscription.walletAddress) throw unproven();
    return {
      version: 1,
      updatedAt: Date.now(),
      records: store.records.filter((item) => item.endpoint !== subscription.endpoint),
    };
  });
  return true;
}

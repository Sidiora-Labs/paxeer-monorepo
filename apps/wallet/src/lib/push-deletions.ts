import { defineStorageRepository } from '@/platform/storage/registry';

export interface PushDeletionIntent {
  readonly endpoint: string;
  readonly walletAddress: string;
}

export const MAX_PUSH_DELETIONS = 16;

function parsePushDeletions(input: unknown): PushDeletionIntent[] {
  if (!Array.isArray(input) || input.length > MAX_PUSH_DELETIONS) {
    throw new TypeError('Invalid pending push deletions');
  }
  const endpoints = new Set<string>();
  return input.map((value: unknown) => {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
      throw new TypeError('Invalid pending push deletion');
    }
    const intent = value as Record<string, unknown>;
    if (
      Object.keys(intent).length !== 2 ||
      typeof intent.endpoint !== 'string' ||
      intent.endpoint.length < 12 ||
      intent.endpoint.length > 2048 ||
      typeof intent.walletAddress !== 'string' ||
      !/^0x[0-9a-fA-F]{40}$/.test(intent.walletAddress)
    ) {
      throw new TypeError('Invalid pending push deletion');
    }
    const endpoint = new URL(intent.endpoint);
    if (
      endpoint.protocol !== 'https:' ||
      endpoint.username ||
      endpoint.password ||
      endpoints.has(intent.endpoint)
    ) {
      throw new TypeError('Invalid pending push endpoint');
    }
    endpoints.add(intent.endpoint);
    return { endpoint: intent.endpoint, walletAddress: intent.walletAddress.toLowerCase() };
  });
}

export const pushDeletionRepository = defineStorageRepository<PushDeletionIntent[]>({
  id: 'push-deletions',
  key: 'paxport:push-deletions:v1',
  owner: 'notifications',
  schema: 'Pending endpoint and owning wallet; no subscription keys or signing credentials',
  version: 1,
  area: 'local',
  sensitivity: 'security-relevant',
  retention: 'Until browser removal and acknowledged server deletion; at most 16 endpoints',
  quotaBytes: 65536,
  migration: 'None',
  resetOn: ['uninstall'],
  corruption: 'fail-closed',
  prohibitedData: ['private keys', 'subscription keys', 'signatures'],
  fallback: () => [],
  parse: parsePushDeletions,
});

export function retainPushDeletion(intent: PushDeletionIntent): void {
  pushDeletionRepository.update((pending) => {
    const existing = pending.find((item) => item.endpoint === intent.endpoint);
    if (existing) {
      if (existing.walletAddress !== intent.walletAddress.toLowerCase()) {
        throw new Error('Push deletion belongs to another wallet');
      }
      return pending;
    }
    if (pending.length >= MAX_PUSH_DELETIONS) {
      throw new Error('Pending push deletion limit reached');
    }
    return [...pending, intent];
  });
}

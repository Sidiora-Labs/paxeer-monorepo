import { defineStorageRepository } from '@/platform/storage/registry';
import type { PendingSendRecord } from '@/platform/storage/repositories';
import { validateEvmAddress, validateTokenAmount } from '@/lib/txValidation';
import { readTokenPrecision } from '@/widgets/send/useSendableTokens';

export interface ScopedPendingSend extends PendingSendRecord {
  sender: string;
  chainId: number;
}

export interface PendingSendRecovery {
  event: ScopedPendingSend;
  reconcileAt: number | null;
}

export function pendingSendKey(event: ScopedPendingSend): string {
  return `${event.chainId}:${event.sender.toLowerCase()}:${event.txHash.toLowerCase()}`;
}

function parsePendingSends(input: unknown): Record<string, PendingSendRecovery> {
  if (!input || typeof input !== 'object' || Array.isArray(input))
    throw new TypeError('Pending sends must be a record.');
  const entries = Object.entries(input);
  if (entries.length > 128) throw new TypeError('Too many pending sends.');
  return Object.fromEntries(
    entries.map(([key, raw]) => {
      if (!raw || typeof raw !== 'object' || Array.isArray(raw))
        throw new TypeError('Pending send recovery is invalid.');
      const { event, reconcileAt } = raw as PendingSendRecovery;
      if (
        !event ||
        typeof event !== 'object' ||
        Array.isArray(event) ||
        Object.keys(raw).some((field) => !['event', 'reconcileAt'].includes(field)) ||
        Object.keys(event).some(
          (field) =>
            ![
              'sender',
              'chainId',
              'tokenAddress',
              'symbol',
              'amount',
              'decimals',
              'recipient',
              'txHash',
              'timestamp',
            ].includes(field)
        ) ||
        typeof event.sender !== 'string' ||
        typeof event.recipient !== 'string' ||
        (event.tokenAddress !== undefined && typeof event.tokenAddress !== 'string') ||
        !Number.isSafeInteger(event.chainId) ||
        event.chainId <= 0 ||
        typeof event.txHash !== 'string' ||
        !/^0x[0-9a-fA-F]{64}$/.test(event.txHash) ||
        typeof event.symbol !== 'string' ||
        !event.symbol ||
        event.symbol.length > 20 ||
        typeof event.amount !== 'string' ||
        event.amount.length > 100 ||
        readTokenPrecision(event.decimals) === null ||
        !Number.isSafeInteger(event.timestamp) ||
        event.timestamp < 0 ||
        (reconcileAt !== null && (!Number.isSafeInteger(reconcileAt) || reconcileAt < 0))
      )
        throw new TypeError('Pending send identity or precision is invalid.');
      validateEvmAddress(event.sender);
      validateEvmAddress(event.recipient);
      if (event.tokenAddress !== undefined) validateEvmAddress(event.tokenAddress);
      validateTokenAmount(event.amount, event.decimals);
      if (key !== pendingSendKey(event)) throw new TypeError('Pending send key is invalid.');
      return [key, { event: { ...event }, reconcileAt }];
    })
  );
}

export const scopedPendingSendsRepository = defineStorageRepository<
  Record<string, PendingSendRecovery>
>({
  id: 'portfolio-pending-sends',
  key: 'paxport:v1:portfolio-pending-sends',
  owner: 'operations',
  schema: 'Record<chainId:sender:hash, PendingSendRecovery>',
  version: 1,
  area: 'session',
  sensitivity: 'private-metadata',
  retention: 'Browser session, until the original account and chain reconcile',
  quotaBytes: 131072,
  migration: 'New scoped identity; unscoped legacy events cannot establish a sender or chain',
  resetOn: ['reset', 'logout', 'custody-switch', 'account-removal', 'uninstall'],
  corruption: 'fail-closed',
  prohibitedData: ['private keys', 'seed phrases', 'full transaction approval'],
  fallback: () => ({}),
  parse: parsePendingSends,
});

export function storeScopedPendingSend(event: ScopedPendingSend): void {
  scopedPendingSendsRepository.update((records) => {
    const key = pendingSendKey(event);
    return records[key] ? records : { ...records, [key]: { event, reconcileAt: null } };
  });
}

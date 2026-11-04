import type { TransferIdentity, TransferObservation } from '@paxeer/wallet';
import { PAXEER_CONFIG } from '@/lib/constants';
import {
  preferencesRepository,
  putTransferRecord,
  transferObservationsRepository,
  type JsonValue,
} from '@/platform/storage/repositories';

export interface TransferOrigin {
  readonly chainId: number;
  readonly rpcUrl: string;
  readonly explorerUrl: string;
  readonly statusUrl: string;
}

export function transferStorageKey(identity: TransferIdentity): string {
  return `${identity.chainId}:${identity.hash.toLowerCase()}`;
}

function endpoint(value: unknown, relative = false): string {
  if (typeof value !== 'string' || !value || value.length > 2048)
    throw new Error('Saved transfer network endpoint is invalid.');
  if (relative && value.startsWith('/') && !value.startsWith('//')) return value;
  const url = new URL(value);
  if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password)
    throw new Error('Saved transfer network endpoint is invalid.');
  return value;
}

export function resolveTransferOrigin(identity: TransferIdentity): TransferOrigin {
  const raw = transferObservationsRepository.read()[transferStorageKey(identity)];
  if (raw && typeof raw === 'object' && !Array.isArray(raw) && raw.origin !== undefined) {
    const origin = raw.origin;
    if (
      !origin ||
      typeof origin !== 'object' ||
      Array.isArray(origin) ||
      origin.chainId !== identity.chainId
    )
      throw new Error('Saved transfer network does not match the submitted chain.');
    return Object.freeze({
      chainId: identity.chainId,
      rpcUrl: endpoint(origin.rpcUrl),
      explorerUrl: endpoint(origin.explorerUrl),
      statusUrl: endpoint(origin.statusUrl, true),
    });
  }
  const preferences = preferencesRepository.read();
  if (identity.chainId === PAXEER_CONFIG.chainId) {
    return Object.freeze({
      chainId: identity.chainId,
      rpcUrl: endpoint(preferences.customRpc ?? PAXEER_CONFIG.rpcUrl),
      explorerUrl: endpoint(PAXEER_CONFIG.explorerBase),
      statusUrl: endpoint(
        process.env.NEXT_PUBLIC_PAXEER_EXPLORER_URL ||
          process.env.NEXT_PUBLIC_EXPLORER_STATUS_URL ||
          PAXEER_CONFIG.blockscoutApiBase,
        true
      ),
    });
  }
  const network = preferences.networks.find((saved) => saved.chainId === identity.chainId);
  if (!network)
    throw new Error(
      `Network configuration for submitted chain ${identity.chainId} is unavailable.`
    );
  return Object.freeze({
    chainId: identity.chainId,
    rpcUrl: endpoint(network.rpcUrl),
    explorerUrl: endpoint(network.explorerUrl),
    statusUrl: endpoint(network.explorerUrl),
  });
}

export function saveTransferObservation(
  observation: TransferObservation,
  origin: TransferOrigin
): void {
  const stored = JSON.parse(JSON.stringify({ ...observation, origin })) as JsonValue;
  transferObservationsRepository.update((records) =>
    putTransferRecord(records, transferStorageKey(observation.identity), stored)
  );
}

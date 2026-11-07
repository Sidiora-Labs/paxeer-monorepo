import BigNumber from 'bignumber.js';

import type { PaxeerXAsset, PaxeerXIdentities, PaxeerXIdentityKind } from 'types/api/paxeerX';

export const IDENTITY_ORDER: Array<PaxeerXIdentityKind> = [ 'evm', 'pax', 'did' ];

export const IDENTITY_LABELS: Record<PaxeerXIdentityKind, string> = {
  evm: 'EVM address',
  pax: 'Paxeer address',
  did: 'LayerX DID',
};

export interface IdentityEntry {
  kind: PaxeerXIdentityKind;
  label: string;
  value: string;
}

// Identities that the account does not have are omitted entirely rather than rendered as empty rows.
export function listIdentities(identities: PaxeerXIdentities): Array<IdentityEntry> {
  return IDENTITY_ORDER
    .map((kind) => ({ kind, label: IDENTITY_LABELS[kind], value: identities[kind] }))
    .filter((entry): entry is IdentityEntry => typeof entry.value === 'string' && entry.value.length > 0);
}

export function assetLabel(asset: PaxeerXAsset | null): string {
  if (!asset) {
    return 'Unknown asset';
  }

  return asset.symbol || asset.denom || asset.id;
}

// Raw integer amounts are scaled by the asset's own decimals; an unknown scale is shown verbatim.
export function formatAmount(value: string, asset: PaxeerXAsset | null): string {
  const amount = new BigNumber(value);

  if (!amount.isFinite()) {
    return value;
  }

  if (!asset || asset.decimals === null) {
    return amount.toFormat();
  }

  return amount.dividedBy(new BigNumber(10).pow(asset.decimals)).toFormat();
}

const KIND_LABELS: Record<string, string> = {
  transaction: 'Transaction',
  token_transfer: 'Token transfer',
};

export function activityKindLabel(kind: string): string {
  const known = KIND_LABELS[kind];

  if (known) {
    return known;
  }

  const humanized = kind.replaceAll(/[-_]+/g, ' ').trim();

  if (!humanized) {
    return kind;
  }

  return humanized.charAt(0).toUpperCase() + humanized.slice(1);
}

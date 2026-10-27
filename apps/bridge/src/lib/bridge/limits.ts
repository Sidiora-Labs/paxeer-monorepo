import type { BridgeChain } from './chains';
import type { PaxeerPairState, VaultState } from './use-bridge-state';

export type AmountLimitArgs = {
  direction: 'in' | 'out';
  amount: bigint;
  perTx: bigint;
  total: bigint;
  outstanding: bigint;
  balance: bigint | undefined;
};

export function amountLimitError({ direction, amount, perTx, total, outstanding, balance }: AmountLimitArgs): string | undefined {
  if (balance !== undefined && amount > balance) {
    return 'That is more than your available balance.';
  }
  if (perTx > BigInt(0) && amount > perTx) {
    return 'Above the vault per-transaction cap.';
  }
  if (direction === 'in' && total > BigInt(0) && outstanding + amount > total) {
    return 'That would exceed the vault\'s total capacity for this asset.';
  }
  if (direction === 'out' && amount > outstanding) {
    return 'That is more than the vault\'s outstanding balance for this asset.';
  }
  return undefined;
}

export function maxSpendable(balance: bigint, isNative: boolean, gasReserve: bigint): bigint {
  if (!isNative) {
    return balance;
  }
  return gasReserve > balance ? BigInt(0) : balance - gasReserve;
}

export type RouteBlockerArgs = {
  paxeerState: PaxeerPairState;
  vaultState: VaultState;
  attestationApiConfigured: boolean;
  foreign?: Pick<BridgeChain, 'state' | 'label' | 'vault'>;
  direction?: 'in' | 'out';
  paxeerConfigured?: boolean;
};

export function routeBlockers({
  paxeerState,
  vaultState,
  attestationApiConfigured,
  foreign,
  direction,
  paxeerConfigured,
}: RouteBlockerArgs): Array<string> {
  const list: Array<string> = [];
  if (foreign?.state === 'not-deployed') {
    list.push(`The ${ foreign.label } vault is not deployed yet. This route opens once the deployment record is published.`);
  }
  if (foreign?.state === 'awaiting-backend') {
    list.push(
      direction === 'in' ?
        'Deposits from Solana need a Solana wallet and the deployed custody program; its program id is still a placeholder in the bridge configuration.' :
        'Withdrawals to Solana also need the bridge backend to relay your full Solana address to the custody program. That backend is not connected yet.',
    );
  }
  if (paxeerConfigured === false) {
    list.push('The Paxeer RPC is not configured for this deployment, so the bridge cannot be verified on-chain here.');
  } else {
    if (paxeerState.error) {
      list.push('The Paxeer bridge state could not be read. The route cannot be verified on-chain right now.');
    }
    if (paxeerState.loaded) {
      if (paxeerState.paused) {
        list.push('The bridge is paused on Paxeer. No deposits or withdrawals will be accepted until it reopens.');
      }
      if (!paxeerState.registered) {
        list.push('This chain is not registered with the Paxeer bridge yet. Governance registers a chain after its vault is deployed.');
      } else if (!paxeerState.enabled) {
        list.push('This chain is registered but currently disabled on Paxeer.');
      }
      if (paxeerState.registered && foreign?.vault && paxeerState.vault &&
        paxeerState.vault.toLowerCase() !== foreign.vault.toLowerCase()) {
        list.push('The vault registered on Paxeer for this chain does not match the vault this app is configured with. Do not deposit.');
      }
    }
  }
  if (vaultState.loaded && vaultState.paused) {
    list.push('The vault on the foreign chain is paused.');
  }
  if (!attestationApiConfigured) {
    list.push('The bridge attestation API is not configured for this deployment.');
  }
  return list;
}

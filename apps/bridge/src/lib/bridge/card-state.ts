import { formatAmount, hasExcessPrecision, parseAmount } from './amount';
import type { BridgeAsset } from './chains';
import { amountLimitError, maxSpendable, routeBlockers } from './limits';
import type { RouteBlockerArgs } from './limits';

export type CardStateInputs = RouteBlockerArgs & {
  direction: 'in' | 'out';
  asset: Pick<BridgeAsset, 'symbol' | 'decimals' | 'isNative' | 'perTxCap'>;
  amount: string;
  balance: bigint | undefined;
  gasReserve: bigint;
  allowanceError: boolean;
  isConnected: boolean;
  onSourceChain: boolean;
  destinationValid: boolean;
  depositPlanAvailable: boolean;
  busy: boolean;
};

function cardInputError(inputs: CardStateInputs): string | undefined {
  const { amount, asset, balance, direction, foreign, paxeerConfigured, paxeerState, vaultState } = inputs;
  if (amount.trim() === '') {
    return undefined;
  }
  if (hasExcessPrecision(amount, asset.decimals)) {
    return `${ asset.symbol } supports at most ${ asset.decimals } decimal places.`;
  }
  const value = parseAmount(amount, asset.decimals);
  if (value === undefined) {
    return 'Enter a valid amount.';
  }
  if (value === 0n) {
    return 'Enter an amount greater than zero.';
  }
  const limitError = amountLimitError({
    direction,
    amount: value,
    balance,
    perTx: foreign?.vault ? (vaultState.loaded ? vaultState.perTx : BigInt(asset.perTxCap)) : 0n,
    total: foreign?.vault && vaultState.loaded ? vaultState.total : 0n,
    outstanding: vaultState.outstanding,
  });
  if (limitError) {
    return limitError;
  }
  if (paxeerConfigured && paxeerState.loaded && paxeerState.registered) {
    if (paxeerState.maxPerTx > 0n && value > paxeerState.maxPerTx) {
      return `Above the Paxeer per-transaction cap of ${ formatAmount(paxeerState.maxPerTx, asset.decimals) } ${ asset.symbol }.`;
    }
    if (direction === 'in' && paxeerState.maxInFlight > 0n && paxeerState.inFlight + value > paxeerState.maxInFlight) {
      return 'That would exceed the bridge\'s in-flight capacity for this asset right now.';
    }
  }
  return undefined;
}

export function cardState(inputs: CardStateInputs) {
  const { amount, asset, balance, direction, gasReserve } = inputs;
  const inputError = cardInputError(inputs);
  const blockers = routeBlockers(inputs);
  if (direction === 'in' && !asset.isNative && inputs.allowanceError) {
    blockers.push('Could not read the token allowance. Wait for the allowance to refresh before depositing.');
  }
  const maxAmount = balance === undefined ?
    undefined :
    formatAmount(maxSpendable(balance, direction === 'in' && asset.isNative, gasReserve), asset.decimals);
  const value = parseAmount(amount, asset.decimals);
  const ready =
    inputs.isConnected &&
    inputs.onSourceChain &&
    blockers.length === 0 &&
    value !== undefined &&
    value > 0n &&
    inputError === undefined &&
    inputs.destinationValid &&
    (direction !== 'in' || inputs.depositPlanAvailable) &&
    !inputs.busy;
  return { inputError, blockers, maxAmount, ready };
}

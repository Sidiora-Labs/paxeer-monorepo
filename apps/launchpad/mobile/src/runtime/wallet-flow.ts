import type { TradeIdentity } from './trade-recovery.ts';
import { walletRejected } from './flow-state.ts';

export function replacementOutcome(
  identity: TradeIdentity,
  transaction: {
    from: string;
    to: string | null;
    chainId: number;
    nonce: number;
    input: string;
    value: bigint;
  },
): 'same_action' | 'cancelled' | 'replaced_action' {
  const saved = identity;
  const same = (a: string | null, b: string) =>
    a?.toLowerCase() === b.toLowerCase();
  if (
    !same(transaction.from, saved.request.from) ||
    transaction.chainId !== Number(saved.request.chainId) ||
    transaction.nonce !== saved.nonce
  )
    throw new Error(
      'Replacement must use the same account, chain and nonce',
    );
  if (
    same(transaction.to, saved.request.to) &&
    same(transaction.input, saved.request.data) &&
    transaction.value === BigInt(saved.request.valueRaw)
  )
    return 'same_action';
  return same(transaction.to, saved.request.from) &&
    transaction.input === '0x' &&
    transaction.value === 0n
    ? 'cancelled'
    : 'replaced_action';
}

export function walletSubmissionState(
  error: unknown,
  handedOff: boolean,
): 'ready' | 'unknown' {
  return !handedOff || walletRejected(error) ? 'ready' : 'unknown';
}

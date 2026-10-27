import { base58Decode } from './solana';
import type { recipientRecordFromResponse } from './solana';

type RecipientLookup = ReturnType<typeof recipientRecordFromResponse>;

export function solanaWithdrawalBlocker(lookup: RecipientLookup | undefined, destination: string): string | undefined {
  if (lookup === undefined) {
    return 'The Solana recipient record has not been verified. Wait for the lookup to succeed before withdrawing.';
  }
  if (!lookup.registered) {
    return 'Register this destination\'s recipient record on Solana before withdrawing.';
  }
  const destinationKey = base58Decode(destination);
  if (!destinationKey || destinationKey.length !== 32 || lookup.key.length !== 32 ||
    !destinationKey.every((byte, index) => byte === lookup.key[index])) {
    return 'The registered Solana recipient key does not match the destination address.';
  }
  return undefined;
}

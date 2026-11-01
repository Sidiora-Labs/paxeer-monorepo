import { verifyMessage } from 'ethers';
import { pushOwnershipMessage } from '@/lib/push';
import { HttpBoundaryError } from './http';

export { pushOwnershipMessage } from '@/lib/push';

export function verifyPushOwnership(
  input: { endpoint: string; walletAddress: string; issuedAt: number; signature: string },
  now: number
): void {
  const unproven = () =>
    new HttpBoundaryError(
      401,
      'SUBSCRIPTION_UNPROVEN',
      'Push subscription ownership could not be verified'
    );
  if (
    !Number.isSafeInteger(input.issuedAt) ||
    input.issuedAt < now - 300 ||
    input.issuedAt > now + 60 ||
    typeof input.signature !== 'string'
  ) {
    throw unproven();
  }
  let signer: string;
  try {
    signer = verifyMessage(
      pushOwnershipMessage(input.endpoint, input.walletAddress, input.issuedAt),
      input.signature
    );
  } catch {
    throw unproven();
  }
  if (signer.toLowerCase() !== input.walletAddress.toLowerCase()) {
    throw unproven();
  }
}

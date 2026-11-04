import { sha256, stringToHex } from 'viem';
import { Buffer } from 'buffer';
import type { CurrentIdentity } from './service-client.ts';

export interface OperationWallet {
  readonly supportedChain: number;
  readonly connectedAccount: string | undefined;
  signMessage(message: string, signal: AbortSignal): Promise<string>;
}
export type AccountAction =
  | 'watchlist.update'
  | 'watchlist.remove'
  | 'profile.update'
  | 'preferences.update';
export type AccountMethod = 'PUT' | 'DELETE' | 'PATCH';
export interface SignedAccountOperation {
  operation: string;
  signature: string;
  idempotencyKey: string;
  requestId: string;
  expiresAt: number;
}
export function sameOperationIdentity(a: CurrentIdentity, b: CurrentIdentity) {
  return (
    a.accountId === b.accountId &&
    a.sessionId === b.sessionId &&
    a.authEpoch === b.authEpoch &&
    a.wallet.toLowerCase() === b.wallet.toLowerCase()
  );
}
export async function signAccountOperation(
  wallet: OperationWallet,
  identity: CurrentIdentity,
  input: {
    action: AccountAction;
    method: AccountMethod;
    path: string;
    body: string;
  },
  requestId: () => Promise<string>,
  signal: AbortSignal,
  assertCurrent: () => void,
): Promise<SignedAccountOperation> {
  const check = () => {
    signal.throwIfAborted();
    assertCurrent();
    if (
      wallet.supportedChain !== 125 ||
      wallet.connectedAccount?.toLowerCase() !==
        identity.wallet.toLowerCase() ||
      Date.parse(identity.accessExpiresAt) <= Date.now()
    )
      throw new Error('Current chain125 session wallet required');
  };
  check();
  const id = await requestId(),
    second = await requestId();
  const uuid =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  if (!uuid.test(id) || !uuid.test(second))
    throw new Error('Invalid native operation identity');
  const issuedAt = Math.floor(Date.now() / 1000),
    expiresAt = issuedAt + 300;
  const envelope = {
    version: 2,
    chainId: '125',
    environment: 'production',
    actor: identity.wallet.toLowerCase(),
    action: input.action,
    method: input.method,
    path: input.path,
    bodyHash: sha256(stringToHex(input.body)).slice(2),
    nonce: (id + second).replace(/-/g, '').toLowerCase(),
    issuedAt,
    expiresAt,
  };
  // Go's OperationEnvelope.Statement marshals these fields in declaration order.
  const encoded = JSON.stringify(envelope).replace(
    /[<>&\u2028\u2029]/g,
    (character) =>
      `\\u${character.charCodeAt(0).toString(16).padStart(4, '0')}`,
  );
  check();
  const signature = await wallet.signMessage(
    `KindleLaunch signed operation v2\n${encoded}`,
    signal,
  );
  check();
  if (
    !/^0x(?:[0-9a-f]{2}){1,4096}$/i.test(signature) ||
    Date.now() >= expiresAt * 1000
  )
    throw new Error('Invalid or expired wallet operation signature');
  return {
    operation: Buffer.from(encoded, 'utf8')
      .toString('base64')
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, ''),
    signature,
    idempotencyKey: id,
    requestId: id,
    expiresAt,
  };
}

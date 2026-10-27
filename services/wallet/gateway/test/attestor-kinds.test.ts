import { expect, test } from 'vitest';
import * as attestor from '../src/attestor/client.js';
import type { Curve, SignKind, SignPayload } from '../src/attestor/client.js';

function signKindsExcludeRetiredKinds(): void {
  const expected: Record<SignKind, Curve> = {
    evm_tx: 'secp256k1',
    eip712: 'secp256k1',
    personal_message: 'secp256k1',
    eth_sign_digest: 'secp256k1',
    lx_send_authorization: 'ed25519',
    lx_bind: 'ed25519',
    lx_grant: 'ed25519',
  };
  expect(Object.keys(attestor.KIND_CURVES).sort()).toEqual(Object.keys(expected).sort());
  expect(attestor.KIND_CURVES).toEqual(expected);
  for (const name of ['CUSTODY_DOMAIN', 'CUSTODY_TARGET', 'CUSTODY_ABI', 'decodeCustody']) {
    expect(attestor).not.toHaveProperty(name);
  }

  const sessionId = 'approved-session';
  const keyId = 'wallet-key';
  const signers = ['node-1', 'node-2', 'node-3'];
  const transaction = attestor.unsignedTransactionBytes({
    type: 'eip1559',
    chainId: 125,
    nonce: 0,
    to: '0x0000000000000000000000000000000000000001',
    value: 1n,
    gas: 21_000n,
    maxFeePerGas: 2n,
    maxPriorityFeePerGas: 1n,
  });
  expect(attestor.signRequestBody(sessionId, keyId, signers, { kind: 'evm_tx', transaction })).toEqual({
    session_id: sessionId,
    key_id: keyId,
    kind: 'evm_tx',
    signers,
    transaction: transaction.slice(2),
  });

  const payload: Extract<SignPayload, { kind: 'lx_send_authorization' }> = {
    kind: 'lx_send_authorization',
    activity: '0xABCD',
    disclosure: {
      account: 'account-1',
      module: 'pay',
      operation: 1,
      amounts: [{ asset: 'native', amount: '1' }],
      destinations: ['account-2'],
      sequence: '0',
      not_before: '0',
      not_after: '60',
    },
    approval: {
      version: 1,
      principal: 'account-1',
      key_id: keyId,
      network_id: 125,
      protocol_version: 1,
      session_id: sessionId,
      activity_digest: 'ab'.repeat(32),
      expires_at: '60',
    },
  };
  expect(attestor.signRequestBody(sessionId, keyId, signers, payload)).toEqual({
    session_id: sessionId,
    key_id: keyId,
    kind: 'lx_send_authorization',
    signers,
    activity: 'abcd',
    disclosure: payload.disclosure,
    approval: payload.approval,
  });
  expect(() => attestor.signRequestBody('another-session', keyId, signers, payload)).toThrow(
    'the approval binding names another session or key',
  );
}

test(signKindsExcludeRetiredKinds);

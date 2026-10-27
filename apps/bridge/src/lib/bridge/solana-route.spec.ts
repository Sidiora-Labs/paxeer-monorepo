import { expect, test } from 'vitest';

import { base58Encode } from './solana';
import { solanaWithdrawalBlocker } from './solana-route';

function solanaWithdrawalBlockerRequiresARegisteredRecord() {
  const destination = base58Encode(new Uint8Array(32).fill(7));
  expect(solanaWithdrawalBlocker({ registered: false }, destination))
    .toBe('Register this destination\'s recipient record on Solana before withdrawing.');
  expect(solanaWithdrawalBlocker(undefined, destination))
    .toBe('The Solana recipient record has not been verified. Wait for the lookup to succeed before withdrawing.');
}

function solanaWithdrawalBlockerRefusesARecordForAnotherKey() {
  const key = Uint8Array.from({ length: 32 }, (_, index) => index);
  const destination = base58Encode(key);
  const mismatch = 'The registered Solana recipient key does not match the destination address.';
  for (let index = 0; index < key.length; index += 1) {
    const otherKey = key.slice();
    otherKey[index] ^= 1;
    expect(solanaWithdrawalBlocker({ registered: true, key: otherKey }, destination)).toBe(mismatch);
  }
  expect(solanaWithdrawalBlocker({ registered: true, key: key.slice() }, destination)).toBeUndefined();
  expect(solanaWithdrawalBlocker({ registered: true, key }, `  ${ destination }  `)).toBeUndefined();
  expect(solanaWithdrawalBlocker({ registered: true, key: new Uint8Array(32) }, '11111111111111111111111111111111'))
    .toBeUndefined();
  for (const invalid of [ '', 'invalid', base58Encode(key.slice(0, 31)), base58Encode(new Uint8Array(33)) ]) {
    expect(solanaWithdrawalBlocker({ registered: true, key }, invalid)).toBe(mismatch);
  }
  for (const invalidKey of [ key.slice(0, 31), new Uint8Array(33) ]) {
    expect(solanaWithdrawalBlocker({ registered: true, key: invalidKey }, destination)).toBe(mismatch);
  }
}

test(solanaWithdrawalBlockerRequiresARegisteredRecord.name, solanaWithdrawalBlockerRequiresARegisteredRecord);
test(solanaWithdrawalBlockerRefusesARecordForAnotherKey.name, solanaWithdrawalBlockerRefusesARecordForAnotherKey);

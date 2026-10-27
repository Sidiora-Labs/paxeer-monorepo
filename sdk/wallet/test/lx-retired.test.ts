import { expect, test } from 'vitest';
import { PaxeerWallet } from '../src/index.js';

function paxeer_wallet_has_no_lx_activity_methods(): void {
  for (const method of [
    'reviewLxActivity',
    'reviewLxSendAuthorization',
    'lxReview',
    'approveLxActivity',
    'signApprovedLxActivity',
    'lxApprovalStatus',
  ]) {
    expect(method in PaxeerWallet.prototype, method).toBe(false);
  }

  for (const method of [
    'reviewEvmTransaction',
    'approveEvmTransaction',
    'evmApprovalStatus',
  ] as const) {
    expect(PaxeerWallet.prototype[method], method).toBeTypeOf('function');
  }
}

test(paxeer_wallet_has_no_lx_activity_methods);

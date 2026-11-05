import { readdirSync, readFileSync } from 'node:fs';
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

function wallet_tests_invoke_no_retired_lx_method(): void {
  const directory = new URL('./', import.meta.url);
  const files = readdirSync(directory).filter(
    (file) => file.endsWith('.test.ts') && file !== 'lx-retired.test.ts'
  );

  expect(files).not.toContain('point.test.ts');
  expect(files.length).toBeGreaterThan(0);

  for (const file of files) {
    const source = readFileSync(new URL(file, directory), 'utf8');
    for (const name of [
      'reviewLxActivity',
      'reviewLxSendAuthorization',
      'lxReview',
      'approveLxActivity',
      'signApprovedLxActivity',
      'lxApprovalStatus',
      'lxActivity',
    ]) {
      expect(source, `${file} must not reference ${name}`).not.toMatch(new RegExp(`\\b${name}\\b`));
    }
  }
}

test(wallet_tests_invoke_no_retired_lx_method);

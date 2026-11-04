import { describe, expect, it } from 'vitest';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { PreparedTransactionError, deploymentRole, gasCostUpperBound, validatePreparedTransaction, type TransactionRequest } from './prepared-transaction';

const wallet = '0x1111111111111111111111111111111111111111';
const intent = { action: 'funding' as const, bounty: { bountyId: 'b-1', creatorWallet: wallet, assetAddress: CORE_DEPLOYMENT.usdl, budgetRaw: '25000000' } };
const deployment = { Router: CORE_DEPLOYMENT.router, USDL: CORE_DEPLOYMENT.usdl, FeesRouter: CORE_DEPLOYMENT.feesRouter };

function request(overrides: Partial<TransactionRequest> = {}): TransactionRequest {
  return { chainId: '125', from: wallet, to: CORE_DEPLOYMENT.usdl, data: '0x095ea7b3' + '0'.repeat(128), valueRaw: '0', gasLimitRaw: '65000', maxFeePerGasRaw: '1000000000', maxPriorityFeePerGasRaw: null, ...overrides };
}

function reason(run: () => unknown): string | null {
  try { run(); return null; } catch (error) { return error instanceof PreparedTransactionError ? error.reason : 'other'; }
}

describe('validatePreparedTransaction', () => {
  it('refuses even an attested token approval without a deployed bounty escrow operation', () => {
    expect(reason(() => validatePreparedTransaction(request({ from: wallet.toUpperCase().replace('0X', '0x') }), { wallet, deployment, intent }))).toBe('operation');
  });
  it('rejects another chain', () => {
    expect(reason(() => validatePreparedTransaction({ ...request(), chainId: '1' as '125' }, { wallet, deployment, intent }))).toBe('chain');
  });
  it('rejects a signer other than the connected wallet', () => {
    expect(reason(() => validatePreparedTransaction(request({ from: '0x2222222222222222222222222222222222222222' }), { wallet, deployment, intent }))).toBe('from');
    expect(reason(() => validatePreparedTransaction(request(), { wallet: 'not-a-wallet', deployment, intent }))).toBe('from');
  });
  it('rejects contracts outside the runtime deployment authority', () => {
    expect(reason(() => validatePreparedTransaction(request({ to: '0x3333333333333333333333333333333333333333' }), { wallet, deployment, intent }))).toBe('to');
    expect(reason(() => validatePreparedTransaction(request({ to: '0x0000000000000000000000000000000000000000' }), { wallet, intent, deployment: { Zero: '0x0000000000000000000000000000000000000000' } }))).toBe('to');
  });
  it('rejects calldata that is not whole-byte hex with a selector', () => {
    for (const data of ['0x', '0x1234', '0x095ea7b', '095ea7b3', '0x095ea7bz']) {
      expect(reason(() => validatePreparedTransaction(request({ data }), { wallet, deployment, intent }))).toBe('calldata');
    }
  });
  it('rejects malformed amounts and any native principal', () => {
    expect(reason(() => validatePreparedTransaction(request({ gasLimitRaw: '-1' }), { wallet, deployment, intent }))).toBe('amount');
    expect(reason(() => validatePreparedTransaction(request({ maxFeePerGasRaw: (1n << 256n).toString() }), { wallet, deployment, intent }))).toBe('amount');
    expect(reason(() => validatePreparedTransaction(request({ valueRaw: '1' }), { wallet, deployment, intent }))).toBe('value');
  });
});

describe('deploymentRole and gas bound', () => {
  it('maps addresses to role names case-insensitively', () => {
    expect(deploymentRole(CORE_DEPLOYMENT.router.toLowerCase(), deployment)).toBe('Router');
    expect(deploymentRole('0x4444444444444444444444444444444444444444', deployment)).toBeNull();
    expect(deploymentRole('nope', deployment)).toBeNull();
  });
  it('multiplies gas by fee and refuses a non-positive gas', () => {
    expect(gasCostUpperBound(21000n, 2n)).toBe(42000n);
    expect(() => gasCostUpperBound(0n, 2n)).toThrow(PreparedTransactionError);
  });
});

import { describe, expect, it, test } from 'vitest';
import { decodeFunctionData, encodeFunctionData } from 'viem';
import { erc20Abi } from 'sidiora-launchpad-v3';
import { BountyRpcTranscript, transcriptProvider, preparedTokenRequest, bounty, selection, attacker } from './repair-22-33-985.fixtures';
import { prepareBountyReview, sendReviewedBountyTransaction } from './bounty-transaction-attempt';
import { getPublicClient } from '@/core/wallet-sdk/client';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { createFundingIntent, createSettlementIntent, PreparedTransactionError, deploymentRole, gasCostUpperBound, validatePreparedTransaction, type TransactionRequest } from './prepared-transaction';

const wallet = '0x1111111111111111111111111111111111111111';
const intent = { action: 'funding' as const, bounty: { bountyId: 'b-1', creatorWallet: wallet, assetAddress: CORE_DEPLOYMENT.usdl, budgetRaw: '25000000' } };
const deployment = { Router: CORE_DEPLOYMENT.router, USDL: CORE_DEPLOYMENT.usdl, FeesRouter: CORE_DEPLOYMENT.feesRouter };

function request(overrides: Partial<TransactionRequest> = {}): TransactionRequest {
  return { chainId: '125', from: wallet, to: CORE_DEPLOYMENT.usdl, data: encodeFunctionData({ abi: erc20Abi, functionName: 'approve', args: [CORE_DEPLOYMENT.router, 25_000_000n] }), valueRaw: '0', gasLimitRaw: '65000', maxFeePerGasRaw: '1000000000', maxPriorityFeePerGasRaw: null, ...overrides };
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

export async function prepared_call_tests_no_intent_refuse_deployed_calls_before_wallet() {
  const transcript = new BountyRpcTranscript();
  const client = getPublicClient(await transcript.open());
  const provider = transcriptProvider(client);
  try {
    const funding = createFundingIntent(bounty);
    const settlement = createSettlementIntent(bounty, selection);
    const exact = preparedTokenRequest();
    expect(decodeFunctionData({ abi: erc20Abi, data: exact.data as `0x${string}` })).toEqual({ functionName: 'transfer', args: [selection.winnerWallet, 5_000_000n] });
    const cases = [
      { intent: settlement, request: preparedTokenRequest({ data: encodeFunctionData({ abi: erc20Abi, functionName: 'transfer', args: [attacker, 5_000_000n] }) }), reason: 'intent' },
      { intent: settlement, request: preparedTokenRequest({ data: encodeFunctionData({ abi: erc20Abi, functionName: 'transfer', args: [selection.winnerWallet as `0x${string}`, 5_000_001n] }) }), reason: 'amount' },
      { intent: settlement, request: preparedTokenRequest({ to: CORE_DEPLOYMENT.router }), reason: 'operation' },
      { intent: createSettlementIntent({ ...bounty, assetAddress: CORE_DEPLOYMENT.feesRouter }, selection), request: exact, reason: 'operation' },
      { intent: funding, request: preparedTokenRequest({ data: encodeFunctionData({ abi: erc20Abi, functionName: 'approve', args: [attacker, 25_000_000n] }) }), reason: 'operation' },
      { intent: settlement, request: preparedTokenRequest({ data: encodeFunctionData({ abi: erc20Abi, functionName: 'balanceOf', args: [wallet] }) }), reason: 'operation' },
      { intent: settlement, request: exact, reason: 'operation' },
    ];
    for (const entry of cases) {
      const prepareThenSign = async () => {
        const review = await prepareBountyReview(entry.request, { wallet, deployment, intent: entry.intent }, client);
        return sendReviewedBountyTransaction(review, review.tx, provider);
      };
      await expect(prepareThenSign()).rejects.toMatchObject({ reason: entry.reason });
      expect(transcript.calls.filter(call => call.method === 'eth_sendTransaction')).toHaveLength(0);
    }
    expect(transcript.calls).toHaveLength(0);
  } finally { await transcript.close(); }
}

test(prepared_call_tests_no_intent_refuse_deployed_calls_before_wallet.name, prepared_call_tests_no_intent_refuse_deployed_calls_before_wallet);

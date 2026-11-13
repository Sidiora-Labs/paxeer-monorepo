import { expect, test } from 'vitest';
import { encodeFunctionData, type Address, type Hex } from 'viem';
import { erc20Abi } from 'sidiora-launchpad-v3';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import type { Bounty } from './bounties-contract';
import {
  createFundingIntent,
  createSettlementIntent,
  PreparedTransactionError,
  validatePreparedTransaction,
  type BountyTransactionIntent,
  type TransactionRequest,
} from './prepared-transaction';

const creator = '0x1111111111111111111111111111111111111111';
const winner = '0x2222222222222222222222222222222222222222';
const attacker = '0x3333333333333333333333333333333333333333';
const deployment = { USDL: CORE_DEPLOYMENT.usdl, Router: CORE_DEPLOYMENT.router, FeesRouter: CORE_DEPLOYMENT.feesRouter };
const bounty: Bounty = {
  bountyId: 'b-1', creatorWallet: creator, assetAddress: CORE_DEPLOYMENT.usdl,
  budgetRaw: '25000000', title: 'Reviewed bounty', description: 'Reviewed work',
  poolAddress: null, state: 'reviewing',
};
const selection = { submissionId: 's-1', winnerWallet: winner, awardRaw: '5000000' };

// Use the deployed token ABI producer and the JSON transaction DTO transport.
function transported(data: Hex, overrides: Partial<TransactionRequest> = {}): TransactionRequest {
  return JSON.parse(JSON.stringify({
    chainId: '125', from: creator, to: CORE_DEPLOYMENT.usdl, data,
    valueRaw: '0', gasLimitRaw: '65000', maxFeePerGasRaw: '1000000000',
    maxPriorityFeePerGasRaw: null, ...overrides,
  }));
}

function transfer(recipient: Address, amount: bigint): Hex {
  return encodeFunctionData({ abi: erc20Abi, functionName: 'transfer', args: [recipient, amount] });
}

function refusal(request: TransactionRequest, intent: BountyTransactionIntent): PreparedTransactionError {
  let failure: unknown;
  try { validatePreparedTransaction(request, { wallet: creator, deployment, intent }); }
  catch (error) { failure = error; }
  expect(failure).toBeInstanceOf(PreparedTransactionError);
  return failure as PreparedTransactionError;
}

function prepared_call_not_bound_rejects_malicious_selector_and_spender() {
  const funding = createFundingIntent(bounty);
  for (const spender of [attacker, CORE_DEPLOYMENT.router, CORE_DEPLOYMENT.feesRouter] as const) {
    for (const amount of [25_000_000n, (1n << 256n) - 1n]) {
      const data = encodeFunctionData({ abi: erc20Abi, functionName: 'approve', args: [spender, amount] });
      const error = refusal(transported(data), funding);
      expect(error.reason).toBe('operation');
      expect(error.message).toContain('approval spender');
    }
  }
  const delegated = encodeFunctionData({ abi: erc20Abi, functionName: 'transferFrom', args: [creator, attacker, 25_000_000n] });
  expect(refusal(transported(delegated), funding).reason).toBe('operation');
  const read = encodeFunctionData({ abi: erc20Abi, functionName: 'balanceOf', args: [creator] });
  expect(refusal(transported(read), funding).reason).toBe('operation');
  expect(refusal(transported('0xdeadbeef'), funding).reason).toBe('calldata');
}

function prepared_call_not_bound_rejects_changed_winner_amount_asset_and_value() {
  const settlement = createSettlementIntent(bounty, selection);
  expect(refusal(transported(transfer(attacker, 5_000_000n)), settlement).reason).toBe('intent');
  for (const amount of [0n, 1n, 4_999_999n, 5_000_001n, 25_000_001n, (1n << 256n) - 1n]) {
    expect(refusal(transported(transfer(winner, amount)), settlement).reason).toBe('amount');
  }
  expect(refusal(transported(transfer(winner, 5_000_000n), { to: CORE_DEPLOYMENT.router }), settlement).reason).toBe('operation');
  expect(refusal(transported(transfer(winner, 5_000_000n), { valueRaw: '1' }), settlement).reason).toBe('value');
  const otherAsset = createSettlementIntent({ ...bounty, assetAddress: CORE_DEPLOYMENT.feesRouter }, selection);
  expect(refusal(transported(transfer(winner, 5_000_000n)), otherAsset).reason).toBe('operation');
  const tooLarge = createSettlementIntent(bounty, { ...selection, awardRaw: '25000001' });
  expect(refusal(transported(transfer(winner, 25_000_001n)), tooLarge).reason).toBe('amount');
}

function prepared_call_not_bound_requires_canonical_calldata_and_local_review() {
  const settlement = createSettlementIntent(bounty, selection);
  const data = transfer(winner, 5_000_000n);
  expect(refusal(transported(`${data}00`), settlement).reason).toBe('calldata');
  expect(refusal(transported(data.slice(0, -2) as Hex), settlement).reason).toBe('calldata');
  expect(refusal(transported(data), undefined as unknown as BountyTransactionIntent).reason).toBe('intent');
  expect(refusal(transported(data), createSettlementIntent({ ...bounty, creatorWallet: attacker }, selection)).reason).toBe('intent');
  expect(refusal(transported(data), createSettlementIntent(bounty, { ...selection, submissionId: '' })).reason).toBe('intent');
  expect(refusal(transported(data), createSettlementIntent(bounty, { ...selection, winnerWallet: '0x0000000000000000000000000000000000000000' })).reason).toBe('intent');
}

function prepared_call_not_bound_refuses_plain_transfers_without_escrow_identity() {
  const funding = createFundingIntent(bounty);
  for (const recipient of [attacker, winner, CORE_DEPLOYMENT.router] as const) {
    expect(refusal(transported(transfer(recipient, 25_000_000n)), funding).reason).toBe('operation');
  }
  const settlement = createSettlementIntent(bounty, selection);
  const exactTransfer = transported(transfer(winner, 5_000_000n));
  const error = refusal(exactTransfer, settlement);
  expect(error.reason).toBe('operation');
  expect(error.message).toContain('without a deployed escrow operation');
  // An invented role cannot grant an existing router an escrow ABI.
  expect(() => validatePreparedTransaction(exactTransfer, {
    wallet: creator, intent: settlement, deployment: { ...deployment, BountyEscrow: CORE_DEPLOYMENT.router },
  })).toThrow(PreparedTransactionError);
}

function prepared_call_not_bound_snapshots_the_reviewed_bounty_and_winner() {
  const original = { ...bounty };
  const chosen = { ...selection };
  const funding = createFundingIntent(original);
  const settlement = createSettlementIntent(original, chosen);
  original.assetAddress = CORE_DEPLOYMENT.router;
  original.budgetRaw = '1';
  chosen.winnerWallet = attacker;
  chosen.awardRaw = '1';
  expect(funding.bounty).toEqual({ bountyId: 'b-1', creatorWallet: creator, assetAddress: CORE_DEPLOYMENT.usdl, budgetRaw: '25000000' });
  expect(settlement).toEqual({ action: 'settlement', bounty: funding.bounty, ...selection });
  expect(Object.isFrozen(settlement)).toBe(true);
  expect(Object.isFrozen(settlement.bounty)).toBe(true);
  expect(refusal(transported(transfer(attacker, 1n)), settlement).reason).toBe('amount');
}

test(prepared_call_not_bound_rejects_malicious_selector_and_spender.name, prepared_call_not_bound_rejects_malicious_selector_and_spender);
test(prepared_call_not_bound_rejects_changed_winner_amount_asset_and_value.name, prepared_call_not_bound_rejects_changed_winner_amount_asset_and_value);
test(prepared_call_not_bound_requires_canonical_calldata_and_local_review.name, prepared_call_not_bound_requires_canonical_calldata_and_local_review);
test(prepared_call_not_bound_refuses_plain_transfers_without_escrow_identity.name, prepared_call_not_bound_refuses_plain_transfers_without_escrow_identity);
test(prepared_call_not_bound_snapshots_the_reviewed_bounty_and_winner.name, prepared_call_not_bound_snapshots_the_reviewed_bounty_and_winner);

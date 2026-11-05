/**
 * Proves: limits.ts + card-state.ts cardState + submission.ts planDeposit -> TypeScript exports -> proof_submission.spec.ts consumer.
 * Drives amountLimitError, maxSpendable and routeBlockers through cardState, and
 * planDeposit through the viem ABI encoder/decoder used for contract requests.
 * The anchor lists no external caller: the test consumes the same exports as BridgeCard,
 * including its approve/deposit selection, without replacing any module or transport.
 * Repairs limits.ts routeBlockers ignoring vaultState.error, which let cardState report
 * ready after a failed vault read (even with stale, previously loaded state).
 * No persistent state belongs to this surface; repeated calls and changed inputs prove
 * fresh observations. Malformed output is refused by the real ABI encoder/decoder.
 */
import { decodeFunctionData, encodeFunctionData, padHex, sliceHex, zeroAddress } from 'viem';
import { expect, test } from 'vitest';

import { parseAmount } from '@/lib/bridge/amount';
import { cardState } from '@/lib/bridge/card-state';
import type { CardState, CardStateInputs } from '@/lib/bridge/card-state';
import { erc20Abi, paxeerXVaultAbi } from '@/lib/bridge/contracts';
import { amountLimitError, maxSpendable, routeBlockers } from '@/lib/bridge/limits';
import { planDeposit } from '@/lib/bridge/submission';
import type { DepositPlanArgs } from '@/lib/bridge/submission';

const vault = '0x1111111111111111111111111111111111111111';
const recipient = '0x2222222222222222222222222222222222222222';
const token = { address: '0x3333333333333333333333333333333333333333', isNative: false } as const;
const unit = 10n ** 18n;

function submissionInputs(): CardStateInputs {
  return {
    direction: 'in',
    asset: { symbol: 'TST', decimals: 18, isNative: false, perTxCap: String(10n * unit) },
    amount: '1.000000000000000001',
    balance: 3n * unit,
    gasReserve: unit / 1000n,
    foreign: { state: 'configured', label: 'Base', vault },
    paxeerConfigured: true,
    attestationApiConfigured: true,
    paxeerState: {
      paused: false,
      registered: true,
      enabled: true,
      vault,
      finalityDepth: 1,
      denom: 'bridged/test',
      maxInFlight: 100n * unit,
      maxPerTx: 10n * unit,
      inFlight: 0n,
      attestors: [],
      threshold: undefined,
      loaded: true,
      error: false,
    },
    vaultState: {
      paused: false,
      perTx: 10n * unit,
      total: 100n * unit,
      outstanding: 2n * unit,
      attestors: [],
      threshold: undefined,
      loaded: true,
      error: false,
    },
    allowanceError: false,
    isConnected: true,
    onSourceChain: true,
    destinationValid: true,
    depositPlanAvailable: true,
    busy: false,
  };
}

function submissionTokenProducerReachesTheConsumerTwice(): void {
  const inputs = submissionInputs();
  let previousState: CardState | undefined;
  for (const iteration of [ 0n, 1n ]) {
    inputs.amount = iteration === 0n ? '1.000000000000000001' : '1.000000000000000002';
    const amount = parseAmount(inputs.amount, inputs.asset.decimals);
    expect(amount).toBe(unit + iteration + 1n);
    if (amount === undefined) throw new Error('The producer amount must parse.');
    const args: DepositPlanArgs = { asset: token, amount, allowance: amount - 1n, vault, recipient };
    const plan = planDeposit(args);
    const state = cardState({ ...inputs, depositPlanAvailable: plan !== undefined });
    expect(state).toEqual({ inputError: undefined, blockers: [], maxAmount: '3', ready: true });
    expect(state).not.toBe(previousState);
    expect(state.blockers).not.toBe(previousState?.blockers);
    previousState = state;
    expect(routeBlockers(inputs)).toEqual([]);
    expect(amountLimitError({ ...inputs.vaultState, direction: 'in', amount, balance: inputs.balance })).toBeUndefined();
    expect(maxSpendable(inputs.balance!, false, inputs.gasReserve)).toBe(3n * unit);

    const approveStep = plan.find((step) => step.functionName === 'approve');
    const depositStep = plan.find((step) => step.functionName !== 'approve');
    expect(plan.map((step) => step.functionName)).toEqual([ 'approve', 'deposit' ]);
    if (!approveStep || depositStep?.functionName !== 'deposit') throw new Error('Expected token approval and deposit.');
    expect(approveStep.address).toBe(token.address);
    expect(depositStep.address).toBe(vault);
    expect(decodeFunctionData({ abi: erc20Abi, data: encodeFunctionData(approveStep) })).toEqual({
      functionName: 'approve', args: [ vault, amount ],
    });
    expect(decodeFunctionData({ abi: paxeerXVaultAbi, data: encodeFunctionData(depositStep) })).toEqual({
      functionName: 'deposit', args: [ token.address, amount, padHex(recipient, { size: 32 }) ],
    });

    // Observe the allowance refresh through a second real plan, as BridgeCard does.
    const approvedPlan = planDeposit({ ...args, allowance: amount });
    expect(approvedPlan.map((step) => step.functionName)).toEqual([ 'deposit' ]);
    expect(approvedPlan).not.toBe(plan);
    expect(approvedPlan[0]).toEqual(depositStep);
    const approvedStep = approvedPlan[0];
    if (approvedStep.functionName !== 'deposit') throw new Error('Expected a deposit after allowance refresh.');
    expect(decodeFunctionData({ abi: paxeerXVaultAbi, data: encodeFunctionData(approvedStep) })).toEqual({
      functionName: 'deposit', args: [ token.address, amount, padHex(recipient, { size: 32 }) ],
    });
  }
}

function submissionNativeMaxReachesTheConsumerTwice(): void {
  const inputs = submissionInputs();
  inputs.asset = { ...inputs.asset, symbol: 'ETH', isNative: true };
  inputs.balance = 2n * unit;
  for (const allowance of [ 0n, 3n * unit ]) {
    const beforeMax = cardState(inputs);
    expect(beforeMax.maxAmount).toBe('1.999');
    if (beforeMax.maxAmount === undefined) throw new Error('Expected the Max amount.');
    const amount = parseAmount(beforeMax.maxAmount, inputs.asset.decimals);
    expect(amount).toBe(maxSpendable(inputs.balance, true, inputs.gasReserve));
    expect(amount).toBe(1999000000000000000n);
    if (amount === undefined) throw new Error('The Max amount must parse.');
    const plan = planDeposit({ asset: { address: zeroAddress, isNative: true }, amount, allowance, vault, recipient });
    const state = cardState({ ...inputs, amount: beforeMax.maxAmount, depositPlanAvailable: plan !== undefined });
    expect(state).toEqual({ inputError: undefined, blockers: [], maxAmount: '1.999', ready: true });
    expect(plan.map((step) => step.functionName)).toEqual([ 'depositNative' ]);
    const step = plan[0];
    if (step.functionName !== 'depositNative') throw new Error('Expected a native deposit.');
    expect(step.address).toBe(vault);
    expect(step.value).toBe(amount);
    expect(decodeFunctionData({ abi: paxeerXVaultAbi, data: encodeFunctionData(step) })).toEqual({
      functionName: 'depositNative', args: [ padHex(recipient, { size: 32 }) ],
    });
  }
}

function submissionConsumerRefusesRouteAndAmountFailures(): void {
  const inputs = submissionInputs();
  const paused = 'The bridge is paused on Paxeer. No deposits or withdrawals will be accepted until it reopens.';
  const unreadable = 'The vault state could not be read. The route cannot be verified on-chain right now.';
  for (const loaded of [ false, true ]) {
    const failedRead = { ...inputs, vaultState: { ...inputs.vaultState, loaded, error: true } };
    for (let repeat = 0; repeat < 2; repeat++) {
      expect(routeBlockers(failedRead)).toEqual([ unreadable ]);
      expect(cardState(failedRead)).toEqual({ inputError: undefined, blockers: [ unreadable ], maxAmount: '3', ready: false });
    }
    expect(cardState(inputs).ready).toBe(true);
  }
  const pausedInputs = { ...inputs, paxeerState: { ...inputs.paxeerState, paused: true } };
  expect(routeBlockers(pausedInputs)).toEqual([ paused ]);
  expect(cardState(pausedInputs)).toEqual({ inputError: undefined, blockers: [ paused ], maxAmount: '3', ready: false });
  expect(cardState({ ...inputs, onSourceChain: false }).ready).toBe(false);
  expect(cardState({ ...inputs, depositPlanAvailable: false }).ready).toBe(false);
  const malformed = cardState({ ...inputs, amount: '1,5' });
  expect(malformed.inputError).toBe('Enter a valid amount.');
  expect(malformed.ready).toBe(false);
  const amount = 2n * unit + 1n;
  const outstandingError = 'That is more than the vault\'s outstanding balance for this asset.';
  expect(amountLimitError({ ...inputs.vaultState, direction: 'out', amount, balance: inputs.balance })).toBe(outstandingError);
  expect(cardState({ ...inputs, direction: 'out', amount: '2.000000000000000001' })).toEqual({
    inputError: outstandingError, blockers: [], maxAmount: '3', ready: false,
  });
}

function submissionConsumerRefusesMalformedProducerOutput(): void {
  const args: DepositPlanArgs = { asset: token, amount: unit + 1n, allowance: unit + 1n, vault, recipient };
  for (let repeat = 0; repeat < 2; repeat++) {
    const step = planDeposit(args)[0];
    if (step.functionName !== 'deposit') throw new Error('Expected a token deposit.');
    const data = encodeFunctionData(step);
    expect(decodeFunctionData({ abi: paxeerXVaultAbi, data })).toEqual({
      functionName: 'deposit', args: [ token.address, args.amount, padHex(recipient, { size: 32 }) ],
    });
    // Corrupt only output emitted by the real producer; use the actual crossing codecs.
    const malformedStep = { ...step, args: [ step.args[0], step.args[1], sliceHex(step.args[2], 0, 31) ] } as const;
    expect(() => encodeFunctionData(malformedStep)).toThrow();
    expect(() => decodeFunctionData({ abi: paxeerXVaultAbi, data: sliceHex(data, 0, 4) })).toThrow();
    expect(() => planDeposit({ ...args, recipient: '0x1234' })).toThrow();
    const recoveredStep = planDeposit(args)[0];
    if (recoveredStep.functionName !== 'deposit') throw new Error('Expected a deposit after malformed input refusal.');
    expect(encodeFunctionData(recoveredStep)).toBe(data);
  }
}

test(submissionTokenProducerReachesTheConsumerTwice);
test(submissionNativeMaxReachesTheConsumerTwice);
test(submissionConsumerRefusesRouteAndAmountFailures);
test(submissionConsumerRefusesMalformedProducerOutput);

import { expect, test } from 'vitest';

import { parseAmount } from './amount';
import { cardState } from './card-state';
import type { CardStateInputs } from './card-state';
import { amountLimitError, maxSpendable, routeBlockers } from './limits';

function cardInputs(): CardStateInputs {
  return {
    direction: 'in',
    asset: { symbol: 'ETH', decimals: 18, isNative: true, perTxCap: '100000000000000000000' },
    amount: '1',
    balance: 10000000000000000000n,
    gasReserve: 1000000000000000n,
    foreign: { state: 'configured', label: 'Base', vault: '0x0000000000000000000000000000000000000001' },
    paxeerConfigured: true,
    attestationApiConfigured: true,
    paxeerState: {
      paused: false,
      registered: true,
      enabled: true,
      vault: '0x0000000000000000000000000000000000000001',
      finalityDepth: 1,
      denom: 'bridged/test',
      maxInFlight: 100000000000000000000n,
      maxPerTx: 100000000000000000000n,
      inFlight: 0n,
      attestors: [],
      threshold: undefined,
      loaded: true,
      error: false,
    },
    vaultState: {
      paused: false,
      perTx: 100000000000000000000n,
      total: 100000000000000000000n,
      outstanding: 2000000000000000000n,
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

function cardStateIsNotReadyWhileABlockerExists() {
  const inputs = cardInputs();
  expect(cardState(inputs)).toMatchObject({ inputError: undefined, blockers: [], ready: true });
  const blockedRoutes: Array<Partial<CardStateInputs>> = [
    { foreign: { state: 'not-deployed', label: 'Base', vault: undefined } },
    { foreign: { state: 'awaiting-backend', label: 'Solana', vault: undefined } },
    { paxeerConfigured: false },
    { paxeerState: { ...inputs.paxeerState, error: true } },
    { paxeerState: { ...inputs.paxeerState, error: true, loaded: false } },
    { paxeerState: { ...inputs.paxeerState, paused: true } },
    { paxeerState: { ...inputs.paxeerState, registered: false } },
    { paxeerState: { ...inputs.paxeerState, enabled: false } },
    { paxeerState: { ...inputs.paxeerState, vault: '0x0000000000000000000000000000000000000002' } },
    { vaultState: { ...inputs.vaultState, paused: true } },
    { attestationApiConfigured: false },
  ];
  for (const direction of [ 'in', 'out' ] as const) {
    for (const route of blockedRoutes) {
      const args = { ...inputs, ...route, direction };
      const expectedBlockers = routeBlockers(args);
      expect(expectedBlockers.length).toBeGreaterThan(0);
      const state = cardState(args);
      expect(state.blockers).toEqual(expectedBlockers);
      expect(state.inputError).toBeUndefined();
      expect(state.ready).toBe(false);
    }
  }
  const allowanceInputs = { ...inputs, asset: { ...inputs.asset, isNative: false }, allowanceError: true };
  expect(cardState(allowanceInputs).blockers).toContain(
    'Could not read the token allowance. Wait for the allowance to refresh before depositing.',
  );
  expect(cardState(allowanceInputs).ready).toBe(false);
  expect(cardState({ ...allowanceInputs, direction: 'out' }).ready).toBe(true);
  expect(cardState({ ...inputs, allowanceError: true }).ready).toBe(true);

  for (const condition of [
    { isConnected: false },
    { onSourceChain: false },
    { destinationValid: false },
    { depositPlanAvailable: false },
    { busy: true },
    { amount: '' },
    { amount: '0' },
    { amount: 'invalid' },
    { amount: '0.0000000000000000001' },
  ]) {
    expect(cardState({ ...inputs, ...condition }).ready).toBe(false);
  }
  expect(cardState({ ...inputs, direction: 'out', depositPlanAvailable: false }).ready).toBe(true);
  const overOutstanding = { ...inputs, direction: 'out' as const, amount: '3' };
  expect(cardState(overOutstanding).inputError).toBe(amountLimitError({
    direction: 'out',
    amount: 3000000000000000000n,
    balance: inputs.balance,
    perTx: inputs.vaultState.perTx,
    total: inputs.vaultState.total,
    outstanding: inputs.vaultState.outstanding,
  }));
  expect(cardState(overOutstanding).ready).toBe(false);
  expect(cardState({ ...inputs, amount: '11' }).inputError).toBe('That is more than your available balance.');
  expect(cardState({ ...inputs, vaultState: { ...inputs.vaultState, perTx: 1n } }).inputError)
    .toBe('Above the vault per-transaction cap.');
  expect(cardState({ ...inputs, vaultState: { ...inputs.vaultState, total: inputs.vaultState.outstanding } }).inputError)
    .toBe('That would exceed the vault\'s total capacity for this asset.');
  expect(cardState({ ...inputs, paxeerState: { ...inputs.paxeerState, maxPerTx: 1n } }).ready).toBe(false);
  expect(cardState({ ...inputs, paxeerState: { ...inputs.paxeerState, maxInFlight: 1n } }).ready).toBe(false);
}

function cardStateMaxAmountReservesNativeGas() {
  const inputs = cardInputs();
  const balance = 1234567890123456789012n;
  const gasReserve = 1000000000000000n;
  const native = cardState({ ...inputs, balance, gasReserve });
  expect(native.maxAmount).toBe('1234.566890123456789012');
  expect(native.maxAmount).not.toContain(',');
  expect(parseAmount(native.maxAmount!, 18)).toBe(maxSpendable(balance, true, gasReserve));
  const token = cardState({ ...inputs, asset: { ...inputs.asset, isNative: false }, balance, gasReserve });
  expect(token.maxAmount).toBe('1234.567890123456789012');
  expect(parseAmount(token.maxAmount!, 18)).toBe(balance);
  expect(cardState({ ...inputs, direction: 'out', balance, gasReserve }).maxAmount).toBe(token.maxAmount);
  for (const available of [ 0n, gasReserve - 1n, gasReserve ]) {
    expect(cardState({ ...inputs, balance: available, gasReserve }).maxAmount).toBe('0');
  }
  expect(cardState({ ...inputs, balance: undefined }).maxAmount).toBeUndefined();
  expect(cardState({ ...inputs, balance: 1n, gasReserve: 0n }).maxAmount).toBe('0.000000000000000001');
  expect(cardState({
    ...inputs,
    asset: { ...inputs.asset, decimals: 6 },
    balance: 1234567890n,
    gasReserve: 1000n,
  }).maxAmount).toBe('1234.56689');
  expect(inputs.balance).toBe(10000000000000000000n);
  expect(cardState(inputs)).toEqual(cardState(inputs));
}

test(cardStateIsNotReadyWhileABlockerExists);
test(cardStateMaxAmountReservesNativeGas);

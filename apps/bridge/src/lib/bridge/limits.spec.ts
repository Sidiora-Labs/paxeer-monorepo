import { expect, test } from 'vitest';

import { amountLimitError, maxSpendable, routeBlockers } from './limits';
import type { AmountLimitArgs, RouteBlockerArgs } from './limits';

function amountLimitErrorRefusesAWithdrawalAboveOutstanding() {
  const args: AmountLimitArgs = {
    direction: 'out',
    amount: BigInt(11),
    perTx: BigInt(100),
    total: BigInt(100),
    outstanding: BigInt(10),
    balance: BigInt(100),
  };
  expect(amountLimitError(args)).toBe('That is more than the vault\'s outstanding balance for this asset.');
  expect(amountLimitError({ ...args, amount: BigInt(10) })).toBeUndefined();
  expect(amountLimitError({ ...args, outstanding: BigInt(0), amount: BigInt(1) }))
    .toBe('That is more than the vault\'s outstanding balance for this asset.');
  expect(amountLimitError({ ...args, amount: BigInt(101), outstanding: BigInt(200), balance: BigInt(200) }))
    .toBe('Above the vault per-transaction cap.');
  expect(amountLimitError({ ...args, amount: BigInt(10), balance: BigInt(9) }))
    .toBe('That is more than your available balance.');
  expect(amountLimitError({ ...args, amount: BigInt(10), perTx: BigInt(0), balance: undefined })).toBeUndefined();
}

function amountLimitErrorAppliesInFlightCapOnlyToDeposits() {
  const args: AmountLimitArgs = {
    direction: 'in',
    amount: BigInt(6),
    perTx: BigInt(100),
    total: BigInt(100),
    outstanding: BigInt(95),
    balance: BigInt(100),
  };
  expect(amountLimitError(args)).toBe('That would exceed the vault\'s total capacity for this asset.');
  expect(amountLimitError({ ...args, direction: 'out' })).toBeUndefined();
  expect(amountLimitError({ ...args, amount: BigInt(5) })).toBeUndefined();
  expect(amountLimitError({ ...args, total: BigInt(0) })).toBeUndefined();
  expect(amountLimitError({ ...args, outstanding: BigInt(101), direction: 'out' })).toBeUndefined();
  expect(amountLimitError({ ...args, amount: BigInt(101), total: BigInt(1000), balance: BigInt(200) }))
    .toBe('Above the vault per-transaction cap.');
  expect(amountLimitError({ ...args, balance: BigInt(5) })).toBe('That is more than your available balance.');

  const large = BigInt('9007199254740993');
  expect(amountLimitError({ ...args, total: large, outstanding: large - BigInt(5) }))
    .toBe('That would exceed the vault\'s total capacity for this asset.');
}

function maxSpendableReservesNativeGas() {
  const balance = BigInt('1000000000000000000');
  const reserve = BigInt('1000000000000000');
  expect(maxSpendable(balance, true, reserve)).toBe(BigInt('999000000000000000'));
  expect(maxSpendable(balance, false, reserve)).toBe(balance);
  expect(maxSpendable(reserve - BigInt(1), true, reserve)).toBe(BigInt(0));
  expect(maxSpendable(reserve, true, reserve)).toBe(BigInt(0));
  expect(maxSpendable(BigInt(0), true, reserve)).toBe(BigInt(0));
  expect(maxSpendable(balance, true, BigInt(0))).toBe(balance);
  expect(maxSpendable(BigInt(1), false, reserve)).toBe(BigInt(1));
}

function routeBlockersReportsAFailedPaxeerRead() {
  const args: RouteBlockerArgs = {
    paxeerState: {
      paused: false,
      registered: true,
      enabled: true,
      vault: '0x0000000000000000000000000000000000000001',
      finalityDepth: 1,
      denom: 'bridged/test',
      maxInFlight: BigInt(100),
      maxPerTx: BigInt(10),
      inFlight: BigInt(0),
      attestors: [],
      threshold: undefined,
      loaded: true,
      error: false,
    },
    vaultState: {
      paused: false,
      perTx: BigInt(10),
      total: BigInt(100),
      outstanding: BigInt(0),
      attestors: [],
      threshold: undefined,
      loaded: true,
      error: false,
    },
    attestationApiConfigured: true,
  };
  const readBlocker = 'The Paxeer bridge state could not be read. The route cannot be verified on-chain right now.';
  expect(routeBlockers(args)).toEqual([]);
  expect(routeBlockers({ ...args, paxeerState: { ...args.paxeerState, error: true } })).toEqual([ readBlocker ]);
  expect(routeBlockers({ ...args, paxeerState: { ...args.paxeerState, error: true, loaded: false } }))
    .toEqual([ readBlocker ]);
  expect(routeBlockers({ ...args, paxeerState: { ...args.paxeerState, loaded: false } })).toEqual([]);
  expect(routeBlockers({ ...args, paxeerState: { ...args.paxeerState, paused: true } }))
    .toContain('The bridge is paused on Paxeer. No deposits or withdrawals will be accepted until it reopens.');
  expect(routeBlockers({ ...args, paxeerState: { ...args.paxeerState, registered: false } }))
    .toContain('This chain is not registered with the Paxeer bridge yet. Governance registers a chain after its vault is deployed.');
  expect(routeBlockers({ ...args, paxeerState: { ...args.paxeerState, enabled: false } }))
    .toContain('This chain is registered but currently disabled on Paxeer.');
  expect(routeBlockers({ ...args, vaultState: { ...args.vaultState, paused: true } }))
    .toContain('The vault on the foreign chain is paused.');
  expect(routeBlockers({ ...args, attestationApiConfigured: false }))
    .toContain('The bridge attestation API is not configured for this deployment.');
  expect(routeBlockers({ ...args, paxeerConfigured: false }))
    .toContain('The Paxeer RPC is not configured for this deployment, so the bridge cannot be verified on-chain here.');
  expect(routeBlockers({
    ...args,
    foreign: { state: 'not-deployed', label: 'Base', vault: undefined },
  })).toContain('The Base vault is not deployed yet. This route opens once the deployment record is published.');
  for (const direction of [ 'in', 'out' ] as const) {
    expect(routeBlockers({
      ...args,
      direction,
      foreign: { state: 'awaiting-backend', label: 'Solana', vault: undefined },
    }).some((blocker) => blocker.includes('Solana'))).toBe(true);
  }
  expect(routeBlockers({
    ...args,
    foreign: { state: 'configured', label: 'Base', vault: '0x0000000000000000000000000000000000000002' },
  })).toContain('The vault registered on Paxeer for this chain does not match the vault this app is configured with. Do not deposit.');
  expect(routeBlockers({
    ...args,
    foreign: { state: 'configured', label: 'Base', vault: args.paxeerState.vault },
  })).toEqual([]);
}

test(amountLimitErrorRefusesAWithdrawalAboveOutstanding);
test(amountLimitErrorAppliesInFlightCapOnlyToDeposits);
test(maxSpendableReservesNativeGas);
test(routeBlockersReportsAFailedPaxeerRead);

// @vitest-environment jsdom
// @vitest-environment-options {"storageQuota":10000}

import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import type { SwapQuote } from '@/lib/swap/types';

declare const jsdom: { window: Pick<Window, 'localStorage'> };

const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
const KEY = 'pax:swap:operation:v1:125:https%3A%2F%2Frpc.example:0x1111';
const OTHER_KEY = 'pax:swap:operation:v1:125000001:https%3A%2F%2Frpc.example:0x2222';
const operation = { state: 'submitted', hash: `0x${'ab'.repeat(32)}` };

beforeEach(() => {
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    get: () => jsdom.window.localStorage,
  });
  localStorage.clear();
  vi.resetModules();
});

afterEach(() => {
  localStorage.clear();
  if (descriptor) Object.defineProperty(globalThis, 'localStorage', descriptor);
  else Reflect.deleteProperty(globalThis, 'localStorage');
});

async function swap_records_migrate_and_reopen_without_crossing_network_or_wallet_keys() {
  const serialized = JSON.stringify(operation);
  localStorage.setItem(KEY, serialized);
  localStorage.setItem('unrelated', 'preserve');
  let { swapStorage, writeSwapRecord } = await import('@/lib/swap/storage');
  expect(swapStorage().getItem(KEY)).toBe(serialized);
  expect(localStorage.getItem(KEY)).toBe(serialized);
  expect(localStorage.getItem('unrelated')).toBe('preserve');
  expect(JSON.parse(localStorage.getItem(KEY)!)).toEqual(operation);
  writeSwapRecord(swapStorage(), OTHER_KEY, { ...operation, state: 'confirmed' });
  vi.resetModules();
  ({ swapStorage, writeSwapRecord } = await import('@/lib/swap/storage'));
  expect(JSON.parse(swapStorage().getItem(KEY)!)).toEqual(operation);
  expect(JSON.parse(swapStorage().getItem(OTHER_KEY)!)).toEqual({
    ...operation,
    state: 'confirmed',
  });
  swapStorage().removeItem(OTHER_KEY);
  expect(swapStorage().getItem(OTHER_KEY)).toBeNull();
  expect(JSON.parse(swapStorage().getItem(KEY)!)).toEqual(operation);
  const { storageCatalog, resetStorageForLifecycle } = await import('./registry');
  expect(storageCatalog().find((entry) => entry.id === 'swap-records')).toMatchObject({
    owner: 'swap',
    corruption: 'fail-closed',
  });
  resetStorageForLifecycle('reset');
  expect(swapStorage().getItem(KEY)).toBeNull();
  expect(localStorage.getItem('unrelated')).toBe('preserve');
}

async function swap_records_quota_refusal_retains_legacy_data_across_reopen_and_retry() {
  const serialized = JSON.stringify(operation);
  const replacement = {
    ...operation,
    state: 'confirmed',
    receipt: 'persist after quota is released',
  };
  localStorage.setItem(KEY, serialized);
  localStorage.setItem('filler', 'x'.repeat(10_000 - KEY.length - serialized.length - 6));
  expect(() => localStorage.setItem('overflow', 'x')).toThrow(/quota/i);
  for (let attempt = 0; attempt < 2; attempt += 1) {
    const { swapStorage, writeSwapRecord } = await import('@/lib/swap/storage');
    expect(() => writeSwapRecord(swapStorage(), KEY, replacement)).toThrow(
      expect.objectContaining({ name: 'StoragePersistenceError', code: 'STORAGE_WRITE_FAILED' })
    );
    expect(localStorage.getItem(KEY)).toBe(serialized);
    expect(swapStorage().getItem(KEY)).toBe(serialized);
    vi.resetModules();
  }
  localStorage.removeItem('filler');
  const { swapStorage, writeSwapRecord } = await import('@/lib/swap/storage');
  writeSwapRecord(swapStorage(), KEY, replacement);
  expect(JSON.parse(swapStorage().getItem(KEY)!)).toEqual(replacement);
  expect(JSON.parse(localStorage.getItem(KEY)!)).toEqual(replacement);
}

async function swap_records_corruption_refuses_recovery_and_preserves_invalid_legacy_source() {
  localStorage.setItem(KEY, '{broken');
  const { swapStorage } = await import('@/lib/swap/storage');
  expect(() => swapStorage().getItem(KEY)).toThrow(
    expect.objectContaining({ name: 'StorageCorruptionError' })
  );
  expect(localStorage.getItem(KEY)).toBe('{broken');
}

async function swap_journal_reopens_producer_intents_and_clears_only_its_namespace() {
  const { SwapJournal } = await import('@/widgets/swap/swapJournal');
  const { TOKENS } = await import('@/lib/swap/sdk/addresses');
  let { swapJournalStorage } = await import('./swap-records');
  const account = `0x${'11'.repeat(20)}`;
  const quote: SwapQuote = {
    protocol: 'pecor',
    protocolLabel: 'PECOR',
    tokenIn: TOKENS.USDC,
    tokenOut: TOKENS.USDT,
    amountIn: '1000000',
    amountOut: '1000000',
    amountOutDisplay: '1',
    fee: '0',
    feeBps: 0,
    priceImpact: 0,
    sufficient: true,
  };
  const producer = new SwapJournal(swapJournalStorage, account, 125);
  const intent = producer.begin(quote, {
    tokenIn: quote.tokenIn,
    tokenOut: quote.tokenOut,
    amountIn: quote.amountIn,
    slippageBps: 50,
    recipient: account,
  });
  const serialized = localStorage.getItem(producer.key);
  localStorage.setItem(KEY, JSON.stringify(operation));
  expect(swapJournalStorage.length).toBe(1);
  expect(swapJournalStorage.key(0)).toBe(producer.key);
  expect(swapJournalStorage.key(1)).toBeNull();
  vi.resetModules();
  ({ swapJournalStorage } = await import('./swap-records'));
  const reopened = new SwapJournal(swapJournalStorage, account, 125);
  expect(reopened.current()).toEqual(intent);
  expect(localStorage.getItem(producer.key)).toBe(serialized);
  expect(new SwapJournal(swapJournalStorage, account, 125000001).current()).toBeNull();
  expect(new SwapJournal(swapJournalStorage, `0x${'22'.repeat(20)}`, 125).current()).toBeNull();
  expect(() => swapJournalStorage.getItem(KEY)).toThrow('outside the registered namespace');
  localStorage.setItem(producer.key, '{broken');
  expect(() => reopened.current()).toThrow(
    expect.objectContaining({ name: 'StorageCorruptionError' })
  );
  expect(localStorage.getItem(producer.key)).toBe('{broken');
  localStorage.setItem(producer.key, serialized!);
  swapJournalStorage.clear();
  expect(swapJournalStorage.length).toBe(0);
  expect(reopened.current()).toBeNull();
  expect(JSON.parse(localStorage.getItem(KEY)!)).toEqual(operation);
}

test(
  swap_records_migrate_and_reopen_without_crossing_network_or_wallet_keys.name,
  swap_records_migrate_and_reopen_without_crossing_network_or_wallet_keys
);
test(
  swap_records_quota_refusal_retains_legacy_data_across_reopen_and_retry.name,
  swap_records_quota_refusal_retains_legacy_data_across_reopen_and_retry
);
test(
  swap_records_corruption_refuses_recovery_and_preserves_invalid_legacy_source.name,
  swap_records_corruption_refuses_recovery_and_preserves_invalid_legacy_source
);
test(
  swap_journal_reopens_producer_intents_and_clears_only_its_namespace.name,
  swap_journal_reopens_producer_intents_and_clears_only_its_namespace
);

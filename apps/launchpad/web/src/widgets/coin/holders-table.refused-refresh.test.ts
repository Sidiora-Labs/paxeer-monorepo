import { expect, test } from 'vitest';
import type { HoldersPage, HoldersResponse } from '@/core/market/use-holders';
import { buildHolderRows, reconcileHolderFirstPage } from './holders-table';

const wallets = ['0x' + '1'.repeat(40), '0x' + '2'.repeat(40), '0x' + '3'.repeat(40)];
const tokenAddress = '0x' + 'c'.repeat(40);
const token = { decimals: 6, decimalsQuality: 'verified' as const, totalSupplyRaw: '1000000000', supplyQuality: 'verified' as const };

function canonicalPage(revision: string, balances: readonly string[], nextCursor: string | null = null): HoldersPage {
  const response: HoldersResponse = {
    data: {
      coverage: 'complete',
      excludedProtocolAddresses: [],
      exclusionPolicyVersion: '1',
      fromPosition: null,
      toPosition: null,
      items: balances.map((balanceRaw, index) => ({ wallet: wallets[index], tokenAddress, balanceRaw, share: null })),
    },
    generatedAt: '2026-01-01T00:00:00.000Z',
    nextCursor,
    quality: 'complete',
    requestId: 'holders',
    revision,
    watermark: null,
  };
  return { response, rows: response.data.items, canonical: null, legacyShare: null, legacySource: null, nextCursor };
}

function refusedPage(): HoldersPage {
  return {
    response: null,
    rows: [],
    canonical: { state: 'unsupported', operation: 'IF_HOLDERS', status: 503, message: 'Holders unavailable' },
    legacyShare: { [wallets[0]]: { holderAddress: wallets[0], balance: '7000000', pctOfSupply: '70', rank: 1 } },
    legacySource: 'stats-host',
    nextCursor: null,
  };
}

function refusedRefreshKeepsTheLoadedPages() {
  const held = { first: canonicalPage('1', ['20000000', '10000000'], 'later'), later: canonicalPage('1', ['20000000', '10000000', '5000000']) };
  const refused = refusedPage();
  const result = reconcileHolderFirstPage(held, refused);

  expect(result.pages).toBe(held);
  expect(result.pages.first).toBe(held.first);
  expect(result.pages.later).toBe(held.later);
  expect(result.resetCursor).toBe(false);
  expect(result.canonical).toBe(refused.canonical);
  expect(buildHolderRows(Object.values(result.pages), token, null)).toEqual(buildHolderRows(Object.values(held), token, null));
  expect(buildHolderRows([result.pages.first], token, null).map(row => row.balanceRaw)).toEqual(['20000000', '10000000']);
}

function nextAnswerTakesThePlaceOfThePages() {
  const held = { first: canonicalPage('1', ['20000000', '10000000'], 'later'), later: canonicalPage('1', ['20000000', '10000000', '5000000']) };
  const refused = reconcileHolderFirstPage(held, refusedPage());
  expect(refused.canonical).not.toBeNull();
  const next = canonicalPage('2', ['30000000', '15000000']);
  const result = reconcileHolderFirstPage(refused.pages, next);

  expect(result.pages).toEqual({ first: next });
  expect(result.pages.first).toBe(next);
  expect(result.resetCursor).toBe(true);
  expect(result.canonical).toBeNull();
  expect(buildHolderRows(Object.values(result.pages), token, null).map(row => row.balanceRaw)).toEqual(['30000000', '15000000']);
}

function refusedFirstReadWithoutAKeptAnswerShowsTheShareRows() {
  const refused = refusedPage();
  const result = reconcileHolderFirstPage({}, refused);

  expect(result.pages).toEqual({ first: refused });
  expect(result.pages.first).toBe(refused);
  expect(result.resetCursor).toBe(true);
  expect(result.canonical).toBe(refused.canonical);
  expect(buildHolderRows(Object.values(result.pages), token, null)).toEqual([
    { wallet: wallets[0], balanceRaw: '7000000', supplyPercent: 0.7, positionUsdlRaw: null },
  ]);
}

function sameSnapshotKeepsTheLaterPages() {
  const first = canonicalPage('1', ['20000000', '10000000'], 'later');
  const later = canonicalPage('1', ['20000000', '10000000', '5000000']);
  const held = { first, later };
  const refreshed = { ...first, response: { ...first.response!, requestId: 'holders-refresh' } };
  const result = reconcileHolderFirstPage(held, refreshed);

  expect(result.pages).toEqual({ first: refreshed, later });
  expect(result.pages.first).toBe(refreshed);
  expect(result.pages.later).toBe(later);
  expect(result.resetCursor).toBe(false);
  expect(result.canonical).toBeNull();
  expect(buildHolderRows(Object.values(result.pages), token, null)).toEqual(buildHolderRows(Object.values(held), token, null));
}

test(refusedRefreshKeepsTheLoadedPages, refusedRefreshKeepsTheLoadedPages);
test(nextAnswerTakesThePlaceOfThePages, nextAnswerTakesThePlaceOfThePages);
test(refusedFirstReadWithoutAKeptAnswerShowsTheShareRows, refusedFirstReadWithoutAKeptAnswerShowsTheShareRows);
test(sameSnapshotKeepsTheLaterPages, sameSnapshotKeepsTheLaterPages);

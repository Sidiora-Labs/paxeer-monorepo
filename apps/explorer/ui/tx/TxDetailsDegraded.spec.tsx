// @vitest-environment jsdom

import React from 'react';

import { base } from 'mocks/txs/tx';
import { beforeEach, describe, expect, it, test } from 'vitest';

import {
  TxDetailsDegraded, renderTransaction, degradedNodeReceiptReachesOverview,
  degradedRevertedReceiptReachesOverview, degradedReceiptErrorRecoversToIndexedTransaction,
} from './repair-22-33-519.fixtures';
import type { TxQuery } from './useTxQuery';

// The degraded page reads the transaction from the node, and the node query ships the RPC
// placeholders it renders until the node answers. These layout cases leave that query unfetched;
// the enabled-query regressions below exercise node receipts and recovery through the real page.
const buildTxQuery = (status?: number): TxQuery => ({
  data: undefined,
  error: status === undefined ? null : { status, payload: undefined },
  isError: status !== undefined,
  isPending: false,
  isPlaceholderData: false,
  isFetchedAfterMount: false,
  socketStatus: undefined,
  setRefetchEnabled: () => undefined,
} as unknown as TxQuery);

describe('TxDetailsDegraded', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({}), { headers: { 'Content-Type': 'application/json' } });
  });

  it('renders the overview card from the node data', () => {
    const { container } = renderTransaction(<TxDetailsDegraded hash={ base.hash } txQuery={ buildTxQuery(500) }/>, base.hash);

    expect(container.querySelector('[data-tx-info]')).not.toBeNull();
  });

  it('puts the sync warning in the band above the overview card', () => {
    const { container } = renderTransaction(<TxDetailsDegraded hash={ base.hash } txQuery={ buildTxQuery(500) }/>, base.hash);

    const band = container.querySelector('[data-tx-info]')?.previousElementSibling;

    expect(band?.textContent).toContain('Data sync in progress');
  });

  it('keeps the testnet notice in the same band', () => {
    const { container } = renderTransaction(<TxDetailsDegraded hash={ base.hash } txQuery={ buildTxQuery(500) }/>, base.hash);

    const band = container.querySelector('[data-tx-info]')?.previousElementSibling;

    expect(band?.textContent).toContain('This is a testnet transaction only');
  });

  it('drops the sync warning when the indexer simply has no such transaction', () => {
    const { container } = renderTransaction(<TxDetailsDegraded hash={ base.hash } txQuery={ buildTxQuery(404) }/>, base.hash);

    const band = container.querySelector('[data-tx-info]')?.previousElementSibling;

    expect(band?.textContent).not.toContain('Data sync in progress');
    expect(container.querySelector('[data-tx-info]')).not.toBeNull();
  });
});

test(degradedNodeReceiptReachesOverview, degradedNodeReceiptReachesOverview, 10000);
test(degradedRevertedReceiptReachesOverview, degradedRevertedReceiptReachesOverview, 10000);
test(degradedReceiptErrorRecoversToIndexedTransaction, degradedReceiptErrorRecoversToIndexedTransaction, 25000);

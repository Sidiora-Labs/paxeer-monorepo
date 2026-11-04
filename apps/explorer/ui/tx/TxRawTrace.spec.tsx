// @vitest-environment jsdom

import React from 'react';

import * as txMock from 'mocks/txs/tx';
import { beforeEach, describe, expect, it, test } from 'vitest';

import {
  TxRawTrace, renderTransaction, rawTraceOnDemandCompletionReachesPanel, rawTraceEmptyCompletionReachesPanel,
} from './repair-22-33-519.fixtures';
import type { TxQuery } from './useTxQuery';

const buildTxQuery = (data: typeof txMock.base | undefined, overrides: Partial<TxQuery> = {}): TxQuery => ({
  data,
  isError: false,
  isPending: false,
  isPlaceholderData: false,
  socketStatus: undefined,
  setRefetchEnabled: () => undefined,
  ...overrides,
} as unknown as TxQuery);

describe('TxRawTrace', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify([]), { headers: { 'Content-Type': 'application/json' } });
  });

  it('heads the shared table card with the raw trace title', () => {
    const { container } = renderTransaction(<TxRawTrace txQuery={ buildTxQuery(txMock.base) }/>, txMock.base.hash);

    expect(container.querySelector('[data-scan-table-card] [data-title]')?.textContent).toBe('Raw trace');
  });

  it('renders the trace snippet inside the card body', () => {
    const { container } = renderTransaction(<TxRawTrace txQuery={ buildTxQuery(txMock.base) }/>, txMock.base.hash);

    const body = container.querySelector('[data-scan-table-card] [data-body]');

    expect(body?.querySelector('section')).not.toBeNull();
    expect(body?.textContent).toContain('[]');
  });

  it('waits for a pending transaction instead of asking for its trace', () => {
    const { container } = renderTransaction(<TxRawTrace txQuery={ buildTxQuery(txMock.pending) }/>, txMock.base.hash);

    expect(container.textContent).toContain('This transaction is pending confirmation.');
    expect(container.querySelector('[data-scan-table-card]')).toBeNull();
  });

  it('reports a failed transaction request instead of the card', () => {
    const { container } = renderTransaction(<TxRawTrace txQuery={ buildTxQuery(undefined, { isError: true }) }/>, txMock.base.hash);

    expect(container.textContent).toContain('Something went wrong');
    expect(container.querySelector('[data-scan-table-card]')).toBeNull();
  });
});

test(rawTraceOnDemandCompletionReachesPanel, rawTraceOnDemandCompletionReachesPanel, 10000);
test(rawTraceEmptyCompletionReachesPanel, rawTraceEmptyCompletionReachesPanel);

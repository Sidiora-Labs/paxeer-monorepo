// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXActivityItem } from 'types/api/paxeerX';

import * as paxeerXMock from 'mocks/paxeerX/unifiedAccount';
import { describe, expect, it, test } from 'vitest';
import { screen } from 'vitest/lib';

import ActivityList from './ActivityList';
import { UNIFIED_ACCOUNT_PLACEHOLDER } from './placeholderData';
import { render } from './testWrapper';

const pagination = {
  page: paxeerXMock.unifiedAccount.page_number,
  total: paxeerXMock.unifiedAccount.activity_total,
  hasNextPage: Boolean(paxeerXMock.unifiedAccount.next_page_params),
  canGoBackwards: false,
  onNextPageClick: () => window.history.forward(),
  onPrevPageClick: () => window.history.back(),
  resetPage: () => window.history.go(0),
};

function activityTestRetiredCopy() {
  const { container } = render(<ActivityList { ...pagination } items={ paxeerXMock.unifiedAccount.activity }/>);

  const card = container.querySelector('[data-scan-table-card]');

  expect(card?.querySelector('[data-title]')?.textContent).toBe('A total of 2 activity entries found');
  expect(card?.querySelector('[data-note]')?.textContent).toBe('Native network activity, newest first');
}

describe('ActivityList', () => {
  it('shows an empty message when there is no activity', () => {
    const account = paxeerXMock.unifiedAccountEvmOnly;

    const { container } = render(<ActivityList { ...pagination } items={ account.activity } total={ account.activity_total }/>);

    expect(screen.getByRole('status').textContent).toBe('There is no Paxeer X activity for this account yet.');
    expect(container.querySelectorAll('[data-activity]')).toHaveLength(0);
    expect(container.querySelector('[data-title]')?.textContent).toBe('A total of 0 activity entries found');
  });

  it('renders one row per activity entry', () => {
    const { container } = render(<ActivityList { ...pagination } items={ paxeerXMock.unifiedAccount.activity }/>);

    expect(container.querySelectorAll('[data-activity]')).toHaveLength(2);

    const kinds = Array.from(container.querySelectorAll('[data-activity] td:first-child')).map((cell) => cell.textContent);

    expect(kinds).toEqual([ 'Transaction', 'Transaction' ]);
  });

  it('humanizes a kind outside the known vocabulary', () => {
    const item = { ...paxeerXMock.unifiedAccount.activity[0], kind: 'replay_receipt' };

    render(<ActivityList { ...pagination } items={ [ item ] } total={ 1 }/>);

    expect(screen.getByText('Replay receipt')).toBeTruthy();
  });

  it('puts every row on the status ladder', () => {
    const { container, rerender } = render(<ActivityList { ...pagination } items={ paxeerXMock.unifiedAccount.activity }/>);

    expect(container.querySelectorAll('[data-rung]')).toHaveLength(2);
    expect(container.querySelectorAll('[data-rung="committed"]')).toHaveLength(2);
    expect(screen.getAllByText('Committed')).toHaveLength(2);

    rerender(<ActivityList
      { ...pagination }
      items={ [ { ...paxeerXMock.unifiedAccount.activity[0], status: 'receipted' } ] }
      total={ 1 }
    />);

    expect(container.querySelectorAll('[data-rung]')).toHaveLength(1);
    expect(container.querySelector('[data-rung="receipted"]')?.textContent).toBe('Receipted');

    rerender(<ActivityList
      { ...pagination }
      items={ [ { ...paxeerXMock.unifiedAccount.activity[0], status: 'submitted' } ] }
      total={ 1 }
    />);

    expect(container.querySelectorAll('[data-rung]')).toHaveLength(1);
    expect(container.querySelector('[data-rung="submitted"]')?.textContent).toBe('Submitted');
  });

  it('scales amounts by the asset decimals without network-side labels', () => {
    const tokenTransfer: PaxeerXActivityItem = {
      ...paxeerXMock.unifiedAccount.activity[0],
      kind: 'token_transfer',
      asset: paxeerXMock.tokenAsset,
      amount: '2500000',
    };
    const items = [ ...paxeerXMock.unifiedAccount.activity, tokenTransfer ];

    render(<ActivityList { ...pagination } items={ items } total={ 3 }/>);

    expect(screen.getByText('0 HPX')).toBeTruthy();
    expect(screen.getByText('1.5 HPX')).toBeTruthy();
    expect(screen.getByText('2.5 USDX')).toBeTruthy();
    expect(screen.queryByText('Chain')).toBeNull();
    expect(screen.queryByText('Kernel')).toBeNull();
  });

  it('shows an amount unscaled and labelled by its denom when metadata is absent', () => {
    const item = {
      ...paxeerXMock.unifiedAccount.activity[0],
      asset: { ...paxeerXMock.tokenAsset, symbol: null, decimals: null },
      amount: '1200',
    };

    render(<ActivityList { ...pagination } items={ [ item ] } total={ 1 }/>);

    expect(screen.getByText(`1,200 ${ paxeerXMock.tokenAsset.denom }`)).toBeTruthy();
  });
  test(activityTestRetiredCopy.name, activityTestRetiredCopy);

  it('names the action of every row on a method chip', () => {
    const { container } = render(<ActivityList { ...pagination } items={ paxeerXMock.unifiedAccount.activity }/>);

    const methods = Array.from(container.querySelectorAll('[data-activity] [data-scan-method]'))
      .map((chip) => chip.getAttribute('data-scan-method'));

    expect(methods).toEqual([ 'Transaction', 'Transaction' ]);
  });

  it('keeps a token transfer kind on its method chip', () => {
    const tokenTransfer: PaxeerXActivityItem = {
      ...paxeerXMock.unifiedAccount.activity[0],
      kind: 'token_transfer',
      asset: paxeerXMock.tokenAsset,
      amount: '2500000',
    };
    const { container } = render(<ActivityList { ...pagination } items={ [ tokenTransfer ] } total={ 1 }/>);
    const row = container.querySelector('[data-activity]');

    expect(row?.getAttribute('data-kind')).toBe('token_transfer');
    expect(row?.querySelector('td:first-child')?.textContent).toBe('Token transfer');
    expect(row?.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe('Token transfer');
    expect(screen.getByText('2.5 USDX')).toBeTruthy();
  });

  it('heads the card with a loading title while the page is on its way', () => {
    const { container, rerender } = render(<ActivityList
      { ...pagination }
      items={ UNIFIED_ACCOUNT_PLACEHOLDER.activity }
      total={ UNIFIED_ACCOUNT_PLACEHOLDER.activity_total }
      isLoading
    />);

    expect(container.querySelector('[data-scan-table-card] [data-title]')?.textContent).toBe('Loading activity entries…');
    const rows = container.querySelectorAll('[data-activity]');

    expect(rows).toHaveLength(UNIFIED_ACCOUNT_PLACEHOLDER.activity.length);
    rows.forEach((row) => expect(row.querySelector('[data-loading]')).toBeTruthy());
    expect(screen.queryByText('There is no Paxeer X activity for this account yet.')).toBeNull();

    rerender(<ActivityList { ...pagination } items={ paxeerXMock.unifiedAccount.activity }/>);

    expect(container.querySelectorAll('[data-activity]')).toHaveLength(paxeerXMock.unifiedAccount.activity.length);
    expect(container.querySelectorAll('[data-activity] [data-loading]')).toHaveLength(0);
    expect(container.querySelector('[data-title]')?.textContent).toBe('A total of 2 activity entries found');
  });

  it('counts the entries of the page when the node reports no total', () => {
    const { container } = render(<ActivityList { ...pagination } items={ paxeerXMock.unifiedAccount.activity } total={ null }/>);

    expect(container.querySelector('[data-scan-table-card] [data-title]')?.textContent).toBe('2 activity entries on this page');
  });

});

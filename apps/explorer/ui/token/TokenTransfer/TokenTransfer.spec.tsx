// @vitest-environment jsdom

import type { UseQueryResult } from '@tanstack/react-query';
import { act, fireEvent, within, waitFor } from '@testing-library/react';
import React from 'react';

import type { TokenInfo } from 'types/api/token';
import type { PaginationParams } from 'ui/shared/pagination/types';

import type { ResourceError } from 'lib/api/resources';
import { SocketProvider, useSocket } from 'lib/socket/context';
import { tokenInfoERC20a } from 'mocks/tokens/tokenInfo';
import * as tokenTransferMock from 'mocks/tokens/tokenTransfer';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import type { QueryWithPagesResult } from 'ui/shared/pagination/useQueryWithPages';
import { beforeEach, describe, expect, it, test, vi } from 'vitest';
import { createTestSocket } from 'vitest/utils/socketServer';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import TokenTransfer from './TokenTransfer';

Object.defineProperty(document, 'fonts', {
  configurable: true,
  value: { load: () => Promise.resolve([ {} ]) },
});

const HASH = tokenInfoERC20a.address_hash;

const pagination = (overrides?: Partial<PaginationParams>): PaginationParams => ({
  // the socket channel only listens on the first page, and jsdom carries no socket, so the spec pages forward
  page: 2,
  onNextPageClick: vi.fn(),
  onPrevPageClick: vi.fn(),
  resetPage: vi.fn(),
  hasPages: true,
  hasNextPage: true,
  canGoBackwards: true,
  isLoading: false,
  isVisible: true,
  ...overrides,
});

const items = [ tokenTransferMock.erc20, tokenTransferMock.erc20, tokenTransferMock.erc20 ];

const transfersQuery = (overrides?: Partial<QueryWithPagesResult<'general:token_transfers'>>) => ({
  data: { items, next_page_params: null },
  isError: false,
  isPlaceholderData: false,
  pagination: pagination(),
  ...overrides,
}) as unknown as QueryWithPagesResult<'general:token_transfers'>;

const tokenQuery = (data: TokenInfo = tokenInfoERC20a) => ({
  data,
  isError: false,
  isPlaceholderData: false,
}) as unknown as UseQueryResult<TokenInfo, ResourceError<unknown>>;

async function tokenTransferLiveUpdatesFollowDisplayPage() {
  routerState.pathname = '/token/[hash]';
  routerState.query = { hash: HASH };
  fetchMock.resetMocks();
  fetchMock.mockResponse(JSON.stringify({ items: [], next_page_params: null }), { headers: { 'Content-Type': 'application/json' } });
  const stepOptions = (name: string, timeout = 10_000) => ({
    timeout,
    onTimeout: (error: Error) => new Error(`${ name }: ${ error.message }`),
  });
  const step = async<T,>(name: string, action: () => PromiseLike<T>, timeout: number) => {
    let settled = false;
    let value!: T;
    let failure: unknown;
    let failed = false;
    void Promise.resolve().then(action).then((result) => {
      value = result;
      settled = true;
    }, (error: unknown) => {
      failure = error;
      failed = true;
      settled = true;
    });
    await waitFor(() => expect(settled).toBe(true), stepOptions(name, timeout));
    if (failed) {
      throw new Error(`${ name }: ${ failure instanceof Error ? failure.message : String(failure) }`);
    }
    return value;
  };
  const socket = await step('test socket ready', createTestSocket, 2_000);
  const topic = `tokens:${ HASH.toLowerCase() }`;
  let context: ReturnType<typeof useSocket>;

  function SocketObserver() {
    context = useSocket();
    return null;
  }

  const serverPagination = pagination({ page: 1, hasNextPage: false, canGoBackwards: false });
  const serverItems = Array.from({ length: 50 }, () => tokenTransferMock.erc20);
  const tree = (state: { isPlaceholderData?: boolean; isError?: boolean } = {}) => (
    <SocketProvider url={ socket.url }>
      <SocketObserver/>
      <TokenTransfer
        transfersQuery={ transfersQuery({
          data: { items: serverItems, next_page_params: null },
          pagination: serverPagination,
          ...state,
        }) }
        tokenQuery={ tokenQuery() }
      />
    </SocketProvider>
  );
  const scrollToDescriptor = Object.getOwnPropertyDescriptor(Element.prototype, 'scrollTo');
  Object.defineProperty(Element.prototype, 'scrollTo', { configurable: true, value: vi.fn() });
  const view = render(tree({ isPlaceholderData: true }));
  const notices = (text: string) => {
    const matches = within(view.container).getAllByText(text);
    expect(matches).toHaveLength(2);
    expect(matches.filter((notice) => notice.closest('[data-token-transfer-table]'))).toHaveLength(1);
    expect(matches.filter((notice) => !notice.closest('[data-token-transfer-table]'))).toHaveLength(1);
  };
  const joined = async(name: string) => {
    await waitFor(() => {
      expect(context?.channelRegistry.current[topic]?.subscribers).toBe(1);
      expect(context?.channelRegistry.current[topic]?.channel.state).toBe('joined');
    }, stepOptions(name, 2_000));
    return context!.channelRegistry.current[topic].channel;
  };
  const disabled = () => {
    expect(context?.channelRegistry.current[topic]).toBeUndefined();
  };
  const send = async(count: number, name: string, timeout = 10_000) => {
    await step(name, () => act(async() => {
      socket.send(topic, 'token_transfer', { token_transfer: count });
      await new Promise((resolve) => setTimeout(resolve, 350));
    }), timeout);
  };
  const selectRows = async(rows: number) => {
    const started = performance.now();
    const name = `select ${ rows } rows`;
    const select = view.container.querySelector('[data-scan-show-rows] select');
    expect(select).not.toBeNull();
    try {
      fireEvent.change(select!, { target: { value: String(rows) } });
      await waitFor(() => {
        expect(view.container.querySelector('[data-scan-show-rows] [data-value]')?.getAttribute('data-value')).toBe(String(rows));
      }, stepOptions(name));
    } finally {
      process.stdout.write(`${ name } duration: ${ Math.round(performance.now() - started) } ms\n`);
    }
  };
  const page = (value: number) => {
    expect(view.container.querySelector('[data-control="page"]')?.textContent).toBe(`Page ${ value }`);
    expect(serverPagination.page).toBe(1);
    expect(serverPagination.onNextPageClick).not.toHaveBeenCalled();
    expect(serverPagination.onPrevPageClick).not.toHaveBeenCalled();
    expect(serverPagination.resetPage).not.toHaveBeenCalled();
  };
  const paginationControls = () => within(view.container.querySelector<HTMLElement>('[data-footer-pagination]')!);
  const next = async() => {
    fireEvent.click(paginationControls().getByRole('button', { name: 'Next page' }));
    await waitFor(() => {
      page(2);
      disabled();
      expect(within(view.container).queryAllByText(/scanning new token transfers|more token transfers?/)).toHaveLength(0);
    }, stepOptions('next display page'));
  };

  try {
    await waitFor(() => expect(context?.socket).toBeTruthy(), stepOptions('context socket', 2_000));
    disabled();
    view.rerender(tree());
    const firstChannel = await joined('first display page subscription');
    notices('scanning new token transfers...');
    await selectRows(25);
    page(1);
    expect(view.container.querySelectorAll('[data-token-transfer-table] tbody tr')).toHaveLength(26);
    await send(2, 'send 2 transfers on first display page');
    await waitFor(() => notices('2 more token transfers'), stepOptions('both notices count 2'));

    await next();
    expect(firstChannel.state).not.toBe('joined');
    expect(view.container.querySelectorAll('[data-token-transfer-table] tbody tr')).toHaveLength(25);
    await send(7, 'send 7 transfers on second display page', 2_000);
    disabled();
    fireEvent.click(paginationControls().getByRole('button', { name: 'Previous page' }));
    await waitFor(() => page(1), stepOptions('previous page return'));
    const returnedChannel = await joined('previous page return subscription');
    expect(returnedChannel).not.toBe(firstChannel);
    notices('2 more token transfers');
    await send(3, 'send 3 transfers after previous page return');
    await waitFor(() => notices('5 more token transfers'), stepOptions('both notices count 5'));

    await next();
    await send(7, 'send 7 transfers before row-selection reset', 2_000);
    await selectRows(50);
    await waitFor(() => page(1), stepOptions('row-selection reset page return'));
    expect(await joined('row-selection reset subscription')).not.toBe(returnedChannel);
    notices('5 more token transfers');
    expect(view.container.querySelectorAll('[data-token-transfer-table] tbody tr')).toHaveLength(51);
    await send(1, 'send 1 transfer after row-selection reset');
    await waitFor(() => notices('6 more token transfers'), stepOptions('both notices count 6'));

    view.rerender(tree({ isPlaceholderData: true }));
    disabled();
    await send(7, 'send 7 transfers during placeholder state', 2_000);
    view.rerender(tree());
    await joined('placeholder recovery subscription');
    notices('6 more token transfers');
    await send(1, 'send 1 transfer after placeholder recovery');
    await waitFor(() => notices('7 more token transfers'), stepOptions('both notices count 7'));

    view.rerender(tree({ isError: true }));
    disabled();
    await send(7, 'send 7 transfers during error state', 2_000);
    view.rerender(tree());
    await joined('error recovery subscription');
    notices('7 more token transfers');
    await send(1, 'send 1 transfer after error recovery');
    await waitFor(() => notices('8 more token transfers'), stepOptions('both notices count 8'));
  } finally {
    view.unmount();
    if (scrollToDescriptor) {
      Object.defineProperty(Element.prototype, 'scrollTo', scrollToDescriptor);
    } else {
      Reflect.deleteProperty(Element.prototype, 'scrollTo');
    }
    disabled();
    void socket.close();
  }
}

test(tokenTransferLiveUpdatesFollowDisplayPage.name, { timeout: 120_000 }, tokenTransferLiveUpdatesFollowDisplayPage);

describe('TokenTransfer', () => {
  beforeEach(() => {
    routerState.pathname = '/token/[hash]';
    routerState.query = { hash: HASH };
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify({ items: [], next_page_params: null }), { headers: { 'Content-Type': 'application/json' } });
  });

  it('puts the transfers in the shared table card', () => {
    const { container } = render(<TokenTransfer transfersQuery={ transfersQuery() } tokenQuery={ tokenQuery() }/>);

    expect(container.querySelector('[data-scan-table-card] [data-token-transfer-table]')).not.toBeNull();
  });

  it('counts the transfers the token reports in the card header', () => {
    const { container } = render(
      <TokenTransfer transfersQuery={ transfersQuery() } tokenQuery={ tokenQuery() } transfersCount={ 42 }/>,
    );

    expect(container.querySelector('[data-title]')?.textContent).toBe('Latest 3 from a total of 42 token transfers');
  });

  it('counts the current page when the token reports no total and a page follows', () => {
    const { container } = render(<TokenTransfer transfersQuery={ transfersQuery() } tokenQuery={ tokenQuery() }/>);

    expect(container.querySelector('[data-title]')?.textContent).toBe('3 token transfers on this page');
  });

  it('counts the rows it holds when there is no further page', () => {
    const { container } = render(
      <TokenTransfer
        transfersQuery={ transfersQuery({ pagination: pagination({ hasNextPage: false }) }) }
        tokenQuery={ tokenQuery() }
      />,
    );

    expect(container.querySelector('[data-title]')?.textContent).toBe('3 token transfers on this page');
  });

  it('offers the show-records selector and the pagination in the card footer', () => {
    const { container } = render(<TokenTransfer transfersQuery={ transfersQuery() } tokenQuery={ tokenQuery() }/>);

    const footer = container.querySelector('[data-footer]') as HTMLElement;
    const showRows = footer.querySelector('[data-scan-show-rows]') as HTMLElement;

    expect(showRows.textContent).toContain('Show');
    expect(showRows.textContent).toContain('Records');
    expect(footer.querySelector('[data-footer-pagination] [data-scan-pagination]')).not.toBeNull();
  });

  it('renders nothing before the page asks for it', () => {
    const { container } = render(
      <TokenTransfer transfersQuery={ transfersQuery() } tokenQuery={ tokenQuery() } shouldRender={ false }/>,
    );

    expect(container.querySelector('[data-scan-table-card]')).toBeNull();
    expect(container.querySelector('[data-token-transfer-table]')).toBeNull();
  });

  it('says so when the token has no transfer', () => {
    const { container } = render(
      <TokenTransfer
        transfersQuery={ transfersQuery({ data: { items: [], next_page_params: null } }) }
        tokenQuery={ tokenQuery() }
      />,
    );

    expect(container.querySelector('[data-body]')?.textContent).toContain('There are no token transfers.');
  });
});

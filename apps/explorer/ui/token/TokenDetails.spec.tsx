// @vitest-environment jsdom

import type { UseQueryResult } from '@tanstack/react-query';
import React from 'react';

import type { TokenInfo } from 'types/api/token';

import type { ResourceError } from 'lib/api/resources';
import { tokenCounters, tokenInfoERC20b, tokenInfoERC721a } from 'mocks/tokens/tokenInfo';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { waitFor } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import TokenDetails from './TokenDetails';
import TokenMarkets from './TokenMarkets';

Object.defineProperty(document, 'fonts', {
  configurable: true,
  value: { load: () => Promise.resolve([ {} ]) },
});

const HASH = tokenInfoERC20b.address_hash;

const query = (data: TokenInfo) => ({
  data,
  isPlaceholderData: false,
  isPending: false,
  isError: false,
}) as unknown as UseQueryResult<TokenInfo, ResourceError<unknown>>;

const cardTitles = (container: HTMLElement) =>
  Array.from(container.querySelectorAll('[data-token-card]')).map((card) => card.getAttribute('data-token-card'));

const cardRows = (container: HTMLElement, title: string) => {
  const card = container.querySelector(`[data-token-card="${ title }"]`) as HTMLElement;

  return Array.from(card.querySelectorAll('[data-scan-key]')).map((key) => key.textContent ?? '');
};

describe('TokenDetails', () => {
  beforeEach(() => {
    routerState.pathname = '/token/[hash]';
    routerState.query = { hash: HASH };
    fetchMock.resetMocks();
    fetchMock.mockResponse(JSON.stringify(tokenCounters), { headers: { 'Content-Type': 'application/json' } });
  });

  it('groups metadata into TOKEN INFO and CONTRACT INFO', () => {
    const { container } = render(<TokenDetails tokenQuery={ query(tokenInfoERC20b) }/>);

    expect(cardTitles(container)).toEqual([ 'TOKEN INFO', 'CONTRACT INFO' ]);
  });

  it('carries the supply, the holders and the transfers in the overview card', () => {
    const { container } = render(<TokenDetails tokenQuery={ query(tokenInfoERC20b) }/>);

    expect(cardRows(container, 'TOKEN INFO')).toEqual([ 'Max total supply', 'Holders', 'Transfers' ]);
  });

  it('links the holders and the transfers to their counts once the counters answered', async() => {
    const { container } = render(<TokenDetails tokenQuery={ query(tokenInfoERC20b) }/>);

    await waitFor(() => {
      const overview = container.querySelector('[data-token-card="TOKEN INFO"]') as HTMLElement;

      expect(overview.textContent).toContain('8,838,883');
      expect(overview.textContent).toContain('88,282,281');
    });
  });

  it('shows API market data in tiles without deriving an onchain market cap', () => {
    const { container } = render(<TokenMarkets tokenQuery={ query(tokenInfoERC20b) }/>);
    expect(Array.from(container.querySelectorAll('[data-stat-tile]')).map((tile) => tile.getAttribute('data-stat-tile')))
      .toEqual([ 'Price', 'Market cap', 'Volume 24h', 'Holders' ]);
    expect(container.querySelector('[data-stat-tile="Price"] [data-stat-value]')?.textContent).toBe('$0.982');
    expect(container.querySelector('[data-stat-tile="Market cap"] [data-stat-value]')?.textContent).toBe('$115,060,192.36105014');
    expect(container.querySelector('[data-stat-tile="Volume 24h"] [data-stat-value]')?.textContent).toBe('—');
    expect(container.querySelector('[data-onchain-market-cap]')).toBeNull();

  });

  it('names the token contract with its decimals in the other-info card', () => {
    const { container } = render(<TokenDetails tokenQuery={ query(tokenInfoERC20b) }/>);

    expect(cardRows(container, 'CONTRACT INFO')[0]).toBe('Token contract (with 6 decimals)');
    expect(container.querySelector('[data-token-contract] [data-entity-kind="address"]')).not.toBeNull();
  });

  it('keeps all market tiles with dashes when their API fields are absent', () => {
    const { container } = render(<TokenMarkets tokenQuery={ query(tokenInfoERC721a) }/>);
    expect(container.querySelectorAll('[data-stat-tile]')).toHaveLength(4);
    expect(container.querySelector('[data-stat-tile="Price"] [data-stat-value]')?.textContent).toBe('—');
    expect(container.querySelector('[data-stat-tile="Market cap"] [data-stat-value]')?.textContent).toBe('—');
    expect(container.querySelector('[data-stat-tile="Volume 24h"] [data-stat-value]')?.textContent).toBe('—');

  });

  it('keeps the contract row without a decimals note when the token has no decimals', () => {
    const { container } = render(<TokenDetails tokenQuery={ query(tokenInfoERC721a) }/>);

    expect(cardRows(container, 'CONTRACT INFO')[0]).toBe('Token contract');
  });
});

// @vitest-environment jsdom

import React from 'react';

import config from 'configs/app';
import { MultichainContext } from 'lib/contexts/multichain';
import * as statsMock from 'mocks/stats/index';
import * as txsStatsMock from 'mocks/txs/stats';
import { TXS_STATS_MICROSERVICE } from 'stubs/tx';
import { thinsp } from 'toolkit/utils/htmlEntities';
import { render } from 'ui/shared/layout/testWrapper';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { waitFor } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_STATS_API_HOST: '',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

// The host runs the whole suite at once, and these trees mount real entities, so the first render of
// each of them reaches well past the default per-test budget.
vi.setConfig({ testTimeout: 60_000 });

import TxsStats from './TxsStats';

const json = (payload: unknown) => JSON.stringify(payload);

// A responsive style prop reaches the document as one rule per breakpoint, each of them inserted
// into a style element of its own, and jsdom performs no layout, so the widths a declaration belongs
// to are read from the rules that name the element's own class.
const ruleTexts = () => {
  const inline = Array.from(document.querySelectorAll('style')).map((node) => node.textContent ?? '');
  const parsed = Array.from(document.styleSheets).flatMap((sheet) => {
    try {
      return Array.from(sheet.cssRules).map((rule) => rule.cssText);
    } catch {
      return [];
    }
  });

  return [ ...inline, ...parsed ].filter(Boolean);
};

const declarationsOf = (element: Element, property: string) => ruleTexts()
  .filter((text) => Array.from(element.classList).some((name) => new RegExp(`\\.${ name }(?![\\w-])`).test(text)))
  .map((text) => ({ text, value: new RegExp(`(?:^|[;{\\s])${ property }\\s*:\\s*([^;}]+)`).exec(text)?.[1]?.trim() }))
  .filter((rule): rule is { text: string; value: string } => rule.value !== undefined);

describe('TxsStats', () => {
  let feeSum = txsStatsMock.base.transaction_fees_sum_24h;
  let statsFeeSum = '22.184012506492688277';

  beforeEach(() => {
    feeSum = txsStatsMock.base.transaction_fees_sum_24h;
    statsFeeSum = '22.184012506492688277';
    fetchMock.resetMocks();
    fetchMock.mockResponse((request) => {
      if (request.url.includes('/api/v2/transactions/stats')) {
        return {
          body: json({ ...txsStatsMock.base, transaction_fees_sum_24h: feeSum }),
          headers: { 'Content-Type': 'application/json' },
        };
      }

      if (request.url.includes('/api/v1/pages/transactions')) {
        return {
          body: json({
            ...TXS_STATS_MICROSERVICE,
            transactions_fee_24h: {
              ...TXS_STATS_MICROSERVICE.transactions_fee_24h,
              title: 'Total transaction fee',
              value: statsFeeSum,
            },
          }),
          headers: { 'Content-Type': 'application/json' },
        };
      }

      if (request.url.includes('/api/v2/stats')) {
        return { body: json(statsMock.base), headers: { 'Content-Type': 'application/json' } };
      }

      return { body: json({}), headers: { 'Content-Type': 'application/json' } };
    });
  });

  it('opens the page with the seven stat cards in the scan order', async() => {
    const { container } = render(<TxsStats/>);

    await waitFor(() => {
      expect(Array.from(container.querySelectorAll('[data-scan-stat]')).map((card) => card.getAttribute('data-scan-stat')))
        .toEqual([
          'Transactions (24h)',
          'Total transactions',
          'Price updates (24h)',
          'Total price updates',
          'Pending transactions (last 1h)',
          'Total transaction fee (24h)',
          'Avg. transaction fee (24h)',
        ]);
    });
  });

  it('reads the counts and the fees the transaction statistics carry', async() => {
    const { container } = render(<TxsStats/>);

    await waitFor(() => {
      const values = Array.from(container.querySelectorAll('[data-scan-stat]'))
        .map((card) => card.querySelector('[data-value]')?.textContent);

      expect(values[0]).toBe('992,890');
      expect(values[1]).toBe('100,000,000');
      expect(values[2]).toBe('250,000');
      expect(values[3]).toBe('20,000,000');
      expect(values[4]).toBe('4,200');
      expect(values[5]).toContain('22.18');
      expect(values[5]).toContain('ETH');
      expect(values[6]?.startsWith('$')).toBe(true);
    });
  });

  const feeCases = [
    { baseUnits: '392934000000000', coins: '0.000392934', expected: '0.00039', decimals: 18 },
    { baseUnits: '0', coins: '0', expected: '0', decimals: 18 },
    { baseUnits: '1500000000000000000', coins: '1.5', expected: '1.5', decimals: 18 },
    { baseUnits: '1', coins: '0.000000000000000001', expected: '0.000000000000000001', decimals: 18 },
    { baseUnits: '10000000000000000', coins: '0.01', expected: '0.01', decimals: 18 },
    { baseUnits: '100000000000000', coins: '0.0001', expected: '0.0001', decimals: 18 },
    { baseUnits: '9999000000000000', coins: '0.009999', expected: '0.01', decimals: 18 },
    { baseUnits: '1', coins: '0.000001', expected: '0.000001', decimals: 6 },
  ];

  it.each([ 'general', 'stats' ].flatMap((source) => feeCases.map((fee) => ({ ...fee, source }))))(
    'shows $coins coins from $source with the required fraction digits',
    async function showsFeeSum({ baseUnits, coins, expected, decimals, source }) {
      feeSum = baseUnits;
      statsFeeSum = coins;

      const chainConfig = {
        ...config,
        chain: { ...config.chain, currency: { ...config.chain.currency, decimals } },
        apis: { ...config.apis, stats: source === 'stats' ? { endpoint: window.location.origin } : undefined },
      };
      const { container } = render(
        <MultichainContext.Provider value={{ chain: { id: '1', name: 'Ethereum', slug: 'ethereum', app_config: chainConfig } }}>
          <TxsStats/>
        </MultichainContext.Provider>,
      );

      await waitFor(() => {
        const card = container.querySelector('[data-scan-stat="Total transaction fee (24h)"]');

        expect(card?.querySelector('[data-value]')?.textContent).toBe(`${ expected }${ thinsp }ETH`);
        expect(card?.querySelector('[data-delta]')?.textContent).toBe('(7.42%)');
      });
    },
  );

  it('carries deltas derived from the statistics it reads', async() => {
    const { container } = render(<TxsStats/>);

    await waitFor(() => {
      const deltas = Array.from(container.querySelectorAll('[data-scan-stat]'))
        .map((card) => {
          const delta = card.querySelector('[data-delta]');

          return delta ? [ delta.getAttribute('data-delta'), delta.textContent ] : null;
        });

      expect(deltas).toEqual([
        [ 'up', '(0.99%)' ],
        null,
        null,
        null,
        [ 'down', '(0.42%)' ],
        [ 'down', '(7.42%)' ],
        [ 'down', '(7.42%)' ],
      ]);
    });
  });

  it('lets every stat card shrink inside its own track', async() => {
    const { container } = render(<TxsStats/>);

    await waitFor(() => {
      expect(container.querySelectorAll('[data-scan-stat]').length).toBeGreaterThan(0);
    });

    const row = container.querySelector('[data-scan-stat-row]') as HTMLElement;
    const tracks = declarationsOf(row, 'grid-template-columns');

    expect(tracks.length).toBeGreaterThan(0);
    expect(tracks.every((rule) => rule.value.includes('repeat(auto-fit,'))).toBe(true);
    expect(tracks.every((rule) => /minmax\(min\(100%,\s*var\(--chakra-spacing-62\\\.5\)\),\s*1fr\)/.test(rule.value))).toBe(true);
  });
});

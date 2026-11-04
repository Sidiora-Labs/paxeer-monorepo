// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { CrossChainTx } from '@blockscout/zetachain-cctx-types';

import config from 'configs/app';
import { zetaChainCCTX, zetaChainCCTXFailed } from 'mocks/zetaChain/zetaChainCCTX';
import { zetaChainCCTXConfig } from 'mocks/zetaChain/zetaChainCCTXConfig';
import theme from 'toolkit/theme/theme';
import { expect, test } from 'vitest';

import ZetaChainCCTXDetailsLifecycleOut from './ZetaChainCCTXDetailsLifecycleOut';

type MediaQueryListener = NonNullable<Parameters<MediaQueryList['addListener']>[0]>;

class ViewportMediaQueryList extends EventTarget implements MediaQueryList {
  readonly media: string;
  onchange: MediaQueryList['onchange'] = null;
  private readonly listeners = new Map<MediaQueryListener, EventListener>();
  private readonly boundary: number;
  private readonly comparison: string;
  private previousMatch: boolean;

  constructor(query: string) {
    super();
    const condition = /^\((min|max)-width:\s*([\d.]+)(px|em|rem)\)$/.exec(query);
    if (!condition) {
      throw new Error(`Unsupported viewport query: ${ query }`);
    }
    this.media = query;
    this.comparison = condition[1];
    this.boundary = Number(condition[2]) * (condition[3] === 'px' ? 1 : 16);
    this.previousMatch = this.matches;
    window.addEventListener('resize', this.handleResize);
  }

  get matches() {
    return this.comparison === 'min' ? window.innerWidth >= this.boundary : window.innerWidth <= this.boundary;
  }

  private handleResize = () => {
    const matches = this.matches;
    if (matches !== this.previousMatch) {
      this.previousMatch = matches;
      const event = Object.assign(new Event('change'), { matches, media: this.media });
      this.dispatchEvent(event);
      this.onchange?.call(this, event);
    }
  };

  addListener(callback: MediaQueryListener | null) {
    if (callback === null || this.listeners.has(callback)) {
      return;
    }
    const listener: EventListener = (event) => callback.call(this, event as MediaQueryListEvent);
    this.listeners.set(callback, listener);
    this.addEventListener('change', listener);
  }

  removeListener(callback: MediaQueryListener | null) {
    if (callback === null) {
      return;
    }
    const listener = this.listeners.get(callback);
    if (listener) {
      this.removeEventListener('change', listener);
      this.listeners.delete(callback);
    }
  }

  dispose() {
    window.removeEventListener('resize', this.handleResize);
    for (const callback of this.listeners.keys()) {
      this.removeListener(callback);
    }
    this.onchange = null;
  }
}

function installViewportMediaQueries() {
  const original = Object.getOwnPropertyDescriptor(window, 'matchMedia');
  const queries = new Set<ViewportMediaQueryList>();
  const matchMedia: Window['matchMedia'] = (query) => {
    const result = new ViewportMediaQueryList(query);
    queries.add(result);
    return result;
  };
  Object.defineProperty(window, 'matchMedia', { configurable: true, value: matchMedia });

  return () => {
    for (const query of queries) {
      query.dispose();
    }
    if (original) {
      Object.defineProperty(window, 'matchMedia', original);
    } else {
      Reflect.deleteProperty(window, 'matchMedia');
    }
  };
}

function renderOutbound(tx: CrossChainTx, outboundIndex: number, gasUsed: number) {
  const restoreMediaQueries = installViewportMediaQueries();
  const queryClient = new QueryClient();
  queryClient.setQueryData([ 'zetachain-config' ], zetaChainCCTXConfig);

  try {
    const markup = renderToStaticMarkup(React.createElement(ChakraProvider, {
      value: theme,
      children: React.createElement(
        QueryClientProvider,
        { client: queryClient },
        React.createElement(ZetaChainCCTXDetailsLifecycleOut, {
          tx,
          outboundParam: { ...tx.outbound_params[outboundIndex], gas_used: gasUsed },
          isLoading: false,
          isLast: true,
          hasTxAfter: false,
        }),
      ),
    }));
    return new DOMParser().parseFromString(markup, 'text/html');
  } finally {
    queryClient.clear();
    restoreMediaQueries();
  }
}

function outboundMinedGasUnits() {
  expect(config.chain.currency.decimals).toBe(18);
  expect(zetaChainCCTX.outbound_params[0].receiver_chain_id.toString()).not.toBe(config.chain.id);

  for (const [ gasUsed, expected ] of [
    [ 21000, '21,000' ],
    [ 0, '0' ],
    [ 9007199254740991, '9,007,199,254,740,991' ],
  ] as const) {
    const document = renderOutbound(zetaChainCCTX, 0, gasUsed);
    const labels = Array.from(document.querySelectorAll('p')).filter((node) => node.textContent === 'Gas used');

    expect(labels).toHaveLength(1);
    expect(labels[0].nextElementSibling?.textContent?.trim()).toBe(expected);
    expect(document.body.textContent).toContain('Sent tx to Sepolia Testnet');
    expect(document.querySelector('a[href*="/tx/"]')?.getAttribute('href'))
      .toBe(`https://sepolia.etherscan.io/tx/${ zetaChainCCTX.outbound_params[0].hash }`);
  }
}

function revertedGasUnits() {
  for (const [ gasUsed, expected ] of [
    [ 61955, '61,955' ],
    [ 0, '0' ],
    [ 9007199254740991, '9,007,199,254,740,991' ],
  ] as const) {
    const document = renderOutbound(zetaChainCCTXFailed, 1, gasUsed);
    const labels = Array.from(document.querySelectorAll('p')).filter((node) => node.textContent === 'Gas used');

    expect(labels).toHaveLength(1);
    expect(labels[0].nextElementSibling?.textContent?.trim()).toBe(expected);
    expect(document.body.textContent).toContain('Reverted to Sepolia Testnet');
  }
}

test('outboundMinedGasUnits', () => {
  expect.assertions(14);
  outboundMinedGasUnits();
});

test('revertedGasUnits', () => {
  expect.assertions(9);
  revertedGasUnits();
});

// @vitest-environment jsdom

import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { ClusterChainConfig } from 'types/multichain';

import { MultichainContext } from 'lib/contexts/multichain';
import { chainA, chainB } from 'mocks/multichain/chains';
import { erc20 } from 'mocks/tokens/tokenTransfer';
import { Provider } from 'toolkit/chakra/provider';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import TokenTransferListItem from 'ui/shared/TokenTransfer/TokenTransferListItem';
import TokenTransferTableItem from 'ui/shared/TokenTransfer/TokenTransferTableItem';
import { afterAll, beforeAll, expect, test } from 'vitest';

import AddressEntity from './AddressEntity';

const viewportSubscriptions = new Set<() => void>();
const originalMatchMedia = Object.getOwnPropertyDescriptor(window, 'matchMedia');
const originalInnerWidth = Object.getOwnPropertyDescriptor(window, 'innerWidth');

function matchesViewport(query: string): boolean {
  return query.split(',').some((branch) => branch.trim().split(/\s+and\s+/).every((condition) => {
    if (condition === 'screen' || condition === 'all') {
      return true;
    }

    const feature = /^\((min|max)-(width|height):\s*(\d+(?:\.\d+)?)(px|em|rem)\)$/.exec(condition);
    if (!feature) {
      throw new Error(`Unsupported viewport media query: ${ query }`);
    }

    const [ , bound, dimension, value, unit ] = feature;
    // Media-query font units use the browser's initial font size, independent of page styles.
    const threshold = Number(value) * (unit === 'px' ? 1 : 16);
    const size = dimension === 'width' ? window.innerWidth : window.innerHeight;
    return bound === 'min' ? size >= threshold : size <= threshold;
  }));
}

class ViewportMediaQueryEvent extends Event implements MediaQueryListEvent {
  constructor(readonly media: string, readonly matches: boolean) {
    super('change');
  }
}

function viewportMatchMedia(media: string): MediaQueryList {
  const events = new EventTarget();
  const query: MediaQueryList = {
    media,
    get matches() {
      return matchesViewport(media);
    },
    onchange: null,
    addEventListener(
      type: string,
      listener: EventListenerOrEventListenerObject | MediaQueryList['onchange'],
      options?: boolean | AddEventListenerOptions,
    ) {
      events.addEventListener(type, listener as EventListenerOrEventListenerObject | null, options);
    },
    removeEventListener(
      type: string,
      listener: EventListenerOrEventListenerObject | MediaQueryList['onchange'],
      options?: boolean | EventListenerOptions,
    ) {
      events.removeEventListener(type, listener as EventListenerOrEventListenerObject | null, options);
    },
    addListener(listener) {
      if (listener) query.addEventListener('change', listener);
    },
    removeListener(listener) {
      if (listener) query.removeEventListener('change', listener);
    },
    dispatchEvent(event) {
      if (event instanceof ViewportMediaQueryEvent) query.onchange?.call(query, event);
      return events.dispatchEvent(event);
    },
  };
  let previous = query.matches;
  viewportSubscriptions.add(() => {
    const matches = query.matches;
    if (previous !== matches) {
      previous = matches;
      query.dispatchEvent(new ViewportMediaQueryEvent(media, matches));
    }
  });
  return query;
}

function updateViewportQueries() {
  viewportSubscriptions.forEach((update) => update());
}

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', { configurable: true, value: viewportMatchMedia });
  window.addEventListener('resize', updateViewportQueries);
});

afterAll(() => {
  window.removeEventListener('resize', updateViewportQueries);
  viewportSubscriptions.clear();
  if (originalMatchMedia) {
    Object.defineProperty(window, 'matchMedia', originalMatchMedia);
  } else {
    Reflect.deleteProperty(window, 'matchMedia');
  }
  if (originalInnerWidth) Object.defineProperty(window, 'innerWidth', originalInnerWidth);
});

function renderInChain(children: React.ReactNode, chain?: ClusterChainConfig) {
  const container = document.createElement('div');
  container.innerHTML = renderToStaticMarkup(
    React.createElement(Provider, null,
      React.createElement(MultichainContext.Provider, { value: chain ? { chain } : null }, children),
    ),
  );
  return container;
}

test('selectedChainTransferRowsKeepAddressLinks', function selectedChainTransferRowsKeepAddressLinks() {
  const table = React.createElement(TableRoot, null,
    React.createElement(TableBody, null,
      React.createElement(TokenTransferTableItem, erc20),
    ),
  );
  const list = React.createElement(TokenTransferListItem, erc20);

  for (const width of [ 375, 1280 ]) {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: width });
    window.dispatchEvent(new Event('resize'));
    for (const row of [ table, list ]) {
      for (const chain of [ chainA, chainB ]) {
        const container = renderInChain(row, chain);
        const addresses = container.querySelectorAll('[data-entity-kind="address"]');

        expect(addresses).toHaveLength(2);
        expect(Array.from(addresses, (address) => address.querySelector('[data-entity-link]')?.getAttribute('href'))).toEqual([
          `/chain/${ chain.slug }/address/${ erc20.from.hash }`,
          `/chain/${ chain.slug }/address/${ erc20.to.hash }`,
        ]);
        expect(container.querySelector('[data-entity-kind="token"] [data-entity-link]')?.getAttribute('href')).toBe(
          `/chain/${ chain.slug }/token/${ erc20.token?.address_hash }`,
        );
        expect(Array.from(addresses, (address) => address.querySelector('svg use')?.getAttribute('href')?.split('#')[1])).toEqual([
          'contracts/verified',
          'contracts/regular',
        ]);
        for (const address of addresses) {
          expect(address.querySelector('.entity__shield')).toBeNull();
          expect(address.querySelector('[aria-label="copy"]')).not.toBeNull();
        }
      }
    }
  }
});

test('explicitAddressChainOverridesSelectedChain', function explicitAddressChainOverridesSelectedChain() {
  const container = renderInChain(React.createElement(AddressEntity, {
    address: erc20.from,
    chain: chainB,
    query: { tab: 'token_transfers' },
  }), chainA);

  expect(container.querySelector('[data-entity-link]')?.getAttribute('href')).toBe(
    `/chain/${ chainB.slug }/address/${ erc20.from.hash }?tab=token_transfers`,
  );
  expect(container.querySelector('.entity__shield')).not.toBeNull();
});

test('addressWithoutSelectedChainKeepsLocalLink', function addressWithoutSelectedChainKeepsLocalLink() {
  const container = renderInChain(React.createElement(AddressEntity, { address: erc20.from }));

  expect(container.querySelector('[data-entity-link]')?.getAttribute('href')).toBe(`/address/${ erc20.from.hash }`);
  expect(container.querySelector('.entity__shield')).toBeNull();
});

test('explicitAddressHrefAndNoLinkArePreserved', function explicitAddressHrefAndNoLinkArePreserved() {
  const href = '/address/custom-destination';
  const linked = renderInChain(React.createElement(AddressEntity, { address: erc20.from, href }), chainA);
  const unlinked = renderInChain(React.createElement(AddressEntity, { address: erc20.from, noLink: true }), chainA);

  expect(linked.querySelector('[data-entity-link]')?.getAttribute('href')).toBe(href);
  expect(unlinked.querySelector('[data-entity-link]')).toBeNull();
  expect(unlinked.querySelector('[data-entity-content]')?.textContent).toBe(erc20.from.name);
});

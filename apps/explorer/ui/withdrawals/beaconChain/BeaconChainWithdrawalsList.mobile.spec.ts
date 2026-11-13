// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { cleanup, render } from '@testing-library/react';
import React from 'react';

import type { BlockWithdrawalsItem } from 'types/api/block';

import { data as withdrawals } from 'mocks/withdrawals/withdrawals';
import breakpoints from 'toolkit/theme/foundations/breakpoints';
import theme from 'toolkit/theme/theme';
import { afterEach, expect, test, vi } from 'vitest';

import BeaconChainWithdrawalsList from './BeaconChainWithdrawalsList';

vi.hoisted(() => {
  window.__envs = { ...window.__envs, NEXT_PUBLIC_HAS_BEACON_CHAIN: 'true' };
});

class WithdrawalWidthQuery extends EventTarget implements MediaQueryList {
  readonly media: string;
  onchange: MediaQueryList['onchange'] = null;

  constructor(query: string) {
    super();
    this.media = query;
  }

  get matches() {
    const conditions = Array.from(this.media.matchAll(/\((min|max)-width:\s*([\d.]+)(px|r?em)\)/g));
    if (!conditions.length) {
      throw new Error(`Unsupported viewport query: ${ this.media }`);
    }
    return conditions.every(([ , bound, value, unit ]) => {
      const width = Number(value) * (unit === 'px' ? 1 : 16);
      return bound === 'min' ? window.innerWidth >= width : window.innerWidth <= width;
    });
  }

  addListener(listener: Parameters<MediaQueryList['addListener']>[0]) {
    this.addEventListener('change', listener as EventListener | null);
  }

  removeListener(listener: Parameters<MediaQueryList['removeListener']>[0]) {
    this.removeEventListener('change', listener as EventListener | null);
  }
}

const originalWidth = window.innerWidth;
const originalMatchMedia = window.matchMedia;

afterEach(() => {
  cleanup();
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: originalWidth });
  window.matchMedia = originalMatchMedia;
});

function renderMobileBlockWithdrawals(isLoading = false) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: 375 });
  window.matchMedia = (query) => new WithdrawalWidthQuery(query);
  expect(window.innerWidth).toBeLessThan(parseInt(breakpoints.lg, 10));
  expect(window.matchMedia(`(min-width: ${ breakpoints.lg })`).matches).toBe(false);

  const items: Array<BlockWithdrawalsItem> = withdrawals.items.map((item) => ({
    amount: item.amount,
    index: item.index,
    receiver: item.receiver,
    validator_index: item.validator_index,
  }));

  return render(React.createElement(ChakraProvider, {
    value: theme,
    children: React.createElement(BeaconChainWithdrawalsList, { items, view: 'block', isLoading }),
  }));
}

function mobileBlockWithdrawalValue() {
  const result = renderMobileBlockWithdrawals();
  const labels = result.getAllByText('Value', { exact: true });
  expect(labels).toHaveLength(withdrawals.items.length);
  const expectedAmounts = [ '0.00019218', '0.00019218', '0.00018277' ];

  labels.forEach((label, index) => {
    const value = label.nextElementSibling;
    expect(value?.textContent).toBe(`${ expectedAmounts[index] }\u2009ETH`);
    expect(value?.querySelector('[data-loading]')).toBeNull();
    expect(label.parentElement?.textContent).toContain(String(withdrawals.items[index].index));
    expect(label.parentElement?.textContent).toContain(String(withdrawals.items[index].validator_index));
  });
  expect(result.queryByText('Age', { exact: true })).toBeNull();
  expect(result.queryByText('Block', { exact: true })).toBeNull();
  expect(result.getAllByText('To', { exact: true })).toHaveLength(withdrawals.items.length);
}

function mobileBlockWithdrawalValueLoading() {
  const result = renderMobileBlockWithdrawals(true);
  const labels = result.getAllByText('Value', { exact: true });
  expect(labels).toHaveLength(withdrawals.items.length);
  labels.forEach((label) => {
    expect(label.getAttribute('data-loading')).toBe('true');
    expect(label.nextElementSibling?.querySelector('[data-loading="true"]')).not.toBeNull();
  });
  expect(result.queryByText('Age', { exact: true })).toBeNull();
}

test(mobileBlockWithdrawalValue, mobileBlockWithdrawalValue);
test(mobileBlockWithdrawalValueLoading, mobileBlockWithdrawalValueLoading);

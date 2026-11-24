// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { cleanup, render } from '@testing-library/react';
import React from 'react';

import type { GasPriceInfo } from 'types/api/stats';
import type { GasUnit } from 'types/client/gasTracker';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import { MultichainContext } from 'lib/contexts/multichain';
import theme from 'toolkit/theme/theme';
import { afterEach, expect, test } from 'vitest';

import formatGasValue from './formatGasValue';
import GasPrice from './GasPrice';

const zeroPrice: GasPriceInfo = {
  price: 0,
  fiat_price: '0',
  time: null,
  base_fee: 0,
  priority_fee: 0,
};

afterEach(cleanup);

function renderPrices(data: GasPriceInfo | null, units: Array<GasUnit>) {
  const chain: ClusterChainConfig = {
    id: '125',
    name: 'Gas price regression fixture',
    slug: 'gas-price-regression',
    app_config: {
      ...config,
      features: {
        ...config.features,
        gasTracker: { title: 'Gas tracker', isEnabled: true, units },
      },
    },
  };

  const view = render(React.createElement(ChakraProvider, {
    value: theme,
    children: React.createElement(MultichainContext.Provider, {
      value: { chain },
      children: React.createElement(React.Fragment, null,
        React.createElement(GasPrice, { data, className: 'primary', prefix: 'Gas: ' }),
        React.createElement(GasPrice, { data, className: 'secondary', unitMode: 'secondary', prefix: 'Also: ' }),
      ),
    }),
  }));

  const prices = {
    primary: view.container.querySelector('.primary')?.textContent ?? null,
    secondary: view.container.querySelector('.secondary')?.textContent ?? null,
  };
  view.unmount();
  return prices;
}

function gasZeroFallback() {
  expect(renderPrices(zeroPrice, [ 'gwei', 'usd' ])).toEqual({ primary: 'Gas: < 0.1 Gwei', secondary: 'Also: < $0.01' });
  expect(renderPrices(zeroPrice, [ 'usd', 'gwei' ])).toEqual({ primary: 'Gas: < $0.01', secondary: 'Also: < 0.1 Gwei' });

  const withFiatPrice = { ...zeroPrice, fiat_price: '1.25' };
  expect(renderPrices(withFiatPrice, [ 'gwei', 'usd' ])).toEqual({ primary: 'Gas: < 0.1 Gwei', secondary: 'Also: $1.25' });
  expect(renderPrices(withFiatPrice, [ 'usd', 'gwei' ])).toEqual({ primary: 'Gas: $1.25', secondary: 'Also: < 0.1 Gwei' });

  const withoutFiatPrice = { ...zeroPrice, fiat_price: null };
  expect(renderPrices(withoutFiatPrice, [ 'usd', 'gwei' ])).toEqual({ primary: 'Gas: < 0.1 Gwei', secondary: null });
  expect(renderPrices(withoutFiatPrice, [ 'gwei', 'usd' ])).toEqual({ primary: 'Gas: < 0.1 Gwei', secondary: null });

  expect(renderPrices(zeroPrice, [ 'gwei' ])).toEqual({ primary: 'Gas: < 0.1 Gwei', secondary: null });
  expect(renderPrices(zeroPrice, [ 'usd' ])).toEqual({ primary: 'Gas: < $0.01', secondary: null });
}

function gasMissingPrices() {
  const withoutNativePrice = { ...zeroPrice, price: null };
  expect(renderPrices(withoutNativePrice, [ 'gwei', 'usd' ])).toEqual({ primary: 'Gas: < $0.01', secondary: null });
  expect(renderPrices(withoutNativePrice, [ 'usd', 'gwei' ])).toEqual({ primary: 'Gas: < $0.01', secondary: null });

  const withoutPrices = { ...withoutNativePrice, fiat_price: null };
  expect(renderPrices(withoutPrices, [ 'gwei', 'usd' ])).toEqual({ primary: 'N/A Gwei', secondary: null });
  expect(renderPrices(withoutPrices, [ 'usd', 'gwei' ])).toEqual({ primary: '$N/A', secondary: null });
  expect(renderPrices(null, [ 'gwei', 'usd' ])).toEqual({ primary: null, secondary: null });

  const absentNativePrice = { ...zeroPrice };
  Reflect.deleteProperty(absentNativePrice, 'price');
  expect(renderPrices(absentNativePrice, [ 'gwei', 'usd' ])).toEqual({ primary: 'Gas: < $0.01', secondary: null });
  expect(renderPrices(absentNativePrice, [ 'usd', 'gwei' ])).toEqual({ primary: 'Gas: < $0.01', secondary: null });

  const absentFiatPrice = { ...zeroPrice };
  Reflect.deleteProperty(absentFiatPrice, 'fiat_price');
  expect(renderPrices(absentFiatPrice, [ 'usd', 'gwei' ])).toEqual({ primary: 'Gas: < 0.1 Gwei', secondary: null });
  expect(renderPrices(absentFiatPrice, [ 'gwei', 'usd' ])).toEqual({ primary: 'Gas: < 0.1 Gwei', secondary: null });
}

function gasZeroFormat() {
  expect(formatGasValue(zeroPrice, 'gwei')).toBe('< 0.1 Gwei');
  expect(formatGasValue(zeroPrice, 'usd')).toBe('< $0.01');
  expect(formatGasValue({ ...zeroPrice, price: 0.099 }, 'gwei')).toBe('< 0.1 Gwei');
  expect(formatGasValue({ ...zeroPrice, price: 0.1 }, 'gwei')).toBe('0.1 Gwei');
  expect(formatGasValue({ ...zeroPrice, fiat_price: '0.009' }, 'usd')).toBe('< $0.01');
  expect(formatGasValue({ ...zeroPrice, fiat_price: '0.01' }, 'usd')).toBe('$0.01');
  expect(formatGasValue({ ...zeroPrice, price: null }, 'gwei')).toBe('N/A Gwei');
  expect(formatGasValue({ ...zeroPrice, fiat_price: null }, 'usd')).toBe('$N/A');

  const absentPrices = { ...zeroPrice };
  Reflect.deleteProperty(absentPrices, 'price');
  Reflect.deleteProperty(absentPrices, 'fiat_price');
  expect(formatGasValue(absentPrices, 'gwei')).toBe('N/A Gwei');
  expect(formatGasValue(absentPrices, 'usd')).toBe('$N/A');
}

test(gasZeroFallback);
test(gasMissingPrices);
test(gasZeroFormat);

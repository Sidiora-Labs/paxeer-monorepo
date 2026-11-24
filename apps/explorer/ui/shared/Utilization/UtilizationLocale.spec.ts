// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { cleanup, render } from '@testing-library/react';
import React from 'react';

import system from 'toolkit/theme/theme';
import { afterEach, expect, test } from 'vitest';

import Utilization from './Utilization';

afterEach(cleanup);

function renderUtilization(props: React.ComponentProps<typeof Utilization>) {
  const component = render(React.createElement(ChakraProvider, {
    value: system,
    children: React.createElement(Utilization, { ...props, id: 'utilization' }),
  }));
  const root = component.container.querySelector('#utilization');
  if (!(root instanceof HTMLElement)) {
    throw new Error('Utilization must render its root element');
  }
  const bar = root.firstElementChild?.firstElementChild?.firstElementChild;
  if (!(bar instanceof HTMLElement)) {
    throw new Error('Utilization must render its bar inside the skeleton and track');
  }
  return { root, bar };
}

function utilizationLocalizedCss() {
  const { root, bar } = renderUtilization({ value: 0.125, locale: 'de-DE' });

  expect(root.querySelector('span')?.textContent).toBe('12,5%');
  expect(window.getComputedStyle(bar).width).toBe('12.5%');
  expect(root.hasAttribute('locale')).toBe(false);
}

function utilizationWidthPreservesPrecision() {
  const { root, bar } = renderUtilization({ value: 0.123456, locale: 'de-DE', colorScheme: 'gray' });

  expect(root.querySelector('span')?.textContent).toBe('12,35%');
  expect(window.getComputedStyle(bar).width).toBe('12.3456%');
}

function utilizationClampsNumericWidth() {
  for (const [ value, expected ] of [ [ -0.1, '0%' ], [ 0, '0%' ], [ 1, '100%' ], [ 1.2, '100%' ], [ NaN, '0%' ] ] as const) {
    const { root, bar } = renderUtilization({ value, locale: 'de-DE' });

    expect(root.querySelector('span')?.textContent).toBe(expected);
    expect(window.getComputedStyle(bar).width).toBe(expected);
    cleanup();
  }
}

function utilizationDefaultLocale() {
  const { root, bar } = renderUtilization({ value: 0.125 });

  expect(root.querySelector('span')?.textContent).toBe((12.5).toLocaleString(undefined, { maximumFractionDigits: 2 }) + '%');
  expect(window.getComputedStyle(bar).width).toBe('12.5%');
}

test(utilizationLocalizedCss, utilizationLocalizedCss);
test(utilizationWidthPreservesPrecision, utilizationWidthPreservesPrecision);
test(utilizationClampsNumericWidth, utilizationClampsNumericWidth);
test(utilizationDefaultLocale, utilizationDefaultLocale);

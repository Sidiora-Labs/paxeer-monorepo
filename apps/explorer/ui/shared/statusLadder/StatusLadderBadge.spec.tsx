// @vitest-environment jsdom

import React from 'react';

import { PAXEER_X_NATIVE_RUNGS, PAXEER_X_STATUS_RUNGS } from 'types/api/paxeerXLists';

import { Provider } from 'toolkit/chakra/provider';
import { beforeAll, describe, it, expect } from 'vitest';
import { render, screen } from 'vitest/lib';

import { NATIVE_RUNG_ORDER, RUNGS, RUNG_ORDER } from './rungs';
import StatusLadderBadge, { NativeSettlementBadge } from './StatusLadderBadge';

// jsdom does not implement matchMedia, which Chakra's responsive hooks read on mount
const createMediaQueryList = (query: string): MediaQueryList => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: () => undefined,
  removeListener: () => undefined,
  addEventListener: () => undefined,
  removeEventListener: () => undefined,
  dispatchEvent: () => false,
});

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', { writable: true, value: createMediaQueryList });
});

describe('StatusLadderBadge', () => {
  it.each(PAXEER_X_STATUS_RUNGS)('renders the %s rung with its label', (rung) => {
    render(
      <Provider>
        <StatusLadderBadge rung={ rung }/>
      </Provider>,
    );

    const badge = screen.getByText(RUNGS[rung].label);
    expect(badge).toBeDefined();
    expect(badge.closest('[data-rung]')?.getAttribute('data-rung')).toBe(rung);
  });
});

describe('status ladder rungs', () => {
  it('covers the four rungs in ladder order', () => {
    expect(RUNG_ORDER.map((item) => item.rung)).toEqual([ 'pending', 'instant', 'sealed', 'final' ]);
  });

  it('explains every rung in exactly one sentence', () => {
    for (const item of RUNG_ORDER) {
      expect(item.description.endsWith('.')).toBe(true);
      expect(item.description.match(/\./g)).toHaveLength(1);
    }
  });
});

describe('NativeSettlementBadge', () => {
  function rendersEveryNativeSettlementRungWithItsLabel() {
    expect(NATIVE_RUNG_ORDER.map((item) => item.rung)).toEqual([ 'submitted', 'receipted', 'committed' ]);

    const labels = { submitted: 'Submitted', receipted: 'Receipted', committed: 'Committed' };

    for (const rung of PAXEER_X_NATIVE_RUNGS) {
      const { unmount } = render(
        <Provider>
          <NativeSettlementBadge rung={ rung }/>
        </Provider>,
      );

      const badge = screen.getByText(labels[rung]);
      expect(badge).toBeDefined();
      expect(badge.closest('[data-rung]')?.getAttribute('data-rung')).toBe(rung);
      unmount();
    }
  }

  function rendersNothingForARungOutsideTheNativeLadder() {
    expect(() => {
      const { container } = render(<NativeSettlementBadge rung="final"/>);

      expect(container.innerHTML).toBe('');
    }).not.toThrow();
  }

  it('renders every native settlement rung with its label', rendersEveryNativeSettlementRungWithItsLabel);
  it('renders nothing for a rung outside the native ladder', rendersNothingForARungOutsideTheNativeLadder);
});

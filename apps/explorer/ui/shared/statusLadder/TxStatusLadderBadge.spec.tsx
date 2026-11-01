// @vitest-environment jsdom

import React from 'react';

import type { PaxeerXTxStatus } from 'types/api/paxeerXLists';

import { TX_HASH } from 'stubs/tx';
import { render } from 'ui/shared/layout/testWrapper';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { screen, waitFor } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
  };
});

import TxStatusLadderBadgeComponent from './TxStatusLadderBadge';

const TxStatusLadderBadge = TxStatusLadderBadgeComponent;

describe('TxStatusLadderBadge', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
  });

  afterEach(() => {
    fetchMock.resetMocks();
  });

  async function tx_badge_renders_receipted_answer() {
    const answer: PaxeerXTxStatus = {
      rung: 'receipted',
      block_number: null,
      committed_height: 10,
    };

    fetchMock.mockResponse(async(request) => {
      expect(request.method).toBe('GET');
      expect(new URL(request.url).pathname).toBe(`/api/v2/transactions/${ TX_HASH }/status`);

      return {
        body: JSON.stringify(answer),
        headers: { 'Content-Type': 'application/json' },
      };
    });

    render(<TxStatusLadderBadge hash={ TX_HASH }/>);

    await waitFor(() => {
      const badge = screen.getByText('Receipted').closest('[data-rung]');

      expect(badge?.getAttribute('data-rung')).toBe('receipted');
      expect(badge?.hasAttribute('data-loading')).toBe(false);
    });

    expect(screen.getByText('Receipted').textContent).toBe('Receipted');
    expect(fetchMock).toHaveBeenCalledTimes(1);
  }

  test(tx_badge_renders_receipted_answer.name, tx_badge_renders_receipted_answer);
});

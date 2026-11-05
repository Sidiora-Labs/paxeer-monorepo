// @vitest-environment jsdom

import { onlineManager } from '@tanstack/react-query';
import React from 'react';

import type { PaxeerXArchiveReceipt } from 'types/api/paxeerXArchive';

import { PAXEER_X_RECEIPTS_ITEM } from 'stubs/paxeerXLists';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, screen, waitFor } from 'vitest/lib';

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

// The host runs the whole suite at once, and these trees mount real entities, so the first render of
// each of them reaches well past the default per-test budget.
vi.setConfig({ testTimeout: 60_000 });

import PaxeerXReceiptPage from './PaxeerXReceipt';

const receipt: PaxeerXArchiveReceipt = {
  ...PAXEER_X_RECEIPTS_ITEM,
  frozen: true,
  verification_status: 'checkpoint_finalised',
  payload_hash: '0x3ed9d81e7c1001bdda1caa1dc62c0acbbe3d2c671cdc20dc1e65efdaa4186967',
  transaction_hash: '0x8f9e7d6c5b4a39281706f5e4d3c2b1a0998877665544332211ffeeddccbbaa99',
  provenance: { evm: { ...PAXEER_X_RECEIPTS_ITEM.provenance.evm, transaction_hash: '0x8f9e7d6c5b4a39281706f5e4d3c2b1a0998877665544332211ffeeddccbbaa99' } },
  timestamp: '2023-05-22T18:00:36.000000Z',
};

const JSON_HEADERS = { 'Content-Type': 'application/json' };
const archivePath = `/api/v2/paxeer-x/archive/receipts/${ receipt.id }`;
const archiveBodies: Array<unknown> = [];
const unexpectedRequests: Array<string> = [];

function serveArchive(body: unknown) {
  fetchMock.mockResponse((request) => {
    const path = new URL(request.url).pathname;
    if (path !== archivePath) {
      unexpectedRequests.push(path);
      return Promise.reject(new Error(`Unexpected archive request: ${ path }`));
    }
    archiveBodies.push(body);
    return Promise.resolve({ body: JSON.stringify(body), headers: JSON_HEADERS });
  });
}

const setVisibility = (state: DocumentVisibilityState) => {
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
  document.dispatchEvent(new Event('visibilitychange'));
};

const setOnline = (online: boolean) => {
  Object.defineProperty(navigator, 'onLine', { configurable: true, get: () => online });
  window.dispatchEvent(new Event(online ? 'online' : 'offline'));
};

const freshnessOf = (container: HTMLElement) => container.querySelector('[data-receipt-freshness]');

const renderPage = async() => {
  const result = render(<PaxeerXReceiptPage/>);

  await screen.findByText('Verification status');

  return result;
};

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

// Each rule is inserted on its own, so a declaration belongs to the wide layout when the rule that
// carries it is a breakpoint rule and not the base rule, which declares its own minimum width.
const isWide = (text: string) => /^\s*@media[^{]*\(min-width/.test(text);

const LEFT_INSET = 'margin-(?:left|inline-start)';

// The scale resolves a zero inset either to the plain length or to the token of the same name.
const isSpace = (value: string) => !/^0[a-z%]*$/.test(value) && !/spacing-0\b/.test(value);

describe('PaxeerXReceiptPageContent', () => {
  beforeEach(() => {
    routerState.pathname = '/paxeer-x/receipts/[id]';
    routerState.query = { id: receipt.id };
    onlineManager.setOnline(true);
    fetchMock.resetMocks();
    archiveBodies.length = 0;
    unexpectedRequests.length = 0;
    serveArchive(receipt);
  });

  afterEach(() => {
    expect(unexpectedRequests).toEqual([]);
  });

  it('heads the page with the receipt label, its identifier and a copy control', async() => {
    const { container } = await renderPage();

    expect(screen.getByRole('heading', { level: 1 }).textContent).toContain('Kernel receipt');

    const identifier = container.querySelector('[data-receipt-identifier]') as HTMLElement;

    expect(identifier.textContent).toContain(receipt.id);
    expect(identifier.querySelector('[aria-label="copy"]')).toBeTruthy();
  });

  it('puts the api entry beside the section pill', async() => {
    const { container } = await renderPage();

    const tabs = container.querySelector('[data-scan-section-tabs]') as HTMLElement;

    expect(Array.from(tabs.querySelectorAll('[data-tab]')).map((tab) => tab.textContent)).toEqual([ 'Overview' ]);
    expect(tabs.querySelector('[data-right-slot] [data-api-entry]')?.textContent).toBe('API');
  });

  it('renders the receipt on the shared key-value card', async() => {
    const { container } = await renderPage();

    const card = container.querySelector('[data-receipt-details-card]') as HTMLElement;

    expect(card).toBeTruthy();
    expect(card.querySelector('[data-field="id"]')?.textContent).toBe(receipt.id);
    expect(card.querySelector('[data-field="verification_status"]')?.textContent).toBe('Checkpoint finalised');
    expect(card.querySelector(`a[href="/tx/${ receipt.transaction_hash }"]`)).toBeTruthy();
    expect(card.querySelector(`a[href="/block/${ receipt.block_number }"]`)).toBeTruthy();
  });

  it('holds the card back until the node answers', () => {
    const { container } = render(<PaxeerXReceiptPage/>);

    expect(container.querySelector('[data-receipt-details-card]')).toBeNull();
    expect(container.querySelector('[data-receipt-identifier]')?.textContent).toContain(receipt.id);
  });

  async function receipt_page_shows_frozen_verification_status() {
    const { container } = await renderPage();

    expect(screen.getByRole('heading', { level: 1 }).textContent).toContain('Kernel receipt');
    expect(container.querySelector('[data-field="id"]')?.textContent).toBe(receipt.id);
    expect(container.querySelector('[data-verification-status]')?.getAttribute('data-verification-status'))
      .toBe(receipt.verification_status);
    expect(container.querySelector('[data-field="verification_status"]')?.textContent).toBe('Checkpoint finalised');
    expect(container.querySelector('[data-archive-notice]')?.textContent)
      .toContain('Kernel archive. This receipt and its verification status are frozen historical evidence.');
    expect(screen.queryByText('Settlement status')).toBeNull();
    expect(container.querySelector('[data-settlement-status]')).toBeNull();
    expect(container.querySelector('[data-rung]')).toBeNull();
  }

  it('receipt_page_shows_frozen_verification_status', receipt_page_shows_frozen_verification_status);

  it('lets the receipt identifier wrap and keeps its inset for the wide layout only', () => {
    const { container } = render(<PaxeerXReceiptPage/>);

    const identifier = container.querySelector('[data-receipt-identifier]') as HTMLElement;
    const insets = declarationsOf(identifier, LEFT_INSET);
    const spaced = insets.filter((rule) => isSpace(rule.value));

    expect(declarationsOf(identifier, 'flex-wrap').some((rule) => rule.value === 'wrap' && !isWide(rule.text))).toBe(true);
    expect(insets.length).toBeGreaterThan(0);
    expect(spaced.length).toBeGreaterThan(0);
    expect(spaced.every((rule) => isWide(rule.text))).toBe(true);
  });

  describe('freshness of the evidence', () => {
    afterEach(() => {
      onlineManager.setOnline(true);
      Reflect.deleteProperty(document, 'visibilityState');
      Reflect.deleteProperty(navigator, 'onLine');
    });

    it('reports frozen evidence complete once the archive has answered', async() => {
      const { container } = await renderPage();

      await waitFor(() => {
        expect(freshnessOf(container)?.getAttribute('data-receipt-freshness')).toBe('complete');
      });
      expect(freshnessOf(container)?.textContent)
        .toContain('Recorded archive verification status. This historical evidence is frozen; automatic refresh stopped.');
      expect(container.querySelector('[data-receipt-checked-at]')).toBeTruthy();
    });

    it('reports the evidence complete once verification is anchored', async() => {
      serveArchive({ ...receipt, verification_status: 'settlement_anchored' });

      const { container } = await renderPage();

      await waitFor(() => {
        expect(freshnessOf(container)?.getAttribute('data-receipt-freshness')).toBe('complete');
      });
      expect(freshnessOf(container)?.textContent)
        .toContain('Recorded archive verification status. This historical evidence is frozen; automatic refresh stopped.');
    });

    it('keeps the recorded evidence complete while the page is hidden', async() => {
      const { container } = await renderPage();

      act(() => {
        setVisibility('hidden');
      });

      await waitFor(() => {
        expect(freshnessOf(container)?.getAttribute('data-receipt-freshness')).toBe('complete');
      });
      expect(freshnessOf(container)?.textContent).toContain('This historical evidence is frozen; automatic refresh stopped.');
    });

    it('keeps the exact archive body without another request when the page returns', async function archiveReceiptVisibilityDoesNotRefreshFrozenEvidence() {
      const { container } = await renderPage();
      expect(archiveBodies).toEqual([ receipt ]);
      expect(new URL(String(fetchMock.mock.calls[0][0])).pathname).toBe(archivePath);

      act(() => {
        setVisibility('hidden');
      });
      act(() => {
        setVisibility('visible');
      });
      await new Promise((resolve) => setTimeout(resolve, 5_500));

      expect(freshnessOf(container)?.getAttribute('data-receipt-freshness')).toBe('complete');
      expect(container.querySelector('[data-verification-status]')?.getAttribute('data-verification-status')).toBe('checkpoint_finalised');
      expect(archiveBodies).toEqual([ receipt ]);
      expect(fetchMock.mock.calls.filter(([ url ]) => new URL(String(url)).pathname === archivePath)).toHaveLength(1);
    });

    it('keeps the frozen evidence complete when the connection drops', async() => {
      const { container } = await renderPage();

      act(() => {
        setOnline(false);
      });

      await waitFor(() => {
        expect(freshnessOf(container)?.getAttribute('data-receipt-freshness')).toBe('complete');
      });
      expect(freshnessOf(container)?.textContent).toContain('This historical evidence is frozen; automatic refresh stopped.');
      expect(container.querySelector('[data-field="id"]')?.textContent).toBe(receipt.id);
    });
    it.each([
      { ...receipt, frozen: false },
      { ...receipt, verification_status: 'unknown' },
      { ...receipt, id: 'different-receipt' },
      { ...receipt, block_number: -1 },
    ])('refuses an invalid archive response %#', async(body) => {
      serveArchive(body);
      const { container, unmount } = render(<PaxeerXReceiptPage/>);
      await waitFor(() => {
        expect(container.querySelector('[data-receipt-refresh-error]')).not.toBeNull();
      });
      expect(container.querySelector('[data-receipt-details-card]')).toBeNull();
      expect(archiveBodies).toEqual([ body ]);
      unmount();
    });
  });
});

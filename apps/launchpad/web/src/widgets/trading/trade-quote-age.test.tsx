import { JSDOM } from 'jsdom';
import { renderToStaticMarkup } from 'react-dom/server';
import { expect, test } from 'vitest';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { QuickTradeQuoteNotice } from '@/widgets/terminal/right-rail/quick-trade-card';
import { TradeQuoteNotices } from './trade-panel-parts';
import { currentTradeQuote } from './use-trade-panel';
import type { QuoteEnvelope, TradeContext } from './transaction-controls/domain';

const context: TradeContext = {
  chainId: 125,
  manifestRevision: 'quote-age',
  pool: CORE_DEPLOYMENT.poolRegistry,
  token: CORE_DEPLOYMENT.usdl,
  router: CORE_DEPLOYMENT.router,
  sender: CORE_DEPLOYMENT.factory,
  recipient: CORE_DEPLOYMENT.factory,
  inputRaw: 1000000n,
  isBuy: true,
  opticalRevision: 'none',
  opticalData: '0x',
};
const quote: QuoteEnvelope = {
  quoteId: 'reviewed-price',
  context,
  blockHeight: 1n,
  quotedAt: 1000,
  expiresAt: 31000,
  outputRaw: 2000000n,
  feeComponents: [30000n],
  impactBps: 10,
};

function inspectNotices(current: ReturnType<typeof currentTradeQuote>, text: string) {
  const surfaces = [
    <TradeQuoteNotices key="panel" priceMoved={current.priceMoved} staleQuoteCause={current.staleQuoteCause} error={undefined} onRefresh={() => {}} />,
    <QuickTradeQuoteNotice key="quick" stale={!!current.staleQuote && !current.quote} cause={current.staleQuoteCause} onRefresh={() => {}} />,
  ];
  for (const surface of surfaces) {
    const dom = new JSDOM(renderToStaticMarkup(surface));
    try {
      expect(dom.window.document.body.textContent).toContain(text);
      const buttons = dom.window.document.querySelectorAll('button');
      expect(buttons).toHaveLength(1);
      expect(buttons[0].textContent).toBe('Refresh');
      expect(buttons[0].disabled).toBe(false);
      if (current.staleQuoteCause === 'aged') {
        expect(dom.window.document.body.textContent).not.toContain('The price moved.');
      } else {
        expect(dom.window.document.body.textContent).not.toContain('out of date');
      }
    } finally {
      dom.window.close();
    }
  }
}

function agedPriceIsNamedAged() {
  const fresh = currentTradeQuote({ quote }, context, quote.expiresAt - 1);
  expect(fresh).toEqual({ quote, staleQuote: undefined, priceMoved: false, staleQuoteCause: undefined });
  for (const state of [{ quote }, { staleQuote: quote }, { staleQuote: quote, staleQuoteReason: 'submission' as const }]) {
    const aged = currentTradeQuote(state, { ...context }, quote.expiresAt);
    expect(aged.quote).toBeUndefined();
    expect(aged.staleQuote).toBe(quote);
    expect(aged.priceMoved).toBe(true);
    expect(aged.staleQuoteCause).toBe('aged');
    inspectNotices(aged, 'The price is out of date. Refresh to see the current price.');
  }
}
test(agedPriceIsNamedAged.name, agedPriceIsNamedAged);

function movedPriceKeepsItsNotice() {
  for (const now of [quote.expiresAt - 1, quote.expiresAt]) {
    for (const changed of [{ ...context, inputRaw: context.inputRaw + 1n }, { ...context, opticalRevision: 'updated' }, undefined]) {
      const moved = currentTradeQuote({ quote }, changed, now);
      expect(moved.quote).toBeUndefined();
      expect(moved.staleQuote).toBe(quote);
      expect(moved.priceMoved).toBe(true);
      expect(moved.staleQuoteCause).toBe('moved');
      inspectNotices(moved, 'The price moved. Refresh to see the new price.');
    }
    const refused = currentTradeQuote({ staleQuote: quote, staleQuoteReason: 'simulation' }, context, now);
    expect(refused.quote).toBeUndefined();
    expect(refused.staleQuote).toBe(quote);
    expect(refused.priceMoved).toBe(true);
    expect(refused.staleQuoteCause).toBe('moved');
    inspectNotices(refused, 'The price moved. Refresh to see the new price.');
  }
}
test(movedPriceKeepsItsNotice.name, movedPriceKeepsItsNotice);

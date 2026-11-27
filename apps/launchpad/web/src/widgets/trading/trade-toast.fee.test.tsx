import { JSDOM } from 'jsdom';
import { renderToStaticMarkup } from 'react-dom/server';
import { expect, test } from 'vitest';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import TradeToast, { type TradeToastData } from './trade-toast';
import { tradeFeeAsset } from './transaction-controls/providers';

const token = CORE_DEPLOYMENT.factory;
const tokenAsset = { address: token, symbol: 'LTEST' };

function cardData(isBuy: boolean, feeRaw: bigint, decimals = 6): TradeToastData {
  return {
    type: isBuy ? 'buy' : 'sell',
    inputAmount: '10',
    inputSymbol: isBuy ? 'USDL' : 'LTEST',
    outputAmount: '9',
    outputSymbol: isBuy ? 'LTEST' : 'USDL',
    newUsdlBalance: null,
    newTokenBalance: null,
    tokenName: 'LTEST',
    txHash: '',
    feeRaw: feeRaw.toString(),
    feeAsset: {
      ...tradeFeeAsset(isBuy, CORE_DEPLOYMENT.usdl, token, decimals),
      symbol: isBuy ? 'USDL' : undefined,
    },
  };
}

function inspectCard(data: TradeToastData, inspect: (card: Element) => void, asset: { address: string; symbol: string } = tokenAsset) {
  const dom = new JSDOM(renderToStaticMarkup(<TradeToast data={data} tokenAsset={asset} onDismiss={() => {}} />));
  try {
    const card = dom.window.document.querySelector('[role="status"]')!;
    inspect(card);
  } finally {
    dom.window.close();
  }
}

function feeLine(card: Element) {
  return [...card.querySelectorAll('dt')].find(label => label.textContent === 'Fee paid')?.parentElement;
}

function cardShowsTheFeeOfABuyInUsdl() {
  const data = cardData(true, 30000n, 18);
  expect(data.feeAsset).toMatchObject({ address: CORE_DEPLOYMENT.usdl, decimals: 6 });
  inspectCard(data, card => {
    expect(feeLine(card)?.querySelector('dd')?.textContent).toBe('0.03 USDL');
    expect([...card.querySelectorAll('dd, p, span')].some(line => /\braw\s*$/.test(line.textContent ?? ''))).toBe(false);
    expect(card.textContent).not.toMatch(/\braw\b/);
  });
  inspectCard(cardData(true, 0n), card => {
    expect(feeLine(card)?.querySelector('dd')?.textContent).toBe('0 USDL');
  });
}
test(cardShowsTheFeeOfABuyInUsdl.name, cardShowsTheFeeOfABuyInUsdl);

function cardShowsTheFeeOfASellInTheCoin() {
  const data = cardData(false, 7000000n);
  expect(data.feeAsset).toMatchObject({ address: token, decimals: 6 });
  inspectCard(data, card => {
    expect(feeLine(card)?.querySelector('dd')?.textContent).toBe('7 LTEST');
    expect(card.textContent).not.toMatch(/\braw\b/);
  });
  inspectCard(cardData(false, 1234567890123456789n, 18), card => {
    expect(feeLine(card)?.querySelector('dd')?.textContent).toBe('1.234567890123456789 LTEST');
  });
}
test(cardShowsTheFeeOfASellInTheCoin.name, cardShowsTheFeeOfASellInTheCoin);

function cardLeavesTheFeeOutWithoutItsAsset() {
  const data = cardData(false, 7000000n);
  inspectCard({ ...data, feeAsset: undefined }, card => {
    expect(feeLine(card)).toBeUndefined();
    expect(card.textContent).not.toContain('Fee paid');
    expect(card.textContent).not.toMatch(/\braw\b/);
  });
  inspectCard(data, card => {
    expect(feeLine(card)).toBeUndefined();
  }, { address: token, symbol: '' });
  inspectCard(data, card => {
    expect(feeLine(card)).toBeUndefined();
  }, { address: CORE_DEPLOYMENT.usdl, symbol: 'USDL' });
}
test(cardLeavesTheFeeOutWithoutItsAsset.name, cardLeavesTheFeeOutWithoutItsAsset);

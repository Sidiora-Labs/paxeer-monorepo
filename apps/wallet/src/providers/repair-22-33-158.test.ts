// @vitest-environment jsdom
import { act, createElement, memo } from 'react';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { mount } from '@/account/test/render';
import { announceCurrencyChanged, CURRENCY_RATES_EVENT, initCurrencyService } from '@/lib/currency';
import { currencyRatesRepository, preferencesRepository } from '@/platform/storage/repositories';
import { HeroBalance, type HeroBalanceProps } from '@/widgets/portfolio/HeroBalance';
import { LocaleProvider, useLocale } from './LocaleProvider';

const portfolio: HeroBalanceProps = {
  totalUsd: 100,
  loading: false,
  paxPrice: 2,
  dailyPnlUsd: '10',
  dailyPnlPercent: '10',
};

const SubscribedPortfolio = memo(function SubscribedPortfolio(props: HeroBalanceProps) {
  const { locale, currencyRevision } = useLocale();
  return createElement(
    'section',
    { 'data-locale': locale, 'data-currency-revision': currencyRevision },
    createElement(HeroBalance, props)
  );
});

function loadStoredRates(eur: number): void {
  currencyRatesRepository.write({
    base: 'USD',
    rates: { USD: 1, EUR: eur, GBP: 0.5 },
    fetchedAt: Date.now(),
  });
  initCurrencyService();
}

beforeEach(() => {
  localStorage.clear();
  preferencesRepository.write({
    ...preferencesRepository.read(),
    language: 'en',
    currency: 'EUR',
  });
  loadStoredRates(0.8);
});

afterEach(() => {
  localStorage.clear();
});

async function repair_9_refreshes_mounted_portfolio_after_currency_rates_event() {
  const preferences = preferencesRepository.read();
  const events: string[] = [];
  const onRates = (event: Event) => events.push(event.type);
  window.addEventListener(CURRENCY_RATES_EVENT, onRates);
  const view = await mount(
    createElement(LocaleProvider, null, createElement(SubscribedPortfolio, portfolio))
  );
  try {
    const amount = view.query('h1');
    expect(view.text('h1')).toBe('€80.00');
    expect(view.text('section')).toContain('+€8.00');
    expect(view.text('section')).toContain('PAX €1.60');
    expect(view.query('section')?.getAttribute('data-currency-revision')).toBe('0');

    await act(async () => {
      loadStoredRates(0.9);
      announceCurrencyChanged();
    });

    expect(events).toEqual([CURRENCY_RATES_EVENT]);
    expect(view.query('h1')).toBe(amount);
    expect(view.text('h1')).toBe('€90.00');
    expect(view.text('section')).toContain('+€9.00');
    expect(view.text('section')).toContain('PAX €1.80');
    expect(view.query('section')?.getAttribute('data-currency-revision')).toBe('1');
    expect(view.query('section')?.getAttribute('data-locale')).toBe('en');
    expect(preferencesRepository.read()).toEqual(preferences);

    await act(async () => {
      loadStoredRates(1.1);
      announceCurrencyChanged();
    });

    expect(events).toEqual([CURRENCY_RATES_EVENT, CURRENCY_RATES_EVENT]);
    expect(view.query('h1')).toBe(amount);
    expect(view.text('h1')).toBe('€110.00');
    expect(view.query('section')?.getAttribute('data-currency-revision')).toBe('2');
    expect(view.query('section')?.getAttribute('data-locale')).toBe('en');
    expect(preferencesRepository.read()).toEqual(preferences);
  } finally {
    window.removeEventListener(CURRENCY_RATES_EVENT, onRates);
    await view.unmount();
  }
}

async function repair_9_refreshes_mounted_portfolio_when_display_currency_changes() {
  const view = await mount(
    createElement(LocaleProvider, null, createElement(SubscribedPortfolio, portfolio))
  );
  try {
    const amount = view.query('h1');
    expect(view.text('h1')).toBe('€80.00');

    await act(async () => {
      preferencesRepository.write({ ...preferencesRepository.read(), currency: 'GBP' });
      announceCurrencyChanged();
    });

    expect(view.query('h1')).toBe(amount);
    expect(view.text('h1')).toBe('£50.00');
    expect(view.text('section')).toContain('+£5.00');
    expect(view.text('section')).toContain('PAX £1.00');
    expect(view.query('section')?.getAttribute('data-currency-revision')).toBe('1');
    expect(view.query('section')?.getAttribute('data-locale')).toBe('en');

    await act(async () => {
      preferencesRepository.write({ ...preferencesRepository.read(), currency: 'USD' });
      announceCurrencyChanged();
    });

    expect(view.query('h1')).toBe(amount);
    expect(view.text('h1')).toBe('$100.00');
    expect(view.query('section')?.getAttribute('data-currency-revision')).toBe('2');
    expect(view.query('section')?.getAttribute('data-locale')).toBe('en');
  } finally {
    await view.unmount();
  }
}

test(
  repair_9_refreshes_mounted_portfolio_after_currency_rates_event.name,
  repair_9_refreshes_mounted_portfolio_after_currency_rates_event
);
test(
  repair_9_refreshes_mounted_portfolio_when_display_currency_changes.name,
  repair_9_refreshes_mounted_portfolio_when_display_currency_changes
);

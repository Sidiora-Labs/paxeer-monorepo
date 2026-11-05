// @vitest-environment jsdom

import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { screen, waitFor } from 'vitest/lib';

import { counters, counterSemantics, negativeChangeCounter, positiveChangeCounter, utilizationCounter } from './numberWidgetsFixtures';
import { formatCounterUnits, formatCounterValue, getCounterDeltaDirection } from './NumberWidgetsList';
import { renderNumberWidgets, startCountersServer } from './numberWidgetsTestServer';

let server: Awaited<ReturnType<typeof startCountersServer>>;

const renderList = async() => {
  const result = renderNumberWidgets(server.endpoint, { counterSemantics });

  await screen.findByText('Total addresses');

  return result;
};

describe('formatCounterValue', () => {
  it('shortens a large counter and keeps a small one readable', () => {
    expect(formatCounterValue('1804523')).toBe('1.805M');
    expect(formatCounterValue('0.175')).toBe('0.175');
  });
});

describe('formatCounterUnits', () => {
  it('writes seconds against the number and every other unit after a space', () => {
    expect(formatCounterUnits('s')).toBe('s');
    expect(formatCounterUnits('%')).toBe('%');
    expect(formatCounterUnits('PAX')).toBe(' PAX');
    expect(formatCounterUnits()).toBe('');
  });
});

describe('getCounterDeltaDirection', () => {
  it('reads an explicitly declared change as rising or falling', () => {
    expect(getCounterDeltaDirection(positiveChangeCounter, 'change')).toBe('up');
    expect(getCounterDeltaDirection(negativeChangeCounter, 'change')).toBe('down');
    expect(getCounterDeltaDirection({ ...negativeChangeCounter, units: 'PAX' }, 'change')).toBe('down');
  });

  it('leaves an absolute counter without a change tone', () => {
    expect(getCounterDeltaDirection({ id: 'c', title: 'c', value: '42', description: '' })).toBeUndefined();
    expect(getCounterDeltaDirection({ id: 'd', title: 'd', value: 'not a number', units: '%', description: '' })).toBeUndefined();
    expect(getCounterDeltaDirection(utilizationCounter)).toBeUndefined();
    expect(getCounterDeltaDirection(utilizationCounter, 'absolute')).toBeUndefined();
    expect(getCounterDeltaDirection({ ...positiveChangeCounter, value: 'not a number' }, 'change')).toBeUndefined();
  });
});

describe('NumberWidgetsList', () => {
  beforeAll(async() => {
    fetchMock.disableMocks();
    server = await startCountersServer();
  });

  afterAll(async() => {
    await server.close();
    fetchMock.enableMocks();
  });

  it('lays the counters out as a grid of stat cards', async() => {
    const { container } = await renderList();

    const grid = container.querySelector('[data-stats-number-widgets]') as HTMLElement;
    const cards = Array.from(grid.querySelectorAll('[data-scan-stat]'));

    expect(cards).toHaveLength(counters.counters.length);
    expect(cards.map((card) => card.getAttribute('data-scan-stat')))
      .toEqual(counters.counters.map((counter) => counter.title));
  });

  it('gives every card its label, its value and its information icon', async() => {
    const { container } = await renderList();

    const card = container.querySelector('[data-scan-stat="Total addresses"]') as HTMLElement;

    expect(card.querySelector('[data-label]')?.textContent).toBe('Total addresses');
    expect(card.querySelector('[data-value]')?.textContent).toBe('1.805M');
    expect(card.querySelector('[aria-label="hint"]')).not.toBeNull();
  });

  it('writes the units of a counter that carries them beside its value', async() => {
    const { container } = await renderList();

    const card = container.querySelector('[data-scan-stat="Average block time"]') as HTMLElement;

    expect(card.querySelector('[data-value]')?.textContent).toBe('0.175s');
  });

  it('tones a rising change apart from a falling one', async() => {
    const { container } = await renderList();

    await waitFor(() => {
      expect(container.querySelector('[data-scan-stat="Coin price increase"] [data-delta="up"]')?.textContent).toBe('42%');
    });
    expect(container.querySelector('[data-scan-stat="Coin price decrease"] [data-delta="down"]')?.textContent).toBe('-7.4%');
  });

  it('keeps the producer utilization percentage neutral', async() => {
    const { container } = await renderList();
    const card = container.querySelector('[data-scan-stat="Network utilization"]');

    expect(card?.querySelector('[data-value]')?.textContent).toBe('1.554%');
    expect(card?.querySelector('[data-delta]')).toBeNull();
  });
});

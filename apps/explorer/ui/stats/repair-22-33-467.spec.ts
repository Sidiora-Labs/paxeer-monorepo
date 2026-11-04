// @vitest-environment jsdom

import { afterAll, beforeAll, describe, expect, test } from 'vitest';
import { screen } from 'vitest/lib';

import { counterSemantics, negativeChangeCounter, positiveChangeCounter, utilizationCounter } from './numberWidgetsFixtures';
import { getCounterDeltaDirection } from './NumberWidgetsList';
import { renderNumberWidgets, startCountersServer } from './numberWidgetsTestServer';

import './NumberWidgetsList.spec';

describe('repair 22.33.467', () => {
  let server: Awaited<ReturnType<typeof startCountersServer>>;

  beforeAll(async() => {
    fetchMock.disableMocks();
    server = await startCountersServer();
  });

  afterAll(async() => {
    await server.close();
    fetchMock.enableMocks();
  });

  test('statsPercentIsNotDelta', async function statsPercentIsNotDelta() {
    const { container } = renderNumberWidgets(server.endpoint);

    await screen.findByText(utilizationCounter.title);

    const card = container.querySelector('[data-scan-stat="Network utilization"]');

    expect(server.requests).toContain('GET /api/v1/counters');
    expect(card?.querySelector('[data-value]')?.textContent).toBe('1.554%');
    expect(card?.querySelector('[data-delta]')).toBeNull();
    expect(container.querySelector('[data-delta]')).toBeNull();
    expect(getCounterDeltaDirection(utilizationCounter)).toBeUndefined();
  });

  test('statsFixtureAssertsFalseTrend', async function statsFixtureAssertsFalseTrend() {
    const { container } = renderNumberWidgets(server.endpoint, { counterSemantics });

    await screen.findByText(utilizationCounter.title);

    expect(server.requests).toContain('GET /api/v1/counters');
    expect(container.querySelector('[data-scan-stat="Network utilization"] [data-value]')?.textContent).toBe('1.554%');
    expect(container.querySelector('[data-scan-stat="Network utilization"] [data-delta]')).toBeNull();
    expect(container.querySelector('[data-scan-stat="Coin price increase"] [data-delta="up"]')?.textContent).toBe('42%');
    expect(container.querySelector('[data-scan-stat="Coin price decrease"] [data-delta="down"]')?.textContent).toBe('-7.4%');
    expect(getCounterDeltaDirection(positiveChangeCounter, 'change')).toBe('up');
    expect(getCounterDeltaDirection(negativeChangeCounter, 'change')).toBe('down');
    expect(getCounterDeltaDirection({ ...negativeChangeCounter, units: 'PAX' }, 'change')).toBe('down');
    expect(getCounterDeltaDirection({ ...positiveChangeCounter, value: 'NaN' }, 'change')).toBeUndefined();
    expect(getCounterDeltaDirection({ ...positiveChangeCounter, value: 'Infinity' }, 'change')).toBeUndefined();
  });
});

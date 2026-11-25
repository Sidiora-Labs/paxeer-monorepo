import { assertControlEdges } from '@/core/app-foundation/control-edges.test-support';
// @vitest-environment jsdom
import React, { act } from 'react';
import { beforeAll, describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { createRoot, type Root } from 'react-dom/client';
import type { ChartFreshness } from '@/widgets/trading/udf-datafeed';
import { ChartToolbar, freshnessLabel, type ChartToolbarProps } from './chart-toolbar';
import { ChartSplit } from './chart-split';
import { CHART_DEFAULT_HEIGHT, CHART_MAX_HEIGHT, CHART_MIN_HEIGHT } from './use-chart-settings';

beforeAll(() => { (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true; });

const BORDER = /\b(border(?!-0\b|-none\b)(-[a-z0-9/\[\]-]+)?|divide-[a-z]|ring-(?!offset)[a-z0-9]|outline-(?!none|focus|2|offset)[a-z]+)\b/;
const live: ChartFreshness = { sourceObservedAt: '2026-10-12T12:00:00.000Z', lastReceivedAt: Date.parse('2026-10-12T12:00:00.000Z'), lastSourceHeight: '4242', connectionStatus: 'live', lagReason: null };
const noop = () => {};

function props(overrides: Partial<ChartToolbarProps> = {}): ChartToolbarProps {
  return { interval: '5', onIntervalChange: noop, mode: 'price', onModeChange: noop, mcapDisabledReason: null, markers: true, onMarkersChange: noop, markerActors: [], freshness: live, onRetry: noop, ...overrides };
}

function mount(node: React.ReactElement): { root: Root; container: HTMLDivElement } {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  act(() => root.render(node));
  return { root, container };
}
function unmount(root: Root, container: HTMLDivElement) { act(() => root.unmount()); container.remove(); }

function expectRetryVisibility(status: Element, visible: boolean) {
  const retry = status.querySelector('button');
  expect(retry?.textContent).toBe('Try again');
  expect(retry?.classList.contains('invisible')).toBe(!visible);
  expect(retry?.hasAttribute('inert')).toBe(!visible);
  expect(retry?.getAttribute('aria-hidden')).toBe(String(!visible));
}

function chart_retry_keeps_its_element_when_updates_resume() {
  const { root, container } = mount(<ChartToolbar {...props({ freshness: { ...live, connectionStatus: 'degraded' } })} />);
  try {
    const status = container.querySelector('[role="status"]')!;
    const retry = status.querySelector('button');
    expectRetryVisibility(status, true);
    act(() => root.render(<ChartToolbar {...props()} />));
    expect(status.querySelector('button')).toBe(retry);
    expectRetryVisibility(status, false);
    act(() => root.render(<ChartToolbar {...props({ freshness: { ...live, connectionStatus: 'offline' } })} />));
    expect(status.querySelector('button')).toBe(retry);
    expectRetryVisibility(status, true);
  } finally {
    unmount(root, container);
  }
}

describe('ChartToolbar', () => {
  it(chart_retry_keeps_its_element_when_updates_resume);
  it('shows the five inline intervals, a caret for the rest, Price/MCap, markers and a USDL unit without a SOL toggle', () => {
    const html = renderToStaticMarkup(<ChartToolbar {...props()} />);
    const group = html.match(/aria-label="Candle interval".*?<\/div>/s)?.[0] ?? '';
    for (const label of ['1m', '5m', '1h', '4h', 'D']) expect(group).toContain(`>${label}<`);
    expect(group).not.toContain('>15m<');
    expect(html).toContain('aria-label="More intervals"');
    expect(html).toContain('>Price<');
    expect(html).toContain('>MCap<');
    expect(html).toContain('role="switch"');
    expect(html).toContain('>USDL<');
    expect(html).not.toMatch(/\bSOL\b/);
    expect(html).toContain('Live');
    expect(html).toContain('Block 4242');
    expect(assertControlEdges(html)).not.toMatch(BORDER);
  });

  it('promotes a caret interval into the group while it is active', () => {
    const html = renderToStaticMarkup(<ChartToolbar {...props({ interval: '1W' })} />);
    const group = html.match(/aria-label="Candle interval".*?<\/div>/s)?.[0] ?? '';
    expect(group).toContain('>1W<');
    expect(group).toMatch(/aria-checked="true"[^>]*>(<[^>]+>)*1W</);
  });

  it('disables MCap with the real reason when supply is unverified', () => {
    const reason = 'Token supply is partial; market cap needs a verified supply';
    const html = renderToStaticMarkup(<ChartToolbar {...props({ mcapDisabledReason: reason })} />);
    expect(html).toMatch(/<button[^>]*disabled[^>]*>(<[^>]+>)*MCap</);
    expect(html).toContain(`Market cap not available yet: ${reason}`);
  });

  it('lists marker actors as an avatar stack only while markers are on', () => {
    const actors = [{ wallet: '0x1234567890abcdef1234567890abcdef12345678', name: 'Ada', avatarUrl: null }, { wallet: '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd', name: '0xabcd...abcd', avatarUrl: null }];
    expect(renderToStaticMarkup(<ChartToolbar {...props({ markerActors: actors })} />)).toContain('2 traders marked on the chart');
    expect(renderToStaticMarkup(<ChartToolbar {...props({ markerActors: actors, markers: false })} />)).not.toContain('traders marked');
  });

  it('offers a retry while updates are not live and routes interval and mode changes', () => {
    const calls: string[] = [];
    const { root, container } = mount(<ChartToolbar {...props({ freshness: { ...live, connectionStatus: 'degraded', lagReason: 'No source observation within 1000 ms' }, onRetry: () => calls.push('retry'), onIntervalChange: value => calls.push(`interval:${value}`), onModeChange: value => calls.push(`mode:${value}`), onMarkersChange: value => calls.push(`markers:${value}`) })} />);
    const buttons = Array.from(container.querySelectorAll('button'));
    act(() => { buttons.find(button => button.textContent === 'Try again')?.click(); });
    act(() => { buttons.find(button => button.textContent === '4h')?.click(); });
    act(() => { buttons.find(button => button.textContent === 'MCap')?.click(); });
    act(() => { container.querySelector<HTMLButtonElement>('[role="switch"]')?.click(); });
    expect(calls).toEqual(['retry', 'interval:240', 'mode:mcap', 'markers:false']);
    expect(container.textContent).toContain('May be out of date');
    unmount(root, container);
  });

  it('labels every stream state honestly', () => {
    expect(freshnessLabel('live')).toBe('Live');
    expect(freshnessLabel('offline')).toBe('Updates paused');
    expect(freshnessLabel('replaying')).toBe('Syncing...');
    expect(freshnessLabel('resync_required')).toBe('May be out of date');
  });

  it.each(['connecting', 'snapshotting', 'replaying'] as const)('waits for trades with a neutral mark while %s', function waitsForTrades(connectionStatus) {
    const html = renderToStaticMarkup(<ChartToolbar {...props({ noTrades: true, freshness: { ...live, connectionStatus, lastSourceHeight: null } })} />);
    const view = document.createElement('div');
    view.innerHTML = html;
    const status = view.querySelector('[role="status"]')!;
    expect(status.firstElementChild?.textContent).toBe('Waiting for trades');
    expect(status.querySelector('[aria-hidden="true"]')?.className).toContain('bg-text-faint');
    expect(status.querySelector('[data-numeric]')).toBeNull();
    expectRetryVisibility(status, false);
    expect(freshnessLabel(connectionStatus)).toBe('Syncing...');
  });

  it('keeps a known source block while waiting for trades', function waitingForTradesKeepsSourceHeight() {
    const html = renderToStaticMarkup(<ChartToolbar {...props({ noTrades: true, freshness: { ...live, connectionStatus: 'snapshotting' } })} />);
    expect(html).toContain('Waiting for trades');
    expect(html).toContain('Block 4242');
    const view = document.createElement('div');
    view.innerHTML = html;
    expectRetryVisibility(view.querySelector('[role="status"]')!, false);
  });

  it.each([
    ['offline', 'Updates paused', 'bg-danger', true],
    ['live', 'Live', 'bg-success', false],
    ['degraded', 'May be out of date', 'bg-text-faint', true],
    ['resync_required', 'May be out of date', 'bg-text-faint', true],
    ['closed', 'Chart closed', 'bg-text-faint', false],
  ] as const)('keeps the %s status for a coin without trades', function noTradesKeepsOtherFeedStatuses(connectionStatus, label, mark, retry) {
    const view = document.createElement('div');
    view.innerHTML = renderToStaticMarkup(<ChartToolbar {...props({ noTrades: true, freshness: { ...live, connectionStatus, lastSourceHeight: null } })} />);
    const status = view.querySelector('[role="status"]')!;
    expect(status.textContent).toContain(label);
    expect(status.textContent).not.toContain('Waiting for trades');
    expect(status.querySelector('[aria-hidden="true"]')?.className).toContain(mark);
    expect(status.querySelector('[data-numeric]')?.textContent).toBe('Block —');
    expectRetryVisibility(status, retry);
  });

  it.each([false, undefined])('keeps connecting status when noTrades is %s', function connectingKeepsFeedStatus(noTrades) {
    const html = renderToStaticMarkup(<ChartToolbar {...props({ noTrades, freshness: { ...live, connectionStatus: 'connecting', lastSourceHeight: null } })} />);
    expect(html).toContain('Syncing...');
    expect(html).toContain('bg-text-faint');
    expect(html).toContain('Block —');
    expect(html).toContain('Try again');
    expect(html).not.toContain('Waiting for trades');
  });
});

describe('ChartSplit', () => {
  it('renders the body at the clamped height with a keyboard-resizable separator and no border utilities', () => {
    const heights: number[] = [];
    const { root, container } = mount(<ChartSplit height={CHART_MAX_HEIGHT + 500} onHeightChange={value => heights.push(value)}><span>chart</span></ChartSplit>);
    const body = container.querySelector<HTMLDivElement>('div[style]');
    expect(body?.style.height).toBe(`${CHART_MAX_HEIGHT}px`);
    const handle = container.querySelector<HTMLDivElement>('[role="separator"]');
    expect(handle?.getAttribute('aria-valuemin')).toBe(String(CHART_MIN_HEIGHT));
    expect(handle?.getAttribute('aria-valuemax')).toBe(String(CHART_MAX_HEIGHT));
    act(() => { handle?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); });
    expect(heights).toEqual([CHART_DEFAULT_HEIGHT]);
    expect(assertControlEdges(container.innerHTML)).not.toMatch(BORDER);
    unmount(root, container);
  });
});

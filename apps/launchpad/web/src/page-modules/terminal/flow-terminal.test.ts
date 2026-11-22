import { createRequire } from 'node:module';
import React, { act } from 'react';
import { afterAll, afterEach, expect, test } from 'vitest';
import { BaseError, InvalidParamsRpcError, TransactionRejectedRpcError, UserRejectedRequestError } from 'viem';
import { HttpError, Operations, SdkClient } from '@kindle/sdk';
import { serveRecorded, v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import { cancellationReviewFailure } from '@/widgets/terminal/activity-tabs/order-cancellation-review';

interface BrowserDom { window: Window & typeof globalThis & { close(): void } }
const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000', pretendToBeVisual: true });
const globals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  globals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
globals.window = dom.window;
globals.document = dom.window.document;
globals.Event = dom.window.Event;
globals.CustomEvent = dom.window.CustomEvent;
globals.IS_REACT_ACT_ENVIRONMENT = true;

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { SimilarTokensList } = await import('@/widgets/terminal/right-rail/similar-tokens-list');
const { ErrorState, UnavailableState } = await import('@/ui/primitives');

const cleanup: Array<() => void | Promise<void>> = [];
afterEach(async () => { for (const close of cleanup.splice(0).reverse()) await close(); });
afterAll(() => dom.window.close());

function mountSimilar(props: Omit<React.ComponentProps<typeof SimilarTokensList>, 'poolAddress' | 'sort'>) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const render = (next: typeof props) => act(() => root.render(React.createElement(QueryClientProvider, { client }, React.createElement(SimilarTokensList, { poolAddress: v2Reads.pools.real, sort: 'liq', ...next }))));
  render(props);
  cleanup.push(() => { act(() => root.unmount()); client.clear(); container.remove(); });
  return { container, render };
}

async function terminalSimilarCoinsEndsLoadingWhenTheSymbolIsAbsent() {
  let refreshes = 0;
  const props = { symbol: null, onRetrySymbol: () => { refreshes++; } };
  const view = mountSimilar({ ...props, symbolPending: true });
  expect(view.container.textContent).toBe('Loading...');
  view.render({ ...props, symbolPending: false });
  expect(view.container.textContent).toContain('This coin has no symbol to search for.');
  expect(view.container.textContent).not.toContain('Loading...');
  const refresh = view.container.querySelector('button');
  expect(refresh?.textContent).toBe('Refresh coin');
  act(() => refresh!.click());
  expect(refreshes).toBe(1);
  view.render({ ...props, symbol: '   ' });
  expect(view.container.textContent).toContain('Similar coins are not available yet.');
  expect(view.container.textContent).not.toContain('Loading...');
}
test(terminalSimilarCoinsEndsLoadingWhenTheSymbolIsAbsent.name, terminalSimilarCoinsEndsLoadingWhenTheSymbolIsAbsent);

async function terminalSimilarCoinsShowsTheProducerFailureAndOffersRetry() {
  const recorded = v2Reads.responses.poolDetailUnavailable;
  const replay = await serveRecorded([recorded]);
  cleanup.push(() => replay.close());
  const stream = replay.origin.replace('http:', 'ws:');
  const operations = new Operations(new SdkClient({ apiOrigin: replay.origin, identityOrigin: replay.origin, mediaOrigin: replay.origin, streamOrigin: stream, approvedStreamOrigins: [stream], production: false }));
  const pool = new URL(recorded.path, replay.origin).pathname.split('/').at(-1)!;
  const read = () => operations.call('IF_POOL_DETAIL', { path: { poolAddress: pool } });
  const failure = await read().catch((error: unknown) => error);
  expect(failure).toBeInstanceOf(HttpError);
  expect(failure).toMatchObject({ status: recorded.status, code: 'DEPENDENCY_UNAVAILABLE' });
  const readAttempts = [`GET ${recorded.path}`, `GET ${recorded.path}`, `GET ${recorded.path}`];
  expect(replay.requests).toEqual(readAttempts);
  let retried: Promise<unknown> | undefined;
  const view = mountSimilar({ symbol: null, symbolError: failure, onRetrySymbol: () => { retried = read().catch((error: unknown) => error); } });
  expect(view.container.textContent).toContain("Similar coins can't load right now.");
  expect(view.container.textContent).not.toContain('Loading...');
  expect(view.container.textContent).not.toContain('DEPENDENCY_UNAVAILABLE');
  const retry = view.container.querySelector('button');
  expect(retry?.textContent).toBe('Retry');
  act(() => retry!.click());
  expect(await retried).toBeInstanceOf(HttpError);
  expect(replay.requests).toEqual([...readAttempts, ...readAttempts]);
}
test(terminalSimilarCoinsShowsTheProducerFailureAndOffersRetry.name, terminalSimilarCoinsShowsTheProducerFailureAndOffersRetry);

function terminalInlineStatesKeepTheirRecoveryControls() {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  cleanup.push(() => { act(() => root.unmount()); container.remove(); });
  let retries = 0;
  const onRetry = () => { retries++; };
  act(() => root.render(React.createElement(ErrorState, { variant: 'inline', message: 'Orders could not load.', onRetry, retrying: true })));
  expect(container.querySelector('button')?.disabled).toBe(true);
  act(() => container.querySelector('button')!.click());
  expect(retries).toBe(0);
  act(() => root.render(React.createElement(ErrorState, { variant: 'inline', message: 'Orders could not load.', onRetry })));
  expect(container.querySelector('button')?.textContent).toBe('Retry');
  act(() => container.querySelector('button')!.click());
  expect(retries).toBe(1);
  act(() => root.render(React.createElement(UnavailableState, { variant: 'inline', title: 'This coin', operation: 'IF_POOL_DETAIL', status: v2Reads.responses.poolDetailUnavailable.status, onRetry })));
  expect(container.textContent).toContain('This coin is not available yet');
  expect(container.querySelector('button')?.textContent).toBe('Try again');
  act(() => container.querySelector('button')!.click());
  expect(retries).toBe(2);
}
test(terminalInlineStatesKeepTheirRecoveryControls.name, terminalInlineStatesKeepTheirRecoveryControls);

function terminalCancellationReopensTheReviewAfterWalletRefusal() {
  for (const rejection of [new UserRejectedRequestError(new Error('Declined')), new TransactionRejectedRpcError(new Error('Not accepted')), new InvalidParamsRpcError(new Error('Invalid transaction parameters'))]) {
    const failure = cancellationReviewFailure(new BaseError('Wallet request failed', { cause: rejection }), true, null);
    expect(failure.unresolved).toBe(false);
    expect(failure.message).toBe('The wallet did not accept the cancellation. You can try again or keep your order.');
  }
}
test(terminalCancellationReopensTheReviewAfterWalletRefusal.name, terminalCancellationReopensTheReviewAfterWalletRefusal);

function terminalCancellationKeepsAnUncertainSignatureSeparateFromARefusal() {
  const interrupted = new Error('Connection interrupted');
  expect(cancellationReviewFailure(interrupted, true, null)).toEqual({ unresolved: true, message: 'The cancellation outcome is unresolved. Check its status before trying another cancellation.' });
  expect(cancellationReviewFailure(interrupted, false, null).unresolved).toBe(false);
  const hash = '0x' + 'ab'.repeat(32);
  expect(cancellationReviewFailure(new UserRejectedRequestError(interrupted), true, hash).unresolved).toBe(true);
}
test(terminalCancellationKeepsAnUncertainSignatureSeparateFromARefusal.name, terminalCancellationKeepsAnUncertainSignatureSeparateFromARefusal);

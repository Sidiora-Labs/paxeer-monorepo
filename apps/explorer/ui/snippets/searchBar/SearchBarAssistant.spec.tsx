// @vitest-environment jsdom

import { render as baseRender } from '@testing-library/react';
import { apiResolver } from 'next/dist/server/api-utils/node/api-resolver';
import nodeFetch from 'node-fetch';
import { createServer } from 'node:http';
import React from 'react';

import type { QuickSearchResultItem } from 'types/client/search';

import * as searchFixture from 'mocks/search/index';
import assistantRoute from 'pages/api/assistant';
import { Provider } from 'toolkit/chakra/provider';
import colors from 'toolkit/theme/foundations/colors';
import semanticTokens from 'toolkit/theme/foundations/semanticTokens';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { afterAll, beforeAll, beforeEach, afterEach, expect, test, vi } from 'vitest';
import { fireEvent, wrapper as TestApp } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_ENABLED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
    NEXT_PUBLIC_ASSISTANT_ENABLED: 'true',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

const EnabledSearchBarDesktop = (await import('./SearchBarDesktop')).default;
const EnabledSearchBarMobile = (await import('./SearchBarMobile')).default;
window.__envs.NEXT_PUBLIC_ASSISTANT_ENABLED = 'false';
vi.resetModules();
const DisabledSearchBarDesktop = (await import('./SearchBarDesktop')).default;
window.__envs.NEXT_PUBLIC_ASSISTANT_ENABLED = 'true';

let origin: string;
let matches: Array<QuickSearchResultItem> = [];
const answerText = 'Paxeer X uses chain id 125.';
const server = createServer((req, res) => {
  void apiResolver(req, res, {}, { 'default': assistantRoute }, {
    dev: false,
    previewModeId: 'ask-panel-test',
    previewModeEncryptionKey: 'ask-panel-test',
    previewModeSigningKey: 'ask-panel-test',
  }, false);
});

beforeAll(async() => {
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Expected a local TCP listener');
  origin = `http://127.0.0.1:${ address.port }`;
});

afterAll(async() => {
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
});
afterEach(() => vi.unstubAllEnvs());

beforeEach(() => {
  routerState.pathname = '/';
  routerState.query = {};
  window.localStorage.clear();
  matches = [];
  vi.stubEnv('EXPLORER_ASSISTANT_BASE_URL', 'https://assistant.example/v1');
  vi.stubEnv('EXPLORER_ASSISTANT_MODEL', 'test/model');
  vi.stubEnv('EXPLORER_ASSISTANT_API_KEY', 'panel-test-secret');
  fetchMock.resetMocks();
  fetchMock.mockResponse(async(request) => {
    if (request.url === 'https://assistant.example/v1/chat/completions') {
      return {
        body: JSON.stringify({ choices: [ { message: { content: answerText } } ] }),
        headers: { 'Content-Type': 'application/json' },
      };
    }
    if (request.url.endsWith('/api/assistant')) {
      const response = await nodeFetch(`${ origin }/api/assistant`, {
        method: request.method,
        headers: { 'Content-Type': 'application/json' },
        body: await request.text(),
      });
      return { body: await response.text(), status: response.status, headers: { 'Content-Type': 'application/json' } };
    }
    return {
      body: JSON.stringify(request.url.includes('/api/v2/search/quick') ? matches : {}),
      headers: { 'Content-Type': 'application/json' },
    };
  });
});

async function loadSearch(enabled: boolean) {
  return enabled ? EnabledSearchBarDesktop : DisabledSearchBarDesktop;
}

function typeQuestion(container: HTMLElement, term: string): HTMLInputElement {
  const input = container.querySelector('input')!;
  fireEvent.focus(input);
  fireEvent.change(input, { target: { value: term } });
  return input;
}

function askRow(): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find(button => button.textContent?.startsWith('Ask:'));
}

async function askRowPresentForQuestionsAndNoMatches(): Promise<void> {
  const SearchBarDesktop = await loadSearch(true);
  const { container } = render(<SearchBarDesktop/>);
  typeQuestion(container, 'Explain this network');
  await vi.waitFor(() => expect(askRow()?.textContent).toBe('Ask: Explain this network'));
  expect(document.querySelector('[role="heading"]')?.textContent).toBe('ASK');
  matches = [ searchFixture.address1 ];
  typeQuestion(container, 'What is this address?');
  await vi.waitFor(() => expect(askRow()?.textContent).toBe('Ask: What is this address?'));
  await vi.waitFor(() => expect(document.querySelector('[data-scroll-target="cat_address"]')).not.toBeNull());
  expect(document.querySelector('[role="heading"]')?.textContent).toBe('ASK');
}
test(askRowPresentForQuestionsAndNoMatches.name, askRowPresentForQuestionsAndNoMatches, 20_000);

async function askRowAndPanelAbsentWhenDisabled(): Promise<void> {
  const SearchBarDesktop = await loadSearch(false);
  const { container } = render(<SearchBarDesktop/>);
  typeQuestion(container, 'What is Paxeer?');
  await vi.waitFor(() => expect(document.body.textContent).toContain('No results found.'));
  expect(askRow()).toBeUndefined();
  expect(Array.from(document.querySelectorAll('[role="heading"]')).some(el => el.textContent === 'ASK')).toBe(false);
  expect(document.body.textContent).not.toContain('model output, not chain data');
  expect(fetchMock.mock.calls.some(([ url ]) => String(url).endsWith('/api/assistant'))).toBe(false);
}
test(askRowAndPanelAbsentWhenDisabled.name, askRowAndPanelAbsentWhenDisabled, 20_000);

async function entityMatchesKeepOrdinarySearchWithoutAsk(): Promise<void> {
  matches = [ searchFixture.address1 ];
  const SearchBarDesktop = await loadSearch(true);
  const { container } = render(<SearchBarDesktop/>);
  typeQuestion(container, '0xb64a');
  await vi.waitFor(() => expect(document.querySelector('[data-scroll-target="cat_address"]')).not.toBeNull());
  expect(askRow()).toBeUndefined();
  expect(document.body.textContent).not.toContain('model output, not chain data');
}
test(entityMatchesKeepOrdinarySearchWithoutAsk.name, entityMatchesKeepOrdinarySearchWithoutAsk, 20_000);

async function askClickRendersRealRouteAnswerAndLabel(): Promise<void> {
  routerState.pathname = '/address/[hash]';
  routerState.query = { hash: '0x123' };
  const SearchBarDesktop = await loadSearch(true);
  const { container } = render(<SearchBarDesktop/>);
  typeQuestion(container, 'What chain is this?');
  await vi.waitFor(() => expect(askRow()).toBeDefined());
  fireEvent.click(askRow()!);
  await vi.waitFor(() => expect(document.body.textContent).toContain(answerText));
  const heading = Array.from(document.querySelectorAll('[role="heading"]')).find(el => el.textContent === 'ASK')!;
  expect(heading).toBeDefined();
  expect(heading.parentElement?.textContent).toContain('model output, not chain data · test/model');
  expect(heading.parentElement?.querySelector('[role="status"]')?.textContent).toContain(answerText);
  const sent = fetchMock.mock.calls.find(([ url ]) => String(url).endsWith('/api/assistant'))!;
  expect(String(sent[0])).toBe('/explorer/api/assistant');
  expect(sent[1]?.method).toBe('POST');
  expect(JSON.parse(String(sent[1]?.body))).toEqual({
    question: 'What chain is this?', context: { route: '/address/[hash]', entityKind: 'address', identifier: '0x123' },
  });
  expect(fetchMock.mock.calls.filter(([ url ]) => String(url).includes('/chat/completions'))).toHaveLength(1);
  expect(document.body.textContent).not.toContain('panel-test-secret');
  typeQuestion(container, 'Another question?');
  await vi.waitFor(() => expect(document.body.textContent).not.toContain(answerText));
}
test(askClickRendersRealRouteAnswerAndLabel.name, askClickRendersRealRouteAnswerAndLabel, 20_000);

async function enterSubmitsAskAndShowsUnavailableError(): Promise<void> {
  vi.stubEnv('EXPLORER_ASSISTANT_BASE_URL', '');
  const SearchBarDesktop = await loadSearch(true);
  const { container } = render(<SearchBarDesktop/>);
  const input = typeQuestion(container, 'Can you help?');
  await vi.waitFor(() => expect(askRow()).toBeDefined());
  fireEvent.submit(input.closest('form')!);
  await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent)
    .toBe('Assistant unavailable. Please try again later.'));
  expect(fetchMock.mock.calls.filter(([ url ]) => String(url).endsWith('/api/assistant'))).toHaveLength(1);
  expect(fetchMock.mock.calls.filter(([ url ]) => String(url).includes('/chat/completions'))).toHaveLength(0);
  expect(document.body.textContent).not.toContain('model output, not chain data');
}
test(enterSubmitsAskAndShowsUnavailableError.name, enterSubmitsAskAndShowsUnavailableError, 20_000);

async function mobileAskUsesTheSameRouteAndPanel(): Promise<void> {
  render(<EnabledSearchBarMobile/>);
  fireEvent.keyDown(window, { key: 'k', ctrlKey: true });
  await vi.waitFor(() => expect(document.querySelector('input')).not.toBeNull());
  typeQuestion(document.body, 'What is Paxeer?');
  await vi.waitFor(() => expect(askRow()).toBeDefined());
  fireEvent.click(askRow()!);
  await vi.waitFor(() => expect(document.body.textContent).toContain(answerText));
  expect(document.body.textContent).toContain('model output, not chain data · test/model');
  expect(Array.from(document.querySelectorAll('[role="heading"]')).some(el => el.textContent === 'ASK')).toBe(true);
}
test(mobileAskUsesTheSameRouteAndPanel.name, mobileAskUsesTheSameRouteAndPanel, 20_000);

function atPath(root: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => {
    if (typeof node !== 'object' || node === null) return undefined;
    return (node as Record<string, unknown>)[key];
  }, root);
}

function resolveColour(path: string, mode: '_light' | '_dark', visited = new Set<string>()): string {
  expect(visited.has(path)).toBe(false);
  const next = new Set(visited).add(path);
  let node = atPath(semanticTokens, path) ?? atPath({ colors }, path);
  if (typeof node === 'object' && node !== null && !('value' in node)) node = (node as Record<string, unknown>).DEFAULT;
  const value = atPath(node, 'value');
  const raw = typeof value === 'string' ? value : atPath(value, mode);
  if (typeof raw !== 'string' || !raw.length) throw new Error(`Unresolved colour ${ path } in ${ mode }`);
  const reference = /^\{(.+)\}$/.exec(raw);
  return reference ? resolveColour(reference[1], mode, next) : raw;
}

async function answerPanelResolvesTokensInBothColourModes(): Promise<void> {
  const SearchBarDesktop = await loadSearch(true);
  for (const mode of [ 'light', 'dark' ] as const) {
    const Wrapper = ({ children }: { children: React.ReactNode }) => (
      <Provider forcedTheme={ mode }><TestApp>{ children }</TestApp></Provider>
    );
    const { container, unmount } = baseRender(<SearchBarDesktop/>, { wrapper: Wrapper });
    await vi.waitFor(() => expect(document.documentElement.classList.contains(mode)).toBe(true));
    typeQuestion(container, 'What chain is this?');
    await vi.waitFor(() => expect(askRow()).toBeDefined());
    fireEvent.click(askRow()!);
    await vi.waitFor(() => expect(document.body.textContent).toContain(answerText));
    const heading = Array.from(document.querySelectorAll('[role="heading"]')).find(el => el.textContent === 'ASK')!;
    const panel = heading.parentElement!;
    expect(window.getComputedStyle(panel).backgroundColor).toBe('var(--chakra-colors-raised)');
    expect(window.getComputedStyle(panel).color).toBe('var(--chakra-colors-text-primary)');
    const disclaimer = Array.from(panel.querySelectorAll('p')).find(el => el.textContent?.startsWith('model output'))!;
    expect(window.getComputedStyle(disclaimer).color).toBe('var(--chakra-colors-text-muted)');
    for (const token of [ 'raised', 'text.primary', 'text.muted', 'hairline' ]) {
      expect(resolveColour(`colors.${ token }`, mode === 'light' ? '_light' : '_dark')).toMatch(/^#|^rgba?\(/);
    }
    unmount();
  }
}
test(answerPanelResolvesTokensInBothColourModes.name, answerPanelResolvesTokensInBothColourModes, 30_000);

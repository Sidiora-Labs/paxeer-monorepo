// @vitest-environment jsdom

import { noop } from 'es-toolkit';
import React from 'react';

import * as searchMock from 'mocks/search/index';
import { render } from 'ui/shared/layout/testWrapper';
import type { Category } from 'ui/shared/search/utils';
import { describe, it, expect, beforeEach, vi } from 'vitest';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
    NEXT_PUBLIC_ASSISTANT_ENABLED: 'true',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import useQuickSearchQuery from '../useQuickSearchQuery';
import { useSearchAssistant } from '../useSearchQuery';
import SearchBarSuggest from './SearchBarSuggest';

const TERM = '0xb64a';
const RESULTS = [ searchMock.address1, searchMock.tx1, searchMock.block1 ];
const ANSWER = { answer: 'The kernel executes transactions as they arrive.\n\nValidators check, replay and commit.', model: 'assistant/model' };
let results = RESULTS;

interface HarnessProps {
  category?: Category;
  withAssistant?: boolean;
  answer?: ReturnType<typeof useSearchAssistant>['answer'];
  error?: string;
  term?: string;
}

const Harness = ({ category, withAssistant, answer, error, term = TERM }: HarnessProps): React.JSX.Element => {
  const search = useQuickSearchQuery();
  const { debouncedSearchTerm, handleSearchTermChange, query, zetaChainCCTXQuery, externalSearchItem } = search;
  const assistant = useSearchAssistant(search);

  React.useEffect(() => {
    handleSearchTermChange(term);
  }, [ handleSearchTermChange, term ]);

  return (
    <SearchBarSuggest
      query={ query }
      zetaChainCCTXQuery={ zetaChainCCTXQuery }
      externalSearchItem={ externalSearchItem }
      searchTerm={ debouncedSearchTerm }
      onItemClick={ noop }
      category={ category }
      assistant={ withAssistant ? { ...assistant, answer, error } : undefined }
    />
  );
};

const shownCategories = (container: HTMLElement): Array<string | null> => Array.from(container.querySelectorAll('[data-scroll-target] [data-id]'))
  .map((item) => item.getAttribute('data-id'));

vi.setConfig({ testTimeout: 60_000 });

describe('SearchBarSuggest', () => {
  beforeEach(() => {
    results = RESULTS;
    fetchMock.resetMocks();
    fetchMock.mockResponse((request) => {
      const body = request.url.includes('/api/v2/search/quick') ? results : {};

      return { body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } };
    });
  });

  it('groups the results under every category they fall in when no category is chosen', async() => {
    const { container } = render(<Harness/>);

    await vi.waitFor(() => {
      expect(shownCategories(container)).toEqual([ 'address', 'transaction', 'block' ]);
    }, { timeout: 30_000, interval: 100 });
  });

  it('narrows the results and drops the category tabs to the chosen category', async() => {
    const { container } = render(<Harness category="block"/>);

    await vi.waitFor(() => {
      expect(shownCategories(container)).toEqual([ 'block' ]);
    }, { timeout: 30_000, interval: 100 });
    expect(container.querySelector('[role="tablist"]')).toBeNull();
  });

  it('reports no results when the chosen category holds none of them', async() => {
    const { container } = render(<Harness category="token"/>);

    await vi.waitFor(() => {
      expect(container.textContent).toContain('No results found.');
    }, { timeout: 30_000, interval: 100 });
    expect(shownCategories(container)).toEqual([]);
  });

  it('shows the answer and model without an empty results message', async function answerReplacesEmptyResults() {
    results = [];
    const { container, findByRole } = render(<Harness withAssistant answer={ ANSWER } term="What does the kernel do?"/>);
    const question = await findByRole('button', { name: 'Ask: What does the kernel do?' });
    await vi.waitFor(() => expect(question.hasAttribute('disabled')).toBe(false));
    await vi.waitFor(() => expect(container.textContent).not.toContain('We are searching, please wait...'));

    const region = container.querySelector('[role="status"]')!;
    expect(region.getAttribute('aria-live')).toBe('polite');
    expect(region.firstElementChild?.textContent).toBe(ANSWER.answer);
    expect(region.lastElementChild?.textContent).toBe('model output, not chain data · assistant/model');
    expect(container.textContent).not.toContain('No results found.');
    expect(shownCategories(container)).toEqual([]);
  });

  it('shows an available question without an empty results message', async function questionReplacesEmptyResults() {
    results = [];
    const { container, findByRole } = render(<Harness withAssistant term="Explain the kernel"/>);

    await findByRole('button', { name: 'Ask: Explain the kernel' });

    expect(container.textContent).not.toContain('No results found.');
    expect(shownCategories(container)).toEqual([]);
  });

  it('shows an empty results message without an assistant block', async function emptyResultsWithoutAssistant() {
    results = [];
    const { container, findByText } = render(<Harness/>);

    await findByText('No results found.');

    expect(container.querySelector('[role="status"]')).toBeNull();
    expect(shownCategories(container)).toEqual([]);
  });

  it('keeps the answer unbounded and aligns the question with its text', async function answerFlowsWithThePanel() {
    const { container, findByRole } = render(<Harness withAssistant answer={ ANSWER }/>);
    const question = await findByRole('button', { name: `Ask: ${ TERM }` });
    const region = container.querySelector<HTMLElement>('[role="status"]')!;
    const regionStyle = window.getComputedStyle(region);
    const answerStyle = window.getComputedStyle(region.firstElementChild!);
    const questionStyle = window.getComputedStyle(question);

    expect(regionStyle.maxHeight).toBe('');
    expect(regionStyle.height).toBe('');
    expect(regionStyle.overflowY).toBe('');
    expect(answerStyle.maxWidth).toBe('72ch');
    expect(answerStyle.whiteSpace).toBe('pre-wrap');
    expect(answerStyle.overflowWrap).toBe('anywhere');
    expect(questionStyle.paddingInline).toBe('var(--chakra-spacing-0)');
    expect(questionStyle.minHeight).toBe('var(--chakra-sizes-11)');
    expect(window.getComputedStyle(region.parentElement!).flexShrink).toBe('0');
  });

  it('shows an assistant error without an empty results message', async function errorReplacesEmptyResults() {
    results = [];
    const { container, findByRole } = render(<Harness withAssistant error="Assistant unavailable. Please try again later."/>);

    expect((await findByRole('alert')).textContent).toBe('Assistant unavailable. Please try again later.');
    await vi.waitFor(() => expect(container.textContent).not.toContain('We are searching, please wait...'));
    expect(container.textContent).not.toContain('No results found.');
  });

  it('keeps the block height countdown alongside the assistant', async function blockCountdownRemainsAvailable() {
    results = [];
    const { container, findByRole } = render(<Harness withAssistant term="999999999"/>);

    const countdown = await findByRole('link', { name: 'estimated time for this block' });

    expect(countdown.getAttribute('href')).toContain('/block/countdown/999999999');
    await findByRole('button', { name: 'Ask: 999999999' });
    expect(container.textContent).not.toContain('No results found.');
  });
});

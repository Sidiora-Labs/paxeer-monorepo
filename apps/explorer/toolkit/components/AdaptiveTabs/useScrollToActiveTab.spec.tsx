// @vitest-environment jsdom

import { act, cleanup, render } from '@testing-library/react';
import React from 'react';

import { afterEach, beforeEach, expect, test, vi } from 'vitest';

import useScrollToActiveTab from './useScrollToActiveTab';

type Props = Parameters<typeof useScrollToActiveTab>[0];

function ScrollToActiveTab(props: Props) {
  useScrollToActiveTab(props);
  return null;
}

function createTabs() {
  const list = document.createElement('div');
  const tabs = [ document.createElement('button'), document.createElement('button'), document.createElement('button') ];
  list.append(...tabs);
  document.body.append(list);

  const scrollTo = vi.fn();
  Object.defineProperty(list, 'scrollTo', { value: scrollTo });
  vi.spyOn(list, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 100, 20));
  tabs.forEach((tab) => {
    vi.spyOn(tab, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 60, 20));
  });

  const props: Props = {
    activeTabIndex: 2,
    tabsRefs: tabs.map((tab) => ({ current: tab })),
    listRef: { current: list },
    isMobile: true,
  };

  return { props, scrollTo, list };
}

function advanceScrollTimer() {
  act(() => {
    vi.advanceTimersByTime(300);
  });
}

beforeEach(function useScrollTimers() {
  vi.useFakeTimers();
});

afterEach(function restoreScrollTimers() {
  cleanup();
  document.body.replaceChildren();
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

test('ignores a negative active index on mobile', function ignoresNegativeActiveIndex() {
  const { props, scrollTo } = createTabs();
  render(<ScrollToActiveTab { ...props } activeTabIndex={ -1 }/>);

  expect(vi.getTimerCount()).toBe(0);
  expect(advanceScrollTimer).not.toThrow();
  expect(scrollTo).not.toHaveBeenCalled();
});

test('ignores an active index at the list length', function ignoresIndexAtListLength() {
  const { props, scrollTo } = createTabs();
  render(<ScrollToActiveTab { ...props } activeTabIndex={ props.tabsRefs.length }/>);

  expect(vi.getTimerCount()).toBe(0);
  expect(advanceScrollTimer).not.toThrow();
  expect(scrollTo).not.toHaveBeenCalled();
});

test('ignores an active index beyond the list', function ignoresIndexBeyondList() {
  const { props, scrollTo } = createTabs();
  render(<ScrollToActiveTab { ...props } activeTabIndex={ props.tabsRefs.length + 1 }/>);

  expect(vi.getTimerCount()).toBe(0);
  expect(advanceScrollTimer).not.toThrow();
  expect(scrollTo).not.toHaveBeenCalled();
});

test('ignores an absent active entry', function ignoresAbsentActiveEntry() {
  const { props, scrollTo } = createTabs();
  delete props.tabsRefs[props.activeTabIndex];
  render(<ScrollToActiveTab { ...props }/>);

  expect(vi.getTimerCount()).toBe(0);
  expect(advanceScrollTimer).not.toThrow();
  expect(scrollTo).not.toHaveBeenCalled();
});

test('ignores an active ref without a current element', function ignoresAbsentCurrentElement() {
  const { props, scrollTo } = createTabs();
  const { unmount } = render(<button ref={ props.tabsRefs[props.activeTabIndex] }/>);
  unmount();
  expect(props.tabsRefs[props.activeTabIndex].current).toBeNull();
  render(<ScrollToActiveTab { ...props }/>);

  expect(vi.getTimerCount()).toBe(0);
  expect(advanceScrollTimer).not.toThrow();
  expect(scrollTo).not.toHaveBeenCalled();
});

test('ignores an active entry removed before the timer fires', function ignoresEntryRemovedBeforeTimer() {
  const { props, scrollTo } = createTabs();
  render(<ScrollToActiveTab { ...props }/>);
  expect(vi.getTimerCount()).toBe(1);
  delete props.tabsRefs[props.activeTabIndex];

  expect(advanceScrollTimer).not.toThrow();
  expect(scrollTo).not.toHaveBeenCalled();
});

test('ignores an active index outside a shortened list when the timer fires', function ignoresShortenedListBeforeTimer() {
  const { props, scrollTo } = createTabs();
  render(<ScrollToActiveTab { ...props }/>);
  expect(vi.getTimerCount()).toBe(1);
  props.tabsRefs.pop();

  expect(advanceScrollTimer).not.toThrow();
  expect(scrollTo).not.toHaveBeenCalled();
});

test('ignores an active element removed before the timer fires', function ignoresElementRemovedBeforeTimer() {
  const { props, scrollTo } = createTabs();
  const { unmount } = render(<button ref={ props.tabsRefs[props.activeTabIndex] }/>);
  render(<ScrollToActiveTab { ...props }/>);
  expect(vi.getTimerCount()).toBe(1);
  unmount();
  expect(props.tabsRefs[props.activeTabIndex].current).toBeNull();

  expect(advanceScrollTimer).not.toThrow();
  expect(scrollTo).not.toHaveBeenCalled();
});

test('scrolls to a valid active tab after the delay', function scrollsToValidActiveTab() {
  const { props, scrollTo } = createTabs();
  render(<ScrollToActiveTab { ...props }/>);

  act(() => {
    vi.advanceTimersByTime(299);
  });
  expect(scrollTo).not.toHaveBeenCalled();
  act(() => {
    vi.advanceTimersByTime(1);
  });
  expect(scrollTo).toHaveBeenCalledExactlyOnceWith({ left: 120, behavior: 'smooth' });
});

test('scrolls to the start for an active tab within the first page', function scrollsToStartWithinFirstPage() {
  const { props, scrollTo } = createTabs();
  render(<ScrollToActiveTab { ...props } activeTabIndex={ 0 }/>);

  advanceScrollTimer();
  expect(scrollTo).toHaveBeenCalledExactlyOnceWith({ left: 0 });
});

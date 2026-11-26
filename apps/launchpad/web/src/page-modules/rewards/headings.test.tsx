import React, { act } from 'react';
import { afterAll, afterEach, expect, test } from 'vitest';
import { cleanup, close, connectWallet, expectHeading, mount, signIn } from '../heading.test-support';
import type { RewardsResponse } from '../../../../../../sdk/launchpad/packages/production-contracts/src/types.gen';

const { default: RewardsModule } = await import('./index');
afterEach(cleanup);
afterAll(close);

function rewards_without_a_wallet_has_one_visible_heading() {
  const view = mount(<RewardsModule />, '/rewards');
  expectHeading(view.container, 'Connect wallet to see your rewards');
  expect(view.container.querySelector('h1')?.className).toBe('text-[length:var(--sl-text-card-title)] leading-[var(--sl-leading-md)] font-semibold text-text');
}

async function rewards_without_a_session_and_after_sign_in_error_has_one_heading() {
  await connectWallet();
  const view = mount(<RewardsModule />, '/rewards');
  expectHeading(view.container, 'Sign in to see your rewards');
  const button = [...view.container.querySelectorAll('button')].find(node => node.textContent === 'Sign in');
  expect(button).toBeDefined();
  await act(async () => button!.click());
  await expect.poll(() => view.container.querySelector('[role="alert"]'), { timeout: 10000 }).not.toBeNull();
  expectHeading(view.container, 'Sign in to see your rewards');
}

async function rewards_loading_empty_populated_and_errors_keep_one_heading() {
  const wallet = await connectWallet();
  signIn(wallet);
  const view = mount(<RewardsModule />, '/rewards');
  expect(view.container.querySelector('[data-account-view="rewards"]')?.getAttribute('aria-busy')).toBe('true');
  expectHeading(view.container, 'Rewards');
  const query = view.queries.getQueryCache().getAll().find(item => item.queryKey.includes('rewards'))!;
  expect(query).toBeDefined();
  const page: RewardsResponse = { data: { items: [] }, generatedAt: new Date().toISOString(), nextCursor: null, quality: 'complete', requestId: crypto.randomUUID(), revision: '1', watermark: null };
  act(() => view.queries.setQueryData(query.queryKey, page));
  await act(async () => { await new Promise<void>(resolve => setTimeout(resolve, 0)); });
  expectHeading(view.container, 'Rewards');
  expect([...view.container.querySelectorAll('h2')].map(node => node.textContent)).toContain('No rewards yet');
  act(() => view.queries.setQueryData(query.queryKey, { ...page, data: { items: [{ wallet, rewardId: 'creator-fees', programId: 'Creator fees', assetAddress: wallet, claimableRaw: '1', claimedRaw: '0', earnedRaw: '1', sourcePosition: null }] } } satisfies RewardsResponse));
  await act(async () => { await new Promise<void>(resolve => setTimeout(resolve, 0)); });
  expectHeading(view.container, 'Rewards');
  expect(view.container.querySelector('[data-reward-id] h2')?.textContent).toBe('Creator fees');
  const disclosure = view.container.querySelector<HTMLElement>('summary#reward-programs');
  expect(disclosure).toBeDefined();
  act(() => disclosure!.click());
  expectHeading(view.container, 'Rewards');
  expect(view.container.querySelectorAll('[data-program] h2').length).toBeGreaterThan(0);
  for (const error of [new Error('Rewards unavailable'), new TypeError('Session required')]) {
    act(() => query.setState({ data: undefined, status: 'error', error, fetchStatus: 'idle' }));
    await act(async () => { await new Promise<void>(resolve => setTimeout(resolve, 0)); });
    expectHeading(view.container, 'Rewards');
    expect([...view.container.querySelectorAll('h2')].map(node => node.textContent)).toContain(error instanceof TypeError ? 'Sign in to see your rewards' : "Rewards can't load right now");
  }
}

test(rewards_without_a_wallet_has_one_visible_heading.name, rewards_without_a_wallet_has_one_visible_heading);
test(rewards_without_a_session_and_after_sign_in_error_has_one_heading.name, rewards_without_a_session_and_after_sign_in_error_has_one_heading, 15000);
test(rewards_loading_empty_populated_and_errors_keep_one_heading.name, rewards_loading_empty_populated_and_errors_keep_one_heading);

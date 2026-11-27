import React, { act } from 'react';
import { QueryClient } from '@tanstack/react-query';
import { afterAll, afterEach, expect, test } from 'vitest';
import { cleanup, close, connectWallet, expectHeading, mount, signIn } from '../heading.test-support';
import { queryKeys } from '@/core/query-keys';
import type { ProfileIdentity } from '@/widgets/profile/use-profile-data';

const { default: ProfileModule } = await import('./index');
const { default: ProfileView } = await import('@/widgets/profile/profile-view');
const { EmptyState, ErrorState } = await import('@/ui/primitives');
const { SignedOutState } = await import('@/ui/shared/signed-out-state');
const { default: WalletGate } = await import('@/ui/shared/wallet-gate');
afterEach(cleanup);
afterAll(close);

function profile_signed_out_has_one_visible_heading() {
  const view = mount(<ProfileModule />, '/profile');
  expectHeading(view.container, 'Sign in to see your profile');
  expect(view.container.querySelector('[data-profile-route]')?.className).not.toContain('min-h-');
  expect(view.container.querySelector('button')?.className).toContain('bg-surface-inverse');
  expect(view.container.querySelector('h1')?.className).toBe('text-[length:var(--sl-text-card-title)] leading-[var(--sl-leading-md)] font-semibold text-text');
}

async function profile_connected_loading_empty_and_error_keep_one_heading() {
  const wallet = await connectWallet();
  const view = mount(<ProfileModule />, '/profile');
  expectHeading(view.container, 'Profile');
  expect(view.container.querySelector('h1 .sr-only')?.textContent).toBe('Profile');
  act(() => view.queries.setQueryData(queryKeys.userProfile(wallet), null));
  await act(async () => { await new Promise<void>(resolve => setTimeout(resolve, 0)); });
  expectHeading(view.container, `${wallet.slice(0, 8)}...${wallet.slice(-4)}`.toLowerCase());
  const query = view.queries.getQueryCache().find({ queryKey: queryKeys.userProfile(wallet) });
  act(() => query!.setState({ status: 'error', error: new Error('Profile is unavailable'), fetchStatus: 'idle' }));
  await act(async () => { await new Promise<void>(resolve => setTimeout(resolve, 0)); });
  expectHeading(view.container, `${wallet.slice(0, 8)}...${wallet.slice(-4)}`.toLowerCase());
  const profile: ProfileIdentity = { wallet, displayName: 'Paxeer builder', bio: '', avatarAssetId: null, bannerAssetId: null, website: null, socials: {}, createdAt: null, revision: '1', creatorPools: [] };
  act(() => view.queries.setQueryData(queryKeys.userProfile(wallet), profile));
  await act(async () => { await new Promise<void>(resolve => setTimeout(resolve, 0)); });
  expectHeading(view.container, 'Paxeer builder');
  expect(view.container.querySelector('h1')?.className).toBe('truncate text-lg font-bold text-text');
  const tabs = view.container.querySelector('[role="tablist"][aria-label="Profile sections"]')!;
  expect([...tabs.querySelectorAll('[role="tab"]')].map(tab => tab.textContent)).toEqual(['Open', 'Closed', 'Replies', 'Activity']);
  expect(tabs.className).toContain('overflow-x-auto');
  const current = tabs.querySelector('[aria-selected="true"]')!;
  expect(current.querySelector('.bg-text')).not.toBeNull();
  expect(current.className).not.toContain('bg-surface-3');
  const counts = view.container.querySelector('header dl:not([aria-hidden])')!;
  expect(counts.className).toContain('flex-nowrap');
  expect(counts.className).toContain('whitespace-nowrap');
}

async function profile_rewards_tab_keeps_the_profile_heading() {
  const wallet = await connectWallet();
  signIn(wallet);
  const queries = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnMount: false, gcTime: 0 } } });
  queries.setQueryData(queryKeys.userProfile(wallet), null);
  const view = mount(<ProfileModule />, '/profile', 'tab=rewards', queries);
  expectHeading(view.container, `${wallet.slice(0, 8)}...${wallet.slice(-4)}`.toLowerCase());
  const title = [...view.container.querySelectorAll('h2')].find(node => node.textContent === 'Rewards' && !node.classList.contains('sr-only'));
  expect(title).toBeDefined();
  expect(title?.classList.contains('text-xl')).toBe(true);
}

function profile_invalid_address_has_one_heading() {
  const view = mount(<ProfileView walletAddress="invalid" />, '/profile/invalid');
  expectHeading(view.container, 'This profile does not exist');
}

function shared_title_levels_keep_their_defaults() {
  const view = mount(<><EmptyState title="Empty" /><SignedOutState title="Sign in" /><WalletGate>Account</WalletGate><ErrorState title="Please try again" /></>, '/profile');
  expect([...view.container.querySelectorAll('h3')].map(node => node.textContent)).toEqual(['Empty', 'Sign in', 'Connect your wallet to continue', 'Please try again']);
  expect(view.container.querySelectorAll('h1,h2,h4')).toHaveLength(0);
}

test(profile_signed_out_has_one_visible_heading.name, profile_signed_out_has_one_visible_heading);
test(profile_connected_loading_empty_and_error_keep_one_heading.name, profile_connected_loading_empty_and_error_keep_one_heading);
test(profile_rewards_tab_keeps_the_profile_heading.name, profile_rewards_tab_keeps_the_profile_heading);
test(profile_invalid_address_has_one_heading.name, profile_invalid_address_has_one_heading);
test(shared_title_levels_keep_their_defaults.name, shared_title_levels_keep_their_defaults);

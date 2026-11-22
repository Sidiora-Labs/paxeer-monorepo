import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import React, { act } from 'react';
import { afterAll, beforeEach, expect, test } from 'vitest';
import { privateKeyToAccount, generatePrivateKey } from 'viem/accounts';
import type { V2ContractTypes } from '@kindle/sdk';

interface JoinedProcess { role: string; ready_url: string; pid: number }
const joined = JSON.parse(readFileSync(process.env.JOINED_CONFIG_FILE!, 'utf8')) as {
  stream_application_origin: string; api_origin: string; rpc_url: string; processes: JoinedProcess[];
};
function service(role: string) {
  const entry = joined.processes.find(process => process.role === role);
  if (!entry) throw new Error(`The community flow requires the ${role} service`);
  return { ...entry, origin: new URL(entry.ready_url).origin };
}
const gateway = service('gateway');
const social = service('social');
const previousEnvironment = { ...process.env };
process.env.NEXT_PUBLIC_GATEWAY_URL = gateway.origin;
process.env.NEXT_PUBLIC_METADATA_API_URL = gateway.origin;
process.env.NEXT_PUBLIC_DATA_API_URL = joined.api_origin;
process.env.NEXT_PUBLIC_PNL_API_URL = joined.api_origin;
process.env.NEXT_PUBLIC_RPC_URL = joined.rpc_url;

interface BrowserDom { window: Window & typeof globalThis }
const require = createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => BrowserDom };
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: joined.stream_application_origin });
const globals = globalThis as unknown as Record<string, unknown>;
const keys = ['window', 'document', 'self', 'IS_REACT_ACT_ENVIRONMENT'] as const;
const descriptors = new Map(keys.map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
globals.window = dom.window;
globals.document = dom.window.document;
globals.self = dom.window;
globals.IS_REACT_ACT_ENVIRONMENT = true;

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { createConfig, http, WagmiProvider } = await import('wagmi');
const { injected } = await import('wagmi/connectors');
const { defineChain } = await import('viem');
const { PathParamsContext } = await import('next/dist/shared/lib/hooks-client-context.shared-runtime');
const { default: ConversationModule } = await import('./conversation');
const { useDmChat } = await import('@/hooks/chat/use-chat');
const { useBounties, useBounty } = await import('@/widgets/bounties/use-bounties');
const { useBountySubmissions } = await import('@/widgets/bounties/use-bounty');
const { useCreateBounty } = await import('@/widgets/bounties/use-bounty-mutations');
const { usePoolCallouts } = await import('@/core/market/use-feed');
const { queryKeys } = await import('@/core/query-keys');
const { bountiesContract } = await import('@/widgets/bounties/bounties-contract');
const { accountSession, accountEpoch, installAccountSession, clearAccountSession } = await import('@/widgets/profile/account-transport');
const { getDmMessages, reviseChatMessage } = await import('@/core/clients/chat-api');
const { disconnect } = await import('@/core/clients/chat-ws');
const { operationsFor } = await import('@/core/market/operations');
const { browserGatewaySessionClient } = await import('@/core/market/client');
const { FollowControl, CommentFeed } = await import('@/widgets/community-interactions/interaction-controls');
const { InteractionClient } = await import('@/widgets/community-interactions/interaction-client');
const { CalloutComposer } = await import('@/widgets/social/callout-composer');
const { FollowButton } = await import('@/widgets/social/follow-button');
const { fetchFollowStats, setFollowing } = await import('@/widgets/social/social-transport');
const { CreateBountyForm } = await import('@/widgets/bounties/create-bounty-form');
const { FundingStep } = await import('@/widgets/bounties/funding-step');
const { PrivacySettings } = await import('@/widgets/community-interactions/privacy-settings');
const { ConnectedModerationCases } = await import('@/widgets/community-safety/moderation-case');
const { SafetyClient, SafetyRequestError } = await import('@/widgets/community-safety/safety-client');

const chain = defineChain({ id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [joined.rpc_url] } } });
const config = createConfig({ chains: [chain], connectors: [injected()], transports: { 125: http() }, storage: null, ssr: false, multiInjectedProviderDiscovery: false });
let owner = privateKeyToAccount(generatePrivateKey());
let peer = privateKeyToAccount(generatePrivateKey());

beforeEach(() => { clearAccountSession(false); disconnect(); owner = privateKeyToAccount(generatePrivateKey()); peer = privateKeyToAccount(generatePrivateKey()); });

async function login(account = owner) {
  async function post(path: string, body: unknown) {
    const response = await fetch(gateway.origin + path, { method: 'POST', headers: { Origin: joined.stream_application_origin, 'Content-Type': 'application/json', 'Idempotency-Key': crypto.randomUUID() }, body: JSON.stringify(body) });
    const value = await response.json();
    expect(response.status, JSON.stringify(value)).toBeLessThan(300);
    return value;
  }
  const { data: { challenge } } = await post('/v2/auth/challenges', { wallet: account.address, origin: joined.stream_application_origin });
  const signature = await account.signMessage({ message: challenge.message });
  const { data } = await post('/v2/auth/sessions', { challengeId: challenge.challengeId, proof: { wallet: account.address, signature, kind: 'eoa_eip191' }, deviceLabel: 'Web browser' });
  await act(async () => {
    installAccountSession(data.session, data.accessToken, accountEpoch());
    const connector = config.connectors[0]!;
    config.setState(state => ({ ...state, status: 'connected', current: connector.uid, connections: new Map([[connector.uid, { accounts: [account.address], chainId: 125, connector }]]) }));
  });
  return data as { session: V2ContractTypes['Session']; accessToken: string };
}
interface Socket {
  on(event: 'message', listener: (raw: Buffer) => void): void;
  once(event: 'error', listener: (error: Error) => void): void;
  send(value: string): void;
  close(): void;
}
const { WebSocket } = require('ws') as { WebSocket: new (url: string, options: { headers: Record<string, string> }) => Socket };
async function produceDm(sender = peer, recipient = owner, content = 'Community conversation') {
  const socket = new WebSocket(social.origin.replace('http:', 'ws:') + '/ws', { headers: { 'X-Actor-Wallet': sender.address } });
  try {
    return await new Promise<{ id: string; conversation_id: string; sequence: string; revision: string }>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('The conversation was not persisted')), 8000);
      socket.once('error', error => { clearTimeout(timer); reject(error); });
      socket.on('message', raw => {
        const frame = JSON.parse(String(raw));
        if (frame.type === 'session_accepted') socket.send(JSON.stringify({ type: 'dm', to: recipient.address, content, client_message_id: crypto.randomUUID() }));
        if (frame.type === 'message_persisted') { clearTimeout(timer); resolve(frame); }
        if (frame.type === 'error') { clearTimeout(timer); reject(new Error(JSON.stringify(frame))); }
      });
    });
  } finally { socket.close(); }
}
async function waitFor(check: () => boolean, timeout = 12000, state?: () => unknown) {
  const deadline = Date.now() + timeout;
  while (!check()) {
    if (Date.now() > deadline) throw new Error(`The community view did not reach its expected state${state ? `: ${JSON.stringify(state())}` : ''}`);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 25)); });
  }
}
function mount(element: React.ReactNode) {
  const container = document.createElement('div'); document.body.appendChild(container);
  const root = createRoot(container);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const render = async (child: React.ReactNode) => act(async () => { root.render(React.createElement(WagmiProvider, { config, reconnectOnMount: false }, React.createElement(QueryClientProvider, { client }, child))); });
  return { container, client, render: () => render(element), async close() { await act(async () => root.unmount()); client.clear(); container.remove(); disconnect(); } };
}
let chat: ReturnType<typeof useDmChat>;
function ChatProbe({ id }: { id: string }) { chat = useDmChat(id, peer.address); return React.createElement('div', null, chat.loading ? 'Loading messages' : chat.error ?? chat.messages.map(row => row.content).join('\n')); }

async function community_conversation_returns_after_same_wallet_sign_in() {
  await login();
  const message = await produceDm();
  const view = mount(React.createElement(PathParamsContext.Provider, { value: { conversationId: message.conversation_id } }, React.createElement(ConversationModule)));
  try {
    await view.render();
    await waitFor(() => view.container.textContent!.includes('Community conversation'));
    const session = accountSession();
    await act(async () => clearAccountSession(false));
    expect(view.container.textContent).toContain('Sign in to read this conversation');
    expect(view.container.textContent).not.toContain('Community conversation');
    await login();
    expect(accountSession()).not.toBe(session);
    await waitFor(() => view.container.textContent!.includes('Community conversation'));
    expect(view.container.querySelector('#dm-composer')?.getAttribute('maxlength')).toBe('500');
    expect(view.container.querySelector('a[href="/chat"]')).not.toBeNull();
  } finally { await view.close(); }
}

async function community_session_replacement_cancels_private_reads_and_replies() {
  await login();
  const message = await produceDm(peer, owner, 'Private reply context');
  const view = mount(React.createElement(ChatProbe, { id: message.conversation_id }));
  try {
    await view.render(); await waitFor(() => !chat.loading && chat.messages.some(row => row.id === message.id));
    act(() => { chat.setReplyTo(chat.messages.find(row => row.id === message.id)!); chat.markRead(message.sequence); });
    expect(chat.replyTo?.id).toBe(message.id);
    await login();
    expect(chat.replyTo).toBeNull();
    await waitFor(() => !chat.loading && chat.messages.some(row => row.id === message.id));
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 2100)); });
    const page = await getDmMessages(message.conversation_id);
    expect(BigInt(page.readThroughSequence)).toBeLessThan(BigInt(message.sequence));
    act(() => chat.markRead(message.sequence));
    await waitFor(() => chat.unreadCount === 0);
    expect((await getDmMessages(message.conversation_id)).readThroughSequence).toBe(message.sequence);
    await act(async () => clearAccountSession(false));
    expect(chat.messages).toEqual([]); expect(chat.replyTo).toBeNull(); expect(chat.isAuthenticated).toBe(false);
  } finally { await view.close(); }
}

async function community_history_timeout_has_an_explicit_recovery() {
  await login();
  const message = await produceDm(peer, owner, 'Recovered conversation');
  const view = mount(React.createElement(ChatProbe, { id: message.conversation_id }));
  process.kill(social.pid, 'SIGSTOP');
  try {
    await view.render();
    await waitFor(() => !chat.loading && !!chat.error, 15000);
    expect(chat.messages).toEqual([]);
    process.kill(social.pid, 'SIGCONT');
    act(() => chat.retryHistory());
    await waitFor(() => !chat.loading && chat.messages.some(row => row.id === message.id));
    expect(chat.messages.find(row => row.id === message.id)?.content).toBe('Recovered conversation');
  } finally { process.kill(social.pid, 'SIGCONT'); await view.close(); }
}

async function community_message_edits_respect_the_service_character_limit() {
  await login();
  const message = await produceDm(owner, peer, 'Author message');
  const view = mount(React.createElement(ChatProbe, { id: message.conversation_id }));
  try {
    await view.render(); await waitFor(() => !chat.loading && chat.messages.some(row => row.id === message.id));
    let updated = false;
    await act(async () => { updated = await chat.reviseMessage(chat.messages.find(row => row.id === message.id)!, 'x'.repeat(501)); });
    expect(updated).toBe(false); expect(chat.error).toBe('Messages must contain between 1 and 500 characters');
    expect((await getDmMessages(message.conversation_id)).messages.find(row => row.id === message.id)?.revision).toBe('1');
    await act(async () => { updated = await chat.reviseMessage(chat.messages.find(row => row.id === message.id)!, '🙂'.repeat(500)); });
    expect(updated).toBe(true);
    const edited = chat.messages.find(row => row.id === message.id)!;
    expect(edited.content).toBe('🙂'.repeat(500)); expect(edited.revision).toBe('2');
    const removed = await reviseChatMessage(edited.id, edited.revision, '', true);
    expect(removed.deleted).toBe(true); expect(removed.content).toBe('');
  } finally { await view.close(); }
}

let bounties: ReturnType<typeof useBounties>;
let bounty: ReturnType<typeof useBounty>;
let submissions: ReturnType<typeof useBountySubmissions>;
let creation: ReturnType<typeof useCreateBounty>;
function BountyProbe({ id }: { id: string }) {
  bounties = useBounties({}); bounty = useBounty(id); submissions = useBountySubmissions(id); creation = useCreateBounty();
  const created = creation.mutation.data?.data;
  return React.createElement('div', null, bounties.items.map(row => row.title).join('\n'), created ? React.createElement(FundingStep, { bounty: created.bounty, request: created.fundingTransactionRequest }) : null);
}
async function createDraft(title: string) {
  return operationsFor(browserGatewaySessionClient()).call('IF_BOUNTY_CREATE', { body: { title, description: 'Community contribution', assetAddress: bountiesContract.usdlAddress, budgetRaw: '1000000000000000000' } });
}
async function community_creators_can_read_their_private_bounty_drafts() {
  await login(); const created = await createDraft('Community draft'); const id = created.data.bounty.bountyId;
  expect(created.data.fundingTransactionRequest).toBeNull();
  const view = mount(React.createElement(BountyProbe, { id }));
  try {
    await view.render();
    await waitFor(() => bounty.state === 'ready' && bounties.items.some(row => row.bountyId === id) && submissions.loadable.state === 'ready');
    expect(bounty.state === 'ready' && bounty.data.data.bounty.creatorWallet.toLowerCase()).toBe(owner.address.toLowerCase());
    expect(submissions.loadable.state === 'ready' && submissions.loadable.data).toEqual([]);
    await act(async () => clearAccountSession(false));
    expect(bounties.items).toEqual([]); expect(bounty.state).not.toBe('ready');
    await waitFor(() => bounties.loadable.state === 'ready');
    expect(bounties.items.some(row => row.bountyId === id)).toBe(false);
    await login(peer);
    await waitFor(() => bounty.state === 'unsupported', 12000, () => ({ bounty, queries: view.client.getQueryCache().getAll().map(query => ({ key: query.queryKey, status: query.state.status, fetching: query.state.fetchStatus, error: query.state.error })) }));
    expect(bounties.items.some(row => row.bountyId === id)).toBe(false);
    await login();
    await waitFor(() => bounty.state === 'ready' && bounties.items.some(row => row.bountyId === id));
    expect(view.client.getQueryCache().getAll().filter(query => typeof query.meta?.communityScope === 'number').every(query => query.meta?.communityScope === accountEpoch())).toBe(true);
  } finally { await view.close(); }
}

async function community_bounty_creation_result_clears_with_the_session() {
  await login(); const created = await createDraft('Creation scope');
  const view = mount(React.createElement(BountyProbe, { id: created.data.bounty.bountyId }));
  try {
    await view.render();
    await act(async () => { await creation.mutation.mutateAsync({ title: 'Scoped creation', description: 'Community contribution', assetAddress: bountiesContract.usdlAddress, budgetRaw: '1000000000000000000' }); });
    await waitFor(() => creation.mutation.isSuccess);
    expect(creation.mutation.isSuccess).toBe(true);
    expect(creation.mutation.data?.data.bounty.state).toBe('draft');
    expect(creation.mutation.data?.data.fundingTransactionRequest).toBeNull();
    expect(view.container.textContent).toContain('Funding not ready yet');
    expect(view.container.querySelector(`a[href="/bounties/${creation.mutation.data!.data.bounty.bountyId}"]`)?.textContent).toBe('Open bounty');
    await login();
    expect(creation.mutation.isSuccess).toBe(false); expect(creation.mutation.data).toBeUndefined();
    expect(view.container.textContent).not.toContain('Funding not ready yet');
  } finally { await view.close(); }
}

let callouts: ReturnType<typeof usePoolCallouts>;
function CalloutProbe({ pool }: { pool: string }) { callouts = usePoolCallouts(pool); return null; }
async function community_pool_callouts_are_read_from_the_social_gateway() {
  await login();
  const chainFixture = JSON.parse(readFileSync(process.env.JOINED_CHAIN_FILE!, 'utf8'));
  const created = await operationsFor(browserGatewaySessionClient()).call('IF_CALLOUT_CREATE', { body: { body: 'Community pool update', clientCalloutId: crypto.randomUUID(), direction: 'neutral', disclosure: 'organic', poolAddress: chainFixture.pool } });
  const view = mount(React.createElement(CalloutProbe, { pool: chainFixture.pool }));
  try {
    await view.render(); await waitFor(() => callouts.state !== 'loading');
    expect(callouts.state).toBe('ready');
    expect(callouts.state === 'ready' && callouts.data.data.items.some(row => row.calloutId === created.data.callout.calloutId)).toBe(true);
  } finally { await view.close(); }
}

async function click(view: ReturnType<typeof mount>, text: string) {
  const button = [...view.container.querySelectorAll('button')].find(button => button.textContent === text);
  expect(button).toBeDefined(); await act(async () => button!.click());
}
async function community_follow_stats_can_retry_after_a_read_timeout() {
  await login(); const view = mount(React.createElement(FollowButton, { wallet: peer.address }));
  process.kill(social.pid, 'SIGSTOP');
  try {
    await view.render(); await waitFor(() => view.container.querySelector('[role="alert"]') !== null, 15000);
    process.kill(social.pid, 'SIGCONT'); await click(view, 'Try again');
    await waitFor(() => !!view.container.querySelector('button[aria-pressed="false"]'));
    expect((await fetchFollowStats(peer.address)).isFollowing).toBe(false);
    await setFollowing(peer.address, false);
    expect((await fetchFollowStats(peer.address)).isFollowing).toBe(true);
    await setFollowing(peer.address, true);
    expect((await fetchFollowStats(peer.address)).isFollowing).toBe(false);
  } finally { process.kill(social.pid, 'SIGCONT'); await view.close(); }
}

async function community_interaction_reads_offer_recovery_after_timeouts() {
  await login(); const chainFixture = JSON.parse(readFileSync(process.env.JOINED_CHAIN_FILE!, 'utf8'));
  const client = new InteractionClient();
  const view = mount(React.createElement(React.Fragment, null, React.createElement(FollowControl, { client, wallet: peer.address }), React.createElement(CommentFeed, { client, pool: chainFixture.pool, viewer: owner.address, isCreator: false })));
  process.kill(social.pid, 'SIGSTOP');
  try {
    await view.render(); await waitFor(() => view.container.querySelectorAll('[role="alert"]').length === 2, 15000);
    expect([...view.container.querySelectorAll('button')].filter(button => button.textContent === 'Try again')).toHaveLength(2);
    process.kill(social.pid, 'SIGCONT');
    for (const button of [...view.container.querySelectorAll('button')].filter(button => button.textContent === 'Try again')) await act(async () => button.click());
    await waitFor(() => view.container.textContent!.includes('No replies yet') && view.container.querySelectorAll('[role="alert"]').length === 0);
    expect(view.container.querySelector('button[aria-pressed="false"]')?.hasAttribute('disabled')).toBe(false);
  } finally { process.kill(social.pid, 'SIGCONT'); await view.close(); }
}

async function enterValue(input: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const prototype = input.tagName === 'TEXTAREA' ? dom.window.HTMLTextAreaElement.prototype : dom.window.HTMLInputElement.prototype;
  await act(async () => {
    Object.getOwnPropertyDescriptor(prototype, 'value')!.set!.call(input, value);
    input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
}
async function community_reward_asset_stays_selected_while_its_read_reloads() {
  await login(); const chainFixture = JSON.parse(readFileSync(process.env.JOINED_CHAIN_FILE!, 'utf8'));
  const view = mount(React.createElement(CreateBountyForm));
  const api = service('api');
  try {
    await view.render();
    await enterValue(view.container.querySelector<HTMLInputElement>('input[type="search"]')!, chainFixture.pool);
    await waitFor(() => !!view.container.querySelector('ul[aria-label="Matching coins"] button'));
    await act(async () => { view.container.querySelector<HTMLButtonElement>('ul[aria-label="Matching coins"] button')!.click(); });
    await waitFor(() => [...view.container.querySelectorAll('button')].some(button => button.textContent!.startsWith('$') && !button.disabled));
    const token = [...view.container.querySelectorAll('button')].find(button => button.textContent!.startsWith('$'))!;
    await act(async () => token.click());
    expect(token.getAttribute('aria-checked')).toBe('true');
    process.kill(api.pid, 'SIGSTOP');
    act(() => { void view.client.resetQueries({ queryKey: queryKeys.poolSnapshot(chainFixture.pool.toLowerCase()) }); });
    await waitFor(() => view.container.textContent!.includes('Loading coin...'));
    const usdl = [...view.container.querySelectorAll('button')].find(button => button.textContent === 'USDL')!;
    expect(usdl.getAttribute('aria-checked')).toBe('false');
    await act(async () => { view.container.querySelector('form')!.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true })); });
    expect(view.container.textContent).toContain('Choose an available reward asset');
    expect(view.container.textContent).not.toContain('Bounty created!');
  } finally { process.kill(api.pid, 'SIGCONT'); await view.close(); }
}

async function community_callout_drafts_clear_when_the_session_is_replaced() {
  await login(); const chainFixture = JSON.parse(readFileSync(process.env.JOINED_CHAIN_FILE!, 'utf8'));
  const view = mount(React.createElement(CalloutComposer, { poolAddress: chainFixture.pool }));
  try {
    await view.render();
    const editor = view.container.querySelector('textarea')!;
    await enterValue(editor, 'Private callout draft');
    expect(editor.value).toBe('Private callout draft');
    await login();
    expect(view.container.querySelector('textarea')?.value).toBe('');
    expect(view.container.textContent).not.toContain('Posted');
  } finally { await view.close(); }
}

async function community_bounty_probe_errors_keep_the_draft_and_a_retry_control() {
  await login(); const view = mount(React.createElement(CreateBountyForm));
  process.kill(social.pid, 'SIGSTOP');
  try {
    await view.render();
    const title = view.container.querySelector<HTMLInputElement>('input[required]')!;
    await enterValue(title, 'Community contribution');
    await waitFor(() => view.client.getQueryCache().getAll().some(query => query.state.status === 'error'), 15000);
    const form = view.container.querySelector('form')!;
    await act(async () => { form.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true })); });
    expect(view.container.textContent).toContain("Can't create bounties right now");
    process.kill(social.pid, 'SIGCONT');
    await click(view, 'Try again');
    await waitFor(() => {
      const probe = view.client.getQueryCache().find({ queryKey: queryKeys.bountiesProbe(), exact: true });
      return probe?.state.status === 'success' && probe.state.fetchStatus === 'idle';
    });
    await waitFor(() => view.client.getQueryCache().getAll().some(query => query.state.status === 'success'));
    await waitFor(() => !view.container.textContent!.includes("Can't create bounties right now"));
    expect(view.client.getQueryCache().getAll().some(query => query.state.status === 'success')).toBe(true);
    expect(view.container.querySelector('form')).not.toBeNull();
    expect(title.value).toBe('Community contribution');
  } finally { process.kill(social.pid, 'SIGCONT'); await view.close(); }
}

async function community_privacy_read_errors_keep_a_retry_control() {
  await login(); const view = mount(React.createElement(PrivacySettings, { client: new InteractionClient() }));
  try {
    await view.render(); await waitFor(() => view.container.querySelector('[role="alert"]') !== null);
    expect(view.container.textContent).toContain("Can't load your settings right now");
    await click(view, 'Try again');
    await waitFor(() => view.container.querySelector('[role="alert"]') !== null);
    expect(view.container.querySelector('select')?.disabled).toBe(true);
    expect([...view.container.querySelectorAll('button')].some(button => button.textContent === 'Try again')).toBe(true);
  } finally { await view.close(); }
}

async function community_moderation_read_errors_keep_a_retry_control() {
  await login(); const client = new SafetyClient(gateway.origin, () => accountSession()?.accessToken);
  const view = mount(React.createElement(ConnectedModerationCases, { client, canResolve: false }));
  try {
    await view.render(); await waitFor(() => view.container.textContent!.includes('Something went wrong, try again'));
    expect(view.container.textContent).not.toContain('No reports to review');
    await click(view, 'Try again');
    await waitFor(() => view.container.textContent!.includes('Something went wrong, try again'));
    expect([...view.container.querySelectorAll('button')].some(button => button.textContent === 'Try again')).toBe(true);
  } finally { await view.close(); }
}

async function community_safety_requests_end_when_the_service_stops_answering() {
  await login(); const client = new SafetyClient(gateway.origin, () => accountSession()?.accessToken);
  process.kill(gateway.pid, 'SIGSTOP');
  try {
    const pending = client.report({ targetType: 'profile', targetId: peer.address, category: 'spam', description: 'Unwanted messages' });
    await expect(pending).rejects.toMatchObject({ name: 'TimeoutError' });
  } finally { process.kill(gateway.pid, 'SIGCONT'); }
  await expect(client.cases()).rejects.toBeInstanceOf(SafetyRequestError);
}

for (const scenario of [community_conversation_returns_after_same_wallet_sign_in, community_session_replacement_cancels_private_reads_and_replies, community_history_timeout_has_an_explicit_recovery, community_message_edits_respect_the_service_character_limit, community_creators_can_read_their_private_bounty_drafts, community_bounty_creation_result_clears_with_the_session, community_pool_callouts_are_read_from_the_social_gateway, community_follow_stats_can_retry_after_a_read_timeout, community_interaction_reads_offer_recovery_after_timeouts, community_reward_asset_stays_selected_while_its_read_reloads, community_callout_drafts_clear_when_the_session_is_replaced, community_bounty_probe_errors_keep_the_draft_and_a_retry_control, community_privacy_read_errors_keep_a_retry_control, community_moderation_read_errors_keep_a_retry_control, community_safety_requests_end_when_the_service_stops_answering]) test(scenario.name, scenario, 30000);

afterAll(() => {
  clearAccountSession(false); disconnect(); dom.window.close();
  for (const key of keys) { const descriptor = descriptors.get(key); if (descriptor) Object.defineProperty(globalThis, key, descriptor); else Reflect.deleteProperty(globalThis, key); }
  for (const key of ['NEXT_PUBLIC_GATEWAY_URL', 'NEXT_PUBLIC_METADATA_API_URL', 'NEXT_PUBLIC_DATA_API_URL', 'NEXT_PUBLIC_PNL_API_URL', 'NEXT_PUBLIC_RPC_URL']) { const value = previousEnvironment[key]; if (value === undefined) delete process.env[key]; else process.env[key] = value; }
});

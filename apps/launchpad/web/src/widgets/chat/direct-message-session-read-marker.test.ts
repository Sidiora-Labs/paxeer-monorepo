import { randomBytes } from 'node:crypto';
import { createServer, type ServerResponse } from 'node:http';
import { createRequire } from 'node:module';
import type { AddressInfo } from 'node:net';
import type { Duplex } from 'node:stream';
import React, { act } from 'react';
import { afterAll, afterEach, expect, test } from 'vitest';
import type { ProfileGetResponse, Session } from '../../../../../../sdk/launchpad/packages/production-contracts/src/types.gen';

interface BrowserDom { window: Window & typeof globalThis }
const { JSDOM } = createRequire(import.meta.url)('jsdom') as {
  JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom;
};
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost:3000', pretendToBeVisual: true });
const globals = globalThis as unknown as Record<string, unknown>;
globals.window = dom.window;
globals.document = dom.window.document;
globals.self = dom.window;
globals.IS_REACT_ACT_ENVIRONMENT = true;

// Loopback peer for the production REST codecs and protocol-v2 WebSocket client.
interface FixtureSocket {
  send(data: string): void;
  on(event: 'message', listener: (data: Buffer) => void): void;
  on(event: 'close', listener: () => void): void;
}
interface FixtureSocketServer {
  handleUpgrade(request: import('node:http').IncomingMessage, socket: Duplex, head: Buffer, callback: (socket: FixtureSocket) => void): void;
  close(callback: () => void): void;
}
const { WebSocketServer } = createRequire(import.meta.url)('ws') as {
  WebSocketServer: new (options: { noServer: true; handleProtocols: (protocols: Set<string>) => string | false }) => FixtureSocketServer;
};
const owner = '0x1111111111111111111111111111111111111111' as const;
const peer = '0x2222222222222222222222222222222222222222';
const otherPeer = '0x3333333333333333333333333333333333333333';
const room = 'dm:visible-range';
interface DurableRow {
  type: 'dm'; id: string; conversation_id: string; sender: string; content: string;
  created_at: number; sequence: string; revision: string; deleted: boolean;
}
interface ConversationRow {
  id: string; peer: string; unread_count: number; last_message_at: number; read_sequence: string; muted: boolean;
}
const sessions = new Map<string, Session>();
const conversations = new Map<string, ConversationRow[]>();
const profileNames = new Map<string, string>();
const holdLists = new Set<string>();
const holdProfiles = new Set<string>();
const pending: { kind: 'list' | 'profile'; response: ServerResponse; body: unknown }[] = [];
const seen: { path: string; token: string }[] = [];
const reads: { conversation_id: string; through_sequence: string; token: string }[] = [];
const messages: DurableRow[] = [];
const tickets = new Map<string, string>();
const subscribed = new Set<FixtureSocket>();
let readThrough = '0';

function json(response: ServerResponse, status: number, value: unknown) {
  response.writeHead(status, { 'Content-Type': 'application/json' }).end(JSON.stringify(value));
}
function profileBody(wallet: string): ProfileGetResponse {
  return { data: { profile: { wallet, displayName: profileNames.get(wallet) ?? 'Recorded peer', avatarAssetId: null, bannerAssetId: null, bio: '', website: null, revision: '1' } }, generatedAt: new Date().toISOString(), nextCursor: null, quality: 'complete', requestId: 'chat-repair-fixture', revision: '1', watermark: null };
}
function appendMessage(sender = peer): DurableRow {
  const sequence = String(messages.length + 1);
  const message: DurableRow = { type: 'dm', id: `message-${sequence}`, conversation_id: room, sender, content: `Received message ${sequence}`, created_at: 1_760_000_000, sequence, revision: '1', deleted: false };
  messages.push(message);
  for (const socket of subscribed) socket.send(JSON.stringify(message));
  return message;
}
const server = createServer(async (request, response) => {
  const url = new URL(request.url ?? '/', 'http://fixture');
  const token = (request.headers.authorization ?? '').replace(/^Bearer /, '');
  seen.push({ path: url.pathname, token });
  if (url.pathname.startsWith('/v2/profiles/')) {
    const wallet = url.pathname.slice('/v2/profiles/'.length);
    const body = profileBody(wallet);
    if (holdProfiles.has(wallet)) { pending.push({ kind: 'profile', response, body }); return; }
    return json(response, 200, body);
  }
  const session = sessions.get(token);
  if (!session || Date.parse(session.accessExpiresAt) <= Date.now()) return json(response, 401, { code: 'AUTH_EXPIRED' });
  if (url.pathname === '/v2/chat/conversations') {
    const body = { conversations: conversations.get(token) ?? [] };
    if (holdLists.has(token)) { pending.push({ kind: 'list', response, body }); return; }
    return json(response, 200, body);
  }
  if (url.pathname === '/v2/chat/history') {
    if (url.searchParams.get('conversation_id') !== room) return json(response, 404, { code: 'NOT_FOUND' });
    const cursor = Number(url.searchParams.get('cursor') ?? '0');
    const page = messages.slice(cursor, cursor + 50);
    return json(response, 200, { messages: page, next_cursor: String(cursor + page.length), read_through_sequence: readThrough, reset_required: false });
  }
  if (url.pathname === '/v2/chat/read' && request.method === 'POST') {
    let body = '';
    for await (const chunk of request) body += String(chunk);
    const marker: { conversation_id: string; through_sequence: string } = JSON.parse(body);
    if (marker.conversation_id !== room || !messages.some(message => message.sequence === marker.through_sequence)) return json(response, 400, { code: 'INVALID_MARKER' });
    reads.push({ ...marker, token });
    if (BigInt(marker.through_sequence) > BigInt(readThrough)) readThrough = marker.through_sequence;
    for (const socket of subscribed) socket.send(JSON.stringify({ type: 'read_marker', ...marker }));
    return json(response, 200, {});
  }
  if (url.pathname === '/v2/stream/tickets' && request.method === 'POST') {
    let body = '';
    for await (const chunk of request) body += String(chunk);
    const binding: { topics: string[]; schemaMajor: number; endpoint: string; origin: string } = JSON.parse(body);
    if (binding.schemaMajor !== 2 || binding.endpoint !== '/v2/stream' || binding.origin !== window.location.origin || binding.topics.join(',') !== `private.chat.${owner}`) return json(response, 403, { code: 'SCOPE_DENIED' });
    const ticket = randomBytes(32).toString('hex');
    tickets.set(ticket, binding.topics.join(','));
    return json(response, 200, { data: { ticket, expiresAt: new Date(Date.now() + 30_000).toISOString(), subprotocol: 'sidiora.v2' } });
  }
  return json(response, 404, { code: 'NOT_FOUND' });
});
const socketServer = new WebSocketServer({ noServer: true, handleProtocols: protocols => protocols.has('sidiora.v2') ? 'sidiora.v2' : false });
server.on('upgrade', (request, socket, head) => {
  const url = new URL(request.url ?? '/', 'http://fixture');
  const protocols = String(request.headers['sec-websocket-protocol']).split(',').map(value => value.trim());
  const ticket = protocols.find(value => value.startsWith('ticket.'))?.slice(7) ?? '';
  const topics = tickets.get(ticket);
  tickets.delete(ticket);
  if (url.pathname !== '/v2/stream' || !topics || topics !== url.searchParams.get('topics')) { socket.end('HTTP/1.1 401 Unauthorized\r\nConnection: close\r\n\r\n'); return; }
  socketServer.handleUpgrade(request, socket, head, ws => {
    ws.send(JSON.stringify({ type: 'session_accepted', protocol_version: 2 }));
    ws.on('message', data => {
      const frame: { type: string } = JSON.parse(String(data));
      if (frame.type === 'subscribe_dms') { subscribed.add(ws); ws.send(JSON.stringify({ type: 'subscribed_dms' })); }
    });
    ws.on('close', () => subscribed.delete(ws));
  });
});
const origin = await new Promise<string>(resolve => server.listen(0, '127.0.0.1', () => resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`)));
const previousGateway = process.env.NEXT_PUBLIC_GATEWAY_URL;
process.env.NEXT_PUBLIC_GATEWAY_URL = origin;

const { createRoot } = await import('react-dom/client');
const { createConfig, http, WagmiProvider } = await import('wagmi');
const { injected } = await import('wagmi/connectors');
const { defineChain } = await import('viem');
const { default: ConversationList } = await import('./conversation-list');
const { default: DmThread } = await import('./dm-thread');
const { accountEpoch, accountSession, clearAccountSession, installAccountSession } = await import('@/widgets/profile/account-transport');
const chatWs = await import('@/core/clients/chat-ws');
const chain = defineChain({ id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [origin] } } });
const config = createConfig({ chains: [chain], connectors: [injected()], transports: { 125: http() }, storage: null, ssr: false, multiInjectedProviderDiscovery: false });
const connector = config.connectors[0]!;
config.setState(state => ({ ...state, status: 'connected', current: connector.uid, connections: new Map([[connector.uid, { accounts: [owner], chainId: 125, connector }]]) }));
let view: { root: ReturnType<typeof createRoot>; container: HTMLDivElement } | undefined;

function installSession(label: string, lifetime = 3_600_000, wallet = owner) {
  const session: Session = { sessionId: label, wallet, accessExpiresAt: new Date(Date.now() + lifetime).toISOString(), refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(), revokedAt: null, scopes: ['account'], deviceLabel: 'Recorded chat fixture' };
  sessions.set(label, session);
  installAccountSession(session, label, accountEpoch());
  return accountSession()!;
}
function listFor(token: string, wallet: string, unread = 3) {
  conversations.set(token, [{ id: `dm:${token}`, peer: wallet, unread_count: unread, last_message_at: 1_760_000_000, read_sequence: '0', muted: false }]);
}
function mount(node: React.ReactElement) {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  view = { root, container };
  act(() => root.render(React.createElement(WagmiProvider, { config, reconnectOnMount: false }, node)));
  return container;
}
async function waitFor(check: () => boolean) {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error(`Chat fixture timed out: ${view?.container.textContent}`);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 15)); });
  }
}
async function settle(milliseconds: number) { await act(async () => { await new Promise(resolve => setTimeout(resolve, milliseconds)); }); }
function release(kind: 'list' | 'profile') {
  for (const item of pending.filter(item => item.kind === kind)) json(item.response, 200, item.body);
  for (let index = pending.length - 1; index >= 0; index--) if (pending[index]!.kind === kind) pending.splice(index, 1);
}
function click(container: HTMLElement, text: string) {
  const button = [...container.querySelectorAll('button')].find(button => button.textContent === text);
  expect(button, `Expected ${text}`).toBeDefined();
  act(() => button!.click());
}
async function mountThread() {
  installSession('thread');
  for (let index = 1; index <= 400; index++) appendMessage(index === 400 ? owner : peer);
  const container = mount(React.createElement(DmThread, { conversationId: room, peerAddress: peer }));
  await waitFor(() => container.querySelector('[data-message-id="message-400"]') !== null && subscribed.size === 1);
  return container;
}
function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: state });
  act(() => document.dispatchEvent(new dom.window.Event('visibilitychange')));
}

async function dm_session_revocation_logout_and_same_wallet_login() {
  listFor('first', peer);
  listFor('second', otherPeer, 7);
  profileNames.set(peer, 'First private peer');
  profileNames.set(otherPeer, 'Second private peer');
  const session = installSession('first');
  const container = mount(React.createElement(ConversationList));
  await waitFor(() => container.textContent?.includes('First private peer') === true);
  expect(container.textContent).toContain('3 unread');
  act(() => clearAccountSession(false));
  expect(session.signal.aborted).toBe(true);
  expect(container.querySelector('a[href^="/chat/"]')).toBeNull();
  expect(container.textContent).toContain('Sign in to see your messages');
  act(() => { installSession('second'); });
  await waitFor(() => container.textContent?.includes('Second private peer') === true);
  expect(container.textContent).toContain('7 unread');
  expect(container.textContent).not.toContain('First private peer');
  expect(seen.filter(request => request.path === '/v2/chat/conversations').map(request => request.token)).toEqual(['first', 'second']);
}
async function dm_session_revocation_rejects_pending_list_after_replacement() {
  listFor('pending', peer);
  listFor('replacement', otherPeer);
  holdLists.add('pending');
  profileNames.set(otherPeer, 'Current private peer');
  const oldSession = installSession('pending');
  const container = mount(React.createElement(ConversationList));
  await waitFor(() => pending.some(item => item.kind === 'list'));
  act(() => { installSession('replacement'); });
  expect(oldSession.signal.aborted).toBe(true);
  await waitFor(() => container.textContent?.includes('Current private peer') === true);
  release('list');
  await settle(100);
  expect(container.querySelector('a[href="/chat/dm%3Apending"]')).toBeNull();
  expect(container.textContent).toContain('Current private peer');
  expect(seen.some(request => request.path === `/v2/profiles/${peer}`)).toBe(false);
}
async function dm_session_revocation_rejects_pending_peer_metadata() {
  listFor('old-profile', peer);
  listFor('new-profile', peer);
  profileNames.set(peer, 'Revoked display name');
  holdProfiles.add(peer);
  installSession('old-profile');
  const container = mount(React.createElement(ConversationList));
  await waitFor(() => pending.some(item => item.kind === 'profile'));
  holdProfiles.delete(peer);
  profileNames.set(peer, 'Current display name');
  act(() => { installSession('new-profile'); });
  await waitFor(() => container.textContent?.includes('Current display name') === true);
  release('profile');
  await settle(100);
  expect(container.textContent).not.toContain('Revoked display name');
  expect(container.textContent).toContain('Current display name');
}
async function dm_session_revocation_expiry_with_unchanged_wallet() {
  listFor('expiring', peer);
  profileNames.set(peer, 'Expiring private peer');
  const session = installSession('expiring', 1000);
  const container = mount(React.createElement(ConversationList));
  await waitFor(() => container.textContent?.includes('Expiring private peer') === true);
  await waitFor(() => accountSession() === null);
  expect(session.signal.aborted).toBe(true);
  expect(container.textContent).toContain('Sign in to see your messages');
  expect(container.textContent).not.toContain('Expiring private peer');
  expect(container.querySelector('a[href^="/chat/"]')).toBeNull();
}
async function dm_visible_read_marker_preserves_unseen_arrival_in_earlier_messages() {
  const container = await mountThread();
  expect(reads).toEqual([]);
  click(container, 'Earlier messages');
  expect(container.querySelector('[data-message-id="message-200"]')).not.toBeNull();
  await act(async () => { appendMessage(); });
  await waitFor(() => container.querySelector('[data-message-id="message-201"]') !== null);
  expect(container.querySelector('[data-message-id="message-401"]')).toBeNull();
  await waitFor(() => reads.length > 0);
  expect(reads).toEqual([{ conversation_id: room, through_sequence: '201', token: 'thread' }]);
  expect(readThrough).toBe('201');
  expect(messages.filter(message => message.sender !== owner && BigInt(message.sequence) > BigInt(readThrough))).toHaveLength(199);
  click(container, 'Later messages');
  expect(container.querySelector('[data-message-id="message-401"]')).not.toBeNull();
  await waitFor(() => reads.length === 2);
  expect(reads[1]).toEqual({ conversation_id: room, through_sequence: '401', token: 'thread' });
}
async function dm_visible_read_marker_cancels_when_hidden_and_marks_received_sequence() {
  const container = await mountThread();
  setVisibility('hidden');
  await settle(2200);
  expect(reads).toEqual([]);
  setVisibility('visible');
  await waitFor(() => reads.length === 1);
  // Message 400 is the viewer's own message; 399 is the highest received row.
  expect(reads).toEqual([{ conversation_id: room, through_sequence: '399', token: 'thread' }]);
  click(container, 'Earlier messages');
  await settle(2200);
  expect(reads).toHaveLength(1);
}
async function dm_visible_read_marker_cancels_on_unmount() {
  await mountThread();
  act(() => view!.root.unmount());
  view!.container.remove();
  view = undefined;
  await settle(2200);
  expect(reads).toEqual([]);
}

test(dm_session_revocation_logout_and_same_wallet_login.name, dm_session_revocation_logout_and_same_wallet_login);
test(dm_session_revocation_rejects_pending_list_after_replacement.name, dm_session_revocation_rejects_pending_list_after_replacement);
test(dm_session_revocation_rejects_pending_peer_metadata.name, dm_session_revocation_rejects_pending_peer_metadata);
test(dm_session_revocation_expiry_with_unchanged_wallet.name, dm_session_revocation_expiry_with_unchanged_wallet);
test(dm_visible_read_marker_preserves_unseen_arrival_in_earlier_messages.name, dm_visible_read_marker_preserves_unseen_arrival_in_earlier_messages, 12_000);
test(dm_visible_read_marker_cancels_when_hidden_and_marks_received_sequence.name, dm_visible_read_marker_cancels_when_hidden_and_marks_received_sequence, 12_000);
test(dm_visible_read_marker_cancels_on_unmount.name, dm_visible_read_marker_cancels_on_unmount, 8000);

afterEach(async () => {
  if (view) { act(() => view!.root.unmount()); view.container.remove(); view = undefined; }
  clearAccountSession(false);
  chatWs.disconnect();
  release('list'); release('profile');
  sessions.clear(); conversations.clear(); profileNames.clear(); holdLists.clear(); holdProfiles.clear();
  seen.length = 0; reads.length = 0; messages.length = 0; readThrough = '0';
  subscribed.clear(); tickets.clear();
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
});
afterAll(async () => {
  process.env.NEXT_PUBLIC_GATEWAY_URL = previousGateway;
  await new Promise<void>(resolve => socketServer.close(resolve));
  await new Promise<void>(resolve => { server.close(() => resolve()); server.closeAllConnections(); });
  dom.window.close();
});

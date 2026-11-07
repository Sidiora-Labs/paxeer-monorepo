import { createHash } from 'node:crypto';
import { readFileSync, statSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import { createRequire } from 'node:module';
import type { AddressInfo } from 'node:net';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import React, { act } from 'react';
import { afterAll, expect, test } from 'vitest';
import { repositoryRoot, requiredSources, sourceManifestCommand } from './qualification-sources.mjs';

function chat_qualification_paths_bind_actual_checkout() {
  const root = repositoryRoot();
  expect(repositoryRoot(dirname(fileURLToPath(import.meta.url)))).toBe(root);
  expect(statSync(join(root, 'codify.kvx')).isFile()).toBe(true);
  expect(() => repositoryRoot('/')).toThrow('repository marker codify.kvx not found');
  const wire = sourceManifestCommand('--manifest');
  const manifest: Record<string, string> = JSON.parse(wire);
  expect(Object.keys(manifest)).toEqual([...requiredSources]);
  for (const [path, digest] of Object.entries(manifest)) {
    expect(path).toMatch(/^(apps|services)\/launchpad\//);
    expect(statSync(join(root, path)).isFile()).toBe(true);
    expect(digest).toBe(createHash('sha256').update(readFileSync(join(root, path))).digest('hex'));
  }
  expect(sourceManifestCommand('--validate', wire)).toBe('');
  const qualifier = readFileSync(join(root, requiredSources[0]!), 'utf8');
  expect(qualifier).toContain("['node', str(Path(__file__).with_name('qualification-sources.mjs')), '--validate']");
  expect(qualifier).toContain("input=json.dumps(config['source_files']), text=True, check=True, timeout=30");
  expect(qualifier).toContain("if (directory / 'codify.kvx').is_file():");
  expect(qualifier).not.toMatch(/client\/apps\/web|media\/social|parents\[6\]/);
}

function chat_qualification_paths_reject_drift_and_escape() {
  const manifest: Record<string, string> = JSON.parse(sourceManifestCommand('--manifest'));
  const first = requiredSources[0]!;
  const omitted = { ...manifest };
  delete omitted[first];
  expect(() => sourceManifestCommand('--validate', JSON.stringify(omitted))).toThrow('omits a required actual code path');
  expect(() => sourceManifestCommand('--validate', JSON.stringify({ ...manifest, [first]: '0'.repeat(64) }))).toThrow('source candidate digest mismatch');
  expect(() => sourceManifestCommand('--validate', JSON.stringify({ ...manifest, '/etc/passwd': '0'.repeat(64) }))).toThrow('source candidate digest mismatch');
  expect(() => sourceManifestCommand('--validate', JSON.stringify({ ...manifest, '../../../../etc/passwd': '0'.repeat(64) }))).toThrow('source candidate digest mismatch');
  expect(() => sourceManifestCommand('--validate', JSON.stringify({ ...manifest, 'not-a-source': '0'.repeat(64) }))).toThrow();
  expect(() => sourceManifestCommand('--validate', JSON.stringify({ ...manifest, 'apps/launchpad/web/src/hooks/chat': '0'.repeat(64) }))).toThrow('source candidate digest mismatch');
}

test(chat_qualification_paths_bind_actual_checkout.name, chat_qualification_paths_bind_actual_checkout);
test(chat_qualification_paths_reject_drift_and_escape.name, chat_qualification_paths_reject_drift_and_escape);

interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url: string }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000' });
const globals = globalThis as unknown as Record<string, unknown>;
globals.window = dom.window;
globals.document = dom.window.document;
globals.IS_REACT_ACT_ENVIRONMENT = true;

const { createRoot } = await import('react-dom/client');
const { createConfig, http, WagmiProvider } = await import('wagmi');
const { injected } = await import('wagmi/connectors');
const { defineChain } = await import('viem');
const { usePoolChat, useDmChat } = await import('./use-chat');
const { accountEpoch, clearAccountSession, installAccountSession } = await import('@/widgets/profile/account-transport');
const { disconnect } = await import('@/core/clients/chat-ws');

const owner = '0x1111111111111111111111111111111111111111' as const;
const nextOwner = '0x2222222222222222222222222222222222222222' as const;
const peer = '0x3333333333333333333333333333333333333333' as const;
const firstPool = '0x4444444444444444444444444444444444444444';
const nextPool = '0x5555555555555555555555555555555555555555';
const chain = defineChain({ id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: ['http://127.0.0.1:1'] } } });
const config = createConfig({ chains: [chain], connectors: [injected()], transports: { 125: http() }, storage: null, ssr: false, multiInjectedProviderDiscovery: false });
const reads: { token: string; conversation_id: string; through_sequence: string }[] = [];
const historyRequests: string[] = [];
let server: Server | undefined;
let currentChat: ReturnType<typeof usePoolChat> | undefined;

function installIdentity(wallet: typeof owner | typeof nextOwner) {
  clearAccountSession(false);
  installAccountSession({ sessionId: `session-${wallet}`, wallet, accessExpiresAt: new Date(Date.now() + 3_600_000).toISOString(), refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(), revokedAt: null, scopes: ['account'], deviceLabel: 'Chat fixture' }, `access-${wallet}`, accountEpoch());
  const connector = config.connectors[0]!;
  config.setState(state => ({ ...state, status: 'connected', current: connector.uid, connections: new Map([[connector.uid, { accounts: [wallet], chainId: 125, connector }]]) }));
}

async function startServer() {
  if (server) return;
  const gateway = new URL(process.env.NEXT_PUBLIC_GATEWAY_URL!);
  server = createServer(async (request, response) => {
    const url = new URL(request.url!, gateway);
    const token = request.headers.authorization ?? '';
    response.setHeader('Content-Type', 'application/json');
    if (![owner, nextOwner].some(wallet => token === `Bearer access-${wallet}`)) {
      response.writeHead(401).end(JSON.stringify({ code: 'SESSION_REQUIRED' }));
    } else if (url.pathname === '/v2/chat/history') {
      const conversation = url.searchParams.get('conversation_id')!;
      historyRequests.push(conversation);
      const sequence = conversation === `pool:${firstPool}` || conversation === 'dm:first' ? '11' : '23';
      response.end(JSON.stringify({ messages: [{ id: `message-${conversation}`, conversation_id: conversation, sender: peer, content: 'Unread fixture message', created_at: 1, sequence, revision: '1', deleted: false }], next_cursor: '', read_through_sequence: '0', reset_required: false }));
    } else if (url.pathname === '/v2/chat/read' && request.method === 'POST') {
      let body = '';
      for await (const chunk of request) body += String(chunk);
      const marker: { conversation_id: string; through_sequence: string } = JSON.parse(body);
      reads.push({ token, ...marker });
      response.end('{}');
    } else if (url.pathname === '/v2/stream/tickets') {
      response.writeHead(403).end(JSON.stringify({ code: 'Chat stream unavailable in this history-only fixture' }));
    } else {
      response.writeHead(404).end(JSON.stringify({ code: 'UNRECORDED_REQUEST' }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server!.once('error', reject);
    server!.listen(Number(gateway.port), gateway.hostname, resolve);
  });
  expect((server.address() as AddressInfo).port).toBe(Number(gateway.port));
}

function PoolProbe({ target }: { target: string }) { currentChat = usePoolChat(target); return null; }
function DmProbe({ target }: { target: string }) { currentChat = useDmChat(target, peer); return null; }

async function waitFor(check: () => boolean) {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() >= deadline) throw new Error('Chat fixture did not reach the expected state');
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
  }
}

async function exerciseSwitch(kind: 'pool' | 'dm', switchWallet: boolean) {
  await startServer();
  reads.length = 0;
  historyRequests.length = 0;
  installIdentity(owner);
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const Probe = kind === 'pool' ? PoolProbe : DmProbe;
  const first = kind === 'pool' ? firstPool : 'dm:first';
  const second = switchWallet ? first : kind === 'pool' ? nextPool : 'dm:next';
  const secondConversation = kind === 'pool' ? `pool:${second}` : second;
  const render = async (target: string) => {
    await act(async () => { root.render(React.createElement(WagmiProvider, { config, reconnectOnMount: false }, React.createElement(Probe, { target }))); });
    await waitFor(() => currentChat?.loading === false && currentChat.messages[0]?.conversationId === (kind === 'pool' ? `pool:${target}` : target));
  };
  try {
    await render(first);
    expect(currentChat!.unreadCount).toBe(1);
    act(() => currentChat!.markRead());
    if (switchWallet) {
      await act(async () => { installIdentity(nextOwner); });
      await waitFor(() => currentChat?.wallet === nextOwner && !currentChat.loading && currentChat.messages.length === 1);
    } else {
      await render(second);
    }
    expect(reads).toEqual([]);
    expect(currentChat!.unreadCount).toBe(1);
    act(() => { currentChat!.markRead(); currentChat!.markRead(); });
    await waitFor(() => reads.length === 1 && currentChat?.unreadCount === 0);
    expect(reads).toEqual([{ token: `Bearer access-${switchWallet ? nextOwner : owner}`, conversation_id: secondConversation, through_sequence: second === first ? '11' : '23' }]);
    expect(historyRequests).toContain(secondConversation);
    act(() => currentChat!.markRead());
    expect(reads).toHaveLength(1);
  } finally {
    await act(async () => root.unmount());
    clearAccountSession(false);
    disconnect();
    container.remove();
  }
}

async function chat_read_timer_switches_pool_conversations() { await exerciseSwitch('pool', false); }
async function chat_read_timer_switches_dm_conversations() { await exerciseSwitch('dm', false); }
async function chat_read_timer_switches_wallets() { await exerciseSwitch('pool', true); }

test(chat_read_timer_switches_pool_conversations.name, chat_read_timer_switches_pool_conversations, 10_000);
test(chat_read_timer_switches_dm_conversations.name, chat_read_timer_switches_dm_conversations, 10_000);
test(chat_read_timer_switches_wallets.name, chat_read_timer_switches_wallets, 10_000);

afterAll(async () => {
  clearAccountSession(false);
  disconnect();
  await new Promise<void>((resolve, reject) => { if (!server) return resolve(); server.close(error => error ? reject(error) : resolve()); server.closeAllConnections(); });
  dom.window.close();
});

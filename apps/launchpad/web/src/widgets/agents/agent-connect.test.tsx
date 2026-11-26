// @vitest-environment jsdom
import { readFile } from 'node:fs/promises';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeAll, expect, test } from 'vitest';
import type { launchpadAgentInterfaceDocument } from '@/core/agent-interface';
import { assertControlEdges } from '@/core/app-foundation/control-edges.test-support';
import { AgentConnect } from './agent-connect';
import { AGENT_INTERFACE_PATH, agentClientConfiguration, type AgentConnection } from './agent-connection';

let connection: Extract<AgentConnection, { status: 'ready' }>;
let root: Root | null = null;
let host: HTMLDivElement;
const agent = { did: 'did:matrix:trader:1234567890abcdef', label: 'Trader' };

beforeAll(async () => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const configuration = process.env.JOINED_CONFIG_FILE;
  if (!configuration) throw new Error('Joined services are required');
  const joined = JSON.parse(await readFile(configuration, 'utf8')) as { browser_origin: string };
  const response = await fetch(`${joined.browser_origin}${AGENT_INTERFACE_PATH}`);
  expect(response.status).toBe(200);
  const document: ReturnType<typeof launchpadAgentInterfaceDocument> = await response.json();
  connection = { status: 'ready', entry: document.entry, environment: document.environment, endpoints: document.endpoints };
}, 60000);
afterEach(async () => {
  if (root) await act(async () => root!.unmount());
  root = null;
  host?.remove();
});

async function agentConnectShowsTheServedConfigurationWithoutAKey() {
  host = document.createElement('div'); document.body.appendChild(host); root = createRoot(host);
  await act(async () => root!.render(<AgentConnect agent={agent} connection={connection} />));
  const configuration = host.querySelector<HTMLTextAreaElement>('#agent-client-configuration')!;
  expect(configuration.readOnly).toBe(true);
  expect(configuration.value).toBe(agentClientConfiguration(connection));
  expect(JSON.parse(configuration.value)).toEqual({ mcpServers: { launchpad: {
    command: 'node', args: ['sdk/launchpad/packages/agent-kit/dist/mcp.js'],
    env: { KINDLE_GATEWAY_URL: connection.endpoints.walletGateway, KINDLE_DATA_URL: connection.endpoints.data, KINDLE_AGENT_KEY_FILE: '<AGENT_KEY_FILE>' },
  } } });
  expect(host.querySelector('a')!.getAttribute('href')).toBe(AGENT_INTERFACE_PATH);
  expect(host.querySelectorAll('li')).toHaveLength(3);
  for (const text of ['0600', 'seedHex', 'trader', 'repository root', 'agent_identity', 'list_coins', agent.did]) expect(host.textContent).toContain(text);
  expect(host.querySelectorAll('input, textarea:not([readonly])')).toHaveLength(0);
  expect(host.innerHTML).not.toMatch(/[0-9a-fA-F]{64}/);
  expect(configuration.value).not.toContain('seedHex');
  expect(configuration.value).not.toContain('access_token');
  assertControlEdges(host.innerHTML);
  expect(navigator.clipboard).toBeUndefined();
  await act(async () => host.querySelector<HTMLButtonElement>('button')!.click());
  expect(host.querySelector('[role="status"]')!.textContent).toBe('Copy is unavailable. Select the configuration and copy it manually.');
  expect(configuration.value).toBe(agentClientConfiguration(connection));
}

async function agentConnectShowsSelectionAndConfigurationStates() {
  host = document.createElement('div'); document.body.appendChild(host); root = createRoot(host);
  await act(async () => root!.render(<AgentConnect agent={null} connection={connection} />));
  expect(host.textContent).toContain('Choose an agent to connect');
  expect(host.querySelector('textarea')).toBeNull();
  await act(async () => root!.render(<AgentConnect agent={agent} connection={{ status: 'unavailable' }} />));
  expect(host.textContent).toContain('Connection details unavailable');
  expect(host.querySelector('textarea')).toBeNull();
  expect(host.querySelector('button')!.textContent).toBe('Reload connection details');
  assertControlEdges(host.innerHTML);
}

test(agentConnectShowsTheServedConfigurationWithoutAKey.name, agentConnectShowsTheServedConfigurationWithoutAKey);
test(agentConnectShowsSelectionAndConfigurationStates.name, agentConnectShowsSelectionAndConfigurationStates);

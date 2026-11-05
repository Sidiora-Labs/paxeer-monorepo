// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, expect, test, vi } from 'vitest';
import { mount } from '@/account/test/render';
import { getActiveRpcUrl, PAXEER_CONFIG, RPC_CHANGED_EVENT } from '@/lib/constants';
import { rootNetwork, virtualNetwork } from '@/lib/networks';
import { preferencesRepository } from '@/platform/storage/repositories';
import { NetworkSwitcher } from './NetworkSwitcher';

vi.hoisted(() => {
  vi.stubEnv('NEXT_PUBLIC_PAXEER_RPC_URL', 'https://rpc.example');
});

beforeEach(() => {
  localStorage.clear();
});

afterAll(() => vi.unstubAllEnvs());

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
});

function chainIdResponse(chainId: number) {
  return new Response(
    JSON.stringify({ jsonrpc: '2.0', id: 1, result: `0x${chainId.toString(16)}` }),
    {
      status: 200,
      headers: { 'content-type': 'application/json' },
    }
  );
}

async function network_switcher_adds_switches_and_removes_virtual_chains() {
  const first = virtualNetwork(125000001, 'Virtual chain 1');
  const second = virtualNetwork(125000002, 'Virtual chain 2');
  const fetchChainId = vi.fn(async (url: string, init: RequestInit) => {
    expect(init.method).toBe('POST');
    expect(JSON.parse(String(init.body))).toEqual({
      jsonrpc: '2.0',
      id: 1,
      method: 'eth_chainId',
      params: [],
    });
    if (url === first.rpcUrl) return chainIdResponse(first.chainId);
    if (url === second.rpcUrl) return chainIdResponse(second.chainId);
    throw new Error(`Unexpected RPC URL: ${url}`);
  });
  vi.stubGlobal('fetch', fetchChainId);
  const observedRpcUrls: string[] = [];
  const onRpcChanged = () => observedRpcUrls.push(getActiveRpcUrl());
  window.addEventListener(RPC_CHANGED_EVENT, onRpcChanged);
  let view = await mount(<NetworkSwitcher />);
  try {
    const rootSelector = `[data-chain-id="${rootNetwork().chainId}"] button`;
    const firstSelector = `[data-chain-id="${first.chainId}"] button`;
    expect(view.query(rootSelector)?.getAttribute('aria-pressed')).toBe('true');
    expect(view.query('button[aria-label^="Remove"]')).toBeNull();
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);

    await view.type('#virtual-chain-id', '1');
    await view.click('button[type="submit"]');
    await view.until(() => view.query(firstSelector) !== null, 'first network saved');
    expect(fetchChainId).toHaveBeenNthCalledWith(
      1,
      'https://rpc.example/chain/1',
      expect.any(Object)
    );
    expect(preferencesRepository.read().networks).toEqual([first]);
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);
    expect(view.query(firstSelector)?.getAttribute('aria-pressed')).toBe('false');

    observedRpcUrls.length = 0;
    await view.click(firstSelector);
    expect(preferencesRepository.read().activeChainId).toBe(first.chainId);
    expect(getActiveRpcUrl()).toBe('https://rpc.example/chain/1');
    expect(observedRpcUrls).toEqual(['https://rpc.example/chain/1']);
    expect(view.query(firstSelector)?.getAttribute('aria-pressed')).toBe('true');
    expect(view.query(rootSelector)?.getAttribute('aria-pressed')).toBe('false');

    await view.unmount();
    view = await mount(<NetworkSwitcher />);
    expect(view.query(firstSelector)?.getAttribute('aria-pressed')).toBe('true');
    expect(getActiveRpcUrl()).toBe(first.rpcUrl);

    await view.type('#virtual-chain-id', second.explorerUrl);
    await view.click('button[type="submit"]');
    await view.until(
      () => view.query(`[data-chain-id="${second.chainId}"]`) !== null,
      'URL network saved'
    );
    expect(fetchChainId).toHaveBeenCalledTimes(2);
    expect(preferencesRepository.read().networks).toEqual([first, second]);

    await view.click('[aria-label="Remove Virtual chain 2"]');
    expect(preferencesRepository.read().networks).toEqual([first]);
    expect(getActiveRpcUrl()).toBe(first.rpcUrl);

    await view.click(rootSelector);
    expect(preferencesRepository.read().activeChainId).toBe(rootNetwork().chainId);
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);
    await view.click(firstSelector);
    observedRpcUrls.length = 0;
    await view.click('[aria-label="Remove Virtual chain 1"]');
    expect(preferencesRepository.read().networks).toEqual([]);
    expect(preferencesRepository.read().activeChainId).toBe(rootNetwork().chainId);
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);
    expect(observedRpcUrls).toEqual([PAXEER_CONFIG.rpcUrl]);
    expect(view.query(firstSelector)).toBeNull();
    expect(view.query(rootSelector)?.getAttribute('aria-pressed')).toBe('true');
  } finally {
    window.removeEventListener(RPC_CHANGED_EVENT, onRpcChanged);
    await view.unmount();
  }
}

async function network_switcher_refuses_a_mismatched_chain_id() {
  const fetchChainId = vi.fn(async () => chainIdResponse(125000002));
  vi.stubGlobal('fetch', fetchChainId);
  const before = preferencesRepository.read();
  const observedRpcUrls: string[] = [];
  const onRpcChanged = () => observedRpcUrls.push(getActiveRpcUrl());
  window.addEventListener(RPC_CHANGED_EVENT, onRpcChanged);
  const view = await mount(<NetworkSwitcher />);
  try {
    await view.type('#virtual-chain-id', '1');
    await view.click('button[type="submit"]');
    await view.until(() => view.query('[role="alert"]') !== null, 'mismatch reported');
    expect(view.text('[role="alert"]')).toBe(
      'RPC reports chain 125000002; expected chain 125000001.'
    );
    expect(fetchChainId).toHaveBeenCalledTimes(1);
    expect(fetchChainId).toHaveBeenCalledWith('https://rpc.example/chain/1', expect.any(Object));
    expect(preferencesRepository.read()).toEqual(before);
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);
    expect(observedRpcUrls).toEqual([]);
    expect(view.query('[data-chain-id="125000001"]')).toBeNull();
    expect(view.query('button[aria-pressed="true"]')?.textContent).toContain('Paxeer');
  } finally {
    window.removeEventListener(RPC_CHANGED_EVENT, onRpcChanged);
    await view.unmount();
  }
}

test(network_switcher_adds_switches_and_removes_virtual_chains);
test(network_switcher_refuses_a_mismatched_chain_id);

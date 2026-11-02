// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, expect, test, vi } from 'vitest';
import { mount, type Mounted } from '@/account/test/render';
import { getActiveRpcUrl, PAXEER_CONFIG } from '@/lib/constants';
import { rootNetwork, virtualNetwork, type Network } from '@/lib/networks';
import { getMessages } from '@/lib/locale/messages';
import { getPriorityMessages } from '@/lib/locale/priority-messages';
import { preferencesRepository, currencyRatesRepository } from '@/platform/storage/repositories';
import { LocaleProvider } from '@/providers/LocaleProvider';
import { PWAProvider } from '@/providers/PWAProvider';
import { ThemeProvider } from '@/theme/ThemeProvider';
import { WalletProvider } from '@/wallet/WalletProvider';
import { SettingsWidget } from './SettingsWidget';

vi.hoisted(() => {
  vi.stubEnv('NEXT_PUBLIC_PAXEER_RPC_URL', 'https://rpc.example');
  vi.stubEnv('NEXT_PUBLIC_CHAIN_ID', '125');
  vi.stubEnv('NEXT_PUBLIC_EXPLORER_BASE', 'https://explorer.example');
});

// Supply jsdom's missing browser media-query API using CSS evaluation.
class CssMediaQueryList extends EventTarget implements MediaQueryList {
  onchange: MediaQueryList['onchange'] = null;

  constructor(readonly media: string) {
    super();
  }

  get matches(): boolean {
    const style = document.createElement('style');
    const probe = document.createElement('div');
    probe.id = 'settings-media-query-probe';
    style.textContent = `#${probe.id} { position: static; } @media ${this.media} { #${probe.id} { position: fixed; } }`;
    document.head.appendChild(style);
    document.body.appendChild(probe);
    try {
      return window.getComputedStyle(probe).position === 'fixed';
    } finally {
      probe.remove();
      style.remove();
    }
  }

  addListener(listener: (this: MediaQueryList, event: MediaQueryListEvent) => void): void {
    this.addEventListener('change', listener as EventListener);
  }

  removeListener(listener: (this: MediaQueryList, event: MediaQueryListEvent) => void): void {
    this.removeEventListener('change', listener as EventListener);
  }
}

const originalMatchMedia = Object.getOwnPropertyDescriptor(window, 'matchMedia');

beforeEach(() => {
  localStorage.clear();
  currencyRatesRepository.write({ base: 'USD', rates: { USD: 1 }, fetchedAt: Date.now() });
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: (query: string) => new CssMediaQueryList(query),
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
  if (originalMatchMedia) Object.defineProperty(window, 'matchMedia', originalMatchMedia);
  else Reflect.deleteProperty(window, 'matchMedia');
});

afterAll(() => vi.unstubAllEnvs());

const messages = getMessages('en');
const priorityMessages = getPriorityMessages('en');
const switcher = 'section[aria-label="Wallet networks"]';
const customRpcInput = 'input[type="url"]';

async function mountSettings() {
  const navigations: string[] = [];
  const explorerPaths: (string | undefined)[] = [];
  const view = await mount(
    <WalletProvider config={null} identity={null}>
      <LocaleProvider>
        <ThemeProvider>
          <PWAProvider>
            <SettingsWidget
              onNavigate={(route) => navigations.push(route)}
              onPaxscan={(path) => explorerPaths.push(path)}
            />
          </PWAProvider>
        </ThemeProvider>
      </LocaleProvider>
    </WalletProvider>
  );
  return { view, navigations, explorerPaths };
}

async function clickButton(view: Mounted, label: string) {
  const button = view
    .all<HTMLButtonElement>('button')
    .find((entry) => entry.textContent?.includes(label));
  expect(button, `button containing ${label}`).toBeDefined();
  button!.setAttribute('data-settings-action', 'target');
  try {
    await view.click('[data-settings-action="target"]');
  } finally {
    button!.removeAttribute('data-settings-action');
  }
}

async function openNetworkView(view: Mounted) {
  expect(view.query(switcher)).toBeNull();
  await clickButton(view, messages.settings.networkRpc);
  expect(view.text('h2')).toBe(messages.settings.networkRpc);
  expect(view.query(switcher)).not.toBeNull();
}

function expectNetworkCard(view: Mounted, network: Network) {
  const card = view.query(switcher)?.previousElementSibling;
  expect(card).not.toBeNull();
  expect(Array.from(card!.querySelectorAll('span'), (span) => span.textContent)).toEqual([
    messages.settings.network,
    network.name,
    'Chain ID',
    String(network.chainId),
  ]);
  expect(view.query(switcher)?.nextElementSibling?.querySelector('input')).toBe(
    view.query(customRpcInput)
  );
}

function chainIdResponse(chainId: number, id: string | number = 1) {
  return new Response(JSON.stringify({ jsonrpc: '2.0', id, result: `0x${chainId.toString(16)}` }), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  });
}

async function settings_network_view_adds_switches_removes_and_persists_virtual_networks() {
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
  let { view, navigations, explorerPaths } = await mountSettings();
  try {
    await clickButton(view, messages.nav.contacts);
    expect(navigations).toEqual(['contacts']);
    await clickButton(view, messages.settings.viewOnExplorer);
    expect(explorerPaths).toEqual(['/address/undefined']);
    await openNetworkView(view);
    expectNetworkCard(view, rootNetwork());
    expect(view.query(customRpcInput)).not.toBeNull();
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);

    await view.type('#virtual-chain-id', '1');
    await view.click(`${switcher} button[type="submit"]`);
    await view.until(() => view.query('[data-chain-id="125000001"]') !== null);
    expect(preferencesRepository.read().networks).toEqual([first]);
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);
    await view.click('[data-chain-id="125000001"] button');
    expect(preferencesRepository.read().activeChainId).toBe(first.chainId);
    expect(getActiveRpcUrl()).toBe(first.rpcUrl);
    expectNetworkCard(view, first);

    await view.type('#virtual-chain-id', second.explorerUrl);
    await view.click(`${switcher} button[type="submit"]`);
    await view.until(() => view.query('[data-chain-id="125000002"]') !== null);
    expect(fetchChainId).toHaveBeenCalledTimes(2);
    expect(preferencesRepository.read().networks).toEqual([first, second]);
    await view.click('[data-chain-id="125000002"] button');
    expect(preferencesRepository.read().activeChainId).toBe(second.chainId);
    expect(getActiveRpcUrl()).toBe(second.rpcUrl);
    expectNetworkCard(view, second);

    await view.click('[aria-label="Remove Virtual chain 1"]');
    expect(preferencesRepository.read().networks).toEqual([second]);
    expect(view.query('[data-chain-id="125000001"]')).toBeNull();
    expect(getActiveRpcUrl()).toBe(second.rpcUrl);
    const savedPreferences = preferencesRepository.read();
    await view.unmount();
    ({ view } = await mountSettings());
    await openNetworkView(view);
    expect(preferencesRepository.read()).toEqual(savedPreferences);
    expect(view.query('[data-chain-id="125000001"]')).toBeNull();
    expect(view.query('[data-chain-id="125000002"] button')?.getAttribute('aria-pressed')).toBe(
      'true'
    );
    expectNetworkCard(view, second);
    expect(getActiveRpcUrl()).toBe(second.rpcUrl);
    expect(fetchChainId).toHaveBeenCalledTimes(2);

    await view.click('[aria-label="Remove Virtual chain 2"]');
    expect(preferencesRepository.read().networks).toEqual([]);
    expect(preferencesRepository.read().activeChainId).toBe(rootNetwork().chainId);
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);
    expectNetworkCard(view, rootNetwork());
    expect(view.query('[data-chain-id="125000002"]')).toBeNull();
    expect(view.query('[data-chain-id="125"] button')?.getAttribute('aria-pressed')).toBe('true');
  } finally {
    await view.unmount();
  }
}

async function settings_network_view_refuses_a_mismatched_endpoint_chain_id() {
  const first = virtualNetwork(125000001, 'Virtual chain 1');
  const fetchChainId = vi.fn(async () => chainIdResponse(125000002));
  vi.stubGlobal('fetch', fetchChainId);
  const { view } = await mountSettings();
  try {
    await openNetworkView(view);
    const before = preferencesRepository.read();
    await view.type('#virtual-chain-id', '1');
    await view.click(`${switcher} button[type="submit"]`);
    await view.until(() => view.query(`${switcher} [role="alert"]`) !== null);
    expect(view.text(`${switcher} [role="alert"]`)).toBe(
      'RPC reports chain 125000002; expected chain 125000001.'
    );
    expect(fetchChainId).toHaveBeenCalledExactlyOnceWith(first.rpcUrl, expect.any(Object));
    expect(preferencesRepository.read()).toEqual(before);
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);
    expect(view.query('[data-chain-id="125000001"]')).toBeNull();
    expectNetworkCard(view, rootNetwork());
  } finally {
    await view.unmount();
  }
}

async function settings_network_view_keeps_custom_rpc_save_and_reset_controls() {
  const customRpc = 'https://custom-rpc.example/';
  const fetchChainId = vi.fn(async (url: URL, init: RequestInit) => {
    expect(url.toString()).toBe(customRpc);
    expect(init.method).toBe('POST');
    expect(JSON.parse(String(init.body))).toEqual({
      jsonrpc: '2.0',
      id: 'paxport-rpc-check',
      method: 'eth_chainId',
      params: [],
    });
    return chainIdResponse(PAXEER_CONFIG.chainId, 'paxport-rpc-check');
  });
  vi.stubGlobal('fetch', fetchChainId);
  const { view } = await mountSettings();
  try {
    await openNetworkView(view);
    await view.type(customRpcInput, customRpc);
    await clickButton(view, messages.settings.saveRpc);
    await view.until(() => preferencesRepository.read().customRpc === customRpc);
    expect(view.text('[role="status"]')).toContain(`Connected to chain ${PAXEER_CONFIG.chainId}`);
    expect(getActiveRpcUrl()).toBe(customRpc);
    expect(fetchChainId).toHaveBeenCalledTimes(1);
    await clickButton(view, priorityMessages.defaultRpc);
    expect(preferencesRepository.read().customRpc).toBeNull();
    expect(view.query<HTMLInputElement>(customRpcInput)?.value).toBe('');
    expect(getActiveRpcUrl()).toBe(PAXEER_CONFIG.rpcUrl);
    expect(view.text('[role="status"]')).toBe('Using the default Paxeer RPC');
    expectNetworkCard(view, rootNetwork());
  } finally {
    await view.unmount();
  }
}

test(settings_network_view_adds_switches_removes_and_persists_virtual_networks);
test(settings_network_view_refuses_a_mismatched_endpoint_chain_id);
test(settings_network_view_keeps_custom_rpc_save_and_reset_controls);

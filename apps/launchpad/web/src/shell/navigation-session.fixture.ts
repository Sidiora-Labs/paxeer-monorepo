import { createConfig, createStorage, http } from 'wagmi';
import { injected } from 'wagmi/connectors';
import { defineChain } from 'viem';
import { accountEpoch, clearAccountSession, installAccountSession } from '@/widgets/profile/account-transport';

export const NAVIGATION_ACCOUNT = '0x1111111111111111111111111111111111111111';

class NavigationWallet {
  readonly calls: string[] = [];
  private readonly listeners = new Map<string, Set<(...args: unknown[]) => void>>();

  async request({ method }: { method: string; params?: unknown }): Promise<unknown> {
    this.calls.push(method);
    switch (method) {
      case 'eth_chainId': return '0x7d';
      case 'eth_accounts':
      case 'eth_requestAccounts': return [NAVIGATION_ACCOUNT];
      case 'wallet_requestPermissions': return [{ parentCapability: 'eth_accounts', caveats: [{ type: 'restrictReturnedAccounts', value: [NAVIGATION_ACCOUNT] }] }];
      default: throw Object.assign(new Error(`Unsupported method ${method}`), { code: 4200 });
    }
  }

  on(event: string, listener: (...args: unknown[]) => void) {
    const listeners = this.listeners.get(event) ?? new Set();
    listeners.add(listener);
    this.listeners.set(event, listeners);
    return this;
  }

  removeListener(event: string, listener: (...args: unknown[]) => void) {
    this.listeners.get(event)?.delete(listener);
    return this;
  }
}

const chain = defineChain({ id: 125, name: 'Navigation fixture', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: ['http://127.0.0.1:1'] } } });

export function makeNavigationConfig() {
  const wallet = new NavigationWallet();
  const values = new Map<string, string>();
  const storage = createStorage({ storage: {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value); },
    removeItem: (key: string) => { values.delete(key); },
  } });
  (window as Window & { ethereum?: unknown }).ethereum = wallet;
  return createConfig({ chains: [chain], connectors: [injected()], storage, transports: { 125: http('http://127.0.0.1:1', { retryCount: 0, timeout: 500 }) }, multiInjectedProviderDiscovery: false });
}

export function installNavigationSession(wallet = NAVIGATION_ACCOUNT) {
  installAccountSession({
    accessExpiresAt: new Date(Date.now() + 3_600_000).toISOString(),
    deviceLabel: 'Web browser',
    refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(),
    revokedAt: null,
    scopes: ['account'],
    sessionId: '8f0b7a8e-3a55-4f0f-9a43-1b1d7c3f2a10',
    wallet,
  }, 'navigation-fixture-access', accountEpoch());
}

export function clearNavigationSession() {
  clearAccountSession(false);
  delete (window as Window & { ethereum?: unknown }).ethereum;
}

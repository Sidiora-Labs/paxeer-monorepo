import { act, type ComponentType } from 'react';
import { renderToString } from 'react-dom/server';
import { afterAll, expect, test, vi } from 'vitest';
import { builtinEnvironments } from 'vitest/environments';
import { cookieToInitialState, serialize } from 'wagmi';

type AppModules = {
  createWagmiConfig: typeof import('@/core/wagmi-config').createWagmiConfig;
  AppProviders: typeof import('@/ui/providers/app-providers').default;
  CreateChainModule: ComponentType;
};

async function loadApp(): Promise<AppModules> {
  const [wagmi, providers, page] = await Promise.all([
    import('@/core/wagmi-config'), import('@/ui/providers/app-providers'), import('@/page-modules/create-chain'),
  ]);
  return { createWagmiConfig: wagmi.createWagmiConfig, AppProviders: providers.default, CreateChainModule: page.default };
}

function createPage(app: AppModules, cookie: string) {
  const { AppProviders, CreateChainModule } = app;
  return <AppProviders wagmiInitialState={cookieToInitialState(app.createWagmiConfig(), cookie)}><CreateChainModule /></AppProviders>;
}

const creator = '0x1111111111111111111111111111111111111111';
const connectedCookie = `wagmi.store=${serialize({
  state: {
    chainId: 125, current: 'visitor',
    connections: new Map([['visitor', { accounts: [creator], chainId: 125, connector: { id: 'injected', name: 'Injected', type: 'injected', uid: 'visitor' } }]]),
  },
  version: 2,
})}`;

let teardown: ((global: typeof globalThis) => unknown) | undefined;
afterAll(async () => { await teardown?.(globalThis); });

async function chain_wizard_hydrates_without_a_mismatch() {
  expect(typeof window).toBe('undefined');
  const server = await loadApp();
  const fresh = renderToString(createPage(server, ''));
  const connected = renderToString(createPage(server, connectedCookie));
  const afterVisitor = renderToString(createPage(server, ''));
  expect(connected).toContain('Launch a chain');
  expect(connected).not.toContain('>Connect wallet<');
  expect(afterVisitor).toBe(fresh);
  expect(afterVisitor).toContain('>Connect wallet<');
  expect(afterVisitor).toContain('Name your chain');
  expect(afterVisitor).toContain('Use 3 to 32 lowercase letters');

  const env = await builtinEnvironments.jsdom.setup(globalThis, { jsdom: { url: 'http://localhost/create/chain' } });
  teardown = env.teardown;
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.resetModules();
  const browser = await loadApp();
  const { hydrateRoot } = await import('react-dom/client');
  const container = document.createElement('div');
  container.innerHTML = afterVisitor;
  document.body.appendChild(container);
  const errors: string[] = [];
  const recoverable: string[] = [];
  const original = console.error;
  console.error = (...args: unknown[]) => { errors.push(args.map(String).join(' ')); };
  let root: ReturnType<typeof hydrateRoot> | undefined;
  try {
    await act(async () => {
      root = hydrateRoot(container, createPage(browser, document.cookie), {
        onRecoverableError: error => { recoverable.push(error instanceof Error ? error.message : String(error)); },
      });
    });
    expect(container.querySelector('h1')?.textContent).toBe('Launch a chain');
    expect(container.querySelector<HTMLInputElement>('#chain-name')?.value).toBe('');
    expect(container.textContent).toContain('Connect wallet');
    expect(container.querySelector('li[aria-current="step"]')?.textContent).toBe('1. Name');
  } finally {
    await act(async () => root?.unmount());
    console.error = original;
    container.remove();
  }
  expect(recoverable).toEqual([]);
  expect(errors.filter(message => /hydrat|did not match|server rendered|#418/i.test(message))).toEqual([]);
}

test(chain_wizard_hydrates_without_a_mismatch, { timeout: 60_000 });

async function chain_wizard_hydrates_saved_confirmation_across_reload() {
  const browser = await loadApp();
  const { ChainDraftStorage } = await import('./chain-storage');
  const { hydrateRoot } = await import('react-dom/client');
  const storage = new ChainDraftStorage(creator);
  const txHash = `0x${'ab'.repeat(32)}` as const;
  storage.saveSubmission({
    draftId: 'interrupted', version: 1, creator, chainId: 125, name: 'saved-chain',
    tier: 'growth', feeShareBps: 0, updatedAt: Date.now(),
  }, { creator, chainId: 125, txHash, phase: 'confirm', bondRaw: '23000000000000000000' });
  const serialized = localStorage.getItem('chain-submission-v1');
  for (let reload = 0; reload < 2; reload++) {
    const markup = renderToString(createPage(browser, ''));
    expect(markup).not.toContain(txHash);
    const container = document.createElement('div');
    container.innerHTML = markup;
    document.body.appendChild(container);
    const recoverable: string[] = [];
    let root: ReturnType<typeof hydrateRoot> | undefined;
    try {
      await act(async () => {
        root = hydrateRoot(container, createPage(browser, ''), {
          onRecoverableError: error => { recoverable.push(String(error)); },
        });
      });
      expect(container.querySelector('[aria-label="Submitted transaction"]')?.textContent).toContain(txHash);
      expect(container.textContent).toContain(creator);
      expect(container.textContent).toContain('Root chain: 125');
      expect(container.querySelector('li[aria-current="step"]')?.textContent).toBe('3. Review and launch');
      const retry = Array.from(container.querySelectorAll('button')).find(node => node.textContent === 'Retry confirmation');
      expect(retry?.disabled).toBe(true);
      expect(container.textContent).toContain('belongs to a different account or chain');
      expect(container.querySelector('[aria-label="Live chain"]')).toBeNull();
      expect(localStorage.getItem('chain-submission-v1')).toBe(serialized);
      expect(recoverable).toEqual([]);
    } finally {
      await act(async () => root?.unmount());
      container.remove();
    }
  }
  localStorage.clear();
}

test(chain_wizard_hydrates_saved_confirmation_across_reload, { timeout: 60_000 });

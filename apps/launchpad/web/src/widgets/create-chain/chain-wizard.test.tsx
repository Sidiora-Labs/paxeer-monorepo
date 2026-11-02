// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeAll, expect, test, vi } from 'vitest';
import { createPublicClient, createWalletClient, custom, defineChain } from 'viem';
import ChainWizard from './chain-wizard';
import { ChainLaunchAdapter, type ChainLaunchResult, type ChainLaunchStep } from './chain-adapters';
import { walletNetwork, type ChainDraft, type ChainsParams } from './chain-domain';
import { ChainDraftStorage } from './chain-storage';

const creator = '0x1111111111111111111111111111111111111111';
const chain = defineChain({
  id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: ['https://rpc.example'] } },
  blockExplorers: { default: { name: 'Explorer', url: 'https://explorer.example' } },
});
const walletRequests = vi.fn(async () => null);
const client = createPublicClient({ chain, transport: custom({ request: walletRequests }) });
const wallet = createWalletClient({ chain, account: creator, transport: custom({ request: walletRequests }) });

vi.mock('wagmi', () => ({
  useAccount: () => ({ address: creator, chainId: 125 }),
  usePublicClient: () => client,
  useWalletClient: () => ({ data: wallet }),
  useSwitchChain: () => ({ switchChainAsync: vi.fn() }),
}));

const params: ChainsParams = [creator, 23000000000000000000n, 2000000000000000000n, 500000n, 4000000n, 10000000n, 100];
const network = walletNetwork({ id: 7, chainId: 125000007, name: 'my-chain' }, 'https://rpc.example', 'https://explorer.example');
const launched: ChainLaunchResult = {
  id: 7, chainId: 125000007, name: 'my-chain', rpcUrl: network.rpcUrls[0],
  explorerUrl: network.blockExplorerUrls[0], walletNetwork: network, txHash: `0x${'ab'.repeat(32)}`,
};

let root: Root | undefined;
let container: HTMLDivElement;

beforeAll(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

afterEach(async () => {
  await act(async () => root?.unmount());
  root = undefined;
  container?.remove();
  localStorage.clear();
  vi.restoreAllMocks();
  walletRequests.mockClear();
});

async function mount() {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => root!.render(<ChainWizard />));
}

function button(label: string): HTMLButtonElement {
  const found = Array.from(container.querySelectorAll('button')).find(node => node.textContent === label);
  expect(found, `Button ${label}`).toBeDefined();
  return found!;
}

async function click(label: string) {
  await act(async () => button(label).click());
}

async function typeName(value: string) {
  const input = container.querySelector<HTMLInputElement>('#chain-name')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

function mockReads() {
  const read = vi.spyOn(ChainLaunchAdapter.prototype, 'readParams').mockResolvedValue(params);
  const availability = vi.spyOn(ChainLaunchAdapter.prototype, 'isNameAvailable').mockImplementation(async name => name !== 'taken');
  return { read, availability };
}

async function chain_wizard_walks_three_steps_and_shows_the_live_chain() {
  const { read, availability } = mockReads();
  let report: ((step: ChainLaunchStep) => void) | undefined;
  let receipt: ((result: ChainLaunchResult) => void) | undefined;
  const launch = vi.spyOn(ChainLaunchAdapter.prototype, 'launch').mockImplementation((_draft, onStep) => {
    report = onStep;
    onStep('create');
    return new Promise(resolve => { receipt = resolve; });
  });
  const copy = vi.fn(async (_value: string) => {});
  vi.stubGlobal('navigator', Object.assign(Object.create(navigator), { clipboard: { writeText: copy } }));
  try {
    await mount();
    expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('1. Name');
    expect(container.textContent).toContain('Use 3 to 32 lowercase letters, numbers or hyphens.');
    expect(button('Next').disabled).toBe(true);
    await typeName('taken');
    expect(availability).toHaveBeenCalledWith('taken');
    expect(container.textContent).toContain('This name is taken. Choose another.');
    expect(button('Next').disabled).toBe(true);
    await click('Next');
    expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('1. Name');

    await typeName('my-chain');
    expect(container.textContent).toContain('This name is available.');
    expect(button('Next').disabled).toBe(false);
    await click('Next');
    expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('2. Size');
    expect(container.querySelectorAll('input[type="radio"]')).toHaveLength(3);
    for (const allowance of ['500,000 gas and 65,536 bytes', '1,500,000 gas and 262,144 bytes', '4,000,000 gas and 1,048,576 bytes']) {
      expect(container.textContent).toContain(allowance);
    }
    expect(read).toHaveBeenCalledTimes(1);
    expect(container.textContent).toContain('Bond: 23 PAX');
    expect(container.textContent).toContain('Creation fee: 2 PAX');
    expect(container.textContent).toContain('Total: 25 PAX');
    await act(async () => container.querySelector<HTMLInputElement>('input[value="growth"]')!.click());
    expect(new ChainDraftStorage(creator).restoreDraft()).toMatchObject({ name: 'my-chain', tier: 'growth' });
    await click('Next');
    expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('3. Review and launch');
    expect(container.textContent).toContain('my-chain · Growth');
    expect(launch).not.toHaveBeenCalled();
    await click('Launch');
    expect(launch).toHaveBeenCalledTimes(1);
    expect(launch.mock.calls[0][0]).toMatchObject({ name: 'my-chain', tier: 'growth', creator, chainId: 125, feeShareBps: 0 });
    expect(button('Launch').disabled).toBe(true);
    expect(container.querySelector('ol[aria-label="Steps"] [aria-current="step"]')?.textContent).toContain('Confirm in your wallet');
    await click('Launch');
    expect(launch).toHaveBeenCalledTimes(1);
    await act(async () => { report!('confirm'); });
    expect(container.querySelector('ol[aria-label="Steps"] [aria-current="step"]')?.textContent).toContain('Waiting for the block');
    expect(container.querySelector('[aria-label="Live chain"]')).toBeNull();
    await act(async () => { report!('ready'); receipt!(launched); });
    expect(container.querySelector('[aria-label="Live chain"]')?.textContent).toContain('Chain ID: 125000007');
    expect(container.textContent).toContain('Registry ID: 7');
    expect(container.querySelector<HTMLInputElement>('#chain-rpc')?.value).toBe(launched.rpcUrl);
    expect(container.querySelector('a')?.getAttribute('href')).toBe(launched.explorerUrl);
    expect(container.querySelector('a')?.textContent).toContain('View explorer');
    expect(Array.from(container.querySelectorAll('button')).some(node => node.textContent === 'Launch')).toBe(false);
    await click('Copy RPC URL');
    expect(copy).toHaveBeenCalledExactlyOnceWith(launched.rpcUrl);
    expect(container.textContent).toContain('RPC URL copied.');
    await click('Add to wallet');
    expect(walletRequests).toHaveBeenCalledExactlyOnceWith({ method: 'wallet_addEthereumChain', params: [launched.walletNetwork] });
    expect(container.textContent).toContain('Chain added to your wallet.');
    expect(new ChainDraftStorage(creator).restoreDraft()).toBeNull();
  } finally {
    vi.unstubAllGlobals();
  }
}

async function chain_wizard_restores_drafts_and_ignores_stale_name_checks() {
  mockReads();
  const storage = new ChainDraftStorage(creator);
  const draft: ChainDraft = { draftId: 'saved', version: 1, creator, chainId: 125, name: 'saved-chain', tier: 'pro', feeShareBps: 0, updatedAt: Date.now() };
  storage.saveDraft(draft);
  let oldCheck: ((value: boolean) => void) | undefined;
  vi.mocked(ChainLaunchAdapter.prototype.isNameAvailable).mockImplementation(name => name === 'old-name'
    ? new Promise(resolve => { oldCheck = resolve; }) : Promise.resolve(name !== 'taken'));
  await mount();
  expect(container.querySelector<HTMLInputElement>('#chain-name')?.value).toBe('saved-chain');
  await typeName('old-name');
  expect(button('Next').disabled).toBe(true);
  await typeName('taken');
  await act(async () => { oldCheck!(true); });
  expect(button('Next').disabled).toBe(true);
  expect(container.textContent).toContain('This name is taken.');
  await typeName('new-name');
  await click('Next');
  expect(container.querySelector<HTMLInputElement>('input[value="pro"]')?.checked).toBe(true);
  await click('Back');
  await typeName('Invalid Name');
  expect(button('Next').disabled).toBe(true);
  expect(container.textContent).toContain('Check the name rules above.');
  expect(new ChainDraftStorage('0x2222222222222222222222222222222222222222').restoreDraft()).toBeNull();
  localStorage.setItem(`chain-draft-v1:125:${creator}`, JSON.stringify({ ...draft, updatedAt: Date.now() - 31 * 86400000 }));
  expect(() => storage.restoreDraft()).toThrow('invalid or expired');
}

test(chain_wizard_walks_three_steps_and_shows_the_live_chain);
test(chain_wizard_restores_drafts_and_ignores_stale_name_checks);

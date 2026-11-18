// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeAll, expect, test, vi } from 'vitest';
import { createPublicClient, createWalletClient, custom, defineChain, decodeFunctionResult, encodeAbiParameters, encodeEventTopics, type Hex, type TransactionReceipt } from 'viem';
import ChainWizard, { ChainLaunchSummary } from './chain-wizard';
import vectors from '../../../../../../tests/vectors/chains/abi-vectors.json';
import { ChainLaunchAdapter, chainsAbi, type ChainLaunchResult, type ChainLaunchStep } from './chain-adapters';
import { chainRevenueOperator, walletNetwork, type ChainDraft, type ChainsParams } from './chain-domain';
import { ChainDraftStorage } from './chain-storage';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS, USDL_ADDRESS } from '@/core/network/contracts';

const creator = '0x1111111111111111111111111111111111111111';
const chain = defineChain({
  id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: ['https://rpc.example'] } },
  blockExplorers: { default: { name: 'Explorer', url: 'https://explorer.example' } },
});
let mountedAccount: string = creator;
let mountedChainId = 125;

const walletRequests = vi.fn(async () => null);
const client = createPublicClient({ chain, transport: custom({ request: walletRequests }) });
const wallet = createWalletClient({ chain, account: creator, transport: custom({ request: walletRequests }) });

vi.mock('wagmi', () => ({
  useAccount: () => ({ address: mountedAccount, chainId: mountedChainId }),
  usePublicClient: () => client,
  useWalletClient: () => ({ data: wallet }),
  useSwitchChain: () => ({ switchChainAsync: vi.fn() }),
}));

const params: ChainsParams = [creator, 23000000000000000000n, 2000000000000000000n, 500000n, 4000000n, 10000000n, 100];
const network = walletNetwork({ id: 7, chainId: 125000007, name: 'my-chain' }, 'https://rpc.example', 'https://explorer.example');
const launched: ChainLaunchResult = {
  id: 7, chainId: 125000007, name: 'my-chain', tokenName: 'My Token', tokenSymbol: 'MYT',
  token: '0x3333333333333333333333333333333333333333', pool: '0x4444444444444444444444444444444444444444',
  tradingPath: '/coin/0x4444444444444444444444444444444444444444', rpcUrl: network.rpcUrls[0],
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
  mountedAccount = creator; mountedChainId = 125;
});

async function mount(view = <ChainWizard />) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => root!.render(view));
}

function button(label: string): HTMLButtonElement {
  const found = Array.from(container.querySelectorAll('button')).find(node => node.textContent === label);
  expect(found, `Button ${label}`).toBeDefined();
  return found!;
}

async function click(label: string) {
  await act(async () => button(label).click());
}

async function typeInput(id: string, value: string) {
  const input = container.querySelector<HTMLInputElement>(id)!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

async function typeName(value: string) { await typeInput('#chain-name', value); }
async function typeToken() { await typeInput('#chain-token-name', 'My Token'); await typeInput('#chain-token-symbol', 'MYT'); }

function mockReads() {
  vi.spyOn(client, 'readContract').mockImplementation(async request => {
    if (request.functionName === 'creationFee') return 1000000n;
    if (request.functionName === 'usdlAddress') return USDL_ADDRESS;
    if (request.functionName === 'decimals') return 6;
    if (request.functionName === 'balanceOf' || request.functionName === 'allowance') return 1000000n;
    throw new Error('Unexpected read');
  });
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
    expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('1. Name, current step');
    expect(container.textContent).toContain('Use 3 to 32 lowercase letters, numbers or hyphens.');
    expect(button('Next').disabled).toBe(true);
    await typeName('taken');
    expect(availability).toHaveBeenCalledWith('taken');
    expect(container.textContent).toContain('This name is taken. Choose another.');
    expect(button('Next').disabled).toBe(true);
    await click('Next');
    expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('1. Name, current step');

    await typeName('my-chain');
    await typeToken();
    expect(container.textContent).toContain('This name is available.');
    expect(button('Next').disabled).toBe(false);
    await click('Next');
    expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('2. Size, current step');
    expect(container.querySelectorAll('input[type="radio"]')).toHaveLength(3);
    for (const allowance of ['500,000 gas and 65,536 bytes', '1,500,000 gas and 262,144 bytes', '4,000,000 gas and 1,048,576 bytes']) {
      expect(container.textContent).toContain(allowance);
    }
    expect(read).toHaveBeenCalledTimes(1);
    expect(container.textContent).toContain('Bond: 23 PAX');
    expect(container.textContent).toContain('Creation fee: 2 PAX');
    expect(container.textContent).toContain('Total: 25 PAX');
    expect(container.textContent).toContain('Market creation charge: 1 USDL');
    await act(async () => container.querySelector<HTMLInputElement>('input[value="growth"]')!.click());
    expect(new ChainDraftStorage(creator).restoreDraft()).toMatchObject({ name: 'my-chain', tier: 'growth' });
    await click('Next');
    expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('3. Review and launch, current step');
    expect(container.textContent).toContain('my-chain · Growth');
    expect(launch).not.toHaveBeenCalled();
    await click('Launch chain and token');
    expect(launch).toHaveBeenCalledTimes(1);
    expect(launch.mock.calls[0][0]).toMatchObject({ name: 'my-chain', tokenName: 'My Token', tokenSymbol: 'MYT', feeStrategy: 0, tier: 'growth', creator, chainId: 125, feeShareBps: 0 });
    expect(button('Launch chain and token').disabled).toBe(true);
    expect(container.querySelector('ol[aria-label="Steps"] [aria-current="step"]')?.textContent).toContain('Confirm in your wallet');
    await click('Launch chain and token');
    expect(launch).toHaveBeenCalledTimes(1);
    await act(async () => { report!('confirm'); });
    expect(container.querySelector('ol[aria-label="Steps"] [aria-current="step"]')?.textContent).toContain('Waiting for the block');
    expect(container.querySelector('[aria-label="Live chain"]')).toBeNull();
    await act(async () => { report!('ready'); receipt!(launched); });
    expect(container.querySelector('[aria-label="Live chain"]')?.textContent).toContain('Chain ID: 125000007');
    expect(container.textContent).toContain('Registry ID: 7');
    expect(container.textContent).toContain('My Token (MYT)');
    expect(container.textContent).toContain('Pool: 0x4444444444444444444444444444444444444444');
    expect(container.querySelector<HTMLInputElement>('#chain-rpc')?.value).toBe(launched.rpcUrl);
    expect(Array.from(container.querySelectorAll('a')).some(link => link.getAttribute('href') === launched.explorerUrl)).toBe(true);
    expect(Array.from(container.querySelectorAll('a')).some(link => link.getAttribute('href') === launched.tradingPath)).toBe(true);
    expect(container.textContent).toContain('View explorer');
    expect(Array.from(container.querySelectorAll('button')).some(node => node.textContent === 'Launch chain and token')).toBe(false);
    await click('Copy RPC URL');
    expect(copy).toHaveBeenCalledExactlyOnceWith(launched.rpcUrl);
    expect(container.textContent).toContain('RPC URL copied.');
    await click('Add to wallet');
    expect(walletRequests).toHaveBeenCalledExactlyOnceWith({ method: 'wallet_addEthereumChain', params: [launched.walletNetwork] }, undefined);
    expect(container.textContent).toContain('Chain added to your wallet.');
    expect(new ChainDraftStorage(creator).restoreDraft()).toBeNull();
  } finally {
    vi.unstubAllGlobals();
  }
}

async function chain_wizard_restores_drafts_and_ignores_stale_name_checks() {
  mockReads();
  const storage = new ChainDraftStorage(creator);
  const draft: ChainDraft = { draftId: 'saved', version: 2, creator, chainId: 125, name: 'saved-chain', tokenName: 'Saved Token', tokenSymbol: 'SVT', feeStrategy: 0, tier: 'pro', feeShareBps: 0, updatedAt: Date.now() };
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
  localStorage.setItem(`chain-draft-v2:125:${creator}`, JSON.stringify({ ...draft, updatedAt: Date.now() - 31 * 86400000 }));
  expect(() => storage.restoreDraft()).toThrow('invalid or expired');
}

function recoveryFixture() {
  const params = decodeFunctionResult({ abi: chainsAbi, functionName: 'params', data: vectors.params.returndata as Hex });
  const record = decodeFunctionResult({ abi: chainsAbi, functionName: 'getChain', data: vectors.getChain.returndata as Hex });
  const txHash = launched.txHash;
  const blockHash: Hex = `0x${'cd'.repeat(32)}`;
  const receipt: TransactionReceipt = {
    status: 'success', transactionHash: txHash, blockHash, blockNumber: 42n, transactionIndex: 0,
    from: creator, to: LAUNCHPAD_ADDRESS, contractAddress: null, cumulativeGasUsed: 60000n,
    gasUsed: 60000n, effectiveGasPrice: 1000000000n, logsBloom: `0x${'00'.repeat(256)}`,
    type: 'eip1559', logs: [{
      address: LAUNCHPAD_ADDRESS,
      topics: encodeEventTopics({ abi: CHAIN_LAUNCH_ABI, eventName: 'ChainLaunched', args: { chain: 1, pool: launched.pool, token: launched.token } }) as [Hex, ...Hex[]],
      data: encodeAbiParameters([{ type: 'uint64' }, { type: 'uint256' }, { type: 'address' }, { type: 'uint256' }, { type: 'uint256' }], [125000001n, 7n, creator, 100n, 42n]),
      blockHash, blockNumber: 42n, transactionHash: txHash, transactionIndex: 0, logIndex: 0, removed: false,
    }],
  };
  let receiptError = 'Receipt timeout';
  let readbackError = '';
  let sends = 0;
  let receiptReads = 0;
  let nameAvailable = true;
  const read = vi.spyOn(client, 'readContract').mockImplementation(async request => {
    if (request.functionName === 'params') return params;
    if (request.functionName === 'creationFee') return 1000000n;
    if (request.functionName === 'usdlAddress') return USDL_ADDRESS;
    if (request.functionName === 'decimals') return 6;
    if (request.functionName === 'balanceOf' || request.functionName === 'allowance') return 1000000n;
    if (request.functionName === 'launchedChain') return [1, 125000001n, launched.pool, launched.token, 7n, creator, 42n];
    if (request.functionName === 'name') return 'My Token';
    if (request.functionName === 'symbol') return 'MYT';
    if (request.functionName === 'chainByName') return nameAvailable ? 0 : 1;
    if (request.functionName === 'getChain') {
      if (readbackError) throw new Error(readbackError);
      return [record[0], record[1], chainRevenueOperator(record[0]), record[3], record[4], record[5], record[6], 0, ...record.slice(8)];
    }
    throw new Error('Unexpected registry method');
  });
  vi.spyOn(client, 'getChainId').mockResolvedValue(125);
  vi.spyOn(client, 'estimateContractGas').mockResolvedValue(60000n);
  vi.spyOn(client, 'getGasPrice').mockResolvedValue(1000000000n);
  vi.spyOn(wallet, 'getChainId').mockResolvedValue(125);
  vi.spyOn(wallet, 'getAddresses').mockResolvedValue([creator]);
  vi.spyOn(wallet, 'writeContract').mockImplementation(async () => { sends++; return txHash; });
  vi.spyOn(client, 'waitForTransactionReceipt').mockImplementation(async () => {
    receiptReads++;
    if (receiptError) throw new Error(receiptError);
    return receipt;
  });
  return {
    receipt, read, txHash,
    get sends() { return sends; }, get receiptReads() { return receiptReads; },
    receiptError(value: string) { receiptError = value; },
    readbackError(value: string) { readbackError = value; },
    nameAvailable(value: boolean) { nameAvailable = value; },
  };
}

async function reviewRecoveryLaunch() {
  await mount();
  await typeName('demo');
  await typeToken();
  await click('Next');
  await click('Next');
  await click('Launch chain and token');
}

async function remount() {
  await act(async () => root!.unmount());
  root = undefined;
  container.remove();
  await mount();
}

async function chain_wizard_recovers_timeout_across_remount_and_browser_reload() {
  const f = recoveryFixture();
  await reviewRecoveryLaunch();
  expect(f.sends).toBe(1);
  expect(container.textContent).toContain(f.txHash);
  expect(container.querySelector('[role="alert"]')?.textContent).toContain('Receipt timeout');
  expect(button('Launch chain and token').disabled).toBe(true);
  expect(Array.from(container.querySelectorAll('button')).some(node => node.textContent === 'Back')).toBe(false);
  const saved = new ChainDraftStorage(creator).restoreSubmission();
  expect(saved).toMatchObject({ draft: { name: 'demo', tier: 'starter' }, submission: { creator, chainId: 125, txHash: f.txHash, phase: 'confirm' } });
  await click('Launch chain and token');
  expect(f.sends).toBe(1);
  await remount();
  expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('3. Review and launch, current step');
  await click('Retry confirmation');
  expect(container.textContent).toContain('Receipt timeout');
  expect(f.sends).toBe(1);
  const body = localStorage.getItem('chain-submission-v2')!;
  await act(async () => root!.unmount());
  root = undefined; container.remove(); localStorage.clear();
  localStorage.setItem('chain-submission-v2', body);
  await mount();
  expect(button('Launch chain and token').disabled).toBe(true);
  f.receiptError(''); f.nameAvailable(false);
  await click('Retry confirmation');
  expect(container.querySelector('[aria-label="Live chain"]')?.textContent).toContain('Chain ID: 125000001');
  expect(f.sends).toBe(1);
  expect(new ChainDraftStorage(creator).restoreSubmission()).toBeNull();
}

async function chain_wizard_recovers_registry_readback_and_rejects_mismatched_accounts() {
  const f = recoveryFixture();
  f.receiptError(''); f.readbackError('Readback interrupted');
  await reviewRecoveryLaunch();
  expect(container.textContent).toContain('Readback interrupted');
  expect(container.querySelector('[aria-label="Live chain"]')).toBeNull();
  for (const changed of ['account', 'chain'] as const) {
    mountedAccount = changed === 'account' ? '0x2222222222222222222222222222222222222222' : creator;
    mountedChainId = changed === 'chain' ? 1 : 125;
    await act(async () => root!.render(<ChainWizard />));
    expect(container.textContent).toContain('belongs to a different account or chain');
    expect(container.textContent).toContain(f.txHash);
    expect(button('Retry confirmation').disabled).toBe(true);
    expect(button('Launch chain and token').disabled).toBe(true);
    const before = f.receiptReads;
    await click('Retry confirmation');
    expect(f.receiptReads).toBe(before);
    expect(f.sends).toBe(1);
    await remount();
    expect(container.textContent).toContain('belongs to a different account or chain');
  }
  mountedAccount = creator; mountedChainId = 125;
  await act(async () => root!.render(<ChainWizard />));
  f.readbackError('');
  await click('Retry confirmation');
  expect(container.querySelector('[aria-label="Live chain"]')).not.toBeNull();
  expect(f.sends).toBe(1);
}

async function chain_wizard_revalidates_after_a_confirmed_terminal_revert() {
  const f = recoveryFixture();
  await reviewRecoveryLaunch();
  f.receiptError(''); f.receipt.status = 'reverted'; f.nameAvailable(false);
  await click('Retry confirmation');
  expect(container.textContent).toContain('transaction reverted');
  expect(container.querySelector('[aria-current="step"]')?.textContent).toBe('1. Name, current step');
  expect(button('Next').disabled).toBe(true);
  expect(container.querySelector('[aria-label="Live chain"]')).toBeNull();
  expect(new ChainDraftStorage(creator).restoreSubmission()?.submission.phase).toBe('reverted');
  expect(f.sends).toBe(1);
  f.nameAvailable(true); f.receipt.status = 'success';
  await typeName('another'); await typeName('demo');
  await click('Next'); await click('Next'); await click('Launch chain and token');
  expect(container.querySelector('[aria-label="Live chain"]')).not.toBeNull();
  expect(f.sends).toBe(2);
}

async function chain_wizard_fields_keep_their_identity_attributes_and_placeholders() {
  mockReads();
  await mount();
  const fields = [
    ['chain-name', 'chainName', 'Chain name', 'Name your chain', 32],
    ['chain-token-name', 'tokenName', 'Coin name', 'Name your coin', 50],
    ['chain-token-symbol', 'tokenSymbol', 'Ticker', 'Add a coin ticker (e.g. PAXE)', 10],
  ] as const;
  for (const [id, name, label, placeholder, maxLength] of fields) {
    const input = container.querySelector<HTMLInputElement>(`#${id}`)!;
    expect(container.querySelector(`label[for="${id}"]`)?.textContent).toBe(label);
    expect(input.name).toBe(name);
    expect(input.placeholder).toBe(placeholder);
    expect(input.maxLength).toBe(maxLength);
    expect(input.autocomplete).toBe('off');
    expect(input.disabled).toBe(false);
    expect(input.closest('.sl-field')).not.toBeNull();
  }
  for (const id of ['chain-name', 'chain-token-symbol']) {
    const input = container.querySelector<HTMLInputElement>(`#${id}`)!;
    expect(input.getAttribute('autocapitalize')).toBe('none');
    expect(input.getAttribute('autocorrect')).toBe('off');
    expect(input.getAttribute('spellcheck')).toBe('false');
  }
  const name = container.querySelector<HTMLInputElement>('#chain-name')!;
  expect(name.getAttribute('aria-describedby')).toBe('chain-name-rule chain-name-status');
  expect(container.querySelector('#chain-name-rule')?.textContent).toBe('Use 3 to 32 lowercase letters, numbers or hyphens. Start and end with a letter or number.');
  expect(container.querySelector('#chain-name-status')?.getAttribute('role')).toBe('status');
  await typeName('Invalid Name');
  expect(name.getAttribute('aria-invalid')).toBe('true');
  expect(container.querySelector('#chain-name-status')?.textContent).toBe('Check the name rules above.');
  await typeName('named-chain');
  expect(name.hasAttribute('aria-invalid')).toBe(false);
  expect(container.querySelector('#chain-name-status')?.textContent).toBe('This name is available.');
}

async function chain_wizard_counters_count_utf8_bytes_and_keep_the_bounds() {
  mockReads();
  await mount();
  expect(container.querySelector('#chain-token-name-count')).toBeNull();
  expect(container.querySelector('#chain-token-symbol-count')).toBeNull();
  await typeName('byte-chain');
  await typeInput('#chain-token-name', 'Café 🪙');
  await typeInput('#chain-token-symbol', 'É');
  expect(container.querySelector('#chain-token-name-count')?.textContent).toBe('10/50');
  expect(container.querySelector('#chain-token-symbol-count')?.textContent).toBe('2/10');
  expect(container.querySelector('#chain-token-name')?.getAttribute('aria-describedby')).toBe('chain-token-name-count chain-token-name-helper');
  expect(container.querySelector('#chain-token-symbol')?.getAttribute('aria-describedby')).toBe('chain-token-symbol-count chain-token-symbol-helper');
  expect(container.querySelector('#chain-token-name-helper')?.textContent).toBe('Name must be 1 to 50 bytes.');
  expect(container.querySelector('#chain-token-symbol-helper')?.textContent).toBe('Ticker must be 1 to 10 bytes.');
  expect(button('Next').disabled).toBe(false);
  await typeInput('#chain-token-symbol', 'ééééé');
  expect(container.querySelector('#chain-token-symbol-count')?.textContent).toBe('10/10');
  expect(container.querySelector('#chain-token-symbol-count')?.getAttribute('aria-live')).toBe('polite');
  expect(button('Next').disabled).toBe(false);
  await typeInput('#chain-token-symbol', 'éééééé');
  expect(container.querySelector('#chain-token-symbol-count')?.textContent).toBe('12/10');
  expect(button('Next').disabled).toBe(true);
  await typeInput('#chain-token-symbol', 'PAXE');
  await typeInput('#chain-token-name', 'é'.repeat(25));
  expect(container.querySelector('#chain-token-name-count')?.textContent).toBe('50/50');
  expect(button('Next').disabled).toBe(false);
  await typeInput('#chain-token-name', 'é'.repeat(26));
  expect(container.querySelector('#chain-token-name-count')?.textContent).toBe('52/50');
  expect(button('Next').disabled).toBe(true);
  await typeInput('#chain-token-name', '');
  await typeInput('#chain-token-symbol', '');
  expect(container.querySelector('#chain-token-name-count')).toBeNull();
  expect(container.querySelector('#chain-token-symbol-count')).toBeNull();
}

async function chain_wizard_marks_current_completed_and_pending_steps() {
  mockReads();
  await mount();
  const steps = () => Array.from(container.querySelectorAll('ol[aria-label="Launch a chain steps"] > li'));
  expect(steps().map(node => node.getAttribute('data-step-state'))).toEqual(['current', 'pending', 'pending']);
  expect(steps().map(node => node.querySelector('.rounded-pill')?.textContent)).toEqual(['1', '2', '3']);
  expect(steps()[0].getAttribute('aria-current')).toBe('step');
  expect(steps()[1].textContent).toBe('2. Size, not started');
  await typeName('step-chain');
  await typeToken();
  await click('Next');
  expect(steps().map(node => node.getAttribute('data-step-state'))).toEqual(['done', 'current', 'pending']);
  expect(steps()[0].textContent).toBe('1. Name, done');
  expect(steps()[0].hasAttribute('aria-current')).toBe(false);
  expect(steps()[1].getAttribute('aria-current')).toBe('step');
  await click('Next');
  expect(steps().map(node => node.getAttribute('data-step-state'))).toEqual(['done', 'done', 'current']);
  expect(steps()[1].textContent).toBe('2. Size, done');
  expect(steps()[2].getAttribute('aria-current')).toBe('step');
  await click('Back');
  expect(steps().map(node => node.getAttribute('data-step-state'))).toEqual(['done', 'current', 'pending']);
}

async function chain_wizard_summary_follows_the_names_size_and_rewards() {
  const { read, availability } = mockReads();
  await mount();
  const summary = container.querySelector('aside [role="region"][aria-labelledby="chain-summary-title"]')!;
  expect(summary.querySelector('h2')?.textContent).toBe('Launch summary');
  expect(summary.textContent).toContain('Not named yet');
  await typeName('summary-chain');
  await typeInput('#chain-token-name', 'Summary Coin');
  await typeInput('#chain-token-symbol', 'SUM');
  expect(summary.textContent).toContain('summary-chain');
  expect(summary.textContent).toContain('Summary Coin (SUM)');
  expect(summary.textContent).toContain('Starter');
  expect(summary.textContent).toContain('500,000 gas and 65,536 bytes per block.');
  expect(summary.textContent).toContain('25 PAX');
  expect(summary.textContent).toContain('1 USDL');
  const namesRead = availability.mock.calls.length;
  const feesRead = vi.mocked(client.readContract).mock.calls.length;
  await click('Holders');
  expect(summary.textContent).toContain('Holders');
  await click('Next');
  await act(async () => container.querySelector<HTMLInputElement>('input[value="growth"]')!.click());
  const selected = container.querySelector('label[data-selected="true"]')!;
  expect(selected.querySelector<HTMLInputElement>('input')?.value).toBe('growth');
  expect(selected.textContent).toContain('Selected');
  expect(summary.textContent).toContain('Growth');
  expect(summary.textContent).toContain('1,500,000 gas and 262,144 bytes per block.');
  expect(read).toHaveBeenCalledTimes(1);
  expect(availability).toHaveBeenCalledTimes(namesRead);
  expect(vi.mocked(client.readContract)).toHaveBeenCalledTimes(feesRead);
  await click('Next');
  const review = container.querySelector('section > dl')!;
  expect(Array.from(review.querySelectorAll('dt')).map(node => node.textContent)).toEqual(['Chain and size', 'Coin and creator rewards', 'Block capacity']);
  expect(Array.from(review.querySelectorAll('dd')).map(node => node.textContent)).toEqual(['summary-chain · Growth', 'Summary Coin (SUM) · Holders', 'Up to 1,500,000 gas and 262,144 bytes per block.']);
  expect(button('Launch chain and token').compareDocumentPosition(button('Back')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
}

async function chain_launch_summary_renders_existing_state_without_requests() {
  const fetchRequests = vi.spyOn(globalThis, 'fetch');
  const reads = vi.spyOn(client, 'readContract');
  await mount(<ChainLaunchSummary name="" tokenName="" tokenSymbol="" tier="starter" feeStrategy={0} params={null} marketFee={null} />);
  const summary = container.querySelector('[aria-labelledby="chain-summary-title"]')!;
  expect(summary.textContent).toContain('Not named yet');
  expect(summary.textContent).toContain('Coin name (Ticker)');
  expect(summary.textContent).toContain('Awaiting chain costs');
  expect(summary.textContent).toContain('Awaiting market fee');
  await act(async () => root!.render(<ChainLaunchSummary name="quiet-chain" tokenName="Quiet Coin" tokenSymbol="QUIET" tier="pro" feeStrategy={1} params={params} marketFee={1000000n} />));
  expect(summary.textContent).toContain('quiet-chain');
  expect(summary.textContent).toContain('Quiet Coin (QUIET)');
  expect(summary.textContent).toContain('Pro');
  expect(summary.textContent).toContain('4,000,000 gas and 1,048,576 bytes per block.');
  expect(summary.textContent).toContain('Burn');
  expect(summary.textContent).toContain('25 PAX');
  expect(summary.textContent).toContain('1 USDL');
  expect(fetchRequests).not.toHaveBeenCalled();
  expect(reads).not.toHaveBeenCalled();
  expect(walletRequests).not.toHaveBeenCalled();
}

test(chain_wizard_fields_keep_their_identity_attributes_and_placeholders.name, chain_wizard_fields_keep_their_identity_attributes_and_placeholders);
test(chain_wizard_counters_count_utf8_bytes_and_keep_the_bounds.name, chain_wizard_counters_count_utf8_bytes_and_keep_the_bounds);
test(chain_wizard_marks_current_completed_and_pending_steps.name, chain_wizard_marks_current_completed_and_pending_steps);
test(chain_wizard_summary_follows_the_names_size_and_rewards.name, chain_wizard_summary_follows_the_names_size_and_rewards);
test(chain_launch_summary_renders_existing_state_without_requests.name, chain_launch_summary_renders_existing_state_without_requests);
test(chain_wizard_walks_three_steps_and_shows_the_live_chain.name, chain_wizard_walks_three_steps_and_shows_the_live_chain);
test(chain_wizard_restores_drafts_and_ignores_stale_name_checks.name, chain_wizard_restores_drafts_and_ignores_stale_name_checks);
test(chain_wizard_recovers_timeout_across_remount_and_browser_reload.name, chain_wizard_recovers_timeout_across_remount_and_browser_reload);
test(chain_wizard_recovers_registry_readback_and_rejects_mismatched_accounts.name, chain_wizard_recovers_registry_readback_and_rejects_mismatched_accounts);
test(chain_wizard_revalidates_after_a_confirmed_terminal_revert.name, chain_wizard_revalidates_after_a_confirmed_terminal_revert);

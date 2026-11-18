// @vitest-environment jsdom
import { EventEmitter } from 'node:events';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import { act, createElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createConfig, WagmiProvider } from 'wagmi';
import { connect, disconnect } from 'wagmi/actions';
import { injected } from 'wagmi/connectors';
import {
  createPublicClient, createWalletClient, decodeFunctionData, decodeFunctionResult, defineChain, encodeFunctionData, erc20Abi, http, toHex,
  type Address, type EIP1193Provider, type Hex, type PublicClient, type WalletClient,
} from 'viem';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS, PROTOCOL_CONFIG_ADDRESS } from '@/core/network/contracts';
import { poolPath } from '@/core/app-foundation/navigation';
import ChainWizard from '@/widgets/create-chain/chain-wizard';
import { ChainLaunchAdapter, chainLaunchRequest, chainsAbi } from '@/widgets/create-chain/chain-adapters';
import { chainCost, chainRevenueOperator, CHAINS_PRECOMPILE, walletNetwork, type ChainDraft } from '@/widgets/create-chain/chain-domain';
import { ChainFeeAdapter } from '@/widgets/create-chain/chain-fees';
import { ChainDraftStorage } from '@/widgets/create-chain/chain-storage';
import vectors from '../../../../../../tests/vectors/chains/abi-vectors.json';

interface PreparedChain {
  ownedFixture: boolean;
  rpcUrl: string;
  creator: Address;
  admin: Address;
  deployment: { protocolConfig: Address; usdl: Address };
}
let producer: PreparedChain & { origin: string; token: Address };
let joined: { api_origin: string; browser_origin: string };
let market: { pool: Address; token: Address };
let approvalHash: Hex;
let proxy: ReturnType<typeof createServer>;
let origin: string;
function chainNetwork(url: string) {
  return defineChain({
    id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    rpcUrls: { default: { http: [url] } },
    blockExplorers: { default: { name: 'Explorer', url: `${url}/explorer` } },
  });
}

function chainWallet() {
  return createWalletClient({ chain, account: producer.creator, transport: http(origin, { retryCount: 0 }) });
}

let chain: ReturnType<typeof chainNetwork>;
let client: ReturnType<typeof createPublicClient>;
let wallet: ReturnType<typeof chainWallet>;
let scope: { creator: string; chainId: number; epoch: number };
const blocked = new Set<string>();
const requests: Array<{ method: string; params?: unknown[] }> = [];
let root: Root | undefined;
let container: HTMLDivElement;
let queryClient: QueryClient | undefined;
let config: ReturnType<typeof createConfig> | undefined;

const actEnvironment = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
const previousActEnvironment = actEnvironment.IS_REACT_ACT_ENVIRONMENT;

beforeAll(async () => {
  actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
  const fixtureFile = process.env.L27_PHASE2_FIXTURE;
  const joinedFile = process.env.JOINED_CONFIG_FILE;
  const chainFile = process.env.JOINED_CHAIN_FILE;
  if (!fixtureFile || !joinedFile || !chainFile) throw new Error('Prepared chain and joined services are required.');
  const fixture = JSON.parse(readFileSync(fixtureFile, 'utf8')).phase2 as PreparedChain;
  expect(fixture.ownedFixture).toBe(true);
  expect(new URL(fixture.rpcUrl).hostname).toBe('127.0.0.1');
  const joinedBytes = readFileSync(joinedFile);
  expect(createHash('sha256').update(joinedBytes).digest('hex')).toBe(process.env.JOINED_CONFIG_SHA256);
  joined = JSON.parse(joinedBytes.toString());
  market = JSON.parse(readFileSync(chainFile, 'utf8'));
  producer = { ...fixture, creator: fixture.admin, origin: fixture.rpcUrl, token: fixture.deployment.usdl };
  proxy = createServer(async (request, response) => {
    try {
      let body = '';
      for await (const chunk of request) body += chunk.toString();
      const rpc = JSON.parse(body) as { method: string; params?: unknown[] };
      requests.push(rpc);
      const call = rpc.params?.[0] as { to?: string; data?: string } | undefined;
      if (blocked.has(rpc.method) || call?.data && blocked.has(call.data.slice(0, 10))) {
        response.destroy();
        return;
      }
      if (rpc.method === 'eth_call' && call?.to?.toLowerCase() === PROTOCOL_CONFIG_ADDRESS.toLowerCase()) {
        call.to = producer.deployment.protocolConfig;
      }
      const upstream = await fetch(producer.origin, {
        method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(rpc),
      });
      response.writeHead(upstream.status, { 'content-type': 'application/json' });
      response.end(await upstream.text());
    } catch { response.destroy(); }
  });
  await new Promise<void>(resolve => proxy.listen(0, '127.0.0.1', resolve));
  const address = proxy.address();
  if (!address || typeof address === 'string') throw new Error('Chain transport did not bind.');
  origin = `http://127.0.0.1:${address.port}`;
  chain = chainNetwork(origin);
  client = createPublicClient({ chain, cacheTime: 0, transport: http(origin, { retryCount: 0 }) });
  wallet = chainWallet();
  expect((await wallet.getAddresses())[0].toLowerCase()).toBe(producer.creator.toLowerCase());
  scope = { creator: producer.creator.toLowerCase(), chainId: 125, epoch: 1 };
}, 20000);

afterEach(async () => {
  await act(async () => root?.unmount());
  root = undefined;
  container?.remove();
  queryClient?.clear();
  if (config) await disconnect(config);
  config = undefined;
  blocked.clear();
  localStorage.clear();
});

afterAll(async () => {
  proxy?.closeAllConnections();
  if (proxy) await new Promise<void>(resolve => proxy.close(() => resolve()));
  actEnvironment.IS_REACT_ACT_ENVIRONMENT = previousActEnvironment;
});

function draft(name: string): ChainDraft {
  return {
    draftId: name, version: 2, creator: scope.creator, chainId: 125, name,
    tokenName: 'Chain Flow Token', tokenSymbol: 'CFT', feeStrategy: 2,
    tier: 'starter', feeShareBps: 100, updatedAt: Date.now(),
  };
}

function button(label: string): HTMLButtonElement {
  const element = Array.from(container.querySelectorAll('button')).find(node => node.textContent === label);
  expect(element, label).toBeDefined();
  return element!;
}

async function click(label: string) {
  expect(button(label).disabled, label).toBe(false);
  await act(async () => button(label).click());
}

async function rendered(text: string) {
  await expect.poll(async () => {
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
    return container.textContent;
  }, { timeout: 5000 }).toContain(text);
}

async function mount() {
  const provider = Object.assign(new EventEmitter(), { request: wallet.request }) as EIP1193Provider & EventEmitter;
  config = createConfig({
    chains: [chain], storage: null, multiInjectedProviderDiscovery: false,
    connectors: [injected({ shimDisconnect: false, target: { id: 'chain-flow', name: 'Chain flow', provider } })],
    transports: { [chain.id]: http(origin, { retryCount: 0 }) },
  });
  await connect(config, { connector: config.connectors[0] });
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => root!.render(createElement(WagmiProvider, { config: config!, reconnectOnMount: false },
    createElement(QueryClientProvider, { client: queryClient! }, createElement(ChainWizard)))));
}

function submissions() { return requests.filter(request => request.method === 'eth_sendTransaction'); }

async function flow_chain_launch_retries_cost_and_name_reads() {
  new ChainDraftStorage(scope.creator).saveDraft(draft('flow-reads'));
  const nameSelector = encodeFunctionData({ abi: chainsAbi, functionName: 'chainByName', args: ['flow-reads'] }).slice(0, 10);
  const paramsSelector = encodeFunctionData({ abi: chainsAbi, functionName: 'params' }).slice(0, 10);
  blocked.add(nameSelector); blocked.add(paramsSelector);
  await mount();
  await rendered('Name availability could not be checked.');
  await rendered('Chain costs could not be loaded.');
  expect(button('Next').disabled).toBe(true);
  expect(container.querySelector('[aria-busy="true"]')).toBeNull();
  const beforeName = requests.filter(request => (request.params?.[0] as { data?: string })?.data?.startsWith(nameSelector)).length;
  blocked.delete(nameSelector);
  await click('Retry name check');
  await rendered('Name availability could not be checked.');
  expect(requests.filter(request => (request.params?.[0] as { data?: string })?.data?.startsWith(nameSelector)).length).toBeGreaterThan(beforeName);
  const beforeParams = requests.filter(request => (request.params?.[0] as { data?: string })?.data === paramsSelector).length;
  blocked.delete(paramsSelector);
  await click('Retry chain costs');
  await rendered('Chain costs could not be loaded.');
  expect(requests.filter(request => (request.params?.[0] as { data?: string })?.data === paramsSelector).length).toBeGreaterThan(beforeParams);
  expect(button('Next').disabled).toBe(true);
  expect(container.querySelector<HTMLInputElement>('#chain-name')?.value).toBe('flow-reads');
  expect(container.querySelector<HTMLInputElement>('#chain-name')?.disabled).toBe(false);
  expect(submissions()).toHaveLength(0);
}

async function flow_chain_launch_recovers_exact_fee_approval_without_resubmitting() {
  const fees = new ChainFeeAdapter(client as PublicClient, wallet as WalletClient, () => scope);
  const funding = await fees.read({ ...scope });
  expect(funding.token).toBe(producer.token.toLowerCase());
  expect(funding.decimals).toBe(6);
  expect(funding.fee).toBe(1_000_000n);
  expect(funding.balance).toBeGreaterThanOrEqual(funding.fee);
  expect(funding.allowance).toBe(0n);
  const before = submissions().length;
  const storage = new ChainDraftStorage(scope.creator);
  const approval = await fees.approve({ ...scope }, funding.fee, saved => storage.saveFeeApproval(saved));
  expect(submissions()).toHaveLength(before + 1);
  const sent = submissions().at(-1)!.params![0] as { to: string; data: Hex };
  expect(sent.to.toLowerCase()).toBe(funding.token);
  expect(sent.data).toBe(encodeFunctionData({ abi: erc20Abi, functionName: 'approve', args: [LAUNCHPAD_ADDRESS, funding.fee] }));
  blocked.add('eth_chainId');
  await expect(fees.confirm(approval, { ...scope })).rejects.toThrow();
  expect(storage.restoreFeeApproval()).toEqual(approval);
  blocked.clear();
  const recovered = new ChainDraftStorage(scope.creator).restoreFeeApproval()!;
  const reopened = new ChainFeeAdapter(client as PublicClient, wallet as WalletClient, () => scope);
  expect(await reopened.confirm(recovered, { ...scope })).toBe('success');
  storage.saveDraft(draft('flow-fee'));
  await mount();
  await rendered(recovered.txHash);
  expect(container.querySelector<HTMLInputElement>('#chain-name')?.disabled).toBe(true);
  await expect.poll(async () => {
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
    return button('Retry fee confirmation').disabled;
  }, { timeout: 5000 }).toBe(false);
  await click('Retry fee confirmation');
  await rendered('Market fee approved.');
  expect(container.querySelector<HTMLInputElement>('#chain-name')?.disabled).toBe(false);
  expect(storage.restoreFeeApproval()).toBeNull();
  expect(submissions()).toHaveLength(before + 1);
  const expected = { ...scope };
  scope.epoch++;
  await expect(fees.approve(expected, funding.fee, () => {})).rejects.toThrow('Wallet changed');
  expect(submissions()).toHaveLength(before + 1);
  const reset = await wallet.writeContract({ account: producer.creator, chain, address: funding.token, abi: erc20Abi, functionName: 'approve', args: [LAUNCHPAD_ADDRESS, 0n] });
  expect((await client.waitForTransactionReceipt({ hash: reset })).status).toBe('success');
}

async function flow_chain_launch_retries_market_fee_funding() {
  const fees = new ChainFeeAdapter(client as PublicClient, wallet as WalletClient, () => scope);
  const before = submissions().length;
  blocked.add(encodeFunctionData({ abi: erc20Abi, functionName: 'decimals' }).slice(0, 10));
  await expect(fees.read({ ...scope })).rejects.toThrow('HTTP request failed.');
  blocked.clear();
  const funding = await fees.read({ ...scope });
  expect(funding.token).toBe(producer.token.toLowerCase());
  expect(funding.decimals).toBe(6);
  expect(funding.fee).toBe(1_000_000n);
  expect(funding.balance).toBeGreaterThanOrEqual(funding.fee);
  expect(funding.allowance).toBe(0n);
  expect(submissions()).toHaveLength(before);
  const changed = { ...scope };
  scope.epoch++;
  await expect(fees.read(changed)).rejects.toThrow('Wallet changed');
}

async function flow_chain_launch_reviews_native_request_and_joined_market() {
  const reviewed = draft('flow-launch');
  const fees = new ChainFeeAdapter(client as PublicClient, wallet as WalletClient, () => scope);
  const funding = await fees.read({ ...scope });
  const before = submissions().length;
  const storage = new ChainDraftStorage(scope.creator);
  const approval = await fees.approve({ ...scope }, funding.fee, saved => storage.saveFeeApproval(saved));
  expect(await fees.confirm(approval, { ...scope })).toBe('success');
  expect((await fees.read({ ...scope })).allowance).toBe(funding.fee);
  expect(submissions()).toHaveLength(before + 1);
  approvalHash = approval.txHash;
  storage.clearFeeApproval(approval.txHash);
  expect(storage.restoreFeeApproval()).toBeNull();

  const params = decodeFunctionResult({ abi: chainsAbi, functionName: 'params', data: vectors.params.returndata as Hex });
  const record = decodeFunctionResult({ abi: chainsAbi, functionName: 'getChain', data: vectors.getChain.returndata as Hex });
  const cost = chainCost(params, reviewed.tier, '0');
  const request = chainLaunchRequest(reviewed, scope.creator, BigInt(cost.totalValueRaw));
  expect(request.address).toBe(LAUNCHPAD_ADDRESS);
  expect(request.account).toBe(scope.creator);
  expect(request.value).toBe(params[1] + params[2]);
  expect(decodeFunctionData({ abi: CHAIN_LAUNCH_ABI, data: encodeFunctionData(request) })).toEqual({
    functionName: 'launchChain', args: [reviewed.tokenName, reviewed.tokenSymbol, reviewed.feeStrategy,
      '0x0000000000000000000000000000000000000000', reviewed.name, 500000n, 65536n, reviewed.feeShareBps],
  });
  expect(chainRevenueOperator(record[0])).toBe(`0x${createHash('sha256').update(`launchpad_chain_revenue_${record[0]}`).digest('hex').slice(0, 40)}`);
  expect(chainRevenueOperator(record[0])).not.toBe(scope.creator);
  const network = walletNetwork({ id: record[0], chainId: Number(record[4]), name: reviewed.name }, origin, `${origin}/explorer`);
  expect(network.chainId).toBe(toHex(record[4]));
  expect(network.rpcUrls).toEqual([`${origin}/chain/${record[0]}`]);
  expect(network.blockExplorerUrls).toEqual([`${origin}/explorer/chains/${record[0]}`]);
  expect(network.nativeCurrency).toEqual({ name: 'PAX', symbol: 'PAX', decimals: 18 });
  const response = await fetch(`${joined.api_origin}/v2/pools/${market.pool}`, { headers: { 'X-Schema-Version': '2' } });
  expect(response.status).toBe(200);
  expect(response.headers.get('X-Schema-Version')).toBe('2');
  const body = await response.json();
  expect(body.data.pool.poolAddress.toLowerCase()).toBe(market.pool.toLowerCase());
  expect(body.data.pool.tokenAddress.toLowerCase()).toBe(market.token.toLowerCase());
  const trading = await fetch(new URL(poolPath(125, market.pool), joined.browser_origin));
  expect(trading.status).toBe(200);
  expect(await trading.text()).toContain(`data-market-pool="${market.pool.toLowerCase()}"`);
  expect(await client.getBytecode({ address: CHAINS_PRECOMPILE })).toBeUndefined();
  const adapter = new ChainLaunchAdapter(client as PublicClient, wallet as WalletClient, () => scope);
  await expect(adapter.launch(reviewed, () => {})).rejects.toThrow();
  expect(submissions()).toHaveLength(before + 1);
}

async function flow_chain_launch_retries_saved_state_and_preserves_the_submitted_hash() {
  const storage = new ChainDraftStorage(scope.creator);
  const params = decodeFunctionResult({ abi: chainsAbi, functionName: 'params', data: vectors.params.returndata as Hex });
  const reviewed = draft('flow-saved');
  storage.saveSubmission(reviewed, {
    creator: scope.creator, chainId: 125, txHash: approvalHash, phase: 'confirm', bondRaw: params[1].toString(),
  });
  const saved = localStorage.getItem('chain-submission-v2')!;
  localStorage.setItem('chain-submission-v2', '{');
  const before = submissions().length;
  await mount();
  expect(container.querySelector('[role="alert"]')).not.toBeNull();
  expect(container.querySelector('a[href="/create"]')).not.toBeNull();
  await click('Retry saved launch');
  expect(localStorage.getItem('chain-submission-v2')).toBe('{');
  localStorage.setItem('chain-submission-v2', saved);
  await click('Retry saved launch');
  await rendered(approvalHash);
  await expect.poll(() => button('Retry confirmation').disabled).toBe(false);
  await click('Retry confirmation');
  await rendered('Expected one ChainLaunched receipt event.');
  expect(submissions()).toHaveLength(before);
  expect(ChainDraftStorage.restoreSubmission()?.submission.txHash).toBe(approvalHash);
  expect(localStorage.getItem('chain-submission-v2')).toBe(saved);
  expect(container.querySelector('[aria-label="Live chain"]')).toBeNull();
  expect(button('Launch chain and token').disabled).toBe(true);
  expect(button('Retry confirmation').disabled).toBe(false);
}

test(flow_chain_launch_retries_cost_and_name_reads.name, { timeout: 20000 }, flow_chain_launch_retries_cost_and_name_reads);
test(flow_chain_launch_retries_market_fee_funding.name, { timeout: 20000 }, flow_chain_launch_retries_market_fee_funding);
test(flow_chain_launch_recovers_exact_fee_approval_without_resubmitting.name, { timeout: 20000 }, flow_chain_launch_recovers_exact_fee_approval_without_resubmitting);
test(flow_chain_launch_reviews_native_request_and_joined_market.name, { timeout: 20000 }, flow_chain_launch_reviews_native_request_and_joined_market);
test(flow_chain_launch_retries_saved_state_and_preserves_the_submitted_hash.name, { timeout: 20000 }, flow_chain_launch_retries_saved_state_and_preserves_the_submitted_hash);

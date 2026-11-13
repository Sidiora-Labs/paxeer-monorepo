import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, test } from 'vitest';
import { createPublicClient, createWalletClient, custom, decodeFunctionData, encodeFunctionResult, getAddress, parseAbi, type EIP1193RequestFn } from 'viem';
import { HOME_CHAIN_ABI, ensureHomeChain, homeChainOfPool } from './home-chain';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS } from '@/core/network/contracts';

const pool = getAddress('0x4444444444444444444444444444444444444444');
const rpc = 'https://rpc.example/evm';
const explorer = 'https://explorer.example';
const registryAbi = parseAbi(['function getChain(uint32 id) view returns (uint32 id,string name,address operator,uint8 status,uint64 chainId,uint64 gasAllowance,uint64 byteAllowance,uint16 feeShareBps,uint256 bond,uint256 baseFeePerGas,uint64 createdHeight)']);

function publicReader(home: bigint) {
  return createPublicClient({
    transport: custom({ request: (async ({ method, params }: { method: string; params?: unknown }) => {
      if (method !== 'eth_call') throw new Error(`Unexpected RPC method ${method}`);
      const call = (params as [{ to: string; data: `0x${string}` }])[0];
      if (call.to.toLowerCase() === LAUNCHPAD_ADDRESS.toLowerCase()) {
        const decoded = decodeFunctionData({ abi: HOME_CHAIN_ABI, data: call.data });
        if (decoded.functionName === 'homeChainOf') return encodeFunctionResult({ abi: HOME_CHAIN_ABI, functionName: 'homeChainOf', result: home });
        if (decoded.functionName === 'launchedChainOfPool') return encodeFunctionResult({ abi: HOME_CHAIN_ABI, functionName: 'launchedChainOfPool', result: 7 });
      }
      const decoded = decodeFunctionData({ abi: registryAbi, data: call.data });
      if (decoded.functionName !== 'getChain') throw new Error('Unexpected registry read');
      return encodeFunctionResult({ abi: registryAbi, functionName: 'getChain', result: [7, 'my-chain', pool, 1, 125000007n, 500000n, 65536n, 1000, 1n, 1n, 1n] });
    }) as EIP1193RequestFn }),
  });
}

function statefulWallet(known: boolean) {
  let chain = 125;
  const requests: string[] = [];
  const wallet = createWalletClient({ transport: custom({ request: (async ({ method, params }: { method: string; params?: unknown }) => {
    if (method === 'eth_chainId') return `0x${chain.toString(16)}`;
    if (method === 'wallet_switchEthereumChain') {
      requests.push(method);
      if (!known) throw { code: 4902 };
      chain = Number((params as [{ chainId: string }])[0].chainId);
      return null;
    }
    if (method === 'wallet_addEthereumChain') {
      requests.push(method);
      known = true;
      expect((params as [{ rpcUrls: string[] }])[0].rpcUrls).toEqual(['https://rpc.example/evm/chain/7']);
      return null;
    }
    throw new Error(`Unexpected wallet method ${method}`);
  }) as EIP1193RequestFn }) });
  return { wallet, requests };
}

describe('market home chain', () => {
  test('resolves root and launched market node paths', async function homeChainIdsAndPaths() {
    const root = await homeChainOfPool(publicReader(0n), pool, rpc, explorer);
    expect(root).toEqual({ registryNumber: 0, chainId: 125, rpcUrl: rpc, walletNetwork: null });
    const launched = await homeChainOfPool(publicReader(125000007n), pool, rpc, explorer);
    expect(launched.chainId).toBe(125000007);
    expect(launched.rpcUrl).toBe('https://rpc.example/evm/chain/7');
    expect(launched.walletNetwork).toMatchObject({ chainId: '0x7735947', chainName: 'my-chain on Paxeer', blockExplorerUrls: ['https://explorer.example/chains/7'] });
  });

  test('switches, adds an unknown chain, then switches again', async function homeChainSwitchAndAddOrder() {
    const home = await homeChainOfPool(publicReader(125000007n), pool, rpc, explorer);
    const { wallet, requests } = statefulWallet(false);
    await ensureHomeChain(wallet, home);
    expect(requests).toEqual(['wallet_switchEthereumChain', 'wallet_addEthereumChain', 'wallet_switchEthereumChain']);
    expect(await wallet.getChainId()).toBe(home.chainId);
  });

  test('does not switch a root market', async function rootMarketKeepsWalletChain() {
    const home = await homeChainOfPool(publicReader(0n), pool, rpc, explorer);
    const { wallet, requests } = statefulWallet(false);
    await ensureHomeChain(wallet, home);
    expect(requests).toEqual([]);
  });

  test('uses names published by the launchpad ABI', function homeChainAbiNames() {
    const published = JSON.parse(readFileSync(resolve(process.cwd(), '../../../precompiles/launchpad/abi.json'), 'utf8')) as Array<{ type: string; name: string }>;
    const names = new Set(published.filter(item => item.type === 'function').map(item => item.name));
    for (const name of ['homeChainOf', 'launchedChainOfPool']) expect(names.has(name)).toBe(true);
    for (const item of HOME_CHAIN_ABI.filter(item => item.type === 'function')) expect(names.has(item.name)).toBe(true);
    for (const item of CHAIN_LAUNCH_ABI.filter(item => item.type === 'function')) {
      if (['homeChainOf', 'launchedChainOfPool'].includes(item.name)) expect(names.has(item.name)).toBe(true);
    }
  });
});

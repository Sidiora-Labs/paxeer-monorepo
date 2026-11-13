import {
  createPublicClient,
  defineChain,
  erc20Abi,
  getAddress,
  http,
  parseAbi,
  type Address,
  type PublicClient,
  type WalletClient,
} from 'viem';
import { LAUNCHPAD_ADDRESS } from '@/core/network/contracts';
import { CHAINS_PRECOMPILE, VIRTUAL_CHAIN_ID_BASE, chainIdOf, walletNetwork } from '../create-chain/chain-domain';
import { reviewedRouterAbi } from '../trading/transaction-controls/domain';

export const HOME_CHAIN_ABI = parseAbi([
  'function homeChainOf(address pool) view returns (uint64)',
  'function launchedChainOfPool(address pool) view returns (uint32)',
]);
const chainRecordAbi = parseAbi([
  'function getChain(uint32 id) view returns (uint32 id,string name,address operator,uint8 status,uint64 chainId,uint64 gasAllowance,uint64 byteAllowance,uint16 feeShareBps,uint256 bond,uint256 baseFeePerGas,uint64 createdHeight)',
]);

export interface HomeChain {
  registryNumber: number;
  chainId: number;
  rpcUrl: string;
  walletNetwork: ReturnType<typeof walletNetwork> | null;
}

export async function homeChainOfPool(
  rootClient: PublicClient,
  pool: Address,
  rpcBase: string,
  explorerBase: string,
): Promise<HomeChain> {
  const home = await rootClient.readContract({ address: LAUNCHPAD_ADDRESS, abi: HOME_CHAIN_ABI, functionName: 'homeChainOf', args: [pool] });
  if (home === 0n) return { registryNumber: 0, chainId: 125, rpcUrl: rpcBase, walletNetwork: null };
  const registryNumber = Number(home - BigInt(VIRTUAL_CHAIN_ID_BASE));
  if (!Number.isInteger(registryNumber) || registryNumber < 1 || registryNumber > 0xffffffff || chainIdOf(registryNumber) !== Number(home)) {
    throw new Error('Invalid market home chain');
  }
  const recorded = await rootClient.readContract({ address: LAUNCHPAD_ADDRESS, abi: HOME_CHAIN_ABI, functionName: 'launchedChainOfPool', args: [pool] });
  if (recorded !== registryNumber) throw new Error('Market home chain does not match its registry record');
  const chain = await rootClient.readContract({ address: CHAINS_PRECOMPILE, abi: chainRecordAbi, functionName: 'getChain', args: [registryNumber] });
  if (chain[0] !== registryNumber || chain[4] !== home) throw new Error('Invalid chain registry record');
  const network = walletNetwork({ id: registryNumber, chainId: Number(home), name: chain[1] }, rpcBase, explorerBase);
  return { registryNumber, chainId: Number(home), rpcUrl: network.rpcUrls[0], walletNetwork: network };
}

function unknownChain(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false;
  const value = error as { code?: unknown; cause?: unknown; data?: unknown };
  return value.code === 4902 || unknownChain(value.cause) || unknownChain(value.data);
}

export async function ensureHomeChain(wallet: WalletClient, home: HomeChain): Promise<void> {
  if (home.registryNumber === 0 || await wallet.getChainId() === home.chainId) return;
  const chainId = `0x${home.chainId.toString(16)}`;
  try {
    await wallet.request({ method: 'wallet_switchEthereumChain', params: [{ chainId }] });
  } catch (error) {
    if (!unknownChain(error) || !home.walletNetwork) throw new Error(`Switch to ${home.walletNetwork?.chainName ?? 'the market home chain'} in your wallet to trade.`, { cause: error });
    await wallet.request({ method: 'wallet_addEthereumChain', params: [home.walletNetwork] });
    await wallet.request({ method: 'wallet_switchEthereumChain', params: [{ chainId }] });
  }
  if (await wallet.getChainId() !== home.chainId) throw new Error(`Switch to ${home.walletNetwork?.chainName ?? 'the market home chain'} in your wallet to trade.`);
}

export function homeChainClient(home: HomeChain) {
  return createPublicClient({
    chain: defineChain({
      id: home.chainId,
      name: home.walletNetwork?.chainName ?? 'Paxeer Network',
      nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
      rpcUrls: { default: { http: [home.rpcUrl] } },
    }),
    transport: http(home.rpcUrl),
  });
}

export function homeChainMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return /WrongChain|wrong.chain/i.test(message) ? 'Switch to this market’s home chain in your wallet to trade.' : message;
}

export async function approveHomeChainTrade(wallet: WalletClient, home: HomeChain, account: Address, token: Address, router: Address, amount: bigint) {
  if (home.registryNumber === 0 || amount <= 0n) throw new Error('A launched market and a positive approval are required.');
  await ensureHomeChain(wallet, home);
  if (!(await wallet.getAddresses()).some(value => value.toLowerCase() === account.toLowerCase())) throw new Error('Wallet account changed.');
  const client = homeChainClient(home);
  const hash = await wallet.writeContract({ account, chain: client.chain, address: token, abi: erc20Abi, functionName: 'approve', args: [router, amount] });
  const receipt = await client.waitForTransactionReceipt({ hash });
  if (receipt.status !== 'success') throw new Error('Approval was not confirmed.');
  return receipt;
}

export async function submitHomeChainTrade(wallet: WalletClient, home: HomeChain, account: Address, router: Address, pool: Address, token: Address, amount: bigint, minimum: bigint, deadline: bigint, quoteExpiresAt: number, isBuy: boolean) {
  if (home.registryNumber === 0 || amount <= 0n || minimum <= 0n || deadline <= BigInt(Math.floor(Date.now() / 1000)) || Date.now() >= quoteExpiresAt) throw new Error('Review the trade amount, quote and deadline.');
  await ensureHomeChain(wallet, home);
  if (!(await wallet.getAddresses()).some(value => value.toLowerCase() === account.toLowerCase())) throw new Error('Wallet account changed.');
  const client = homeChainClient(home);
  const spender = getAddress(router);
  const allowance = await client.readContract({ address: token, abi: erc20Abi, functionName: 'allowance', args: [account, spender] });
  if (allowance < amount) throw new Error('Approve the trade amount on this chain.');
  const request = { address: spender, abi: reviewedRouterAbi, functionName: isBuy ? 'buy' as const : 'sell' as const, args: [pool, amount, minimum, deadline] as const, account, chain: client.chain };
  await client.simulateContract(request);
  if (Date.now() >= quoteExpiresAt || deadline <= BigInt(Math.floor(Date.now() / 1000))) throw new Error('Trade quote expired. Review again.');
  const hash = await wallet.writeContract(request);
  const receipt = await client.waitForTransactionReceipt({ hash });
  if (receipt.status !== 'success') throw new Error('Trade was not confirmed.');
  return receipt;
}

import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
  createPublicClient, createWalletClient, custom, encodeFunctionData,
  erc20Abi, keccak256, pad, stringToHex, zeroAddress,
  type Abi, type Address, type EIP1193Provider,
} from 'viem';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';

const require = createRequire(import.meta.url);
const ganache = require('ganache') as {
  provider(options: object): EIP1193Provider & { disconnect(): Promise<void> };
};
const solc = require('solc') as {
  compile(input: string, callbacks: { import(path: string): { contents: string } }): string;
};
interface Artifact {
  abi: Abi;
  evm: { bytecode: { object: string }; deployedBytecode: { object: string } };
}

export const swapTestChain = {
  id: 125, name: 'Swap regression EVM',
  nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: ['http://127.0.0.1:1'] } },
} as const;

export async function deploySwapChain() {
  const sourcesRoot = resolve(__dirname, '../../../../../../contracts/launchpad');
  const roots = [
    'core/SidioraERC20', 'protocol/ProtocolConfig', 'protocol/Treasury',
    'data/PoolRegistry', 'data/FeeAccumulator', 'core/SidioraNFT',
    'core/SidioraPool', 'core/PoolBeacon', 'core/SidioraFactory',
    'periphery/Router', 'periphery/Quoter', 'meta-ag/proxy/ERC1967Proxy',
  ];
  const sources = Object.fromEntries(roots.map(path => [`contracts/${path}.sol`, {
    content: readFileSync(resolve(sourcesRoot, `contracts/${path}.sol`), 'utf8'),
  }]));
  const compiled = JSON.parse(solc.compile(JSON.stringify({
    language: 'Solidity', sources,
    settings: {
      optimizer: { enabled: true, runs: 200 }, viaIR: true, evmVersion: 'shanghai',
      outputSelection: { '*': { '*': ['abi', 'evm.bytecode.object', 'evm.deployedBytecode.object'] } },
    },
  }), { import: path => ({ contents: readFileSync(resolve(sourcesRoot, path), 'utf8') }) })) as {
    errors?: { severity: string; formattedMessage: string }[];
    contracts: Record<string, Record<string, Artifact>>;
  };
  const errors = compiled.errors?.filter(error => error.severity === 'error') ?? [];
  if (errors.length) throw new Error(errors.map(error => error.formattedMessage).join('\n'));
  function artifact(name: string): Artifact {
    const found = Object.values(compiled.contracts).flatMap(file => file[name] ? [file[name]] : []);
    if (found.length !== 1) throw new Error(`Expected one real contract artifact: ${name}`);
    return found[0];
  }

  const provider = ganache.provider({
    chain: { chainId: 125, hardfork: 'shanghai' },
    // Injected-wallet writes omit gas; estimate it for the real multihop execution.
    miner: { defaultTransactionGasLimit: 'estimate' },
    wallet: { totalAccounts: 2 }, logging: { quiet: true },
  });
  const transport = custom(provider, { retryCount: 0 });
  const client = createPublicClient({ chain: swapTestChain, transport, pollingInterval: 10 });
  const wallet = createWalletClient({ chain: swapTestChain, transport, pollingInterval: 10 });
  const [account] = await wallet.getAddresses();
  async function deploy(name: string, args: readonly unknown[] = []): Promise<Address> {
    const a = artifact(name);
    const hash = await wallet.deployContract({ account, abi: a.abi, bytecode: `0x${a.evm.bytecode.object}`, args, gas: 25000000n });
    const receipt = await client.waitForTransactionReceipt({ hash });
    if (receipt.status !== 'success' || !receipt.contractAddress) throw new Error(`Deployment failed: ${name}`);
    return receipt.contractAddress;
  }
  async function write(name: string, address: Address, functionName: string, args: readonly unknown[] = []) {
    const hash = await wallet.writeContract({ account, address, abi: artifact(name).abi, functionName, args, gas: 25000000n });
    const receipt = await client.waitForTransactionReceipt({ hash });
    if (receipt.status !== 'success') throw new Error(`Transaction failed: ${name}.${functionName}`);
    return receipt;
  }
  async function proxy(name: string, args: readonly unknown[], pinned?: Address) {
    const implementation = await deploy(name);
    const data = encodeFunctionData({ abi: artifact(name).abi, functionName: 'initialize', args });
    if (!pinned) return deploy('ERC1967Proxy', [implementation, data]);
    // Install the production proxy runtime at the addresses used by the real web hooks.
    const rpc = provider.request.bind(provider) as (args: { method: string; params: unknown[] }) => Promise<unknown>;
    await rpc({ method: 'evm_setAccountCode', params: [pinned, `0x${artifact('ERC1967Proxy').evm.deployedBytecode.object}`] });
    await rpc({ method: 'evm_setAccountStorageAt', params: [pinned,
      '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc', pad(implementation)] });
    await write(name, pinned, 'initialize', args);
    return pinned;
  }
  try {
    const usdl = await deploy('SidioraERC20', ['USDL', 'USDL', 1000000000000000n, account]);
    const config = await proxy('ProtocolConfig', [usdl, zeroAddress, account]);
    const treasury = await proxy('Treasury', [zeroAddress, account]);
    const registry = await proxy('PoolRegistry', [zeroAddress, account]);
    const accumulator = await proxy('FeeAccumulator', [config, treasury, registry, zeroAddress, usdl, account]);
    const nft = await proxy('SidioraNFT', ['Pool NFT', 'PNFT', zeroAddress, account]);
    const poolImplementation = await deploy('SidioraPool');
    const beacon = await deploy('PoolBeacon', [poolImplementation, account]);
    const factory = await proxy('SidioraFactory', [beacon, nft, registry, zeroAddress, config, treasury, accumulator, usdl, account]);
    const router = await proxy('Router', [factory, registry, config, usdl, account], CORE_DEPLOYMENT.router);
    await proxy('Quoter', [registry, config, account], CORE_DEPLOYMENT.quoter);
    for (const [name, address, role, grantee] of [
      ['PoolRegistry', registry, 'FACTORY_ROLE', factory],
      ['FeeAccumulator', accumulator, 'FACTORY_ROLE', factory],
      ['SidioraNFT', nft, 'MINTER_ROLE', factory],
      ['Treasury', treasury, 'DEPOSITOR_ROLE', factory],
      ['Treasury', treasury, 'DEPOSITOR_ROLE', router],
      ['Treasury', treasury, 'DEPOSITOR_ROLE', accumulator],
      ['SidioraFactory', factory, 'ROUTER_ROLE', router],
    ] as const) await write(name, address, 'grantRole', [keccak256(stringToHex(role)), grantee]);
    await write('SidioraERC20', usdl, 'approve', [router, 1000000000000000n]);
    const tokens: Address[] = [];
    const pools: Address[] = [];
    for (const label of ['Quote A', 'Quote B', 'Quote C']) {
      await write('Router', router, 'createMarket', [label, label, 0, zeroAddress]);
      const listed = await client.readContract({ address: registry, abi: artifact('PoolRegistry').abi, functionName: 'getAllPools', args: [0n, 10n] }) as Address[];
      const pool = listed[listed.length - 1];
      const token = await client.readContract({ address: pool, abi: artifact('SidioraPool').abi, functionName: 'tokenAddress' }) as Address;
      await write('Router', router, 'buy', [pool, 1000000000n, 1n, BigInt(Math.floor(Date.now() / 1000) + 300)]);
      await write('SidioraERC20', token, 'approve', [router, 1000000000000000n]);
      pools.push(pool); tokens.push(token);
    }
    async function balance(token: Address) {
      return client.readContract({ address: token, abi: erc20Abi, functionName: 'balanceOf', args: [account] });
    }
    async function quote(tokenIn: Address, tokenOut: Address, amountIn: bigint) {
      return client.readContract({ address: CORE_DEPLOYMENT.quoter, abi: artifact('Quoter').abi, functionName: 'quoteMultihop', args: [tokenIn, tokenOut, amountIn] }) as Promise<{
        amountOut: bigint; combinedPriceImpactBps: bigint;
      }>;
    }
    return { provider, client, wallet, account, tokens, pools, balance, quote };
  } catch (error) {
    await provider.disconnect();
    throw error;
  }
}

export type SwapChain = Awaited<ReturnType<typeof deploySwapChain>>;

import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import { ethers } from 'ethers';
import { createCustomCommon, Mainnet, Hardfork } from '@ethereumjs/common';
import { createBlock } from '@ethereumjs/block';
import { createLegacyTx } from '@ethereumjs/tx';
import { Account, createAddressFromString, bytesToHex, hexToBytes } from '@ethereumjs/util';
import { createVM, runTx } from '@ethereumjs/vm';
import { CONTRACTS, TOKENS } from './sdk';

interface CompiledContract {
  abi: ethers.InterfaceAbi;
  evm: { bytecode: { object: string }; deployedBytecode: { object: string } };
}
interface Compilation {
  contracts: Record<string, Record<string, CompiledContract>>;
  errors?: { severity: string; formattedMessage: string }[];
}

let compiled: Compilation | undefined;
function contracts(): Compilation {
  if (compiled) return compiled;
  const require = createRequire(import.meta.url);
  const solc: {
    compile(input: string, callbacks: { import(path: string): { contents: string } }): string;
  } = require('solc');
  const root = resolve(__dirname, '../../../../../contracts/launchpad/contracts');
  const sources = [
    'WPAX9.sol',
    'protocol/ProtocolConfig.sol',
    'protocol/Treasury.sol',
    'data/PoolRegistry.sol',
    'data/FeeAccumulator.sol',
    'core/SidioraNFT.sol',
    'core/SidioraFactory.sol',
    'core/SidioraPool.sol',
    'core/PoolBeacon.sol',
    'periphery/Router.sol',
    'periphery/Quoter.sol',
    'meta-ag/proxy/ERC1967Proxy.sol',
  ];
  const result: Compilation = JSON.parse(
    solc.compile(
      JSON.stringify({
        language: 'Solidity',
        sources: Object.fromEntries(
          sources.map((path) => [path, { content: readFileSync(resolve(root, path), 'utf8') }])
        ),
        settings: {
          optimizer: { enabled: true, runs: 200 },
          viaIR: true,
          evmVersion: 'shanghai',
          outputSelection: {
            '*': { '*': ['abi', 'evm.bytecode.object', 'evm.deployedBytecode.object'] },
          },
        },
      }),
      { import: (path) => ({ contents: readFileSync(resolve(root, path), 'utf8') }) }
    )
  );
  const errors = result.errors?.filter((error) => error.severity === 'error') ?? [];
  if (errors.length) throw new Error(errors.map((error) => error.formattedMessage).join('\n'));
  compiled = result;
  return result;
}

function artifact(path: string, name: string): CompiledContract {
  const contract = contracts().contracts[path]?.[name];
  if (!contract) throw new Error(`Missing compiled production contract ${path}:${name}`);
  return contract;
}

/** An EIP-1193 transport backed by the actual JavaScript EVM, with one transaction per block. */
export async function createSwapEvm(chainId = 125) {
  const common = createCustomCommon({ chainId }, Mainnet, { hardfork: Hardfork.Shanghai });
  const vm = await createVM({ common });
  const privateKey = hexToBytes(`0x${'12'.repeat(32)}`);
  const wallet = new ethers.Wallet(bytesToHex(privateKey));
  const owner = wallet.address;
  await vm.stateManager.putAccount(createAddressFromString(owner), new Account(0n, 10n ** 30n));
  let height = 0;
  let timeOffset = 0;
  let timestamp = BigInt(Math.floor(Date.now() / 1000));
  const transactions = new Map<string, Record<string, unknown>>();
  const receipts = new Map<string, Record<string, unknown>>();
  const h = (value: bigint | number) => ethers.toQuantity(value);
  const address = (value: string) => createAddressFromString(value);
  const blocks = new Map<number, ReturnType<typeof createBlock>>();
  function block(number = height) {
    const existing = blocks.get(number);
    if (existing) return existing;
    const next = createBlock(
      { header: { number: BigInt(number), timestamp, gasLimit: 100_000_000n, baseFeePerGas: 1n } },
      { common }
    );
    blocks.set(number, next);
    return next;
  }
  block();

  async function call(input: Record<string, string>, estimate: boolean): Promise<string> {
    await vm.stateManager.checkpoint();
    try {
      const result = await vm.evm.runCall({
        caller: address(input.from ?? owner),
        to: input.to ? address(input.to) : undefined,
        data: hexToBytes((input.data ?? '0x') as `0x${string}`),
        value: BigInt(input.value ?? 0),
        gasLimit: 90_000_000n,
        block: block(),
      });
      if (result.execResult.exceptionError) {
        throw {
          code: 3,
          message: 'execution reverted',
          data: bytesToHex(result.execResult.returnValue),
        };
      }
      return estimate ? h(20_000_000) : bytesToHex(result.execResult.returnValue);
    } finally {
      await vm.stateManager.revert();
    }
  }

  async function handleRequest({
    method,
    params = [],
  }: {
    method: string;
    params?: readonly unknown[] | Record<string, unknown>;
  }) {
    if (!Array.isArray(params)) throw new Error('EVM RPC requires positional parameters');
    const first = params[0];
    switch (method) {
      case 'eth_chainId':
        return h(chainId);
      case 'eth_accounts':
      case 'eth_requestAccounts':
        return [owner];
      case 'eth_blockNumber':
        return h(height);
      case 'eth_gasPrice':
      case 'eth_maxPriorityFeePerGas':
        return h(1_000_000_000);
      case 'eth_getBalance':
        return h((await vm.stateManager.getAccount(address(String(first))))?.balance ?? 0n);
      case 'eth_getTransactionCount':
        return h((await vm.stateManager.getAccount(address(String(first))))?.nonce ?? 0n);
      case 'eth_getCode':
        return bytesToHex(await vm.stateManager.getCode(address(String(first))));
      case 'eth_call':
        return call(first as Record<string, string>, false);
      case 'eth_estimateGas':
        return call(first as Record<string, string>, true);
      case 'eth_getTransactionByHash':
        return transactions.get(String(first)) ?? null;
      case 'eth_getTransactionReceipt':
        return receipts.get(String(first)) ?? null;
      case 'eth_getBlockByNumber': {
        const number = first === 'latest' ? height : Number(BigInt(String(first)));
        const b = block(number);
        return {
          number: h(number),
          hash: bytesToHex(b.hash()),
          parentHash: bytesToHex(b.header.parentHash),
          timestamp: h(b.header.timestamp),
          nonce: '0x0000000000000000',
          difficulty: '0x0',
          gasLimit: h(b.header.gasLimit),
          gasUsed: h(b.header.gasUsed),
          miner: ethers.ZeroAddress,
          extraData: '0x',
          baseFeePerGas: '0x1',
          transactions: [],
        };
      }
      case 'eth_sendTransaction': {
        const input = first as Record<string, string>;
        if (input.from.toLowerCase() !== owner.toLowerCase())
          throw new Error('Unknown EVM fixture account');
        const account = await vm.stateManager.getAccount(address(owner));
        const tx = createLegacyTx(
          {
            nonce: account!.nonce,
            gasLimit: 30_000_000n,
            gasPrice: 1_000_000_000n,
            to: input.to ? address(input.to) : undefined,
            value: BigInt(input.value ?? 0),
            data: hexToBytes((input.data ?? '0x') as `0x${string}`),
          },
          { common }
        ).sign(privateKey);
        height++;
        timestamp = BigInt(Math.floor(Date.now() / 1000) + timeOffset);
        const current = block();
        const result = await runTx(vm, { tx, block: current });
        const hash = bytesToHex(tx.hash());
        const blockHash = bytesToHex(current.hash());
        transactions.set(hash, {
          hash,
          blockHash,
          blockNumber: h(height),
          transactionIndex: '0x0',
          from: owner,
          to: input.to ?? null,
          nonce: h(tx.nonce),
          gas: h(tx.gasLimit),
          gasPrice: h(tx.gasPrice),
          value: h(tx.value),
          input: bytesToHex(tx.data),
          chainId: h(chainId),
          type: '0x0',
          v: h(tx.v!),
          r: h(tx.r!),
          s: h(tx.s!),
        });
        const logs = (result.execResult.logs ?? []).map(([addr, topics, data], index) => ({
          address: bytesToHex(addr),
          topics: topics.map(bytesToHex),
          data: bytesToHex(data),
          blockHash,
          blockNumber: h(height),
          transactionHash: hash,
          transactionIndex: '0x0',
          logIndex: h(index),
          removed: false,
        }));
        receipts.set(hash, {
          transactionHash: hash,
          blockHash,
          blockNumber: h(height),
          transactionIndex: '0x0',
          from: owner,
          to: input.to ?? null,
          contractAddress: result.createdAddress?.toString() ?? null,
          cumulativeGasUsed: h(result.totalGasSpent),
          gasUsed: h(result.totalGasSpent),
          effectiveGasPrice: h(tx.gasPrice),
          logsBloom: bytesToHex(result.bloom.bitvector),
          status: result.execResult.exceptionError ? '0x0' : '0x1',
          type: '0x0',
          logs,
        });
        return hash;
      }
      default:
        throw new Error(`Unsupported EVM RPC method ${method}`);
    }
  }
  // EVM state checkpoints are sequential, as they are in a node's block executor.
  let queue: Promise<unknown> = Promise.resolve();
  function request(args: {
    method: string;
    params?: readonly unknown[] | Record<string, unknown>;
  }): Promise<unknown> {
    const result = queue.then(() => handleRequest(args));
    queue = result.catch(() => undefined);
    return result;
  }
  const provider = new ethers.BrowserProvider({ request }, undefined, { cacheTimeout: -1 });
  const signer = await provider.getSigner();
  async function deploy(path: string, name: string, args: readonly unknown[] = []) {
    const contract = artifact(path, name);
    const deployed = await new ethers.ContractFactory(
      contract.abi,
      `0x${contract.evm.bytecode.object}`,
      signer
    ).deploy(...args);
    await deployed.waitForDeployment();
    return new ethers.Contract(await deployed.getAddress(), contract.abi, signer);
  }
  async function proxy(path: string, name: string, fixed?: string) {
    const implementation = await deploy(path, name);
    const deployed = await deploy('meta-ag/proxy/ERC1967Proxy.sol', 'ERC1967Proxy', [
      await implementation.getAddress(),
      '0x',
    ]);
    if (fixed) {
      // Install the same deployed proxy code and implementation slot at the application's canonical address.
      await vm.stateManager.putCode(
        address(fixed),
        await vm.stateManager.getCode(address(await deployed.getAddress()))
      );
      const slot = hexToBytes('0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc');
      await vm.stateManager.putStorage(
        address(fixed),
        slot,
        hexToBytes(ethers.zeroPadValue(await implementation.getAddress(), 32) as `0x${string}`)
      );
    }
    return new ethers.Contract(
      fixed ?? (await deployed.getAddress()),
      artifact(path, name).abi,
      signer
    );
  }
  async function send(contract: ethers.Contract, name: string, ...args: unknown[]) {
    const tx: ethers.ContractTransactionResponse = await contract.getFunction(name).send(...args);
    const receipt = await tx.wait();
    if (receipt?.status !== 1) throw new Error(`Production fixture transaction failed: ${name}`);
    return receipt;
  }
  async function markets() {
    const wpax = artifact('WPAX9.sol', 'WPAX9');
    await vm.stateManager.putCode(
      address(TOKENS.USDL.address),
      hexToBytes(`0x${wpax.evm.deployedBytecode.object}`)
    );
    const usdl = new ethers.Contract(TOKENS.USDL.address, wpax.abi, signer);
    const ordinary = await deploy('WPAX9.sol', 'WPAX9');
    const config = await proxy('protocol/ProtocolConfig.sol', 'ProtocolConfig');
    const treasury = await proxy('protocol/Treasury.sol', 'Treasury');
    const registry = await proxy('data/PoolRegistry.sol', 'PoolRegistry', CONTRACTS.PoolRegistry);
    const accumulator = await proxy('data/FeeAccumulator.sol', 'FeeAccumulator');
    const nft = await proxy('core/SidioraNFT.sol', 'SidioraNFT');
    const poolImplementation = await deploy('core/SidioraPool.sol', 'SidioraPool');
    const beacon = await deploy('core/PoolBeacon.sol', 'PoolBeacon', [
      await poolImplementation.getAddress(),
      owner,
    ]);
    const factory = await proxy('core/SidioraFactory.sol', 'SidioraFactory');
    const router = await proxy('periphery/Router.sol', 'Router', CONTRACTS.Router);
    const quoter = await proxy('periphery/Quoter.sol', 'Quoter', CONTRACTS.Quoter);
    await send(config, 'initialize', TOKENS.USDL.address, ethers.ZeroAddress, owner);
    await send(config, 'setCreationFee', 0);
    await send(config, 'setFeeWeights', 0, 0, 0);
    await send(treasury, 'initialize', ethers.ZeroAddress, owner);
    await send(registry, 'initialize', ethers.ZeroAddress, owner);
    await send(
      accumulator,
      'initialize',
      await config.getAddress(),
      await treasury.getAddress(),
      CONTRACTS.PoolRegistry,
      ethers.ZeroAddress,
      TOKENS.USDL.address,
      owner
    );
    await send(nft, 'initialize', 'Sidiora', 'SIDNFT', ethers.ZeroAddress, owner);
    await send(
      factory,
      'initialize',
      await beacon.getAddress(),
      await nft.getAddress(),
      CONTRACTS.PoolRegistry,
      ethers.ZeroAddress,
      await config.getAddress(),
      await treasury.getAddress(),
      await accumulator.getAddress(),
      TOKENS.USDL.address,
      owner
    );
    await send(
      router,
      'initialize',
      await factory.getAddress(),
      CONTRACTS.PoolRegistry,
      await config.getAddress(),
      TOKENS.USDL.address,
      owner
    );
    await send(quoter, 'initialize', CONTRACTS.PoolRegistry, await config.getAddress(), owner);
    await send(registry, 'grantRole', ethers.id('FACTORY_ROLE'), await factory.getAddress());
    await send(accumulator, 'grantRole', ethers.id('FACTORY_ROLE'), await factory.getAddress());
    await send(nft, 'grantRole', ethers.id('MINTER_ROLE'), await factory.getAddress());
    await send(factory, 'grantRole', ethers.id('ROUTER_ROLE'), CONTRACTS.Router);
    await send(treasury, 'grantRole', ethers.id('DEPOSITOR_ROLE'), await accumulator.getAddress());
    await send(usdl, 'deposit', { value: ethers.parseEther('1') });
    await send(usdl, 'approve', CONTRACTS.Router, ethers.MaxUint256);
    const markets = [];
    for (const symbol of ['FIRST', 'SECOND']) {
      await send(factory, 'createMarket', symbol, symbol, 0, ethers.ZeroAddress);
      const pools: string[] = await registry.getPoolsByCreator(owner);
      const pool = pools[pools.length - 1];
      const poolContract = new ethers.Contract(
        pool,
        artifact('core/SidioraPool.sol', 'SidioraPool').abi,
        signer
      );
      const tokenAddress: string = await poolContract.tokenAddress();
      const tokenContract = new ethers.Contract(
        tokenAddress,
        artifact('core/SidioraERC20.sol', 'SidioraERC20').abi,
        signer
      );
      await send(router, 'buy', pool, ethers.parseUnits('1000', 6), 0, ethers.MaxUint256);
      await send(tokenContract, 'approve', CONTRACTS.Router, ethers.MaxUint256);
      markets.push({
        pool,
        contract: tokenContract,
        token: { address: tokenAddress, name: symbol, symbol, decimals: 6, isLaunchpad: true },
      });
    }
    return { router, registry, usdl, ordinary, markets, factory, treasury, accumulator, send };
  }
  return {
    provider,
    signer,
    owner,
    vm,
    common,
    markets,
    request,
    advanceTime(seconds: number) {
      timeOffset += seconds;
      timestamp += BigInt(seconds);
    },
  };
}

import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { setImmediate } from 'node:timers/promises';
import { ethers } from 'ethers';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { NATIVE_PAX } from './constants';
import { executeSwap, SwapRevertedError } from './executor';
import { CONTRACTS, SIDIORA_ROUTER_ABI, TOKENS, quoteBuy, quoteMultihop, quoteSell } from './sdk';
import type { SwapParams, SwapQuote } from './types';

let node: ChildProcess;
let provider: ethers.JsonRpcProvider | undefined;
let signer: ethers.JsonRpcSigner;
let output: ethers.Contract;
let quote: SwapQuote;
let params: SwapParams;

function artifact(path: string): {
  abi: ethers.InterfaceAbi;
  bytecode: string;
  deployedBytecode: string;
} {
  return JSON.parse(
    readFileSync(
      resolve(__dirname, '../../../../../contracts/launchpad/artifacts/contracts', path),
      'utf8'
    )
  );
}

async function installProxy(
  path: string,
  address = ethers.Wallet.createRandom().address
): Promise<ethers.Contract> {
  const contract = artifact(path);
  const implementation = await new ethers.ContractFactory(
    contract.abi,
    contract.bytecode,
    signer
  ).deploy();
  await implementation.waitForDeployment();
  const proxy = artifact('meta-ag/proxy/ERC1967Proxy.sol/ERC1967Proxy.json');
  await signer.provider.send('anvil_setCode', [address, proxy.deployedBytecode]);
  await signer.provider.send('anvil_setStorageAt', [
    address,
    '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc',
    ethers.zeroPadValue(await implementation.getAddress(), 32),
  ]);
  return new ethers.Contract(address, contract.abi, signer);
}

beforeEach(async () => {
  node = spawn(
    process.env.ANVIL_BIN ?? 'anvil',
    ['--host', '127.0.0.1', '--port', '0', '--chain-id', '125'],
    { stdio: ['ignore', 'pipe', 'pipe'] }
  );
  const url = await new Promise<string>((resolveUrl, reject) => {
    let stdout = '';
    let stderr = '';
    node.once('error', reject);
    node.once('exit', (code) => reject(new Error(`Anvil exited (${code}): ${stderr}`)));
    node.stderr?.on('data', (chunk: Buffer) => {
      stderr += chunk.toString();
    });
    node.stdout?.on('data', (chunk: Buffer) => {
      stdout += chunk.toString();
      const listening = /Listening on (127\.0\.0\.1:\d+)/.exec(stdout);
      if (listening) resolveUrl(`http://${listening[1]}`);
    });
  });
  provider = new ethers.JsonRpcProvider(url, 125, {
    staticNetwork: true,
    batchMaxCount: 1,
    cacheTimeout: -1,
  });
  provider.pollingInterval = 10;
  signer = await provider.getSigner();
  const owner = await signer.getAddress();
  const wrapped = artifact('WPAX9.sol/WPAX9.json');
  for (const address of [TOKENS.WPAX9.address, TOKENS.USDL.address]) {
    await provider.send('anvil_setCode', [address, wrapped.deployedBytecode]);
  }
  output = new ethers.Contract(TOKENS.USDL.address, wrapped.abi, signer);
  const oracle = await installProxy(
    'meta-ag/oracle/PriceOracle.sol/PriceOracle.json',
    CONTRACTS.PriceOracle
  );
  const vault = await installProxy(
    'meta-ag/vault/PECORVault.sol/PECORVault.json',
    CONTRACTS.PECORVault
  );
  const pecor = await installProxy('meta-ag/engine/PECOR.sol/PECOR.json', CONTRACTS.PECOR);
  await (await oracle.initialize(owner)).wait();
  await (await oracle.setRelayer(owner, true)).wait();
  await (await vault.initialize(TOKENS.WPAX9.address, ethers.ZeroAddress, owner)).wait();
  await (
    await pecor.initialize(
      CONTRACTS.PriceOracle,
      CONTRACTS.PECORVault,
      TOKENS.WPAX9.address,
      ethers.ZeroAddress,
      owner
    )
  ).wait();
  await (await pecor.setPriceImpact(false, 0)).wait();
  await (await vault.setOperator(CONTRACTS.PECOR, true)).wait();
  for (const address of [TOKENS.WPAX9.address, TOKENS.USDL.address]) {
    await (await vault.registerToken(address, address === TOKENS.USDL.address)).wait();
    await (
      await oracle.registerToken(address, 3600, 10000, 1, ethers.parseEther('10'), 3600)
    ).wait();
    await (await oracle.updatePrice(address, ethers.parseEther('1'))).wait();
  }
  await (await output.deposit({ value: ethers.parseEther('100') })).wait();
  await (await output.approve(CONTRACTS.PECORVault, ethers.MaxUint256)).wait();
  await (await vault.deposit(TOKENS.USDL.address, ethers.parseEther('100'))).wait();
  quote = {
    protocol: 'pecor',
    protocolLabel: 'PECOR',
    tokenIn: NATIVE_PAX,
    tokenOut: { ...TOKENS.USDL, decimals: 18 },
    amountIn: ethers.parseEther('1').toString(),
    amountOut: ethers.parseEther('1').toString(),
    amountOutDisplay: '1',
    fee: '0',
    feeBps: 0,
    priceImpact: 0,
    sufficient: true,
  };
  params = {
    tokenIn: quote.tokenIn,
    tokenOut: quote.tokenOut,
    amountIn: quote.amountIn,
    slippageBps: 0,
    recipient: owner,
  };
  await provider.send('evm_setAutomine', [false]);
}, 15000);

afterEach(async () => {
  provider?.destroy();
  provider = undefined;
  if (node && node.exitCode === null && node.signalCode === null) {
    const exited = once(node, 'exit');
    node.kill('SIGTERM');
    await exited;
  }
});

async function pendingSwap(): Promise<string> {
  let pending: { hash: string }[] = [];
  await expect
    .poll(async () => {
      pending = await signer.provider.send('eth_pendingTransactions', []);
      return pending.length;
    })
    .toBe(1);
  return pending[0].hash;
}

async function execute_swap_returns_after_the_receipt() {
  let settled = false;
  const swap = executeSwap(signer, quote, params);
  void swap.then(
    () => {
      settled = true;
    },
    () => {
      settled = true;
    }
  );
  const hash = await pendingSwap();
  expect(await signer.provider.getTransactionReceipt(hash)).toBeNull();
  await setImmediate();
  expect(settled).toBe(false);
  await signer.provider.send('evm_mine', []);
  const result = await swap;
  expect(result.hash).toBe(hash);
  expect(result.receipt).toBeInstanceOf(ethers.TransactionReceipt);
  expect(result.receipt.hash).toBe(hash);
  expect(result.receipt.status).toBe(1);
  expect(result.receipt.blockNumber).toBeGreaterThan(0);
  expect(await output.balanceOf(params.recipient)).toBe(ethers.parseEther('1'));
}

async function execute_swap_refuses_a_reverted_swap() {
  const swap = executeSwap(signer, quote, params);
  const rejection = expect(swap).rejects.toBeInstanceOf(SwapRevertedError);
  const hash = await pendingSwap();
  await signer.provider.send('evm_setNextBlockTimestamp', [Math.floor(Date.now() / 1000) + 301]);
  await signer.provider.send('evm_mine', []);
  await rejection;
  await expect(swap).rejects.toMatchObject({
    name: 'SwapRevertedError',
    receipt: { hash, status: 0 },
  });
  expect((await signer.provider.getTransactionReceipt(hash))?.status).toBe(0);
  expect(await output.balanceOf(params.recipient)).toBe(0n);
}

async function sidioraMarkets() {
  await signer.provider.send('evm_setAutomine', [true]);
  const owner = await signer.getAddress();
  const config = await installProxy('protocol/ProtocolConfig.sol/ProtocolConfig.json');
  const registry = await installProxy(
    'data/PoolRegistry.sol/PoolRegistry.json',
    CONTRACTS.PoolRegistry
  );
  const treasury = await installProxy('protocol/Treasury.sol/Treasury.json');
  const accumulator = await installProxy('data/FeeAccumulator.sol/FeeAccumulator.json');
  const nft = await installProxy('core/SidioraNFT.sol/SidioraNFT.json');
  const factory = await installProxy('core/SidioraFactory.sol/SidioraFactory.json');
  const router = await installProxy('periphery/Router.sol/Router.json', CONTRACTS.Router);
  const quoter = await installProxy('periphery/Quoter.sol/Quoter.json', CONTRACTS.Quoter);
  const poolArtifact = artifact('core/SidioraPool.sol/SidioraPool.json');
  const implementation = await new ethers.ContractFactory(
    poolArtifact.abi,
    poolArtifact.bytecode,
    signer
  ).deploy();
  await implementation.waitForDeployment();
  const beaconArtifact = artifact('core/PoolBeacon.sol/PoolBeacon.json');
  const beacon = await new ethers.ContractFactory(
    beaconArtifact.abi,
    beaconArtifact.bytecode,
    signer
  ).deploy(await implementation.getAddress(), owner);
  await beacon.waitForDeployment();
  await (await config.initialize(TOKENS.USDL.address, ethers.ZeroAddress, owner)).wait();
  await (await config.setCreationFee(0)).wait();
  await (await config.setFeeWeights(0, 0, 0)).wait();
  await (await registry.initialize(ethers.ZeroAddress, owner)).wait();
  await (await treasury.initialize(ethers.ZeroAddress, owner)).wait();
  await (await nft.initialize('Sidiora', 'SIDNFT', ethers.ZeroAddress, owner)).wait();
  await (
    await accumulator.initialize(
      await config.getAddress(),
      await treasury.getAddress(),
      CONTRACTS.PoolRegistry,
      ethers.ZeroAddress,
      TOKENS.USDL.address,
      owner
    )
  ).wait();
  await (
    await factory.initialize(
      await beacon.getAddress(),
      await nft.getAddress(),
      CONTRACTS.PoolRegistry,
      ethers.ZeroAddress,
      await config.getAddress(),
      await treasury.getAddress(),
      await accumulator.getAddress(),
      TOKENS.USDL.address,
      owner
    )
  ).wait();
  await (
    await router.initialize(
      await factory.getAddress(),
      CONTRACTS.PoolRegistry,
      await config.getAddress(),
      TOKENS.USDL.address,
      owner
    )
  ).wait();
  await (await quoter.initialize(CONTRACTS.PoolRegistry, await config.getAddress(), owner)).wait();
  for (const contract of [registry, accumulator]) {
    await (await contract.grantRole(ethers.id('FACTORY_ROLE'), await factory.getAddress())).wait();
  }
  await (await nft.grantRole(ethers.id('MINTER_ROLE'), await factory.getAddress())).wait();
  await (await factory.grantRole(ethers.id('ROUTER_ROLE'), CONTRACTS.Router)).wait();
  await (await output.deposit({ value: ethers.parseEther('1') })).wait();
  await (await output.approve(CONTRACTS.Router, ethers.MaxUint256)).wait();
  const markets = [];
  for (const symbol of ['FIRST', 'SECOND']) {
    await (await factory.createMarket(symbol, symbol, 0, ethers.ZeroAddress)).wait();
    const pools: string[] = await registry.getPoolsByCreator(owner);
    const pool = pools[pools.length - 1];
    const poolContract = new ethers.Contract(pool, poolArtifact.abi, signer);
    const address: string = await poolContract.tokenAddress();
    const token = new ethers.Contract(
      address,
      artifact('core/SidioraERC20.sol/SidioraERC20.json').abi,
      signer
    );
    await (await router.buy(pool, ethers.parseUnits('1000', 6), 0, ethers.MaxUint256)).wait();
    await (await token.approve(CONTRACTS.Router, ethers.MaxUint256)).wait();
    markets.push({
      pool,
      token: { address, symbol, name: symbol, decimals: 6, isLaunchpad: true },
    });
  }
  return { router, markets };
}

async function sidiora_swap_min_out_uses_confirmed_quote() {
  const { router, markets } = await sidioraMarkets();
  const market = markets[0];
  const slippageBps = 137;
  for (const isBuy of [false, true]) {
    const amountIn = ethers.parseUnits(isBuy ? '1' : '10000', 6);
    const getQuote = isBuy ? quoteBuy : quoteSell;
    const confirmed = await getQuote(market.pool, amountIn, signer.provider);
    const confirmedQuote: SwapQuote = {
      ...quote,
      protocol: 'sidiora',
      poolAddress: market.pool,
      tokenIn: isBuy ? TOKENS.USDL : market.token,
      tokenOut: isBuy ? market.token : TOKENS.USDL,
      amountIn: amountIn.toString(),
      amountOut: confirmed.amountOut.toString(),
    };
    await (await router.buy(market.pool, ethers.parseUnits('1', 6), 0, ethers.MaxUint256)).wait();
    const moved = await getQuote(market.pool, amountIn, signer.provider);
    expect(moved.amountOut).not.toBe(confirmed.amountOut);
    const expectedMinOut =
      (BigInt(confirmedQuote.amountOut) * BigInt(10000 - slippageBps)) / 10000n;
    expect((moved.amountOut * BigInt(10000 - slippageBps)) / 10000n).not.toBe(expectedMinOut);
    await signer.provider.send('evm_setAutomine', [false]);
    const swap = executeSwap(signer, confirmedQuote, {
      ...params,
      tokenIn: confirmedQuote.tokenIn,
      tokenOut: confirmedQuote.tokenOut,
      amountIn: confirmedQuote.amountIn,
      slippageBps,
    });
    const hash = await pendingSwap();
    const transaction = await signer.provider.getTransaction(hash);
    expect(transaction?.to).toBe(CONTRACTS.Router);
    const call = new ethers.Interface(SIDIORA_ROUTER_ABI).parseTransaction({
      data: transaction!.data,
    });
    expect(call?.name).toBe(isBuy ? 'buy' : 'sell');
    expect(call?.args[0]).toBe(market.pool);
    expect(call?.args[1]).toBe(amountIn);
    expect(call?.args[2]).toBe(expectedMinOut);
    await signer.provider.send('evm_mine', []);
    expect((await swap).receipt.status).toBe(1);
    await signer.provider.send('evm_setAutomine', [true]);
  }
}

async function sidiora_multihop_min_out_uses_confirmed_quote() {
  const { router, markets } = await sidioraMarkets();
  const [first, second] = markets;
  const amountIn = ethers.parseUnits('1', 6);
  const slippageBps = 213;
  const confirmed = await quoteMultihop(
    first.token.address,
    second.token.address,
    amountIn,
    signer.provider
  );
  const confirmedQuote: SwapQuote = {
    ...quote,
    protocol: 'sidiora-multihop',
    tokenIn: first.token,
    tokenOut: second.token,
    amountIn: amountIn.toString(),
    amountOut: confirmed.amountOut.toString(),
  };
  await (await router.buy(second.pool, ethers.parseUnits('1', 6), 0, ethers.MaxUint256)).wait();
  const moved = await quoteMultihop(
    first.token.address,
    second.token.address,
    amountIn,
    signer.provider
  );
  expect(moved.amountOut).not.toBe(confirmed.amountOut);
  const expectedMinOut = (BigInt(confirmedQuote.amountOut) * BigInt(10000 - slippageBps)) / 10000n;
  expect((moved.amountOut * BigInt(10000 - slippageBps)) / 10000n).not.toBe(expectedMinOut);
  let failure: unknown;
  try {
    await executeSwap(signer, confirmedQuote, {
      ...params,
      tokenIn: confirmedQuote.tokenIn,
      tokenOut: confirmedQuote.tokenOut,
      amountIn: confirmedQuote.amountIn,
      slippageBps,
    });
  } catch (error) {
    failure = error;
  }
  expect(ethers.isError(failure, 'CALL_EXCEPTION')).toBe(true);
  if (!ethers.isError(failure, 'CALL_EXCEPTION')) throw failure;
  expect(failure.action).toBe('estimateGas');
  expect(failure.transaction.to).toBe(CONTRACTS.Router);
  const call = new ethers.Interface(SIDIORA_ROUTER_ABI).parseTransaction({
    data: failure.transaction.data,
  });
  expect(call?.name).toBe('swapTokenToToken');
  expect(call?.args[0]).toBe(first.token.address);
  expect(call?.args[1]).toBe(second.token.address);
  expect(call?.args[2]).toBe(amountIn);
  expect(call?.args[3]).toBe(expectedMinOut);
}

test(execute_swap_returns_after_the_receipt);
test(execute_swap_refuses_a_reverted_swap);
test(sidiora_swap_min_out_uses_confirmed_quote, { timeout: 15000 });
test(sidiora_multihop_min_out_uses_confirmed_quote, { timeout: 15000 });

import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { setImmediate } from 'node:timers/promises';
import { ethers } from 'ethers';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { NATIVE_PAX } from './constants';
import { executeSwap, SwapRevertedError } from './executor';
import { CONTRACTS, TOKENS } from './sdk';
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

async function installProxy(path: string, address: string): Promise<ethers.Contract> {
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

test(execute_swap_returns_after_the_receipt);
test(execute_swap_refuses_a_reverted_swap);

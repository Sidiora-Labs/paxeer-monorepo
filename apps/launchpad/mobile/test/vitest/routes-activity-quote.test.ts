import { afterAll, beforeAll, expect, test } from 'vitest';
import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { readFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import {
  createPublicClient,
  createTestClient,
  createWalletClient,
  defineChain,
  encodeFunctionData,
  http,
  getAddress,
  pad,
  keccak256,
  parseAbi,
  toHex,
  zeroAddress,
  type Abi,
  type Address,
  type Hex,
} from 'viem';
import { validateContract } from '../../../../../sdk/launchpad/packages/sdk/src/validate.ts';
import { quote } from '../../src/runtime/routes/activity.ts';
import type { PublicConfiguration } from '../../src/runtime/service-client.ts';

const quoteAbi = parseAbi([
  'function quoteExactInput(address pool,uint256 amountIn,bool isBuy) view returns((uint256 amountOut,uint256 feeAmount,uint256 priceImpactBps))',
]);
const poolAbi = parseAbi([
  'function pause()',
  'function unpause()',
  'function syncReserves() returns(uint256,uint256)',
]);
let anvil: ChildProcess | undefined;
let fixture: Awaited<ReturnType<typeof deployFixture>>;

async function artifact(
  path: string,
): Promise<{ abi: Abi; bytecode: Hex; deployedBytecode: Hex }> {
  return JSON.parse(
    await readFile(
      new URL(
        `../../../../../contracts/launchpad/artifacts/contracts/${path}.json`,
        import.meta.url,
      ),
      'utf8',
    ),
  );
}

async function deployFixture(url: string) {
  const chain = defineChain({
    id: 125,
    name: 'Quote fixture',
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    rpcUrls: { default: { http: [url] } },
  });
  const client = createPublicClient({
    chain,
    transport: http(url, { retryCount: 0 }),
    cacheTime: 0,
    pollingInterval: 10,
  });
  const wallet = createWalletClient({
    chain,
    transport: http(url, { retryCount: 0 }),
  });
  const [account] = await wallet.getAddresses();
  if (!account) throw new Error('Anvil fixture account missing');
  async function mined(hash: Hex) {
    const receipt = await client.waitForTransactionReceipt({ hash });
    expect(receipt.status).toBe('success');
    return receipt;
  }
  async function deploy(path: string, args: readonly unknown[] = []) {
    const contract = await artifact(path);
    const receipt = await mined(
      await wallet.deployContract({ account, ...contract, args }),
    );
    if (!receipt.contractAddress) throw new Error('Deployment address missing');
    return getAddress(receipt.contractAddress);
  }
  async function proxy(path: string, args: readonly unknown[]) {
    const implementation = await deploy(path);
    const { abi } = await artifact(path);
    return deploy('meta-ag/proxy/ERC1967Proxy.sol/ERC1967Proxy', [
      implementation,
      encodeFunctionData({ abi, functionName: 'initialize', args }),
    ]);
  }
  async function write(
    address: Address,
    abi: Abi,
    functionName: string,
    args: readonly unknown[] = [],
    value = 0n,
  ) {
    return mined(
      await wallet.writeContract({
        account,
        address,
        abi,
        functionName,
        args,
        value,
      }),
    );
  }
  const token = await deploy('WPAX9.sol/WPAX9');
  const usdl = await deploy('WPAX9.sol/WPAX9');
  const config = await proxy('protocol/ProtocolConfig.sol/ProtocolConfig', [
    usdl,
    zeroAddress,
    account,
  ]);
  const registry = await proxy('data/PoolRegistry.sol/PoolRegistry', [
    zeroAddress,
    account,
  ]);
  const accumulator = await deploy('data/FeeAccumulator.sol/FeeAccumulator');
  const poolImplementation = await deploy('core/SidioraPool.sol/SidioraPool');
  const beacon = await deploy('base/UpgradeableBeacon.sol/UpgradeableBeacon', [
    poolImplementation,
    account,
  ]);
  const poolContract = await artifact('core/SidioraPool.sol/SidioraPool');
  const pool = await deploy('base/BeaconProxy.sol/BeaconProxy', [
    beacon,
    encodeFunctionData({
      abi: poolContract.abi,
      functionName: 'initialize',
      args: [
        token,
        usdl,
        zeroAddress,
        accumulator,
        zeroAddress,
        config,
        account,
        10_000_000_000n,
        1_000_000_000_000_000n,
      ],
    }),
  ]);
  const wrappedAbi = (await artifact('WPAX9.sol/WPAX9')).abi;
  for (const [asset, balance] of [
    [token, 1_000_000_000_000_000n],
    [usdl, 10_000_000_000n],
  ] as const) {
    await write(asset, wrappedAbi, 'deposit', [], balance);
    await write(asset, wrappedAbi, 'transfer', [pool, balance]);
  }
  await write(pool, poolAbi, 'syncReserves');
  const registryAbi = (await artifact('data/PoolRegistry.sol/PoolRegistry'))
    .abi;
  await write(registry, registryAbi, 'grantRole', [
    keccak256(toHex('FACTORY_ROLE')),
    account,
  ]);
  await write(registry, registryAbi, 'register', [
    pool,
    token,
    account,
    zeroAddress,
    1n,
  ]);
  const quoter = getAddress('0x793006f4F321b8863F26DBD75b11bAa01DA181dc');
  const testClient = createTestClient({
    chain,
    mode: 'anvil',
    transport: http(url),
  });
  const implementation = await deploy('periphery/Quoter.sol/Quoter');
  const proxyContract = await artifact(
    'meta-ag/proxy/ERC1967Proxy.sol/ERC1967Proxy',
  );
  await testClient.setCode({
    address: quoter,
    bytecode: proxyContract.deployedBytecode,
  });
  await testClient.setStorageAt({
    address: quoter,
    index: '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc',
    value: pad(implementation),
  });
  await write(
    quoter,
    (await artifact('periphery/Quoter.sol/Quoter')).abi,
    'initialize',
    [registry, config, account],
  );
  return {
    url,
    chain,
    client,
    pool,
    quoter,
  };
}

beforeAll(async () => {
  const url = await new Promise<string>((resolve, reject) => {
    let output = '';
    const timer = setTimeout(
      () => reject(new Error(`Anvil startup timed out: ${output}`)),
      10000,
    );
    anvil = spawn(
      process.env.ANVIL_BIN ?? 'anvil',
      ['--host', '127.0.0.1', '--port', '0', '--chain-id', '125'],
      { stdio: ['ignore', 'pipe', 'pipe'] },
    );
    anvil.once('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
    anvil.once('exit', (code) => {
      clearTimeout(timer);
      reject(new Error(`Anvil exited ${code}: ${output}`));
    });
    anvil.stderr!.on('data', (chunk: Buffer) => {
      output += chunk.toString();
    });
    anvil.stdout!.on('data', (chunk: Buffer) => {
      output += chunk.toString();
      const listening = /Listening on (127\.0\.0\.1:\d+)/.exec(output);
      if (listening) {
        clearTimeout(timer);
        resolve(`http://${listening[1]}`);
      }
    });
  });
  fixture = await deployFixture(url);
}, 30000);

afterAll(async () => {
  if (anvil && anvil.exitCode === null && anvil.signalCode === null) {
    const exited = once(anvil, 'exit');
    anvil.kill('SIGTERM');
    await exited;
  }
});

async function mobileQuoteIssuesNoHttpRequest() {
  const observed: { method: string | undefined; path: string | undefined }[] =
    [];
  const server = createServer((request, response) => {
    observed.push({ method: request.method, path: request.url });
    response.writeHead(410);
    response.end();
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  try {
    const bound = server.address();
    if (!bound || typeof bound === 'string')
      throw new Error('Loopback port missing');
    const origin = `http://127.0.0.1:${bound.port}`;
    const config: PublicConfiguration = {
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      streamOrigin: `ws://127.0.0.1:${bound.port}`,
      approvedStreamOrigins: [`ws://127.0.0.1:${bound.port}`],
      production: false,
      canonicalOrigin: origin,
      identityURI: `${origin}/identity`,
      manifestDigest: 'a'.repeat(64),
      manifestURL: `${origin}/manifest.json`,
      optionalFeatures: { chat: false, live: false },
      approvedPlaybackOrigins: [],
      walletConnect: {
        projectId: 'a'.repeat(32),
        walletLink: 'https://wallet.example.com',
        rpcURL: fixture.url,
        iconURL: 'https://wallet.example.com/icon.png',
      },
    };
    const services = { config };
    const signal = new AbortController().signal;
    for (const side of ['buy', 'sell'] as const) {
      for (const slippageBps of [1, 73, 1000]) {
        const inputRaw = side === 'buy' ? '100000007' : '1000000007';
        const result = await quote(
          services,
          { poolAddress: fixture.pool, side, inputRaw, slippageBps },
          signal,
        );
        const block = await fixture.client.getBlock({
          blockNumber: BigInt(result.watermark.height),
        });
        const expected = await fixture.client.readContract({
          address: fixture.quoter,
          abi: quoteAbi,
          functionName: 'quoteExactInput',
          args: [fixture.pool, BigInt(inputRaw), side === 'buy'],
          blockNumber: block.number,
        });
        expect(validateContract('Quote', result)).toBe(true);
        expect(expected.amountOut).toBeGreaterThan(0n);
        expect(expected.feeAmount).toBeGreaterThan(0n);
        expect(result).toMatchObject({
          poolAddress: fixture.pool,
          side,
          inputRaw,
          outputRaw: expected.amountOut.toString(),
          feeRaw: expected.feeAmount.toString(),
          minOutputRaw: (
            (expected.amountOut * BigInt(10000 - slippageBps)) /
            10000n
          ).toString(),
          validUntil: new Date(
            Number(block.timestamp + 30n) * 1000,
          ).toISOString(),
          watermark: {
            chainId: '125',
            height: block.number.toString(),
            blockHash: block.hash,
            integrity: 'healthy',
            observedAt: new Date(Number(block.timestamp) * 1000).toISOString(),
          },
        });
        expect(observed).toEqual([]);
      }
    }
    await expect(
      quote(
        services,
        {
          poolAddress: fixture.pool,
          side: 'buy',
          inputRaw: '1000000',
          slippageBps: 50,
        },
        AbortSignal.abort(),
      ),
    ).rejects.toThrow();
    expect(observed).toEqual([]);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}

test(mobileQuoteIssuesNoHttpRequest, mobileQuoteIssuesNoHttpRequest, 30000);

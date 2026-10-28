import { afterAll, beforeAll, expect, test } from 'vitest';
import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { readFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import {
  createPublicClient,
  createTestClient,
  createWalletClient,
  defineChain,
  encodeFunctionData,
  http,
  keccak256,
  parseAbi,
  toHex,
  zeroAddress,
  type Abi,
  type Address,
  type Hex,
} from 'viem';
import { validateContract } from '../../../../../sdk/launchpad/packages/sdk/src/validate.ts';
import { onchainQuote } from '../../src/runtime/onchain-quote.ts';

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

async function artifact(path: string): Promise<{ abi: Abi; bytecode: Hex }> {
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
    return receipt.contractAddress;
  }
  async function proxy(path: string, args: readonly unknown[]) {
    const implementation = await deploy(path);
    const { abi } = await artifact(path);
    return deploy('test/MockUUPS.sol/UUPSProxy', [
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
  const quoter = await proxy('periphery/Quoter.sol/Quoter', [
    registry,
    config,
    account,
  ]);
  return {
    url,
    chain,
    client,
    pool,
    quoter,
    write,
    testClient: createTestClient({
      chain,
      mode: 'anvil',
      transport: http(url),
    }),
  };
}

beforeAll(async () => {
  const socket = createServer();
  socket.listen(0, '127.0.0.1');
  await once(socket, 'listening');
  const bound = socket.address();
  if (!bound || typeof bound === 'string')
    throw new Error('Fixture port missing');
  await new Promise<void>((resolve, reject) =>
    socket.close((error) => (error ? reject(error) : resolve())),
  );
  anvil = spawn(
    process.env.ANVIL_BIN ?? 'anvil',
    ['--host', '127.0.0.1', '--port', String(bound.port), '--chain-id', '125'],
    { stdio: ['ignore', 'pipe', 'pipe'] },
  );
  await new Promise<void>((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error('Anvil startup timed out')),
      10000,
    );
    anvil!.once('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
    anvil!.once('exit', (code) => {
      clearTimeout(timer);
      reject(new Error(`Anvil exited: ${code}`));
    });
    anvil!.stdout!.on('data', (chunk: Buffer) => {
      if (chunk.toString().includes('Listening on')) {
        clearTimeout(timer);
        resolve();
      }
    });
    anvil!.stderr!.on('data', () => {});
  });
  fixture = await deployFixture(`http://127.0.0.1:${bound.port}`);
  expect(await fixture.client.getChainId()).toBe(125);
}, 30000);

afterAll(async () => {
  if (anvil && anvil.exitCode === null && anvil.signalCode === null) {
    const exited = once(anvil, 'exit');
    anvil.kill('SIGTERM');
    await exited;
  }
});

async function onchainQuoteReadsQuoterAtPinnedBlock() {
  for (const side of ['buy', 'sell'] as const) {
    for (const slippageBps of [1, 73, 1000]) {
      const requests: { method: string; params: unknown[] }[] = [];
      const client = createPublicClient({
        chain: fixture.chain,
        cacheTime: 0,
        transport: http(fixture.url, {
          retryCount: 0,
          onFetchRequest: async (request) => {
            requests.push(await request.clone().json());
          },
          onFetchResponse: async (response) => {
            const body = await response.clone().json();
            if (body.result?.number && body.result?.hash) {
              await fixture.testClient.increaseTime({ seconds: 60 });
              await fixture.write(fixture.pool, poolAbi, 'pause');
            }
          },
        }),
      });
      const inputRaw = side === 'buy' ? '100000007' : '1000000007';
      try {
        const quote = await onchainQuote(
          client,
          fixture.quoter,
          {
            pool: fixture.pool,
            side,
            inputRaw,
            slippageBps,
          },
          new AbortController().signal,
        );
        const height = BigInt(quote.watermark.height);
        const block = await fixture.client.getBlock({ blockNumber: height });
        const expected = await fixture.client.readContract({
          address: fixture.quoter,
          abi: quoteAbi,
          functionName: 'quoteExactInput',
          args: [fixture.pool, BigInt(inputRaw), side === 'buy'],
          blockNumber: height,
        });
        expect(validateContract('Quote', quote)).toBe(true);
        expect(expected.amountOut).toBeGreaterThan(0n);
        expect(expected.feeAmount).toBeGreaterThan(0n);
        expect(quote).toMatchObject({
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
            height: height.toString(),
            blockHash: block.hash,
            integrity: 'healthy',
          },
        });
        expect(
          await fixture.client.getBlockNumber({ cacheTime: 0 }),
        ).toBeGreaterThan(height);
        expect(requests.map((request) => request.method)).toEqual([
          'eth_chainId',
          'eth_getBlockByNumber',
          'eth_call',
        ]);
        expect(requests[1].params).toEqual(['latest', false]);
        expect(requests[2].params).toEqual([
          {
            to: fixture.quoter.toLowerCase(),
            data: encodeFunctionData({
              abi: quoteAbi,
              functionName: 'quoteExactInput',
              args: [fixture.pool, BigInt(inputRaw), side === 'buy'],
            }),
          },
          toHex(height),
        ]);
        await expect(
          fixture.client.readContract({
            address: fixture.quoter,
            abi: quoteAbi,
            functionName: 'quoteExactInput',
            args: [fixture.pool, BigInt(inputRaw), side === 'buy'],
          }),
        ).rejects.toThrow();
      } finally {
        await fixture.write(fixture.pool, poolAbi, 'unpause');
      }
    }
  }
}

async function onchainQuoteRejectsInvalidSlippage() {
  const requests: string[] = [];
  const client = createPublicClient({
    chain: fixture.chain,
    transport: http(fixture.url, {
      retryCount: 0,
      onFetchRequest: async (request) => {
        requests.push((await request.clone().json()).method);
      },
    }),
  });
  for (const slippageBps of [
    -1,
    0,
    1001,
    10000,
    1.5,
    NaN,
    Infinity,
    -Infinity,
  ]) {
    await expect(
      onchainQuote(
        client,
        fixture.quoter,
        {
          pool: fixture.pool,
          side: 'buy',
          inputRaw: '1000000',
          slippageBps,
        },
        new AbortController().signal,
      ),
    ).rejects.toBeInstanceOf(RangeError);
    expect(requests).toEqual([]);
  }
  for (const inputRaw of ['0', '-1']) {
    await expect(
      onchainQuote(
        client,
        fixture.quoter,
        {
          pool: fixture.pool,
          side: 'buy',
          inputRaw,
          slippageBps: 50,
        },
        new AbortController().signal,
      ),
    ).rejects.toBeInstanceOf(RangeError);
    expect(requests).toEqual([]);
  }
  await expect(
    onchainQuote(
      client,
      fixture.quoter,
      {
        pool: fixture.pool,
        side: 'buy',
        inputRaw: '1000000',
        slippageBps: 50,
      },
      AbortSignal.abort(),
    ),
  ).rejects.toThrow();
  expect(requests).toEqual([]);
}

test(
  onchainQuoteReadsQuoterAtPinnedBlock,
  onchainQuoteReadsQuoterAtPinnedBlock,
  30000,
);
test(onchainQuoteRejectsInvalidSlippage, onchainQuoteRejectsInvalidSlippage);

/* Proves: onchain-quote.ts:onchainQuote -> viem HTTP JSON-RPC -> activity.ts:activityRoutes.quote / service-client.ts:MobileServices.
 * A closed and reopened disk journal feeds createNativeQuoteTransactionBuilder,
 * NativeApprovalController, MarketSession and accountRoutes consumers.
 * Repairs: eager React Native imports made service-client.ts and
 * quote-transaction.ts unusable by public module consumers; onchain-quote.ts and
 * routes/activity.ts accepted noncanonical amounts before crossing RPC;
 * native-authority.ts accepted dirty ERC-1967 address words and empty pin sets;
 * market-session.ts retried malformed subscriptions instead of refusing them.
 * Public quote state and pending operations below are emitted by real producers.
 * The disk adapter performs actual file I/O; the wallet adapter signs and sends
 * through the real node. No network, contract, SDK or storage layer is mocked.
 * contracts/launchpad/contracts/periphery/Quoter.sol: Quoter is a production
 * ERC-1967 implementation, whereas the admitted Chain125 deployment uses native
 * surfaces. The native builder must refuse that different runtime, even when
 * its quote is valid; this proof preserves that authority boundary.
 */
import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import {
  mkdir,
  mkdtemp,
  open,
  readFile,
  readdir,
  rename,
  rm,
  writeFile,
  type FileHandle,
} from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterAll, beforeAll, expect, test } from 'vitest';
import {
  createPublicClient,
  createTestClient,
  createWalletClient,
  defineChain,
  encodeFunctionData,
  getAddress,
  http,
  keccak256,
  pad,
  parseAbi,
  toHex,
  zeroAddress,
  type Abi,
  type Address,
  type Hex,
} from 'viem';
import {
  decodeContract,
  SdkClient,
  type Quote,
  type TransactionRequest,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import {
  MobileServices,
  type PublicConfiguration,
} from '../../src/runtime/service-client.ts';
import { MarketSession } from '../../src/runtime/market-session.ts';
import { onchainQuote } from '../../src/runtime/onchain-quote.ts';
import {
  verifyRolePins,
  type RolePin,
} from '../../src/runtime/native-authority.ts';
import {
  createNativeQuoteTransactionBuilder,
  NativeApprovalController,
} from '../../src/runtime/quote-transaction.ts';
import { accountRoutes } from '../../src/runtime/routes/account.ts';
import { activityRoutes } from '../../src/runtime/routes/activity.ts';
import { PendingOperations } from '../../src/runtime/deep-links.ts';
import {
  SessionController,
  type CredentialStore,
} from '../../src/runtime/session.ts';
import { SnapshotCache } from '../../src/runtime/offline-cache.ts';
import type { WalletExecutionProvider } from '../../src/runtime/lifecycle.ts';

const deployment: { sourceManifestSha256: string } = JSON.parse(
  await readFile(
    new URL(
      '../../../../../sdk/launchpad/shared/config/deployment/current.json',
      import.meta.url,
    ),
    'utf8',
  ),
);

type StorageKey = Parameters<CredentialStore['get']>[0];
class DiskCredentialStore implements CredentialStore {
  private closed = false;
  private constructor(
    readonly directory: string,
    private readonly handle: FileHandle,
  ) {}
  static async open(directory: string) {
    await mkdir(directory, { recursive: true, mode: 0o700 });
    return new DiskCredentialStore(directory, await open(directory, 'r'));
  }
  private path(key: StorageKey) {
    if (this.closed) throw new Error('Credential store is closed');
    return join(this.directory, key);
  }
  async get(key: StorageKey) {
    try {
      return await readFile(this.path(key), 'utf8');
    } catch (error) {
      if (
        error &&
        typeof error === 'object' &&
        'code' in error &&
        error.code === 'ENOENT'
      )
        return null;
      throw error;
    }
  }
  async set(key: StorageKey, value: string) {
    const path = this.path(key);
    await writeFile(`${path}.next`, value, { mode: 0o600 });
    await rename(`${path}.next`, path);
  }
  async remove(key: StorageKey) {
    await rm(this.path(key), { force: true });
  }
  async clear() {
    if (this.closed) throw new Error('Credential store is closed');
    for (const key of await readdir(this.directory))
      await rm(join(this.directory, key));
  }
  async close() {
    if (this.closed) return;
    this.closed = true;
    await this.handle.close();
  }
}

const quoterAddress = getAddress('0x793006f4F321b8863F26DBD75b11bAa01DA181dc');
const implementationSlot =
  '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc';
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
let directory: string;

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
    name: 'Launchpad proof',
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    rpcUrls: { default: { http: [url] } },
  });
  const client = createPublicClient({
    chain,
    cacheTime: 0,
    pollingInterval: 10,
    transport: http(url, { retryCount: 0 }),
  });
  const wallet = createWalletClient({
    chain,
    transport: http(url, { retryCount: 0 }),
  });
  const testClient = createTestClient({
    chain,
    mode: 'anvil',
    transport: http(url, { retryCount: 0 }),
  });
  const [account] = await wallet.getAddresses();
  if (!account) throw new Error('Real node account required');
  async function mined(hash: Hex) {
    const receipt = await client.waitForTransactionReceipt({ hash });
    expect(receipt.status).toBe('success');
    return receipt;
  }
  async function deploy(path: string, args: readonly unknown[] = []) {
    const contract = await artifact(path);
    const receipt = await mined(
      await wallet.deployContract({
        account,
        abi: contract.abi,
        bytecode: contract.bytecode,
        args,
      }),
    );
    if (!receipt.contractAddress) throw new Error('Missing deployed address');
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
  const quoterImplementation = await deploy('periphery/Quoter.sol/Quoter');
  const proxyContract = await artifact(
    'meta-ag/proxy/ERC1967Proxy.sol/ERC1967Proxy',
  );
  await testClient.setCode({
    address: quoterAddress,
    bytecode: proxyContract.deployedBytecode,
  });
  await testClient.setStorageAt({
    address: quoterAddress,
    index: implementationSlot,
    value: pad(quoterImplementation),
  });
  await write(
    quoterAddress,
    (await artifact('periphery/Quoter.sol/Quoter')).abi,
    'initialize',
    [registry, config, account],
  );
  const implementationCode = await client.getCode({
    address: quoterImplementation,
  });
  if (!implementationCode) throw new Error('Missing implementation bytecode');
  const pin: RolePin = {
    address: quoterAddress,
    kind: 'erc1967',
    runtimeHash: keccak256(proxyContract.deployedBytecode),
    implementation: quoterImplementation,
    implementationHash: keccak256(implementationCode),
  };
  return {
    url,
    chain,
    client,
    wallet,
    testClient,
    account,
    pool,
    pin,
    write,
    mined,
  };
}

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), 'launchpad-mobile-proof-'));
  const url = await new Promise<string>((resolve, reject) => {
    let output = '';
    const timer = setTimeout(
      () => reject(new Error(`Anvil startup timed out: ${output}`)),
      10000,
    );
    anvil = spawn(
      'anvil',
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
      const match = /Listening on (127\.0\.0\.1:\d+)/.exec(output);
      if (match) {
        clearTimeout(timer);
        resolve(`http://${match[1]}`);
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
  if (directory) await rm(directory, { recursive: true, force: true });
});

function configuration(): PublicConfiguration {
  const origin = 'https://mobile.example.com';
  return {
    apiOrigin: origin,
    identityOrigin: origin,
    mediaOrigin: origin,
    canonicalOrigin: origin,
    identityURI: `${origin}/identity`,
    streamOrigin: 'wss://mobile.example.com',
    approvedStreamOrigins: ['wss://mobile.example.com'],
    production: true,
    manifestDigest: deployment.sourceManifestSha256,
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
}

class RpcWallet implements WalletExecutionProvider {
  readonly supportedChain = 125;
  readonly connectedAccount = fixture.account;
  constructor(readonly services: MobileServices) {}
  async connect(signal: AbortSignal) {
    signal.throwIfAborted();
    return { address: this.connectedAccount, proofKind: 'eoa_eip191' as const };
  }
  async signMessage(message: string, signal: AbortSignal) {
    signal.throwIfAborted();
    return fixture.wallet.signMessage({
      account: this.connectedAccount,
      message,
    });
  }
  async submitApproval(
    request: TransactionRequest,
    nonce: number,
    signal: AbortSignal,
  ) {
    signal.throwIfAborted();
    const transactionHash = await fixture.wallet.sendTransaction({
      account: this.connectedAccount,
      to: getAddress(request.to),
      data: request.data as Hex,
      value: BigInt(request.valueRaw),
      nonce,
    });
    return { transactionHash };
  }
  async execute(
    input: { requestId: string; quote: Quote; returnURL: string },
    signal: AbortSignal,
  ) {
    const request = await createNativeQuoteTransactionBuilder(this.services)(
      input.quote,
      this.connectedAccount,
      signal,
    );
    expect(request.chainId).toBe('125');
    return this.submitApproval(
      { ...request, chainId: '125' },
      await fixture.client.getTransactionCount({
        address: this.connectedAccount,
      }),
      signal,
    );
  }
}

async function proofLaunchpadMobileQuoteSurvivesReopenAndRefusesPausedPool() {
  const services = new MobileServices(configuration());
  const signal = new AbortController().signal;
  const body = {
    poolAddress: fixture.pool,
    side: 'buy' as const,
    inputRaw: '100000007',
    slippageBps: 73,
  };
  const produced = await services.quote(body, body.slippageBps, signal);
  const direct = await onchainQuote(
    fixture.client,
    quoterAddress,
    {
      pool: fixture.pool,
      side: body.side,
      inputRaw: body.inputRaw,
      slippageBps: body.slippageBps,
    },
    signal,
  );
  const routed = await activityRoutes.quote(services, body, signal);
  expect(direct).toEqual(produced);
  expect(routed).toEqual(produced);
  expect(await services.quote(body, body.slippageBps, signal)).toEqual(
    produced,
  );
  const pinned = await fixture.client.readContract({
    address: quoterAddress,
    abi: quoteAbi,
    functionName: 'quoteExactInput',
    args: [fixture.pool, BigInt(body.inputRaw), true],
    blockNumber: BigInt(produced.watermark.height),
  });
  expect(produced.outputRaw).toBe(pinned.amountOut.toString());
  expect(produced.feeRaw).toBe(pinned.feeAmount.toString());
  expect(BigInt(produced.outputRaw)).toBeGreaterThan(0n);
  expect(produced.minOutputRaw).toBe(
    ((pinned.amountOut * 9927n) / 10000n).toString(),
  );
  await verifyRolePins(
    fixture.client,
    [fixture.pin],
    BigInt(produced.watermark.height),
  );
  const path = join(directory, 'quote.json');
  const writer = await open(path, 'wx', 0o600);
  try {
    await writer.writeFile(JSON.stringify(produced));
    await writer.sync();
  } finally {
    await writer.close();
  }
  const reader = await open(path, 'r');
  let recovered: Quote;
  try {
    recovered = decodeContract(
      'Quote',
      JSON.parse(await reader.readFile('utf8')),
    );
  } finally {
    await reader.close();
  }
  expect(recovered).toEqual(produced);
  // Serve only the bytes emitted above: the HTTP crossing never invents a
  // response or substitutes an SDK/native transport implementation.
  const requests: string[] = [];
  const server = createServer((request, response) => {
    requests.push(request.url ?? '');
    void readFile(path)
      .then((bytes) => {
        response.writeHead(200, { 'Content-Type': 'application/json' });
        response.end(bytes);
      })
      .catch((error) => {
        response.destroy(error);
      });
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  try {
    const bound = server.address();
    if (!bound || typeof bound === 'string')
      throw new Error('Proof HTTP port missing');
    const origin = `http://127.0.0.1:${bound.port}`;
    const streamOrigin = `ws://127.0.0.1:${bound.port}`;
    const httpServices = new MobileServices({
      ...configuration(),
      production: false,
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      canonicalOrigin: origin,
      identityURI: `${origin}/identity`,
      streamOrigin,
      approvedStreamOrigins: [streamOrigin],
      manifestURL: `${origin}/manifest.json`,
    });
    expect(
      await httpServices.sdk.request('/v1/proof/quote', 'Quote', { signal }),
    ).toEqual(recovered);
    expect(requests).toEqual(['/v1/proof/quote']);
    const states: string[] = [];
    const snapshots: unknown[] = [];
    const market = new MarketSession(httpServices, {
      snapshot(value) {
        snapshots.push(value);
      },
      event(value) {
        throw new Error(`Unexpected event: ${value.eventId}`);
      },
      state(value) {
        states.push(value);
        if (value === 'stale') market.background();
      },
    });
    try {
      // A real Quote document is valid input to its consumer above, but is
      // refused by the snapshot consumer's real schema validator.
      await market.foreground([`market.stats.${fixture.pool}`]);
      expect(states).toEqual(['closed', 'loading', 'stale', 'closed']);
      expect(snapshots).toEqual([]);
      expect(requests[1]).toBe(
        `/v2/snapshots?topics=market.stats.${fixture.pool}`,
      );
      await expect(httpServices.manifest(signal)).rejects.toThrow(
        'Deployment manifest changed',
      );
      await expect(
        createNativeQuoteTransactionBuilder(httpServices)(
          recovered,
          fixture.account,
          signal,
        ),
      ).rejects.toThrow('Deployment manifest changed');
    } finally {
      market.background();
    }
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
  const builder = createNativeQuoteTransactionBuilder(services);
  await expect(builder(recovered, 'wrong-signer', signal)).rejects.toThrow();
  await expect(
    builder(recovered, fixture.account, AbortSignal.abort()),
  ).rejects.toThrow();
  // The real deployment manifest cannot be fetched after cancellation; no native
  // bridge or RPC call may occur for this consumer refusal.
  await expect(services.manifest(AbortSignal.abort())).rejects.toThrow();
  for (const inputRaw of ['01', '0x10', ' 1', (1n << 256n).toString()]) {
    await expect(
      services.quote({ ...body, inputRaw }, body.slippageBps, signal),
    ).rejects.toThrow();
    await expect(
      onchainQuote(
        fixture.client,
        quoterAddress,
        { pool: fixture.pool, side: 'buy', inputRaw, slippageBps: 73 },
        signal,
      ),
    ).rejects.toThrow();
  }
  await fixture.write(fixture.pool, poolAbi, 'pause');
  try {
    await expect(
      services.quote(body, body.slippageBps, signal),
    ).rejects.toThrow();
  } finally {
    await fixture.write(fixture.pool, poolAbi, 'unpause');
  }
}

async function proofLaunchpadMobilePendingProducerReopensWithoutDuplicateSubmission() {
  const services = new MobileServices(configuration());
  const signal = new AbortController().signal;
  const quote = await services.quote(
    { poolAddress: fixture.pool, side: 'sell', inputRaw: '1000000007' },
    50,
    signal,
  );
  const origin = services.config.canonicalOrigin;
  const storeDirectory = join(directory, 'credentials');
  const producerStore = await DiskCredentialStore.open(storeDirectory);
  const producer = new PendingOperations(producerStore, origin);
  await producer.begin({
    requestId: quote.quoteId,
    chainId: 125,
    startedAt: Date.now(),
    returnRoute: '/portfolio',
    state: 'awaiting_wallet',
  });
  const emitted = await producerStore.get('pending');
  expect(emitted).not.toBeNull();
  await producerStore.close();
  await expect(producerStore.get('pending')).rejects.toThrow('closed');
  const consumerStore = await DiskCredentialStore.open(storeDirectory);
  try {
    const pending = new PendingOperations(consumerStore, origin);
    await pending.restore();
    expect(pending.current).toEqual({ ...producer.current, state: 'unknown' });
    await expect(
      pending.begin({ ...producer.current!, state: 'awaiting_wallet' }),
    ).rejects.toThrow('Reconcile outstanding wallet operation');
    const session = new SessionController(consumerStore, new SnapshotCache());
    const wallet = new RpcWallet(services);
    expect((await wallet.connect(signal)).address).toBe(fixture.account);
    let changes = 0;
    const approval = new NativeApprovalController(
      services,
      session,
      pending,
      wallet,
      () => {
        changes++;
      },
      () => true,
    );
    await approval.restore();
    expect(approval.current).toBeUndefined();
    expect(changes).toBe(1);
    expect(approval.blocking).toBe(false);
    await expect(approval.prepare(quote)).rejects.toThrow(
      'Reconcile the outstanding financial request',
    );
    await expect(approval.submit()).rejects.toThrow(
      'No reviewed native approval step is ready',
    );
    await pending.receive(`${origin}/wallet/return?requestId=${quote.quoteId}`);
    expect(pending.current?.state).toBe('unknown');
    await consumerStore.close();
    const reopened = await DiskCredentialStore.open(storeDirectory);
    try {
      const restored = new PendingOperations(reopened, origin);
      await restored.restore();
      expect(restored.current).toEqual(pending.current);
      await expect(
        restored.receive(`${origin}/wallet/return?requestId=${quote.quoteId}`),
      ).rejects.toThrow('UNRELATED_OR_DUPLICATE_LINK');
      expect(restored.route(`${origin}/token/${quote.poolAddress}`)).toEqual({
        name: 'token',
        address: quote.poolAddress,
      });
      await restored.clearConfirmed();
      expect(await reopened.get('pending')).toBeNull();
    } finally {
      await reopened.close();
    }
  } finally {
    await consumerStore.close();
    await rm(storeDirectory, { recursive: true, force: true });
  }
}

async function proofLaunchpadMobileAuthorityAndRouteRefusals() {
  const signal = new AbortController().signal;
  await expect(verifyRolePins(fixture.client, [], 1n)).rejects.toThrow(
    'Mined block and native role pins required',
  );
  await expect(
    verifyRolePins(fixture.client, [fixture.pin], -1n),
  ).rejects.toThrow('Mined block and native role pins required');
  const validBlock = await fixture.client.getBlockNumber();
  await verifyRolePins(fixture.client, [fixture.pin], validBlock);
  await expect(
    verifyRolePins(
      fixture.client,
      [
        {
          address: quoterAddress,
          kind: 'surface',
          runtimeHash:
            '0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f',
        },
      ],
      validBlock,
    ),
  ).rejects.toThrow('Native trading code identity changed');
  if (fixture.pin.kind !== 'erc1967')
    throw new Error('Expected real proxy role');
  const validWord = pad(fixture.pin.implementation);
  // Anvil's administrative write updates the current block's state. Advance
  // first so the independently verified historical checkpoint stays intact.
  await fixture.testClient.mine({ blocks: 1 });
  await fixture.testClient.setStorageAt({
    address: quoterAddress,
    index: implementationSlot,
    value: `0x01${validWord.slice(4)}`,
  });
  await fixture.testClient.mine({ blocks: 1 });
  try {
    await expect(
      verifyRolePins(
        fixture.client,
        [fixture.pin],
        await fixture.client.getBlockNumber(),
      ),
    ).rejects.toThrow('Native trading code identity changed');
  } finally {
    await fixture.testClient.setStorageAt({
      address: quoterAddress,
      index: implementationSlot,
      value: validWord,
    });
    await fixture.testClient.mine({ blocks: 1 });
  }
  await verifyRolePins(fixture.client, [fixture.pin], validBlock);

  const services = new MobileServices(configuration());
  await expect(services.portfolio(AbortSignal.abort())).rejects.toThrow();
  await expect(
    accountRoutes.portfolio(new SdkClient(configuration()), signal),
  ).rejects.toThrow('Session required');
  await expect(
    activityRoutes.streams(services.sdk, AbortSignal.abort()),
  ).rejects.toThrow();
  expect(() =>
    activityRoutes.receipt(services.sdk, 'unknown-transaction', signal),
  ).toThrow('Invalid transaction hash');
  const states: string[] = [];
  const market = new MarketSession(services, {
    snapshot() {
      throw new Error('Malformed subscriptions must not install a snapshot');
    },
    event() {
      throw new Error('Malformed subscriptions must not deliver an event');
    },
    state(value) {
      states.push(value);
    },
  });
  try {
    for (const topics of [
      ['not-a-market-topic'],
      ['market.stats.not-an-address'],
      ['market.rankings.unknown'],
      [`market.candles.${fixture.pool}.2m`],
      [`market.stats.${fixture.pool}`, 'not-a-market-topic'],
      [],
    ]) {
      await expect(market.foreground(topics)).rejects.toThrow();
      expect(states).toEqual([]);
    }
    market.background();
    expect(states).toEqual(['closed']);
  } finally {
    market.background();
  }
}

test(
  proofLaunchpadMobileQuoteSurvivesReopenAndRefusesPausedPool,
  proofLaunchpadMobileQuoteSurvivesReopenAndRefusesPausedPool,
  30000,
);
test(
  proofLaunchpadMobilePendingProducerReopensWithoutDuplicateSubmission,
  proofLaunchpadMobilePendingProducerReopensWithoutDuplicateSubmission,
  30000,
);
test(
  proofLaunchpadMobileAuthorityAndRouteRefusals,
  proofLaunchpadMobileAuthorityAndRouteRefusals,
  30000,
);

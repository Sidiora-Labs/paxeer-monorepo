import { createServer, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createHash } from 'node:crypto';
import { readFileSync, realpathSync, statSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from 'vitest';
import { parseCLI } from 'vitest/node';
import {
  createPublicClient,
  decodeFunctionData,
  encodeAbiParameters,
  encodeEventTopics,
  encodeFunctionData,
  encodeFunctionResult,
  erc20Abi,
  http,
  keccak256,
  parseAbi,
  parseAbiParameters,
  type Address,
  type Hash,
} from 'viem';
import {
  ChainTradeProvider,
  transactionFingerprint,
  tradeRouterAbi,
  type TradeDeployment,
} from './providers';
import {
  PendingAttemptStore,
  RECEIPT_LOOKUP_DEADLINE_MS,
  ReceiptRecoveryScheduler,
  type PendingStorage,
  type RecoveryObservation,
  type RecoveryUpdate,
} from './recovery';
import type { PendingAttemptRecord } from './domain';
import native from './native-runner.json';
import phase2 from './vitest.phase2.config';

const directory = dirname(fileURLToPath(import.meta.url));
const account = `0x${'11'.repeat(20)}` as Address;
const token = `0x${'22'.repeat(20)}` as Address;
const pool = `0x${'33'.repeat(20)}` as Address;
const router = `0x${'44'.repeat(20)}` as Address;
const quoter = `0x${'55'.repeat(20)}` as Address;
const registry = `0x${'66'.repeat(20)}` as Address;
const emitter = `0x${'77'.repeat(20)}` as Address;
const implementation = `0x${'88'.repeat(20)}` as Address;
const code = '0x6001600055';
const blockHash = `0x${'bb'.repeat(32)}` as Hash;
const poolId = `0x${pool.slice(2).padStart(64, '0')}` as Hash;
const swapAbi = parseAbi([
  'event Swap(bytes32 indexed poolId,address indexed sender,bool isBuy,uint256 amountIn,uint256 amountOut,uint256 fee,uint256 price,uint256 timestamp,uint256 blockNumber)',
]);
const registryAbi = parseAbi([
  'function isRegisteredPool(address pool) view returns(bool)',
]);
const poolAbi = parseAbi(['function tokenAddress() view returns(address)']);
const deployment: TradeDeployment = {
  revision: 'receipt-loopback',
  router,
  quoter,
  registry,
  emitter,
  usdl: token,
  pins: [router, quoter, registry, emitter].map((address) => ({
    address,
    runtimeHash: keccak256(code),
    ...(address === router
      ? { implementation, implementationHash: keccak256(code) }
      : {}),
  })),
};
const pause = (ms: number) => new Promise<void>((done) => setTimeout(done, ms));
async function until(condition: () => boolean, timeout = 3000) {
  const deadline = Date.now() + timeout;
  while (!condition()) {
    if (Date.now() >= deadline)
      throw new Error('Receipt recovery did not reach its observable state');
    await pause(5);
  }
}

/** The loopback peer owns a bounded transaction ledger and emits ABI/RPC wire values.
 * All reads, receipt formatting and evidence validation run through production viem/provider code. */
async function receiptLedger() {
  const records = new Map<Hash, PendingAttemptRecord>();
  const calls: Array<{ method: string; params: unknown[] }> = [];
  let active = 0,
    maximum = 0,
    aborted = 0,
    held = 0;
  let stall: (
    method: string,
    params: unknown[],
    index: number,
  ) => boolean = () => false;
  let partialBody = false;
  const serverErrors: Error[] = [];
  const respond = (method: string, params: unknown[]): unknown => {
    if (method === 'eth_chainId') return '0x7d';
    if (
      method === 'eth_getTransactionReceipt' ||
      method === 'eth_getTransactionByHash'
    ) {
      const record = records.get(params[0] as Hash);
      if (!record) return null;
      const approve = record.action === 'approve';
      const input = approve
        ? encodeFunctionData({
            abi: erc20Abi,
            functionName: 'approve',
            args: [router, 10n],
          })
        : encodeFunctionData({
            abi: tradeRouterAbi,
            functionName: 'buy',
            args: [pool, 10n, 19n, 9999999999n],
          });
      if (method === 'eth_getTransactionByHash')
        return {
          hash: record.txHash,
          from: account,
          to: approve ? token : router,
          input,
          value: '0x0',
          nonce: `0x${record.nonce!.toString(16)}`,
          blockNumber: '0x2',
          blockHash,
          transactionIndex: '0x0',
          gas: '0x186a0',
          gasPrice: '0x1',
          type: '0x0',
          v: '0x1b',
          r: `0x${'01'.repeat(32)}`,
          s: `0x${'02'.repeat(32)}`,
        };
      return {
        transactionHash: record.txHash,
        transactionIndex: '0x0',
        blockHash,
        blockNumber: '0x2',
        from: account,
        to: approve ? token : router,
        status: '0x1',
        type: '0x0',
        cumulativeGasUsed: '0x5208',
        gasUsed: '0x5208',
        effectiveGasPrice: '0x1',
        contractAddress: null,
        logsBloom: `0x${'00'.repeat(256)}`,
        logs: approve
          ? []
          : [
              {
                address: emitter,
                transactionHash: record.txHash,
                transactionIndex: '0x0',
                blockHash,
                blockNumber: '0x2',
                logIndex: '0x0',
                removed: false,
                topics: encodeEventTopics({
                  abi: swapAbi,
                  eventName: 'Swap',
                  args: { poolId, sender: account },
                }),
                data: encodeAbiParameters(
                  parseAbiParameters(
                    'bool,uint256,uint256,uint256,uint256,uint256,uint256',
                  ),
                  [true, 10n, 20n, 1n, 2n, 1n, 2n],
                ),
              },
            ],
      };
    }
    if (method === 'eth_getBlockByNumber')
      return {
        hash: blockHash,
        number: '0x2',
        parentHash: `0x${'aa'.repeat(32)}`,
        timestamp: '0x1',
        transactions: [],
        gasLimit: '0x1c9c380',
        gasUsed: '0x5208',
        size: '0x100',
        difficulty: '0x0',
        totalDifficulty: '0x0',
        extraData: '0x',
        nonce: '0x0000000000000000',
        miner: account,
        logsBloom: `0x${'00'.repeat(256)}`,
        receiptsRoot: blockHash,
        stateRoot: blockHash,
        transactionsRoot: blockHash,
        sha3Uncles: blockHash,
        uncles: [],
      };
    if (method === 'eth_getCode') {
      if (
        ![router, quoter, registry, emitter, implementation, token].includes(
          params[0] as Address,
        )
      )
        throw new Error('Unknown code identity');
      return code;
    }
    if (method === 'eth_getStorageAt')
      return `0x${implementation.slice(2).padStart(64, '0')}`;
    if (method === 'eth_call') {
      const call = params[0] as { to: Address; data: `0x${string}` };
      if (call.to.toLowerCase() === registry) {
        const decoded = decodeFunctionData({
          abi: registryAbi,
          data: call.data,
        });
        if (decoded.args[0].toLowerCase() !== pool)
          throw new Error('Unknown pool');
        return encodeFunctionResult({
          abi: registryAbi,
          functionName: 'isRegisteredPool',
          result: true,
        });
      }
      if (call.to.toLowerCase() === pool) {
        decodeFunctionData({ abi: poolAbi, data: call.data });
        return encodeFunctionResult({
          abi: poolAbi,
          functionName: 'tokenAddress',
          result: token,
        });
      }
      if (call.to.toLowerCase() === token) {
        const decoded = decodeFunctionData({ abi: erc20Abi, data: call.data });
        if (decoded.functionName !== 'decimals')
          throw new Error('Unsupported token read');
        return encodeFunctionResult({
          abi: erc20Abi,
          functionName: 'decimals',
          result: 18,
        });
      }
    }
    throw new Error(`Unsupported ledger read: ${method}`);
  };
  const server = createServer((request, response: ServerResponse) => {
    const chunks: Buffer[] = [];
    request.on('data', (chunk: Buffer) => chunks.push(chunk));
    request.on('end', () => {
      active++;
      maximum = Math.max(maximum, active);
      response.on('close', () => {
        active--;
        if (!response.writableEnded) aborted++;
      });
      try {
        const body = JSON.parse(Buffer.concat(chunks).toString()) as {
          id: number;
          method: string;
          params?: unknown[];
        };
        const params = body.params ?? [];
        calls.push({ method: body.method, params });
        if (stall(body.method, params, calls.length - 1)) {
          held++;
          if (partialBody) {
            response.writeHead(200, { 'Content-Type': 'application/json' });
            response.write('{"jsonrpc":"2.0",');
          }
          return;
        }
        response.setHeader('Content-Type', 'application/json');
        response.end(
          JSON.stringify({
            jsonrpc: '2.0',
            id: body.id,
            result: respond(body.method, params),
          }),
        );
      } catch (error) {
        serverErrors.push(
          error instanceof Error ? error : new Error(String(error)),
        );
        response.writeHead(500);
        response.end();
      }
    });
  });
  await new Promise<void>((done) => server.listen(0, '127.0.0.1', done));
  const client = createPublicClient({
    transport: http(
      `http://127.0.0.1:${(server.address() as AddressInfo).port}`,
      { timeout: 60000, retryCount: 0 },
    ),
  });
  const provider = new ChainTradeProvider(client, undefined, deployment);
  return {
    provider,
    calls,
    get active() {
      return active;
    },
    get maximum() {
      return maximum;
    },
    get aborted() {
      return aborted;
    },
    get held() {
      return held;
    },
    setStall(predicate: typeof stall, stream = false) {
      stall = predicate;
      partialBody = stream;
      calls.length = 0;
    },
    add(index: number, action = 'buy'): PendingAttemptRecord {
      const txHash = `0x${index.toString(16).padStart(64, '0')}` as Hash;
      const input =
        action === 'approve'
          ? encodeFunctionData({
              abi: erc20Abi,
              functionName: 'approve',
              args: [router, 10n],
            })
          : encodeFunctionData({
              abi: tradeRouterAbi,
              functionName: 'buy',
              args: [pool, 10n, 19n, 9999999999n],
            });
      const record: PendingAttemptRecord = {
        version: 1,
        chainId: 125,
        from: account,
        intentId: `receipt-${index}`,
        txHash,
        nonce: index,
        action,
        actionFingerprint: transactionFingerprint(
          action === 'approve' ? token : router,
          input,
        ),
        submittedAt: Date.now(),
      };
      records.set(txHash, record);
      return record;
    },
    async close() {
      server.closeAllConnections();
      await new Promise<void>((done, reject) =>
        server.close((error) => (error ? reject(error) : done())),
      );
      expect(serverErrors).toEqual([]);
    },
  };
}

async function fourSlowReceiptReadsTerminateBeforeSlotsAreReused() {
  const ledger = await receiptLedger();
  const observations: RecoveryObservation[] = [],
    updates: RecoveryUpdate[] = [];
  const scheduler = new ReceiptRecoveryScheduler(
    (attempt, signal) => ledger.provider.readReceipt(attempt, signal),
    (value) => observations.push(value),
  );
  const stalled = Array.from({ length: 4 }, (_, index) =>
    ledger.add(index + 1),
  );
  const later = ledger.add(5);
  ledger.setStall(
    (method, params) =>
      method === 'eth_getTransactionReceipt' &&
      stalled.some((record) => record.txHash === params[0]),
  );
  try {
    for (const record of [...stalled, later])
      scheduler.watch(record, (update) => updates.push(update));
    await until(() => ledger.held === 4);
    expect(
      observations.filter((value) => value.kind === 'read_start'),
    ).toHaveLength(4);
    expect(ledger.active).toBe(4);
    await until(() => updates.length === 5, RECEIPT_LOOKUP_DEADLINE_MS + 3000);
    scheduler.close();
    await until(() => ledger.active === 0);
    expect(ledger.aborted).toBe(4);
    expect(ledger.maximum).toBeLessThanOrEqual(4);
    for (const record of stalled) {
      const result = updates.filter(
        (value) => value.attempt.txHash === record.txHash,
      );
      expect(result).toHaveLength(1);
      expect(result[0].receipt).toBeUndefined();
      expect(result[0].error).toBeInstanceOf(Error);
      expect(result[0].state).toBe('checking');
    }
    expect(
      updates.find((value) => value.attempt.txHash === later.txHash),
    ).toMatchObject({
      state: 'confirmed_success',
      receipt: { transactionHash: later.txHash },
    });
    const starts = observations.filter((value) => value.kind === 'read_start');
    const ends = observations.filter((value) => value.kind === 'read_end');
    expect(starts).toHaveLength(5);
    expect(ends).toHaveLength(5);
    expect(ends.at(-1)?.activeReaders).toBe(0);
    const laterStart = observations.findIndex(
      (value) => value.kind === 'read_start' && value.txHash === later.txHash,
    );
    expect(
      observations
        .slice(0, laterStart)
        .some((value) => value.kind === 'read_end'),
    ).toBe(true);
  } finally {
    scheduler.close();
    await ledger.close();
  }
}
test(
  fourSlowReceiptReadsTerminateBeforeSlotsAreReused.name,
  fourSlowReceiptReadsTerminateBeforeSlotsAreReused,
  20000,
);

async function closedRecoveryReopensPersistedAttemptsWithoutLeakingReaders() {
  const ledger = await receiptLedger();
  const values = new Map<string, string>();
  const storage: PendingStorage = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => {
      values.set(key, value);
    },
    removeItem: (key) => {
      values.delete(key);
    },
  };
  const store = new PendingAttemptStore(storage, account);
  const records = Array.from({ length: 4 }, (_, index) =>
    ledger.add(index + 10),
  );
  for (const record of records) store.save(record);
  const notifications: RecoveryUpdate[] = [],
    observations: RecoveryObservation[] = [];
  const first = new ReceiptRecoveryScheduler(
    (attempt, signal) => ledger.provider.readReceipt(attempt, signal),
    (value) => observations.push(value),
  );
  let reopened: ReceiptRecoveryScheduler | undefined;
  ledger.setStall((method) => method === 'eth_getTransactionReceipt');
  try {
    for (const record of store.load().records)
      first.watch(record, (update) => notifications.push(update));
    await until(() => ledger.held === 4);
    first.close();
    const recovered = new PendingAttemptStore(storage, account);
    expect(recovered.load()).toEqual({
      records: store.load().records,
      corrupt: false,
    });
    ledger.setStall(() => false);
    const provider = new ChainTradeProvider(
      ledger.provider.publicClient,
      undefined,
      deployment,
    );
    reopened = new ReceiptRecoveryScheduler(
      (attempt, signal) => provider.readReceipt(attempt, signal),
      (value) => observations.push(value),
    );
    const completed: RecoveryUpdate[] = [];
    for (const record of recovered.load().records)
      reopened.watch(record, (update) => {
        completed.push(update);
        if (update.state === 'confirmed_success')
          recovered.remove(update.attempt.txHash);
      });
    await until(() => completed.length === 4 && ledger.active === 0);
    expect(ledger.aborted).toBe(4);
    expect(ledger.maximum).toBeLessThanOrEqual(4);
    expect(notifications).toEqual([]);
    expect(
      completed.every((update) => update.state === 'confirmed_success'),
    ).toBe(true);
    expect(new Set(completed.map((update) => update.attempt.txHash)).size).toBe(
      4,
    );
    expect(recovered.load()).toEqual({ records: [], corrupt: false });
    expect(provider.fills.size).toBe(4);
    expect(
      observations.filter((value) => value.kind === 'read_start'),
    ).toHaveLength(8);
    expect(
      observations.filter((value) => value.kind === 'read_end'),
    ).toHaveLength(8);
    expect(
      observations.filter((value) => value.kind === 'read_end').at(-1)
        ?.activeReaders,
    ).toBe(0);
  } finally {
    first.close();
    reopened?.close();
    await ledger.close();
  }
}
test(
  closedRecoveryReopensPersistedAttemptsWithoutLeakingReaders.name,
  closedRecoveryReopensPersistedAttemptsWithoutLeakingReaders,
);

async function everyReceiptAndReplacementProvenanceReadCancelsItsTransport() {
  const ledger = await receiptLedger();
  const record = ledger.add(20);
  try {
    expect(await ledger.provider.readReceipt(record)).toMatchObject({
      transactionHash: record.txHash,
      status: 'success',
      blockNumber: 2n,
    });
    expect(ledger.provider.fills.get(record.txHash)).toMatchObject({
      inputRaw: 10n,
      outputRaw: 20n,
      feeRaw: 1n,
      tokenDecimals: 18,
    });
    const path = [...ledger.calls];
    expect(path.map((call) => call.method)).toEqual([
      'eth_chainId',
      'eth_getTransactionReceipt',
      'eth_getBlockByNumber',
      'eth_getTransactionByHash',
      'eth_chainId',
      'eth_getCode',
      'eth_getStorageAt',
      'eth_getCode',
      'eth_getCode',
      'eth_getCode',
      'eth_getCode',
      'eth_call',
      'eth_call',
      'eth_call',
    ]);
    for (let index = 0; index < path.length; index++) {
      const held = ledger.held,
        aborted = ledger.aborted;
      ledger.provider.fills.clear();
      ledger.setStall(
        (_method, _params, ordinal) => ordinal === index,
        index % 2 === 1,
      );
      const controller = new AbortController();
      const result = ledger.provider.readReceipt(record, controller.signal);
      const rejection = expect(result).rejects.toThrow();
      await until(() => ledger.held === held + 1);
      expect(ledger.calls.at(-1)).toEqual(path[index]);
      controller.abort();
      await rejection;
      await until(() => ledger.active === 0);
      expect(ledger.aborted).toBe(aborted + 1);
      expect(ledger.calls).toHaveLength(index + 1);
      expect(ledger.provider.fills.size).toBe(0);
    }
    ledger.setStall(() => false);
    expect(
      await ledger.provider.readReceipt(ledger.add(21, 'approve')),
    ).toMatchObject({ status: 'success' });
    const replacement = ledger.add(22);
    const original = { ...record, nonce: replacement.nonce };
    for (let index = 0; index < 4; index++) {
      const held = ledger.held,
        aborted = ledger.aborted;
      ledger.setStall((_method, _params, ordinal) => ordinal === index, true);
      const controller = new AbortController();
      const result = ledger.provider.replacement(
        original,
        replacement.txHash as Hash,
        controller.signal,
      );
      const rejection = expect(result).rejects.toThrow();
      await until(() => ledger.held === held + 1);
      controller.abort();
      await rejection;
      await until(() => ledger.active === 0);
      expect(ledger.aborted).toBe(aborted + 1);
      expect(ledger.calls).toHaveLength(index + 1);
    }
    ledger.setStall(() => false);
    expect(
      await ledger.provider.replacement(original, replacement.txHash as Hash),
    ).toMatchObject({
      hash: replacement.txHash,
      changed: false,
      cancelled: false,
    });
    expect(await ledger.provider.readReceipt(record)).toMatchObject({
      status: 'success',
    });
  } finally {
    await ledger.close();
  }
}
test(
  everyReceiptAndReplacementProvenanceReadCancelsItsTransport.name,
  everyReceiptAndReplacementProvenanceReadCancelsItsTransport,
  15000,
);

function configuredNativeRunnerUsesTheCurrentCheckoutAndChainFixture() {
  const root = resolve(directory, '../../../../../../../');
  const web = resolve(root, native.web);
  expect(realpathSync(web)).toBe(
    realpathSync(resolve(directory, '../../../../')),
  );
  const entry = resolve(web, native.entry);
  expect(statSync(entry).isFile()).toBe(true);
  const digest = createHash('sha256').update(readFileSync(entry)).digest('hex');
  expect(digest).toMatch(/^[0-9a-f]{64}$/);
  // Parse the actual shared production command with the installed runner, in process.
  const parsed = parseCLI([
    'vitest',
    ...native.arguments,
    '--outputFile=/private/receipt/vitest.json',
  ]);
  expect(parsed.options.run).toBe(true);
  expect(parsed.options).toHaveProperty('reporter', ['tap', 'json']);
  expect(parsed.options.outputFile).toBe('/private/receipt/vitest.json');
  expect(parsed.options.config).toBe(
    'src/widgets/trading/transaction-controls/vitest.phase2.config.ts',
  );
  if (typeof parsed.options.config !== 'string')
    throw new Error('Native runner config path required');
  expect(realpathSync(resolve(web, parsed.options.config))).toBe(
    realpathSync(resolve(directory, 'vitest.phase2.config.ts')),
  );
  expect(phase2.test?.include).toContain(
    'src/widgets/trading/transaction-controls/receipt.chain.test.ts',
  );
  expect(phase2.test?.include).toContain(
    'src/widgets/trading/transaction-controls/trade-journey.integration.test.tsx',
  );
  expect(
    statSync(resolve(directory, 'fixtures/ZeroResetToken.sol')).isFile(),
  ).toBe(true);
  const source = readFileSync(resolve(directory, 'run-phase2.py'), 'utf8');
  expect(source).toContain(
    "native=json.loads((pathlib.Path(__file__).parent/'native-runner.json').read_text())",
  );
  expect(source).toContain("web=root/native['web']");
  expect(source).toContain("str(web/native['entry']),*native['arguments']");
  expect(source).toContain('cwd=web');
  expect(source).toContain(
    '[(fixture,args.fixture_sha256),(compiler,args.compiler_sha256)]',
  );
  expect(source).toContain(
    "'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest()",
  );
  expect(source).toContain("'L26_FIXTURE_SHA256':args.fixture_sha256");
  expect(source).toContain("'L26_ZERO_RESET_ARTIFACT':str(artifact)");
}
test(
  configuredNativeRunnerUsesTheCurrentCheckoutAndChainFixture.name,
  configuredNativeRunnerUsesTheCurrentCheckoutAndChainFixture,
);

function measurementHashesAndExecutesTheSameConfiguredNativeRunner() {
  configuredNativeRunnerUsesTheCurrentCheckoutAndChainFixture();
  const source = readFileSync(resolve(directory, 'run-phase4.py'), 'utf8');
  expect(source).toContain(
    "native = json.loads((Path(__file__).parent / 'native-runner.json').read_text())",
  );
  expect(source).toContain("web = root / native['web']");
  expect(source).toContain("runner = web / native['entry']");
  expect(source).toContain(
    'configure(args.config, args.config_sha256, root, runner)',
  );
  expect(source).toContain("sha(runner) == c['vitest_entry_sha256']");
  expect(source).toContain(
    "command = [c['node'], str(runner), *native['arguments']",
  );
  expect(source).toContain('subprocess.run(command, cwd=web');
  for (const binding of [
    'configuration_sha256',
    'spec_sha256',
    'compiler_sha256',
    'node_sha256',
    'playwright_entry_sha256',
    'browser_executable_sha256',
    'fixture_sha256',
    'browser_bundle',
    'source_sha256',
  ])
    expect(source).toContain(binding);
  for (const artifact of [
    'measurement.json',
    'rpc.ndjson',
    'operations.ndjson',
    'recovery.ndjson',
    'journey.ndjson',
    'browser.ndjson',
    'browser-trace.zip',
    'trade-desktop.png',
    'trade-mobile.png',
    'browser-assets.json',
    'analytics-binding.json',
    'minimum-boundary.json',
    'optical-boundary.json',
    'lane-peak.json',
    'lane-surge.json',
    'recovery-summary.json',
  ])
    expect(source).toContain(`'${artifact}'`);
  for (const boundary of [
    'shared_capacity_qualified',
    'shared_restore_qualified',
    'production_qualified',
  ])
    expect(source).toContain(`'${boundary}': False`);
  expect(source).toContain("'shared_acceptance_gate': '9.1'");
}
test(
  measurementHashesAndExecutesTheSameConfiguredNativeRunner.name,
  measurementHashesAndExecutesTheSameConfiguredNativeRunner,
);

async function standaloneProvenanceDeadlineTerminatesAStalledResponseBody() {
  const ledger = await receiptLedger();
  const record = ledger.add(30);
  ledger.setStall((method) => method === 'eth_getStorageAt', true);
  const started = Date.now();
  try {
    await expect(ledger.provider.readReceipt(record)).rejects.toThrow(
      'Receipt lookup timed out; outcome remains unknown',
    );
    expect(Date.now() - started).toBeLessThan(
      RECEIPT_LOOKUP_DEADLINE_MS + 3000,
    );
    await until(() => ledger.active === 0);
    expect(ledger.aborted).toBe(1);
    expect(ledger.provider.fills.size).toBe(0);
    ledger.setStall(() => false);
    expect(await ledger.provider.readReceipt(record)).toMatchObject({
      status: 'success',
    });
  } finally {
    await ledger.close();
  }
}
test(
  standaloneProvenanceDeadlineTerminatesAStalledResponseBody.name,
  standaloneProvenanceDeadlineTerminatesAStalledResponseBody,
  15000,
);

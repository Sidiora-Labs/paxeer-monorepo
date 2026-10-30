import { readFileSync, readdirSync } from 'node:fs';
import { createServer } from 'node:http';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it, test, vi } from 'vitest';
import { toFunctionSelector, toFunctionSignature, type Abi } from 'viem';
import {
  EIP712_ABI,
  PRECOMPILE_ADDRESSES,
  PrecompileNotServedError,
  SCHEDULER_ABI,
  STREAMS_ABI,
  TEE_ATTESTOR_ABI,
  TEE_FAMILIES,
  UNSERVED_PRECOMPILES,
  precompileAddressBook,
  scheduler,
  streams,
  unservedPrecompileAt,
} from '../src/precompiles.js';
import { closePool, getPool } from '../src/db/pool.js';
import { executeAgentTransaction, type AgentCtx } from '../src/routes/agentExec.js';

function readRepositoryFile(path: string): string {
  return readFileSync(new URL(`../../../../${path}`, import.meta.url), 'utf8');
}

const addressTable = readRepositoryFile('spec/layerx-merge/contracts/address-table.md');

function tableRows(start: string, end: string): string[][] {
  const startOffset = addressTable.indexOf(start);
  const endOffset = addressTable.indexOf(end, startOffset + start.length);
  if (startOffset < 0 || endOffset < 0) {
    throw new Error(`Missing address-table section: ${start}`);
  }
  return addressTable
    .slice(startOffset + start.length, endOffset)
    .split('\n')
    .filter((line) => line.startsWith('| 0x'))
    .map((line) =>
      line
        .split('|')
        .slice(1, -1)
        .map((cell) => cell.trim()),
    );
}

const addressRows = tableRows('## (a) Address table', '### (a2) Method selectors');
const selectorRows = tableRows('### (a2) Method selectors', '### 0x101A resolution');
const expectedRoutes = {
  orob: '0x0000000000000000000000000000000000000901',
  clearing: '0x0000000000000000000000000000000000000902',
  oracle: '0x0000000000000000000000000000000000000903',
  pofq: '0x0000000000000000000000000000000000000904',
  scheduler: '0x0000000000000000000000000000000000000905',
  streams: '0x0000000000000000000000000000000000000906',
  teeAttestor: '0x0000000000000000000000000000000000000907',
  eip712: '0x0000000000000000000000000000000000000908',
  staking: '0x0000000000000000000000000000000000001005',
  bech32: '0x0000000000000000000000000000000000001004',
  p256: '0x0000000000000000000000000000000000001011',
} as const;
const routeKeys = Object.keys(expectedRoutes) as (keyof typeof expectedRoutes)[];
const registrationSources = [
  { key: 'staking', package: 'staking', symbol: 'StakingAddress', file: 'staking.go' },
  { key: 'bech32', package: 'addr', symbol: 'AddrAddress', file: 'addr.go' },
  { key: 'p256', package: 'p256', symbol: 'P256VerifyAddress', file: 'p256.go' },
] as const;
const setup = readRepositoryFile('precompiles/setup.go')
  .split('func GetCustomPrecompiles(')[1]
  ?.split('\nfunc InitializePrecompiles(')[0];
const abiRoutes = [
  { key: 'scheduler', abi: SCHEDULER_ABI, count: 5 },
  { key: 'streams', abi: STREAMS_ABI, count: 6 },
  { key: 'eip712', abi: EIP712_ABI, count: 3 },
  { key: 'teeAttestor', abi: TEE_ATTESTOR_ABI, count: 4 },
] as const;

function registeredAddress(source: (typeof registrationSources)[number]): string {
  const file = `precompiles/${source.package}/${source.file}`;
  const match = readRepositoryFile(file).match(
    new RegExp(`\\b${source.symbol}\\s*=\\s*"(0x[0-9a-fA-F]{40})"`),
  );
  if (!match?.[1]) throw new Error(`Missing ${source.key} address constant in ${file}`);
  return match[1].toLowerCase();
}

describe('route map', () => {
  it('retains exactly the eleven distinct route keys', () => {
    expect(Object.keys(PRECOMPILE_ADDRESSES).sort()).toEqual([...routeKeys].sort());
    expect(new Set(Object.values(PRECOMPILE_ADDRESSES)).size).toBe(11);
  });

  it.each(routeKeys)('%s resolves to its exact address in contract section (a)', (key) => {
    const address = PRECOMPILE_ADDRESSES[key];
    expect(address, key).toBe(expectedRoutes[key]);
    expect(
      addressRows.some((row) => row[0]?.toLowerCase() === address),
      key,
    ).toBe(true);
  });

  it.each(registrationSources)(
    '$key matches its Go constant and GetCustomPrecompiles entry',
    (source) => {
      expect(PRECOMPILE_ADDRESSES[source.key], source.key).toBe(registeredAddress(source));
      expect(setup, source.key).toMatch(
        new RegExp(`ecommon\\.HexToAddress\\(\\s*${source.package}\\.${source.symbol}\\s*\\)`),
      );
    },
  );

  it.each(routeKeys)('%s resolves to a Go registration or a new merge entry', (key) => {
    const source = registrationSources.find((entry) => entry.key === key);
    if (source) {
      expect(PRECOMPILE_ADDRESSES[key], key).toBe(registeredAddress(source));
      expect(setup, key).toMatch(
        new RegExp(`ecommon\\.HexToAddress\\(\\s*${source.package}\\.${source.symbol}\\s*\\)`),
      );
    } else {
      const entry = addressRows.find((row) => row[0]?.toLowerCase() === PRECOMPILE_ADDRESSES[key]);
      expect(entry?.[3], key).toBe('new');
      expect(entry?.[2], key).toContain('precompiles/layerxagent/');
      expect(entry?.[5], key).toBe('merge plan');
    }
  });

  it('bech32 targets the addr ABI getPaxAddr and getEvmAddr views', () => {
    const abi = JSON.parse(readRepositoryFile('precompiles/addr/abi.json')) as Abi;
    const methods = abi.filter(
      (item) => item.type === 'function' && ['getPaxAddr', 'getEvmAddr'].includes(item.name),
    );
    expect(PRECOMPILE_ADDRESSES.bech32).toBe(registeredAddress(registrationSources[1]));
    expect(
      methods
        .map((item) => {
          if (item.type !== 'function') throw new Error('Expected addr function');
          return [
            toFunctionSignature(item),
            item.stateMutability,
            item.outputs.map((output) => output.type),
          ];
        })
        .sort(),
    ).toEqual([
      ['getEvmAddr(string)', 'view', ['address']],
      ['getPaxAddr(address)', 'view', ['string']],
    ]);
  });
});

describe('agent ABI selectors', () => {
  it.each(abiRoutes)(
    '$key exactly matches every section (a2) signature and selector',
    ({ key, abi, count }) => {
      const address = `0x${PRECOMPILE_ADDRESSES[key].slice(-4)}`;
      const expected = selectorRows
        .filter((row) => row[0]?.toLowerCase() === address)
        .map((row) => [row[1], row[2]])
        .sort();
      const actual = abi
        .map((method) => [toFunctionSignature(method), toFunctionSelector(method)])
        .sort();
      expect(expected, key).toHaveLength(count);
      expect(actual, key).toEqual(expected);
      expect(new Set(actual.map(([, selector]) => selector)).size, key).toBe(count);
    },
  );
});

function goSources(): string[] {
  const root = fileURLToPath(new URL('../../../../', import.meta.url));
  function walk(directory: string): string[] {
    return readdirSync(join(root, directory), { withFileTypes: true }).flatMap((entry) => {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) {
        return ['.git', 'node_modules', 'target'].includes(entry.name) ? [] : walk(path);
      }
      return entry.isFile() && path.endsWith('.go') && !path.endsWith('_test.go') ? [path] : [];
    });
  }
  return walk('');
}

function goImports(source: string): string[] {
  const imports: string[] = [];
  const tokens = /\/\/|\/\*|["'`]|[A-Za-z_]\w*|[()]/g;
  let importing = false;
  let block = false;
  let match: RegExpExecArray | null;
  while ((match = tokens.exec(source)) !== null) {
    const token = match[0];
    if (token === '//' || token === '/*') {
      const end = source.indexOf(token === '//' ? '\n' : '*/', tokens.lastIndex);
      tokens.lastIndex = end < 0 ? source.length : end + (token === '//' ? 1 : 2);
    } else if (token === '"' || token === "'" || token === '`') {
      const start = tokens.lastIndex;
      let end = start;
      while (end < source.length && source[end] !== token) {
        end += source[end] === '\\' && token !== '`' ? 2 : 1;
      }
      if (importing && token !== "'") {
        imports.push(source.slice(start, end));
        if (!block) importing = false;
      }
      tokens.lastIndex = end + 1;
    } else if (token === 'import') {
      importing = true;
    } else if (importing && token === '(') {
      block = true;
    } else if (token === ')') {
      importing = false;
      block = false;
    }
  }
  return imports;
}

const unservedPrecompileTests = {
  async 'refuses a raw send to an unserved precompile address before any wallet read'() {
    const did = 'did:key:z6MkhMVmHTTrRzNBoFKtGAgJcmbopvJ8t83aF2j6cm7uqh5V';
    const ctx: AgentCtx = {
      did,
      ownerUserId: null,
      principal: {
        did,
        owner_user_id: null,
        label: 'Unserved precompile regression',
        key_fingerprint: 'unserved-precompile',
        public_key: '11'.repeat(32),
        wallet_id: null,
        is_frozen: false,
        created_at: new Date(0).toISOString(),
        last_seen_at: null,
      },
    };
    const pool = getPool();
    expect(pool.totalCount).toBe(0);
    try {
      const to = '0x0000000000000000000000000000000000000905';
      const result = await executeAgentTransaction({
        ctx,
        tx: { to, value: 1n },
        intent: { kind: 'transaction', to, value: 1n },
        broadcast: true,
        audit: { request_hash: 'unserved-precompile', ip: null, user_agent: null },
      });
      expect(result).toEqual({
        status: 503,
        body: { error: 'precompile_not_served', precompile: 'scheduler', address: to },
      });
      for (const key of UNSERVED_PRECOMPILES) {
        const address = PRECOMPILE_ADDRESSES[key];
        expect(unservedPrecompileAt(address.toUpperCase())).toBe(key);
        for (const broadcast of [false, true]) {
          await expect(
            executeAgentTransaction({
              ctx,
              tx: { to: address, value: 1n },
              intent: { kind: 'transaction', to: address, value: 1n },
              broadcast,
              audit: { request_hash: 'unserved-precompile', ip: null, user_agent: null },
            }),
          ).resolves.toEqual({
            status: 503,
            body: { error: 'precompile_not_served', precompile: key, address },
          });
        }
      }
      expect(pool.totalCount).toBe(0);
      expect(pool.waitingCount).toBe(0);
      expect(unservedPrecompileAt(undefined)).toBeNull();
      expect(unservedPrecompileAt('0x000000000000000000000000000000000000dEaD')).toBeNull();
    } finally {
      await closePool();
    }
  },
  'address book marks each unserved precompile as not served'() {
    const book = precompileAddressBook();
    expect(book.addresses).toEqual(PRECOMPILE_ADDRESSES);
    expect(book.tee_families).toEqual(TEE_FAMILIES);
    expect(Object.keys(book.served).sort()).toEqual(Object.keys(PRECOMPILE_ADDRESSES).sort());
    for (const key of Object.keys(PRECOMPILE_ADDRESSES) as (keyof typeof PRECOMPILE_ADDRESSES)[]) {
      expect(book.served[key], key).toBe(!UNSERVED_PRECOMPILES.has(key));
      expect(unservedPrecompileAt(PRECOMPILE_ADDRESSES[key]), key).toBe(
        UNSERVED_PRECOMPILES.has(key) ? key : null,
      );
    }
  },
  'lists every 0x09xx route while no node package links a layerxagent precompile'() {
    const routePackages = {
      orob: 'orob',
      clearing: 'clearing',
      oracle: null,
      pofq: 'pofq',
      scheduler: 'scheduler',
      streams: null,
      teeAttestor: 'teeattestor',
      eip712: 'eip712',
    } as const;
    const imports = goSources()
      .filter((file) => !file.startsWith('precompiles/layerxagent/'))
      .flatMap((file) => goImports(readRepositoryFile(file)));
    for (const key of Object.keys(routePackages) as (keyof typeof routePackages)[]) {
      const pkg = routePackages[key];
      const linked =
        pkg !== null && imports.some((path) => path.endsWith(`/precompiles/layerxagent/${pkg}`));
      expect(UNSERVED_PRECOMPILES.has(key), key).toBe(!linked);
    }
  },
};

describe('unserved agent precompiles', () => {
  test(
    unservedPrecompileTests['refuses a raw send to an unserved precompile address before any wallet read'],
    unservedPrecompileTests['refuses a raw send to an unserved precompile address before any wallet read'],
  );

  test(
    unservedPrecompileTests['address book marks each unserved precompile as not served'],
    unservedPrecompileTests['address book marks each unserved precompile as not served'],
  );

  it('lists the agent routes whose constructor package is not linked by the node', () => {
    const agentKeys = routeKeys.filter(
      (key) => !registrationSources.some((source) => source.key === key),
    );
    const node = readRepositoryFile('node/precompiles.go');
    const unlinkedKeys = agentKeys.filter(
      (key) =>
        !node.includes(
          `"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/${key.toLowerCase()}"`,
        ),
    );
    expect([...UNSERVED_PRECOMPILES].sort()).toEqual(unlinkedKeys.sort());
  });

  it(
    unservedPrecompileTests['lists every 0x09xx route while no node package links a layerxagent precompile'],
    unservedPrecompileTests['lists every 0x09xx route while no node package links a layerxagent precompile'],
  );

  it('refuses to build a scheduler deposit or a stream to an unserved address', () => {
    expect(() =>
      scheduler.schedule({
        target: '0x000000000000000000000000000000000000dEaD',
        executeAtBlock: 1n,
        gasLimit: 1n,
        depositWei: 1n,
      }),
    ).toThrowError(PrecompileNotServedError);
    expect(() => scheduler.cancel(1n)).toThrowError(PrecompileNotServedError);
    expect(() =>
      streams.open({
        payee: '0x000000000000000000000000000000000000dEaD',
        token: '0x000000000000000000000000000000000000dEaD',
        ratePerSecond: 1n,
        cap: 1n,
      }),
    ).toThrowError(PrecompileNotServedError);
  });

  it('refuses reads of an unserved address before any RPC', async () => {
    await expect(
      scheduler.pending('0x000000000000000000000000000000000000dEaD'),
    ).rejects.toBeInstanceOf(PrecompileNotServedError);
    await expect(streams.accrued(1n)).rejects.toBeInstanceOf(PrecompileNotServedError);
    await expect(streams.getStream(1n)).rejects.toBeInstanceOf(PrecompileNotServedError);
  });
});

const chainCodeTests = {
  async 'probes the chain for code before serving a linked agent precompile'() {
    let code = '0x';
    const requests: { method: string; params: unknown[] }[] = [];
    const server = createServer(async (req, res) => {
      let body = '';
      for await (const chunk of req) body += chunk.toString();
      const request = JSON.parse(body) as { id: number; method: string; params: unknown[] };
      requests.push({ method: request.method, params: request.params });
      res.setHeader('content-type', 'application/json');
      res.end(
        JSON.stringify(
          request.method === 'eth_getCode'
            ? { jsonrpc: '2.0', id: request.id, result: code }
            : {
                jsonrpc: '2.0',
                id: request.id,
                error: { code: -32601, message: 'Method not found' },
              },
        ),
      );
    });
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const previousRpcUrl = process.env.HYPERPAXEER_RPC_URL;
    try {
      const address = server.address();
      if (!address || typeof address === 'string') throw new Error('Expected a TCP listener');
      process.env.HYPERPAXEER_RPC_URL = `http://127.0.0.1:${address.port}`;
      vi.resetModules();
      const { ensureServed, PrecompileNotServedError, eip712, teeAttestor } = await import(
        '../src/precompiles.js'
      );

      await expect(ensureServed('clearing')).rejects.toBeInstanceOf(PrecompileNotServedError);
      expect(requests).toEqual([
        { method: 'eth_getCode', params: [PRECOMPILE_ADDRESSES.clearing, 'latest'] },
      ]);
      const domainSeparator = `0x${'11'.repeat(32)}` as const;
      const structHash = `0x${'22'.repeat(32)}` as const;
      const reads = [
        { key: 'eip712', read: () => eip712.hashTypedData(domainSeparator, structHash) },
        {
          key: 'eip712',
          read: () =>
            eip712.domainSeparator({
              name: 'Agent',
              version: '1',
              chainId: 125n,
              verifyingContract: PRECOMPILE_ADDRESSES.clearing,
            }),
        },
        {
          key: 'eip712',
          read: () => eip712.recoverTypedSigner(domainSeparator, structHash, '0x'),
        },
        { key: 'teeAttestor', read: () => teeAttestor.verify(0, '0x') },
        {
          key: 'teeAttestor',
          read: () => teeAttestor.verifyAndExpect(0, '0x', domainSeparator),
        },
        { key: 'teeAttestor', read: () => teeAttestor.rootOf(0, 0n) },
        { key: 'teeAttestor', read: () => teeAttestor.rootCount(0) },
      ] as const;
      for (const { read } of reads) {
        await expect(read()).rejects.toBeInstanceOf(PrecompileNotServedError);
      }
      expect(requests.slice(1)).toEqual(
        reads.map(({ key }) => ({
          method: 'eth_getCode',
          params: [PRECOMPILE_ADDRESSES[key], 'latest'],
        })),
      );

      code = '0x60006000f3';
      await expect(ensureServed('clearing')).resolves.toBeUndefined();
      expect(requests).toHaveLength(reads.length + 2);
      expect(requests[reads.length + 1]).toEqual({
        method: 'eth_getCode',
        params: [PRECOMPILE_ADDRESSES.clearing, 'latest'],
      });
      code = '0x';
      await expect(ensureServed('clearing')).resolves.toBeUndefined();
      expect(requests).toHaveLength(reads.length + 2);
      await expect(ensureServed('orob')).rejects.toBeInstanceOf(PrecompileNotServedError);
      expect(requests.at(-1)).toEqual({
        method: 'eth_getCode',
        params: [PRECOMPILE_ADDRESSES.orob, 'latest'],
      });
    } finally {
      if (previousRpcUrl === undefined) delete process.env.HYPERPAXEER_RPC_URL;
      else process.env.HYPERPAXEER_RPC_URL = previousRpcUrl;
      vi.resetModules();
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((err) => (err ? reject(err) : resolve())),
      );
    }
  },
};

test(
  chainCodeTests['probes the chain for code before serving a linked agent precompile'],
  chainCodeTests['probes the chain for code before serving a linked agent precompile'],
);

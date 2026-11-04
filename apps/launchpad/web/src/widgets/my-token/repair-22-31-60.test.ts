import { createRequire } from 'node:module';
import { mkdtemp, writeFile, readFile, chmod, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, join } from 'node:path';
import { spawn, execFileSync, type ChildProcess } from 'node:child_process';
import { createServer } from 'node:net';
import { EventEmitter } from 'node:events';
import { test, expect } from 'vitest';
import * as viem from 'viem';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';

type Coin = {
  Pool: viem.Address;
  Token: viem.Address;
  Creator: viem.Address;
  Name: string;
  Symbol: string;
};
type Chain = {
  origin: string;
  provider: viem.EIP1193Provider & {
    on: EventEmitter['on'];
    removeListener: EventEmitter['removeListener'];
  };
  coins: Coin[];
  close(): Promise<void>;
};
const require = createRequire(import.meta.url);
const fixtureDirectory = resolve(__dirname, 'test-runtime');
const repository = resolve(__dirname, '../../../../../..');
const { JSDOM } = require('jsdom') as {
  JSDOM: new (
    html: string,
    options: { url: string },
  ) => { window: Window & typeof globalThis & { close(): void } };
};
const runtime = require('./test-runtime/chain.cjs') as {
  start(root: string, deployment: typeof CORE_DEPLOYMENT, codec: typeof viem): Promise<Chain>;
};

async function unusedPort(): Promise<number> {
  const server = createServer();
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Loopback listener unavailable');
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
  return address.port;
}

async function metadata_token_identity_creator_discovery_through_gateway() {
  const directory = await mkdtemp(join(tmpdir(), 'my-token-proof-'));
  const children: ChildProcess[] = [];
  const originalEnvironment = { ...process.env };
  const originalWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
  const originalDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
  let chain: Chain | undefined;
  let closeView: (() => Promise<void>) | undefined;
  let closeDom: (() => void) | undefined;
  let stopPostgres: (() => void) | undefined;
  let failure: unknown;
  function launch(command: string, args: string[], cwd?: string) {
    const child = spawn(command, args, {
      cwd,
      detached: true,
      env: { ...process.env, GOWORK: 'off' },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    children.push(child);
    let output = '';
    child.stdout!.on('data', (chunk) => {
      output += String(chunk);
    });
    child.stderr!.on('data', (chunk) => {
      output += String(chunk);
    });
    return { child, output: () => output };
  }
  async function ready(
    process: ReturnType<typeof launch>,
    predicate: (output: string) => boolean,
    timeoutMs: number,
  ) {
    const until = Date.now() + timeoutMs;
    while (Date.now() < until) {
      if (predicate(process.output())) return process.output();
      if (process.child.exitCode !== null)
        throw new Error(`Real dependency exited ${process.child.exitCode}: ${process.output()}`);
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    throw new Error(`Real dependency readiness timed out: ${process.output()}`);
  }
  async function waitFor(
    predicate: () => boolean,
    act: (callback: () => Promise<void>) => Promise<void>,
  ) {
    for (let attempt = 0; attempt < 250; attempt++) {
      if (predicate()) return;
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 20));
      });
    }
    throw new Error(`Rendered creator coin did not settle: ${document.body.textContent}`);
  }
  try {
    chain = await runtime.start(repository, CORE_DEPLOYMENT, viem);
    expect(new URL(chain.origin).hostname).toBe('127.0.0.1');
    const [alpha, beta, unknown] = chain.coins;
    expect(
      new Set(chain.coins.flatMap((coin) => [coin.Pool.toLowerCase(), coin.Token.toLowerCase()]))
        .size,
    ).toBe(6);
    const client = viem.createPublicClient({ transport: viem.http(chain.origin) });
    const creatorPools = await client.readContract({
      address: CORE_DEPLOYMENT.quoter,
      abi: viem.parseAbi(['function getPoolsByCreator(address) view returns (address[])']),
      functionName: 'getPoolsByCreator',
      args: [alpha.Creator],
    });
    expect(creatorPools.map((pool) => pool.toLowerCase())).toEqual(
      chain.coins.map((coin) => coin.Pool.toLowerCase()),
    );

    const postgresDirectory = execFileSync('pg_config', ['--bindir'], { encoding: 'utf8' }).trim();
    const postgresPort = await unusedPort(),
      redisPort = await unusedPort();
    await chmod(directory, 0o755);
    const data = join(directory, 'postgres');
    const asPostgres = process.getuid?.() === 0;
    const pg = (binary: string, args: string[]) =>
      asPostgres
        ? {
            command: 'runuser',
            args: ['-u', 'postgres', '--', join(postgresDirectory, binary), ...args],
          }
        : { command: join(postgresDirectory, binary), args };
    if (asPostgres) execFileSync('install', ['-d', '-o', 'postgres', '-m', '700', data]);
    const initialization = pg('initdb', ['-D', data, '-A', 'trust', '-U', 'proof', '--no-locale']);
    execFileSync(initialization.command, initialization.args, { stdio: 'pipe' });
    const postgresCommand = pg('postgres', [
      '-D',
      data,
      '-h',
      '127.0.0.1',
      '-p',
      String(postgresPort),
      '-k',
      data,
    ]);
    const postgres = launch(postgresCommand.command, postgresCommand.args);
    stopPostgres = () => {
      const shutdown = pg('pg_ctl', ['-D', data, '-m', 'fast', '-w', 'stop']);
      execFileSync(shutdown.command, shutdown.args, { stdio: 'pipe' });
    };
    const cache = launch('redis-server', [
      '--bind',
      '127.0.0.1',
      '--port',
      String(redisPort),
      '--save',
      '',
      '--appendonly',
      'no',
    ]);
    await Promise.all([
      ready(
        postgres,
        (output) => output.includes('database system is ready to accept connections'),
        15000,
      ),
      ready(cache, (output) => output.includes('Ready to accept connections'), 15000),
    ]);
    const metadataConfig = join(directory, 'metadata.json');
    await writeFile(
      metadataConfig,
      JSON.stringify({
        DatabaseURL: `postgres://proof@127.0.0.1:${postgresPort}/postgres?sslmode=disable`,
        RedisURL: `redis://127.0.0.1:${redisPort}/0`,
        Directory: directory,
        Coins: [alpha, beta],
      }),
      { mode: 0o600 },
    );
    const metadata = launch(
      'go',
      ['run', '-mod=readonly', '.', metadataConfig],
      join(fixtureDirectory, 'metadata'),
    );
    const metadataOutput = await ready(
      metadata,
      (output) => /^https:\/\/127\.0\.0\.1:\d+$/m.test(output),
      480000,
    );
    const metadataOrigin = metadataOutput.match(/^https:\/\/127\.0\.0\.1:\d+$/m)![0];
    const gateway = launch(
      'go',
      ['run', '-mod=readonly', '.', metadataOrigin, directory],
      join(fixtureDirectory, 'gateway'),
    );
    const gatewayOutput = await ready(
      gateway,
      (output) => /^http:\/\/127\.0\.0\.1:\d+$/m.test(output),
      480000,
    );
    const gatewayOrigin = gatewayOutput.match(/^http:\/\/127\.0\.0\.1:\d+$/m)![0];
    process.env.NEXT_PUBLIC_METADATA_API_URL = gatewayOrigin;
    process.env.NEXT_PUBLIC_DATA_API_URL = gatewayOrigin;
    process.env.NEXT_PUBLIC_GATEWAY_URL = gatewayOrigin;
    process.env.NEXT_PUBLIC_STATS_API_URL = gatewayOrigin;
    const knownResponse = await fetch(`${gatewayOrigin}/metadata/${alpha.Token}`);
    expect(knownResponse.status).toBe(200);
    expect(await knownResponse.json()).toMatchObject({
      token_address: alpha.Token.toLowerCase(),
      pool_address: alpha.Pool.toLowerCase(),
      name: alpha.Name,
      symbol: alpha.Symbol,
    });
    const unknownResponse = await fetch(`${gatewayOrigin}/metadata/${unknown.Token}`);
    expect(unknownResponse.status).toBe(200);
    expect(await unknownResponse.json()).toMatchObject({
      token_address: unknown.Token.toLowerCase(),
      pool_address: null,
      name: null,
      symbol: null,
    });
    const wrongIdentity = await fetch(`${gatewayOrigin}/metadata/${alpha.Pool}`);
    expect(await wrongIdentity.json()).toMatchObject({
      pool_address: null,
      name: null,
      symbol: null,
    });

    const dom = new JSDOM('<html><body><main id="root"></main></body></html>', {
      url: 'http://localhost:3050',
    });
    closeDom = () => dom.window.close();
    Object.defineProperty(globalThis, 'window', { value: dom.window, configurable: true });
    Object.defineProperty(globalThis, 'document', {
      value: dom.window.document,
      configurable: true,
    });
    Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
    const { createElement, act } = await import('react');
    const { createRoot } = await import('react-dom/client');
    const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
    const { createConfig, http, WagmiProvider } = await import('wagmi');
    const { injected } = await import('wagmi/connectors');
    const { reconnect, disconnect } = await import('wagmi/actions');
    const { default: MyTokenView, fetchCreatorPoolMetadata } = await import('./my-token-view');
    const chainDefinition = {
      id: 125,
      name: 'Owned creator-discovery EVM',
      nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
      rpcUrls: { default: { http: [chain.origin] } },
    } as const;
    const config = createConfig({
      storage: null,
      chains: [chainDefinition],
      connectors: [
        injected({ target: { id: 'creator-proof', name: 'Owned EVM', provider: chain.provider } }),
      ],
      multiInjectedProviderDiscovery: false,
      transports: { 125: http(chain.origin, { retryCount: 0 }) },
    });
    const query = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
    const container = document.getElementById('root')!;
    const root = createRoot(container);
    closeView = async () => {
      await act(async () => root.unmount());
      query.clear();
      await disconnect(config);
    };
    await act(async () => {
      await reconnect(config, { connectors: config.connectors });
    });
    expect(config.state.status).toBe('connected');
    expect(config.state.connections.get(config.state.current!)?.accounts[0].toLowerCase()).toBe(
      alpha.Creator.toLowerCase(),
    );
    await act(async () => {
      root.render(
        createElement(
          WagmiProvider,
          { config, reconnectOnMount: false },
          createElement(QueryClientProvider, { client: query }, createElement(MyTokenView)),
        ),
      );
    });
    expect(container.textContent).not.toContain('Sign in');
    await waitFor(
      () =>
        container.textContent?.includes(alpha.Name) === true &&
        [...container.querySelectorAll('button')].some(
          (button) => button.textContent === beta.Symbol,
        ),
      act,
    );
    expect(container.textContent).toContain(alpha.Token);
    expect(container.textContent).toContain(alpha.Pool);
    expect([...container.querySelectorAll('button')].map((button) => button.textContent)).toContain(
      alpha.Symbol,
    );
    const select = async (label: string) => {
      const button = [...container.querySelectorAll('button')].find(
        (button) => button.textContent === label,
      );
      expect(button).toBeDefined();
      await act(async () => button!.click());
    };
    await select(beta.Symbol);
    await waitFor(() => container.textContent?.includes(beta.Name) === true, act);
    expect(container.textContent).toContain(beta.Token);
    expect(container.textContent).toContain(beta.Pool);
    const unknownButton = [...container.querySelectorAll('button')].find((button) =>
      button.textContent?.includes(unknown.Pool.slice(0, 6)),
    );
    expect(unknownButton).toBeDefined();
    await act(async () => unknownButton!.click());
    await waitFor(
      () => container.querySelector('[role="alert"]')?.textContent === 'Coin info unavailable',
      act,
    );
    expect(container.textContent).not.toContain(unknown.Name);
    expect(container.textContent).not.toContain(unknown.Symbol);
    await expect(fetchCreatorPoolMetadata(alpha.Pool, unknown.Token)).rejects.toThrow(
      'does not identify',
    );
    await expect(fetchCreatorPoolMetadata(alpha.Pool, beta.Token)).rejects.toThrow(
      'does not identify',
    );
    await expect(fetchCreatorPoolMetadata(alpha.Pool, viem.zeroAddress)).rejects.toThrow(
      'identity is unavailable',
    );
    const requests = (await readFile(join(directory, 'requests.log'), 'utf8'))
      .toLowerCase()
      .split('\n');
    for (const coin of [alpha, beta, unknown])
      expect(requests).toContain(`/metadata/${coin.Token.toLowerCase()}`);
    expect(requests.some((path) => path.startsWith(`/v2/pools/${alpha.Pool.toLowerCase()}`))).toBe(
      true,
    );
    expect(requests.some((path) => path.startsWith(`/v2/pools/${beta.Pool.toLowerCase()}`))).toBe(
      true,
    );
    expect(
      requests.some((path) =>
        chain!.coins.some((coin) => path.startsWith(`/v2/pools/${coin.Token.toLowerCase()}`)),
      ),
    ).toBe(false);
  } catch (error) {
    failure = error;
  } finally {
    const cleanupErrors: unknown[] = [];
    async function cleanup(action: () => void | Promise<void>) {
      try {
        await action();
      } catch (error) {
        cleanupErrors.push(error);
      }
    }
    if (closeView) await cleanup(closeView);
    if (closeDom) await cleanup(closeDom);
    if (originalWindow) Object.defineProperty(globalThis, 'window', originalWindow);
    else Reflect.deleteProperty(globalThis, 'window');
    if (originalDocument) Object.defineProperty(globalThis, 'document', originalDocument);
    else Reflect.deleteProperty(globalThis, 'document');
    for (const key of Object.keys(process.env))
      if (!(key in originalEnvironment)) delete process.env[key];
    Object.assign(process.env, originalEnvironment);
    if (stopPostgres) await cleanup(stopPostgres);
    for (const child of children.reverse()) {
      await cleanup(async () => {
        if (!child.pid || child.exitCode !== null || child.signalCode !== null) return;
        const exited = new Promise<void>((resolve, reject) => {
          const timeout = setTimeout(() => {
            try {
              process.kill(-child.pid!, 'SIGKILL');
            } catch (error) {
              if ((error as NodeJS.ErrnoException).code !== 'ESRCH') reject(error);
            }
            reject(new Error(`Dependency ${child.pid} did not stop after SIGTERM`));
          }, 10000);
          child.once('close', () => {
            clearTimeout(timeout);
            resolve();
          });
          child.once('error', (error) => {
            clearTimeout(timeout);
            reject(error);
          });
        });
        try {
          process.kill(-child.pid, 'SIGTERM');
        } catch (error) {
          if ((error as NodeJS.ErrnoException).code !== 'ESRCH') throw error;
        }
        await exited;
      });
    }
    if (chain) await cleanup(() => chain!.close());
    await cleanup(() => rm(directory, { recursive: true, force: true }));
    if (cleanupErrors.length)
      failure = new AggregateError(
        failure ? [failure, ...cleanupErrors] : cleanupErrors,
        'Creator discovery proof or dependency shutdown failed',
      );
  }
  if (failure) throw failure;
}

test(
  metadata_token_identity_creator_discovery_through_gateway,
  metadata_token_identity_creator_discovery_through_gateway,
  900000,
);

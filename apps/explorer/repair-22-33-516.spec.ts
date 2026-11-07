// @vitest-environment jsdom
import { once } from 'node:events';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createServer, get } from 'node:http';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { env } from 'node:process';
import * as promClient from 'prom-client';
import * as v from 'valibot';
import { parse } from 'yaml';

import type { EssentialDappsChainConfig } from 'types/client/marketplace';

import { afterEach, beforeEach, expect, test, vi } from 'vitest';

import rewrites from './nextjs/rewrites';

const packageRoot = __dirname;
const browserWindow = window;
const windowDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'window');
const originalDirectory = process.cwd();
const fixtureEnvs = {
  NEXT_RUNTIME: 'nodejs',
  NEXT_PUBLIC_MARKETPLACE_ENABLED: 'true',
  NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG: JSON.stringify({ revoke: { chains: [ '125' ] } }),
  NEXT_PUBLIC_ACCOUNT_AUTH_PROVIDER: 'dynamic',
  NEXT_PUBLIC_ACCOUNT_DYNAMIC_ENVIRONMENT_ID: 'explorer-chain-parity-fixture',
  NEXT_PUBLIC_MULTICHAIN_ENABLED: 'false',
  NEXT_PUBLIC_NETWORK_ID: '1',
  NEXT_PUBLIC_NETWORK_RPC_URL: 'http://127.0.0.1:38545',
  PROMETHEUS_METRICS_ENABLED: 'true',
};
const originalEnvs = new Map(Object.keys(fixtureEnvs).map((key) => [ key, env[key] ]));
const originalBrowserEnvs = browserWindow.__envs;
const originalBrowserChains = browserWindow.__essentialDappsChains;
let fixtureDirectory: string;
let fixture: { chains: Array<EssentialDappsChainConfig> };

function setServerRuntime() {
  Object.defineProperty(globalThis, 'window', { configurable: true, value: undefined });
}

function setBrowserRuntime() {
  Object.defineProperty(globalThis, 'window', windowDescriptor!);
}

beforeEach(async() => {
  Object.assign(env, fixtureEnvs);
  setServerRuntime();
  vi.resetModules();
  const { 'default': config } = await import('configs/app');
  fixture = {
    chains: [ {
      id: '125',
      name: 'Paxeer fixture',
      app_config: {
        app: { ...config.app, baseUrl: 'http://localhost:3000/explorer' },
        chain: { ...config.chain, id: '125', name: 'Paxeer fixture', rpcUrls: [ 'http://127.0.0.1:18545', 'http://127.0.0.1:28545' ] },
        apis: { general: config.apis.general },
      },
    } ],
  };
  fixtureDirectory = mkdtempSync(path.join(tmpdir(), 'explorer-22-33-516-'));
  const assets = path.join(fixtureDirectory, 'public/assets/essential-dapps');
  mkdirSync(assets, { recursive: true });
  writeFileSync(path.join(assets, 'chains.json'), JSON.stringify(fixture));
  process.chdir(fixtureDirectory);
  browserWindow.__envs = Object.fromEntries(Object.entries(env).filter((entry): entry is [string, string] => entry[1] !== undefined));
  browserWindow.__essentialDappsChains = fixture;
});

afterEach(() => {
  process.chdir(originalDirectory);
  rmSync(fixtureDirectory, { recursive: true, force: true });
  setBrowserRuntime();
  browserWindow.__envs = originalBrowserEnvs;
  browserWindow.__essentialDappsChains = originalBrowserChains;
  for (const [ key, value ] of originalEnvs) {
    if (value === undefined) {
      delete env[key];
    } else {
      env[key] = value;
    }
  }
  promClient.register.clear();
  vi.resetModules();
});

test('metricsEdgesReachFrontendExporter', async function metricsEdgesReachFrontendExporter() {
  process.chdir(packageRoot);
  const scrape = v.parse(v.object({
    scrape_configs: v.array(v.object({
      job_name: v.string(),
      metrics_path: v.optional(v.string()),
      scheme: v.optional(v.string()),
      static_configs: v.array(v.object({ targets: v.array(v.string()) })),
    })),
  }), parse(readFileSync(path.join(packageRoot, 'prometheus.yml'), 'utf8')));
  expect(scrape.scrape_configs.map((job) => job.job_name)).toEqual([ 'prometheus', 'frontend' ]);
  const frontend = scrape.scrape_configs.find((job) => job.job_name === 'frontend')!;
  expect(frontend.static_configs).toEqual([ { targets: [ 'host.docker.internal:3000' ] } ]);
  expect(frontend.scheme).toBe('http');
  expect(frontend.metrics_path).toBe('/explorer/node-api/metrics');

  const requirePackage = createRequire(path.join(packageRoot, 'package.json'));
  const nextConfig = requirePackage('./next.config.js');
  const routes = await rewrites();
  const apiRewrite = routes.find((route) => route.destination === '/api/:slug*')!;
  expect(frontend.metrics_path).toBe(nextConfig.basePath + apiRewrite.source.replace(':slug*', 'metrics'));
  const manifest = v.parse(v.object({ scripts: v.record(v.string(), v.string()) }),
    JSON.parse(readFileSync(path.join(packageRoot, 'package.json'), 'utf8')));
  expect(manifest.scripts['monitoring:prometheus:local']).toContain('--add-host=host.docker.internal:host-gateway');
  expect(manifest.scripts['monitoring:prometheus:local']).toContain('-v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml');

  const { 'default': metricsHandler } = await import('./pages/api/metrics');
  const { apiResolver } = await import('next/dist/server/api-utils/node/api-resolver');
  const counter = new promClient.Counter({ name: 'explorer_wiring_probe_total', help: 'Recorded local scrape probe' });
  counter.inc(7);
  const server = createServer((request, response) => {
    if (request.url !== frontend.metrics_path) {
      response.writeHead(404).end();
      return;
    }
    void apiResolver(request, response, {}, { 'default': metricsHandler }, {
      previewModeId: 'fixture', previewModeEncryptionKey: 'fixture', previewModeSigningKey: 'fixture', dev: false,
    }, false);
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const address = server.address();
    if (!address || typeof address === 'string') {
      throw new Error('Expected a TCP listener');
    }
    const result = await new Promise<{ status: number | undefined; contentType: string | undefined; body: string }>((resolve, reject) => {
      get(`http://127.0.0.1:${ address.port }${ frontend.metrics_path }`, (response) => {
        response.setEncoding('utf8');
        let body = '';
        response.on('data', (chunk) => {
          body += chunk;
        });
        response.on('end', () => resolve({ status: response.statusCode, contentType: response.headers['content-type'], body }));
        response.on('error', reject);
      }).on('error', reject);
    });
    expect(result.status).toBe(200);
    expect(result.contentType).toContain('text/plain');
    expect(result.body).toContain('explorer_wiring_probe_total 7');
    expect(result.body).toContain('frontend_nodejs_version_info');
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
});

test('essentialChainsStartupLoadsNodeCache', async function essentialChainsStartupLoadsNodeCache() {
  const { register } = await import('./instrumentation');
  await register();
  rmSync(path.join(fixtureDirectory, 'public/assets/essential-dapps/chains.json'));
  const { getValue } = await import('configs/essential-dapps-chains/config.nodejs');
  expect(getValue()).toEqual(fixture);
});

test('essentialChainsStartupRejectsMissingConfig', async function essentialChainsStartupRejectsMissingConfig() {
  rmSync(path.join(fixtureDirectory, 'public/assets/essential-dapps/chains.json'));
  const { register } = await import('./instrumentation');
  await expect(register()).rejects.toThrow('Cannot load required essential dapp chains configuration');
});

test('essentialChainsMatchServerAndBrowser', async function essentialChainsMatchServerAndBrowser() {
  const { register } = await import('./instrumentation');
  await register();
  const { 'default': serverAccessor } = await import('configs/essential-dapps-chains');
  const serverChains = serverAccessor();
  rmSync(path.join(fixtureDirectory, 'public/assets/essential-dapps/chains.json'));
  expect(serverAccessor()).toEqual(fixture);
  setBrowserRuntime();
  vi.resetModules();
  const { 'default': browserAccessor } = await import('configs/essential-dapps-chains');
  expect(browserAccessor()).toEqual(fixture);
  expect(serverChains).toEqual(browserAccessor());
});

test('essentialChainsReachWagmiTransports', async function essentialChainsReachWagmiTransports() {
  const { register } = await import('./instrumentation');
  await register();
  const { 'default': serverWagmi } = await import('lib/web3/wagmiConfig');
  const serverIds = serverWagmi.config.chains.map((chain) => chain.id);
  expect(serverIds).toEqual([ 125, 1 ]);
  setBrowserRuntime();
  vi.resetModules();
  const { 'default': browserWagmi } = await import('lib/web3/wagmiConfig');
  const browserIds = browserWagmi.config.chains.map((chain) => chain.id);
  expect(browserIds).toContain(125);
  expect(browserIds).toEqual([ 125, 1 ]);
  const transports = v.object({ transports: v.array(v.object({ value: v.object({ url: v.string() }) })) });
  const browserTransport = v.parse(transports, browserWagmi.config.getClient({ chainId: 125 }).transport);
  const expectedUrls = [ ...fixture.chains[0].app_config!.chain.rpcUrls ];
  const generalApi = v.parse(v.object({ endpoint: v.string(), basePath: v.optional(v.string()) }), fixture.chains[0].app_config!.apis.general);
  expectedUrls.push(`${ generalApi.endpoint }${ generalApi.basePath ?? '' }/api/eth-rpc`);
  expect(browserTransport.transports.map((transport) => transport.value.url)).toEqual(expectedUrls);
  expect(serverIds).toEqual(browserIds);
  const serverTransport = v.parse(transports, serverWagmi.config.getClient({ chainId: 125 }).transport);
  expect(serverTransport.transports.map((transport) => transport.value.url)).toEqual(expectedUrls);
  const { 'default': appConfig } = await import('configs/app');
  const currentGeneralApi = v.parse(v.object({ endpoint: v.string(), basePath: v.optional(v.string()) }), appConfig.apis.general);
  const currentChainUrls = [
    ...appConfig.chain.rpcUrls,
    `${ currentGeneralApi.endpoint }${ currentGeneralApi.basePath ?? '' }/api/eth-rpc`,
  ];
  const serverCurrentTransport = v.parse(transports, serverWagmi.config.getClient({ chainId: 1 }).transport);
  const browserCurrentTransport = v.parse(transports, browserWagmi.config.getClient({ chainId: 1 }).transport);
  expect(serverCurrentTransport.transports.map((transport) => transport.value.url)).toEqual(currentChainUrls);
  expect(browserCurrentTransport.transports.map((transport) => transport.value.url)).toEqual(currentChainUrls);
}, 30_000);

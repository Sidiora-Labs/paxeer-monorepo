/**
 * Proves: runtime-authority.ts:parsePrivateFixtureAuthorityProfileV1 -> env NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE -> layout.tsx:RootLayout -> app-providers.tsx:AppProviders.
 * Also proves deployment.ts:CORE_DEPLOYMENT -> contracts.ts:erc20ApprovalRequest
 * and providers.ts:ChainTradeProvider, including productionTradeDeployment pins.
 * Repairs: RootLayout read the production cookie key for private sessions and
 * PrivateFixtureWeb3Provider discarded initialState. The profile was also captured
 * at module load, so a reopened request could receive stale configuration.
 * Production trade pins now derive every identity from CORE_DEPLOYMENT, including
 * the EventEmitter implementation, rather than duplicating its reviewed values.
 * ANVIL_BIN -> test-anvil.ts:resolveAnvilBinary uses the actual process environment.
 * No RPC or wallet is impersonated: this proves configuration, persistence and
 * calldata admission, without claiming that a transaction has executed on chain.
 */
import { execFileSync } from 'node:child_process';
import { closeSync, fsyncSync, mkdtempSync, openSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { Children, createElement, isValidElement, type ComponentProps, type ReactElement, type ReactNode } from 'react';
import { renderToString } from 'react-dom/server';
import { afterEach, beforeAll, expect, test } from 'vitest';
import { createPublicClient, decodeFunctionData, encodeErrorResult, http, parseAbi } from 'viem';
import { useConfig, type Config } from 'wagmi';
import { CORE_DEPLOYMENT, CORE_TRADE_EXECUTION_ROLES } from './core/network/deployment';
import { erc20ApprovalRequest, ROUTER_ADDRESS, QUOTER_ADDRESS, FEES_ROUTER_ADDRESS, OPTICAL_REGISTRY_ADDRESS, USDL_ADDRESS, PROTOCOL_CONFIG_ADDRESS } from './core/network/contracts';
import { parsePrivateFixtureAuthorityProfileV1, resolvePrivateFixtureAuthorityProfileV1, type PrivateFixtureAuthorityProfileV1 } from './core/runtime-authority';
import { resolveAnvilBinary } from './core/test-anvil';
import { ChainTradeProvider, OpticalRejection, opticalRejection, productionTradeDeployment, tradeRouterAbi } from './widgets/trading/transaction-controls/providers';
import type { TradeContext } from './widgets/trading/transaction-controls/domain';

let RootLayout: typeof import('./app/layout').default;
const { CookieJar, JSDOM } = createRequire(import.meta.url)('jsdom');
let AppProviders: typeof import('./ui/providers/app-providers').default;
let createPrivateFixtureWagmiConfig: typeof import('./ui/providers/app-providers').createPrivateFixtureWagmiConfig;
let requestLayout: (cookie: string) => ReturnType<typeof RootLayout>;
const originalProfile = process.env.NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE;
const originalAnvil = process.env.ANVIL_BIN;
const originalNodeEnv = process.env.NODE_ENV;

beforeAll(async () => {
  // Decision 2282: build dist-entry dependencies before importing their consumers.
  // sdk depends on sdk-v2; stats-sdk has no workspace dependencies.
  const root = join(dirname(fileURLToPath(import.meta.url)), '../../../..');
  for (const name of ['sdk-v2', 'sdk', 'stats-sdk']) {
    execFileSync('pnpm', ['--dir', join(root, 'sdk/launchpad/packages', name), 'run', 'build'], { stdio: 'pipe' });
  }
  createRequire(import.meta.url)('next/dist/server/node-environment-baseline');
  const { NextRequest } = await import('next/server');
  const { createRequestStoreForAPI } = await import('next/dist/server/async-storage/request-store');
  const { createWorkStore } = await import('next/dist/server/async-storage/work-store');
  const { workAsyncStorage } = await import('next/dist/server/app-render/work-async-storage.external');
  const { workUnitAsyncStorage } = await import('next/dist/server/app-render/work-unit-async-storage.external');
  ({ default: RootLayout } = await import('./app/layout'));
  ({ default: AppProviders, createPrivateFixtureWagmiConfig } = await import('./ui/providers/app-providers'));
  requestLayout = cookie => {
    const request = new NextRequest('https://localhost:3050/', { headers: { cookie } });
    const store = createRequestStoreForAPI(request, { pathname: '/' }, { tags: [], expirationsByCacheKind: new Map() }, undefined, undefined);
    const work = createWorkStore({ page: '/layout', buildId: 'launchpad-path-proof', previouslyRevalidatedTags: [], renderOpts: {
      supportsDynamicResponse: true, experimental: { isRoutePPREnabled: false, cacheComponents: false, authInterrupts: false },
      waitUntil: undefined, onAfterTaskError: undefined,
      onClose: callback => request.signal.addEventListener('abort', callback, { once: true }),
    } });
    return workAsyncStorage.run(work, () => workUnitAsyncStorage.run(store, () => RootLayout({ children: 'launchpad' })));
  };
}, 60_000);

afterEach(() => {
  for (const [key, value] of Object.entries({ NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE: originalProfile, ANVIL_BIN: originalAnvil, NODE_ENV: originalNodeEnv })) {
    if (value === undefined) delete process.env[key]; else process.env[key] = value;
  }
});

function reviewedProfile(): PrivateFixtureAuthorityProfileV1 {
  const fields = { EventEmitter: 'eventEmitter', ProtocolConfig: 'protocolConfig', Router: 'router', Quoter: 'quoter', SidioraFactory: 'factory', PoolRegistry: 'poolRegistry', FeeAccumulator: 'feeAccumulator', SidioraNFT: 'nft', FeesRouter: 'feesRouter', OpticalRegistry: 'opticalRegistry', PoolBeacon: 'poolBeacon', USDL: 'usdl' } as const;
  const roles = Object.fromEntries(Object.entries(fields).map(([role, field]) => {
    // USDL's runtime is attested at activation, not recorded in this manifest.
    // It is not consumed as an execution pin by this configuration-only proof.
    const identity = field === 'usdl' ? CORE_DEPLOYMENT.roleIdentity.router : CORE_DEPLOYMENT.roleIdentity[field];
    return [role, { ...identity, kind: identity.kind === 'surface' ? 'direct' : identity.kind, address: CORE_DEPLOYMENT[field] }];
  }));
  return parsePrivateFixtureAuthorityProfileV1({
    schemaVersion: 1, profile: 'PRIVATE_FIXTURE', chainId: CORE_DEPLOYMENT.chainId,
    applicationOrigin: 'https://localhost:3050', publicRPC: 'https://127.0.0.1:8545', explorerBase: 'https://localhost:4000',
    sourceManifestSha256: CORE_DEPLOYMENT.manifestSha256, networkSha256: CORE_DEPLOYMENT.manifestSha256,
    expectedBeaconOwner: CORE_DEPLOYMENT.router, roles,
    revision: { schemaVersion: 2, deploymentVersion: 'reviewed-configuration-proof', manifestSha256: CORE_DEPLOYMENT.manifestSha256,
      capabilitySha256: CORE_DEPLOYMENT.manifestSha256, contractSupportSha256: CORE_DEPLOYMENT.manifestSha256,
      serialInterfaceSha256: CORE_DEPLOYMENT.manifestSha256, revisionSha256: CORE_DEPLOYMENT.manifestSha256 },
    sourceArtifacts: { chainSha256: CORE_DEPLOYMENT.manifestSha256, compilerArtifactsSha256: CORE_DEPLOYMENT.manifestSha256, reviewedManifestSha256: CORE_DEPLOYMENT.manifestSha256 },
  });
}

function providerElement(node: ReactNode): ReactElement<ComponentProps<typeof AppProviders>> | undefined {
  for (const child of Children.toArray(node)) {
    if (!isValidElement<ComponentProps<typeof AppProviders>>(child)) continue;
    if (child.type === AppProviders) return child;
    const found = providerElement(child.props.children);
    if (found) return found;
  }
}

async function NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE() {
  const profile = reviewedProfile();
  process.env.NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE = JSON.stringify(profile);
  expect(parsePrivateFixtureAuthorityProfileV1(process.env.NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE)).toEqual(profile);
  expect(Object.isFrozen(profile.roles.Router)).toBe(true);
  const root = mkdtempSync(join(tmpdir(), 'launchpad-session-'));
  let browser = new JSDOM('', { url: profile.applicationOrigin });
  try {
    // The producer writes through the same config and cookieStorage as AppProviders.
    const config = createPrivateFixtureWagmiConfig(profile);
    const selected = config.connectors[0].uid;
    const originalWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
    const originalDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
    try {
      Object.defineProperty(globalThis, 'window', { configurable: true, value: browser.window });
      Object.defineProperty(globalThis, 'document', { configurable: true, value: browser.window.document });
      config.setState({ ...config.state, current: selected });
      const firstCookie = browser.window.document.cookie;
      expect(firstCookie).toContain(`${config.storage!.key}.store=`);
      config.setState({ ...config.state, current: selected });
      expect(browser.window.document.cookie).toBe(firstCookie);
    } finally {
      if (originalWindow) Object.defineProperty(globalThis, 'window', originalWindow); else Reflect.deleteProperty(globalThis, 'window');
      if (originalDocument) Object.defineProperty(globalThis, 'document', originalDocument); else Reflect.deleteProperty(globalThis, 'document');
    }
    const sessionFile = join(root, 'session.json');
    const fd = openSync(sessionFile, 'wx', 0o600);
    try { writeFileSync(fd, JSON.stringify(browser.cookieJar.serializeSync())); fsyncSync(fd); } finally { closeSync(fd); }
    browser.window.close();
    // Reopen the real cookie jar from disk in a new browser session.
    browser = new JSDOM('', { url: profile.applicationOrigin, cookieJar: CookieJar.deserializeSync(readFileSync(sessionFile, 'utf8')) });
    const cookie = browser.window.document.cookie;
    const first = providerElement(await requestLayout(cookie));
    expect(first).toBeDefined();
    expect(first!.props.privateFixtureProfileInput).toBe(JSON.stringify(profile));
    expect(first!.props.wagmiInitialState?.current).toBe(selected);
    expect(first!.props.wagmiInitialState?.chainId).toBe(125);
    const observed: Config[] = [];
    function ObservePrivateConfig() {
      const active = useConfig(); observed.push(active);
      return createElement('output', null, active.state.current);
    }
    const html = renderToString(createElement(AppProviders, { ...first!.props, children: createElement(ObservePrivateConfig) }));
    expect(html).toContain(selected);
    expect(observed).toHaveLength(1);
    expect(observed[0].state.current).toBe(selected);
    expect(observed[0].chains[0].rpcUrls.default.http).toEqual([profile.publicRPC]);
    expect(observed[0].storage!.key).toBe(config.storage!.key);
    const reopened = providerElement(await requestLayout(cookie));
    expect(reopened!.props.wagmiInitialState).toEqual(first!.props.wagmiInitialState);
    expect(parsePrivateFixtureAuthorityProfileV1(JSON.stringify(profile))).toEqual(profile);

    process.env.NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE = '{';
    const malformed = providerElement(await requestLayout(cookie));
    expect(malformed!.props.privateFixtureProfileInput).toBe('{');
    const refused = renderToString(createElement(AppProviders, { ...malformed!.props, children: createElement(ObservePrivateConfig) }));
    expect(refused).toContain('Something went wrong, reload the page');
    expect(observed).toHaveLength(1);
    expect(() => resolvePrivateFixtureAuthorityProfileV1({ privateFixtureProfile: profile, privateFixtureProfileInput: JSON.stringify(profile) })).toThrow('ambiguous');
    expect(Reflect.set(process.env, 'NODE_ENV', 'production')).toBe(true);
    expect(() => parsePrivateFixtureAuthorityProfileV1(JSON.stringify(profile))).toThrow('unavailable in production');
  } finally { browser.window.close(); rmSync(root, { recursive: true, force: true }); }
}

function deploymentFlowsIntoContractsAndTradeCalldata() {
  expect([ROUTER_ADDRESS, QUOTER_ADDRESS, FEES_ROUTER_ADDRESS, OPTICAL_REGISTRY_ADDRESS, USDL_ADDRESS, PROTOCOL_CONFIG_ADDRESS]).toEqual([
    CORE_DEPLOYMENT.router, CORE_DEPLOYMENT.quoter, CORE_DEPLOYMENT.feesRouter, CORE_DEPLOYMENT.opticalRegistry, CORE_DEPLOYMENT.usdl, CORE_DEPLOYMENT.protocolConfig,
  ]);
  const approval = erc20ApprovalRequest({ token: USDL_ADDRESS, spender: ROUTER_ADDRESS, amount: 1_000_000n });
  expect(approval).toMatchObject({ address: USDL_ADDRESS, chainId: 125, functionName: 'approve', args: [ROUTER_ADDRESS, 1_000_000n] });
  expect(() => erc20ApprovalRequest({ token: USDL_ADDRESS, spender: QUOTER_ADDRESS, amount: 1_000_000n })).toThrow('Approval spender is unavailable');
  expect(productionTradeDeployment.pins).toHaveLength(CORE_TRADE_EXECUTION_ROLES.length);
  CORE_TRADE_EXECUTION_ROLES.forEach((role, index) => {
    const identity = CORE_DEPLOYMENT.roleIdentity[role], pin = productionTradeDeployment.pins[index];
    expect(pin.address).toBe(CORE_DEPLOYMENT[role]);
    expect(pin.runtimeHash).toBe(identity.runtimeHash);
    if ('implementationAddress' in identity) {
      expect(pin.implementation).toBe(identity.implementationAddress);
      expect(pin.implementationHash).toBe(identity.implementationHash);
    }
  });
  const client = createPublicClient({ transport: http('http://127.0.0.1:8545', { retryCount: 0 }) });
  const provider = new ChainTradeProvider(client);
  expect(provider.deployment).toBe(productionTradeDeployment);
  const context: TradeContext = { chainId: 125, manifestRevision: CORE_DEPLOYMENT.manifestSha256, router: ROUTER_ADDRESS,
    pool: CORE_DEPLOYMENT.poolRegistry, token: USDL_ADDRESS, sender: CORE_DEPLOYMENT.router, recipient: CORE_DEPLOYMENT.router,
    inputRaw: 1_000_000n, isBuy: true, opticalRevision: 'none', opticalData: '0x' };
  const encoded = provider.swapData(context, 990_000n, 2_000_000_000n);
  expect(decodeFunctionData({ abi: tradeRouterAbi, data: encoded })).toMatchObject({ functionName: 'buy', args: [context.pool, context.inputRaw, 990_000n, 2_000_000_000n] });
  expect(provider.swapData(context, 990_000n, 2_000_000_000n)).toBe(encoded);
  expect(() => provider.swapData({ ...context, recipient: QUOTER_ADDRESS }, 990_000n, 2_000_000_000n)).toThrow('recipient_route_unavailable');
  expect(() => new ChainTradeProvider(client, undefined, { ...productionTradeDeployment, pins: [] })).toThrow('missing_deployment_code_pin');
}

function Error() {
  const encoded = encodeErrorResult({ abi: parseAbi(['error CooldownActive()']), errorName: 'CooldownActive' });
  const rejection = opticalRejection({ cause: { data: encoded } });
  expect(rejection).toBeInstanceOf(globalThis.Error);
  expect(rejection).toBeInstanceOf(OpticalRejection);
  expect(rejection).toMatchObject({ name: 'OpticalRejection', reason: 'CooldownActive', retryEligibility: 'after cooldown', message: 'CooldownActive; retry: after cooldown' });
  const unknown = new globalThis.Error('unrecognised execution error');
  expect(opticalRejection(unknown)).toBe(unknown);
  expect(opticalRejection({ data: '0xnothex' })).toEqual({ data: '0xnothex' });
}

function ANVIL_BIN() {
  // Resolution validates an executable file, not its brand; use the real Node
  // binary, never a fake shell script or a pretend RPC server.
  process.env.ANVIL_BIN = process.execPath;
  const resolved = resolveAnvilBinary();
  expect(resolved).toBe(process.execPath);
  expect(execFileSync(resolved, ['--version'], { encoding: 'utf8' }).trim()).toBe(process.version);
  expect(resolveAnvilBinary(process.env)).toBe(resolved);
  expect(() => resolveAnvilBinary({ ANVIL_BIN: join(tmpdir(), 'absent-launchpad-anvil'), PATH: '' })).toThrow('anvil binary unavailable');
}

test(NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE);
test(deploymentFlowsIntoContractsAndTradeCalldata);
test(Error);
test(ANVIL_BIN);

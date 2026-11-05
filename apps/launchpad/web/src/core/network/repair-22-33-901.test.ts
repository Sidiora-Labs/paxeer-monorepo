// @vitest-environment jsdom

import { createHash } from 'node:crypto';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { runInNewContext } from 'node:vm';
import ts from 'typescript';
import { expect, test } from 'vitest';
import { cookieStorage, cookieToInitialState, createStorage } from 'wagmi';
import * as deployment from './deployment';
import * as contracts from './contracts';
import { productionTradeDeployment } from '@/widgets/trading/transaction-controls/providers';
import { createPrivateFixtureWagmiStorage } from '@/ui/providers/app-providers';
import { createWagmiConfig } from '@/core/wagmi-config';

type Generator = {
  deriveDeployment(root: string): typeof deployment.CORE_DEPLOYMENT;
  renderHelper(data: typeof deployment.CORE_DEPLOYMENT): string;
  checkClient(root: string, data: typeof deployment.CORE_DEPLOYMENT, helper: string): { checked: boolean };
};

const root = resolve(__dirname, '../../../../../..');
const require = createRequire(import.meta.url);
const base = require(join(root, 'tools/launchpad/production/interfaces/contracts/client-deployment.cjs'));
const { createDeploymentGenerator } = require('./deployment-generation.cjs') as {
  createDeploymentGenerator(base: unknown): Generator;
};
const generator = createDeploymentGenerator(base);
const contractRoot = join(root, 'contracts/launchpad');
const clientRoot = join(root, 'apps/launchpad/web/src');
const manifestFile = join(contractRoot, 'deployments/paxeer-x-network-addresses.json');
const reviewed = JSON.parse(readFileSync(join(root, 'sdk/launchpad/shared/config/deployment/current.json'), 'utf8'));

function deploymentRegenerationPreservesFullHelper() {
  const data = generator.deriveDeployment(contractRoot);
  const rendered = generator.renderHelper(data);
  expect(readFileSync(join(__dirname, 'deployment.ts'), 'utf8')).toBe(rendered);
  expect(data).toEqual(deployment.CORE_DEPLOYMENT);
  expect(data.manifestSha256).toBe(createHash('sha256').update(readFileSync(manifestFile)).digest('hex'));
  expect(generator.checkClient(clientRoot, data, rendered).checked).toBe(true);

  const output: Record<string, unknown> = {};
  runInNewContext(ts.transpileModule(rendered, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText, { exports: output });
  expect(Object.keys(output).sort()).toEqual(Object.keys(deployment).sort());
  expect(output.CORE_DEPLOYMENT).toEqual(deployment.CORE_DEPLOYMENT);
  expect(output.CORE_TRADE_EXECUTION_ROLES).toEqual(deployment.CORE_TRADE_EXECUTION_ROLES);
  expect(output.PRIVATE_FIXTURE_WAGMI_STORAGE_KEY).toBe(deployment.PRIVATE_FIXTURE_WAGMI_STORAGE_KEY);
  expect(output.OPTIONAL_CONTRACT_CAPABILITIES).toEqual(deployment.OPTIONAL_CONTRACT_CAPABILITIES);
  const approval = output.isActiveApprovalSpender as typeof deployment.isActiveApprovalSpender;
  expect(approval(data.router)).toBe(true);
  expect(approval(data.treasury)).toBe(false);
  const assertApproval = output.assertActiveApprovalSpender as typeof deployment.assertActiveApprovalSpender;
  expect(() => assertApproval(data.router)).not.toThrow();
  expect(() => assertApproval(data.treasury)).toThrow(/no verified active deployment/);
  const assertOptional = output.assertOptionalContractCapability as typeof deployment.assertOptionalContractCapability;
  for (const capability of ['metaAg', 'pecorOrders'] as const) {
    expect(() => assertOptional(capability)).toThrow(/no verified active deployment/);
  }
  expect(Object.isFrozen(output.CORE_DEPLOYMENT)).toBe(true);
  expect(Object.isFrozen(output.OPTIONAL_CONTRACT_CAPABILITIES)).toBe(true);
  expect(Object.isFrozen(output.CORE_TRADE_EXECUTION_ROLES)).toBe(true);
}

function deploymentRegenerationRetainsReviewedIdentities() {
  const data = generator.deriveDeployment(contractRoot);
  for (const [key, role] of Object.entries({
    router: 'Router', quoter: 'Quoter', feesRouter: 'FeesRouter', opticalRegistry: 'OpticalRegistry',
    protocolConfig: 'ProtocolConfig', factory: 'SidioraFactory', nft: 'SidioraNFT',
    eventEmitter: 'EventEmitter', feeAccumulator: 'FeeAccumulator', poolRegistry: 'PoolRegistry',
    poolBeacon: 'PoolBeacon', treasury: 'Treasury', usdl: 'USDL', launchpadPrecompile: 'LaunchpadPrecompile',
  }) as Array<[keyof typeof data.roleIdentity, string]>) {
    const { address, ...identity } = reviewed.roles[role];
    expect(data[key].toLowerCase()).toBe(address.toLowerCase());
    expect(data.roleIdentity[key]).toEqual(identity);
  }
}

function deploymentRegenerationRejectsUnreviewedTreasury() {
  const directory = mkdtempSync(join(tmpdir(), 'deployment-regeneration-'));
  try {
    mkdirSync(join(directory, 'deployments'));
    const manifest = JSON.parse(readFileSync(manifestFile, 'utf8'));
    for (const mutation of ['missing', 'unconfirmed', 'record', 'address'] as const) {
      const changed = structuredClone(manifest);
      if (mutation === 'missing') delete changed.Treasury_proxy;
      if (mutation === 'unconfirmed') changed._deployment.records.Treasury_proxy.status = 'pending';
      if (mutation === 'record') changed._deployment.records.Treasury_proxy.address = manifest.Router_proxy;
      if (mutation === 'address') {
        changed.Treasury_proxy = manifest.Router_proxy;
        changed._deployment.records.Treasury_proxy.address = manifest.Router_proxy;
      }
      writeFileSync(join(directory, 'deployments/paxeer-x-network-addresses.json'), JSON.stringify(changed));
      expect(() => generator.deriveDeployment(directory)).toThrow(/Treasury/);
    }
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

function deploymentRegenerationFeedsWebConsumers() {
  const data = generator.deriveDeployment(contractRoot);
  expect([
    contracts.ROUTER_ADDRESS, contracts.QUOTER_ADDRESS, contracts.FEES_ROUTER_ADDRESS,
    contracts.OPTICAL_REGISTRY_ADDRESS, contracts.PROTOCOL_CONFIG_ADDRESS, contracts.USDL_ADDRESS,
  ]).toEqual([data.router, data.quoter, data.feesRouter, data.opticalRegistry, data.protocolConfig, data.usdl]);
  expect(productionTradeDeployment).toEqual({
    revision: data.manifestSha256, router: data.router, quoter: data.quoter,
    registry: data.poolRegistry, emitter: data.eventEmitter, usdl: data.usdl,
    pins: deployment.CORE_TRADE_EXECUTION_ROLES.map(role => {
      const identity = data.roleIdentity[role];
      return { address: data[role], runtimeHash: identity.runtimeHash,
        ...('implementationAddress' in identity ? { implementation: identity.implementationAddress, implementationHash: identity.implementationHash } : {}) };
    }),
  });
  expect(contracts.erc20ApprovalRequest({ token: data.usdl, spender: data.router, amount: 1n })).toMatchObject({
    chainId: data.chainId, address: data.usdl, functionName: 'approve', args: [data.router, 1n],
  });
  expect(() => contracts.erc20ApprovalRequest({ token: data.usdl, spender: data.treasury, amount: 1n })).toThrow(/spender/);
}

async function deploymentRegenerationPreservesProviderCookieStorage() {
  const storage = createPrivateFixtureWagmiStorage();
  const publicStorage = createStorage({ storage: cookieStorage });
  try {
    await storage.setItem('recentConnectorId', 'injected');
    expect(document.cookie).toContain(`${deployment.PRIVATE_FIXTURE_WAGMI_STORAGE_KEY}.recentConnectorId=`);
    expect(await publicStorage.getItem('recentConnectorId')).toBeNull();
    const state = { chainId: deployment.CORE_DEPLOYMENT.chainId, connections: new Map(), current: null };
    await storage.setItem('store', { state, version: 1 });
    const config = createWagmiConfig();
    const recovered = cookieToInitialState({ ...config, storage }, document.cookie);
    expect(recovered).toEqual(state);
    const reopened = createPrivateFixtureWagmiStorage();
    expect(await reopened.getItem('store')).toEqual({ state, version: 1 });
  } finally {
    await storage.removeItem('recentConnectorId');
    await storage.removeItem('store');
  }
}

test(deploymentRegenerationPreservesFullHelper.name, deploymentRegenerationPreservesFullHelper);
test(deploymentRegenerationRetainsReviewedIdentities.name, deploymentRegenerationRetainsReviewedIdentities);
test(deploymentRegenerationRejectsUnreviewedTreasury.name, deploymentRegenerationRejectsUnreviewedTreasury);
test(deploymentRegenerationFeedsWebConsumers.name, deploymentRegenerationFeedsWebConsumers);
test(deploymentRegenerationPreservesProviderCookieStorage.name, deploymentRegenerationPreservesProviderCookieStorage);

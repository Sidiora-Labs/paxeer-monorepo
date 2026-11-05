import { spawn } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { createPublicClient, createTestClient, keccak256, pad, type Hex, type PublicClient, type TestClient } from 'viem';
import { getIpcRpcClient, ipc } from 'viem/node';
import { expect, test } from 'vitest';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { resolveAnvilBinary } from '@/core/test-anvil';
import { ChainTradeProvider, EvidenceIntegrityError, productionTradeDeployment, type TradeDeployment } from './providers';

const surfaceRuntime = '0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3';
const implementationSlot = '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc';
const surfaceRoles = ['router', 'quoter', 'poolRegistry'] as const;

function artifactRuntime(path: string): Hex {
  const artifact: unknown = JSON.parse(readFileSync(resolve(__dirname, '../../../../../../../contracts/launchpad/artifacts', path), 'utf8'));
  if (!artifact || typeof artifact !== 'object' || !('deployedBytecode' in artifact) ||
      typeof artifact.deployedBytecode !== 'string' || !/^0x(?:[0-9a-fA-F]{2})+$/.test(artifact.deployedBytecode)) {
    throw new Error(`Compiled runtime unavailable: ${path}`);
  }
  return artifact.deployedBytecode as Hex;
}

async function withSurfaceChain(run: (fixture: {
  provider: ChainTradeProvider;
  client: PublicClient;
  node: TestClient<'anvil'>;
  proxyRuntime: Hex;
}) => Promise<void>) {
  const binary = resolveAnvilBinary(process.env);
  const proxyRuntime = artifactRuntime('contracts/meta-ag/proxy/ERC1967Proxy.sol/ERC1967Proxy.json');
  const emitterRuntime = artifactRuntime('contracts/data/EventEmitter.sol/EventEmitter.json');
  const directory = mkdtempSync(join(tmpdir(), 'trade-surface-'));
  const socketPath = join(directory, 'anvil.ipc');
  const proc = spawn(binary, [
    '--host', '127.0.0.1', '--port', '0', '--ipc', socketPath, '--chain-id', '125',
    '--accounts', '0', '--threads', '2', '--silent',
  ], { stdio: 'ignore' });
  let spawnError: Error | undefined;
  proc.once('error', error => { spawnError = error; });
  const stopped = new Promise<void>(resolve => proc.once('close', () => resolve()));
  const client = createPublicClient({ transport: ipc(socketPath, { reconnect: false, retryCount: 0, timeout: 500 }) });
  const node = createTestClient({ mode: 'anvil', transport: ipc(socketPath, { reconnect: false, retryCount: 0, timeout: 500 }) });
  let connected = false;
  function alive() {
    if (spawnError) throw spawnError;
    if (proc.exitCode !== null || proc.signalCode !== null) throw new Error('Owned Anvil exited');
  }
  try {
    const deadline = Date.now() + 5000;
    while (!connected && Date.now() < deadline) {
      alive();
      try {
        expect(await client.getChainId()).toBe(125);
        connected = true;
      } catch {
        await delay(25);
      }
    }
    expect(connected).toBe(true);
    alive();
    expect((surfaceRuntime.length - 2) / 2).toBe(41);
    for (const role of surfaceRoles) {
      expect(keccak256(surfaceRuntime)).toBe(CORE_DEPLOYMENT.roleIdentity[role].runtimeHash);
      await node.setCode({ address: CORE_DEPLOYMENT[role], bytecode: surfaceRuntime });
    }
    const emitterPin = productionTradeDeployment.pins.find(pin => pin.address === CORE_DEPLOYMENT.eventEmitter);
    expect(emitterPin).toEqual({
      address: CORE_DEPLOYMENT.eventEmitter,
      runtimeHash: CORE_DEPLOYMENT.roleIdentity.eventEmitter.runtimeHash,
      implementation: CORE_DEPLOYMENT.roleIdentity.eventEmitter.implementationAddress,
      implementationHash: CORE_DEPLOYMENT.roleIdentity.eventEmitter.implementationHash,
    });
    const implementation = CORE_DEPLOYMENT.roleIdentity.eventEmitter.implementationAddress;
    await node.setCode({ address: CORE_DEPLOYMENT.eventEmitter, bytecode: proxyRuntime });
    await node.setCode({ address: implementation, bytecode: emitterRuntime });
    await node.setStorageAt({ address: CORE_DEPLOYMENT.eventEmitter, index: implementationSlot, value: pad(implementation) });
    const deployment: TradeDeployment = {
      ...productionTradeDeployment,
      pins: productionTradeDeployment.pins.map(pin => pin.address === CORE_DEPLOYMENT.eventEmitter ? {
        address: pin.address, runtimeHash: keccak256(proxyRuntime), implementation,
        implementationHash: keccak256(emitterRuntime),
      } : pin),
    };
    await node.mine({ blocks: 1 });
    const provider = new ChainTradeProvider(client, undefined, deployment);
    await run({ provider, client, node, proxyRuntime });
  } finally {
    if (connected) (await getIpcRpcClient(socketPath, { reconnect: false })).close();
    proc.kill('SIGTERM');
    await stopped;
    rmSync(directory, { recursive: true, force: true });
  }
}

async function chainTradeProviderAcceptsSurfaceRuntime() {
  await withSurfaceChain(async ({ provider, client }) => {
    const blockNumber = await client.getBlockNumber({ cacheTime: 0 });
    for (const role of surfaceRoles) {
      const address = CORE_DEPLOYMENT[role];
      expect(productionTradeDeployment.pins.find(pin => pin.address === address)).toEqual({
        address, runtimeHash: CORE_DEPLOYMENT.roleIdentity[role].runtimeHash,
      });
      expect(await client.getCode({ address, blockNumber })).toBe(surfaceRuntime);
      expect(await client.getStorageAt({ address, slot: implementationSlot, blockNumber })).toBe(pad('0x0'));
    }
    await expect(provider.verify(blockNumber)).resolves.toBeUndefined();
  });
}

async function chainTradeProviderRejectsProxyCodeAtSurfaceRole() {
  await withSurfaceChain(async ({ provider, client, node, proxyRuntime }) => {
    expect(keccak256(proxyRuntime)).not.toBe(keccak256(surfaceRuntime));
    await expect(provider.verify(await client.getBlockNumber({ cacheTime: 0 }))).resolves.toBeUndefined();
    for (const role of surfaceRoles) {
      const address = CORE_DEPLOYMENT[role];
      await node.setCode({ address, bytecode: proxyRuntime });
      await node.mine({ blocks: 1 });
      const blockNumber = await client.getBlockNumber({ cacheTime: 0 });
      expect(await client.getCode({ address, blockNumber })).toBe(proxyRuntime);
      await expect(provider.verify(blockNumber)).rejects.toThrow(new EvidenceIntegrityError('deployment_code_mismatch'));
      await node.setCode({ address, bytecode: surfaceRuntime });
      await node.mine({ blocks: 1 });
      await expect(provider.verify(await client.getBlockNumber({ cacheTime: 0 }))).resolves.toBeUndefined();
    }
  });
}

test(chainTradeProviderAcceptsSurfaceRuntime, { timeout: 15000 }, chainTradeProviderAcceptsSurfaceRuntime);
test(chainTradeProviderRejectsProxyCodeAtSurfaceRole, { timeout: 15000 }, chainTradeProviderRejectsProxyCodeAtSurfaceRole);

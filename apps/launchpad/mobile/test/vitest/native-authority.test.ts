import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import { afterAll, beforeAll, expect, test } from 'vitest';
import {
  createPublicClient,
  createTestClient,
  getAddress,
  http,
  keccak256,
  type Hex,
} from 'viem';
import {
  verifyRolePins,
  type RolePin,
} from '../../src/runtime/native-authority.ts';

const surfaceRuntime =
  '0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3';
const surfaceRuntimeHash =
  '0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f';
const proxyRuntime: Hex = JSON.parse(
  readFileSync(
    new URL(
      '../../../../../sdk/launchpad/shared/config/deployment/testdata/evm-artifacts.json',
      import.meta.url,
    ),
    'utf8',
  ),
).contracts.UUPSProxy.runtime;
const pins = [
  '0x78FB06f525cD964388f2305A489035263E31C6e4',
  '0x793006f4F321b8863F26DBD75b11bAa01DA181dc',
  '0x1dC357d2B7B850332a91F06A9E66634F46fC4Ba7',
].map(
  (address): RolePin => ({
    address: getAddress(address),
    kind: 'surface',
    runtimeHash: surfaceRuntimeHash,
  }),
);

function chainClients(url: string) {
  const transport = http(url, { retryCount: 0, timeout: 2000 });
  return {
    reader: createPublicClient({ transport }),
    writer: createTestClient({ mode: 'anvil', transport }),
  };
}

let anvil: ChildProcess | undefined;
let clients: ReturnType<typeof chainClients>;

beforeAll(async () => {
  const url = await new Promise<string>((resolve, reject) => {
    let output = '';
    const timer = setTimeout(
      () => reject(new Error(`Anvil startup timed out: ${output}`)),
      10000,
    );
    anvil = spawn(
      process.env.ANVIL_BIN || 'anvil',
      [
        '--host',
        '127.0.0.1',
        '--port',
        '0',
        '--chain-id',
        '125',
        '--no-mining',
      ],
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
  clients = chainClients(url);
  expect(await clients.reader.getChainId()).toBe(125);
}, 15000);

afterAll(async () => {
  if (anvil && anvil.exitCode === null && anvil.signalCode === null) {
    const exited = once(anvil, 'exit');
    anvil.kill('SIGTERM');
    await exited;
  }
});

async function installSurfaceRoles() {
  for (const pin of pins) {
    await clients.writer.setCode({
      address: pin.address,
      bytecode: surfaceRuntime,
    });
  }
  await clients.writer.mine({ blocks: 1 });
  return clients.reader.getBlockNumber({ cacheTime: 0 });
}

async function surfacePinAcceptsSurfaceRuntime() {
  expect(keccak256(surfaceRuntime)).toBe(surfaceRuntimeHash);
  const block = await installSurfaceRoles();
  for (const pin of pins) {
    expect(
      await clients.reader.getCode({
        address: pin.address,
        blockNumber: block,
      }),
    ).toBe(surfaceRuntime);
    expect(
      await clients.reader.getStorageAt({
        address: pin.address,
        slot: '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc',
        blockNumber: block,
      }),
    ).toBe(`0x${'00'.repeat(32)}`);
  }
  await expect(
    verifyRolePins(clients.reader, pins, block),
  ).resolves.toBeUndefined();
}

async function surfacePinRejectsProxyRuntime() {
  expect(keccak256(proxyRuntime)).toBe(
    '0xb2f39d5c380b45131e836d4beb5254d4e5c410d35935bf270e8492755f7da6f7',
  );
  const surfaceBlock = await installSurfaceRoles();
  await clients.writer.mine({ blocks: 1 });
  for (const pin of pins) {
    await clients.writer.setCode({
      address: pin.address,
      bytecode: proxyRuntime,
    });
    await clients.writer.mine({ blocks: 1 });
    const block = await clients.reader.getBlockNumber({ cacheTime: 0 });
    expect(
      await clients.reader.getCode({
        address: pin.address,
        blockNumber: block,
      }),
    ).toBe(proxyRuntime);
    await expect(verifyRolePins(clients.reader, pins, block)).rejects.toThrow(
      'Native trading code identity changed',
    );
    await expect(
      verifyRolePins(clients.reader, pins, surfaceBlock),
    ).resolves.toBeUndefined();
    await clients.writer.setCode({
      address: pin.address,
      bytecode: surfaceRuntime,
    });
  }
}

test(surfacePinAcceptsSurfaceRuntime, surfacePinAcceptsSurfaceRuntime);
test(surfacePinRejectsProxyRuntime, surfacePinRejectsProxyRuntime);

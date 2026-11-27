import assert from 'node:assert/strict';
import { spawn, execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { readdirSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'node:net';
import { once } from 'node:events';

const execute = promisify(execFile);
export const repository = fileURLToPath(new URL('../../../../../', import.meta.url));
const contracts = resolve(repository, 'contracts/launchpad');

async function availablePort() {
  const listener = createServer();
  listener.listen(0, '127.0.0.1');
  await once(listener, 'listening');
  const port = listener.address().port;
  await new Promise((done) => listener.close(done));
  return port;
}

export async function startLaunchpadChain() {
  const binary = process.env.ANVIL_BIN ?? 'anvil';
  try {
    await execute(binary, ['--version']);
  } catch {
    throw new Error('The local launchpad journey requires anvil. Install anvil or set ANVIL_BIN.');
  }
  await execute(
    'corepack',
    [
      'pnpm',
      '--dir',
      contracts,
      '--ignore-workspace',
      'install',
      '--frozen-lockfile',
      '--ignore-scripts',
    ],
    { cwd: contracts, maxBuffer: 4 * 1024 * 1024 },
  );
  await execute(
    'corepack',
    ['pnpm', '--dir', contracts, '--ignore-workspace', 'run', 'compile'],
    { cwd: contracts, maxBuffer: 4 * 1024 * 1024 },
  );
  const require = createRequire(resolve(contracts, 'package.json'));
  const ethers = require('ethers');
  const port = await availablePort();
  const child = spawn(
    binary,
    ['--host', '127.0.0.1', '--port', String(port), '--chain-id', '125', '--silent'],
    { stdio: ['ignore', 'pipe', 'pipe'] },
  );
  let output = '';
  child.stdout.on('data', (value) => {
    output += value;
  });
  child.stderr.on('data', (value) => {
    output += value;
  });
  child.on('error', (error) => {
    output += error.message;
  });
  const rpcUrl = `http://127.0.0.1:${port}`;
  const provider = new ethers.JsonRpcProvider(rpcUrl, 125, {
    staticNetwork: true,
    cacheTimeout: -1,
    batchMaxCount: 1,
  });
  provider.pollingInterval = 50;
  const stop = async () => {
    provider.destroy();
    if (child.exitCode === null && child.signalCode === null) {
      const exited = once(child, 'exit');
      child.kill('SIGTERM');
      const timer = setTimeout(() => child.kill('SIGKILL'), 5000);
      await exited;
      clearTimeout(timer);
    }
  };
  try {
    const until = Date.now() + 15000;
    for (;;) {
      assert.equal(child.exitCode, null, output);
      try {
        assert.equal(await provider.send('eth_chainId', []), '0x7d');
        break;
      } catch (error) {
        if (Date.now() > until) throw error;
      }
      await new Promise((done) => setTimeout(done, 50));
    }
    const signer = await provider.getSigner(0);
    const admin = await signer.getAddress();
    const artifactFiles = [];
    function walk(directory) {
      for (const entry of readdirSync(directory, { withFileTypes: true })) {
        const path = resolve(directory, entry.name);
        if (entry.isDirectory()) walk(path);
        else if (entry.name.endsWith('.json') && !entry.name.endsWith('.dbg.json'))
          artifactFiles.push(path);
      }
    }
    walk(resolve(contracts, 'artifacts/contracts'));
    const artifacts = new Map();
    function artifact(name) {
      if (artifacts.has(name)) return artifacts.get(name);
      const matches = artifactFiles.filter((path) => path.endsWith(`/${name}.json`));
      assert.equal(matches.length, 1, name);
      const file = matches[0];
      const value = JSON.parse(readFileSync(file, 'utf8'));
      const debug = JSON.parse(readFileSync(file.replace(/\.json$/, '.dbg.json'), 'utf8'));
      const build = JSON.parse(readFileSync(resolve(dirname(file), debug.buildInfo), 'utf8'));
      for (const [source, input] of Object.entries(build.input.sources)) {
        if (source.startsWith('contracts/'))
          assert.equal(readFileSync(resolve(contracts, source), 'utf8'), input.content, source);
      }
      artifacts.set(name, value);
      return value;
    }
    const receipts = [];
    async function confirmed(transaction) {
      const receipt = await (await transaction).wait();
      assert.equal(receipt.status, 1);
      receipts.push(receipt);
      return receipt;
    }
    async function deploy(name, args = []) {
      const value = artifact(name);
      const contract = await new ethers.ContractFactory(value.abi, value.bytecode, signer).deploy(
        ...args,
      );
      await confirmed(contract.deploymentTransaction());
      return contract;
    }
    async function proxy(name, args) {
      const implementation = await deploy(name);
      const deployed = await deploy('UUPSProxy', [
        implementation.target,
        implementation.interface.encodeFunctionData('initialize', args),
      ]);
      return new ethers.Contract(deployed.target, artifact(name).abi, signer);
    }
    const usdl = await deploy('SidioraERC20', [
      'USD Ledger',
      'USDL',
      1000000000n * 1000000n,
      admin,
    ]);
    const emitter = await proxy('EventEmitter', [admin]);
    await confirmed(emitter.reinitializeV2(admin));
    const config = await proxy('ProtocolConfig', [usdl.target, emitter.target, admin]);
    const treasury = await proxy('Treasury', [emitter.target, admin]);
    const registry = await proxy('PoolRegistry', [emitter.target, admin]);
    const accumulator = await proxy('FeeAccumulator', [
      config.target,
      treasury.target,
      registry.target,
      emitter.target,
      usdl.target,
      admin,
    ]);
    const nft = await proxy('SidioraNFT', ['Sidiora Pool NFT', 'SIDNFT', emitter.target, admin]);
    const implementation = await deploy('SidioraPool');
    const beacon = await deploy('PoolBeacon', [implementation.target, admin]);
    const factory = await proxy('SidioraFactory', [
      beacon.target,
      nft.target,
      registry.target,
      emitter.target,
      config.target,
      treasury.target,
      accumulator.target,
      usdl.target,
      admin,
    ]);
    const router = await proxy('Router', [
      factory.target,
      registry.target,
      config.target,
      usdl.target,
      admin,
    ]);
    const quoter = await proxy('Quoter', [registry.target, config.target, admin]);
    const role = (value) => ethers.id(value);
    for (const [contract, name, grantee] of [
      [registry, 'FACTORY_ROLE', factory.target],
      [accumulator, 'FACTORY_ROLE', factory.target],
      [nft, 'MINTER_ROLE', factory.target],
      [treasury, 'DEPOSITOR_ROLE', accumulator.target],
      [treasury, 'DEPOSITOR_ROLE', factory.target],
      [treasury, 'DEPOSITOR_ROLE', router.target],
      [factory, 'ROUTER_ROLE', router.target],
    ])
      await confirmed(contract.grantRole(role(name), grantee));
    await confirmed(emitter.setPoolRegistry(registry.target));
    await confirmed(emitter.setSidioraFactory(factory.target));
    for (const producer of [config, treasury, registry, accumulator, nft, router])
      await confirmed(emitter.setAuthorizedEmitter(producer.target, true));
    for (const [producer, names] of [
      [config, ['emitConfigUpdated']],
      [registry, ['emitPoolRegistered']],
      [accumulator, ['emitFeeFlow', 'emitFeeRecorded', 'emitFeeDistributed']],
      [nft, ['emitFeeStrategyChanged', 'emitNftMint']],
      [router, ['emitRouterTrade']],
    ])
      for (const name of names)
        await confirmed(
          emitter.setTypedEmitter(
            producer.target,
            emitter.interface.getFunction(name).selector,
            true,
          ),
        );
    assert.equal(await config.creationFee(), 1000000n);
    const deployment = {
      chainId: 125,
      router: router.target,
      quoter: quoter.target,
      protocolConfig: config.target,
      usdl: usdl.target,
      spender: router.target,
      poolRegistry: registry.target,
      creationFeeRaw: '1000000',
    };
    return {
      rpcUrl,
      provider,
      signer,
      ethers,
      usdl,
      router,
      quoter,
      registry,
      factory,
      config,
      receipts,
      artifact,
      confirmed,
      deployment,
      stop,
    };
  } catch (error) {
    await stop();
    throw error;
  }
}

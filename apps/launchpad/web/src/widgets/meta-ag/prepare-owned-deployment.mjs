import { createServer } from 'node:http';
import { createHash, randomBytes } from 'node:crypto';
import { readFileSync, writeFileSync, readdirSync, mkdirSync, appendFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

export function prepareOwnedCoreDeployment(inputFile, profileFile, creationFile, tradeFile) {
  const value = JSON.parse(readFileSync(inputFile, 'utf8'));
  const profile = JSON.parse(readFileSync(profileFile, 'utf8'));
  const core = value.phase2;
  if (!core?.ownedFixture || profile.profile !== 'PRIVATE_FIXTURE' ||
      profile.revision.deploymentVersion !== core.authorityDeployment.deploymentVersion)
    throw new Error('Source-bound owned creation authority required');
  const roles = profile.roles;
  const deployment = core.deployment;
  for (const [field, role] of Object.entries({ router: 'Router', usdl: 'USDL', poolRegistry: 'PoolRegistry', quoter: 'Quoter', factory: 'SidioraFactory' }))
    if (deployment[field].toLowerCase() !== roles[role].address.toLowerCase())
      throw new Error('Owned deployment role differs: ' + role);
  if (deployment.usdlSpender.toLowerCase() !== roles.Router.address.toLowerCase())
    throw new Error('Owned Solidity deployment must spend through its Router proxy');
  deployment.revision = profile.revision.revisionSha256;
  deployment.tradeDeployment = {
    revision: deployment.revision,
    router: roles.Router.address, quoter: roles.Quoter.address,
    registry: roles.PoolRegistry.address, emitter: roles.EventEmitter.address,
    usdl: roles.USDL.address,
    pins: ['Router', 'Quoter', 'PoolRegistry', 'EventEmitter'].map(name => ({
      address: roles[name].address, runtimeHash: roles[name].runtimeHash,
      implementation: roles[name].implementationAddress,
      implementationHash: roles[name].implementationHash,
    })),
  };
  writeFileSync(creationFile, JSON.stringify(value, null, 2) + '\n', { mode: 0o600, flag: 'wx' });
  writeFileSync(tradeFile, JSON.stringify({
    realm: 'private-anvil125', productionActivation: false,
    manifest: core.authorityDeployment, genesisHash: core.genesisHash,
    account: core.tradingAccount, pool: core.pool, token: core.token, deployment,
  }, null, 2) + '\n', { mode: 0o600, flag: 'wx' });
}

async function prepareAdvancedDeployment() {
const [coreFile, output, contracts, readyFile] = process.argv.slice(2);
const core = JSON.parse(readFileSync(coreFile, 'utf8')).phase2;
if (!core?.ownedFixture || !/^http:\/\/127\.0\.0\.1:\d+$/.test(core.rpcUrl))
  throw new Error('Owned creation deployment required for advanced inputs');
mkdirSync(output, { mode: 0o700 });
const ethers = await import(
  pathToFileURL(join(contracts, 'node_modules/ethers/lib.esm/index.js')).href
);
const provider = new ethers.JsonRpcProvider(core.rpcUrl);
if ((await provider.getNetwork()).chainId !== 125n)
  throw new Error('Advanced chain mismatch');
const [admin, user, keeper, relayer] = await Promise.all(
  [0, 6, 4, 5].map((index) => provider.getSigner(index))
);
const sha = (bytes) => createHash('sha256').update(bytes).digest('hex');
const artifactFiles = [];
function inventory(path) {
  for (const entry of readdirSync(path, { withFileTypes: true })) {
    const file = join(path, entry.name);
    if (entry.isDirectory()) inventory(file);
    else if (entry.name.endsWith('.json') && !entry.name.endsWith('.dbg.json'))
      artifactFiles.push(file);
  }
}
inventory(join(contracts, 'artifacts/contracts'));
const sources = {};
sources[new URL(import.meta.url).pathname] = sha(readFileSync(new URL(import.meta.url)));
function artifact(name) {
  const files = artifactFiles.filter((file) => file.endsWith('/' + name + '.json'));
  if (files.length !== 1)
    throw new Error('Unique compiled advanced artifact required: ' + name);
  const bytes = readFileSync(files[0]);
  sources[files[0]] = sha(bytes);
  return JSON.parse(bytes);
}
const deploy = async (name, args = []) => {
  const compiled = artifact(name);
  if (name.startsWith('Mock'))
    throw new Error('Advanced qualification cannot deploy mock contracts');
  const contract = await new ethers.ContractFactory(
    compiled.abi,
    compiled.bytecode,
    admin
  ).deploy(...args);
  await contract.waitForDeployment();
  const receipt = await contract.deploymentTransaction().wait();
  if (receipt.status !== 1) throw new Error('Advanced deployment failed: ' + name);
  appendFileSync(
    join(output, 'deployments.ndjson'),
    JSON.stringify({
      name,
      address: contract.target,
      hash: receipt.hash,
      blockHash: receipt.blockHash,
      artifactSha256:
        sources[artifactFiles.find((file) => file.endsWith('/' + name + '.json'))],
    }) + '\n',
    { mode: 0o600 }
  );
  return contract;
};
const proxy = async (name, args) => {
  const implementation = await deploy(name);
  const address = await deploy('UUPSProxy', [
    implementation.target,
    implementation.interface.encodeFunctionData('initialize', args),
  ]);
  return implementation.attach(address.target);
};
const commit = async (transaction) => {
  const receipt = await (await transaction).wait();
  if (receipt.status !== 1) throw new Error('Advanced setup transaction reverted');
  return receipt;
};
const roles = core.authorityDeployment.roles;
const usdl = new ethers.Contract(roles.USDL.address, artifact('SidioraERC20').abi, admin);
const token = new ethers.Contract(core.token, artifact('SidioraERC20').abi, admin);
const pool = new ethers.Contract(core.pool, artifact('SidioraPool').abi, admin);
const coreRouter = new ethers.Contract(roles.Router.address, artifact('Router').abi, admin);
const wrapped = await deploy('WPAX9');
const oracle = await proxy('PriceOracle', [admin.address]);
const hub = await proxy('OracleHub', [oracle.target, 10000, 3000, admin.address]);
const priceAdapter = await deploy('PriceOracleAdapter', [oracle.target]);
await commit(hub.registerAdapter(priceAdapter.target, 10));
await commit(oracle.setRelayer(admin.address, true));
await commit(oracle.setRelayer(relayer.address, true));
const tracker = await proxy('TransactionTracker', [admin.address]);
const vault = await proxy('PECORVault', [wrapped.target, tracker.target, admin.address]);
const engine = await proxy('PECOR', [
  oracle.target,
  vault.target,
  wrapped.target,
  tracker.target,
  admin.address,
]);
const orders = await proxy('PECOROrders', [
  oracle.target,
  vault.target,
  tracker.target,
  admin.address,
]);
const vaultAdapter = await deploy('VaultAdapter', [
  vault.target,
  oracle.target,
  0,
  admin.address,
  admin.address,
]);
const sidioraAdapter = await deploy('SidioraAdapter', [
  roles.PoolRegistry.address,
  roles.Quoter.address,
  roles.Router.address,
  usdl.target,
  admin.address,
]);
const router = await proxy('MetaAGRouter', [hub.target, 10000, admin.address]);
const quoter = await proxy('MetaAGQuoter', [
  oracle.target,
  vault.target,
  wrapped.target,
  engine.target,
  admin.address,
]);
for (const operator of [engine, orders, vaultAdapter])
  await commit(vault.setOperator(operator.target, true));
await commit(orders.setKeeper(keeper.address, true));
for (const emitter of [engine, orders, router])
  await commit(tracker.setAuthorizedEmitter(emitter.target, true));
// Both registered paths settle with the actual vault or the actual Sidiora pool.
await commit(router.registerAdapter(vaultAdapter.target));
await commit(router.registerAdapter(sidioraAdapter.target));
const one = 10n ** 18n;
const poolPrice = await pool.getPrice();
for (const [contract, price] of [
  [usdl, one],
  [token, poolPrice],
  [wrapped, 2n * one],
]) {
  if (price <= 0n) throw new Error('Owned oracle price must be positive');
  await commit(oracle.registerToken(contract.target, 60, 100, 1, one * 1000000n, 3600));
  await commit(oracle.updatePrice(contract.target, price));
  await commit(vault.registerToken(contract.target, contract === usdl));
}
await commit(usdl.approve(coreRouter.target, ethers.MaxUint256));
await commit(
  coreRouter.buy(
    core.pool,
    1000000000n,
    1,
    (await provider.getBlock('latest')).timestamp + 3600
  )
);
await commit(usdl.approve(vault.target, ethers.MaxUint256));
await commit(vault.deposit(usdl.target, 1000000000000n));
const tokens = await token.balanceOf(admin.address);
if (tokens <= 0n) throw new Error('Actual core purchase did not fund the advanced vault');
await commit(token.approve(vault.target, tokens));
await commit(vault.deposit(token.target, tokens));
await commit(wrapped.deposit({ value: 1000n * one }));
await commit(wrapped.approve(vault.target, 1000n * one));
await commit(vault.deposit(wrapped.target, 1000n * one));
await commit(usdl.transfer(user.address, 10000000000000n));
await commit(usdl.connect(user).approve(orders.target, ethers.MaxUint256));
await commit(usdl.connect(user).approve(router.target, ethers.MaxUint256));
const publishPrices = async () => {
  const currentPrice = await pool.getPrice();
  if (currentPrice <= 0n) throw new Error('Owned pool price is unavailable');
  for (const [asset, price] of [[usdl.target, one], [token.target, currentPrice], [wrapped.target, 2n * one]]) {
    const receipt = await commit(oracle.connect(relayer).updatePrice(asset, price));
    appendFileSync(join(output, 'price-publications.ndjson'), JSON.stringify({
      asset, price: String(price), hash: receipt.hash, blockHash: receipt.blockHash,
    }) + '\n', {mode: 0o600});
  }
};
await publishPrices();
let publishing = false;
const publications = setInterval(async () => {
  if (publishing) return;
  publishing = true;
  try { await publishPrices(); }
  catch (error) { console.error(error); process.exit(1); }
  finally { publishing = false; }
}, 5000);
const revision = sha(JSON.stringify({ sources, core: sha(readFileSync(coreFile)) }));
const executionKey = randomBytes(32).toString('hex');
const server = createServer(async (req, res) => {
  try {
    if (req.method === 'GET' && req.url === '/health') {
      const [chain, authorized, paused, balance, code] = await Promise.all([
        provider.getNetwork(),
        orders.hasRole(ethers.id('KEEPER_ROLE'), keeper.address),
        orders.orderExecutionPaused(),
        provider.getBalance(keeper.address),
        provider.getCode(orders.target),
      ]);
      const healthy =
        chain.chainId === 125n && authorized && !paused && balance > 0n && code !== '0x';
      res.writeHead(healthy ? 200 : 503, {
        'Content-Type': 'application/json',
        'Cache-Control': 'no-store',
      });
      res.end(
        JSON.stringify({
          chainId: 125,
          manifestRevision: revision,
          healthy,
          checkedAt: Date.now(),
        })
      );
      return;
    }
    if (
      req.method === 'POST' &&
      req.url === '/execute' &&
      req.headers.authorization === 'Bearer ' + executionKey
    ) {
      let body = '';
      for await (const chunk of req) {
        body += chunk;
        if (body.length > 1024) throw new Error('Bounded execution request required');
      }
      const input = JSON.parse(body);
      if (
        !/^[1-9][0-9]{0,77}$/.test(input.orderId) ||
        !['limit', 'stop'].includes(input.family)
      )
        throw new Error('Invalid order identity');
      const receipt = await commit(
        orders
          .connect(keeper)
          [
            input.family === 'limit' ? 'executeLimitOrder' : 'executeStopLimitOrder'
          ](BigInt(input.orderId))
      );
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ transactionHash: receipt.hash }));
      return;
    }
    res.writeHead(404).end();
  } catch (error) {
    console.error(error);
    if (!res.headersSent) res.writeHead(503);
    res.end();
  }
});
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const binding = async (contract) => {
  const value = {
    address: contract.target,
    runtimeCodeHash: ethers.keccak256(await provider.getCode(contract.target)),
  };
  const slot = await provider.getStorage(
    contract.target,
    '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc'
  );
  if (BigInt(slot) !== 0n) {
    const address = ethers.getAddress('0x' + slot.slice(-40));
    value.implementation = {
      address,
      runtimeCodeHash: ethers.keccak256(await provider.getCode(address)),
    };
  }
  return value;
};
const adapters = [];
for (const [contract, adapterData] of [
  [vaultAdapter, 'vault-v2'],
  [sidioraAdapter, 'sidiora-v1'],
])
  adapters.push({
    ...(await binding(contract)),
    id: await contract.adapterId(),
    adapterData,
  });
const manifestFile = join(output, 'manifest.json');
writeFileSync(
  manifestFile,
  JSON.stringify({
    version: 2,
    chainId: 125,
    revision,
    deploymentBlock: String(await provider.getBlockNumber()),
    confirmations: 1,
    usdl: usdl.target,
    orders: await binding(orders),
    router: await binding(router),
    quoter: await binding(quoter),
    vault: await binding(vault),
    adapters,
    priceScale: 18,
    priceDenomination: usdl.target,
    supportedKinds: [
      'limit-buy',
      'limit-sell',
      'stop-loss',
      'stop-limit-buy',
      'stop-limit-sell',
    ],
    maximumOrderLifetimeSeconds: '2592000',
    gtcSentinel: '0',
    keeperHealthUrl: `http://127.0.0.1:${server.address().port}/health`,
  }) + '\n',
  { mode: 0o600, flag: 'wx' }
);
const keyFile = (index, address) => {
  const account = ethers.HDNodeWallet.fromPhrase(
    'test test test test test test test test test test test junk',
    undefined,
    `m/44'/60'/0'/0/${index}`
  );
  if (account.address !== ethers.getAddress(address))
    throw new Error('Owned Anvil signer differs from its configured mnemonic');
  const file = join(output, `account-${index}.key`);
  writeFileSync(file, account.privateKey + '\n', { mode: 0o600, flag: 'wx' });
  return file;
};
const descriptor = join(output, 'fixture.json');
writeFileSync(
  descriptor,
  JSON.stringify({
    realm: 'private_fixture',
    rpcUrl: core.rpcUrl,
    manifestFile,
    manifestSha256: sha(readFileSync(manifestFile)),
    userPrivateKeyFile: keyFile(6, user.address),
    keeperPrivateKeyFile: keyFile(4, keeper.address),
    adminPrivateKeyFile: keyFile(0, admin.address),
    storageFile: join(output, 'attempts.json'),
    token: token.target,
    buyAmount: '1.000001',
    limitPrice: ethers.formatUnits(poolPrice * 2n, 18),
    minimumOutput: '0.000001',
    core: {
      router: coreRouter.target,
      pool: pool.target,
      routerCodeHash: ethers.keccak256(await provider.getCode(coreRouter.target)),
      poolCodeHash: ethers.keccak256(await provider.getCode(pool.target)),
      amountInRaw: '1000000',
      minimumOutputRaw: '1',
    },
    multihop: {
      hops: [
        {
          tokenIn: usdl.target,
          tokenOut: wrapped.target,
          adapterId: adapters[0].id,
          adapterAddress: vaultAdapter.target,
          minAmountOut: '1',
          inputDecimals: 6,
          outputDecimals: 18,
        },
        {
          tokenIn: wrapped.target,
          tokenOut: token.target,
          adapterId: adapters[0].id,
          adapterAddress: vaultAdapter.target,
          minAmountOut: '1',
          inputDecimals: 18,
          outputDecimals: 6,
        },
      ],
      inputRaw: '1000000',
      finalMinimumRaw: '1',
      gasLimit: '3000000',
    },
  }) + '\n',
  { mode: 0o600, flag: 'wx' }
);
writeFileSync(
  join(output, 'execution-authority.json'),
  JSON.stringify({
    endpoint: `http://127.0.0.1:${server.address().port}/execute`,
    bearer: executionKey,
  }) + '\n',
  { mode: 0o600, flag: 'wx' }
);
writeFileSync(
  readyFile,
  JSON.stringify({
    L28_CHAIN_FIXTURE_JSON: descriptor,
    L28_CHAIN_FIXTURE_SHA256: sha(readFileSync(descriptor)),
  }) + '\n',
  { mode: 0o600, flag: 'wx' }
);
process.on('SIGTERM', () => {
  clearInterval(publications);
  server.closeAllConnections();
  server.close(() => {
    provider.destroy();
    process.exit(0);
  });
});

}

if (process.argv[2] === "--core") prepareOwnedCoreDeployment(...process.argv.slice(3));
else await prepareAdvancedDeployment();

const fs = require('node:fs');
const path = require('node:path');
const ganache = require('ganache');
const solc = require('solc');

async function start(root, deployment, viem) {
  const sources = {};
  const contracts = path.join(root, 'contracts/launchpad/contracts');
  for (const entry of [
    'periphery/Quoter.sol',
    'data/PoolRegistry.sol',
    'core/SidioraPool.sol',
    'core/SidioraERC20.sol',
    'protocol/ProtocolConfig.sol',
    'data/FeeAccumulator.sol',
    'meta-ag/proxy/ERC1967Proxy.sol',
  ]) {
    sources[entry] = { content: fs.readFileSync(path.join(contracts, entry), 'utf8') };
  }
  const compiled = JSON.parse(
    solc.compile(
      JSON.stringify({
        language: 'Solidity',
        sources,
        settings: {
          evmVersion: 'shanghai',
          optimizer: { enabled: true, runs: 200 },
          viaIR: true,
          outputSelection: {
            '*': { '*': ['abi', 'evm.bytecode.object', 'evm.deployedBytecode.object'] },
          },
        },
      }),
      {
        import(importPath) {
          const file = path.resolve(contracts, importPath);
          if (!file.startsWith(contracts + path.sep)) {
            return { error: 'Contract import must stay inside the shipped contracts directory' };
          }
          return { contents: fs.readFileSync(file, 'utf8') };
        },
      },
    ),
  );
  const errors = (compiled.errors || []).filter((error) => error.severity === 'error');
  if (errors.length) throw new Error(errors.map((error) => error.formattedMessage).join('\n'));
  const server = ganache.server({
    chain: { chainId: 125, hardfork: 'shanghai' },
    wallet: { totalAccounts: 3 },
    logging: { quiet: true },
  });
  await server.listen(0, '127.0.0.1');
  try {
    const origin = `http://127.0.0.1:${server.address().port}`;
    const provider = server.provider;
    const accounts = await provider.request({ method: 'eth_accounts', params: [] });
    const account = accounts[0];
    const client = viem.createPublicClient({ transport: viem.http(origin) });
    const wallet = viem.createWalletClient({ account, transport: viem.http(origin) });
    function artifact(file, name) {
      return compiled.contracts[file][name];
    }
    async function send(data, to) {
      const hash = await wallet.sendTransaction({ chain: null, data, to, gas: 12000000n });
      const receipt = await client.waitForTransactionReceipt({ hash });
      if (receipt.status !== 'success')
        throw new Error('Real fixture contract transaction reverted');
      return receipt;
    }
    async function deploy(file, name, args = []) {
      const contract = artifact(file, name);
      const receipt = await send(
        viem.encodeDeployData({
          abi: contract.abi,
          bytecode: `0x${contract.evm.bytecode.object}`,
          args,
        }),
      );
      return receipt.contractAddress;
    }
    async function write(file, name, address, functionName, args) {
      return send(
        viem.encodeFunctionData({ abi: artifact(file, name).abi, functionName, args }),
        address,
      );
    }
    const registryImpl = await deploy('data/PoolRegistry.sol', 'PoolRegistry');
    const quoterImpl = await deploy('periphery/Quoter.sol', 'Quoter');
    const proxy = artifact('meta-ag/proxy/ERC1967Proxy.sol', 'ERC1967Proxy');
    // Install the shipped proxy at the fixed manifest addresses on this owned EVM.
    // All storage initialization and discovery execute the shipped implementations.
    for (const [address, implementation] of [
      [deployment.poolRegistry, registryImpl],
      [deployment.quoter, quoterImpl],
    ]) {
      await provider.request({
        method: 'evm_setAccountCode',
        params: [address, `0x${proxy.evm.deployedBytecode.object}`],
      });
      await provider.request({
        method: 'evm_setAccountStorageAt',
        params: [
          address,
          '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc',
          viem.pad(implementation),
        ],
      });
    }
    await write('data/PoolRegistry.sol', 'PoolRegistry', deployment.poolRegistry, 'initialize', [
      viem.zeroAddress,
      account,
    ]);
    await write('periphery/Quoter.sol', 'Quoter', deployment.quoter, 'initialize', [
      deployment.poolRegistry,
      deployment.protocolConfig,
      account,
    ]);
    await write('data/PoolRegistry.sol', 'PoolRegistry', deployment.poolRegistry, 'grantRole', [
      viem.keccak256(viem.toHex('FACTORY_ROLE')),
      account,
    ]);
    const poolImpl = await deploy('core/SidioraPool.sol', 'SidioraPool');
    const protocol = await deploy('protocol/ProtocolConfig.sol', 'ProtocolConfig');
    const fees = await deploy('data/FeeAccumulator.sol', 'FeeAccumulator');
    const coins = [];
    for (const [name, symbol] of [
      ['Creator Alpha', 'ALPHA'],
      ['Creator Beta', 'BETA'],
      ['Unpublished token', 'UNKNOWN'],
    ]) {
      const token = await deploy('core/SidioraERC20.sol', 'SidioraERC20', [
        name,
        symbol,
        1000000000000000n,
        account,
      ]);
      const initialization = viem.encodeFunctionData({
        abi: artifact('core/SidioraPool.sol', 'SidioraPool').abi,
        functionName: 'initialize',
        args: [
          token,
          token,
          viem.zeroAddress,
          fees,
          viem.zeroAddress,
          protocol,
          account,
          1000000n,
          1000000000000000n,
        ],
      });
      const pool = await deploy('meta-ag/proxy/ERC1967Proxy.sol', 'ERC1967Proxy', [
        poolImpl,
        initialization,
      ]);
      const registered = await write(
        'data/PoolRegistry.sol',
        'PoolRegistry',
        deployment.poolRegistry,
        'register',
        [pool, token, account, viem.zeroAddress, BigInt(coins.length + 1)],
      );
      const event = viem.parseEventLogs({
        abi: artifact('data/PoolRegistry.sol', 'PoolRegistry').abi,
        logs: registered.logs,
        eventName: 'PoolRegistered',
      })[0];
      if (!event) throw new Error('Registry did not emit canonical identity');
      const tokenAbi = artifact('core/SidioraERC20.sol', 'SidioraERC20').abi;
      coins.push({
        Pool: event.args.pool,
        Token: event.args.token,
        Creator: event.args.creator,
        Name: await client.readContract({
          address: event.args.token,
          abi: tokenAbi,
          functionName: 'name',
        }),
        Symbol: await client.readContract({
          address: event.args.token,
          abi: tokenAbi,
          functionName: 'symbol',
        }),
      });
    }
    return { origin, provider, coins, close: () => server.close() };
  } catch (error) {
    await server.close();
    throw error;
  }
}
module.exports = { start };

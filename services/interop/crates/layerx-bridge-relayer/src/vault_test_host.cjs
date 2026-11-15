// Integration-test host for the actual vault and an actual local EVM node.
// The Rust test host provisions the pinned real compiler and EVM dependencies.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const readline = require('node:readline');
const ganache = require('ganache');
const solc = require('solc');
const ethers = require('ethers');

const idempotentReads = new Set([
  'eth_accounts', 'eth_chainId', 'eth_blockNumber', 'eth_call', 'eth_estimateGas',
  'eth_getBalance', 'eth_getCode', 'eth_getTransactionCount', 'eth_getTransactionByHash',
  'eth_getTransactionReceipt', 'eth_getBlockByNumber', 'eth_getBlockByHash',
  'eth_gasPrice', 'eth_maxPriorityFeePerGas', 'eth_feeHistory',
]);
class VaultProvider extends ethers.JsonRpcProvider {
  async _send(payload) {
    try {
      return await super._send(payload);
    } catch (error) {
      const requests = Array.isArray(payload) ? payload : [payload];
      if ((error.code !== 'ECONNRESET' && error.cause?.code !== 'ECONNRESET') ||
          !requests.every(request => idempotentReads.has(request.method))) {
        throw error;
      }
      return await super._send(payload);
    }
  }
}

const root = process.argv[2];
const server = ganache.server({
  chain: { chainId: 125, hardfork: 'shanghai' },
  wallet: { accounts: [{ secretKey: '0x' + '2a'.repeat(32), balance: '0x3635c9adc5dea00000' }] },
  logging: { quiet: true },
});
const attestor = key => new ethers.Wallet('0x' + key.toString(16).padStart(2, '0').repeat(32)).address;
let provider, owner, artifact;
const vaults = new Map();
function compile() {
  const source = 'contracts/bridge/evm/src/PaxeerXVault.sol';
  const input = { language: 'Solidity', sources: { [source]: { content: fs.readFileSync(path.join(root, source), 'utf8') } }, settings: {
    // Match contracts/bridge/evm/foundry.toml, targeting the local EVM hardfork.
    optimizer: { enabled: true, runs: 100000 }, viaIR: true, evmVersion: 'shanghai',
    metadata: { bytecodeHash: 'none', appendCBOR: false },
    outputSelection: { '*': { '*': ['abi', 'evm.bytecode.object'] } },
  } };
  const output = JSON.parse(solc.compile(JSON.stringify(input), { import(name) {
    const file = name.startsWith('@openzeppelin/') ? require.resolve(name) : path.join(root, name);
    return { contents: fs.readFileSync(file, 'utf8') };
  } }));
  assert.deepEqual((output.errors || []).filter(error => error.severity === 'error'), []);
  return output.contracts[source].PaxeerXVault;
}
async function command(input) {
  if (input.op === 'deploy') {
    artifact ||= compile();
    const factory = new ethers.ContractFactory(artifact.abi, artifact.evm.bytecode.object, owner);
    const vault = await factory.deploy(await owner.getAddress(), input.keys.map(attestor), input.threshold);
    await vault.waitForDeployment();
    const address = await vault.getAddress();
    vaults.set(address.toLowerCase(), vault);
    await (await vault.setCap(ethers.ZeroAddress, 1000000n, 10000000n)).wait();
    const recipient = '0x' + '00'.repeat(12) + 'ab'.repeat(20);
    const receipt = await (await vault.depositNative(recipient, { value: 10000n })).wait();
    // The relayer decodes this actual on-chain event, rather than a hand-built log.
    const raw = await provider.send('eth_getTransactionReceipt', [receipt.hash]);
    const deposit = raw.logs.find(log => log.address.toLowerCase() === address.toLowerCase());
    return { address: address.toLowerCase(), log: deposit };
  }
  const vault = vaults.get(input.vault?.toLowerCase());
  if (input.op === 'rotate') {
    await (await vault.setAttestors(input.keys.map(attestor), input.threshold)).wait();
    return { threshold: String(await vault.threshold()), attestors: Array.from(await vault.attestors()) };
  }
  if (input.op === 'verify' || input.op === 'release') {
    const response = await fetch(input.url, { signal: AbortSignal.timeout(5000) });
    assert.equal(response.status, 200);
    const bundle = await response.json();
    assert.equal(bundle.vault.toLowerCase(), input.vault.toLowerCase());
    const args = [bundle.asset, BigInt(bundle.amount), bundle.recipient, bundle.paxeerTxHash,
      BigInt(bundle.paxeerNonce), input.signatures || bundle.signatures];
    assert.equal((await vault.releaseDigest(args[3], args[4], args[2], args[0], args[1])).toLowerCase(), bundle.digest);
    try {
      await vault.release.staticCall(...args);
    } catch (error) {
      const refusal = vault.interface.parseError(error.data);
      return { accepted: false, refusal: refusal.name, signatures: bundle.signatures };
    }
    if (input.op === 'release') {
      const before = await vault.outstanding(bundle.asset);
      await (await vault.release(...args)).wait();
      assert.equal(await vault.outstanding(bundle.asset), before - BigInt(bundle.amount));
      assert.equal(await vault.nullified(await vault.nullifierOf(args[3], args[4])), true);
    }
    return { accepted: true, signatures: bundle.signatures };
  }
  throw new Error('unknown command: ' + input.op);
}
(async () => {
  await server.listen(0, '127.0.0.1');
  const url = `http://127.0.0.1:${server.address().port}`;
  const request = new ethers.FetchRequest(url);
  request.setHeader('Connection', 'close');
  provider = new VaultProvider(request);
  owner = await provider.getSigner(0);
  console.log(JSON.stringify({ url }));
  for await (const line of readline.createInterface({ input: process.stdin })) {
    try { console.log(JSON.stringify(await command(JSON.parse(line)))); }
    catch (error) { console.log(JSON.stringify({ error: error.stack || String(error) })); }
  }
  provider.destroy();
  await server.close();
})().catch(error => { console.error(error); process.exitCode = 1; });
